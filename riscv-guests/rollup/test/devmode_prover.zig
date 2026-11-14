//! The devmode prover of the tests and tools: one child process of verifier-ray's
//! `generate-devmode-system -prove`, spawned on first use and kept for the rest of the process, that
//! answers a public-input hash and a program VK with the proof image of the devmode system. It ends
//! with the process: its input closes when this one exits.

const std = @import("std");

const paths = @import("prover_paths");
const rollup = @import("rollup");
const verifier_ray = @import("verifier_ray");

const Allocator = std.mem.Allocator;
const verifier = verifier_ray.verifier;
const Child = std.process.Child;

var child: ?Child = null;
var threaded: std.Io.Threaded = undefined;

fn io() std.Io {
    return threaded.io();
}

fn start() !*Child {
    if (child == null) {
        threaded = .init(std.heap.page_allocator, .{});
        child = try std.process.spawn(io(), .{
            .argv = &.{ paths.devmode_prove, "-prove" },
            .stdin = .pipe,
            .stdout = .pipe,
        });
    }
    return &child.?;
}

fn readExactly(file: std.Io.File, buffer: []u8) !void {
    var filled: usize = 0;
    while (filled < buffer.len) {
        const read = try file.readStreaming(io(), &.{buffer[filled..]});
        if (read == 0) return error.ProverEnded;
        filled += read;
    }
}

/// The proof image of the devmode proof that `public_input_hash` and `program_vk` are the public
/// input of, owned by `allocator`.
pub fn prove(allocator: Allocator, program_vk: [32]u8, public_input_hash: [32]u8) ![]u8 {
    const process = try start();
    var request: [2 * 64 + 2]u8 = undefined;
    const line = try std.fmt.bufPrint(&request, "{s} {s}\n", .{
        std.fmt.bytesToHex(public_input_hash, .lower),
        std.fmt.bytesToHex(program_vk, .lower),
    });
    try process.stdin.?.writeStreamingAll(io(), line);

    var length: [4]u8 = undefined;
    try readExactly(process.stdout.?, &length);
    const image = try allocator.alloc(u8, std.mem.readInt(u32, &length, .big));
    errdefer allocator.free(image);
    try readExactly(process.stdout.?, image);
    return image;
}

/// The offset in `image` of the first byte of the first round commitment, which the verifier
/// absorbs into its transcript. The offset is read from the image relocated as the verifier reads
/// it, through the verifier's own types.
pub fn commitmentOffset(allocator: Allocator, image: []const u8) !usize {
    const copy = try allocator.alignedAlloc(u8, .of(verifier.VerifyInput), image.len);
    defer allocator.free(copy);
    @memcpy(copy, image);
    const address = @intFromPtr(copy.ptr);
    verifier_ray.image_relocation.rebase(copy.ptr, copy.len, rollup.recursion.image_base, address);
    const input: *const verifier.VerifyInput = @ptrCast(@alignCast(copy.ptr));
    for (input.proof.rounds) |*round| {
        if (round.commitment) |*commitment| return @intFromPtr(commitment) - address;
    }
    return error.ImageWithoutCommitment;
}
