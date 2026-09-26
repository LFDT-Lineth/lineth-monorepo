package fri

import (
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/consensys/gnark/frontend"
)

// This file is the in-circuit counterpart of the Merkle capping primitives in
// tree.go. Every function mirrors a native one and must be kept in lock-step
// with it; pcs_circuit_test.go checks them against the native implementations.
//
// The running FRI trees are aux-free, so the caps and branches handled here
// carry no auxiliary digests. Input trees keep their own capped path in
// pcs_circuit.go, where auxiliary rows are folded in.

// GnarkMerkleCap mirrors [MerkleCap] for aux-free trees: Nodes is the depth-d
// frontier, left to right, and a depth-zero cap is the empty slice, standing
// for the trusted root itself.
type GnarkMerkleCap struct {
	Nodes []poseidon2.KoalagnarkOctuplet
}

// NewGnarkMerkleCap converts a native cap into its witness assignment.
func NewGnarkMerkleCap(c MerkleCap) GnarkMerkleCap {
	return convertMerkleCap(c, true)
}

// AllocateGnarkMerkleCap returns an unassigned cap with the same geometry as c.
func AllocateGnarkMerkleCap(c MerkleCap) GnarkMerkleCap {
	return convertMerkleCap(c, false)
}

func convertMerkleCap(c MerkleCap, withValues bool) GnarkMerkleCap {
	// Auxiliary digests only arise in multi-level trees; the running trees
	// this mirrors are single-level, so every Aux entry is nil. Reject a cap
	// that carries one rather than silently dropping it from the circuit.
	for _, aux := range c.Aux {
		if aux != nil {
			panic("fri: GnarkMerkleCap: auxiliary digests are not supported")
		}
	}
	return GnarkMerkleCap{Nodes: convertOctuplets(c.Nodes, withValues)}
}

// recoverCapRootGnark mirrors [MerkleCap.RecoverRoot]. aux carries the cap's
// auxiliary digests in heap order and may be nil, or hold nil entries, for the
// levels that have none; the running trees pass nil throughout, while input
// trees supply digests rebuilt from their revealed tables.
func recoverCapRootGnark(
	api *circuit.KoalaBearAPI, nodes []poseidon2.KoalagnarkOctuplet, aux []*poseidon2.KoalagnarkOctuplet,
) poseidon2.KoalagnarkOctuplet {
	n := len(nodes)
	if n == 0 {
		panic("fri: recoverCapRootGnark: empty cap has no root")
	}
	if n&(n-1) != 0 {
		panic(fmt.Sprintf("fri: recoverCapRootGnark: cap has %d nodes, want a power of two", n))
	}
	if aux != nil && len(aux) != n-1 {
		panic(fmt.Sprintf("fri: recoverCapRootGnark: cap has %d auxiliary digests, want %d", len(aux), n-1))
	}

	work := make([]poseidon2.KoalagnarkOctuplet, 2*n-1)
	copy(work[n-1:], nodes)
	for i := n - 2; i >= 0; i-- {
		work[i] = poseidon2.KoalagnarkCompress(api, work[2*i+1], work[2*i+2])
		if aux != nil && aux[i] != nil {
			work[i] = poseidon2.KoalagnarkCompress(api, work[i], *aux[i])
		}
	}
	return work[0]
}

// authenticateCapGnark mirrors [MerkleCap.Authenticate]: it constrains the cap
// to reconstruct the trusted root, and returns the frontier every branch of
// that tree is then checked against. A depth-zero cap must be empty and the
// frontier is the root alone.
func authenticateCapGnark(
	api *circuit.KoalaBearAPI, treeCap GnarkMerkleCap, depth int, root poseidon2.KoalagnarkOctuplet,
) []poseidon2.KoalagnarkOctuplet {
	if depth == 0 {
		if len(treeCap.Nodes) != 0 {
			panic(fmt.Sprintf("fri: authenticateCapGnark: depth-zero cap has %d nodes, want 0", len(treeCap.Nodes)))
		}
		return []poseidon2.KoalagnarkOctuplet{root}
	}
	if want := 1 << depth; len(treeCap.Nodes) != want {
		panic(fmt.Sprintf("fri: authenticateCapGnark: cap has %d nodes, want %d", len(treeCap.Nodes), want))
	}
	assertOctupletEqual(api, recoverCapRootGnark(api, treeCap.Nodes, nil), root)
	return treeCap.Nodes
}

// authenticateToCapGnark mirrors [Branch.AuthenticateToCap]: fold the branch
// from its leaf up to the frontier, then constrain the result to equal the
// frontier node the opening index lands on.
//
// idxBits are the little-endian bits of the leaf index within the capped tree,
// low bit first. The first len(branch.Siblings) of them drive the fold; the
// remaining log2(len(frontier)) select the frontier node, mirroring the native
// `ancestor == frontier[currPos]` check where currPos is idx shifted right
// past the folded levels. The selection is a multiplexer over the whole
// frontier, since the index is a circuit variable.
func authenticateToCapGnark(
	api *circuit.KoalaBearAPI, branch GnarkBranch, idxBits []frontend.Variable,
	frontier []poseidon2.KoalagnarkOctuplet,
) {
	n := len(branch.Siblings)
	depth := frontierDepth(len(frontier))
	if len(idxBits) < n+depth {
		panic(fmt.Sprintf("fri: authenticateToCapGnark: got %d index bits, need %d", len(idxBits), n+depth))
	}

	ancestor := branch.Leaf
	for i := n - 1; i >= 0; i-- {
		ancestor = foldOneLevelGnark(api, ancestor, branch.Siblings[i], nil, idxBits[n-1-i])
	}
	assertOctupletEqual(api, ancestor, selectFrontierNodeGnark(api, frontier, idxBits[n:n+depth]))
}

// selectFrontierNodeGnark returns frontier[idx], idx being the little-endian
// bits selBits. A single-node frontier needs no selection.
func selectFrontierNodeGnark(
	api *circuit.KoalaBearAPI, frontier []poseidon2.KoalagnarkOctuplet, selBits []frontend.Variable,
) poseidon2.KoalagnarkOctuplet {
	if len(frontier) == 1 {
		return frontier[0]
	}
	sel := api.Frontend().FromBinary(selBits...)

	var res poseidon2.KoalagnarkOctuplet
	coord := make([]circuit.Element, len(frontier))
	for c := range res {
		for i := range frontier {
			coord[i] = frontier[i][c]
		}
		res[c] = api.Mux(sel, coord...)
	}
	return res
}

// frontierDepth returns the cap depth a frontier of the given size encodes.
func frontierDepth(size int) int {
	if size == 0 || size&(size-1) != 0 {
		panic(fmt.Sprintf("fri: frontier has %d nodes, want a power of two", size))
	}
	return bits.TrailingZeros(uint(size))
}
