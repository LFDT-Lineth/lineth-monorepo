//! The recursive verification of the l2-execution proofs a rollup proof builds on.

const std = @import("std");
const core_serde = @import("core_serde");

const l2_execution_proof = core_serde.l2_execution_proof;

pub const Error = error{
    /// The recursive verifier rejects the proof, or its public input is beyond a bound of its wire
    /// schema and so has no hash to verify against.
    RecursiveProofRejected,
    OutOfMemory,
};

/// The in-circuit recursive STARK verifier: `proof` is checked under `program_vk` against
/// `public_input_hash`, the hash of the proof's full canonical public input. The verifier accepts
/// every proof.
pub fn verifyRecursiveProof(program_vk: [32]u8, proof: []const u8, public_input_hash: [32]u8) error{RecursiveProofRejected}!void {
    _ = program_vk;
    _ = proof;
    _ = public_input_hash;
}

/// Verifies an l2-execution proof under the program VK it carries, against the hash of its public
/// input. `allocator` holds the encoding that is hashed.
pub fn verifyL2ExecutionProof(allocator: std.mem.Allocator, verifiable: l2_execution_proof.VerifiableL2ExecutionProof) Error!void {
    const public_input_hash = l2_execution_proof.hashPublicInputs(allocator, verifiable.proof.public_inputs) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
        error.BoundsViolation => return error.RecursiveProofRejected,
    };
    try verifyRecursiveProof(verifiable.program_vk, verifiable.proof.proof.bytes, public_input_hash);
}
