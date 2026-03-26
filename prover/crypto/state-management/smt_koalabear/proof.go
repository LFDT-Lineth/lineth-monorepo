package smt_koalabear

import (
	"errors"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

var ErrInvalidProof = errors.New("can't verify Merkle proof")

// ProvedClaim is the composition of a proof with the claim it proves.
type ProvedClaim struct {
	Proof      Proof
	Root, Leaf types.KoalaOctuplet
}

// Proof represents a Merkle proof of membership for the Merkle-tree
type Proof struct {
	Path     int                   `json:"leafIndex"` // Position of the leaf
	Siblings []types.KoalaOctuplet `json:"siblings"`  // length 40
}

// Prove returns a Merkle proof  of membership of the leaf at position `pos` and
// an error if the position is out of bounds.
func (t *Tree) Prove(pos int) (Proof, error) {

	var (
		depth    = t.Depth
		siblings = make([]types.KoalaOctuplet, depth)
		idx      = pos
	)

	if pos >= 1<<depth {
		return Proof{}, fmt.Errorf("pos=%v is too high: max is %v for depth %v", pos, 1<<depth, depth)
	}

	if pos < 0 {
		return Proof{}, fmt.Errorf("pos=%v is negative should be positive or zero", pos)
	}

	for level := 0; level < depth; level++ {
		sibling := t.getNode(level, idx^1) // xor 1, switch the last bits
		siblings[level] = types.KoalaOctuplet(sibling)
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
func RecoverRoot(p *Proof, leaf field.Octuplet) (field.Octuplet, error) {

	current := leaf
	idx := p.Path

	hasher := poseidon2_koalabear.NewMDHasher()

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
func Verify(p *Proof, leaf, root field.Octuplet) error {
	actual, err := RecoverRoot(p, leaf)
	if err != nil {
		return err
	}
	for i := 0; i < 8; i++ {
		if !actual[i].Equal(&root[i]) {
			return ErrInvalidProof
		}
	}
	return nil
}

// String pretty-prints a proof
func (p *Proof) String() string {
	siblingsBytes := make([]string, 0, len(p.Siblings))
	for _, s := range p.Siblings {
		siblingsBytes = append(siblingsBytes, s.Hex())
	}
	return fmt.Sprintf("&smt.Proof{Path: %d, Siblings: %v}", p.Path, siblingsBytes)
}
