package fri

import (
	"fmt"
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
// the module order, whatever the sizes. For every placement of the three
// modules on sizes 0 to 3, each module keeps its powers (A: 0 and 1, C: 2 and 3,
// B: 4), the running power of every level is G = 5, and the powers of a level
// are distinct and below its running power.
func TestDeepPowersGroupedSizeOblivious(t *testing.T) {
	for sizeA := range 4 {
		for sizeC := range 4 {
			for sizeB := range 4 {
				shape := moduleShape(sizeA, sizeC, sizeB)
				d, err := newDeepPowers([]Shape{shape})
				require.NoError(t, err)

				// rows of a module at its size: the groups before it, in
				// ordinal order, come first
				baseBefore := func(size, ordinal int) int {
					n := 0
					for _, g := range shape[size].Groups {
						if g.Ordinal < ordinal {
							n += g.BaseWidth
						}
					}
					return n
				}
				msg := fmt.Sprintf("A at %d, C at %d, B at %d", sizeA, sizeC, sizeB)
				require.Equal(t, 0, d.column(0, sizeA, false, baseBefore(sizeA, 0)), msg)
				require.Equal(t, 1, d.column(0, sizeA, true, 0), msg)
				require.Equal(t, 2, d.column(0, sizeC, false, baseBefore(sizeC, 1)), msg)
				require.Equal(t, 3, d.column(0, sizeC, false, baseBefore(sizeC, 1)+1), msg)
				require.Equal(t, 4, d.column(0, sizeB, false, baseBefore(sizeB, 2)), msg)

				for s, sized := range shape {
					require.Equal(t, 5, d.running[s], msg)
					seen := map[int]bool{}
					for row := range sized.BaseWidth {
						seen[d.column(0, s, false, row)] = true
					}
					for row := range sized.ExtWidth {
						seen[d.column(0, s, true, row)] = true
					}
					require.Len(t, seen, sized.BaseWidth+sized.ExtWidth, "%s: distinct powers at size %d", msg, s)
					for p := range seen {
						require.Less(t, p, d.running[s], msg)
					}
				}
			}
		}
	}
}

// TestDeepPowersGroupedBatches: a grouped commitment numbers its batches in
// order, the second one after all the columns of the first.
func TestDeepPowersGroupedBatches(t *testing.T) {
	d, err := newDeepPowers([]Shape{moduleShape(3, 2, 3), moduleShape(2, 2, 3)})
	require.NoError(t, err)
	require.Equal(t, 5, d.column(1, 2, false, 0), "batch 1, A's base row")
	require.Equal(t, 9, d.column(1, 3, false, 0), "batch 1, B's row")
	require.Equal(t, 10, d.running[2], "running power G = 10 columns")
}

// TestDeepPowersRejectsAmbiguousGroups: a grouped commitment whose numbering
// would depend on the sizes is rejected: a size with columns but no groups
// (in the grouped batch or in another one), or two groups of one ordinal.
func TestDeepPowersRejectsAmbiguousGroups(t *testing.T) {
	ungroupedSize := moduleShape(1, 1, 1)
	ungroupedSize = append(ungroupedSize, SizedShape{BaseWidth: 1})
	_, err := newDeepPowers([]Shape{ungroupedSize})
	require.ErrorContains(t, err, "batch 0 size 2 declares no groups")

	_, err = newDeepPowers([]Shape{moduleShape(1, 0, 1), ungrouped(moduleShape(1, 0, 1))})
	require.ErrorContains(t, err, "batch 1 size 0 declares no groups", "a grouped and an ungrouped batch")

	dup := moduleShape(1, 0, 0)
	dup[0].Groups[1].Ordinal = 0
	_, err = newDeepPowers([]Shape{dup})
	require.ErrorContains(t, err, "two groups of ordinal 0")

	_, err = newDeepPowers([]Shape{moduleShape(1, 0, 1), moduleShape(0, 1, 0)})
	require.NoError(t, err, "batches may reuse each other's ordinals")
}

// TestPCSVerifyRejectsCompensatingClaims: in the grouped fixture, size 1 holds
// two groups (a base column, then an extension column) opened at the same point
// zeta. Raising one group's claim by delta and lowering the other's by delta
// would cancel in the DEEP combination if the two columns shared a power; the
// verifier rejects it. This guards the numbering against regressions (a
// per-group restart of the powers would accept it); the per-size numbering
// before module ordinals rejected it too.
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
