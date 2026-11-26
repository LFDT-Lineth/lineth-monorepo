//! The recursive verification of one proof: the proof image of the devmode system is
//! accepted under the program VK and for the public-input hash it was proved for, and rejected
//! under any other statement or once a round commitment is changed.

const std = @import("std");

const devmode_prover = @import("devmode_prover");
const lineth_recursion = @import("lineth_recursion");

const Allocator = std.mem.Allocator;
const recursion = lineth_recursion.recursion;

const program_vk: [32]u8 = @splat(0x2a);
const public_input_hash: [32]u8 = @splat(0x51);

const VerifiableProof = struct { proof: struct { proof: struct { bytes: []const u8 } } };

/// What a test verifies one proof with: its image, and the verifier sized to it.
const Case = struct {
    image: []u8,
    verifier: recursion.Verifier,

    fn init(a: Allocator, vk: [32]u8, hash: [32]u8) !Case {
        const image = try devmode_prover.prove(a, vk, hash);
        const proofs = [_]VerifiableProof{.{ .proof = .{ .proof = .{ .bytes = image } } }};
        return .{ .image = image, .verifier = try recursion.Verifier.init(a, &proofs) };
    }
};

test "a proof is accepted under the program VK and for the public-input hash it was proved for" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    try proved.verifier.verify(program_vk, proved.image, public_input_hash);
    // The verifier is reusable: the same image verifies again.
    try proved.verifier.verify(program_vk, proved.image, public_input_hash);
}

test "a proof for another public-input hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    var other = public_input_hash;
    other[31] ^= 0x01;
    try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(program_vk, proved.image, other));
}

test "a proof of another program VK is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    var other = program_vk;
    other[31] ^= 0x01;
    try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(other, proved.image, public_input_hash));
}

test "a program VK with a limb that is not a canonical field element is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    const modulus = [4]u8{ 0x7f, 0x00, 0x00, 0x01 };
    for ([_][4]u8{ modulus, .{ 0xff, 0xff, 0xff, 0xff } }) |limb| {
        var vk = program_vk;
        @memcpy(vk[12..16], &limb);
        try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(vk, proved.image, public_input_hash));
    }
}

test "a proof image with a changed round commitment is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    proved.image[try devmode_prover.commitmentOffset(arena.allocator(), proved.image)] ^= 0x01;
    try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(program_vk, proved.image, public_input_hash));
}

test "a truncated or empty proof image is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proved = try Case.init(arena.allocator(), program_vk, public_input_hash);
    for ([_]usize{ 0, 1, proved.image.len / 2, proved.image.len - 1 }) |len| {
        try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(program_vk, proved.image[0..len], public_input_hash));
    }
}

test "a proof longer than the image the verifier is sized to is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proved = try Case.init(a, program_vk, public_input_hash);
    const longer = try std.mem.concat(a, u8, &.{ proved.image, &.{0} });
    try std.testing.expectError(error.RecursiveProofRejected, proved.verifier.verify(program_vk, longer, public_input_hash));
}
