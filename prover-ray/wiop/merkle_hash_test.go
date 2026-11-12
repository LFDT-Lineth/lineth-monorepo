package wiop_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
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

				// With a cap, the tampered root fails the cap check before any query.
				wantNative := "query 0: input tree 0: invalid Merkle proof"
				if capHeight > 0 {
					wantNative = "input cap 0: invalid Merkle cap"
				}
				bad := cloneProof(proof)
				root := bad.Commitments[0]
				one := field.One()
				root[0].Add(&root[0], &one)
				bad.Commitments[0] = root
				require.ErrorContains(t, sys.Verify(bad, pub), wantNative)
				require.ErrorContains(t, cv.solve(t, bad, pub), "is not satisfied")
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

	require.ErrorContains(t, bn.Verify(proofK, pubK), "input tree 0: invalid Merkle proof")
	require.ErrorContains(t, koala.Verify(proofB, pubB), "query 0: input tree 0: invalid Merkle proof")
}

// TestConfigure_MerkleHashConflict checks that a Compile cannot downgrade the
// hash a consumed compile already froze.
func TestConfigure_MerkleHashConflict(t *testing.T) {
	sys, _ := newMerkleHashSystem()
	require.ErrorContains(t, pcs.Validate(sys, pcs.WithMerkleHash(fri.MerkleHashBNPacked)), "WithMerkleHash(bn-packed-poseidon2) conflicts with the consumed compile with koalabear-poseidon2")
	require.PanicsWithValue(t, "unknown Merkle hash MerkleHash(2)",
		func() { pcs.WithMerkleHash(fri.MerkleHashBNPacked + 1) })
}

// TestVerifierCircuit_PLONKProve runs the full gnark PLONK flow (setup,
// prove, verify) on the wiop verifier circuit in both Merkle hash modes, and
// checks that a tampered wiop proof cannot be proven. The SRS comes from
// [unsafekzg], so this is a test of the circuit, not a production setup.
func TestVerifierCircuit_PLONKProve(t *testing.T) {
	if testing.Short() {
		t.Skip("PLONK setup and proving")
	}
	modulus := ecc.BN254.ScalarField()
	for _, hash := range []fri.MerkleHash{fri.MerkleHashKoalaBear, fri.MerkleHashBNPacked} {
		t.Run(hash.String(), func(t *testing.T) {
			sys, col := newMerkleHashSystem(pcs.WithMerkleHash(hash))
			proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(16)) })
			require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

			ccs, err := frontend.Compile(modulus, scs.NewBuilder, wiop.AllocateVerifierCircuit(sys, proof, pub))
			require.NoError(t, err)
			srs, srsLagrange, err := unsafekzg.NewSRS(ccs)
			require.NoError(t, err)
			pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
			require.NoError(t, err)

			assignment := wiop.AllocateVerifierCircuit(sys, proof, pub).AssignVerifierCircuit(proof, pub)
			w, err := frontend.NewWitness(assignment, modulus)
			require.NoError(t, err)
			public, err := w.Public()
			require.NoError(t, err)

			start := time.Now()
			plonkProof, err := plonk.Prove(ccs, pk, w)
			require.NoError(t, err, "honest wiop proof must be provable")
			t.Logf("constraints=%d prove=%v", ccs.GetNbConstraints(), time.Since(start).Round(time.Millisecond))
			require.NoError(t, plonk.Verify(plonkProof, vk, public), "PLONK proof must verify")

			bad := cloneProof(proof)
			root := bad.Commitments[0]
			one := field.One()
			root[0].Add(&root[0], &one)
			bad.Commitments[0] = root
			badW, err := frontend.NewWitness(
				wiop.AllocateVerifierCircuit(sys, proof, pub).AssignVerifierCircuit(bad, pub), modulus)
			require.NoError(t, err)
			_, err = plonk.Prove(ccs, pk, badW)
			require.ErrorContains(t, err, "is not satisfied", "tampered wiop proof must not be provable")
		})
	}
}
