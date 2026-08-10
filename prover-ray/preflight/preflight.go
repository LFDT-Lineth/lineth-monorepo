// Package preflight implements the pre-phase that establishes a shared
// Fiat-Shamir seed across shards without a coordinator.
//
// Each shard receives the full collection of cross-shard column sets S_1 …
// S_n. It commits to each set with FRI (obtaining Merkle roots R_1 … R_n),
// maps each root through an [AdditiveHasher] (landing in a commutative group),
// accumulates the sum A = Σ AdditiveHash(R_i), and converts A to a
// [field.Octuplet] via [AdditiveHasher.ToSeed]. Every shard that holds the
// same S_i data produces the same octuplet regardless of processing order,
// because the group operation is commutative.
//
// The octuplet is used as the Fiat-Shamir seed: each shard's prover and
// verifier call [wiop.Runtime.SetFSState] with it inside a
// [wiop.Round.RegisterPreSamplingHook] so the shared challenges α and β are
// derived from an identical state on every participating shard.
package preflight

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
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

// Run computes the shared Fiat-Shamir seed from a collection of cross-shard
// column sets.
//
// For each set s it commits to s.Table using s.Encoders (obtaining a Merkle
// root), maps the root through hasher.Hash, and accumulates the results with
// hasher.Combine. The final accumulated value is converted to a [field.Octuplet]
// via hasher.ToSeed.
//
// The result is deterministic and order-independent as long as hasher.Combine
// is commutative and associative, ensuring every shard computes the same seed.
func Run[P any](sets []BusInputSet, hasher AdditiveHasher[P]) field.Octuplet {
	acc := hasher.Identity()
	for _, s := range sets {
		cs := fri.Commit(s.Encoders, s.Table)
		root := cs.Tree.Nodes[0]
		a := hasher.Hash(root)
		acc = hasher.Combine(acc, a)
	}
	return hasher.ToSeed(acc)
}
