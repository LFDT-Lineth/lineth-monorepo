package aggregation

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"

	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
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

	proofBW6, setupPos, err := makeBw6Proof(cfg, cf, piProof, piPublicWitness, publicInput)
	if err != nil {
		return "", fmt.Errorf("error when running the BW6 proof: %w", err)
	}

	proofBn254, err := makeBn254Proof(cfg, setupPos, proofBW6, publicInput)
	if err != nil {
		return "", fmt.Errorf("error when running the Bn254 proof (aggregation setupPos=%v): %w", setupPos, err)
	}

	return circuits.SerializeProofSolidityBn254(proofBn254), nil
}

func (cf CollectedFields) AggregationPublicInput(cfg *config.Config) public_input.Aggregation {
	return public_input.Aggregation{
		FinalShnarf:                             cf.FinalShnarf,
		ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
		ParentStateRootHash:                     cf.ParentStateRootHashContract.Hex(),
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
		BaseFee:                                 uint64(cfg.Layer2.BaseFee),
		CoinBase:                                types.EthAddress(cfg.Layer2.CoinBase),
		L2MessageServiceAddr:                    types.EthAddress(cfg.Layer2.MsgSvcContract),
		IsAllowedCircuitID:                      uint64(cfg.Aggregation.IsAllowedCircuitID),
	}
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

	c, err := pi_interconnection.Compile(cfg.PublicInputInterconnection, keccak.WizardCompilationParameters())
	if err != nil {
		return nil, nil, fmt.Errorf("could not create the public-input circuit: %w", err)
	}

	assignment, err := c.Assign(pi_interconnection.Request{
		DataAvailabilities: cf.DecompressionPI,
		Executions:         cf.ExecutionPI,
		Aggregation:        cf.AggregationPublicInput(cfg),
	}, cfg.BlobDecompressionDictStore(string(circuits.DataAvailabilityV2CircuitID)))
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
) (proof plonk.Proof, setupPos int, err error) {

	// This determines which is the best circuit to use for aggregation, we
	// take the smallest circuit that has enough capacity.

	var (
		numProofClaims              = len(cf.ProofClaims)
		biggestAvailable            = 0
		bestSize                    = math.MaxInt
		bestAllowedVkForAggregation []string
		errs                        []error
		allAllowedVKs               = make(map[string]bool) // union of allowed VKs across all setups
	)

	// Log the verifying keys present in sub-proofs for diagnostics
	subProofVKs := collectSubProofVKSummary(cf.ProofClaims)
	logrus.Debugf("aggregation: %v sub-proof claims with %v distinct verifying key(s)", numProofClaims, len(subProofVKs))
	for vk, indices := range subProofVKs {
		logrus.Debugf("  VK %v used by %v sub-proof(s):", vk, len(indices))
		for _, idx := range indices {
			src := "<unknown>"
			if idx < len(cf.ProofClaimSources) {
				src = cf.ProofClaimSources[idx]
			}
			logrus.Debugf("    [%v] %v", idx, src)
		}
	}

	// first we discover available setups
	for setupIdx, maxNbProofs := range cfg.Aggregation.NumProofs {
		biggestAvailable = max(biggestAvailable, maxNbProofs)

		// That's the quickest reject condition we have
		if maxNbProofs < numProofClaims {
			logrus.Debugf("skipping setup for aggregation-%v proof circuit since %v proof claims is greater than what this setup can handle", maxNbProofs, numProofClaims)
			continue
		}

		// read the manifest and the allowed verifying keys digests
		circuitIDStr := circuits.CircuitID(fmt.Sprintf("%s-%d", string(circuits.AggregationCircuitID), maxNbProofs))
		setupPath := cfg.PathForSetup(string(circuitIDStr))
		manifest, err := circuits.ReadSetupManifest(filepath.Join(setupPath, config.ManifestFileName))
		if err != nil {
			return nil, 0, fmt.Errorf("could not read the manifest for circuit %v: %w", circuitIDStr, err)
		}
		allowedVkForAggregation, err := manifest.GetStringArray("allowedVkForAggregationDigests")
		if err != nil {
			return nil, 0, fmt.Errorf("could not read the allowedVkForAggregationDigests: %w", err)
		}
		// Try to read circuit names (may not exist in older manifests)
		allowedVkCircuitNames, _ := manifest.GetStringArray("allowedVkForAggregationCircuitNames")

		for _, vk := range allowedVkForAggregation {
			allAllowedVKs[vk] = true
		}

		// This reject condition may take longer
		if err = doesBw6CircuitSupportVKeys(allowedVkForAggregation, cf.ProofClaims, cf.ProofClaimSources); err != nil {
			logrus.Warnf("skipped setup aggregation-%v: VK mismatch.", maxNbProofs)
			logAllowedVKs(maxNbProofs, allowedVkForAggregation, allowedVkCircuitNames)
			errs = append(errs, fmt.Errorf("skipped setup for aggregation-%v proof circuit: %w", maxNbProofs, err))
			continue
		}

		if maxNbProofs <= bestSize {
			setupPos = setupIdx
			bestSize = maxNbProofs
			bestAllowedVkForAggregation = allowedVkForAggregation
		}
	}

	if bestSize == math.MaxInt {
		// Print actionable summary: list the files that need to be regenerated
		logMismatchedFiles(cf.ProofClaims, cf.ProofClaimSources, allAllowedVKs)

		err := fmt.Errorf(
			"could not find a setup large enough for %v proofs: the biggest available size is %v",
			numProofClaims, biggestAvailable,
		)

		errs = append(errs, err)
		return nil, 0, errors.Join(errs...)
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

	// Pre-flight check: validate that all assigned circuit IDs are allowed by
	// the IsAllowedCircuitID bitmask BEFORE running the expensive BW6 prover.
	// Without this check, disallowed circuits would only be caught inside the
	// circuit constraints, producing a cryptic "assertIsEqual 0==1" error.
	if err := validateCircuitIDsAllowed(cfg.Aggregation.IsAllowedCircuitID, cf.ProofClaims, cf.ProofClaimSources, bestAllowedVkForAggregation); err != nil {
		return nil, 0, err
	}

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

	logrus.Infof("running the BW6 prover with aggregation setupPos=%v (aggregation-%v)", setupPos, bestSize)
	proofBW6, err := aggregation.MakeProof(&setup, bestSize, cf.ProofClaims, piInfo, piBW6)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create BW6 proof: %w", err)
	}
	return proofBW6, setupPos, nil
}

func makeBn254Proof(
	cfg *config.Config,
	setupPos int,
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

	logrus.Infof("running the BN254 emulation prover with aggregation setupPos=%v", setupPos)

	proofBn254, err := emulation.MakeProof(&setup, setupPos, proofBw6, piBn254)
	if err != nil {
		return nil, fmt.Errorf("(for Bn254) gnark's plonk Prover failed with error: %w", err)
	}
	return proofBn254, nil

}

// logAllowedVKs logs the allowed VKs for a given aggregation setup, with
// circuit names if available (from the manifest's allowedVkForAggregationCircuitNames).
func logAllowedVKs(maxNbProofs int, vks []string, circuitNames []string) {
	logrus.Warnf("  Allowed VKs in aggregation-%v setup manifest:", maxNbProofs)
	for i, vk := range vks {
		name := fmt.Sprintf("circuit-id-%d", i)
		if i < len(circuitNames) {
			name = circuitNames[i]
		}
		logrus.Warnf("    [%d] %-25s -> %s", i, name, vk)
	}
}

// collectSubProofVKSummary returns a map from VK hex string to the list of
// sub-proof indices using that VK. This is used for diagnostic logging.
func collectSubProofVKSummary(proofClaims []aggregation.ProofClaimAssignment) map[string][]int {
	vkToIndices := make(map[string][]int)
	for i := range proofClaims {
		vk := proofClaims[i].VerifyingKeyShasum.Hex()
		vkToIndices[vk] = append(vkToIndices[vk], i)
	}
	return vkToIndices
}

// logMismatchedFiles prints an actionable summary of which sub-proof response
// files need to be regenerated. It checks each sub-proof's VK against the union
// of all allowed VKs across all aggregation setups that were tried, and lists
// only the files whose VK is not in any setup's allowed list.
func logMismatchedFiles(proofClaims []aggregation.ProofClaimAssignment, sources []string, allAllowedVKs map[string]bool) {
	type mismatchEntry struct {
		index int
		file  string
		vk    string
	}

	var mismatched []mismatchEntry
	for i, claim := range proofClaims {
		vk := claim.VerifyingKeyShasum.Hex()
		if !allAllowedVKs[vk] {
			src := "<unknown>"
			if i < len(sources) {
				src = sources[i]
			}
			mismatched = append(mismatched, mismatchEntry{index: i, file: src, vk: vk})
		}
	}

	if len(mismatched) == 0 {
		return
	}

	logrus.Errorf("=== AGGREGATION FAILED: Sub-proof VK mismatch ===")
	logrus.Errorf("%d/%d sub-proof response files contain a verifyingKeyShaSum not recognized by any aggregation setup.", len(mismatched), len(proofClaims))
	logrus.Errorf("These sub-proof files were generated with an incompatible prover binary and must be regenerated:")
	for _, m := range mismatched {
		logrus.Errorf("  [%d] %s", m.index, m.file)
		logrus.Errorf("       verifyingKeyShaSum in file: %s", m.vk)
	}
	logrus.Errorf("=================================================")
}

// This function is used to detect if a a BW6 circuit is compatible with a list
// proof's verifier keys. Namely, it takes the list of supported keys, parse them
// as hex-bytes-32 and check that all the proof claims verifier keys are included
// in the list of supported verifier keys.
func doesBw6CircuitSupportVKeys(supportedVkeys []string, proofClaims []aggregation.ProofClaimAssignment, sources []string) error {
	// Parse the required verifying keys into a list of bytes32
	suppBytes32 := make([]types.FullBytes32, len(supportedVkeys))
	for i := range suppBytes32 {
		suppBytes32[i] = types.FullBytes32FromHex(supportedVkeys[i])
	}

	// The list are not expected to be very big therefore, we anticipate that
	// relying on a hashmap is likely not worth the effort.
	var unSupportedIdxs []int
	for k := range proofClaims {
		found := false
		requiredVKey := proofClaims[k].VerifyingKeyShasum
		for i := range suppBytes32 {
			found = found || (suppBytes32[i] == requiredVKey)
		}
		if !found {
			unSupportedIdxs = append(unSupportedIdxs, k)
		}
	}

	if len(unSupportedIdxs) > 0 {
		// Build a detailed mismatch report showing which VKs are unsupported
		unsupportedVKs := make(map[string][]int)
		for _, idx := range unSupportedIdxs {
			vk := proofClaims[idx].VerifyingKeyShasum.Hex()
			unsupportedVKs[vk] = append(unsupportedVKs[vk], idx)
		}
		detail := fmt.Sprintf(
			"BW6 circuit does not support the verifying keys for %v/%v sub-proofs.\n"+
				"  Unsupported sub-proof VKs (not in this setup's allowedVkForAggregationDigests):\n",
			len(unSupportedIdxs), len(proofClaims),
		)
		for vk, indices := range unsupportedVKs {
			detail += fmt.Sprintf("    VK %v -> sub-proof indices:\n", vk)
			for _, idx := range indices {
				src := "<unknown>"
				if idx < len(sources) {
					src = sources[idx]
				}
				detail += fmt.Sprintf("      [%d] %s\n", idx, src)
			}
		}
		detail += "  Allowed VKs in this setup:\n"
		for i, vk := range supportedVkeys {
			detail += fmt.Sprintf("    [%d] %s\n", i, vk)
		}
		detail += "  The sub-proofs were likely generated with an incompatible setup version. " +
			"Re-generate the sub-proofs using the same setup version as the aggregation circuit, or re-generate the aggregation setup to match the sub-proof VKs."
		return fmt.Errorf("%s", detail)
	}

	return nil
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

// validateCircuitIDsAllowed checks that all proof claims have circuit IDs that
// are allowed by the IsAllowedCircuitID bitmask. This is a pre-flight check
// that runs before the expensive BW6 prover to give a clear error message
// instead of a cryptic constraint failure inside the circuit.
func validateCircuitIDsAllowed(bitmask uint64, proofClaims []aggregation.ProofClaimAssignment, sources []string, vkList []string) error {
	type disallowedEntry struct {
		index     int
		file      string
		circuitID int
		name      string
	}

	var disallowed []disallowedEntry
	for i, claim := range proofClaims {
		cid := uint(claim.CircuitID)
		if !circuits.IsCircuitAllowed(bitmask, cid) {
			src := "<unknown>"
			if i < len(sources) {
				src = sources[i]
			}
			name := circuits.CircuitNameByID(cid)
			disallowed = append(disallowed, disallowedEntry{
				index: i, file: src, circuitID: claim.CircuitID, name: name,
			})
		}
	}

	if len(disallowed) == 0 {
		return nil
	}

	logrus.Errorf("=== AGGREGATION FAILED: Disallowed circuit ID ===")
	logrus.Errorf("%d/%d sub-proof(s) use a circuit ID that is not allowed by is_allowed_circuit_id=%d (binary: 0b%b).",
		len(disallowed), len(proofClaims), bitmask, bitmask)
	logrus.Errorf("Allowed circuits: %v", circuits.GetAllowedCircuitNames(bitmask))
	logrus.Errorf("Disallowed sub-proofs:")
	for _, d := range disallowed {
		logrus.Errorf("  [%d] %s -> circuit ID %d (%s)", d.index, d.file, d.circuitID, d.name)
	}
	logrus.Errorf("To fix: either regenerate the sub-proofs using an allowed circuit, or update is_allowed_circuit_id in your config to allow circuit ID %d (%s).",
		disallowed[0].circuitID, disallowed[0].name)
	logrus.Errorf("=================================================")

	return fmt.Errorf(
		"aggregation rejected: %d/%d sub-proof(s) use disallowed circuit ID %d (%s); is_allowed_circuit_id=%d only allows %v",
		len(disallowed), len(proofClaims),
		disallowed[0].circuitID, disallowed[0].name,
		bitmask, circuits.GetAllowedCircuitNames(bitmask),
	)
}
