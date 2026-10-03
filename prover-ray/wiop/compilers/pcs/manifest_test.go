package pcs

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// elisionSystem is a round-0 batch exercising every manifest rule:
//
//	a   data, opened at shift 0            -> Present
//	b   all zero                            -> Zero
//	c   copy of a, opened at shift 0        -> Alias(a)
//	d   copy of a, opened at shift 1 only   -> Present (a is not opened at 1)
//	g   data, opened at shifts 0 and 1      -> Present
//	h   copy of g, opened at shift 1 only   -> Alias(g)
//	z   all zero, opened at shift 1         -> Zero
type elisionSystem struct {
	sys                 *wiop.System
	r0                  *wiop.Round
	a, b, c, d, g, h, z *wiop.Column
	le                  *wiop.LagrangeEval
}

func newElisionSystem() *elisionSystem {
	sys := wiop.NewSystemf("pcs-elision")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	newCol := func(name string) *wiop.Column { return mod.NewColumn(sys.Context.Childf("%s", name), r0) }
	s := &elisionSystem{sys: sys, r0: r0}
	s.a, s.b, s.c, s.d = newCol("a"), newCol("b"), newCol("c"), newCol("d")
	s.g, s.h, s.z = newCol("g"), newCol("h"), newCol("z")
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	s.le = sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{
		s.a.View(), s.b.View(), s.c.View(), s.d.View().Shift(1),
		s.g.View(), s.g.View().Shift(1), s.h.View().Shift(1), s.z.View().Shift(1),
	}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: s.le})
	return s
}

func rampVec(n int, start uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(start + uint64(i))
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

func (s *elisionSystem) assign(rt *wiop.Runtime) {
	rt.AssignColumn(s.a, rampVec(4, 10))
	rt.AssignColumn(s.b, baseVec(4, 0))
	rt.AssignColumn(s.c, rampVec(4, 10))
	rt.AssignColumn(s.d, rampVec(4, 10))
	rt.AssignColumn(s.g, rampVec(4, 20))
	rt.AssignColumn(s.h, rampVec(4, 20))
	rt.AssignColumn(s.z, baseVec(4, 0))
}

// manifestCells returns round 0's manifest cells: Compile appends them after
// every other cell of the round, one per column.
func (s *elisionSystem) manifestCells() []*wiop.Cell {
	n := len(s.r0.Columns)
	return s.r0.Cells[len(s.r0.Cells)-n:]
}

func (s *elisionSystem) manifestOf(proof wiop.Proof) ColumnManifest {
	cells := s.manifestCells()
	out := make(ColumnManifest, len(cells))
	for i, cell := range cells {
		v := proof.Cells[cell.Context.ID].AsBase()
		out[i] = uint32(v.Uint64())
	}
	return out
}

func (s *elisionSystem) prove() (wiop.Proof, wiop.PublicInput) {
	return s.sys.Prove(s.assign)
}

func TestColumnElisionEndToEnd(t *testing.T) {
	s := newElisionSystem()
	Compile(s.sys)

	proof, pub := s.prove()
	require.Equal(t, ColumnManifest{
		ManifestPresent,  // a
		ManifestZero,     // b
		ManifestAlias(0), // c -> a
		ManifestPresent,  // d: opened at shift 1, a is not
		ManifestPresent,  // g
		ManifestAlias(4), // h -> g
		ManifestZero,     // z
	}, s.manifestOf(proof))
	require.NoError(t, s.sys.Verify(proof, pub))
}

func TestColumnElisionDisabled(t *testing.T) {
	s := newElisionSystem()
	Compile(s.sys, CompileOptions{DisableColumnElision: true})

	proof, pub := s.prove()
	require.Equal(t, allPresentManifest(7), s.manifestOf(proof))
	require.NoError(t, s.sys.Verify(proof, pub))
}

// The elided columns' claim cells are not FRI-bound, so the verifier must pin
// them: zero for a Zero column, the target's claim for an alias.
func TestColumnElisionRejectsTamperedElidedClaims(t *testing.T) {
	one := field.ElemFromBase(field.One())

	t.Run("zero column claims non-zero", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		proof.Cells[s.le.EvaluationClaims[1].Context.ID] = one // b
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("alias claims differ from target", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		proof.Cells[s.le.EvaluationClaims[2].Context.ID] = one // c
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("shifted alias claims differ from target", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		proof.Cells[s.le.EvaluationClaims[6].Context.ID] = one // h at shift 1
		require.Error(t, s.sys.Verify(proof, pub))
	})
}

func TestColumnElisionRejectsTamperedManifest(t *testing.T) {
	setCode := func(s *elisionSystem, proof wiop.Proof, col int, code uint32) {
		proof.Cells[s.manifestCells()[col].Context.ID] = field.ElemFromBase(field.NewElement(uint64(code)))
	}

	t.Run("alias target out of range", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		setCode(s, proof, 2, ManifestAlias(9))
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("forward alias", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		setCode(s, proof, 0, ManifestAlias(3))
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("alias of an elided column", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		setCode(s, proof, 6, ManifestAlias(1)) // z -> b (Zero)
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("alias whose shift is not opened on the target", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		setCode(s, proof, 3, ManifestAlias(0)) // d (shift 1) -> a (shift 0 only)
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("present column declared zero", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		// The committed rows still contain a; the verifier's narrower layout no
		// longer matches the opened rows, and the claim pin fails too.
		setCode(s, proof, 0, ManifestZero)
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("elided column declared present", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		setCode(s, proof, 1, ManifestPresent) // b was committed as Zero
		require.Error(t, s.sys.Verify(proof, pub))
	})

	t.Run("every column elided", func(t *testing.T) {
		s := newElisionSystem()
		Compile(s.sys)
		proof, pub := s.prove()
		for i := range s.r0.Columns {
			setCode(s, proof, i, ManifestZero)
		}
		require.Error(t, s.sys.Verify(proof, pub))
	})
}

// An all-zero batch keeps its first column so the batch is never empty.
func TestColumnElisionKeepsOneColumn(t *testing.T) {
	sys := wiop.NewSystemf("pcs-all-zero")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	x := mod.NewColumn(sys.Context.Childf("x"), r0)
	y := mod.NewColumn(sys.Context.Childf("y"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{x.View(), y.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(x, baseVec(4, 0))
		rt.AssignColumn(y, baseVec(4, 0))
	})
	cells := r0.Cells[len(r0.Cells)-2:]
	xCode := proof.Cells[cells[0].Context.ID].AsBase()
	yCode := proof.Cells[cells[1].Context.ID].AsBase()
	require.Equal(t, uint64(ManifestPresent), xCode.Uint64())
	require.Equal(t, uint64(ManifestZero), yCode.Uint64())
	require.NoError(t, sys.Verify(proof, pub))
}

// Elision of the largest columns of a batch must shrink the FRI top domain
// consistently on both sides.
func TestColumnElisionShrinksTopSize(t *testing.T) {
	sys := wiop.NewSystemf("pcs-top")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	small := sys.NewSizedModule(sys.Context.Childf("small"), 4, wiop.PaddingDirectionNone)
	big := sys.NewSizedModule(sys.Context.Childf("big"), 16, wiop.PaddingDirectionNone)
	x := small.NewColumn(sys.Context.Childf("x"), r0)
	y := big.NewColumn(sys.Context.Childf("y"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{x.View(), y.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(x, rampVec(4, 1))
		rt.AssignColumn(y, baseVec(16, 0))
	})
	require.NoError(t, sys.Verify(proof, pub))
}
