//! The recursive verification of a proof.
//!
//! A proof is the image of one devmode proof of the verifier package, encoded for the start of the
//! guest's input region. The statement it is verified against is built here, never read from the
//! image: 32 cells holding the bytes of the hash of the proof's public input, then 8 cells holding
//! the limbs of its program VK, each a 4-byte big-endian canonical KoalaBear element.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const devmode_system = @import("devmode_system");

const Allocator = std.mem.Allocator;
const koalabear = verifier_ray.field.koalabear;
const verifier = verifier_ray.verifier;

pub const Error = error{
    /// The image is malformed, the program VK is not made of canonical field elements, or the
    /// verifier rejects the proof.
    RecursiveProofRejected,
    OutOfMemory,
};

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

    /// `proofs` is a slice of verifiable proofs, each holding its image at `proof.proof.bytes`.
    pub fn init(allocator: Allocator, proofs: anytype) Allocator.Error!Verifier {
        var longest: usize = 0;
        for (proofs) |verifiable| longest = @max(longest, verifiable.proof.proof.bytes.len);
        return .{
            .image = try allocator.alignedAlloc(u8, .of(verifier.VerifyInput), longest),
            .workspace = try allocator.create(Workspace),
        };
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
        verifier_ray.image_relocation.rebase(image.ptr, image.len, verifier_ray.image_relocation.guest_base, address);
        verifier_ray.image_relocation.validateImage(image.ptr, image.len, address) catch return error.RecursiveProofRejected;
        const input: *const verifier.VerifyInput = @ptrCast(@alignCast(image.ptr));
        verifier.verifyWithWorkspace(devmode_system.system_0_spec, devmode_system.system_0_systems, input.proof, &cells, self.workspace) catch return error.RecursiveProofRejected;
    }
};
