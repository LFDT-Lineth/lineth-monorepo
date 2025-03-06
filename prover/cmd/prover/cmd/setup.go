package cmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"

	blob_v0 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	blob_v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	v0 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v0"
	v1 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/emulation"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

type SetupArgs struct {
	Force      bool
	Circuits   string
	DictPath   string // to be deprecated; only used for compiling v0 blob decompression circuit
	DictSize   int
	AssetsDir  string
	ConfigFile string
}

var AllCircuits = []circuits.CircuitID{
	circuits.ExecutionCircuitID,
	circuits.ExecutionLargeCircuitID,
	circuits.BlobDecompressionV0CircuitID,
	circuits.BlobDecompressionV1CircuitID,
	circuits.PublicInputInterconnectionCircuitID,
	circuits.AggregationCircuitID,
	circuits.EmulationCircuitID,
	circuits.EmulationDummyCircuitID, // we want to generate Verifier.sol for this one
}

func Setup(context context.Context, args SetupArgs) error {
	const cmdName = "setup"
	// read config
	cfg, err := config.NewConfigFromFile(args.ConfigFile)
	if err != nil {
		return fmt.Errorf("%s failed to read config file: %w", cmdName, err)
	}

	if args.DictPath != "" {
		// fail early if the dictionary file is not found but was specified.
		if _, err := os.Stat(args.DictPath); err != nil {
			return fmt.Errorf("%s dictionary file not found: %w", cmdName, err)
		}
	}

	// parse inCircuits
	inCircuits := make(map[circuits.CircuitID]bool)
	for _, c := range AllCircuits {
		inCircuits[c] = false
	}
	_inCircuits := strings.Split(args.Circuits, ",")
	for _, c := range _inCircuits {
		if _, ok := inCircuits[circuits.CircuitID(c)]; !ok {
			return fmt.Errorf("%s unknown circuit: %s", cmdName, c)
		}
		inCircuits[circuits.CircuitID(c)] = true
	}

	// create assets dir if needed (example; efs://prover-assets/v0.1.0/)
	os.MkdirAll(filepath.Join(cfg.AssetsDir, cfg.Version), 0755)

	// srs provider
	var srsProvider circuits.SRSProvider
	srsProvider, err = circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		return fmt.Errorf("%s failed to create SRS provider: %w", cmdName, err)
	}
	var foundDecompressionV0 bool // this is a temporary mechanism to make sure we phase out the practice
	// of providing entire dictionaries for setup.

	// for each circuit, we start by compiling the circuit
	// then we do a sha sum and compare against the one in the manifest.json
	for _, c := range AllCircuits {

		setup := inCircuits[c]
		if !setup {
			// we skip aggregation in this first loop since the setup is more complex
			continue
		}
		logrus.Infof("setting up %s", c)

		var builder circuits.Builder
		extraFlags := make(map[string]any)

		// let's compile the circuit.
		switch c {
		case circuits.ExecutionCircuitID, circuits.ExecutionLargeCircuitID:
			limits := cfg.TracesLimits
			if c == circuits.ExecutionLargeCircuitID {
				limits = cfg.TracesLimitsLarge
			}
			extraFlags["cfg_checksum"] = limits.Checksum()
			zkEvm := zkevm.FullZkEvm(&limits, cfg)
			builder = execution.NewBuilder(zkEvm)

		case circuits.BlobDecompressionV0CircuitID:
			dict, err := os.ReadFile(args.DictPath)
			if err != nil {
				return fmt.Errorf("%s failed to read dictionary file: %w", cmdName, err)
			}
			foundDecompressionV0 = true
			extraFlags["maxUsableBytes"] = blob_v0.MaxUsableBytes
			extraFlags["maxUncompressedBytes"] = blob_v0.MaxUncompressedBytes
			builder = v0.NewBuilder(dict)
		case circuits.BlobDecompressionV1CircuitID:
			extraFlags["maxUsableBytes"] = blob_v1.MaxUsableBytes
			extraFlags["maxUncompressedBytes"] = blob_v1.MaxUncompressedBytes
			builder = v1.NewBuilder(args.DictSize)

		case circuits.PublicInputInterconnectionCircuitID:
			builder = pi_interconnection.NewBuilder(cfg.PublicInputInterconnection)
		case circuits.EmulationDummyCircuitID:
			// we can get the Verifier.sol from there.
			builder = dummy.NewBuilder(circuits.MockCircuitIDEmulation, ecc.BN254.ScalarField())
		default:
			continue // dummy, aggregation, emulation or public input circuits are handled later
		}

		if err := updateSetup(context, cfg, args.Force, srsProvider, c, builder, extraFlags); err != nil {
			return err
		}
	}

	if !foundDecompressionV0 && args.DictPath != "" {
		return errors.New("explicit provision of a dictionary is only allowed for backwards compatibility with v0 blob decompression")
	}

	if !(inCircuits[circuits.AggregationCircuitID] || inCircuits[circuits.EmulationCircuitID]) {
		// we are done
		return nil
	}

	// get verifying key for public-input circuit
	piSetup, err := circuits.LoadSetup(cfg, circuits.PublicInputInterconnectionCircuitID)
	if err != nil {
		return fmt.Errorf("%s failed to load public input interconnection setup: %w", cmdName, err)
	}

	// first, we need to collect the verifying keys
	allowedVkForAggregation := make([]plonk.VerifyingKey, 0, len(cfg.Aggregation.AllowedInputs))
	for _, allowedInput := range cfg.Aggregation.AllowedInputs {
		// first if it's a dummy circuit, we just run the setup here, we don't need to persist it.
		if isDummyCircuit(allowedInput) {
			var curveID ecc.ID
			var mockID circuits.MockCircuitID
			switch allowedInput {
			case string(circuits.ExecutionDummyCircuitID):
				curveID = ecc.BLS12_377
				mockID = circuits.MockCircuitIDExecution
			case string(circuits.BlobDecompressionDummyCircuitID):
				curveID = ecc.BLS12_377
				mockID = circuits.MockCircuitIDDecompression
			case string(circuits.EmulationDummyCircuitID):
				curveID = ecc.BN254
				mockID = circuits.MockCircuitIDEmulation
			default:
				return fmt.Errorf("unknown dummy circuit: %s", allowedInput)
			}

			vk, err := getDummyCircuitVK(context, cfg, srsProvider, circuits.CircuitID(allowedInput), dummy.NewBuilder(mockID, curveID.ScalarField()))
			if err != nil {
				return err
			}
			allowedVkForAggregation = append(allowedVkForAggregation, vk)
			continue
		}

		// derive the asset paths
		setupPath := cfg.PathForSetup(allowedInput)
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)
		vk := plonk.NewVerifyingKey(ecc.BLS12_377)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return fmt.Errorf("%s failed to read verifying key for circuit %s: %w", cmdName, allowedInput, err)
		}

		allowedVkForAggregation = append(allowedVkForAggregation, vk)
	}

	// we need to compute the digest of the verifying keys & store them in the manifest
	// for the aggregation circuits to be able to check compatibility at run time with the proofs
	allowedVkForAggregationDigests := listOfChecksums(allowedVkForAggregation)
	extraFlagsForAggregationCircuit := map[string]any{
		"allowedVkForAggregationDigests": allowedVkForAggregationDigests,
	}

	// now for each aggregation circuit, we update the setup if needed, and collect the verifying keys
	allowedVkForEmulation := make([]plonk.VerifyingKey, 0, len(cfg.Aggregation.NumProofs))
	for _, numProofs := range cfg.Aggregation.NumProofs {
		c := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), numProofs))
		logrus.Infof("setting up %s (numProofs=%d)", c, numProofs)

		builder := aggregation.NewBuilder(numProofs, cfg.Aggregation.AllowedInputs, piSetup, allowedVkForAggregation)
		if err := updateSetup(context, cfg, args.Force, srsProvider, c, builder, extraFlagsForAggregationCircuit); err != nil {
			return err
		}

		// read the verifying key
		setupPath := cfg.PathForSetup(string(c))
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)
		vk := plonk.NewVerifyingKey(ecc.BW6_761)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return fmt.Errorf("%s failed to read verifying key for circuit %s: %w", cmdName, c, err)
		}

		allowedVkForEmulation = append(allowedVkForEmulation, vk)
	}

	// now we can update the final (emulation) circuit
	c := circuits.EmulationCircuitID
	logrus.Infof("setting up %s", c)
	builder := emulation.NewBuilder(allowedVkForEmulation)
	return updateSetup(context, cfg, args.Force, srsProvider, c, builder, nil)

}

func isDummyCircuit(cID string) bool {
	switch circuits.CircuitID(cID) {
	case circuits.ExecutionDummyCircuitID, circuits.BlobDecompressionDummyCircuitID, circuits.EmulationDummyCircuitID:
		return true
	}
	return false

}

func getDummyCircuitVK(ctx context.Context, cfg *config.Config, srsProvider circuits.SRSProvider, circuit circuits.CircuitID, builder circuits.Builder) (plonk.VerifyingKey, error) {
	// compile the circuit
	logrus.Infof("compiling %s", circuit)
	ccs, err := builder.Compile()
	if err != nil {
		return nil, fmt.Errorf("failed to compile circuit %s: %w", circuit, err)
	}
	setup, err := circuits.MakeSetup(ctx, circuit, ccs, srsProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to setup circuit %s: %w", circuit, err)
	}

	return setup.VerifyingKey, nil
}

// updateSetup runs the setup for the given circuit if needed.
// it first compiles the circuit, then checks if the files already exist,
// and if so, if the checksums match.
// if the files already exist and the checksums match, it skips the setup.
// else it does the setup and writes the assets to disk.
func updateSetup(ctx context.Context, cfg *config.Config, force bool, srsProvider circuits.SRSProvider, circuit circuits.CircuitID, builder circuits.Builder, extraFlags map[string]any) error {
	if extraFlags == nil {
		extraFlags = make(map[string]any)
	}

	// compile the circuit
	logrus.Infof("compiling %s", circuit)
	ccs, err := builder.Compile()
	if err != nil {
		return fmt.Errorf("failed to compile circuit %s: %w", circuit, err)
	}

	// derive the asset paths
	setupPath := cfg.PathForSetup(string(circuit))
	manifestPath := filepath.Join(setupPath, config.ManifestFileName)

	if !force {
		// we may want to skip setup if the files already exist
		// and the checksums match
		// read manifest if already exists
		if manifest, err := circuits.ReadSetupManifest(manifestPath); err == nil {
			circuitDigest, err := circuits.CircuitDigest(ccs)
			if err != nil {
				return fmt.Errorf("failed to compute circuit digest for circuit %s: %w", circuit, err)
			}

			if manifest.Checksums.Circuit == circuitDigest {
				logrus.Infof("skipping %s (already setup)", circuit)
				return nil
			}
		}
	}

	// run the actual setup
	logrus.Infof("plonk setup for %s", circuit)
	setup, err := circuits.MakeSetup(ctx, circuit, ccs, srsProvider, extraFlags)
	if err != nil {
		return fmt.Errorf("failed to setup circuit %s: %w", circuit, err)
	}

	logrus.Infof("writing assets for %s", circuit)
	return setup.WriteTo(setupPath)
}

// listOfChecksums Computes a list of SHA256 checksums for a list of assets, the result is given
// in hexstring.
func listOfChecksums[T io.WriterTo](assets []T) []string {
	res := make([]string, len(assets))
	h := sha256.New()
	for i := range assets {
		h.Reset()
		_, err := assets[i].WriteTo(h)
		if err != nil {
			// It is unexpected that writing in a hasher could possibly fail.
			panic(err)
		}
		digest := h.Sum(nil)
		res[i] = utils.HexEncodeToString(digest)
	}
	return res
}
