package wiop_test

import (
	"math/big"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// testFRINumQueries keeps the FRI part of the test circuits small. Soundness
// of the tests does not rest on FRI: the tampering they exercise is caught by
// Merkle authentication and the algebraic identities, not by query sampling.
const testFRINumQueries = 2

func useSmallFRI(t *testing.T) {
	t.Helper()
	prev := pcs.FRINumQueries()
	pcs.SetFRINumQueriesForTest(testFRINumQueries)
	t.Cleanup(func() { pcs.SetFRINumQueriesForTest(prev) })
}

type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

// constVec is only safe for tests that never reach the PCS opening: a constant
// column interpolates to a constant polynomial, which makes FRI's deep quotient
// degenerate. Use [nonConstVec] for anything that proves and verifies.
func constVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(val)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// nonConstVec returns a column whose interpolant is not constant.
//
// This matters for every test that exercises the PCS. FRI verifies the deep
// quotient (f(x) − claim)/(x − zeta). When f is constant, the claimed
// evaluation equals that constant at *every* zeta and the encoded codeword is
// that constant at every position, so the numerator vanishes identically. The
// quotient is then zero whatever zeta and the fold challenges are, zeros fold
// to zeros, and no constraint in checkFolds depends on the transcript any
// more — leaving the circuit's Fiat-Shamir derivation completely untested
// while the test still reports success.
func nonConstVec(n int) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(uint64(i*i + 1))
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// newPCSOnlySystem is the smallest PCS-compiled protocol: one committed
// column opened at a coin through a LagrangeEval.
func newPCSOnlySystem() (*wiop.System, *wiop.Column, *wiop.LagrangeEval) {
	sys := wiop.NewSystemf("gnark-pcs")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys)
	return sys, col, le
}

// solveVerifierCircuit compiles the verifier circuit of sys over the given
// field (template and assignment both taken from proof) and reports whether
// the constraint count along with the result of solving the assignment.
func solveVerifierCircuit(
	t *testing.T, sys *wiop.System, template, proof wiop.Proof, pub wiop.PublicInput, modulus *big.Int,
) (int, error) {
	t.Helper()
	circ := wiop.AllocateVerifierCircuit(sys, template, pub)

	var ccs interface {
		IsSolved(witness.Witness, ...solver.Option) error
		GetNbConstraints() int
	}
	var err error
	if modulus.Cmp(field.Modulus()) == 0 {
		ccs, err = frontend.CompileU32(modulus, scs.NewBuilder, circ)
	} else {
		ccs, err = frontend.Compile(modulus, scs.NewBuilder, circ)
	}
	require.NoError(t, err, "verifier circuit must compile")

	assignment := wiop.AllocateVerifierCircuit(sys, template, pub).AssignVerifierCircuit(proof, pub)
	w, err := frontend.NewWitness(assignment, modulus)
	require.NoError(t, err, "assignment must produce a witness")
	return ccs.GetNbConstraints(), ccs.IsSolved(w)
}

func TestVerifierCircuit_PCSOnly(t *testing.T) {
	useSmallFRI(t)
	sys, col, le := newPCSOnlySystem()
	// Must be non-constant, or the deep quotient vanishes and the circuit's
	// Fiat-Shamir derivation stops being constrained by anything. See
	// [nonConstVec].
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(8)) })
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

	t.Run("honest-native", func(t *testing.T) {
		nb, err := solveVerifierCircuit(t, sys, proof, proof, pub, field.Modulus())
		require.NoError(t, err, "honest proof must satisfy the circuit")
		t.Logf("constraints (native koalabear): %d", nb)
	})
	t.Run("honest-emulated-bn254", func(t *testing.T) {
		nb, err := solveVerifierCircuit(t, sys, proof, proof, pub, ecc.BN254.ScalarField())
		require.NoError(t, err, "honest proof must satisfy the circuit")
		t.Logf("constraints (emulated over BN254): %d", nb)
	})
	t.Run("tampered-claim", func(t *testing.T) {
		bad := cloneProof(proof)
		bad.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemFromExt(field.Uint64ToExt(7))
		require.Error(t, sys.Verify(bad, pub), "tampered claim must fail natively")
		_, err := solveVerifierCircuit(t, sys, proof, bad, pub, field.Modulus())
		require.Error(t, err, "tampered claim must not satisfy the circuit")
	})
	t.Run("tampered-commitment", func(t *testing.T) {
		bad := cloneProof(proof)
		root := bad.Commitments[0]
		one := field.One()
		root[0].Add(&root[0], &one)
		bad.Commitments[0] = root
		_, err := solveVerifierCircuit(t, sys, proof, bad, pub, field.Modulus())
		require.Error(t, err, "tampered commitment must not satisfy the circuit")
	})
}

func TestVerifierCircuit_Vanishing(t *testing.T) {
	useSmallFRI(t)
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		if hasDynamicModule(sc.Sys) {
			continue
		}
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			pcs.Compile(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			nb, err := solveVerifierCircuit(t, sc.Sys, proof, proof, pub, field.Modulus())
			require.NoError(t, err, "honest proof must satisfy the circuit")
			t.Logf("constraints (native koalabear): %d", nb)

			// An invalid witness yields a proof of the same shape that the
			// native verifier rejects; the circuit must reject it too.
			invalid := build()
			global.Compile(invalid.Sys)
			pcs.Compile(invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")
			_, err = solveVerifierCircuit(t, sc.Sys, proof, badProof, badPub, field.Modulus())
			require.Error(t, err, "invalid witness must not satisfy the circuit")
		})
	}
}

func TestVerifierCircuit_Vanishing_Emulated(t *testing.T) {
	useSmallFRI(t)
	sc := wioptest.NewMixedRatioVanishingsScenario()
	global.Compile(sc.Sys)
	pcs.Compile(sc.Sys)
	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

	nb, err := solveVerifierCircuit(t, sc.Sys, proof, proof, pub, ecc.BN254.ScalarField())
	require.NoError(t, err, "honest proof must satisfy the emulated circuit")
	t.Logf("constraints (emulated over BN254): %d", nb)
}

// unsupportedAction is a verifier action without an in-circuit counterpart.
type unsupportedAction struct{}

func (unsupportedAction) Check(*wiop.Runtime) error { return nil }

func TestAllocateVerifierCircuit_RejectsUnsupportedAction(t *testing.T) {
	useSmallFRI(t)
	sys, col, _ := newPCSOnlySystem()
	sys.Rounds[1].RegisterVerifierAction(unsupportedAction{})
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, constVec(8, 3)) })

	require.Panics(t, func() { wiop.AllocateVerifierCircuit(sys, proof, pub) },
		"a verifier action without CheckGnark must be rejected instead of dropped")
}

func TestAssignVerifierCircuit_RejectsShapeMismatch(t *testing.T) {
	useSmallFRI(t)
	sys, col, le := newPCSOnlySystem()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, constVec(8, 3)) })
	circ := wiop.AllocateVerifierCircuit(sys, proof, pub)

	bad := cloneProof(proof)
	// The claim is carried as an extension element; retagging it as base
	// changes the transcript layout and must be refused.
	bad.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemFromBase(field.NewElement(3))
	require.Panics(t, func() { circ.AssignVerifierCircuit(bad, pub) },
		"a proof whose cell field tags differ from the template must be rejected")
}

func hasDynamicModule(sys *wiop.System) bool {
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			return true
		}
	}
	return false
}

func cloneProof(p wiop.Proof) wiop.Proof {
	res := wiop.Proof{
		Cells:           make(map[wiop.ObjectID]field.Gen, len(p.Cells)),
		DynamicSizes:    make(map[int]int, len(p.DynamicSizes)),
		Commitments:     make(map[int]field.Octuplet, len(p.Commitments)),
		PCSOpeningProof: p.PCSOpeningProof,
	}
	for k, v := range p.Cells {
		res.Cells[k] = v
	}
	for k, v := range p.DynamicSizes {
		res.DynamicSizes[k] = v
	}
	for k, v := range p.Commitments {
		res.Commitments[k] = v
	}
	return res
}
