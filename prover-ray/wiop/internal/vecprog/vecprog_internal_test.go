package vecprog

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// tree is a reference expression, evaluated point by point.
type tree struct {
	op     Op
	args   []*tree
	base   []field.Element // base leaf
	ext    []field.Ext     // extension leaf
	offset int
	s      *field.Ext // scalar leaf
	isBase bool
}

// at evaluates t at point i of n.
func (t *tree) at(i, n int) field.Ext {
	j := (i + t.offset) % n
	switch {
	case t.s != nil:
		return *t.s
	case t.base != nil:
		return field.Lift(t.base[j])
	case t.ext != nil:
		return t.ext[j]
	}
	vals := make([]field.Ext, len(t.args))
	for k, a := range t.args {
		vals[k] = a.at(i, n)
	}
	return applyExt(t.op, vals)
}

func (t *tree) build(b *Builder) int {
	switch {
	case t.s != nil:
		return b.Scalar(*t.s, t.isBase)
	case t.base != nil:
		return b.Base(t.base, t.offset)
	case t.ext != nil:
		return b.Ext(t.ext, t.offset)
	}
	args := make([]int, len(t.args))
	for k, a := range t.args {
		args[k] = a.build(b)
	}
	return b.Op(t.op, args...)
}

type treeGen struct {
	rng  *rand.Rand
	n    int
	base [][]field.Element
	ext  [][]field.Ext
}

func newTreeGen(rng *rand.Rand, n int) *treeGen {
	g := &treeGen{rng: rng, n: n}
	for range 3 {
		v := field.VecPseudoRandBase(rng, n)
		e := field.VecPseudoRandExt(rng, n)
		for j := range n {
			if rng.IntN(8) == 0 {
				v[j], e[j] = field.Element{}, field.Ext{}
			}
		}
		g.base, g.ext = append(g.base, v), append(g.ext, e)
	}
	return g
}

func (g *treeGen) leaf() *tree {
	switch g.rng.IntN(5) {
	case 0, 1:
		return &tree{base: g.base[g.rng.IntN(len(g.base))], offset: g.rng.IntN(g.n), isBase: true}
	case 2:
		return &tree{ext: g.ext[g.rng.IntN(len(g.ext))], offset: g.rng.IntN(g.n)}
	case 3:
		s := field.Lift(field.VecPseudoRandBase(g.rng, 1)[0])
		return &tree{s: &s, isBase: true}
	}
	s := field.VecPseudoRandExt(g.rng, 1)[0]
	return &tree{s: &s}
}

func (g *treeGen) tree(depth int) *tree {
	if depth == 0 || g.rng.IntN(4) == 0 {
		return g.leaf()
	}
	o := Op(g.rng.IntN(int(Inverse) + 1))
	t := &tree{op: o, isBase: true}
	for range o.Arity() {
		a := g.tree(depth - 1)
		t.args = append(t.args, a)
		t.isBase = t.isBase && a.isBase
	}
	return t
}

// The Store, AddTo and Check sinks must match a point-by-point evaluation,
// over sizes below, at and across a block, and with zeros hitting every
// division and inversion.
func TestSinksMatchTreeEvaluation(t *testing.T) {
	for _, n := range []int{5, BlockSize, 3*BlockSize + 17} {
		for seed := range uint64(6) {
			t.Run(fmt.Sprintf("n=%d/seed=%d", n, seed), func(t *testing.T) {
				g := newTreeGen(rand.New(rand.NewPCG(seed, uint64(n))), n)
				trees := make([]*tree, 8)
				for i := range trees {
					trees[i] = g.tree(5)
				}

				b := NewBuilder()
				stored := make([][]field.Ext, len(trees))
				sum := make([]field.Ext, n)
				for i, tr := range trees {
					id := tr.build(b)
					stored[i] = make([]field.Ext, n)
					b.Store(id, stored[i])
					b.AddTo(id, sum)
				}
				var zeros []int
				guard := trees[1].build(b)
				b.Check(trees[0].build(b), guard, func(i int) { zeros = append(zeros, i) })
				b.Compile(n, 1, 1).Run(nil, nil, 1)

				wantSum := make([]field.Ext, n)
				var wantZeros []int
				for j := range n {
					for i, tr := range trees {
						v := tr.at(j, n)
						require.Equal(t, v, stored[i][j], "tree %d, point %d", i, j)
						wantSum[j].Add(&wantSum[j], &v)
					}
					r, gv := trees[0].at(j, n), trees[1].at(j, n)
					if r.IsZero() && !gv.IsZero() {
						wantZeros = append(wantZeros, j)
					}
				}
				require.Equal(t, wantSum, sum)
				require.Equal(t, wantZeros, zeros)
			})
		}
	}
}

// A log-derivative style constraint, a product of Horner-form random linear
// combinations each used twice, must be lowered without any extension product
// inside the linear combinations and with each combination computed once.
func TestFoldsHornerForms(t *testing.T) {
	g := newTreeGen(rand.New(rand.NewPCG(7, 7)), 64)
	horner := func(k int) *tree {
		beta := field.VecPseudoRandExt(g.rng, 1)[0]
		alpha := field.VecPseudoRandExt(g.rng, 1)[0]
		acc := &tree{base: g.base[0], offset: 1, isBase: true}
		for i := range k - 1 {
			acc = &tree{op: Add, args: []*tree{
				{op: Mul, args: []*tree{{s: &beta}, acc}},
				{base: g.base[i%len(g.base)], offset: i + 2, isBase: true},
			}}
		}
		return &tree{op: Add, args: []*tree{{s: &alpha}, acc}}
	}
	d1, d2 := horner(8), horner(8)
	// d1·d2 − (d1 + d2): each denominator appears twice.
	expr := &tree{op: Sub, args: []*tree{
		{op: Mul, args: []*tree{d1, d2}},
		{op: Add, args: []*tree{d1, d2}},
	}}
	b := NewBuilder()
	b.Store(expr.build(b), make([]field.Ext, g.n))
	p := b.Compile(g.n, 1, 1)

	var linear, extMul int
	for _, st := range p.steps {
		if st.sink >= 0 {
			continue
		}
		nd := &p.nodes[st.node]
		switch {
		case nd.kind == kLinear:
			linear++
		case nd.kind == kOp && !nd.isBase && nd.op == Mul:
			extMul++
		}
	}
	// d1 and d2 are two linear steps; d1 + d2 folds into a third. The only
	// extension product left is d1·d2.
	require.Equal(t, 3, linear)
	require.Equal(t, 1, extMul)
}
