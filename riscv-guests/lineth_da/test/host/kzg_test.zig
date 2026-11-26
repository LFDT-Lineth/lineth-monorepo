const std = @import("std");

const kzg_prover = @import("kzg_prover");
const lineth_da = @import("lineth_da");
const reference_vectors = @import("reference_vectors");

const blob_kzg = lineth_da.encoding.blob_kzg;

const gpa = std.testing.allocator;
const io = std.testing.io;

fn hexOf(bytes: []const u8) ![]u8 {
    return std.fmt.allocPrint(gpa, "0x{x}", .{bytes});
}

test "the prover commits to blobs as the reference vectors say and refuses invalid blobs" {
    var cases = try reference_vectors.loadSuite(gpa, io, "blob_to_kzg_commitment");
    defer reference_vectors.freeCases(gpa, &cases);

    var produced: usize = 0;
    for (cases.items) |case| {
        const commitment = if (case.blobArray()) |blob| kzg_prover.commit(blob) else error.KzgFailed;
        if (std.mem.eql(u8, case.output, "null")) {
            try std.testing.expectError(error.KzgFailed, commitment);
            continue;
        }
        const hex = try hexOf(&(try commitment));
        defer gpa.free(hex);
        try std.testing.expectEqualStrings(case.output, hex);
        produced += 1;
    }
    try std.testing.expectEqual(7, produced);
}

test "the prover proves blobs as the reference vectors say and refuses invalid inputs" {
    var cases = try reference_vectors.loadSuite(gpa, io, "compute_blob_kzg_proof");
    defer reference_vectors.freeCases(gpa, &cases);

    var produced: usize = 0;
    for (cases.items) |case| {
        const proof = if (case.blobArray()) |blob|
            if (case.commitmentArray()) |commitment| kzg_prover.prove(blob, commitment) else error.KzgFailed
        else
            error.KzgFailed;
        if (std.mem.eql(u8, case.output, "null")) {
            try std.testing.expectError(error.KzgFailed, proof);
            continue;
        }
        const hex = try hexOf(&(try proof));
        defer gpa.free(hex);
        try std.testing.expectEqualStrings(case.output, hex);
        produced += 1;
    }
    try std.testing.expectEqual(7, produced);
}

test "an opening verifies as a batch with its blobs, and a changed proof does not verify" {
    var cases = try reference_vectors.loadSuite(gpa, io, "blob_to_kzg_commitment");
    defer reference_vectors.freeCases(gpa, &cases);

    var blobs: std.ArrayList([kzg_prover.blob_len]u8) = .empty;
    defer blobs.deinit(gpa);
    var commitments: std.ArrayList([kzg_prover.point_len]u8) = .empty;
    defer commitments.deinit(gpa);
    var proofs: std.ArrayList([kzg_prover.point_len]u8) = .empty;
    defer proofs.deinit(gpa);
    for (cases.items) |case| {
        if (std.mem.eql(u8, case.output, "null")) continue;
        const opening = try kzg_prover.open(case.blobArray().?);
        try blobs.append(gpa, case.blobArray().?.*);
        try commitments.append(gpa, opening.commitment);
        try proofs.append(gpa, opening.proof);
    }

    try std.testing.expect(blob_kzg.verifyBatch(blobs.items, commitments.items, proofs.items));
    proofs.items[0][47] ^= 1;
    try std.testing.expect(!blob_kzg.verifyBatch(blobs.items, commitments.items, proofs.items));
}

test "threads that open the same blob get the same opening" {
    const blob = try gpa.create([kzg_prover.blob_len]u8);
    defer gpa.destroy(blob);
    for (blob, 0..) |*byte, i| byte.* = if (i % 32 < 2) 0 else @truncate(i *% 31);
    const expected = try kzg_prover.open(blob);

    const Worker = struct {
        fn run(shared: *const [kzg_prover.blob_len]u8, out: *kzg_prover.Opening) void {
            out.* = kzg_prover.open(shared) catch unreachable;
        }
    };
    var openings: [4]kzg_prover.Opening = undefined;
    var threads: [4]std.Thread = undefined;
    for (&threads, &openings) |*thread, *opening| thread.* = try std.Thread.spawn(.{}, Worker.run, .{ blob, opening });
    for (threads) |thread| thread.join();

    for (openings) |opening| try std.testing.expectEqualDeep(expected, opening);
}
