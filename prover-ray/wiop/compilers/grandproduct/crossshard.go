package grandproduct

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// CompileCrossShardPermutations wires up a set of cross-shard permutation
// queries to the message-bus compiler by:
//
//  1. Pre-allocating the coin round immediately after the latest participant
//     round and registering a [wiop.Round.RegisterPreSamplingHook] that seeds
//     the Fiat-Shamir state with the additive hash of the pre-distributed
//     cross-shard column sets. Because [wiop.Runtime.AdvanceRound] fires hooks
//     on both the prover and the verifier, the seed — and therefore the
//     shared challenges α and β derived from it — is identical on every
//     participating shard without any explicit verifier action.
//
//  2. Declaring one [wiop.MessageBus] Send entry per A-side table and one
//     Receive entry per B-side table, all on the same handle. When the caller
//     subsequently runs messagebus.Compile(sys), it reuses the pre-allocated
//     coin round (ensureRoundAfter finds the existing tail round), so α and β
//     land on the hooked round and derive from the shared seed.
//
// The caller must invoke messagebus.Compile(sys) and then
// grandproduct.Compile(sys) after this function returns to discharge the
// declared entries into grand-product columns and verifier checks.
//
// originShard identifies this shard; all entries in one shard's system must
// share the same originShard string (the message-bus compiler panics otherwise).
//
// handle is the bus name that groups the entries. Every participating shard
// must use the same handle for the same logical permutation relation.
//
// skipInShardCheck controls whether the message-bus compiler emits a per-shard
// "product == 1" assertion. Set it to false when the permutation is balanced
// within each shard (each shard's A multiset equals its own B multiset). Set
// it to true when rows cross shard boundaries and the balance check must be
// deferred to a cross-shard join layer.
func CompileCrossShardPermutations[P any](
	sys *wiop.System,
	originShard string,
	handle string,
	perms []*wiop.TableRelationQuery,
	sets []preflight.CrossShardSet,
	hasher preflight.AdditiveHasher[P],
	skipInShardCheck bool,
) {
	if len(perms) == 0 {
		return
	}

	var maxRound *wiop.Round
	for _, q := range perms {
		if r := q.Round(); r != nil && (maxRound == nil || r.ID > maxRound.ID) {
			maxRound = r
		}
	}
	if maxRound == nil {
		panic("wiop/compilers/grandproduct: CompileCrossShardPermutations: permutation queries reference no round-bearing column")
	}

	compCtx := sys.Context.Childf("crossshard-perm")

	// Pre-allocate the coin round. messagebus.Compile's ensureRoundAfter will
	// find this existing tail round and reuse it, placing α and β on the same
	// round where the hook fires.
	coinRound := maxRound.EnsureNext()
	coinRound.RegisterPreSamplingHook(&preflightSeedHook[P]{sets: sets, hasher: hasher})

	// Declare one Send entry per A-side table and one Receive entry per B-side
	// table, all on the same handle so the message-bus compiler aggregates
	// their contributions into a single GrandProduct per shard.
	for qi, q := range perms {
		qCtx := compCtx.Childf("perm-%d", qi)
		for ti, tab := range q.A {
			mb := sys.NewMessageBusSend(qCtx.Childf("send-%d", ti), originShard, handle, tab)
			mb.SkipInShardCheck = skipInShardCheck
		}
		for ti, tab := range q.B {
			mb := sys.NewMessageBusReceive(qCtx.Childf("recv-%d", ti), originShard, handle, tab)
			mb.SkipInShardCheck = skipInShardCheck
		}
	}

	for _, q := range perms {
		q.MarkAsReduced()
	}
}

// preflightSeedHook is a [wiop.ProverAction] registered as a PreSamplingHook
// on the coin round. It runs on both the prover and the verifier (both call
// [wiop.Runtime.AdvanceRound]), computing the shared Fiat-Shamir seed from the
// pre-distributed cross-shard column sets and injecting it via SetFSState so
// that the subsequent α and β coins are identical across shards.
type preflightSeedHook[P any] struct {
	sets   []preflight.CrossShardSet
	hasher preflight.AdditiveHasher[P]
}

// Run implements [wiop.ProverAction].
func (h *preflightSeedHook[P]) Run(rt *wiop.Runtime) {
	seed := preflight.Run(h.sets, h.hasher)
	rt.SetFSState(seed)
}
