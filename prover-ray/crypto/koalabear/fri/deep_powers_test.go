package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// moduleShape returns a one-batch grouped shape of the given sizes: module A
// (ordinal 0: one base row, one extension row), module C (ordinal 1: two base
// rows) and module B (ordinal 2: one base row), at sizes sizeA, sizeC, sizeB.
// Within a size, the groups come in ordinal order, as the PCS compiler builds
// them.
func moduleShape(sizeA, sizeC, sizeB int) Shape {
	shape := make(Shape, max(sizeA, sizeC, sizeB)+1)
	add := func(size int, g GroupShape) {
		shape[size].BaseWidth += g.BaseWidth
		shape[size].ExtWidth += g.ExtWidth
		shape[size].Groups = append(shape[size].Groups, g)
	}
	add(sizeA, GroupShape{BaseWidth: 1, ExtWidth: 1, Ordinal: 0})
	add(sizeC, GroupShape{BaseWidth: 2, Ordinal: 1})
	add(sizeB, GroupShape{BaseWidth: 1, Ordinal: 2})
	return shape
}

// TestDeepPowersGroupedSizeOblivious: the powers of a grouped commitment follow
// the module order, whatever the sizes. Module B keeps its power when it moves
// from A's size to C's, the running power stays G, and the modules sharing a
// size get disjoint powers.
func TestDeepPowersGroupedSizeOblivious(t *testing.T) {
	// B at A's size: row 0 of size 3 is A's base row, row 1 is B's
	d := newDeepPowers([]Shape{moduleShape(3, 2, 3)})
	require.Equal(t, 0, d.column(0, 3, false, 0), "A's base row")
	require.Equal(t, 1, d.column(0, 3, true, 0), "A's extension row")
	require.Equal(t, 2, d.column(0, 2, false, 0), "C's first row")
	require.Equal(t, 3, d.column(0, 2, false, 1), "C's second row")
	require.Equal(t, 4, d.column(0, 3, false, 1), "B's row")
	require.Equal(t, 5, d.running[3], "running power G = 5 columns")
	require.Equal(t, 5, d.running[2])

	// B at C's size: row 2 of size 2 is B's
	d = newDeepPowers([]Shape{moduleShape(3, 2, 2)})
	require.Equal(t, 0, d.column(0, 3, false, 0))
	require.Equal(t, 1, d.column(0, 3, true, 0))
	require.Equal(t, 2, d.column(0, 2, false, 0))
	require.Equal(t, 3, d.column(0, 2, false, 1))
	require.Equal(t, 4, d.column(0, 2, false, 2), "B keeps its power at another size")
	require.Equal(t, 5, d.running[3])
	require.Equal(t, 5, d.running[2])
}

// TestDeepPowersGroupedBatches: a grouped commitment numbers its batches in
// order, the second one after all the columns of the first.
func TestDeepPowersGroupedBatches(t *testing.T) {
	d := newDeepPowers([]Shape{moduleShape(3, 2, 3), moduleShape(2, 2, 3)})
	require.Equal(t, 5, d.column(1, 2, false, 0), "batch 1, A's base row")
	require.Equal(t, 9, d.column(1, 3, false, 0), "batch 1, B's row")
	require.Equal(t, 10, d.running[2], "running power G = 10 columns")
}

// TestDeepPowersTies: groups of equal ordinals are numbered by decreasing
// size, then in their size's order, and a size without groups counts as one
// group of ordinal 0.
func TestDeepPowersTies(t *testing.T) {
	shape := Shape{
		{BaseWidth: 1}, // size 0: no groups
		{BaseWidth: 2, Groups: []GroupShape{{BaseWidth: 1}, {BaseWidth: 1}}}, // size 1: two groups of ordinal 0
	}
	d := newDeepPowers([]Shape{shape})
	require.Equal(t, 0, d.column(0, 1, false, 0))
	require.Equal(t, 1, d.column(0, 1, false, 1))
	require.Equal(t, 2, d.column(0, 0, false, 0))
	require.Equal(t, 3, d.running[0])
}

// TestPCSVerifyRejectsCompensatingClaims: in the grouped fixture, size 1 holds
// two groups (a base column, then an extension column) opened at the same point
// zeta. Raising one group's claim by delta and lowering the other's by delta
// would cancel in the DEEP combination if the two columns shared a power; with
// distinct powers, the verifier rejects it.
func TestPCSVerifyRejectsCompensatingClaims(t *testing.T) {
	fx := newGroupedPCSFixture(t, 1)
	sized := fx.input.ClaimedValues[0][1]
	require.Len(t, sized.Base, 1, "size 1 has one base column")
	require.Len(t, sized.Ext, 1, "and one extension column")

	delta := field.UintsToExt(3, 1, 4, 1, 5, 9)
	sized.Base[0][0].Add(&sized.Base[0][0], &delta)
	sized.Ext[0][0].Sub(&sized.Ext[0][0], &delta)
	require.Error(t, fx.pcs.Verify(fx.input, fx.proof), "compensating claims of two groups accepted")
}
