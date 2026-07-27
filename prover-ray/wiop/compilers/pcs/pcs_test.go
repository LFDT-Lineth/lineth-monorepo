package pcs

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

func init() {
	// Size-4+ fixtures below take the FRI path; threshold 1 is the minimum, since
	// FRI cannot commit a size-1 column. Reveal tests raise it via withReveal.
	SetMaxRevealLenForTest(1)
}

// withReveal sets maxRevealLen for the duration of a test and restores the
// prior value afterwards.
func withReveal(t *testing.T, n int) {
	t.Helper()
	old := maxRevealLen
	SetMaxRevealLenForTest(n)
	t.Cleanup(func() { SetMaxRevealLenForTest(old) })
}

// selfAssignLagrange is a prover action that fills a LagrangeEval's claim cells
// from the committed column assignments. In the real pipeline the global pass
// owns this; here the test supplies it directly.
type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

func baseVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// newPCSSizedSystem builds the smallest protocol the PCS pass can compile: one
// oracle column of the given size committed in round 0, evaluated at a verifier
// coin in round 1 via a LagrangeEval. The claim cell is self-assigned by a
// round-1 prover action so sys.Prove drives the whole flow.
func newPCSSizedSystem(size int) (*wiop.System, *wiop.Column, *wiop.LagrangeEval) {
	sys := wiop.NewSystemf("pcs-it")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), size, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	return sys, col, le
}

// newPCSTestSystem is [newPCSSizedSystem] at size 4, the FRI-path fixture used
// by the Compile tests (reveal is disabled in this package's test init).
func newPCSTestSystem() (*wiop.System, *wiop.Column, *wiop.LagrangeEval) {
	return newPCSSizedSystem(4)
}

// TestCompileEndToEnd checks that an honest witness passes through the full
// commit → open → verify flow that Compile wires up.
func TestCompileEndToEnd(t *testing.T) {
	sys, col, _ := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// The committed column must not survive as raw data in the proof.
	require.Empty(t, proof.Columns, "PCS-compiled proof must not carry oracle columns")
	require.NotNil(t, proof.PCSOpeningProof, "proof must carry the FRI opening proof")
	require.NotEmpty(t, proof.Commitments, "proof must carry the round commitments")

	require.NoError(t, sys.Verify(proof, pub), "honest witness must verify")
}

// TestCompileRejectsWrongClaim checks that tampering with a claimed evaluation
// (the value the FRI opening is meant to bind) is rejected.
func TestCompileRejectsWrongClaim(t *testing.T) {
	sys, col, le := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// The true evaluation of the constant-3 column is 3; claim 0 instead.
	proof.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemZero()

	require.Error(t, sys.Verify(proof, pub), "a tampered evaluation claim must be rejected")
}

// TestCompileDynamicModule checks the full flow when the committed column lives
// in a dynamic module, whose size is only known at prove time. The FRI
// parameters are built from the runtime size on both the prover and verifier.
func TestCompileDynamicModule(t *testing.T) {
	sys := wiop.NewSystemf("pcs-dyn")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})

	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(8, 5)) // size fixed to 8 at prove time
	})
	require.Equal(t, 8, proof.DynamicSizes[0], "dynamic size must travel in the proof")
	require.NoError(t, sys.Verify(proof, pub), "honest dynamic-module witness must verify")
}

// TestCompileRejectsPublicColumn checks that a verifier-visible column in a
// committed round is rejected: it cannot be replaced by a commitment.
func TestCompileRejectsPublicColumn(t *testing.T) {
	sys := wiop.NewSystemf("pcs-pub")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	mod.NewColumn(sys.Context.Childf("pub"), wiop.VisibilityPublic, r0)

	require.Panics(t, func() { Compile(sys) }, "a public committed column must be rejected")
}

// TestCompileRejectsTamperedCommitment checks that corrupting a transported
// round commitment is rejected.
func TestCompileRejectsTamperedCommitment(t *testing.T) {
	sys, col, _ := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// Round 0 owns the only committed batch; flip a byte of its root.
	root := proof.Commitments[0]
	one := field.One()
	root[0].Add(&root[0], &one)
	proof.Commitments[0] = root

	require.Error(t, sys.Verify(proof, pub), "a tampered commitment must be rejected")
}

// proveReveal compiles and proves a one-column system of the given size with
// reveal enabled up to length 8.
func proveReveal(t *testing.T, size int) (*wiop.System, wiop.Proof, wiop.PublicInput) {
	t.Helper()
	withReveal(t, 8)
	sys, col, _ := newPCSSizedSystem(size)
	Compile(sys)
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(size, 3))
	})
	return sys, proof, pub
}

// TestCompileRevealSmallColumn checks the reveal path end to end: an honest
// witness verifies with no FRI opening, the coefficients travel in the proof,
// and tampering with either the claim or the coefficients is rejected.
func TestCompileRevealSmallColumn(t *testing.T) {
	for _, size := range []int{1, 8} {
		t.Run(fmt.Sprintf("honest-size-%d", size), func(t *testing.T) {
			sys, proof, pub := proveReveal(t, size)
			require.Empty(t, proof.Columns, "revealed column must not survive as a raw oracle column")
			require.Nil(t, proof.PCSOpeningProof, "a reveal-only proof carries no FRI opening")
			require.NotEmpty(t, proof.RevealedColumns, "proof must carry the revealed coefficients")
			require.NoError(t, sys.Verify(proof, pub), "honest reveal witness must verify")
		})
	}

	t.Run("tampered-claim", func(t *testing.T) {
		sys, proof, pub := proveReveal(t, 4)
		for id := range proof.Cells { // the sole claim cell; honest value is 3
			proof.Cells[id] = field.ElemZero()
		}
		require.Error(t, sys.Verify(proof, pub), "a tampered reveal claim must be rejected")
	})

	t.Run("tampered-coefficients", func(t *testing.T) {
		sys, proof, pub := proveReveal(t, 4)
		one := field.One()
		for id, coeffs := range proof.RevealedColumns {
			base := coeffs.AsBase()
			base[0].Add(&base[0], &one)
			proof.RevealedColumns[id] = field.VecFromBase(base)
		}
		require.Error(t, sys.Verify(proof, pub), "tampered revealed coefficients must be rejected")
	})
}

// TestCompileRevealAndFRIMixed checks a round owning both a revealed (small) and
// a FRI-committed (large) column: both are discharged and an honest witness
// verifies.
func TestCompileRevealAndFRIMixed(t *testing.T) {
	withReveal(t, 8)

	sys := wiop.NewSystemf("pcs-mixed")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	small := sys.NewSizedModule(sys.Context.Childf("small"), 4, wiop.PaddingDirectionNone)
	large := sys.NewSizedModule(sys.Context.Childf("large"), 16, wiop.PaddingDirectionNone)
	smallCol := small.NewColumn(sys.Context.Childf("smallCol"), wiop.VisibilityOracle, r0)
	largeCol := large.NewColumn(sys.Context.Childf("largeCol"), wiop.VisibilityOracle, r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"),
		[]*wiop.ColumnView{smallCol.View(), largeCol.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})

	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(smallCol, baseVec(4, 3))
		rt.AssignColumn(largeCol, baseVec(16, 5))
	})

	require.NotNil(t, proof.PCSOpeningProof, "mixed proof must carry the FRI opening for the large column")
	require.NotEmpty(t, proof.RevealedColumns, "mixed proof must carry the small column's coefficients")
	require.NoError(t, sys.Verify(proof, pub), "honest mixed witness must verify")
}
