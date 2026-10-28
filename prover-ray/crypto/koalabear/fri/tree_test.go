package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
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

// TestBuildTreeExtOpenAuthenticate checks the Merkle tree round-trip across
// several sizes: every leaf opens to a branch that authenticates against the
// tree root, the opened leaf and its deepest sibling are the adjacent
// (conjugate) pair, and tampering the leaf breaks authentication.
func TestBuildTreeExtOpenAuthenticate(t *testing.T) {

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
		frontier := []field.Octuplet{root}
		for idx := 0; idx < n; idx++ {

			branch := tree.OpenBranch(idx)

			if branch.Leaf != octs[idx] {
				t.Fatalf("n=%d idx=%d: opened leaf does not match leaves[idx]", n, idx)
			}
			last := len(branch.Siblings) - 1
			if last < 0 || branch.Siblings[last] != octs[idx^1] {
				t.Fatalf("n=%d idx=%d: deepest sibling is not the adjacent leaf idx^1", n, idx)
			}

			if err := branch.AuthenticateToCap(idx, frontier); err != nil {
				t.Fatalf("n=%d idx=%d: AuthenticateToCap: %v", n, idx, err)
			}

			// Tampering the leaf must break authentication.
			bad := branch
			bad.Leaf = field.PseudoRandOctuplet(prng)
			if err := bad.AuthenticateToCap(idx, frontier); err == nil {
				t.Fatalf("n=%d idx=%d: tampered leaf authenticated", n, idx)
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
		treeCap := tree.OpenCap(depth)
		if err := treeCap.Authenticate(depth, tree.Root()); err != nil {
			t.Fatalf("depth %d: authenticate cap: %v", depth, err)
		}
		for idx := range leaves {
			branch := tree.OpenBranchToDepth(idx, depth)
			if err := branch.AuthenticateToCap(idx, treeCap.Nodes); err != nil {
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
	treeCap := tree.OpenCap(2)
	if err := treeCap.Authenticate(2, tree.Root()); err != nil {
		t.Fatalf("authenticate cap: %v", err)
	}
	if treeCap.Aux[0] == nil {
		t.Fatal("cap omitted root auxiliary digest")
	}

	// Cap auxiliary values must not alias the committed tree.
	original := tree.Root()
	*treeCap.Aux[0] = field.PseudoRandOctuplet(prng)
	if tree.Root() != original {
		t.Fatal("mutating cap auxiliary digest changed tree root")
	}
	if err := treeCap.Authenticate(2, tree.Root()); err == nil {
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
	treeCap := tree.OpenCap(2)

	badLength := treeCap
	badLength.Nodes = badLength.Nodes[:len(badLength.Nodes)-1]
	if err := badLength.Validate(2); err == nil {
		t.Fatal("truncated cap accepted")
	}
	if err := (MerkleCap{Nodes: []field.Octuplet{{}}}).Validate(0); err == nil {
		t.Fatal("nonempty depth-zero cap accepted")
	}

	badNode := treeCap
	badNode.Nodes = append([]field.Octuplet(nil), treeCap.Nodes...)
	badNode.Nodes[0] = field.PseudoRandOctuplet(prng)
	if err := badNode.Authenticate(2, tree.Root()); err == nil {
		t.Fatal("tampered cap node authenticated")
	}

	branch := tree.OpenBranchToDepth(3, 2)
	branch.Siblings[0] = field.PseudoRandOctuplet(prng)
	if err := branch.AuthenticateToCap(3, treeCap.Nodes); err == nil {
		t.Fatal("tampered branch authenticated")
	}
}

func TestMerkleCapAuthenticatesDepthZero(t *testing.T) {
	root := field.PseudoRandOctuplet(rand.New(utils.NewRandSource(31)))
	if err := (MerkleCap{}).Authenticate(0, root); err != nil {
		t.Fatalf("depth-zero cap should authenticate the trusted root: %v", err)
	}
}

func TestOptimalMerkleCapHeight(t *testing.T) {
	tests := []struct {
		queries uint
		want    uint8
	}{
		{queries: 1, want: 0},
		{queries: 2, want: 1},
		{queries: 3, want: 2},
		{queries: 4, want: 2},
		{queries: 5, want: 3},
		{queries: 229, want: 8},
	}
	for _, test := range tests {
		if got := OptimalMerkleCapHeight(test.queries); got != test.want {
			t.Errorf("queries=%d: got %d, want %d", test.queries, got, test.want)
		}
	}
}

func TestMerkleCapDepthClampsToHeight(t *testing.T) {
	tests := []struct {
		capHeight uint8
		height    int
		want      int
	}{
		{capHeight: 0, height: 8, want: 0},
		{capHeight: 3, height: 1, want: 0},
		{capHeight: 3, height: 8, want: 3},
		{capHeight: 7, height: 8, want: 7},
		{capHeight: 8, height: 8, want: 7},
		{capHeight: 255, height: 4, want: 3},
	}
	for _, test := range tests {
		p := Params{MerkleCapHeight: test.capHeight}
		if got := p.merkleCapDepth(test.height); got != test.want {
			t.Errorf("capHeight=%d height=%d: got %d, want %d", test.capHeight, test.height, got, test.want)
		}
	}
}

// multiInjectionTree builds a 2^height-leaf tree whose level l receives
// counts[l] injected digest slices (0, 1 or several), covering both the scalar
// levels (fewer than batchLanes nodes) and the batched ones.
func multiInjectionTree(prng *rand.Rand, height int, counts []int) (*Tree, [][][]field.Octuplet) {
	bottom := make([]field.Octuplet, 1<<height)
	for i := range bottom {
		bottom[i] = field.PseudoRandOctuplet(prng)
	}
	injections := make([][][]field.Octuplet, height)
	for level := range injections {
		for range counts[level] {
			injected := make([]field.Octuplet, 1<<level)
			for j := range injected {
				injected[j] = field.PseudoRandOctuplet(prng)
			}
			injections[level] = append(injections[level], injected)
		}
	}
	tree := allocTree(len(bottom))
	copy(tree.Nodes[len(bottom)-1:], bottom)
	tree.buildLevels(injections)
	return tree, injections
}

// TestTreeChainsSeveralInjections checks every internal node against the
// definition C(…C(C(left, right), a_1)…, a_m), on scalar and batched levels
// alike, with zero, one and several injections per level.
func TestTreeChainsSeveralInjections(t *testing.T) {
	prng := rand.New(utils.NewRandSource(11))
	const height = 7 // levels of 1..64 nodes: below and above batchLanes
	tree, injections := multiInjectionTree(prng, height, []int{2, 0, 3, 1, 0, 2, 3})

	for level := range height {
		for j := range 1 << level {
			k := 1<<level - 1 + j
			want := poseidon2.Compress(tree.Nodes[2*k+1], tree.Nodes[2*k+2])
			for _, injected := range injections[level] {
				want = poseidon2.Compress(want, injected[j])
			}
			if tree.Nodes[k] != want {
				t.Fatalf("level %d node %d does not match the injection chain", level, j)
			}
		}
	}
}

// TestBranchAuthenticatesSeveralInjections checks that every branch of a tree
// with several injections per level authenticates against the root and against
// a cap frontier, and that tampering with, dropping or reordering an injected
// digest breaks it.
func TestBranchAuthenticatesSeveralInjections(t *testing.T) {
	prng := rand.New(utils.NewRandSource(12))
	const height = 6
	tree, _ := multiInjectionTree(prng, height, []int{1, 2, 0, 3, 2, 1})
	root := []field.Octuplet{tree.Root()}

	for _, depth := range []int{0, 2, 4} {
		frontier := tree.Nodes[1<<depth-1 : 1<<(depth+1)-1]
		for idx := range tree.NumLeaves() {
			branch := tree.OpenBranchToDepth(idx, depth)
			if err := branch.AuthenticateToCap(idx, frontier); err != nil {
				t.Fatalf("depth %d leaf %d: %v", depth, idx, err)
			}
			if depth == 0 {
				if err := branch.AuthenticateToCap(idx, root); err != nil {
					t.Fatalf("leaf %d against the root: %v", idx, err)
				}
			}
		}
	}

	branch := tree.OpenBranch(5)
	for level, injected := range branch.AuxSiblings {
		if len(injected) == 0 {
			continue
		}
		tampered := tree.OpenBranch(5)
		tampered.AuxSiblings[level][0][0].SetUint64(1234)
		if tampered.AuthenticateToCap(5, root) == nil {
			t.Fatalf("level %d: tampered injection accepted", level)
		}
		dropped := tree.OpenBranch(5)
		dropped.AuxSiblings[level] = dropped.AuxSiblings[level][1:]
		if dropped.AuthenticateToCap(5, root) == nil {
			t.Fatalf("level %d: dropped injection accepted", level)
		}
		if len(injected) > 1 {
			swapped := tree.OpenBranch(5)
			swapped.AuxSiblings[level][0], swapped.AuxSiblings[level][1] =
				swapped.AuxSiblings[level][1], swapped.AuxSiblings[level][0]
			if swapped.AuthenticateToCap(5, root) == nil {
				t.Fatalf("level %d: reordered injections accepted", level)
			}
		}
	}
}

// TestCapRootSeveralInjections checks that capRoot rebuilds the root from any
// frontier given every node's injections, and that OpenCap refuses a prefix it
// cannot carry in a MerkleCap.
func TestCapRootSeveralInjections(t *testing.T) {
	prng := rand.New(utils.NewRandSource(13))
	const height = 5
	tree, _ := multiInjectionTree(prng, height, []int{2, 1, 3, 0, 2})

	for depth := 1; depth < height; depth++ {
		frontierStart := 1<<depth - 1
		injections := make([][]field.Octuplet, frontierStart)
		for k := range injections {
			injections[k] = tree.injectedInto(k)
		}
		nodes := tree.Nodes[frontierStart : 2*frontierStart+1]
		if capRoot(nodes, injections) != tree.Root() {
			t.Fatalf("depth %d: capRoot does not rebuild the root", depth)
		}
	}

	defer func() {
		if recover() == nil {
			t.Fatal("OpenCap accepted a node with several injections")
		}
	}()
	tree.OpenCap(2)
}
