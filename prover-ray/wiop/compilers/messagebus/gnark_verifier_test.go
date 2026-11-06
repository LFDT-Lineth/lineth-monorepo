package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// TestGnarkVerifier_SharedRandomness checks the in-circuit
// [messagebus.SharedRandomnessContributionChecker]: a shard compiled with
// shared randomness is accepted by the verifier circuit on an honest proof,
// and rejected when the prover publishes a forged contribution limb.
func TestGnarkVerifier_SharedRandomness(t *testing.T) {
	// The in-circuit PCS verifier requires root dedup off.
	shard1 := buildSeededBidirectionalShard(
		t, "shard-1-gnark", "shard-1", crossShardTrafficShard1, pcs.WithoutRootDedup())
	shard2 := buildSeededBidirectionalShard(
		t, "shard-2-gnark", "shard-2", crossShardTrafficShard2, pcs.WithoutRootDedup())
	g := preflight.Run(busInputSets(t, shard1, shard2))

	_, proof, pub := shard1.runProve(t, g)
	require.NoError(t, shard1.sys.Verify(proof, pub), "honest proof must verify natively")

	sys := shard1.sys
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder,
		wiop.AllocateVerifierCircuit(sys, proof, pub))
	require.NoError(t, err, "verifier circuit must compile")
	t.Logf("constraints: %d", ccs.GetNbConstraints())

	solve := func(proof wiop.Proof, pub wiop.PublicInput) error {
		w, err := frontend.NewWitness(
			wiop.AllocateVerifierCircuit(sys, proof, pub).AssignVerifierCircuit(proof, pub),
			ecc.BN254.ScalarField())
		require.NoError(t, err)
		_, err = ccs.Solve(w)
		return err
	}
	require.NoError(t, solve(proof, pub), "honest proof must satisfy the circuit")

	// A dishonest prover publishes a wrong contribution limb. The lazy cell is
	// resolved before the round is absorbed, so the transcript stays
	// consistent with the forged value and the PCS still verifies: only the
	// contribution check can reject the proof. Tampering the public input after
	// the fact would not test that, since the diverging transcript alone gets
	// it rejected.
	cell, pos := sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedContributionPI, 0)
	require.GreaterOrEqual(t, pos, 0)
	honest := cell.Assigner
	cell.Assigner = func(rt *wiop.Runtime) field.Gen {
		return honest(rt).Add(field.ElemFromBase(field.One())) // contribution limb 0 is off by one
	}
	defer func() { cell.Assigner = honest }()

	_, badProof, badPub := shard1.runProve(t, g) // the malformed proof
	// Natively, SharedRandomnessContributionChecker.Check finds the mismatch.
	require.Error(t, sys.Verify(badProof, badPub), "forged contribution must fail natively")
	// In-circuit, CheckGnark's assertion against HashGnark(commitment) fails.
	require.Error(t, solve(badProof, badPub), "forged contribution must not satisfy the circuit")
}
