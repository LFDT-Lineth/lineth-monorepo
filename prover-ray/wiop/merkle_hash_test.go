package wiop_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// newMerkleHashSystem builds the PCS-only system of [newPCSOnlyCapSystem]
// with the given extra PCS options.
func newMerkleHashSystem(opts ...pcs.Option) (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("gnark-pcs-merkle-hash")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 16, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, append(testPCSOptions(), opts...)...)
	return sys, col
}

// TestVerifierCircuit_MerkleHash checks both Merkle hash modes end to end,
// natively and in the verifier circuit, with and without a Merkle cap, and
// logs the circuit sizes.
func TestVerifierCircuit_MerkleHash(t *testing.T) {
	for _, hash := range []fri.MerkleHash{fri.MerkleHashKoalaBear, fri.MerkleHashBNPacked} {
		for _, capHeight := range []int{0, 2} {
			t.Run(fmt.Sprintf("%v/cap=%d", hash, capHeight), func(t *testing.T) {
				sys, col := newMerkleHashSystem(pcs.WithMerkleHash(hash), pcs.WithMerkleCapHeight(capHeight))
				require.Equal(t, hash, pcs.MerkleHash(sys), "System must record its Merkle hash")

				proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(16)) })
				require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

				cv := compileVerifierCircuit(t, sys, proof, pub)
				require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
				t.Logf("constraints: %d", cv.ccs.GetNbConstraints())

				bad := cloneProof(proof)
				root := bad.Commitments[0]
				one := field.One()
				root[0].Add(&root[0], &one)
				bad.Commitments[0] = root
				require.Error(t, sys.Verify(bad, pub), "tampered commitment must fail natively")
				require.Error(t, cv.solve(t, bad, pub), "tampered commitment must not satisfy the circuit")
			})
		}
	}
}

// TestCompile_MerkleHashBindsProof checks that the Merkle hash is a property
// of the System: a proof made in one mode is rejected by the other.
func TestCompile_MerkleHashBindsProof(t *testing.T) {
	koala, colK := newMerkleHashSystem()
	bn, colB := newMerkleHashSystem(pcs.WithMerkleHash(fri.MerkleHashBNPacked))

	proofK, pubK := koala.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(colK, nonConstVec(16)) })
	proofB, pubB := bn.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(colB, nonConstVec(16)) })
	require.NotEqual(t, proofK.Commitments, proofB.Commitments, "modes must commit to different roots")

	require.Error(t, bn.Verify(proofK, pubK), "KoalaBear proof must not verify in BN-packed mode")
	require.Error(t, koala.Verify(proofB, pubB), "BN-packed proof must not verify in KoalaBear mode")
}

// TestConfigure_MerkleHashConflict checks that a Compile cannot downgrade the
// hash a consumed compile already froze.
func TestConfigure_MerkleHashConflict(t *testing.T) {
	sys, _ := newMerkleHashSystem()
	require.Error(t, pcs.Validate(sys, pcs.WithMerkleHash(fri.MerkleHashBNPacked)))
	require.Panics(t, func() { pcs.WithMerkleHash(fri.MerkleHashBNPacked + 1) })
}
