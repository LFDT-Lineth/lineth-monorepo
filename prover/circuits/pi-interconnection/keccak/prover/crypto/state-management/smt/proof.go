package smt

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
)

// ProvedClaim is the composition of a proof with the claim it proves.
type ProvedClaim struct {
	Proof      Proof
	Root, Leaf types.Bytes32
}

// Proof represents a Merkle proof of membership for the Merkle-tree
type Proof struct {
	Path     int             `json:"leafIndex"` // Position of the leaf
	Siblings []types.Bytes32 `json:"siblings"`  // length 40
}

// Prove returns a Merkle proof  of membership of the leaf at position `pos` and
// an error if the position is out of bounds.
func (t *Tree) Prove(pos int) (Proof, error) {
	depth := t.Config.Depth
	siblings := make([]types.Bytes32, depth)
	idx := pos

	if pos >= 1<<depth {
		return Proof{}, fmt.Errorf("pos=%v is too high: max is %v for depth %v", pos, 1<<depth, depth)
	}

	if pos < 0 {
		return Proof{}, fmt.Errorf("pos=%v is negative should be positive or zero", pos)
	}

	for level := 0; level < depth; level++ {
		sibling := t.getNode(level, idx^1) // xor 1, switch the last bits
		siblings[level] = sibling
		idx >>= 1 // erase the last bit
	}

	// Sanity-check: the idx should be zero
	if idx != 0 {
		panic("idx should be zero")
	}

	return Proof{
		Siblings: siblings,
		Path:     pos,
	}, nil
}

// MustProve runs [Tree.Prove] and panics on error
func (t *Tree) MustProve(pos int) Proof {
	proof, err := t.Prove(pos)
	if err != nil {
		utils.Panic("could not prover: %v", err.Error())
	}
	return proof
}

// RecoverRoot returns the root recovered from the Merkle proof.
func (p *Proof) RecoverRoot(conf *Config, leaf types.Bytes32) (types.Bytes32, error) {

	if p.Path > 1<<conf.Depth {
		return types.Bytes32{}, fmt.Errorf("invalid proof: path is %v larger than the number of leaves in the tree %v", p.Path, 1<<len(p.Siblings))
	}

	if p.Path < 0 {
		return types.Bytes32{}, fmt.Errorf("invalid proof: path is negative %v", p.Path)
	}

	if len(p.Siblings) != conf.Depth {
		return types.Bytes32{}, fmt.Errorf("the proof contains %v siblings but the tree has a depth of %v", len(p.Siblings), conf.Depth)
	}

	var (
		current = leaf
		idx     = p.Path
	)

	hasher := conf.HashFunc()
	for _, sibling := range p.Siblings {
		left, right := current, sibling
		if idx&1 == 1 {
			left, right = right, left
		}
		current = hashLR(hasher, left, right)
		idx >>= 1
	}

	// Sanity-check: the idx should be zero. We already checked the path to
	// be within bounds.
	if idx != 0 {
		panic("idx should be zero")
	}

	return current, nil
}

// Verify the Merkle-proof against a hash and a root
func (p *Proof) Verify(conf *Config, leaf, root types.Bytes32) bool {
	actual, err := p.RecoverRoot(conf, leaf)
	if err != nil {
		fmt.Printf("mtree verify: %v\n", err.Error())
		return false
	}
	return actual == root
}

// String pretty-prints a proof
func (p *Proof) String() string {
	return fmt.Sprintf("&smt.Proof{Path: %d, Siblings: %x}", p.Path, p.Siblings)
}
