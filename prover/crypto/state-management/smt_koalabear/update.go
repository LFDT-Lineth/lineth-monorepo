package smt_koalabear

import (
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Update overwrites a leaf in the tree and updates the associated parent nodes.
func (t *Tree) Update(pos int, newVal field.Octuplet) {

	current := newVal
	idx := pos

	if pos >= 1<<t.Depth {
		utils.Panic("out of bound %v", pos)
	}

	hasher := poseidon2_koalabear.NewMDHasher()
	for level := 0; level < t.Depth; level++ {
		// store the newly computed node
		t.updateNode(level, idx, current)
		sibling := t.getNode(level, idx^1) // xor 1, switch the last bits
		left, right := current, sibling
		if idx&1 == 1 {
			left, right = right, left
		}
		current = hashLR(hasher, left, right)
		idx >>= 1
	}

	// Sanity-check: the idx should be zero
	if idx != 0 {
		panic("idx should be zero")
	}

	t.Root = current
}
