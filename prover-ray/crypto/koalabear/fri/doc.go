// Package fri implements multi-degree FRI over the KoalaBear field and a batch
// polynomial-commitment scheme built on top of it.
//
// # Committing
//
// A batch of committed polynomials is a [MultiSizeTable]: one [SizedTable] per
// power-of-two size, holding base-field and extension-field rows. [Commit]
// Reed-Solomon encodes every row and Merkleizes the result into a single
// [Tree].
//
// That tree is 3-ary. A node combines its two children with an optional
// auxiliary digest, Nodes[i] = H(Nodes[2i+1], Nodes[2i+2], Aux[i]), and the
// auxiliary digest at depth d commits to the rows whose encoded size is 2^d.
// Polynomials of every size therefore share one tree and one root: a size-2^d
// row is reachable from any leaf's branch, because every branch passes through
// exactly one node at depth d.
//
// # Folding
//
// FRI folds the committed codeword one round at a time, halving its length
// against a challenge supplied by the caller, and commits each intermediate
// layer in its own tree. Levels whose codeword length matches the running
// polynomial are absorbed at the round where the lengths coincide, which is
// what makes the scheme multi-degree. Folding stops at
// Params.LogFinalPolySize, and the final polynomial is revealed outright.
//
// # Querying
//
// The verifier samples Params.NumQueries positions. Each query opens the input
// trees and every running layer, and checks that the openings fold
// consistently from one layer to the next.
//
// Query branches all pass through the same upper part of a tree, so that part
// is sent once as a [MerkleCap] rather than repeated in every branch. The cap
// is authenticated against the root once per tree; each branch is then
// authenticated only from the frontier down. See merkleCapDepth for how the
// frontier depth is chosen.
//
// # PCS
//
// The PCS layer turns the above into a batch commitment scheme: several
// batches are committed independently, opened at a single point zeta shared by
// all of them, and reduced to one FRI instance over the DEEP quotients. Its
// column layout, alpha_DEEP assignment and prover/verifier call sequence are
// documented at the head of pcs.go.
//
// Fiat-Shamir is the caller's responsibility throughout. Every method that
// needs a challenge takes it as a parameter; nothing in this package reaches
// into a transcript.
package fri
