package conglomeration

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/stretchr/testify/require"
)

// Positions of the roles in the public inputs of [testSystem]: the product on
// the single bus handle, then the program VK, the guest output and, when
// seeded, γ and the contribution.
const (
	busPos          = 0
	programVKPos    = busPos + 1
	guestOutputPos  = programVKPos + risc5.NumProgramVK
	seedPos         = guestOutputPos + risc5.NumGuestPublicOutputs
	contributionPos = seedPos + risc5.NumSharedRandomness
)

// testSystem declares a system with one message-bus handle and the public
// inputs of a RISCV-ZKC shard proof registered under their tags, the bus
// product as an extension cell, without compiling it: the mergers only read
// the public inputs by tag.
func testSystem(seeded bool) *wiop.System {
	sys := wiop.NewSystemf("conglomeration-test")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf("test")
	mod := sys.NewSizedModule(ctx.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(ctx.Childf("col"), r0)
	sys.NewMessageBusSend(ctx.Childf("send"), "shard", "route", wiop.NewTable(col.View()))
	sys.NewMessageBusReceive(ctx.Childf("recv"), "shard", "route", wiop.NewTable(col.View()))
	register := func(tag wiop.PublicInputTag, n int, ext bool) {
		for i := range n {
			sys.RegisterPublicInputs(tag, r0.NewCell(ctx.Childf("%s-%d", tag, i), ext), i)
		}
	}
	register(messagebus.PublicInputTag, 1, true)
	register(risc5.ProgramVKPI, risc5.NumProgramVK, false)
	register(risc5.GuestPublicOutputsPI, risc5.NumGuestPublicOutputs, false)
	if seeded {
		register(risc5.SharedRandomnessPI, risc5.NumSharedRandomness, false)
		register(risc5.SharedRandomnessContributionPI, risc5.NumSharedRandomnessContributions, false)
	}
	return sys
}

func elem(v uint) field.Gen { return field.ElemFromBase(field.NewElement(uint64(v))) }

// shardNode is a shard proof over sys with bus product bus, program VK 7 and
// guest output 9; its seed and contribution, if any, are zero.
func shardNode(sys *wiop.System, i uint, bus field.Gen) Node {
	pub := make(wiop.PublicInput, len(sys.PublicInputs))
	pub[busPos] = bus
	for k := programVKPos; k < guestOutputPos; k++ {
		pub[k] = elem(7)
	}
	for k := guestOutputPos; k < len(pub); k++ {
		pub[k] = elem(9)
	}
	for k := seedPos; k < len(pub); k++ {
		pub[k] = elem(0)
	}
	return Node{Coverage: []uint{i}, Pub: pub}
}

// reduce adds the nodes, in order, to a reducer over merger and closes it.
func reduce(merger Merger, maxInFlight int, nodes ...Node) (Node, error) {
	r := NewReducer(merger, maxInFlight)
	for _, nd := range nodes {
		if err := r.Add(nd); err != nil {
			return Node{}, err
		}
	}
	return r.Close()
}

func requireElemEqual(t *testing.T, want, got field.Gen, msg string) {
	t.Helper()
	diff := got.Sub(want)
	require.True(t, diff.IsZero(), msg)
}

// arityMerger records the number of children of every merge, per kind of
// children.
type arityMerger struct {
	MockMerger
	mu      sync.Mutex
	arities map[Kind][]int
}

func (a *arityMerger) Merge(children []Node) (Node, error) {
	a.mu.Lock()
	a.arities[children[0].Kind] = append(a.arities[children[0].Kind], len(children))
	a.mu.Unlock()
	return a.MockMerger.Merge(children)
}

// arrivalOrder returns the shard indexes 0..n-1 in a scrambled, deterministic
// order.
func arrivalOrder(n int) []uint {
	order := make([]uint, n)
	for k, i := range rand.New(rand.NewSource(int64(n))).Perm(n) {
		order[k] = uint(i)
	}
	return order
}

// TestReducer checks that a Reducer reduces any number of shard proofs,
// arriving in any order, to one merged node covering all of them and carrying
// the product of their bus products, merging [RecursionTreeArity] nodes of a
// kind at a time and fewer only in the last merge of each kind; a lone shard
// is lifted.
func TestReducer(t *testing.T) {
	sys := testSystem(false)
	for _, arrivals := range [][]uint{{0}, {2, 0, 1}, {5, 6, 0, 2, 1, 3, 4}, arrivalOrder(13), arrivalOrder(36), arrivalOrder(37)} {
		t.Run(fmt.Sprintf("%d shards", len(arrivals)), func(t *testing.T) {
			merger := &arityMerger{MockMerger: NewMockMerger(sys), arities: map[Kind][]int{}}
			nodes := make([]Node, len(arrivals))
			for k, i := range arrivals {
				nodes[k] = shardNode(sys, i, elem(i+1))
			}
			root, err := reduce(merger, 2, nodes...)
			require.NoError(t, err, "arrivals %v", arrivals)

			want := make([]uint, len(arrivals))
			factorial := elem(1)
			for i := range want {
				want[i] = uint(i)
				factorial = factorial.Mul(elem(uint(i) + 1))
			}
			require.Equal(t, want, root.Coverage, "arrivals %v: the root should cover every shard", arrivals)
			require.Equal(t, Merged, root.Kind, "arrivals %v: the root should be a merged node", arrivals)
			requireElemEqual(t, factorial, root.Pub[busPos], "the root bus product should be the product of the shards'")
			require.Equal(t, wiop.Proof{}, root.Proof, "a mock merge carries no proof")
			for kind, arities := range merger.arities {
				for k, arity := range arities[:len(arities)-1] {
					require.Equal(t, RecursionTreeArity, arity, "arrivals %v: only the last merge of kind %d may have fewer children, merge %d has %d", arrivals, kind, k, arity)
				}
			}
			if len(arrivals) == 1 {
				require.Equal(t, map[Kind][]int{Shard: {1}}, merger.arities, "a lone shard should be lifted by a single-child merge")
			}
		})
	}
}

// countingMerger records the peak number of concurrent Merge calls, holding
// the first one until release is closed so that the reducer has the chance to
// start more.
type countingMerger struct {
	MockMerger
	release chan struct{}
	started chan struct{} // signalled once by the first call

	mu             sync.Mutex
	inFlight, peak int
}

func (a *countingMerger) Merge(children []Node) (Node, error) {
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
	return a.MockMerger.Merge(children)
}

// TestReducer_MaxInFlight checks that a Reducer never runs more than
// maxInFlight merges at once.
func TestReducer_MaxInFlight(t *testing.T) {
	sys := testSystem(false)
	merger := &countingMerger{
		MockMerger: NewMockMerger(sys),
		release:    make(chan struct{}),
		started:    make(chan struct{}, 1),
	}
	// Enough shards for two full merges that could run at once: the first
	// merge is held until release while every other shard is added, so a
	// second one could start if the bound were not enforced.
	r := NewReducer(merger, 1)
	for i := range 2 * RecursionTreeArity {
		require.NoError(t, r.Add(shardNode(sys, uint(i), elem(1))))
	}
	<-merger.started
	close(merger.release)
	_, err := r.Close()
	require.NoError(t, err)
	require.Equal(t, 1, merger.peak, "at most one merge should run at once")
}

// TestMockMerger_Merge checks the checks of a mock merged node.
func TestMockMerger_Merge(t *testing.T) {
	sys := testSystem(false)
	merger := NewMockMerger(sys)
	otherVK, otherOutput, otherShape := shardNode(sys, 1, elem(2)), shardNode(sys, 1, elem(2)), shardNode(sys, 1, elem(2))
	otherVK.Pub[programVKPos], otherOutput.Pub[guestOutputPos] = elem(8), elem(8)
	otherShape.Pub = otherShape.Pub[:2]
	merged := shardNode(sys, 1, elem(2))
	merged.Kind = Merged

	tooMany := make([]Node, RecursionTreeArity+1)
	for i := range tooMany {
		tooMany[i] = shardNode(sys, uint(i), elem(1))
	}

	cases := []struct {
		name     string
		children []Node
		wantErr  string
	}{
		{name: "merges coverage and multiplies bus", children: []Node{shardNode(sys, 4, elem(2)), shardNode(sys, 0, elem(3)), shardNode(sys, 2, elem(5))}},
		{name: "no child", children: nil, wantErr: "merging 0 nodes, want 1 to 6"},
		{name: "too many children", children: tooMany, wantErr: "merging 7 nodes, want 1 to 6"},
		{name: "single merged node", children: []Node{merged}, wantErr: "merging the single merged node [1]"},
		{name: "mixed kinds", children: []Node{shardNode(sys, 0, elem(1)), merged}, wantErr: "nodes [0] and [1] of different kinds"},
		{name: "different program VK", children: []Node{shardNode(sys, 0, elem(1)), otherVK}, wantErr: "nodes [0] and [1] have different program VK"},
		{name: "different guest output", children: []Node{shardNode(sys, 0, elem(1)), otherOutput}, wantErr: "nodes [0] and [1] have different guest output"},
		{name: "different shapes", children: []Node{shardNode(sys, 0, elem(1)), otherShape}, wantErr: "node [1] has 2 public inputs"},
		{name: "overlapping coverage", children: []Node{{Coverage: []uint{0, 1}, Pub: shardNode(sys, 0, elem(1)).Pub}, shardNode(sys, 1, elem(1))}, wantErr: "shard 1 is covered twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := merger.Merge(tc.children)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []uint{0, 2, 4}, node.Coverage, "the node should cover every child")
			require.Equal(t, Merged, node.Kind, "a merge should yield a merged node")
			requireElemEqual(t, elem(30), node.Pub[busPos], "the bus product should be multiplied")
			require.Equal(t, wiop.Proof{}, node.Proof, "a mock node carries no proof")
		})
	}
}

// TestMockMerger_Merge_Lift checks that a lone shard proof is lifted into a
// merged node with the same public inputs.
func TestMockMerger_Merge_Lift(t *testing.T) {
	sys := testSystem(false)
	shard := shardNode(sys, 3, elem(5))
	node, err := NewMockMerger(sys).Merge([]Node{shard})
	require.NoError(t, err)
	require.Equal(t, Node{Kind: Merged, Coverage: []uint{3}, Pub: shard.Pub}, node, "a lift should keep the coverage and public inputs")
}

// TestMockMerger_Finalize checks the root checks: complete coverage and a
// balanced bus.
func TestMockMerger_Finalize(t *testing.T) {
	sys := testSystem(false)
	merger := NewMockMerger(sys)
	root := func(bus field.Gen, coverage ...uint) Node {
		node := shardNode(sys, 0, bus)
		node.Kind, node.Coverage = Merged, coverage
		return node
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
		{name: "shard proof", root: shardNode(sys, 0, elem(1)), numShards: 1, wantErr: "finalizing the shard proof [0], want a merged node"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := merger.Finalize(tc.root, tc.numShards)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestReducer_MergeError checks that a merge failure is returned.
func TestReducer_MergeError(t *testing.T) {
	sys := testSystem(false)
	bad := shardNode(sys, 1, elem(1))
	bad.Pub[programVKPos] = elem(8)
	_, err := reduce(NewMockMerger(sys), 1, shardNode(sys, 0, elem(1)), bad)
	require.ErrorContains(t, err, "different program VK")
}

// seededShardNode is a shard proof over the seeded sys with a balanced bus,
// the seed gamma and the given contribution.
func seededShardNode(sys *wiop.System, i uint, gamma field.Octuplet, contribution multisethashing.MSetHash) Node {
	node := shardNode(sys, i, elem(1))
	for k, g := range gamma {
		node.Pub[seedPos+k] = field.ElemFromBase(g)
	}
	for k, c := range contribution {
		node.Pub[contributionPos+k] = field.ElemFromBase(c)
	}
	return node
}

// TestMockMerger_SharedRandomness checks that the children must agree on
// γ, that their contributions add up, and that the root's γ must be the seed
// derived from the sum of the contributions.
func TestMockMerger_SharedRandomness(t *testing.T) {
	sys := testSystem(true)
	merger := NewMockMerger(sys)
	// More shards than the arity, so that contributions add over two levels.
	contributions := make([]multisethashing.MSetHash, RecursionTreeArity+2)
	for i := range contributions {
		contributions[i] = multisethashing.Hash(field.Octuplet{field.NewElement(uint64(i + 1))})
	}
	sum := multisethashing.Identity()
	for _, c := range contributions {
		sum = multisethashing.Combine(sum, c)
	}
	gamma := multisethashing.ToSeed(sum)
	nodes := make([]Node, len(contributions))
	for i, c := range contributions {
		nodes[i] = seededShardNode(sys, uint(i), gamma, c)
	}

	t.Run("root seed derives from the contributions", func(t *testing.T) {
		root, err := reduce(merger, 1, nodes...)
		require.NoError(t, err)
		require.Equal(t, sum, messagebus.GetSharedRandomnessContribution(sys, root.Pub), "the contributions should add up")
		_, err = merger.Finalize(root, uint(len(nodes)))
		require.NoError(t, err)
	})

	t.Run("different seeds", func(t *testing.T) {
		other := seededShardNode(sys, 1, field.Octuplet{}, contributions[1])
		_, err := merger.Merge([]Node{nodes[0], other})
		require.ErrorContains(t, err, "different shared randomness seed")
	})

	t.Run("seed not derived from the contributions", func(t *testing.T) {
		// A single shard's seed must be derived from its own contribution.
		lifted, err := merger.Merge([]Node{nodes[0]})
		require.NoError(t, err)
		_, err = merger.Finalize(lifted, 1)
		require.ErrorContains(t, err, "not derived from the shards' contributions")
	})
}
