// Package backend wires the prover-ray proving kernel into a job-processing
// backend.
//
// It has two responsibilities:
//
//  1. Setup (once): load RISCV-ZKC.bin, run circuit compiler passes, hold a
//     reusable [Core] ready to prove.
//
//  2. Per-proof: for each [Job], load the guest ELF named by
//     [Job.GuestProgramID], decode the guest input, build the three ZkC input
//     blobs, invoke the shared prove core, and return a serialized [Result].
//
// This package does not own request polling or coordinator JSON parsing;
// callers construct Jobs and pass them to [Core.Prove].
//
// # Status
//
// This is a first-iteration mock. For L2 execution jobs, the job adapter uses
// utils/ssz.EncodeStatelessInput to turn a coordinator payload into the framed
// bytes carried by [Job.Payload]. [Core.Prove] passes those bytes through
// [decodePayload], and the guest data-section builder adds the guest's length
// prefix while building the ZkC input blobs. [SerializeProof] (wiop.Proof →
// wire bytes) is still stubbed and returns [ErrNotImplemented].
//
// With sharding ([Config.ShardSize]), the shard proofs of a job are reduced to
// one node by a [conglomeration.Merger]. The implementations,
// [conglomeration.MockMerger] and [conglomeration.RecursionMerger], are for dev
// and test: neither verifies the children's proofs in-circuit (the recursion
// merger verifies them in Go, through the mock in-machine verifier), so a
// sharded job in production returns [ErrNotImplemented] until the in-machine
// verifier does. A merged node's public inputs are the shard layout followed
// by the recursion machine's key box (see package recursionguests/node), so
// they are wider than a shard proof's.
//
// Circuit compiler passes and [wiop.Materialize] are also not yet wired
// in [New]; see the inline comments in [core.go].
package backend
