// Package conglomeration reduces the shard proofs of a sharded execution to a
// single root proof through a tree of merges of up to [RecursionTreeArity]
// nodes each, performed by a [Merger]. The recursion machine (package
// recursion) will implement the merger for production; for dev and test,
// [RecursionMerger] proves the merges with the machine's node program without
// verifying the children, and [MockMerger] produces no proof at all.
//
// Every node of the tree exposes the public-input layout of a shard proof
// (see zkcdriver/risc5). Across shards, the message-bus products multiply and
// the shared randomness contributions add; the program VK, the guest output
// and the shared randomness seed γ are the same on every shard. At the root,
// every message-bus handle balances and γ is the seed the contributions add
// up to: that is the external check of γ the message-bus compiler leaves to
// the conglomeration (see the soundness precondition in package messagebus).
//
// A merge takes children of one [Kind] only: shard proofs are verified by the
// leaf verifier of the recursion machine and merged nodes by its internal
// verifier, two different programs, and a merged node will carry a public
// input (the verification key of the node program) that a shard proof lacks.
// A lone shard proof is lifted into a merged node by a single-child merge, so
// the root checks only ever run on a merged node.
package conglomeration

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Kind tells a shard proof from a node merged by the [Merger].
type Kind int

const (
	// Shard is a shard proof, whose Coverage is its single shard index. The
	// zero value, so a [Node] built from a shard proof needs no Kind.
	Shard Kind = iota
	// Merged is a node returned by [Merger.Merge], whose Coverage is the
	// sorted, duplicate-free union of its children's.
	Merged
)

// Node is a proof in the conglomeration tree: a shard proof or a merged node,
// told apart by Kind. Shard proofs and merged nodes expose the same
// public-input layout, so the merged nodes can be the children of another
// node at any depth.
type Node struct {
	Kind     Kind
	Coverage []uint
	Proof    wiop.Proof
	Pub      wiop.PublicInput
}

// RecursionTreeArity is the maximum number of children of a node of the
// conglomeration tree: the most proofs one recursion step verifies. A merge
// takes 1 to RecursionTreeArity children; a [Reducer] fills every merge as
// much as possible, so only the last merge of each kind has fewer than
// RecursionTreeArity.
const RecursionTreeArity = 6

// Merger performs the merge steps of the conglomeration tree and the root
// finalisation.
type Merger interface {
	// Merge verifies 1 to [RecursionTreeArity] children of the same [Kind]
	// and returns their parent, a [Merged] node covering the union of their
	// coverage. A single child is accepted for a shard proof only: the lift
	// that turns a lone shard proof into a merged node. It rejects a child
	// count out of that range, mixed kinds, overlapping coverage, a child of
	// another public-input shape, and children that disagree on the public
	// inputs every shard shares: the program VK, the guest output and, when
	// the system has one, the shared randomness seed γ.
	Merge(children []Node) (Node, error)
	// Finalize runs the root checks on the [Merged] node covering all
	// numShards shards: its coverage is exactly {0..numShards-1}, every
	// message-bus handle balances and, when the system has one, γ is the seed
	// the shards' contributions add up to. It returns the root proof.
	Finalize(root Node, numShards uint) (Node, error)
}

// checkChildren checks the child count and kinds of a merge: 1 to
// [RecursionTreeArity] children of one kind, a single one only for a shard
// proof.
func checkChildren(children []Node) error {
	if len(children) < 1 || len(children) > RecursionTreeArity {
		return fmt.Errorf("merging %d nodes, want 1 to %d", len(children), RecursionTreeArity)
	}
	first := children[0]
	for _, child := range children[1:] {
		if child.Kind != first.Kind {
			return fmt.Errorf("merging nodes %v and %v of different kinds", first.Coverage, child.Coverage)
		}
	}
	if len(children) == 1 && first.Kind != Shard {
		return fmt.Errorf("merging the single merged node %v: only a shard proof is lifted alone", first.Coverage)
	}
	return nil
}

// checkRoot checks that root is a merged node: the root checks never run on
// a bare shard proof, which is lifted first.
func checkRoot(root Node) error {
	if root.Kind != Merged {
		return fmt.Errorf("finalizing the shard proof %v, want a merged node", root.Coverage)
	}
	return nil
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

// DefaultMaxInFlightMerges bounds the concurrent [Merger.Merge] calls of a
// [Reducer]: a real merged node is a full proof and competes for memory with
// shard proving.
const DefaultMaxInFlightMerges = 1

// Reducer reduces the nodes pushed to it by [Reducer.Add] to a single merged
// node. It merges [RecursionTreeArity] ready nodes of one kind, in arrival
// order, in the background as soon as they are available, at most
// maxInFlight merges at a time, and waits for more nodes or running merges
// rather than merge fewer. Only [Reducer.Close], once nothing more can
// arrive, merges the fewer nodes left: the shard proofs first (a lone one is
// lifted), then the merged nodes, so only the last merge of each kind has
// fewer children. It does not run the root checks: the caller calls
// [Merger.Finalize] if the node covers all the shards.
type Reducer struct {
	merger      Merger
	maxInFlight int

	mu       sync.Mutex
	cond     sync.Cond // signalled when a merge completes
	shards   []Node    // ready shard proofs
	merged   []Node    // ready merged nodes
	inFlight int
	err      error // the first merge failure
}

// NewReducer returns a reducer merging with merger, at most maxInFlight
// merges at a time. maxInFlight must be at least 1; merger is called as soon
// as one node is added.
func NewReducer(merger Merger, maxInFlight int) *Reducer {
	r := &Reducer{merger: merger, maxInFlight: maxInFlight}
	r.cond.L = &r.mu
	return r
}

// Add pushes a shard proof or a merged node, starting a merge if enough
// nodes of its kind are ready. It returns the error of a merge that failed
// since the previous call, if any: the reduction is then lost and the caller
// need not prove the remaining shards. Add must not be called after
// [Reducer.Close].
func (r *Reducer) Add(node Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	switch node.Kind {
	case Shard:
		r.shards = append(r.shards, node)
	case Merged:
		r.merged = append(r.merged, node)
	default:
		return fmt.Errorf("node %v has unknown kind %d", node.Coverage, node.Kind)
	}
	r.startFullMerges()
	return nil
}

// Close waits for the running merges, performs the last merges of the tree
// and returns the resulting merged node, or the error of the first failed
// merge.
func (r *Reducer) Close() (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for {
		r.startFullMerges()
		for r.inFlight > 0 && r.err == nil {
			r.cond.Wait()
		}
		if r.err != nil {
			return Node{}, r.err
		}
		// Fewer than RecursionTreeArity nodes of a kind are left and nothing
		// more will arrive: the last merge of that kind, the shard proofs
		// first since their result is a merged node.
		switch {
		case len(r.shards) > 0:
			r.start(&r.shards, len(r.shards))
		case len(r.merged) > 1:
			r.start(&r.merged, len(r.merged))
		case len(r.merged) == 1:
			return r.merged[0], nil
		default:
			return Node{}, errors.New("no node was added")
		}
	}
}

// startFullMerges starts merges of RecursionTreeArity ready nodes of a kind
// as long as there are enough and the in-flight bound allows. Called with mu
// held.
func (r *Reducer) startFullMerges() {
	for r.inFlight < r.maxInFlight && len(r.shards) >= RecursionTreeArity {
		r.start(&r.shards, RecursionTreeArity)
	}
	for r.inFlight < r.maxInFlight && len(r.merged) >= RecursionTreeArity {
		r.start(&r.merged, RecursionTreeArity)
	}
}

// start merges the first k ready nodes of a kind in the background; the
// result is a ready merged node. Called with mu held.
func (r *Reducer) start(ready *[]Node, k int) {
	children := slices.Clone((*ready)[:k])
	*ready = slices.Delete(*ready, 0, k)
	r.inFlight++
	go func() {
		node, err := r.merger.Merge(children)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.inFlight--
		switch {
		case err != nil:
			if r.err == nil {
				r.err = err
			}
		default:
			r.merged = append(r.merged, node)
			r.startFullMerges()
		}
		r.cond.Broadcast()
	}()
}
