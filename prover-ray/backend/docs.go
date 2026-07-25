// Package backend wires the prover-ray proving kernel into a job-processing
// backend.
//
// It has two responsibilities:
//
//  1. Setup (once): load RISCV-ZKC.bin and the guest ELF, run circuit
//     compiler passes, hold a reusable [Core] ready to prove.
//
//  2. Per-proof: for each [Job], decode the SSZ payload, build the three
//     ZkC pub-input blobs, invoke the shared prove core, and return a
//     serialized [Result].
//
// Delivery (gateway pull or filesystem controller) is out of scope here:
// callers pull jobs and dispatch them to [Core.Prove].
//
// # Status
//
// This is a first-iteration mock. [EncodeStatelessInput] SSZ-encodes the
// coordinator payload (single block) into the already-framed bytes carried by
// [Job.Payload]. [Core.Prove] passes those bytes through [decodePayload], and
// [sszBlobs] adds the guest's length prefix while building the ZkC pub-input
// blobs. [SerializeProof] (wiop.Proof → wire bytes) is still stubbed and
// returns [ErrNotImplemented].
//
// Circuit compiler passes and [wiop.Materialize] are also not yet wired
// in [New]; see the inline comments in [core.go].
package backend
