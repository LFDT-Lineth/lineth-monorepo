// Package preflight implements the pre-phase that establishes γ, the shared
// Fiat-Shamir seed the message bus binds every shard against.
//
// [Run] takes the full collection of bus input sets S_1 … S_n — one per shard —
// commits to each with FRI (obtaining Merkle roots R_1 … R_n), maps each root
// into the multiset-hash group, sums them, and compresses the sum to a
// [field.Octuplet]. Because the group operation is commutative, the result does
// not depend on the order the sets are processed in, so no coordinator has to
// impose one.
//
// This runs in the orchestrator, once, before any shard proof is produced — not
// inside a proof. It cannot run inside one: it consumes every shard's data,
// while a shard's prover holds only its own, and a verifier holds none at all.
// The resulting γ is handed to each shard as a public input; see
// [github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5.RegisterSharedRandomness]
// for how it enters a shard proof, and why a shard leaves it unconstrained.
package preflight

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// The multiset hash provides the commutative group the roots are accumulated in:
// [Hash] maps a root into the group, [Combine] is the group operation,
// [Identity] is the neutral element and [ToSeed] compresses a group element back
// to an octuplet. They are re-exported here so that callers reproducing γ do not
// have to know which concrete multiset hash backs it.
var (
	Hash     = multisethashing.Hash
	Combine  = multisethashing.Combine
	Identity = multisethashing.Identity
	ToSeed   = multisethashing.ToSeed
)

// BusInputSet bundles one logical cross-shard column fragment with the RS
// encoders needed to commit to it. Build one BusInputSet per shard fragment
// before calling [Run].
type BusInputSet struct {
	// Table holds the raw (unenecoded) column data for one shard's fragment
	// of a shared logical column.
	Table fri.MultiSizeTable
	// Encoders are the per-size RS encoders matching the sizes present in
	// Table. They must satisfy the [fri.assertValidMultiEncoder] invariant.
	Encoders []*fri.RSEncoder
}

// Run computes γ, the shared Fiat-Shamir seed, from the bus input sets of every
// participating shard.
//
// For each set s it commits to s.Table using s.Encoders (obtaining a Merkle
// root) and accumulates the root; the final accumulated value is compressed
// into the returned octuplet.
//
// The result is deterministic and order-independent. Callers must pass the sets
// of *all* shards: a γ computed from a subset binds only that subset, and the
// shards left out would be proving against a seed unrelated to their own data.
func Run(sets []BusInputSet) field.Octuplet {
	acc := Identity()
	for _, s := range sets {
		cs := fri.Commit(s.Encoders, s.Table)
		acc = Combine(acc, Hash(cs.Tree.Root()))
	}
	return ToSeed(acc)
}
