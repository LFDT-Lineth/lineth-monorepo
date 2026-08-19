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

func TestMerkleCapAuthenticatesBranches(t *testing.T) {
	prng := rand.New(utils.NewRandSource(19))
	leaves := field.VecPseudoRandExt(prng, 16)
	tree := buildTreeExt(leaves)
	height := tree.NumLevel() - 1

	for depth := 1; depth < height; depth++ {
		cap := tree.OpenCap(depth)
		if err := cap.Authenticate(depth, tree.Root()); err != nil {
			t.Fatalf("depth %d: authenticate cap: %v", depth, err)
		}
		for idx := range leaves {
			branch := tree.OpenBranchToDepth(idx, depth)
			if err := branch.AuthenticateToCap(idx, cap.Nodes); err != nil {
				t.Fatalf("depth %d index %d: authenticate branch: %v", depth, idx, err)
			}
		}
	}
}

func TestMerkleCapAuthenticatesAuxiliaryPrefix(t *testing.T) {
	prng := rand.New(utils.NewRandSource(23))
	tree := NewTree([][]field.Octuplet{
		pseudoRandOctuplets(prng, 1),
		nil,
		pseudoRandOctuplets(prng, 4),
	})
	cap := tree.OpenCap(2)
	if err := cap.Authenticate(2, tree.Root()); err != nil {
		t.Fatalf("authenticate cap: %v", err)
	}
	if cap.Aux[0] == nil {
		t.Fatal("cap omitted root auxiliary digest")
	}

	// Cap auxiliary values must not alias the committed tree.
	original := tree.Root()
	*cap.Aux[0] = field.PseudoRandOctuplet(prng)
	if tree.Root() != original {
		t.Fatal("mutating cap auxiliary digest changed tree root")
	}
	if err := cap.Authenticate(2, tree.Root()); err == nil {
		t.Fatal("tampered cap authenticated")
	}
}

func pseudoRandOctuplets(prng *rand.Rand, n int) []field.Octuplet {
	values := make([]field.Octuplet, n)
	for i := range values {
		values[i] = field.PseudoRandOctuplet(prng)
	}
	return values
}

func TestMerkleCapRejectsMalformedAndTamperedProofs(t *testing.T) {
	prng := rand.New(utils.NewRandSource(29))
	tree := buildTreeExt(field.VecPseudoRandExt(prng, 8))
	cap := tree.OpenCap(2)

	badLength := cap
	badLength.Nodes = badLength.Nodes[:len(badLength.Nodes)-1]
	if err := badLength.Validate(2); err == nil {
		t.Fatal("truncated cap accepted")
	}
	if err := (MerkleCap{Nodes: []field.Octuplet{{}}}).Validate(0); err == nil {
		t.Fatal("nonempty depth-zero cap accepted")
	}

	badNode := cap
	badNode.Nodes = append([]field.Octuplet(nil), cap.Nodes...)
	badNode.Nodes[0] = field.PseudoRandOctuplet(prng)
	if err := badNode.Authenticate(2, tree.Root()); err == nil {
		t.Fatal("tampered cap node authenticated")
	}

	branch := tree.OpenBranchToDepth(3, 2)
	branch.Siblings[0] = field.PseudoRandOctuplet(prng)
	if err := branch.AuthenticateToCap(3, cap.Nodes); err == nil {
		t.Fatal("tampered branch authenticated")
	}
}

func TestMerkleCapDepth(t *testing.T) {
	tests := []struct {
		queries uint
		height  int
		want    int
	}{
		{queries: 1, height: 8, want: 0},
		{queries: 2, height: 1, want: 0},
		{queries: 2, height: 8, want: 1},
		{queries: 3, height: 8, want: 2},
		{queries: 4, height: 8, want: 2},
		{queries: 229, height: 12, want: 8},
		{queries: 229, height: 4, want: 3},
	}
	for _, test := range tests {
		if got := merkleCapDepth(test.queries, test.height); got != test.want {
			t.Errorf("queries=%d height=%d: got %d, want %d", test.queries, test.height, got, test.want)
		}
	}
}
