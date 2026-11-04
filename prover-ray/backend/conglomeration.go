package backend

import (
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Node is a proof in the conglomeration tree: a shard proof, whose Coverage is
// its single shard index, or an aggregation node, whose Coverage is the sorted,
// duplicate-free union of its children's. Shard proofs and nodes expose the
// same public-input layout, so a node can be the child of another node at any
// depth.
type Node struct {
	Coverage []uint
	Proof    wiop.Proof
	Pub      wiop.PublicInput
}

// Aggregator performs the 2-to-1 aggregation steps of the conglomeration tree
// and the root finalisation. [recursionAggregator] implements it with the
// recursion machine's node program, without verifying the children yet, and
// [mockAggregator] without proofs at all, for the reducer's unit tests; the
// production implementation verifies the children's proofs in the machine.
type Aggregator interface {
	// Aggregate verifies two children and returns their parent, covering the
	// union of their coverage. It rejects overlapping coverage, a child of
	// another public-input shape, and children that disagree on the program VK
	// or the guest output.
	Aggregate(left, right Node) (Node, error)
	// Finalize runs the root checks on the node covering all numShards shards:
	// its coverage is exactly {0..numShards-1} and every message-bus handle
	// balances. It returns the root proof.
	Finalize(root Node, numShards uint) (Node, error)
}

// mockAggregator is the [Aggregator] of the reducer's unit tests. It produces no proof: an
// aggregated node's Proof is the zero wiop.Proof, so a mock node can never
// pass for a verifiable proof, and its Pub is the left child's with the
// message-bus products multiplied. A shard proof that reaches Finalize
// unaggregated keeps its proof.
type mockAggregator struct {
	layout piLayout
}

// Aggregate implements [Aggregator].
func (m mockAggregator) Aggregate(left, right Node) (Node, error) {
	coverage, err := mergeCoverage(left.Coverage, right.Coverage)
	if err != nil {
		return Node{}, err
	}
	if len(left.Pub) != len(right.Pub) {
		return Node{}, fmt.Errorf("nodes %v and %v have public inputs of different shapes",
			left.Coverage, right.Coverage)
	}
	for _, shared := range []struct {
		name      string
		positions []int
	}{{"program VK", m.layout.programVK}, {"guest output", m.layout.guestOutput}} {
		for _, pos := range shared.positions {
			if diff := left.Pub[pos].Sub(right.Pub[pos]); !diff.IsZero() {
				return Node{}, fmt.Errorf("nodes %v and %v have different %s",
					left.Coverage, right.Coverage, shared.name)
			}
		}
	}
	pub := slices.Clone(left.Pub)
	for _, pos := range m.layout.bus {
		pub[pos] = left.Pub[pos].Mul(right.Pub[pos])
	}
	return Node{Coverage: coverage, Pub: pub}, nil
}

// Finalize implements [Aggregator].
func (m mockAggregator) Finalize(root Node, numShards uint) (Node, error) {
	if err := checkCoverageComplete(root.Coverage, numShards); err != nil {
		return Node{}, err
	}
	bus := make([]field.Gen, len(m.layout.bus))
	for h, pos := range m.layout.bus {
		bus[h] = root.Pub[pos]
	}
	if err := checkBusBalanced(bus); err != nil {
		return Node{}, err
	}
	return root, nil
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

// checkBusBalanced checks that the product over all shards on each message-bus
// handle is one.
func checkBusBalanced(bus []field.Gen) error {
	for h, p := range bus {
		if diff := p.Sub(field.ElemOne()); !diff.IsZero() {
			return fmt.Errorf("message-bus handle %d does not balance across the shards: product is %v", h, p)
		}
	}
	return nil
}

// pairingPolicy picks the positions i < j in ready of the two nodes to
// aggregate next; ok is false when none can be paired yet. [pairAnyTwo]
// aggregates any two proofs in any order and tree shape
type pairingPolicy func(ready []Node) (i, j int, ok bool)

// pairAnyTwo pairs the first two ready nodes.
func pairAnyTwo(ready []Node) (int, int, bool) {
	return 0, 1, len(ready) >= 2
}

// defaultMaxInFlightAggregations bounds the concurrent [Aggregator.Aggregate]
// calls of [conglomerate]: a real aggregation node is a full proof and
// competes for memory with shard proving.
const defaultMaxInFlightAggregations = 1

// conglomerate drains the n shard proofs received on in and reduces them to a
// single node with agg, pairing per pair as proofs arrive, at most maxInFlight
// aggregations at a time. n and maxInFlight must be at least 1; agg is only
// called when n > 1. It does not run the root checks: the caller calls
// [Aggregator.Finalize] if the node covers all the shards. It fails if in
// closes before n proofs arrived, or if pair leaves more than one node
// unpaired.
func conglomerate(in <-chan Node, n int, agg Aggregator, pair pairingPolicy, maxInFlight int) (Node, error) {
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
	for {
		for inFlight < maxInFlight {
			i, j, ok := pair(ready)
			if !ok {
				break
			}
			if i < 0 || i >= j || j >= len(ready) {
				return Node{}, fmt.Errorf("pairing policy returned positions %d and %d for %d ready nodes", i, j, len(ready))
			}
			left, right := ready[i], ready[j]
			ready = slices.Delete(ready, j, j+1)
			ready = slices.Delete(ready, i, i+1)
			inFlight++
			go func() {
				node, err := agg.Aggregate(left, right)
				done <- result{node: node, err: err}
			}()
		}
		if received == n && inFlight == 0 {
			if len(ready) != 1 {
				return Node{}, fmt.Errorf("the pairing policy left %d nodes unpaired", len(ready))
			}
			return ready[0], nil
		}
		if received == n {
			in = nil // all the shard proofs arrived, wait for the aggregations only
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
