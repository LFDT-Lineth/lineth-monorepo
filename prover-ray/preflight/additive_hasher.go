package preflight

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// AdditiveHasher maps Merkle roots to an additive group P so that the
// aggregate A = Σ Hash(R_i) is independent of the order in which the roots
// are combined. This commutativity is what lets every shard derive the same
// shared randomness from the same set of cross-shard column roots without any
// coordinator-imposed ordering.
//
// Typical implementations map to an elliptic-curve group (e.g. multi-set
// hash via a curve point for each Octuplet chunk); the group law is then
// point addition.
type AdditiveHasher[P any] interface {
	// Hash maps a Merkle tree root to an element of the additive group.
	Hash(root field.Octuplet) P

	// Combine is the group operation. It must be commutative and associative
	// so that the order of the R_i does not affect the aggregate.
	Combine(a, b P) P

	// Identity returns the neutral element of the group (the element e such
	// that Combine(e, x) == x for all x).
	Identity() P

	// ToSeed converts an accumulated group element to the Fiat-Shamir seed
	// octuplet that is passed to [wiop.Runtime.SetFSState]. The shared
	// permutation challenges α and β are then derived from the post-seed FS
	// state, which is identical on every shard because the accumulated group
	// element is order-independent.
	ToSeed(p P) field.Octuplet
}
