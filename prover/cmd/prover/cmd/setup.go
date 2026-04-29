package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/utils/signal"

	blob_v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	gnarkio "github.com/consensys/gnark/io"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	daconfig "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/config"
	blobdecompression "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
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
	AssetsDir  string
	ConfigFile string
}

var AllCircuits = []circuits.CircuitID{
	circuits.ExecutionCircuitID,
	circuits.ExecutionLargeCircuitID,
	circuits.ExecutionLimitlessCircuitID,
	circuits.DataAvailabilityV2CircuitID,
	circuits.InvalidityNonceBalanceCircuitID,
	circuits.InvalidityPrecompileLogsCircuitID,
	circuits.InvalidityPrecompileLogsLargeCircuitID,
	circuits.InvalidityPrecompileLogsLimitlessCircuitID,
	circuits.InvalidityFilteredAddressCircuitID,
	circuits.PublicInputInterconnectionCircuitID,
	circuits.AggregationCircuitID,
	circuits.EmulationCircuitID,
	circuits.EmulationDummyCircuitID, // we want to generate Verifier.sol for this one
}

// PayloadCircuits defines the ordered list of payload circuits that can be aggregated.
// This order corresponds to circuit IDs 0-5 in GlobalCircuitIDMapping.
// Infrastructure circuits (emulation, aggregation, pi-interconnection, emulation-dummy) are NOT included here.
var PayloadCircuits = []string{
	"execution-dummy",         // ID 0
	"data-availability-dummy", // ID 1
	"execution",               // ID 2
	"execution-large",         // ID 3
	"execution-limitless",     // ID 4
	"data-availability-v2",    // ID 5
}

// Setup orchestrates the setup process for specified circuits, ensuring assets are generated or updated as needed.
func Setup(ctx context.Context, args SetupArgs) error {
	const cmdName = "setup"

	// This allows the user to dump stacktraces by sending a SIGUSR1 to the
	// current process.
	signal.RegisterStackTraceDumpHandler()

	// Read config from file
	cfg, err := config.NewConfigFromFile(args.ConfigFile)
	if err != nil {
		return fmt.Errorf("%s failed to read config file: %w", cmdName, err)
	}

	// Parse inCircuits
	inCircuits, err := parseCircuitInputs(args.Circuits)
	if err != nil {
		return fmt.Errorf("%s unknown circuit: %w", cmdName, err)
	}

	// Create assets dir if needed (example; efs://prover-assets/v0.1.0/)
	if err := os.MkdirAll(filepath.Join(cfg.AssetsDir, cfg.Version), 0755); err != nil {
		return fmt.Errorf("%s failed to create assets directory: %w", cmdName, err)
	}

	if err := copyConfigToAssets(cfg, args.ConfigFile, cmdName); err != nil {
		return fmt.Errorf("%s failed to copy config file to the assets directory: %w", cmdName, err)
	}

	// srs provider
	srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		return fmt.Errorf("%s failed to create SRS provider: %w", cmdName, err)
	}

	// Setup non-aggregation and non-emulation circuits first
	// For each circuit, we start by compiling the circuit, and
	// then we do a SHA-sum and compare against the one in the manifest.json
	for _, c := range AllCircuits {
		if !inCircuits[c] || c == circuits.AggregationCircuitID ||
			c == circuits.EmulationCircuitID {
			// we skip aggregation/emulation circuits in this first loop since the setup is more complex
			continue
		}

		logrus.Infof("--- Start of circuit %s setup ---", c)

		// Build the circuit
		builder, extraFlags, err := createCircuitBuilder(c, cfg, args)
		if err != nil {
			return fmt.Errorf("%s failed to create builder for circuit %s: %w", cmdName, c, err)
		}

		if err := updateSetup(ctx, cfg, args.Force, srsProvider, c, builder, extraFlags); err != nil {
			return err
		}

		if c == circuits.ExecutionCircuitID || c == circuits.ExecutionLargeCircuitID {
			if err := serializeInnerCircuit(cfg, c); err != nil {
				return err
			}
		}
	}

	// Early exit if no aggregation or emulation circuits
	if !inCircuits[circuits.AggregationCircuitID] && !inCircuits[circuits.EmulationCircuitID] {
		// we are done
		return nil
	}

	// Get verifying key for public-input circuit
	piSetup, err := circuits.LoadSetup(cfg, circuits.PublicInputInterconnectionCircuitID)
	if err != nil {
		return fmt.Errorf("%s failed to load public input interconnection setup: %w", cmdName, err)
	}

	// Collect verifying keys for payload circuits only (IDs 0-5)
	// The IsAllowedCircuitID bitmask in the config determines which ones are actually allowed at runtime
	payloadVks, err := collectPayloadVerifyingKeys(ctx, cfg, srsProvider)
	if err != nil {
		return err
	}

	// Setup aggregation circuits
	allowedVkForEmulation, err := setupAggregationCircuits(ctx, cfg, args.Force, srsProvider, inCircuits, &piSetup, payloadVks)
	if err != nil {
		return err
	}

	// Setup emulation circuit if needed
	if inCircuits[circuits.EmulationCircuitID] {
		logrus.Infof("setting up %s", circuits.EmulationCircuitID)
		builder := emulation.NewBuilder(allowedVkForEmulation)
		return updateSetup(ctx, cfg, args.Force, srsProvider, circuits.EmulationCircuitID, builder, nil)
	}

	logrus.Infof("Done setting up circuits and writing the assets to disk :)")
	return nil
}

// updateSetup: Runs the setup for the given circuit if needed.
// It first compiles the circuit, then checks if the files already exist, and if so, if the checksums match.
// If the files already exist and the checksums match, it skips the setup.
// else it does the setup and writes the assets to disk.
func updateSetup(ctx context.Context, cfg *config.Config, force bool,
	srsProvider circuits.SRSProvider, circuit circuits.CircuitID,
	builder circuits.Builder, extraFlags map[string]any,
) error {
	if extraFlags == nil {
		extraFlags = make(map[string]any)
	}

	// compile the circuit
	logrus.Infof("Compiling circuit %s", circuit)
	ccs, err := builder.Compile()
	if err != nil {
		return fmt.Errorf("failed to compile circuit %s: %w", circuit, err)
	}

	// Derive the asset paths
	setupPath := cfg.PathForSetup(string(circuit))
	manifestPath := filepath.Join(setupPath, config.ManifestFileName)
	logrus.Infof("Manifest path: %s", manifestPath)

	// check if setup can be skipped
	if !force {
		// we may want to skip setup if the files already exist
		// and the checksums match
		// read manifest if already exists
		if manifest, err := circuits.ReadSetupManifest(manifestPath); err == nil {
			circuitDigest, err := circuits.CircuitDigest(ccs)
			if err != nil {
				return fmt.Errorf("failed to compute circuit digest for circuit %s: %w", circuit, err)
			}
			logrus.Infof("Manifest checksum: %s, Computed circuit digest: %s", manifest.Checksums.Circuit, circuitDigest)
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

	// write the assets
	err = setup.WriteTo(setupPath)
	if err != nil {
		return fmt.Errorf("failed to write assets for circuit %s: %w", circuit, err)
	}

	logrus.Infof("Successfully wrote circuit %s to %s", circuit, setupPath)
	logrus.Infof("--- End of circuit %s setup ---", circuit)
	return nil
}

// serializeInnerCircuit serializes the compiled inner circuit (wizard IOP) to disk
// if the config has serialization enabled.
func serializeInnerCircuit(cfg *config.Config, c circuits.CircuitID) error {
	enabled, innerCircuitPath := cfg.ExecutionCircuitBin(string(c))
	if !enabled {
		return nil
	}

	limits := cfg.TracesLimits
	if c == circuits.ExecutionLargeCircuitID {
		limits.SetLargeMode()
	}

	// FullZkEvm is memoized via sync.Once — returns the cached compiled instance
	zkEvm := zkevm.FullZkEvm(&limits, cfg)
	logrus.Infof("Serializing inner circuit for %s to %s", c, innerCircuitPath)
	if err := serde.StoreToDisk(innerCircuitPath, zkEvm, false); err != nil {
		return fmt.Errorf("failed to serialize inner circuit for %s: %w", c, err)
	}
	return nil
}

// parseCircuitInputs: Converts the comma-separated circuit string into a map of enabled circuits.
func parseCircuitInputs(circuitsStr string) (map[circuits.CircuitID]bool, error) {
	inCircuits := make(map[circuits.CircuitID]bool)
	for _, c := range AllCircuits {
		inCircuits[c] = false
	}

	for _, c := range strings.Split(circuitsStr, ",") {
		circuitID := circuits.CircuitID(c)
		if _, ok := inCircuits[circuitID]; !ok {
			return nil, fmt.Errorf("invalid circuit: %s", c)
		}
		inCircuits[circuitID] = true
	}
	return inCircuits, nil
}

// createCircuitBuilder: Constructs the appropriate circuit builder and extra flags based on the circuit ID.
func createCircuitBuilder(c circuits.CircuitID, cfg *config.Config, args SetupArgs,
) (circuits.Builder, map[string]any, error) {
	extraFlags := make(map[string]any)

	switch c {
	case circuits.ExecutionCircuitID:
		limits := cfg.TracesLimits
		extraFlags["cfg_checksum"] = limits.Checksum()
		zkEvm := zkevm.FullZkEvm(&limits, cfg)
		return execution.NewBuilder(zkEvm), extraFlags, nil

	case circuits.ExecutionLargeCircuitID:
		limits := cfg.TracesLimits
		limits.SetLargeMode()
		extraFlags["cfg_checksum"] = limits.Checksum()
		zkEvm := zkevm.FullZkEvmLarge(&limits, cfg)
		return execution.NewBuilder(zkEvm), extraFlags, nil

	case circuits.ExecutionLimitlessCircuitID:
		executionLimitlessPath := cfg.PathForSetup("execution-limitless")
		limits := cfg.TracesLimits
		extraFlags["cfg_checksum"] = limits.Checksum()

		logrus.Info("Setting up limitless prover assets")
		asset := zkevm.NewLimitlessZkEVM(cfg)

		// Unlike for the other circuits, the limitless prover assets are written
		// to disk directly before returning the circuit builder. The reason is
		// that the limitless prover assets are large and we want to avoid keeping
		// them in memory. The second reason is that returning them alongside the
		// build would change the structure of the function for just one case.
		logrus.Infof("Writing limitless prover assets to path: %s", executionLimitlessPath)
		if err := asset.Store(cfg); err != nil {
			return nil, nil, fmt.Errorf("failed to write limitless prover assets: %w", err)
		}

		compCong := asset.DistWizard.CompiledConglomeration
		vkMerkleRoot := asset.DistWizard.VerificationKeyMerkleTree.GetRoot()

		asset = nil
		runtime.GC()

		return execution.NewBuilderLimitless(compCong.RecursionCompBLS, &limits, vkMerkleRoot), extraFlags, nil

	case circuits.DataAvailabilityV2CircuitID:
		extraFlags["maxUsableBytes"] = blob_v1.MaxUsableBytes
		extraFlags["maxUncompressedBytes"] = cfg.DataAvailability.MaxUncompressedNbBytes
		extraFlags["dictNbBytes"] = cfg.DataAvailability.DictNbBytes
		extraFlags["maxNbBatches"] = cfg.DataAvailability.MaxNbBatches
		return blobdecompression.NewBuilder(daconfig.FromGlobalConfig(cfg.DataAvailability)), extraFlags, nil

	case circuits.PublicInputInterconnectionCircuitID:
		return pi_interconnection.NewBuilder(cfg.PublicInputInterconnection), extraFlags, nil

	case circuits.InvalidityNonceBalanceCircuitID:
		// BadNonce/BadBalance circuit needs KeccakCompiledIOP
		keccakComp := invalidity.MakeKeccakCompiledIOP(cfg.Invalidity.MaxRlpByteSize, keccak.WizardCompilationParameters()...)
		return invalidity.NewBuilder(
			invalidity.Config{
				Depth:             smt_koalabear.DefaultDepth, // account trie depth
				KeccakCompiledIOP: keccakComp,
				MaxRlpByteSize:    cfg.Invalidity.MaxRlpByteSize,
			},
			&invalidity.BadNonceBalanceCircuit{}), extraFlags, nil

	case circuits.InvalidityPrecompileLogsCircuitID:
		limits := cfg.TracesLimits
		extraFlags["cfg_checksum"] = limits.Checksum()
		zkEvm := zkevm.FullZkEvm(&limits, cfg)
		return invalidity.NewBuilder(invalidity.Config{
			ZkEvmComp: zkEvm.RecursionCompiledIOP,
		}, &invalidity.BadPrecompileCircuit{}), extraFlags, nil

	case circuits.InvalidityPrecompileLogsLargeCircuitID:
		limits := cfg.TracesLimits
		limits.SetLargeMode()
		extraFlags["cfg_checksum"] = limits.Checksum()
		zkEvm := zkevm.FullZkEvmLarge(&limits, cfg)
		return invalidity.NewBuilder(invalidity.Config{
			ZkEvmComp: zkEvm.RecursionCompiledIOP,
		}, &invalidity.BadPrecompileCircuit{}), extraFlags, nil

	case circuits.InvalidityPrecompileLogsLimitlessCircuitID:
		limitlessPath := cfg.PathForSetup("invalidity-precompile-logs-limitless")
		extraFlags["cfg_checksum"] = cfg.TracesLimits.Checksum()

		logrus.Info("Setting up limitless invalidity prover assets")
		asset := zkevm.NewLimitlessZkEVM(cfg)

		logrus.Infof("Writing limitless invalidity prover assets to path: %s", limitlessPath)
		if err := asset.Store(cfg); err != nil {
			return nil, nil, fmt.Errorf("failed to write limitless invalidity prover assets: %w", err)
		}
		compCong := asset.DistWizard.CompiledConglomeration
		asset = nil
		runtime.GC()

		return invalidity.NewBuilderLimitless(compCong.RecursionCompBLS), extraFlags, nil
	case circuits.InvalidityFilteredAddressCircuitID:
		keccakComp := invalidity.MakeKeccakCompiledIOP(cfg.Invalidity.MaxRlpByteSize, keccak.WizardCompilationParameters()...)
		return invalidity.NewBuilder(
			invalidity.Config{
				Depth:             smt_koalabear.DefaultDepth,
				KeccakCompiledIOP: keccakComp,
				MaxRlpByteSize:    cfg.Invalidity.MaxRlpByteSize,
			},
			&invalidity.FilteredAddressCircuit{}), extraFlags, nil

	case circuits.EmulationDummyCircuitID:
		// we can get the Verifier.sol from there.
		return dummy.NewBuilder(circuits.MockCircuitIDEmulation, ecc.BN254.ScalarField()), extraFlags, nil

	default:
		return nil, nil, fmt.Errorf("unsupported circuit: %s", c)
	}
}

// collectPayloadVerifyingKeys gathers verifying keys for payload circuits only (IDs 0-5).
// These are the circuits that can be aggregated. Infrastructure circuits (emulation,
// aggregation, pi-interconnection, emulation-dummy) are excluded.
// The returned slice is indexed by payload circuit ID (0-5).
func collectPayloadVerifyingKeys(ctx context.Context, cfg *config.Config, srsProvider circuits.SRSProvider) ([]plonk.VerifyingKey, error) {
	payloadVks := make([]plonk.VerifyingKey, len(PayloadCircuits))

	for i, circuitName := range PayloadCircuits {
		logrus.Infof("Collecting verifying key for payload circuit %s (ID %d)", circuitName, i)

		if isPayloadDummyCircuit(circuitName) {
			curveID, mockID, err := getDummyCircuitParams(circuitName)
			if err != nil {
				return nil, err
			}
			vk, err := getDummyCircuitVK(ctx, srsProvider, circuits.CircuitID(circuitName), dummy.NewBuilder(mockID, curveID.ScalarField()))
			if err != nil {
				return nil, err
			}
			payloadVks[i] = vk
			continue
		}

		// Read VK from disk for non-dummy circuits
		setupPath := cfg.PathForSetup(circuitName)
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)

		vk := plonk.NewVerifyingKey(ecc.BLS12_377)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return nil, fmt.Errorf("failed to read verifying key for circuit %s: %w", circuitName, err)
		}
		payloadVks[i] = vk
	}

	return payloadVks, nil
}

// isPayloadDummyCircuit returns true if the circuit is a dummy circuit for payload testing.
// Note: emulation-dummy is NOT a payload dummy - it's an infrastructure circuit dummy.
func isPayloadDummyCircuit(cID string) bool {
	switch circuits.CircuitID(cID) {
	case circuits.ExecutionDummyCircuitID, circuits.DataAvailabilityDummyCircuitID:
		return true
	}
	return false
}

// getDummyCircuitParams returns the curve and mock ID for a dummy circuit.
func getDummyCircuitParams(cID string) (ecc.ID, circuits.MockCircuitID, error) {
	switch circuits.CircuitID(cID) {
	case circuits.ExecutionDummyCircuitID:
		return ecc.BLS12_377, circuits.MockCircuitIDExecution, nil
	case circuits.DataAvailabilityDummyCircuitID:
		return ecc.BLS12_377, circuits.MockCircuitIDDecompression, nil
	case circuits.EmulationDummyCircuitID:
		return ecc.BN254, circuits.MockCircuitIDEmulation, nil
	case circuits.InvalidityNonceBalanceDummyCircuitID:
		return ecc.BLS12_377, circuits.MockCircuitIDInvalidityNonceBalance, nil
	case circuits.InvalidityPrecompileLogsDummyCircuitID:
		return ecc.BLS12_377, circuits.MockCircuitIDInvalidityPrecompileLogs, nil
	case circuits.InvalidityFilteredAddressDummyCircuitID:
		return ecc.BLS12_377, circuits.MockCircuitIDInvalidityFilteredAddress, nil
	default:
		return 0, 0, fmt.Errorf("unknown dummy circuit: %s", cID)
	}
}

// setupAggregationCircuits: Configures aggregation circuits and collects their verifying keys for emulation.
func setupAggregationCircuits(ctx context.Context, cfg *config.Config, force bool,
	srsProvider circuits.SRSProvider, inCircuits map[circuits.CircuitID]bool,
	piSetup *circuits.Setup, payloadVks []plonk.VerifyingKey,
) ([]plonk.VerifyingKey, error) {

	if !inCircuits[circuits.AggregationCircuitID] {

		// Aggregation was not requested, but emulation may still need the
		// aggregation verifying keys. Try to read them from disk.
		if inCircuits[circuits.EmulationCircuitID] {
			allowedVkForEmulation := make([]plonk.VerifyingKey, 0, len(cfg.Aggregation.NumProofs))
			for _, numProofs := range cfg.Aggregation.NumProofs {
				c := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), numProofs))
				setupPath := cfg.PathForSetup(string(c))
				vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)

				vk := plonk.NewVerifyingKey(ecc.BW6_761)
				if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
					return nil, fmt.Errorf("failed to read verifying key for circuit %s (needed by emulation): %w", c, err)
				}
				allowedVkForEmulation = append(allowedVkForEmulation, vk)
			}
			return allowedVkForEmulation, nil
		}

		return nil, nil
	}

	// we need to compute the digest of the verifying keys & store them in the manifest
	// for the aggregation circuits to be able to check compatibility at run time with the proofs
	extraFlags := map[string]any{
		"allowedVkForAggregationDigests":      listOfChecksums(payloadVks),
		"allowedVkForAggregationCircuitNames": PayloadCircuits,
	}

	allowedVkForEmulation := make([]plonk.VerifyingKey, 0, len(cfg.Aggregation.NumProofs))

	for _, numProofs := range cfg.Aggregation.NumProofs {
		c := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), numProofs))
		logrus.Infof("setting up %s (numProofs=%d)", c, numProofs)

		// Pass payload VKs to aggregation builder
		builder := aggregation.NewBuilder(numProofs, *piSetup, payloadVks)

		if err := updateSetup(ctx, cfg, force, srsProvider, c, builder, extraFlags); err != nil {
			return nil, err
		}

		// read the verifying key
		setupPath := cfg.PathForSetup(string(c))
		vkPath := filepath.Join(setupPath, config.VerifyingKeyFileName)

		vk := plonk.NewVerifyingKey(ecc.BW6_761)
		if err := circuits.ReadVerifyingKey(vkPath, vk); err != nil {
			return nil, fmt.Errorf("failed to read verifying key for circuit %s: %w", c, err)
		}
		allowedVkForEmulation = append(allowedVkForEmulation, vk)
	}

	return allowedVkForEmulation, nil
}

// getDummyCircuitVK compiles a dummy circuit and returns its verifying key.
// This is used by collectVerifyingKeys to get VKs for dummy circuits.
func getDummyCircuitVK(ctx context.Context, srsProvider circuits.SRSProvider, circuit circuits.CircuitID, builder circuits.Builder) (plonk.VerifyingKey, error) {
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

// listOfChecksums computes a list of SHA256 checksums for a list of assets.
// It uses WriteRawTo (uncompressed/raw encoding) to match the prove-time
// checksum computed by circuits.ObjectChecksum / writeToWriter.
//
// IMPORTANT: All gnark VKs implement WriteRawTo. Using WriteTo instead would
// produce compressed-encoding hashes that differ from prove-time hashes,
// causing aggregation VK mismatch failures at runtime.
func listOfChecksums[T io.WriterTo](assets []T) []string {
	res := make([]string, len(assets))
	h := sha256.New()
	for i := range assets {
		h.Reset()
		raw, ok := any(assets[i]).(gnarkio.WriterRawTo)
		if !ok {
			panic(fmt.Sprintf("listOfChecksums: asset at index %d (type %T) does not implement gnarkio.WriterRawTo; "+
				"using WriteTo would produce a different hash than prove-time objectChecksum", i, assets[i]))
		}
		if _, err := raw.WriteRawTo(h); err != nil {
			panic(fmt.Sprintf("listOfChecksums: WriteRawTo failed for asset %d: %v", i, err))
		}
		digest := h.Sum(nil)
		res[i] = utils.HexEncodeToString(digest)
	}
	return res
}

// copyConfigToAssets creates the config directory under assets dir and copies the config file
func copyConfigToAssets(cfg *config.Config, configFilePath string, cmdName string) error {
	configDir := filepath.Join(cfg.AssetsDir, cfg.Version, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("%s failed to create config directory: %w", cmdName, err)
	}

	configFileDst := filepath.Join(configDir, filepath.Base(configFilePath))

	srcFile, err := os.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("%s failed to open config file for copying: %w", cmdName, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(configFileDst)
	if err != nil {
		return fmt.Errorf("%s failed to create destination config file: %w", cmdName, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("%s failed to copy config file: %w", cmdName, err)
	}

	return nil
}
