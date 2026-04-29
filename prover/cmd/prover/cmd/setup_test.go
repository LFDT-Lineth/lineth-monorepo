package cmd

import (
	"context"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	frBls "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	frBw6 "github.com/consensys/gnark-crypto/ecc/bw6-761/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/gnark/test"
	backendagg "github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits/dummy"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAggregationWithMultipleVKs tests that the aggregation circuit correctly handles
// proofs from different inner circuits (with different verifying keys, specially different types of invalidities).
// This validates:
// - VK digest to CircuitID mapping via AssignCircuitIDToProofClaims
// - Aggregation circuit accepts proofs verified against different VKs
// - Setup flow produces valid setups for multiple circuit types
//
// Uses dummy circuits for speed - does not test business logic.
func TestAggregationWithMultipleVKs(t *testing.T) {

	const (
		nbExecutions     = 2
		nbDecompressions = 2
		nbInvalidities   = 2 // Two different invalidity VKs (nonce-balance + precompile-logs)
	)

	logrus.Info("=== Starting Aggregation Test with Multiple VKs ===")

	// Step 1: Setup SRS provider and dummy circuits using the SAME flow as setup.go
	// Use the real SRS store (same as production) so different circuit sizes share the same SRS base
	logrus.Info("Step 1: Setting up SRS provider and dummy circuits via setup flow")

	// Use real SRS from prover-assets (same as production setup)
	// This ensures all circuits share the same SRS base, allowing different MockCircuitIDs
	srsProvider, err := circuits.NewSRSStore("../../../prover-assets/kzgsrs")
	require.NoError(t, err, "Failed to load SRS store - make sure prover-assets/kzgsrs exists")

	ctx := context.Background()

	// Define allowed inputs EXACTLY as they would appear in production config
	// This is the same format as cfg.Aggregation.AllowedInputs in config.yaml
	// The ORDER here determines the CircuitID used in aggregation (via VK checksum lookup)
	allowedInputs := []string{
		"execution-dummy",
		"data-availability-dummy",
		"invalidity-nonce-balance-dummy",
		"invalidity-precompile-logs-dummy",
	}

	// Use the same code path as production (collectVerifyingKeys) to setup dummy circuits
	// The only difference is we keep the full Setup (with proving key) instead of just VK
	innerSetups := make([]circuits.Setup, len(allowedInputs))
	mockCircuitIDs := make([]circuits.MockCircuitID, len(allowedInputs))
	for i, input := range allowedInputs {
		curveID, mockID, err := getDummyCircuitParams(input)
		require.NoError(t, err, "getDummyCircuitParams failed for %s", input)

		builder := dummy.NewBuilder(mockID, curveID.ScalarField())
		ccs, err := builder.Compile()
		require.NoError(t, err, "Compile failed for %s", input)

		setup, err := circuits.MakeSetup(ctx, circuits.CircuitID(input), ccs, srsProvider, nil)
		require.NoError(t, err, "MakeSetup failed for %s", input)

		innerSetups[i] = setup
		mockCircuitIDs[i] = mockID
	}

	// Build VK digest list (same format as production's manifest)
	// This is passed to AssignCircuitIDToProofClaims to auto-assign CircuitIDs
	vkDigests := make([]string, len(innerSetups))
	vkeys := make([]plonk.VerifyingKey, len(innerSetups))
	for i, setup := range innerSetups {
		vkDigests[i] = setup.VerifyingKeyDigest()
		vkeys[i] = setup.VerifyingKey
		logrus.Infof("  Setup complete for %s (mockID=%d, vkDigest=%s)",
			allowedInputs[i], mockCircuitIDs[i], vkDigests[i][:16]+"...")
	}

	// CRITICAL: Verify each circuit type has a UNIQUE VK digest
	// This ensures production setup generates distinct VKs for each invalidity subcircuit
	seenVKDigests := make(map[string]string) // vkDigest -> circuitName
	for i, vkDigest := range vkDigests {
		if existingCircuit, exists := seenVKDigests[vkDigest]; exists {
			t.Fatalf("VK digest collision: %s and %s have the same VK digest %s",
				existingCircuit, allowedInputs[i], vkDigest[:32]+"...")
		}
		seenVKDigests[vkDigest] = allowedInputs[i]
	}
	logrus.Infof("  Verified: all %d circuit types have unique VK digests", len(allowedInputs))

	// Specifically verify the two invalidity subcircuits have different VKs
	nonceBalanceIdx := circuitNameToIdx(allowedInputs, "invalidity-nonce-balance-dummy")
	precompileLogsIdx := circuitNameToIdx(allowedInputs, "invalidity-precompile-logs-dummy")
	require.NotEqual(t, vkDigests[nonceBalanceIdx], vkDigests[precompileLogsIdx],
		"invalidity-nonce-balance-dummy and invalidity-precompile-logs-dummy must have different VK digests")

	// Build circuit name → setup mapping (for proof generation)
	// In production, each prover (execution/decompression/invalidity) uses its own setup
	circuitNameToSetup := make(map[string]int)
	for i, name := range allowedInputs {
		circuitNameToSetup[name] = i
	}

	// Step 2: Setup PI Interconnection circuit
	logrus.Info("Step 2: Compiling PI Interconnection circuit")
	piConfig := config.PublicInput{
		MaxNbDataAvailability: nbDecompressions,
		MaxNbExecution:        nbExecutions,
		MaxNbInvalidity:       nbInvalidities,
		ExecutionMaxNbMsg:     2,
		L2MsgMerkleDepth:      5,
		L2MsgMaxNbMerkle:      1,
		MockKeccakWizard:      true,
	}

	piCircuit := pi_interconnection.DummyCircuit{
		ExecutionPublicInput:     make([]frontend.Variable, piConfig.MaxNbExecution),
		ExecutionFPI:             make([]frontend.Variable, piConfig.MaxNbExecution),
		DecompressionPublicInput: make([]frontend.Variable, piConfig.MaxNbDataAvailability),
		DecompressionFPI:         make([]frontend.Variable, piConfig.MaxNbDataAvailability),
		InvalidityPublicInput:    make([]frontend.Variable, piConfig.MaxNbInvalidity),
		InvalidityFPI:            make([]frontend.Variable, piConfig.MaxNbInvalidity),
	}

	piCs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &piCircuit)
	require.NoError(t, err)

	piSetup, err := circuits.MakeSetup(context.TODO(), circuits.PublicInputInterconnectionCircuitID, piCs, srsProvider, nil)
	require.NoError(t, err)

	// Define which VK each invalidity proof uses (by circuit name)
	// This tests that different invalidity subcircuits produce different VKs
	invaliditySubcircuits := []string{
		"invalidity-nonce-balance-dummy",   // First invalidity proof
		"invalidity-precompile-logs-dummy", // Second invalidity proof
	}

	// Step 3: Generate dummy proofs for all circuits
	// Each proof uses its corresponding circuit setup (looked up by name, not hardcoded index)
	logrus.Info("Step 3: Generating dummy proofs for all circuit types")

	totalProofs := nbExecutions + nbDecompressions + nbInvalidities
	innerProofClaims := make([]aggregation.ProofClaimAssignment, totalProofs)
	innerPIElements := make([]frBls.Element, totalProofs)

	// Define circuit types order: executions, decompressions, invalidities
	circuitTypes := make([]pi_interconnection.InnerCircuitType, totalProofs)
	for i := 0; i < nbExecutions; i++ {
		circuitTypes[i] = pi_interconnection.Execution
	}
	for i := 0; i < nbDecompressions; i++ {
		circuitTypes[nbExecutions+i] = pi_interconnection.Decompression
	}
	for i := 0; i < nbInvalidities; i++ {
		circuitTypes[nbExecutions+nbDecompressions+i] = pi_interconnection.Invalidity
	}

	// Map each proof to its circuit NAME (production-like: each prover knows its circuit)
	proofCircuitNames := make([]string, totalProofs)
	invalIdx := 0
	for i := 0; i < totalProofs; i++ {
		switch circuitTypes[i] {
		case pi_interconnection.Execution:
			proofCircuitNames[i] = "execution-dummy"
		case pi_interconnection.Decompression:
			proofCircuitNames[i] = "blob-decompression-dummy"
		case pi_interconnection.Invalidity:
			// Use the correct invalidity subcircuit for this proof
			proofCircuitNames[i] = invaliditySubcircuits[invalIdx]
			invalIdx++
		}
	}

	// Generate proofs - each proof includes the VK checksum (production-like)
	for i := 0; i < totalProofs; i++ {
		circuitName := proofCircuitNames[i]
		setupIdx := circuitNameToSetup[circuitName]
		mockID := mockCircuitIDs[setupIdx]

		_, err = innerPIElements[i].SetRandom()
		require.NoError(t, err)

		// Use the correct MockCircuitID for the dummy assignment
		assignment := dummy.Assign(mockID, innerPIElements[i])

		proof, err := circuits.ProveCheck(
			&innerSetups[setupIdx], assignment,
			emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
			emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		)
		require.NoError(t, err)

		// Set VerifyingKeyShasum from the setup's VK digest (production-like)
		// CircuitID will be auto-determined below via VK checksum lookup
		vkDigest := vkDigests[setupIdx]
		innerProofClaims[i] = aggregation.ProofClaimAssignment{
			Proof:              proof,
			PublicInput:        innerPIElements[i],
			VerifyingKeyShasum: types.FullBytes32FromHex(vkDigest),
			// CircuitID is NOT set here - will be auto-assigned below
		}

		logrus.Infof("  Generated proof %d/%d: type=%v, circuit=%s, mockID=%d",
			i+1, totalProofs, circuitTypes[i], circuitName, mockID)
	}

	// Auto-assign CircuitID using the EXACT production function from prove.go
	backendagg.AssignCircuitIDToProofClaims(vkDigests, innerProofClaims)
	logrus.Info("  CircuitIDs auto-assigned via AssignCircuitIDToProofClaims (production function)")

	// CRITICAL: Verify each proof got the correct CircuitID based on its VK
	// CircuitID should match the index in allowedInputs (which is the order in vkDigests)
	for i, claim := range innerProofClaims {
		circuitName := proofCircuitNames[i]
		expectedCircuitID := circuitNameToSetup[circuitName]
		require.Equal(t, expectedCircuitID, claim.CircuitID,
			"Proof %d (%s) has wrong CircuitID: got %d, expected %d",
			i, circuitName, claim.CircuitID, expectedCircuitID)
	}
	logrus.Info("  Verified: all proofs have correct CircuitIDs matching their VK digests")

	// Verify the two invalidity proofs have DIFFERENT CircuitIDs
	inval1CircuitID := innerProofClaims[nbExecutions+nbDecompressions].CircuitID   // First invalidity
	inval2CircuitID := innerProofClaims[nbExecutions+nbDecompressions+1].CircuitID // Second invalidity
	require.NotEqual(t, inval1CircuitID, inval2CircuitID,
		"The two invalidity proofs must have different CircuitIDs (got %d and %d)", inval1CircuitID, inval2CircuitID)
	logrus.Infof("  Verified: invalidity proofs have distinct CircuitIDs (%d and %d)", inval1CircuitID, inval2CircuitID)

	// Step 4: Generate PI Interconnection proof
	logrus.Info("Step 4: Generating PI Interconnection proof")

	// Partition public inputs by circuit type
	innerPiPartition := utils.RightPad(utils.Partition(innerPIElements, circuitTypes), 3)
	execPIElements := utils.RightPad(innerPiPartition[pi_interconnection.Execution], piConfig.MaxNbExecution)
	decompPIElements := utils.RightPad(innerPiPartition[pi_interconnection.Decompression], piConfig.MaxNbDataAvailability)
	invalPIElements := utils.RightPad(innerPiPartition[pi_interconnection.Invalidity], piConfig.MaxNbInvalidity)

	// Aggregation public input
	aggregationPIBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	var aggregationPI frBw6.Element
	aggregationPI.SetBytes(aggregationPIBytes)

	// Set IsAllowedCircuitID bitmask to allow all circuit IDs used in the test
	isAllowedMask := (1 << len(allowedInputs)) - 1

	piAssignment := pi_interconnection.DummyCircuit{
		AggregationPublicInput:   [2]frontend.Variable{aggregationPIBytes[:16], aggregationPIBytes[16:]},
		ExecutionPublicInput:     utils.ToVariableSlice(execPIElements),
		DecompressionPublicInput: utils.ToVariableSlice(decompPIElements),
		DecompressionFPI:         utils.ToVariableSlice(pow5(decompPIElements)),
		ExecutionFPI:             utils.ToVariableSlice(pow5(execPIElements)),
		NbExecution:              len(innerPiPartition[pi_interconnection.Execution]),
		NbDecompression:          len(innerPiPartition[pi_interconnection.Decompression]),
		NbInvalidity:             len(innerPiPartition[pi_interconnection.Invalidity]),
		InvalidityPublicInput:    utils.ToVariableSlice(invalPIElements),
		InvalidityFPI:            utils.ToVariableSlice(pow5(invalPIElements)),
		IsAllowedCircuitID:       isAllowedMask,
	}

	piW, err := frontend.NewWitness(&piAssignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	piProof, err := circuits.ProveCheck(
		&piSetup, &piAssignment,
		emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
	)
	require.NoError(t, err)

	piPubW, err := piW.Public()
	require.NoError(t, err)

	piInfo := aggregation.PiInfo{
		Proof:         piProof,
		PublicWitness: piPubW,
		ActualIndexes: pi_interconnection.InnerCircuitTypesToIndexes(&piConfig, circuitTypes),
	}

	// Step 5: Build and test the aggregation circuit
	logrus.Info("Step 5: Building and testing aggregation circuit")

	aggrCircuit, err := aggregation.AllocateCircuit(totalProofs, piSetup, vkeys)
	require.NoError(t, err)

	aggrAssignment, err := aggregation.AssignAggregationCircuit(totalProofs, innerProofClaims, piInfo, aggregationPI)
	require.NoError(t, err)

	// Verify the circuit is satisfied
	err = test.IsSolved(aggrCircuit, aggrAssignment, ecc.BW6_761.ScalarField())
	assert.NoError(t, err, "Aggregation circuit should be satisfied")

	logrus.Info("=== Aggregation Test with Multiple VKs PASSED ===")
	logrus.Infof("  Aggregated %d proofs with %d different VKs", totalProofs, len(allowedInputs))
}

// pow5 computes x^5 for each element (used for dummy FPI computation)
func pow5(s []frBls.Element) []frBls.Element {
	res := make([]frBls.Element, len(s))
	for i := range s {
		res[i].
			Mul(&s[i], &s[i]).
			Mul(&res[i], &res[i]).
			Mul(&res[i], &s[i])
	}
	return res
}

// circuitNameToIdx returns the index of a circuit name in the allowedInputs slice
func circuitNameToIdx(allowedInputs []string, name string) int {
	for i, n := range allowedInputs {
		if n == name {
			return i
		}
	}
	return -1
}
