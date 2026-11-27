package risc5_test

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/consensys/gnark-crypto/ecc"
	cs_bn254 "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

// newProgramVKSystem builds a minimal PCS-compiled system whose program round
// commits one column and exposes its commitment as the program VK.
func newProgramVKSystem() (*wiop.System, *wiop.Column) {
	// Round 0 is the program round; round 1 holds the trace and the coin.
	sys := wiop.NewSystemf("program-vk-gnark")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	// RegisterProgramVK panics if the trace is in the program round.
	sys.DynamicSizeRound = r1.ID
	// One column committed in the program round, so its commitment is the VK.
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	// Open the column at a coin so the PCS has something to prove.
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	// Register the VK cells and the ProgramVKChecker before compiling the PCS.
	risc5.RegisterProgramVK(sys)
	pcs.Compile(sys, pcs.WithFRINumQueries(2))
	return sys, col
}

// TestProgramVKChecker_Gnark checks that the in-circuit program-VK check
// accepts the honest VK and rejects a forged limb, like the native check.
func TestProgramVKChecker_Gnark(t *testing.T) {
	sys, col := newProgramVKSystem()
	// Non-constant column values, so the PCS opening is not trivial.
	vals := make([]field.Element, 8)
	for i := range vals {
		vals[i].SetUint64(uint64(i*i + 1))
	}
	// Prove natively and check the honest proof verifies with a non-zero VK.
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vals)})
	})
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")
	require.NotEqual(t, field.Octuplet{}, risc5.GetProgramVK(sys, pub), "the program round must be committed")

	// Compile the in-circuit verifier over BN254; the ProgramVKChecker runs
	// through CheckGnark.
	modulus := ecc.BN254.ScalarField()
	circ := wiop.AllocateVerifierCircuit(sys)
	ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
	require.NoError(t, err)

	// solve assigns (proof, pub) to the circuit and returns the solver error.
	solve := func(pub wiop.PublicInput) error {
		w, err := frontend.NewWitness(circ.AssignVerifierCircuit(proof, pub), modulus)
		require.NoError(t, err)
		_, err = ccs.Solve(w)
		return err
	}

	require.NoError(t, solve(pub), "honest VK must satisfy the circuit")

	// Flip one bit of VK limb 3: both verifiers must reject it.
	_, pos := sys.LookupPublicInputByTag(risc5.ProgramVKPI, 3)
	forged := append(wiop.PublicInput(nil), pub...)
	limb := forged[pos].AsBase()
	limb.SetUint64(limb.Uint64() ^ 1)
	forged[pos] = field.ElemFromBase(limb)
	require.ErrorIs(t, sys.Verify(proof, forged), risc5.ErrProgramVKMismatch)
	requireUnsatisfied(t, solve(forged), "forged VK must not satisfy the circuit")
}

// TestProgramVKChecker_Gnark_NoTranscript isolates the in-circuit check: with
// no PCS the expected VK is zero, and no coin is drawn after the VK cells, so
// the Fiat-Shamir replay cannot catch a forged limb; only CheckGnark can.
func TestProgramVKChecker_Gnark_NoTranscript(t *testing.T) {
	// No PCS and no coins: round 0 holds the VK cells, round 1 the trace.
	sys := wiop.NewSystemf("program-vk-gnark-nopcs")
	sys.NewRound()
	r1 := sys.NewRound()
	sys.DynamicSizeRound = r1.ID
	risc5.RegisterProgramVK(sys)

	// Nothing to assign. Without a commitment the VK is all zeros.
	proof, pub := sys.Prove(func(*wiop.Runtime) {})
	require.NoError(t, sys.Verify(proof, pub))
	require.Equal(t, field.Octuplet{}, risc5.GetProgramVK(sys, pub))

	// Compile the in-circuit verifier and define the solve helper, as above.
	modulus := ecc.BN254.ScalarField()
	circ := wiop.AllocateVerifierCircuit(sys)
	ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
	require.NoError(t, err)
	solve := func(pub wiop.PublicInput) error {
		w, err := frontend.NewWitness(circ.AssignVerifierCircuit(proof, pub), modulus)
		require.NoError(t, err)
		_, err = ccs.Solve(w)
		return err
	}
	require.NoError(t, solve(pub), "honest VK must satisfy the circuit")

	// Set each VK limb in turn to one instead of zero, so a check that skips
	// any limb is caught. No transcript absorbs the VK cells, so only
	// CheckGnark can make the circuit reject the forgery.
	for i := range risc5.NumProgramVK {
		t.Run(fmt.Sprintf("limb-%d", i), func(t *testing.T) {
			_, pos := sys.LookupPublicInputByTag(risc5.ProgramVKPI, i)
			forged := append(wiop.PublicInput(nil), pub...)
			forged[pos] = field.ElemFromBase(field.One())
			require.ErrorIs(t, sys.Verify(proof, forged), risc5.ErrProgramVKMismatch)
			requireUnsatisfied(t, solve(forged), "forged VK must not satisfy the circuit")
		})
	}
}

// requireUnsatisfied requires err to be the solver rejecting a constraint, not
// any other failure (a bad assignment, a missing value).
func requireUnsatisfied(t *testing.T, err error, msg string) {
	t.Helper()
	var unsat *cs_bn254.UnsatisfiedConstraintError
	require.ErrorAs(t, err, &unsat, msg)
}
