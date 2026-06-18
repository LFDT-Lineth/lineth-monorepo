package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

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
