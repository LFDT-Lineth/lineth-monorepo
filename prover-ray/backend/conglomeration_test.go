package backend

import (
	"errors"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// testLayout is the public-input layout of the test shard proofs: (bus
// product, program VK, guest output).
var testLayout = piLayout{bus: []int{0}, programVK: []int{1}, guestOutput: []int{2}}

func elem(v uint) field.Gen { return field.ElemFromBase(field.NewElement(uint64(v))) }

// shardNode is a shard proof of the test layout, with bus product bus.
func shardNode(i uint, bus field.Gen) Node {
	return Node{Coverage: []uint{i}, Pub: wiop.PublicInput{bus, elem(7), elem(9)}}
}

// reduce runs conglomerate over the given nodes, sent in order on a closed
// channel, expecting n of them.
func reduce(n int, agg Aggregator, pair pairingPolicy, maxInFlight int, nodes ...Node) (Node, error) {
	ch := make(chan Node, len(nodes))
	for _, nd := range nodes {
		ch <- nd
	}
	close(ch)
	return conglomerate(ch, n, agg, pair, maxInFlight)
}

func requireElemEqual(t *testing.T, want, got field.Gen, msg string) {
	t.Helper()
	diff := got.Sub(want)
	require.True(t, diff.IsZero(), msg)
}

// TestConglomerate checks that conglomerate reduces any number of shard
// proofs, arriving in any order, to one node covering all of them and carrying
// the product of their bus products.
func TestConglomerate(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	for _, arrivals := range [][]uint{{0}, {2, 0, 1}, {4, 1, 3, 0, 2}, {5, 6, 0, 2, 1, 3, 4}} {
		t.Run("", func(t *testing.T) {
			nodes := make([]Node, len(arrivals))
			for k, i := range arrivals {
				nodes[k] = shardNode(i, elem(i+1))
			}
			root, err := reduce(len(arrivals), agg, pairAnyTwo, 2, nodes...)
			require.NoError(t, err, "arrivals %v", arrivals)

			want := make([]uint, len(arrivals))
			factorial := elem(1)
			for i := range want {
				want[i] = uint(i)
				factorial = factorial.Mul(elem(uint(i) + 1))
			}
			require.Equal(t, want, root.Coverage, "arrivals %v: the root should cover every shard", arrivals)
			requireElemEqual(t, factorial, root.Pub[0], "the root bus product should be the product of the shards'")
			if len(arrivals) > 1 {
				require.Equal(t, wiop.Proof{}, root.Proof, "a mock aggregation carries no proof")
			}
		})
	}
}

// TestConglomerate_MissingProofs checks that conglomerate fails when the
// channel closes before every shard proof arrived.
func TestConglomerate_MissingProofs(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	_, err := reduce(3, agg, pairAnyTwo, 1, shardNode(1, elem(1)), shardNode(0, elem(1)))
	require.ErrorContains(t, err, "only 2 of 3 shard proofs arrived")
}

// countingAggregator records the peak number of concurrent Aggregate calls,
// holding the first one until release is closed so that the reducer has the
// chance to start more.
type countingAggregator struct {
	mockAggregator
	release chan struct{}
	started chan struct{} // signalled once by the first call

	mu             sync.Mutex
	inFlight, peak int
}

func (a *countingAggregator) Aggregate(left, right Node) (Node, error) {
	a.mu.Lock()
	a.inFlight++
	a.peak = max(a.peak, a.inFlight)
	a.mu.Unlock()
	select {
	case a.started <- struct{}{}:
	default:
	}
	<-a.release
	defer func() {
		a.mu.Lock()
		a.inFlight--
		a.mu.Unlock()
	}()
	return a.mockAggregator.Aggregate(left, right)
}

// TestConglomerate_MaxInFlight checks that conglomerate never runs more than
// maxInFlight aggregations at once.
func TestConglomerate_MaxInFlight(t *testing.T) {
	agg := &countingAggregator{
		mockAggregator: mockAggregator{layout: testLayout},
		release:        make(chan struct{}),
		started:        make(chan struct{}, 1),
	}
	nodes := make([]Node, 4)
	for i := range nodes {
		nodes[i] = shardNode(uint(i), elem(1))
	}
	ch := make(chan Node, len(nodes))
	for _, nd := range nodes {
		ch <- nd
	}
	close(ch)

	done := make(chan error, 1)
	go func() {
		_, err := conglomerate(ch, len(nodes), agg, pairAnyTwo, 1)
		done <- err
	}()
	// Once the first aggregation runs, the reducer has drained the channel and
	// started every aggregation the bound allows.
	<-agg.started
	close(agg.release)
	require.NoError(t, <-done)
	require.Equal(t, 1, agg.peak, "at most one aggregation should run at once")
}

// TestConglomerate_PolicyLeavesUnpaired checks that a pairing policy that
// never pairs is reported rather than hanging.
func TestConglomerate_PolicyLeavesUnpaired(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	never := func([]Node) (int, int, bool) { return 0, 0, false }
	_, err := reduce(2, agg, never, 1, shardNode(0, elem(1)), shardNode(1, elem(1)))
	require.ErrorContains(t, err, "left 2 nodes unpaired")
}

// TestMockAggregator_Aggregate checks the checks of a mock aggregation node.
func TestMockAggregator_Aggregate(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	otherVK, otherOutput, otherShape := shardNode(1, elem(2)), shardNode(1, elem(2)), shardNode(1, elem(2))
	otherVK.Pub[1], otherOutput.Pub[2] = elem(8), elem(8)
	otherShape.Pub = otherShape.Pub[:2]

	cases := []struct {
		name        string
		left, right Node
		wantErr     string
	}{
		{name: "merges coverage and multiplies bus", left: shardNode(0, elem(2)), right: shardNode(2, elem(3))},
		{name: "different program VK", left: shardNode(0, elem(1)), right: otherVK, wantErr: "nodes [0] and [1] have different program VK"},
		{name: "different guest output", left: shardNode(0, elem(1)), right: otherOutput, wantErr: "nodes [0] and [1] have different guest output"},
		{name: "different shapes", left: shardNode(0, elem(1)), right: otherShape, wantErr: "public inputs of different shapes"},
		{name: "overlapping coverage", left: Node{Coverage: []uint{0, 1}, Pub: shardNode(0, elem(1)).Pub}, right: shardNode(1, elem(1)), wantErr: "shard 1 is covered twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := agg.Aggregate(tc.left, tc.right)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []uint{0, 2}, node.Coverage, "the node should cover both children")
			requireElemEqual(t, elem(6), node.Pub[0], "the bus product should be multiplied")
			require.Equal(t, wiop.Proof{}, node.Proof, "a mock node carries no proof")
		})
	}
}

// TestMockAggregator_Finalize checks the root checks: complete coverage and a
// balanced bus.
func TestMockAggregator_Finalize(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	root := func(bus field.Gen, coverage ...uint) Node {
		return Node{Coverage: coverage, Pub: wiop.PublicInput{bus, elem(7), elem(9)}}
	}
	cases := []struct {
		name      string
		root      Node
		numShards uint
		wantErr   string
	}{
		{name: "complete and balanced", root: root(elem(1), 0, 1, 2), numShards: 3},
		{name: "single shard", root: root(elem(1), 0), numShards: 1},
		{name: "missing shard", root: root(elem(1), 0, 2), numShards: 3, wantErr: "missing shard 1"},
		{name: "extra shard", root: root(elem(1), 0, 1, 2, 3), numShards: 3, wantErr: "unexpected shard 3"},
		{name: "unbalanced bus", root: root(elem(2), 0, 1, 2), numShards: 3, wantErr: "does not balance across the shards"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agg.Finalize(tc.root, tc.numShards)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestConglomerate_AggregationError checks that an aggregation failure is
// returned.
func TestConglomerate_AggregationError(t *testing.T) {
	agg := mockAggregator{layout: testLayout}
	bad := shardNode(1, elem(1))
	bad.Pub[1] = elem(8)
	_, err := reduce(2, agg, pairAnyTwo, 1, shardNode(0, elem(1)), bad)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrNotImplemented))
	require.ErrorContains(t, err, "different program VK")
}
