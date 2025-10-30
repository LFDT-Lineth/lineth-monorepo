package smt

import (
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Update overwrites a leaf in the tree and updates the associated parent nodes.
func (t *Tree) Update(pos int, newVal types.Bytes32) {
	depth := t.Config.Depth
	current := newVal
	idx := pos

	if pos >= 1<<depth {
		utils.Panic("out of bound %v", pos)
	}

	hasher := t.Config.HashFunc()

	for level := 0; level < t.Config.Depth; level++ {
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
