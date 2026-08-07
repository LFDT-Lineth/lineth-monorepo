package fri

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

func TestNewTree_MatchesScalarReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bottomLog int
		auxLevel  func(int) bool
	}{
		{bottomLog: 0, auxLevel: func(int) bool { return false }},
		{bottomLog: 1, auxLevel: func(int) bool { return false }},
		{bottomLog: 4, auxLevel: func(int) bool { return true }},
		{bottomLog: 5, auxLevel: func(level int) bool { return level%2 == 0 }},
		// The 2^10 bottom reaches the parallel path at its first internal level.
		{bottomLog: 10, auxLevel: func(level int) bool { return level%3 == 0 }},
	}

	for caseID, test := range tests {
		name := fmt.Sprintf("bottom=2^%d/case=%d", test.bottomLog, caseID)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			prng := rand.New(utils.NewRandSource(int64(100 + caseID)))
			leaves := make([][]field.Octuplet, test.bottomLog+1)
			for level := range leaves {
				if level != test.bottomLog && !test.auxLevel(level) {
					continue
				}
				leaves[level] = make([]field.Octuplet, 1<<level)
				for i := range leaves[level] {
					// Exercise zero field elements alongside deterministic random
					// values without making an entire tree trivially uniform.
					if (i+level)%5 != 0 || len(leaves[level]) == 1 {
						leaves[level][i] = field.PseudoRandOctuplet(prng)
					}
				}
			}

			want := newTreeScalarReference(leaves)
			got := NewTree(leaves)
			if !slices.Equal(got.Nodes, want.Nodes) {
				t.Fatal("parallel tree nodes differ from the scalar reference")
			}
			assertEqualAux(t, got.Aux, want.Aux)

			for _, idx := range sampledLeafIndices(got.NumLeaves()) {
				branch := got.OpenBranch(idx)
				root, err := branch.RecoverRoot(idx)
				if err != nil {
					t.Fatalf("leaf %d: recover root: %v", idx, err)
				}
				if root != got.Root() {
					t.Fatalf("leaf %d: recovered root differs from tree root", idx)
				}
			}
		})
	}
}

// TestBuildTreeExt_MatchesScalarReference covers the auxiliary-free complete
// binary tree path (nil upperLeaves in buildLevels) against the scalar
// reference, across sizes spanning the batch and parallel thresholds.
func TestBuildTreeExt_MatchesScalarReference(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 4, 8, 16, 32, 1024} {
		t.Run(fmt.Sprintf("leaves=%d", n), func(t *testing.T) {
			t.Parallel()

			prng := rand.New(utils.NewRandSource(int64(n)))
			leaves := make([]field.Ext, n)
			for i := range leaves {
				leaves[i] = field.PseudoRandExt(prng)
			}

			refLeaves := make([][]field.Octuplet, utils.Log2Ceil(n)+1)
			refLeaves[len(refLeaves)-1] = mapExtToOctuplet(leaves)
			want := newTreeScalarReference(refLeaves)
			got := buildTreeExt(leaves)
			if !slices.Equal(got.Nodes, want.Nodes) {
				t.Fatal("parallel tree nodes differ from the scalar reference")
			}
			assertEqualAux(t, got.Aux, want.Aux)
		})
	}
}

func newTreeScalarReference(leaves [][]field.Octuplet) *Tree {
	bottom := len(leaves) - 1
	n := len(leaves[bottom])
	nodes := make([]field.Octuplet, 2*n-1)
	aux := make([]*field.Octuplet, n-1)
	copy(nodes[n-1:], leaves[bottom])

	for level := bottom - 1; level >= 0; level-- {
		levelSize := 1 << level
		levelStart := levelSize - 1
		for j := range levelSize {
			k := levelStart + j
			if len(leaves[level]) != 0 {
				aux[k] = &leaves[level][j]
			}
			nodes[k] = hashNode(nodes[2*k+1], nodes[2*k+2], aux[k])
		}
	}
	return &Tree{Nodes: nodes, Aux: aux}
}

// mapExtToOctuplet converts extension elements into zero-padded octuplets, the
// leaf encoding buildTreeExt writes into the tree bottom.
func mapExtToOctuplet(exts []field.Ext) []field.Octuplet {
	res := make([]field.Octuplet, len(exts))
	for i := range exts {
		limbs := extLimbs(exts[i])
		copy(res[i][:], limbs[:])
	}
	return res
}

func assertEqualAux(t *testing.T, got, want []*field.Octuplet) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("aux length=%d, want %d", len(got), len(want))
	}
	for i := range got {
		if (got[i] == nil) != (want[i] == nil) {
			t.Fatalf("aux[%d] nil mismatch", i)
		}
		if got[i] != nil && *got[i] != *want[i] {
			t.Fatalf("aux[%d] value mismatch", i)
		}
	}
}

func sampledLeafIndices(n int) []int {
	if n == 1 {
		return nil
	}
	indices := []int{0, n / 2, n - 1}
	if n <= 32 {
		indices = make([]int, n)
		for i := range indices {
			indices[i] = i
		}
	}
	return indices
}

// TestOctupletExtRoundTrip checks that mapExtToOctuplet pads an extension element
// into the low six octuplet coordinates (leaving 6 and 7 zero) and that
// octupletToExt is its exact inverse.
func TestOctupletExtRoundTrip(t *testing.T) {

	prng := rand.New(utils.NewRandSource(42))
	cases := []field.Ext{
		field.IntsToExt(0, 0, 0, 0, 0, 0),
		field.IntsToExt(1, 2, 3, 4, 5, 6),
		field.IntsToExt(7, 0, 0, 0, 0, 11),
		field.PseudoRandExt(prng),
		field.PseudoRandExt(prng),
	}

	octs := mapExtToOctuplet(cases)
	if len(octs) != len(cases) {
		t.Fatalf("mapExtToOctuplet returned %d octuplets, want %d", len(octs), len(cases))
	}

	for i, e := range cases {
		o := octs[i]
		if !o[6].IsZero() || !o[7].IsZero() {
			t.Fatalf("case %d: padding coords must be zero, got [6]=%s [7]=%s",
				i, o[6].String(), o[7].String())
		}
		back, err := octupletToExt(o)
		if err != nil {
			t.Fatalf("case %d: octupletToExt: %v", i, err)
		}
		if !back.Equal(&e) {
			t.Fatalf("case %d: round-trip mismatch: got %s want %s", i, back.String(), e.String())
		}
	}
}

// TestBuildTreeExtOpenRecover checks the Merkle tree round-trip across several
// sizes: every leaf opens to a branch whose recovered root matches the tree
// root, the opened leaf and its deepest sibling are the adjacent (conjugate)
// pair, and tampering the leaf breaks recovery.
func TestBuildTreeExtOpenRecover(t *testing.T) {

	prng := rand.New(utils.NewRandSource(7))

	for _, n := range []int{2, 4, 8, 16} {

		leaves := make([]field.Ext, n)
		for i := range leaves {
			leaves[i] = field.PseudoRandExt(prng)
		}
		octs := mapExtToOctuplet(leaves)
		tree := buildTreeExt(leaves)

		if tree.NumLeaves() != n {
			t.Fatalf("n=%d: NumLeaves=%d, want %d", n, tree.NumLeaves(), n)
		}

		root := tree.Root()
		for idx := 0; idx < n; idx++ {

			branch := tree.OpenBranch(idx)

			if branch.Leaf != octs[idx] {
				t.Fatalf("n=%d idx=%d: opened leaf does not match leaves[idx]", n, idx)
			}
			last := len(branch.Siblings) - 1
			if last < 0 || branch.Siblings[last] != octs[idx^1] {
				t.Fatalf("n=%d idx=%d: deepest sibling is not the adjacent leaf idx^1", n, idx)
			}

			got, err := branch.RecoverRoot(idx)
			if err != nil {
				t.Fatalf("n=%d idx=%d: RecoverRoot: %v", n, idx, err)
			}
			if got != root {
				t.Fatalf("n=%d idx=%d: recovered root != tree root", n, idx)
			}

			// Tampering the leaf must break recovery.
			bad := branch
			bad.Leaf = field.PseudoRandOctuplet(prng)
			if tampered, _ := bad.RecoverRoot(idx); tampered == root {
				t.Fatalf("n=%d idx=%d: tampered leaf still recovers the root", n, idx)
			}
		}
	}
}
