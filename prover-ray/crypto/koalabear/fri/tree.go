package fri

import (
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Tree is a Merkle tree for multi-size FRI. The tree is 3-ary, each node may
// have:
//
//   - 0 children: leaf
//   - 2 children: internal node and there is no batch of polynomial
//     evaluations corresponding to this layer.
//   - 3 children: internal node and there is a batch of polynomial
//     evaluations corresponding to this layer.
type Tree struct {
	// Nodes stores the nodes of the tree. The first node is the root. The
	// children of node k are at indices 2k+1 and 2k+2.
	Nodes []field.Octuplet
	// Aux stores the auxiliary leaves of the tree. Aux[i] is the auxiliary
	// leaf of Nodes[i]. Thus Nodes[i] = H(Nodes[2*i+1], Nodes[2*i+2], Aux[i])
	Aux []*field.Octuplet
}

// Proof is a Merkle opening proof for a single leaf.
type Branch struct {
	Leaf        field.Octuplet
	Siblings    []field.Octuplet
	AuxSiblings []field.Octuplet
}

// NewTree builds a new Tree from the given leaves. The leaves must be provided
// in the following order.
//
//	for all 0 <= i < len(leaves): leaves[i] = 2**(N-i-1) or 0
func NewTree(leaves [][]field.Octuplet) *Tree {

	if len(leaves) == 0 {
		panic("at least one level must be provided")
	}

	if len(leaves[0]) == 0 {
		panic("the first level must be non-empty")
	}

	for i, _ := range leaves {
		n := len(leaves[i])
		if n != 0 && len(leaves[i]) != 1<<(len(leaves)-i-1) {
			panic("leaves must be provided in the following order: " +
				"for all 0 <= i < len(leaves): leaves[i] = 2**(N-i-1)")
		}
	}

	var (
		nodes = make([]field.Octuplet, 2*len(leaves[0])-1)
		aux   = make([]*field.Octuplet, len(leaves[0])-1)
	)

	copy(nodes[len(leaves[0])-1:], leaves[0])

	for i := 1; i < len(leaves); i++ {

		var (
			n             = 1 << (len(leaves) - i - 1)
			levelStartPos = 1<<i - 1
		)

		for j := 0; j < n; j++ {

			k := levelStartPos + j

			if aux[k] != nil {
				panic("indices on aux are wrong and we are overlapping values")
			}

			if len(leaves[i]) > 0 {
				// we already asserted that len(leaves[i]) == n. So this will
				// not go OOB.
				aux[k] = &leaves[i][j]
			}

			left, right := nodes[2*k+1], nodes[2*k+2]
			if (nodes[k] != field.Octuplet{}) {
				panic("already computed node; the indexing must be wrong")
			}

			nodes[k] = hashNode(left, right, aux[k])
		}
	}

	// as the tree cannot be empty (as per our sanity-checks), the root cannot
	// be zero.
	if nodes[0] == (field.Octuplet{}) {
		panic("sanity-check failed : the root is zero.")
	}

	return &Tree{
		Nodes: nodes,
		Aux:   aux,
	}
}

// Root returns the Merkle root digest. Build must be called first.
func (t *Tree) Root() field.Octuplet {
	return t.Nodes[0]
}

// OpenProof returns the Merkle opening proof for the leaf at 0-based index idx.
func (t *Tree) OpenProof(idx int) (Proof, error) {
	return t.Nodes.OpenProof(idx)
}

// hashNode hashes two field.Octuplets and an optional field.Octuplet.
func hashNode(left, right field.Octuplet, aux *field.Octuplet) field.Octuplet {
	hasher := poseidon2.NewMDHasher()
	hasher.WriteElements(left[:]...)
	hasher.WriteElements(right[:]...)
	if aux != nil {
		hasher.WriteElements(aux[:]...)
	}
	return hasher.SumDigest()
}
