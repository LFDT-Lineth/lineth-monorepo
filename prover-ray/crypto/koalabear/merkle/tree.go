package merkle

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
)

type LeafHash = hash.Digest
type NodeHash = hash.Digest

// NodeHasher combines two child digests into a parent digest.
type NodeHasher interface {
	HashNode(left, right hash.Digest) hash.Digest
}

// Tree is a binary Merkle tree whose number of leaves is a power of two.
//
// Nodes are stored 1-indexed in a flat slice of length 2*nLeaves:
//   - nodes[1]              = root
//   - nodes[nLeaves..2*nLeaves-1] = leaves (leaf i at nodes[nLeaves+i])
//   - children of node k   = nodes[2k] (left) and nodes[2k+1] (right)
//   - parent of node k     = nodes[k/2]
type Tree struct {
	nodes      []hash.Digest
	nLeaves    int
	nodeHasher NodeHasher
}

// Proof is an opening proof for a single leaf.
// Siblings[0] is the sibling at the leaf level; Siblings[depth-1] is the
// sibling one level below the root.
type Proof struct {
	LeafIdx  int           // 0-based index of the opened leaf
	Siblings []hash.Digest // sibling digests, leaf-level first
}

// New creates a Tree for nLeaves leaves. nLeaves must be a positive power of two.
func New(nLeaves int, nh NodeHasher) (*Tree, error) {
	if nLeaves <= 0 || nLeaves&(nLeaves-1) != 0 {
		return nil, fmt.Errorf("merkle: nLeaves must be a positive power of two, got %d", nLeaves)
	}
	return &Tree{
		nodes:      make([]hash.Digest, 2*nLeaves),
		nLeaves:    nLeaves,
		nodeHasher: nh,
	}, nil
}

// BuildIthLeaf sets the i-th already-hashed leaf.
func (t *Tree) BuildIthLeaf(leaf hash.Digest, i int) error {
	if i < 0 || i >= t.nLeaves {
		return fmt.Errorf("merkle: leaf index %d out of range [0, %d)", i, t.nLeaves)
	}
	n := t.nLeaves
	t.nodes[n+i] = leaf
	return nil
}

// BuildNodes call this function after all the BuildIthLeaf have been called
func (t *Tree) BuildNodes() error {
	n := t.nLeaves
	for i := n - 1; i >= 1; i-- {
		t.nodes[i] = t.nodeHasher.HashNode(t.nodes[2*i], t.nodes[2*i+1])
	}
	return nil
}

// Build sets all already-hashed leaves, then builds internal nodes bottom-up.
// len(leaves) must equal nLeaves.
func (t *Tree) Build(leaves []hash.Digest) error {
	if len(leaves) != t.nLeaves {
		return fmt.Errorf("merkle: got %d leaves, want %d", len(leaves), t.nLeaves)
	}
	n := t.nLeaves
	for i, leaf := range leaves {
		t.nodes[n+i] = leaf
	}
	for i := n - 1; i >= 1; i-- {
		t.nodes[i] = t.nodeHasher.HashNode(t.nodes[2*i], t.nodes[2*i+1])
	}
	return nil
}

// Root returns the Merkle root digest. Build must be called first.
func (t *Tree) Root() hash.Digest {
	return t.nodes[1]
}

// OpenProof returns the Merkle opening proof for the leaf at 0-based index idx.
func (t *Tree) OpenProof(idx int) (Proof, error) {
	if idx < 0 || idx >= t.nLeaves {
		return Proof{}, fmt.Errorf("merkle: leaf index %d out of range [0, %d)", idx, t.nLeaves)
	}
	depth := log2(t.nLeaves)
	siblings := make([]hash.Digest, depth)
	pos := t.nLeaves + idx
	for k := 0; k < depth; k++ {
		siblings[k] = t.nodes[pos^1] // pos^1 flips the last bit to select the sibling
		pos >>= 1
	}
	return Proof{LeafIdx: idx, Siblings: siblings}, nil
}

// Verify checks that proof is a valid Merkle opening proof for leaf under root.
// The same node hasher used to build the tree must be supplied.
func Verify(root hash.Digest, proof Proof, leaf hash.Digest, nh NodeHasher) bool {
	h := leaf
	idx := proof.LeafIdx
	for _, sibling := range proof.Siblings {
		if idx&1 == 0 {
			h = nh.HashNode(h, sibling) // current node is the left child
		} else {
			h = nh.HashNode(sibling, h) // current node is the right child
		}
		idx >>= 1
	}
	return h == root
}

func log2(n int) int {
	k := 0
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}
