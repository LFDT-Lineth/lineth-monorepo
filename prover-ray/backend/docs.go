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
// one node by a [conglomeration.Merger]; only a mock exists, which partial
// uses, so a sharded job in full returns [ErrNotImplemented] until the
// recursion machine lands. The modes are described in docs/prover-modes.md.
//
// Circuit compiler passes and [wiop.Materialize] are also not yet wired
// in [New]; see the inline comments in [core.go].
package backend
