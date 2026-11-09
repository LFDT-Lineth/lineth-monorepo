//! The recursive verification of the l2-execution proofs a rollup proof builds on.
//!
//! A proof is the image of one devmode proof of the verifier package, encoded for the start of the
//! guest's input region. The statement it is verified against is built here, never read from the
//! image: 32 cells holding the bytes of the hash of the proof's public input, then 8 cells holding
//! the limbs of its program VK, each a 4-byte big-endian canonical KoalaBear element.

const std = @import("std");
const core_serde = @import("core_serde");
const verifier_ray = @import("verifier_ray");
const devmode_system = @import("devmode_system");

const Allocator = std.mem.Allocator;
const l2_execution_proof = core_serde.l2_execution_proof;
const koalabear = verifier_ray.field.koalabear;
const verifier = verifier_ray.verifier;

pub const Error = error{
    /// The image is malformed, the program VK is not made of canonical field elements, the
    /// verifier rejects the proof, or the proof's public input is beyond a bound of its wire schema
    /// and so has no hash to verify against.
    RecursiveProofRejected,
    OutOfMemory,
};

/// The address the proof images are encoded for.
pub const image_base: usize = 0x08800000;

const hash_cells = 32;
const vk_limbs = 8;
comptime {
    std.debug.assert(devmode_system.system_0_public_input.refs.len == hash_cells + vk_limbs);
}

const Workspace = verifier.Workspace(devmode_system.system_0_public_input);

/// The memory of verifying one proof after the other: the aligned copy a proof image is relocated
/// in, sized to the longest proof, and the verifier's workspace.
pub const Verifier = struct {
    image: []align(@alignOf(verifier.VerifyInput)) u8,
    workspace: *Workspace,

    pub fn init(allocator: Allocator, proofs: []const l2_execution_proof.VerifiableL2ExecutionProof) Allocator.Error!Verifier {
        var longest: usize = 0;
        for (proofs) |verifiable| longest = @max(longest, verifiable.proof.proof.bytes.len);
        return .{
            .image = try allocator.alignedAlloc(u8, .of(verifier.VerifyInput), longest),
            .workspace = try allocator.create(Workspace),
        };
    }

    /// Verifies an l2-execution proof under the program VK it carries, against the hash of its
    /// public input. `allocator` holds the encoding that is hashed.
    pub fn verifyL2ExecutionProof(self: Verifier, allocator: Allocator, verifiable: l2_execution_proof.VerifiableL2ExecutionProof) Error!void {
        const public_input_hash = l2_execution_proof.hashPublicInputs(allocator, verifiable.proof.public_inputs) catch |err| switch (err) {
            error.OutOfMemory => return error.OutOfMemory,
            error.BoundsViolation => return error.RecursiveProofRejected,
        };
        try self.verify(verifiable.program_vk, verifiable.proof.proof.bytes, public_input_hash);
    }

    /// Checks the proof image `proof` under `program_vk` against `public_input_hash`; a proof
    /// longer than the copy this verifier holds is rejected.
    pub fn verify(self: Verifier, program_vk: [32]u8, proof: []const u8, public_input_hash: [32]u8) error{RecursiveProofRejected}!void {
        var cells: [hash_cells + vk_limbs]verifier_ray.protocol.Scalar = undefined;
        for (public_input_hash, cells[0..hash_cells]) |byte, *cell| cell.* = .{ .base = koalabear.Element.init(byte) };
        for (cells[hash_cells..], 0..) |*cell, limb| {
            const element = koalabear.Element.fromBytesCanonical(program_vk[limb * koalabear.bytes ..][0..koalabear.bytes].*) catch return error.RecursiveProofRejected;
            cell.* = .{ .base = element };
        }

        if (proof.len > self.image.len) return error.RecursiveProofRejected;
        const image = self.image[0..proof.len];
        @memcpy(image, proof);
        const address = @intFromPtr(image.ptr);
        verifier_ray.image_relocation.rebase(image.ptr, image.len, image_base, address);
        verifier_ray.image_relocation.validateImage(image.ptr, image.len, address) catch return error.RecursiveProofRejected;
        const input: *const verifier.VerifyInput = @ptrCast(@alignCast(image.ptr));
        verifier.verifyWithWorkspace(devmode_system.system_0_spec, devmode_system.system_0_systems, input.proof, &cells, self.workspace) catch return error.RecursiveProofRejected;
    }
};
