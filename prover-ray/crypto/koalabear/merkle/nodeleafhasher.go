package merkle

import "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"

// Tree is a binary Merkle tree whose number of leaves is a power of two.
//
// Nodes are stored 1-indexed in a flat slice of length 2*nLeaves:
//   - nodes[1]              = root
//   - nodes[nLeaves..2*nLeaves-1] = leaves (leaf i at nodes[nLeaves+i])
//   - children of node k   = nodes[2k] (left) and nodes[2k+1] (right)
//   - parent of node k     = nodes[k/2]
type Tree struct {
	Nodes []hash.Digest

	nLeaves int
}
