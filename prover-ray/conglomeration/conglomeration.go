// Package conglomeration reduces the shard proofs of a sharded execution to a
// single root proof through a tree of merges of up to [RecursionTreeArity]
// nodes each, performed by a [Merger]. The recursion machine (package
// recursion) will implement the merger for production; [MockMerger]
// implements it for dev and test, without producing proofs.
//
// Every node of the tree exposes the public-input layout of a shard proof
// (see zkcdriver/risc5). Across shards, the message-bus products multiply and
// the shared randomness contributions add; the program VK, the guest output
// and the shared randomness seed γ are the same on every shard. At the root,
// every message-bus handle balances and γ is the seed the contributions add
// up to: that is the external check of γ the message-bus compiler leaves to
// the conglomeration (see the soundness precondition in package messagebus).
package conglomeration

import (
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Node is a proof in the conglomeration tree: a shard proof, whose Coverage is
// its single shard index, or a merged node, whose Coverage is the sorted,
// duplicate-free union of its children's. Shard proofs and nodes expose the
// same public-input layout, so a node can be the child of another node at any
// depth.
type Node struct {
	Coverage []uint
	Proof    wiop.Proof
	Pub      wiop.PublicInput
}

// RecursionTreeArity is the maximum number of children of a node of the
// conglomeration tree: the most proofs one recursion step verifies. A merge
// takes 2 to RecursionTreeArity children; [Conglomerate] fills every merge as
// much as possible, so only the last merge of the tree has fewer than
// RecursionTreeArity.
const RecursionTreeArity = 6

// Merger performs the merge steps of the conglomeration tree and the root
// finalisation.
type Merger interface {
	// Merge verifies 2 to [RecursionTreeArity] children and returns their
	// parent, covering the union of their coverage. It rejects a child count
	// out of that range, overlapping coverage, a child of another
	// public-input shape, and children that disagree on the public inputs
	// every shard shares: the program VK, the guest output and, when the
	// system has one, the shared randomness seed γ.
	Merge(children []Node) (Node, error)
	// Finalize runs the root checks on the node covering all numShards shards:
	// its coverage is exactly {0..numShards-1}, every message-bus handle
	// balances and, when the system has one, γ is the seed the shards'
	// contributions add up to. It returns the root proof.
	Finalize(root Node, numShards uint) (Node, error)
}

// mergeCoverage merges two sorted, duplicate-free coverages, failing on a
// shard covered by both.
func mergeCoverage(a, b []uint) ([]uint, error) {
	merged := make([]uint, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			merged = append(merged, a[i])
			i++
		case b[j] < a[i]:
			merged = append(merged, b[j])
			j++
		default:
			return nil, fmt.Errorf("shard %d is covered twice", a[i])
		}
	}
	merged = append(merged, a[i:]...)
	merged = append(merged, b[j:]...)
	return merged, nil
}

// checkCoverageComplete checks that the sorted, duplicate-free coverage is
// exactly {0..numShards-1}, naming the first missing or unexpected shard.
func checkCoverageComplete(coverage []uint, numShards uint) error {
	for k := uint(0); k < numShards; k++ {
		if k >= uint(len(coverage)) || coverage[k] != k {
			return fmt.Errorf("missing shard %d", k)
		}
	}
	if uint(len(coverage)) > numShards {
		return fmt.Errorf("unexpected shard %d, the trace has %d shards", coverage[numShards], numShards)
	}
	return nil
}

// DefaultMaxInFlightMerges bounds the concurrent [Merger.Merge] calls of
// [Conglomerate]: a real merged node is a full proof and competes for memory
// with shard proving.
const DefaultMaxInFlightMerges = 1

// Conglomerate drains the n shard proofs received on in and reduces them to a
// single node with merger, at most maxInFlight merges at a time, filling
// every merge as much as possible: it merges [RecursionTreeArity] ready nodes,
// in arrival order, as soon as they are available, and waits for more shard
// proofs or running merges rather than merge fewer. Only once every shard
// proof has arrived and no merge is running does it merge the fewer nodes
// left, so only the last merge of the tree has fewer children. n and maxInFlight must be at least 1; merger is only called
// when n > 1. It does not run the root checks: the caller calls
// [Merger.Finalize] if the node covers all the shards. It fails if in closes
// before n proofs arrived.
func Conglomerate(in <-chan Node, n int, merger Merger, maxInFlight int) (Node, error) {
	type result struct {
		node Node
		err  error
	}
	var (
		ready    []Node
		done     = make(chan result, n)
		received int
		inFlight int
	)
	start := func(k int) {
		children := slices.Clone(ready[:k])
		ready = slices.Delete(ready, 0, k)
		inFlight++
		go func() {
			node, err := merger.Merge(children)
			done <- result{node: node, err: err}
		}()
	}
	for {
		for inFlight < maxInFlight && len(ready) >= RecursionTreeArity {
			start(RecursionTreeArity)
		}
		if received == n && inFlight == 0 {
			if len(ready) == 1 {
				return ready[0], nil
			}
			// Fewer than RecursionTreeArity nodes are left and nothing more
			// will arrive: the last merge of the tree.
			start(len(ready))
		}
		if received == n {
			in = nil // all the shard proofs arrived, wait for the merges only
		}
		select {
		case p, ok := <-in:
			if !ok {
				return Node{}, fmt.Errorf("only %d of %d shard proofs arrived", received, n)
			}
			received++
			ready = append(ready, p)
		case r := <-done:
			inFlight--
			if r.err != nil {
				return Node{}, r.err
			}
			ready = append(ready, r.node)
		}
	}
}
