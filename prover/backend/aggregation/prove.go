package aggregation

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"

	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/emulation"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/sirupsen/logrus"

	"github.com/consensys/gnark-crypto/ecc"
	frBW6 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"

	frBn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

func Prove(cfg *config.Config, req *Request) (*Response, error) {
	cf, err := collectFields(cfg, req)
	if err != nil {
		return nil, fmt.Errorf("could not collect the fields: %w", err)
	}

	return CraftResponse(cfg, cf)
}

// Run the concrete prover for the aggregation
func makeProof(
	cfg *config.Config,
	cf *CollectedFields,
	publicInput string,
) (proof string, err error) {

	if cfg.Aggregation.ProverMode == config.ProverModeDev {
		// In the development mode, we generate a fake proof
		return makeDummyProof(cfg, publicInput, circuits.MockCircuitIDEmulation), nil
	}

	piProof, piPublicWitness, err := makePiProof(cfg, cf)
	if err != nil {
		return "", fmt.Errorf("could not create the public input proof: %w", err)
	}

	proofBW6, circuitID, err := makeBw6Proof(cfg, cf, piProof, piPublicWitness, publicInput)
	if err != nil {
		return "", fmt.Errorf("error when running the BW6 proof: %w", err)
	}

	proofBn254, err := makeBn254Proof(cfg, circuitID, proofBW6, publicInput)
	if err != nil {
		return "", fmt.Errorf("error when running the Bn254 proof: %w", err)
	}

	return circuits.SerializeProofSolidityBn254(proofBn254), nil
}

func makePiProof(cfg *config.Config, cf *CollectedFields) (plonk.Proof, witness.Witness, error) {

	var setup circuits.Setup
	setupErr := make(chan error, 1)

	go func() {
		var err error
		setup, err = circuits.LoadSetup(cfg, circuits.PublicInputInterconnectionCircuitID)
		setupErr <- err
		close(setupErr)
	}()

	c, err := pi_interconnection.Compile(cfg.PublicInputInterconnection, pi_interconnection.WizardCompilationParameters()...)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create the public-input circuit: %w", err)
	}

	assignment, err := c.Assign(pi_interconnection.Request{
		Decompressions: cf.DecompressionPI,
		Executions:     cf.ExecutionPI,
		Aggregation: public_input.Aggregation{
			FinalShnarf:                             cf.FinalShnarf,
			ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
			ParentStateRootHash:                     cf.ParentStateRootHash,
			ParentAggregationLastBlockTimestamp:     cf.ParentAggregationLastBlockTimestamp,
			FinalTimestamp:                          cf.FinalTimestamp,
			LastFinalizedBlockNumber:                cf.LastFinalizedBlockNumber,
			FinalBlockNumber:                        cf.FinalBlockNumber,
			LastFinalizedL1RollingHash:              cf.LastFinalizedL1RollingHash,
			L1RollingHash:                           cf.L1RollingHash,
			LastFinalizedL1RollingHashMessageNumber: cf.LastFinalizedL1RollingHashMessageNumber,
			L1RollingHashMessageNumber:              cf.L1RollingHashMessageNumber,
			L2MsgRootHashes:                         cf.L2MsgRootHashes,
			L2MsgMerkleTreeDepth:                    utils.ToInt(cf.L2MsgTreeDepth),
			ChainID:                                 uint64(cfg.Layer2.ChainID),
			L2MessageServiceAddr:                    types.EthAddress(cfg.Layer2.MsgSvcContract),
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not assign the public input circuit: %w", err)
	}

	w, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField(), frontend.PublicOnly()) // TODO @Tabaie make ProveCheck return witness instead of extracting this twice
	if err != nil {
		return nil, nil, fmt.Errorf("could not extract interconnection circuit public witness: %w", err)
	}

	if err = <-setupErr; err != nil { // wait for setup to load and check for errors
		return nil, nil, fmt.Errorf("could not load the setup: %w", err)
	}

	proverOpts := emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), setup.Circuit.Field())
	verifierOpts := emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), setup.Circuit.Field())

	proof, err := circuits.ProveCheck(&setup, &assignment, proverOpts, verifierOpts)

	return proof, w, err
}

// Generates a fake proof. The public input is given in hex string format.
// Returns the proof in hex string format. The circuit ID parameter specifies
// for which circuit should the proof be generated.
func makeDummyProof(cfg *config.Config, input string, circID circuits.MockCircuitID) string {
	// TODO @gbotrel why do we do setup at run time here? we could factorize with other paths.
	srsProvider, err := circuits.NewSRSStore(cfg.PathForSRS())
	if err != nil {
		panic(err)
	}

	setup, err := dummy.MakeUnsafeSetup(srsProvider, circID, ecc.BN254.ScalarField())
	if err != nil {
		panic(err)
	}
	var (
		x      frBn254.Element
		xBytes []byte
	)

	// We encode the input ourselves so we can trust that the decoding will be
	// successful and the error can safely be ignored.
	xBytes, _ = hexutil.Decode(input)
	x.SetBytes(xBytes)

	return dummy.MakeProof(&setup, x, circID)
}

func makeBw6Proof(
	cfg *config.Config,
	cf *CollectedFields,
	piProof plonk.Proof,
	piPublicWitness witness.Witness,
	publicInput string,
) (proof plonk.Proof, circuitID int, err error) {

	// This determines which is the best circuit to use for aggregation, we
	// take the smallest circuit that has enough capacity.

	var (
		numProofClaims              = len(cf.ProofClaims)
		biggestAvailable            = 0
		bestSize                    = math.MaxInt
		bestSetupPos                = -1
		bestAllowedVkForAggregation []string
	)

	// first we discover available setups
	for _, maxNbProofs := range cfg.Aggregation.NumProofs {
		biggestAvailable = max(biggestAvailable, maxNbProofs)

		// That's the quickest reject condition we have
		if maxNbProofs < numProofClaims {
			logrus.Debugf("skipping setup with %v proofs < %v claims", maxNbProofs, numProofClaims)
			continue
		}

		// read the manifest and the allowed verifying keys digests
		circuitID := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), maxNbProofs))
		setupPath := cfg.PathForSetup(string(circuitID))
		manifest, err := circuits.ReadSetupManifest(filepath.Join(setupPath, config.ManifestFileName))
		if err != nil {
			return nil, 0, fmt.Errorf("could not read the manifest for circuit %v: %w", circuitID, err)
		}
		allowedVkForAggregation, err := manifest.GetStringArray("allowedVkForAggregationDigests")
		if err != nil {
			return nil, 0, fmt.Errorf("could not read the allowedVkForAggregationDigests: %w", err)
		}

		// This reject condition may take longer
		if !doesBw6CircuitSupportVKeys(allowedVkForAggregation, cf.ProofClaims) {
			logrus.Infof("skipping setup with %v proofs because it does not support the required verifying keys", maxNbProofs)
			continue
		}

		if maxNbProofs <= bestSize {
			bestSize = maxNbProofs
			bestAllowedVkForAggregation = allowedVkForAggregation
		}
	}

	if bestSize == math.MaxInt {
		return nil, 0, fmt.Errorf(
			"could not find a setup large enough for %v proofs: the biggest available size is %v",
			numProofClaims, biggestAvailable,
		)
	}

	logrus.Infof("reading the BW6 setup for %v proofs", bestSize)
	c := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), bestSize))
	setup, err := circuits.LoadSetup(cfg, c)
	if err != nil {
		return nil, 0, fmt.Errorf("could not load the setup for circuit %v: %w", c, err)
	}

	// Now, that we have selected "the best" setup to use to aggregate all the
	// proofs. We need to assign a circuit ID to all the proofClaims. We could
	// not do it before because the "ordering" of the verifying keys can be
	// circuit dependent. So, we needed to pick the circuit first.

	assignCircuitIDToProofClaims(bestAllowedVkForAggregation, cf.ProofClaims)

	// Although the public input is restrained to fit on the BN254 scalar field,
	// the BW6 field is larger. This allows us to represent the public input as a
	// single field element.

	var (
		piBW6 frBW6.Element
	)
	if _, err = piBW6.SetString(publicInput); err != nil {
		return nil, 0, fmt.Errorf("could not parse the public input: %w", err)
	}

	// set pi proof info
	piInfo := aggregation.PiInfo{
		Proof:         piProof,
		PublicWitness: piPublicWitness,
		ActualIndexes: pi_interconnection.InnerCircuitTypesToIndexes(&cfg.PublicInputInterconnection, cf.InnerCircuitTypes),
	}

	logrus.Infof("running the BW6 prover")
	proofBW6, err := aggregation.MakeProof(&setup, bestSize, cf.ProofClaims, piInfo, piBW6)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create BW6 proof: %w", err)
	}
	return proofBW6, bestSetupPos, nil
}

func makeBn254Proof(
	cfg *config.Config,
	circuitID int,
	proofBw6 plonk.Proof,
	publicInput string,
) (proof plonk.Proof, err error) {

	logrus.Infof("reading the BN254 setup from disk...")

	setup, err := circuits.LoadSetup(cfg, circuits.EmulationCircuitID)
	if err != nil {
		return nil, fmt.Errorf("could not read the BN254 setup: %w", err)
	}

	logrus.Infof("running the prover for the BN254 circuit...")

	var piBn254 frBn254.Element
	_, err = piBn254.SetString(publicInput)
	if err != nil {
		return nil, fmt.Errorf("could not parse the public input: %w", err)
	}

	logrus.Infof("running the Bn254 prover")

	proofBn254, err := emulation.MakeProof(&setup, circuitID, proofBw6, piBn254)
	if err != nil {
		return nil, fmt.Errorf("(for Bn254) gnark's plonk Prover failed with error: %w", err)
	}
	return proofBn254, nil

}

// This function is used to detect if a a BW6 circuit is compatible with a list
// proof's verifier keys. Namely, it takes the list of supported keys, parse them
// as hex-bytes-32 and check that all the proof claims verifier keys are included
// in the list of supported verifier keys.
func doesBw6CircuitSupportVKeys(supportedVkeys []string, proofClaims []aggregation.ProofClaimAssignment) bool {
	// Parse the required verifying keys into a list of bytes32
	suppBytes32 := make([]types.FullBytes32, len(supportedVkeys))
	for i := range suppBytes32 {
		suppBytes32[i] = types.FullBytes32FromHex(supportedVkeys[i])
	}

	// The list are not expected to be very big therefore, we anticipate that
	// relying on a hashmap is likely not worth the effort.
	for k := range proofClaims {
		found := false
		requiredVKey := proofClaims[k].VerifyingKeyShasum
		for i := range suppBytes32 {
			found = found || (suppBytes32[i] == requiredVKey)
		}
		if !found {
			return false
		}
	}

	return true
}

// This function assigns circuits ID to the input proof claims based on the list
// of verifying keys that are supported by the circuit. This function mutates the
// proofClaims parameter. It will panic if one of the claim's verifying key is
// missing from the supportedVkeys. Therefore, this function should only be
// called after `doesBW6CircuitSupportsVKeys` has been called and has returned
// true.
func assignCircuitIDToProofClaims(supportedVkeys []string, proofClaims []aggregation.ProofClaimAssignment) {

	// Parse the required verifying keys into a list of bytes32
	suppBytes32 := make([]types.FullBytes32, len(supportedVkeys))
	for i := range suppBytes32 {
		suppBytes32[i] = types.FullBytes32FromHex(supportedVkeys[i])
	}

	// The list are not expected to be very big therefore, we anticipate that
	// relying on a hashmap is likely not worth the effort.
	for k := range proofClaims {
		found := false
		for i := range suppBytes32 {
			isThisOne := suppBytes32[i] == proofClaims[k].VerifyingKeyShasum
			if isThisOne {
				proofClaims[k].CircuitID = i
				found = true
				break
			}
		}
		if !found {
			// Here, we panic because we were supposed to have run the above
			// `doesBw6CircuitSupportVKeys` before calling the current function.
			utils.Panic(
				"proof %v requires vkey %v, which was not found in the list %v",
				k, proofClaims[k].VerifyingKeyShasum.Hex(), supportedVkeys,
			)
		}

	}
}
