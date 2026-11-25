package recursion

import (
	"fmt"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// orderCircuit checks the canonical order of G groups of heights up to H, the
// heights and the order its inputs (in that order).
func orderCircuit(G, H int) *Machine {
	m := &Machine{}
	table := func() Table {
		cells := make([]int, G)
		for i := range cells {
			cells[i] = m.InputScalar()
		}
		return m.Table(cells)
	}
	heights := table()
	m.AssertCanonicalOrder(table(), heights, H)
	return m
}

func orderInputs(heights, order []int) []Cell {
	res := make([]Cell, 0, len(heights)+len(order))
	for _, x := range append(slices.Clone(heights), order...) {
		res = append(res, ScalarCell(field.NewElement(uint64(x))))
	}
	return res
}

// TestCanonicalOrderExhaustive: for 3 groups of heights up to 2, every height
// assignment and every order of indices below 3, the proof verifies iff the
// order is the canonical one.
func TestCanonicalOrderExhaustive(t *testing.T) {
	const G, H = 3, 2
	p, err := orderCircuit(G, H).Compile(testPCSOpts...)
	require.NoError(t, err)
	tuples := func(n, base int) [][]int {
		res := make([][]int, 0, pow(base, n))
		for x := range pow(base, n) {
			tu := make([]int, n)
			for i := range tu {
				tu[i], x = x%base, x/base
			}
			res = append(res, tu)
		}
		return res
	}
	for _, hs := range tuples(G, H+1) {
		canonical := canonicalOrder(hs)
		for _, order := range tuples(G, G) {
			r, err := p.Execute(orderInputs(hs, order))
			require.NoError(t, err)
			err = proveAndVerify(p, r)
			if slices.Equal(order, canonical) {
				require.NoError(t, err, "heights %v, order %v", hs, order)
			} else {
				require.Error(t, err, "heights %v, order %v accepted", hs, order)
			}
		}
	}
}

func pow(b, n int) int {
	r := 1
	for range n {
		r *= b
	}
	return r
}

// TestCanonicalOrder: the extreme steps (from height H, group 0, to height 0,
// group G − 1, the largest honest step), and a single group.
func TestCanonicalOrder(t *testing.T) {
	for _, tc := range []struct {
		G, H    int
		heights []int
	}{
		{2, 8, []int{8, 0}},
		{2, 8, []int{0, 8}},
		{5, 30, []int{30, 0, 30, 0, 17}},
		{1, 3, []int{2}},
	} {
		t.Run(fmt.Sprint(tc.heights), func(t *testing.T) {
			p, err := orderCircuit(tc.G, tc.H).Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(orderInputs(tc.heights, canonicalOrder(tc.heights)))
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r))
		})
	}
}

// TestCanonicalOrderRejects: an order with a group out of the table does not
// execute (its height lookup fails), and its proof would not verify.
func TestCanonicalOrderRejects(t *testing.T) {
	p, err := orderCircuit(3, 4).Compile(testPCSOpts...)
	require.NoError(t, err)
	_, err = p.Execute(orderInputs([]int{4, 2, 1}, []int{0, 1, 3}))
	require.Error(t, err)

	single, err := orderCircuit(1, 4).Compile(testPCSOpts...)
	require.NoError(t, err)
	_, err = single.Execute(orderInputs([]int{2}, []int{1}))
	require.Error(t, err)
}

// TestCanonicalOrderBounds: the steps' sums must stay below the modulus.
func TestCanonicalOrderBounds(t *testing.T) {
	build := func(G, H int) func() { return func() { orderCircuit(G, H) } }
	require.NotPanics(t, build(256, field.Bits-1))
	require.Panics(t, build(1<<14, field.Bits-1))
	require.Panics(t, build(2, field.Bits))
	require.Panics(t, func() {
		m := &Machine{}
		a, b := m.Table([]int{m.InputScalar()}), m.Table([]int{m.InputScalar(), m.InputScalar()})
		m.AssertCanonicalOrder(a, b, 3)
	})
}

// TestDynamicPathWithOrderCheck: the order check rejects what the dynamic path
// alone accepts, a repeated group with its root recomputed
// (TestDynamicPathOrderIsTheCallers), and accepts the trees of every shape.
func TestDynamicPathWithOrderCheck(t *testing.T) {
	m := &Machine{}
	dpathCircuitOn(m)
	m.AssertCanonicalOrder(Table{tag: dpOrderTable, n: dpG}, Table{tag: dpHeightTable, n: dpG}, dpH)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)

	for i, hs := range [][]int{{5, 3, 3, 2}, {8, 8, 8, 8}, {3, 0, 2, 0}, {0, 0, 0, 0}} {
		r, err := p.Execute(dpathTree(t, hs, uint64(i)).cells())
		require.NoError(t, err, "heights %v", hs)
		require.NoError(t, proveAndVerify(p, r), "heights %v", hs)
	}

	for name, order := range map[string][]int{
		"repeated group":        {0, 1, 1, 3},
		"equal heights swapped": {0, 2, 1, 3},
	} {
		t.Run(name, func(t *testing.T) {
			v := dpathTree(t, []int{5, 3, 3, 2}, 7)
			v.order = order
			r, err := p.Execute(v.cells())
			require.NoError(t, err)
			rows := r.dpathRows[0]
			v.root = rows[len(rows)-1].out // the root of that order: the path alone accepts it
			r, err = p.Execute(v.cells())
			require.NoError(t, err)
			require.Error(t, proveAndVerify(p, r), "non-canonical order accepted")
		})
	}
}
