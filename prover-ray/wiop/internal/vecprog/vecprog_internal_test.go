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

// at evaluates t at point i of cosets of n points, coset k of the points
// reading coset k·step of the leaves' tables.
func (t *tree) at(i, n, step int) field.Ext {
	j := i/n*step*n + (i%n+t.offset)%n
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
		vals[k] = a.at(i, n, step)
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

// newTreeGen draws trees over cosets of n points, from tables of size points.
func newTreeGen(rng *rand.Rand, n, size int) *treeGen {
	g := &treeGen{rng: rng, n: n}
	for range 3 {
		v := field.VecPseudoRandBase(rng, size)
		e := field.VecPseudoRandExt(rng, size)
		for j := range size {
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
				g := newTreeGen(rand.New(rand.NewPCG(seed, uint64(n))), n, n)
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
						v := tr.at(j, n, 1)
						require.Equal(t, v, stored[i][j], "tree %d, point %d", i, j)
						wantSum[j].Add(&wantSum[j], &v)
					}
					r, gv := trees[0].at(j, n, 1), trees[1].at(j, n, 1)
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

// The Accumulate sinks must add coef·root·cancellation over ratio cosets of n
// points, coset k reading coset k·step of the tables, then scale coset k by
// scale[k], as a point-by-point evaluation does: on sizes below, at and across
// a block, with subtrees shared across sinks and zeros hitting every division
// and inversion.
func TestAccumulateMatchesTreeEvaluation(t *testing.T) {
	for _, tc := range []struct{ n, ratio, step int }{
		{4, 1, 1}, {8, 2, 2}, {75, 4, 1}, {BlockSize, 1, 4}, {2*BlockSize + 40, 4, 1},
	} {
		for seed := range uint64(4) {
			t.Run(fmt.Sprintf("n=%d/ratio=%d/step=%d/seed=%d", tc.n, tc.ratio, tc.step, seed), func(t *testing.T) {
				size, N := tc.n*tc.ratio*tc.step, tc.n*tc.ratio
				g := newTreeGen(rand.New(rand.NewPCG(seed, uint64(tc.n))), tc.n, size)
				b := NewBuilder()
				want := make([]field.Ext, N)
				var prev *tree
				for range 12 {
					tr := g.tree(5)
					if prev != nil && g.rng.IntN(3) == 0 {
						tr = &tree{op: Mul, args: []*tree{prev, tr}}
					}
					prev = tr
					var cancellation []field.Element
					if g.rng.IntN(2) == 0 {
						cancellation = g.base[g.rng.IntN(len(g.base))]
					}
					coef := field.VecPseudoRandExt(g.rng, 1)[0]
					b.Accumulate(tr.build(b), cancellation, coef)
					for j := range N {
						v := tr.at(j, tc.n, tc.step)
						if cancellation != nil {
							v.MulByElement(&v, &cancellation[j/tc.n*tc.step*tc.n+j%tc.n])
						}
						v.Mul(&v, &coef)
						want[j].Add(&want[j], &v)
					}
				}
				scale := field.VecPseudoRandBase(g.rng, tc.ratio)
				for j := range want {
					want[j].MulByElement(&want[j], &scale[j/tc.n])
				}

				got := make([]field.Ext, N)
				b.Compile(tc.n, tc.ratio, tc.step).Run(got, scale, 0)
				require.Equal(t, want, got)
			})
		}
	}
}

// A log-derivative style constraint, a product of Horner-form random linear
// combinations each used twice, must be lowered without any extension product
// inside the linear combinations and with each combination computed once.
func TestFoldsHornerForms(t *testing.T) {
	g := newTreeGen(rand.New(rand.NewPCG(7, 7)), 64, 64)
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

// A padded leaf must read row (i + offset) mod n of its padded coset: data at
// rows [start, start+len(data)), the padding value elsewhere, over data placed
// at the start or the end of the coset or overflowing it, and offsets that
// wrap within or across blocks.
func TestPaddedLeavesMatchNaive(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 8))
	for _, n := range []int{8, BlockSize, 3*BlockSize + 40} {
		for _, layout := range []struct{ dataLen, start int }{
			{n, 0}, {n / 2, 0}, {n / 2, n - n/2}, {n + 3, -3}, {0, 0},
		} {
			t.Run(fmt.Sprintf("n=%d/len=%d/start=%d", n, layout.dataLen, layout.start), func(t *testing.T) {
				data := field.VecPseudoRandBase(rng, layout.dataLen)
				dataExt := field.VecPseudoRandExt(rng, layout.dataLen)
				pad := field.VecPseudoRandBase(rng, 1)[0]
				row := func(p int) (field.Element, field.Ext) {
					if j := p - layout.start; j >= 0 && j < layout.dataLen {
						return data[j], dataExt[j]
					}
					return pad, field.Lift(pad)
				}
				for _, offset := range []int{0, 1, n - 1, n / 3} {
					b := NewBuilder()
					gotBase, gotExt := make([]field.Ext, n), make([]field.Ext, n)
					b.Store(b.BasePadded(data, pad, layout.start, offset), gotBase)
					b.Store(b.ExtPadded(dataExt, pad, layout.start, offset), gotExt)
					b.Compile(n, 1, 1).Run(nil, nil, 3)
					for i := range n {
						wb, we := row((i + offset) % n)
						require.Equal(t, field.Lift(wb), gotBase[i], "base, offset %d, row %d", offset, i)
						require.Equal(t, we, gotExt[i], "ext, offset %d, row %d", offset, i)
					}
				}
			})
		}
	}
}

// The interleaved-chain batch inversion must equal one inversion per element,
// zeros mapping to zero, for lengths around and across the lane width.
func TestBatchInvertExtMatchesInverse(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 22))
	for _, n := range []int{0, 1, 5, invLanes - 1, invLanes, invLanes + 1, 3*invLanes + 7, BlockSize} {
		a := field.VecPseudoRandExt(rng, n)
		for i := 0; i < n; i += 4 {
			a[i] = field.Ext{}
		}
		dst, prefix := make([]field.Ext, n), make([]field.Ext, n)
		batchInvertExt(a, dst, prefix)
		for i := range a {
			var want field.Ext
			want.Inverse(&a[i])
			require.Equal(t, want, dst[i], "n=%d, element %d", n, i)
		}
	}
}

// Hash-consing must merge only identical nodes: a padded leaf differs from
// another over the same data by its start, its padding or its offset, and a
// linear form from another over the same atoms by a coefficient.
func TestInternSeparatesNodesThatDiffer(t *testing.T) {
	b := NewBuilder()
	data := make([]field.Element, 4)
	var pad, pad2 field.Element
	pad2.SetUint64(2)

	leaf := b.BasePadded(data, pad, 0, 0)
	require.Equal(t, leaf, b.BasePadded(data, pad, 0, 0), "an identical leaf is merged")
	others := []int{
		b.BasePadded(data, pad, 1, 0),
		b.BasePadded(data, pad2, 0, 0),
		b.BasePadded(data, pad, 0, 1),
		b.Base(data, 0),
	}
	seen := map[int]bool{leaf: true}
	for i, id := range others {
		require.False(t, seen[id], "leaf %d was merged with a different one", i)
		seen[id] = true
	}

	x, y := b.Base(data, 0), b.Base(data, 1)
	s1, s2 := field.Ext{}, field.Ext{}
	s1.B0.A0.SetUint64(3)
	s2.B0.A0.SetUint64(5)
	s1.B1.A0.SetUint64(1) // extension coefficients, so the sums are linear forms
	s2.B1.A0.SetUint64(1)
	form := func(a, c field.Ext) int {
		return b.Op(Add, b.Op(Mul, b.Scalar(a, false), x), b.Op(Mul, b.Scalar(c, false), y))
	}
	first, again := form(s1, s2), form(s1, s2)
	require.Equal(t, kLinear, b.nodes[first].kind)
	require.Equal(t, first, again, "an identical linear form is merged")
	require.NotEqual(t, first, form(s2, s1), "linear forms with different coefficients are distinct")
}

// A program built once over parameters and slotted tables must give, on every
// run, exactly what a program built with those values as constants and those
// tables as leaves gives: accumulate sinks with a parameter-dependent
// coefficient and a slotted cancellation, Horner forms with parameter
// coefficients, subtrees over parameters only, and parameters equal to zero
// included.
func TestParamsAndSlotsMatchConstants(t *testing.T) {
	const n, ratio = 3*BlockSize + 5, 2
	rng := rand.New(rand.NewPCG(11, 13))
	g := newTreeGen(rng, n, n*ratio)

	// build lowers the same trees with every scalar leaf either a constant
	// (params == nil) or Params[i], and every table either a direct leaf
	// (slotted == false) or a slot.
	type leafRef struct {
		base []field.Element
		ext  []field.Ext
	}
	var scalarLeaves []*tree
	var tableLeaves []leafRef
	var collect func(tr *tree)
	collect = func(tr *tree) {
		switch {
		case tr.s != nil:
			scalarLeaves = append(scalarLeaves, tr)
		case tr.base != nil || tr.ext != nil:
			tableLeaves = append(tableLeaves, leafRef{tr.base, tr.ext})
		}
		for _, a := range tr.args {
			collect(a)
		}
	}
	trees := make([]*tree, 6)
	for i := range trees {
		trees[i] = g.tree(5)
		collect(trees[i])
	}
	scalarIdx := map[*tree]int{}
	for i, tr := range scalarLeaves {
		scalarIdx[tr] = i
	}
	baseSlot, extSlot := map[*field.Element]int{}, map[*field.Ext]int{}
	for _, l := range tableLeaves {
		if l.base != nil {
			if _, ok := baseSlot[&l.base[0]]; !ok {
				baseSlot[&l.base[0]] = len(baseSlot)
			}
		} else if _, ok := extSlot[&l.ext[0]]; !ok {
			extSlot[&l.ext[0]] = len(extSlot)
		}
	}

	var build func(b *Builder, tr *tree, params bool) int
	build = func(b *Builder, tr *tree, params bool) int {
		switch {
		case tr.s != nil && params:
			return b.Param(scalarIdx[tr], tr.isBase)
		case tr.s != nil:
			return b.Scalar(*tr.s, tr.isBase)
		case tr.base != nil && params:
			return b.BaseSlot(baseSlot[&tr.base[0]], tr.offset)
		case tr.base != nil:
			return b.Base(tr.base, tr.offset)
		case tr.ext != nil && params:
			return b.ExtSlot(extSlot[&tr.ext[0]], tr.offset)
		case tr.ext != nil:
			return b.Ext(tr.ext, tr.offset)
		}
		args := make([]int, len(tr.args))
		for k, a := range tr.args {
			args[k] = build(b, a, params)
		}
		return b.Op(tr.op, args...)
	}
	// coin is the accumulation coefficient: Params[len(scalarLeaves)].
	coinSlot := len(scalarLeaves)
	run := func(p *Program, bind *Bindings) []field.Ext {
		agg := make([]field.Ext, n*ratio)
		p.RunWith(bind, agg, nil, 0)
		return agg
	}

	// Every other sink is cancelled by the table Base[cancelSlot].
	cancelSlot := len(baseSlot)
	cancelOf := func(i int) int {
		if i%2 == 1 {
			return cancelSlot
		}
		return -1
	}

	b := NewBuilder()
	coinPow := b.Scalar(field.Lift(field.One()), true)
	coin := b.Param(coinSlot, false)
	for i, tr := range trees {
		b.AccumulateBy(build(b, tr, true), cancelOf(i), coinPow)
		coinPow = b.Op(Mul, coinPow, coin)
	}
	cached := b.Compile(n, ratio, 1)

	// Run 1 sets the first scalar parameters to zero, run 2 the coin.
	for runIdx := range 3 {
		// Fresh values for every parameter and table.
		bind := &Bindings{Params: make([]field.Ext, coinSlot+1)}
		for i, tr := range scalarLeaves {
			if tr.isBase {
				*tr.s = field.Lift(field.VecPseudoRandBase(rng, 1)[0])
			} else {
				*tr.s = field.VecPseudoRandExt(rng, 1)[0]
			}
			if runIdx == 1 && i < 2 {
				*tr.s = field.Ext{}
			}
			bind.Params[i] = *tr.s
		}
		coinVal := field.VecPseudoRandExt(rng, 1)[0]
		if runIdx == 2 {
			coinVal = field.Ext{}
		}
		bind.Params[coinSlot] = coinVal
		cancellation := field.VecPseudoRandBase(rng, n*ratio)
		bind.Base = make([][]field.Element, len(baseSlot)+1)
		bind.Base[cancelSlot] = cancellation
		bind.Ext = make([][]field.Ext, len(extSlot))
		for _, tbl := range g.base {
			copy(tbl, field.VecPseudoRandBase(rng, len(tbl)))
			if s, ok := baseSlot[&tbl[0]]; ok {
				bind.Base[s] = tbl
			}
		}
		for _, tbl := range g.ext {
			copy(tbl, field.VecPseudoRandExt(rng, len(tbl)))
			if s, ok := extSlot[&tbl[0]]; ok {
				bind.Ext[s] = tbl
			}
		}

		fresh := NewBuilder()
		var pow field.Ext
		pow.SetOne()
		for i, tr := range trees {
			var cancel []field.Element
			if cancelOf(i) >= 0 {
				cancel = cancellation
			}
			fresh.Accumulate(build(fresh, tr, false), cancel, pow)
			pow.Mul(&pow, &coinVal)
		}
		want := run(fresh.Compile(n, ratio, 1), nil)
		require.Equal(t, want, run(cached, bind), "run %d", runIdx)
	}
}

// A program reading a parameter or a slotted table cannot run without
// bindings, nor any program accumulating into a short aggregate.
func TestRunRejectsMissingBindings(t *testing.T) {
	const n = 8
	b := NewBuilder()
	b.AccumulateBy(b.Op(Mul, b.Param(0, false), b.BaseSlot(0, 0)), -1, b.Scalar(field.Lift(field.One()), true))
	p := b.Compile(n, 1, 1)
	require.Panics(t, func() { p.Run(make([]field.Ext, n), nil, 1) }, "no bindings")
	bind := &Bindings{Base: [][]field.Element{make([]field.Element, n)}, Params: make([]field.Ext, 1)}
	require.Panics(t, func() { p.RunWith(bind, make([]field.Ext, n-1), nil, 1) }, "short aggregate")
	require.NotPanics(t, func() { p.RunWith(bind, make([]field.Ext, n), nil, 1) })
}
