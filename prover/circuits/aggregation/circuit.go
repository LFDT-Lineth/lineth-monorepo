// The bw6circuit package provides an implementation of the BW6 proof aggregation
// circuit. This circuits can aggregate several PLONK proofs.
package aggregation

import (
	"errors"
	"fmt"
	"slices"

	frBn254 "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/native/sw_bls12377"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/math/emulated"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
)

// shorthand for the emulated types as this can get verbose very quickly with
// generics. `em` stands for emulated
type (
	emFr       = sw_bls12377.ScalarField
	emG1       = sw_bls12377.G1Affine
	emG2       = sw_bls12377.G2Affine
	emGT       = sw_bls12377.GT
	emProof    = emPlonk.Proof[emFr, emG1, emG2]
	emVkey     = emPlonk.VerifyingKey[emFr, emG1, emG2]
	emCircVKey = emPlonk.CircuitVerifyingKey[emFr, emG1]
	emWitness  = emPlonk.Witness[emFr]
	// emBaseVkey = emPlonk.BaseVerifyingKey[emFr, emG1, emG2]
)

// The Circuit is used to aggregate multiple execution proofs and
// aggregation proofs together.
type Circuit struct {
	// The list of claims to be provided to the circuit.
	ProofClaims []proofClaim `gnark:",secret"`

	// List of ALL available payload verifying keys (from GlobalCircuitIDMapping, IDs 0-5).
	// This is treated as a constant by the circuit - all payload verifying keys are always
	// included regardless of configuration. The IsAllowedCircuitID bitmask from the
	// PI circuit's public input controls which circuits are actually allowed at runtime.
	//
	// The bitmask uses LSb to MSb encoding where each bit position corresponds to a
	// circuit ID from GlobalCircuitIDMapping:
	//   - Bit 0 (LSb): execution-dummy (ID 0)
	//   - Bit 1: data-availability-dummy (ID 1)
	//   - Bit 2: execution (ID 2)
	//   - Bit 3: execution-large (ID 3)
	//   - Bit 4: execution-limitless (ID 4)
	//   - Bit 5: data-availability-v2 (ID 5)
	//
	// Examples:
	//   Mainnet (IsAllowedCircuitID = 60 = 0b111100):
	//     - Bits 0-1 are 0 → dummy circuits (IDs 0-1) are DISALLOWED
	//     - Bits 2-5 are 1 → production circuits (IDs 2-5) are ALLOWED
	//
	//   Testnet (IsAllowedCircuitID = 63 = 0b111111):
	//     - All bits 0-5 are 1 → all circuits including dummies are ALLOWED
	verifyingKeys []emVkey `gnark:"-"`

	publicInputVerifyingKey        emVkey              `gnark:"-"`
	PublicInputProof               emProof             `gnark:",secret"`
	PublicInputWitness             emWitness           `gnark:",secret"` // ordered for the PI circuit
	PublicInputWitnessClaimIndexes []frontend.Variable `gnark:",secret"`

	// general public input
	PublicInput frontend.Variable `gnark:",public"`
}

func (c *Circuit) Define(api frontend.API) error {
	// match the public input with the public input of the PI circuit
	field, err := emulated.NewField[emFr](api)
	if err != nil {
		return err
	}

	// match general PI input with that of the interconnection circuit
	piBits := api.ToBinary(c.PublicInput, frBn254.Bits)
	assertSlicesEqualZEXT(api, piBits[:16*8], field.ToBitsCanonical(&c.PublicInputWitness.Public[1]))
	assertSlicesEqualZEXT(api, piBits[16*8:], field.ToBitsCanonical(&c.PublicInputWitness.Public[0]))

	api.AssertIsDifferent(c.PublicInput, 0) // making sure at least one element of the PI circuit's public input is nonzero to justify using incomplete arithmetic

	// Prepare the list of all verifying keys including the PI circuit's VK
	vks := append(slices.Clone(c.verifyingKeys), c.publicInputVerifyingKey)
	piVkIndex := len(vks) - 1

	// Extract the IsAllowedCircuitID bitmask from the PI circuit's public inputs.
	// This is the LAST public input of the PI circuit and encodes which circuit IDs
	// are allowed in this proof.
	//
	// The mask is an emulated field element, so we use field.ToBitsCanonical() to
	// convert it to bits instead of api.ToBinary().
	mask := c.PublicInputWitness.Public[len(c.PublicInputWitness.Public)-1]
	maskBits := field.ToBitsCanonical(&mask)

	// Convert the bitmask to a boolean array where isCircuitAllowed[i] indicates
	// whether circuit ID i is allowed.
	//
	// We only need len(c.verifyingKeys) bits for the circuit allowlist (payload circuits).
	// The maskBits from ToBitsCanonical may have more bits than needed, so we truncate.
	// If maskBits has fewer bits (shouldn't happen), we pad with zeros.
	//
	// Example: If mask = 60 = 0b111100, then:
	//   isCircuitAllowed[0] = 0 (execution-dummy NOT allowed)
	//   isCircuitAllowed[1] = 0 (data-availability-dummy NOT allowed)
	//   isCircuitAllowed[2] = 1 (execution ALLOWED)
	//   isCircuitAllowed[3] = 1 (execution-large ALLOWED)
	//   isCircuitAllowed[4] = 1 (execution-limitless ALLOWED)
	//   isCircuitAllowed[5] = 1 (data-availability-v2 ALLOWED)
	isCircuitAllowed := make([]frontend.Variable, len(c.verifyingKeys))
	for i := range isCircuitAllowed {
		if i < len(maskBits) {
			isCircuitAllowed[i] = maskBits[i]
		} else {
			isCircuitAllowed[i] = 0
		}
	}

	// For each proof claim, verify that its circuit ID is allowed by the bitmask
	for i := range c.ProofClaims {
		// TODO @Tabaie is this necessary? can't think of an attack if this is removed
		api.AssertIsDifferent(c.ProofClaims[i].CircuitID, piVkIndex)

		// Check if the circuit ID of this proof claim is allowed.
		// We iterate through all possible circuit IDs and check if this claim's
		// circuit ID matches any of them. If it matches circuit ID j, we check
		// if isCircuitAllowed[j] is 1.
		//
		// Example: Suppose we have a proof claim with CircuitID = 4 (execution-limitless)
		// and mask = 60 = 0b111100:
		//   - When j = 4: isGoodPod = 1 (circuit ID matches)
		//                 isAllowed = isCircuitAllowed[4] = 1 (bit 4 is set)
		//   - For all other j: isGoodPod = 0, isAllowed remains unchanged
		//   - Final assertion: isAllowed must equal 1
		//
		// If the circuit was NOT allowed (e.g., CircuitID = 0 with mask = 60):
		//   - When j = 0: isGoodPod = 1, isAllowed = isCircuitAllowed[0] = 0
		//   - Final assertion would FAIL since isAllowed = 0 ≠ 1
		isAllowed := frontend.Variable(0)
		for j := range isCircuitAllowed {
			isGoodPod := api.IsZero(api.Sub(c.ProofClaims[i].CircuitID, j))
			isAllowed = api.Select(isGoodPod, isCircuitAllowed[j], isAllowed)
		}

		// Assert that the circuit ID is allowed (its bit in the bitmask is 1)
		api.AssertIsEqual(isAllowed, frontend.Variable(1))
	}

	// create a lookup table of actual public inputs
	actualPI := make([]logderivlookup.Table, (emFr{}).NbLimbs())
	for i := range actualPI {
		actualPI[i] = logderivlookup.New(api)
	}

	for _, claim := range c.ProofClaims {
		if len(claim.PublicInput.Public) != 1 {
			return errors.New("expected 1 public input per decompression/execution circuit")
		}
		pi := claim.PublicInput.Public[0]
		for i := range actualPI {
			actualPI[i].Insert(pi.Limbs[i])
		}
	}

	// The PI circuit's public inputs structure:
	// [0]: First half of aggregation public input (AggregationPublicInput[0])
	// [1]: Second half of aggregation public input (AggregationPublicInput[1])
	// [2..N+M+1]: Public inputs from execution and DA circuits (N+M total)
	// [N+M+2]: IsAllowedCircuitID bitmask
	//
	// PublicInputWitnessClaimIndexes has length N+M (from GetMaxNbCircuitsSum)
	// Therefore: len(Public) = len(PublicInputWitnessClaimIndexes) + 3
	if len(c.PublicInputWitnessClaimIndexes)+3 != len(c.PublicInputWitness.Public) {
		return errors.New("expected the number of public inputs to match the number of public input witness claim indexes (plus 3 for aggregation PI and IsAllowedCircuitID)")
	}

	// verify that every valid input to the PI circuit is accounted for
	for i, actualI := range c.PublicInputWitnessClaimIndexes {
		hubPI := &c.PublicInputWitness.Public[i+2]
		isNonZero := api.Sub(1, field.IsZero(hubPI)) // if a PI is zero, due to preimage resistance we can infer that the PI circuit is not using it
		for j := range actualPI {
			internal.AssertEqualIf(api, isNonZero, actualPI[j].Lookup(actualI)[0], hubPI.Limbs[j])
		}
	}

	claims := append(slices.Clone(c.ProofClaims), proofClaim{
		CircuitID:   piVkIndex,
		Proof:       c.PublicInputProof,
		PublicInput: c.PublicInputWitness,
	})

	// Verify the constraints the execution proofs
	if err = verifyClaimBatch(api, vks, claims); err != nil {
		return fmt.Errorf("processing execution proofs: %w", err)
	}

	return nil
}

// Instantiate a new Circuit from a list of verification keys and
// a maximal number of proofs. The function should only be called with the
// purpose of running `frontend.Compile` over it.
func AllocateCircuit(nbProofs int, pi circuits.Setup, verifyingKeys []plonk.VerifyingKey) (*Circuit, error) {

	var (
		err           error
		emVKeys       = make([]emVkey, len(verifyingKeys))
		csPlaceHolder = getPlaceHolderCS()
		proofClaims   = make([]proofClaim, nbProofs)
	)

	for i := range verifyingKeys {
		emVKeys[i], err = emPlonk.ValueOfVerifyingKey[emFr, emG1, emG2](verifyingKeys[i])
		if err != nil {
			return nil, fmt.Errorf("while converting the verifying key #%v (execution) into its emulated gnark version: %w", i, err)
		}
	}

	for i := range proofClaims {
		proofClaims[i] = allocatableClaimPlaceHolder(csPlaceHolder)
	}

	piVkEm, err := emPlonk.ValueOfVerifyingKey[emFr, emG1, emG2](pi.VerifyingKey)
	if err != nil {
		return nil, fmt.Errorf("while converting the PI interconnection verifying key into its emulated gnark version: %w", err)
	}

	return &Circuit{
		ProofClaims:                    proofClaims,
		verifyingKeys:                  emVKeys,
		publicInputVerifyingKey:        piVkEm,
		PublicInputProof:               emPlonk.PlaceholderProof[emFr, emG1, emG2](pi.Circuit),
		PublicInputWitness:             emPlonk.PlaceholderWitness[emFr](pi.Circuit),
		PublicInputWitnessClaimIndexes: make([]frontend.Variable, pi_interconnection.GetMaxNbCircuitsSum(pi.Circuit)),
	}, nil
}

func verifyClaimBatch(api frontend.API, vks []emVkey, claims []proofClaim) error {
	verifier, err := emPlonk.NewVerifier[emFr, emG1, emG2, emGT](api)
	if err != nil {
		return fmt.Errorf("while instantiating the verifier: %w", err)
	}

	var (
		bvk       = vks[0].BaseVerifyingKey
		cvks      = make([]emCircVKey, len(vks)-1)
		switches  = make([]frontend.Variable, len(claims))
		proofs    = make([]emProof, len(claims))
		witnesses = make([]emWitness, len(claims))
	)

	for i := range cvks {
		cvks[i] = vks[i].CircuitVerifyingKey
	}

	for i := 1; i < len(vks)-1; i++ { // TODO @Tabaie make sure these don't generate any constraints
		assertBaseKeyEquals(api, bvk, vks[i].BaseVerifyingKey)
	}

	for i := range claims {
		proofs[i] = claims[i].Proof
		switches[i] = claims[i].CircuitID
		witnesses[i] = claims[i].PublicInput
	}

	lastProofI := len(proofs) - 1
	if err = verifier.AssertDifferentProofs(
		bvk, cvks,
		switches[:lastProofI],
		proofs[:lastProofI],
		witnesses[:lastProofI],
		emPlonk.WithCompleteArithmetic(),
	); err != nil {
		return fmt.Errorf("AssertDifferentProofs returned an error: %w", err)
	}

	// The PI proof cannot be batched with the rest because it has more than one public input
	// complete arithmetic is not necessary here because the circuit is nontrivial and at least one element of the public input is nonzero
	if err = verifier.AssertProof(vks[len(cvks)], proofs[lastProofI], witnesses[lastProofI]); err != nil {
		return fmt.Errorf("AssertProof returned an error: %w", err)
	}

	return nil
}

// assertSlicesEqualZEXT asserts two slices are equal, extending the shorter slice by zeros so the lengths match
func assertSlicesEqualZEXT(api frontend.API, a, b []frontend.Variable) {
	// let a be the shorter one
	if len(b) < len(a) {
		a, b = b, a
	}
	for i := range a {
		api.AssertIsEqual(a[i], b[i])
	}
	for i := len(a); i < len(b); i++ {
		api.AssertIsEqual(b[i], 0)
	}
}

// assertBaseKeyEquals is very aggressive in equality testing between emulated elements. The representations have to be exactly equal, not only equal modulo the group size
func assertBaseKeyEquals(api frontend.API, a, b emPlonk.BaseVerifyingKey[emFr, emG1, emG2]) {
	internal.AssertSliceEquals(api, a.CosetShift.Limbs, b.CosetShift.Limbs)

	assertG2AffEquals := func(a, b sw_bls12377.G2Affine) {
		api.AssertIsEqual(a.P.X.A0, b.P.X.A0)
		api.AssertIsEqual(a.P.X.A1, b.P.X.A1)
		api.AssertIsEqual(a.P.Y.A0, b.P.Y.A0)
		api.AssertIsEqual(a.P.Y.A1, b.P.Y.A1)
	}

	api.AssertIsEqual(a.Kzg.G1.X, b.Kzg.G1.X)
	api.AssertIsEqual(a.Kzg.G1.Y, b.Kzg.G1.Y)
	assertG2AffEquals(a.Kzg.G2[0], b.Kzg.G2[0])
	assertG2AffEquals(a.Kzg.G2[1], b.Kzg.G2[1])
	// NOT CHECKING THE LINE EVALUATIONS

	api.AssertIsEqual(a.NbPublicVariables, b.NbPublicVariables)
}
