package smt_bls12377

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
)

var ErrInvalidProof = errors.New("can't verify Merkle proof")

// ProvedClaim is the composition of a proof with the claim it proves.
type ProvedClaim struct {
	Proof      Proof
	Root, Leaf fr.Element
}

// Proof represents a Merkle proof of membership for the Merkle-tree
type Proof struct {
	Path     int          `json:"leafIndex"` // Position of the leaf
	Siblings []fr.Element `json:"siblings"`  // length 40
}

// Prove returns a Merkle proof  of membership of the leaf at position `pos` and
// an error if the position is out of bounds.
func (t *Tree) Prove(pos int) (Proof, error) {
	depth := t.Depth
	siblings := make([]fr.Element, depth)
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
func RecoverRoot(p *Proof, leaf fr.Element) (fr.Element, error) {

	current := leaf
	idx := p.Path

	for _, sibling := range p.Siblings {
		left, right := current, sibling
		if idx&1 == 1 {
			left, right = right, left
		}
		current = hashLR(left, right)
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
func Verify(p *Proof, leaf, root fr.Element) error {
	actual, err := RecoverRoot(p, leaf)
	if err != nil {
		return err
	}
	if !actual.Equal(&root) {
		return ErrInvalidProof
	}
	return nil
}

// String pretty-prints a proof
func (p *Proof) String() string {
	return fmt.Sprintf("&smt.Proof{Path: %d, Siblings: %x}", p.Path, p.Siblings)
}
