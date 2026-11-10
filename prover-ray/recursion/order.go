package recursion

import (
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// AssertCanonicalOrder checks that the table order lists the groups of a tree
// in its canonical order: the order fri.Tree chains them in, which
// DynamicPath.Order must be and the dpath chip does not check. order[k] = π_k
// is a group, heights[g] the height of group g, and the order is the groups
// by decreasing height, equal heights by increasing index. It is a check per
// tree: every query's dynamic path into the tree reads the same tables.
//
// It takes the keys key_k = π_k − G·h_k, h_k = heights[π_k], and checks that
// they strictly increase: key_(k+1) − key_k − 1 is below 2^b, for the b of
// the honest steps (up to (H + 1)·G − 2, H = maxHeight). Reading heights at
// π_k bounds π_k below G (an index out of the table matches no entry).
//
// The keys of two equal π are equal, both reading the same height, so the π_k
// are distinct: the sum of j − i steps, key_j − key_i, is in [1, (G − 1)·2^b],
// below the modulus, so nonzero. Being G of them below G, they are a
// permutation of the groups. The keys order the groups canonically for
// heights in [0, H]: a dynamic path into the tree checks that, as it reads
// the height of every group against its own height, in [0, H].
//
// The cost is 4 E6 rows, a lookup and a decomposition (BitsWidth rows) per
// step, and one lookup and two rows for the first group.
func (m *Machine) AssertCanonicalOrder(order, heights Table, maxHeight int) {
	m.checkTable(order)
	m.checkTable(heights)
	G := order.Len()
	if heights.Len() != G {
		panic(fmt.Sprintf("AssertCanonicalOrder: %d groups ordered, %d heights", G, heights.Len()))
	}
	if maxHeight < 0 || maxHeight >= field.Bits {
		panic(fmt.Sprintf("AssertCanonicalOrder: height %d", maxHeight))
	}
	src := m.tables[order.tag].src
	if G == 1 {
		m.Lookup(heights, src[0]) // π_0 = 0
		return
	}
	b := max(bits.Len(uint((maxHeight+1)*G-2)), 1)
	if b >= BitsWidth || uint64(G-1)<<b >= field.Modulus().Uint64() {
		panic(fmt.Sprintf("AssertCanonicalOrder: %d groups, height %d: the steps' sums overflow", G, maxHeight))
	}

	g, one := m.Const(ScalarCell(field.NewElement(uint64(G)))), m.Const(ScalarCell(field.One()))
	key := func(k int) int {
		h := m.Lookup(heights, src[k])
		return m.Sub(src[k], m.Mul(g, h))
	}
	prev := key(0)
	for k := 1; k < G; k++ {
		next := key(k)
		d := m.Sub(m.Sub(next, prev), one)
		m.AssertEqE6(d, m.Bits(d, b).Shr[0])
		prev = next
	}
}
