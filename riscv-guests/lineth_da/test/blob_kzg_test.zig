const std = @import("std");

const lineth_da = @import("lineth_da");
const reference_vectors = @import("reference_vectors");
const support = @import("support");

const blob_kzg = lineth_da.encoding.blob_kzg;

const gpa = std.testing.allocator;
const io = std.testing.io;

const blob_len = blob_kzg.blob_len;

const Case = reference_vectors.Case;

fn loadSuite(suite: []const u8) !std.ArrayList(Case) {
    return reference_vectors.loadSuite(gpa, io, suite);
}

fn freeCases(cases: *std.ArrayList(Case)) void {
    reference_vectors.freeCases(gpa, cases);
}

fn expectCase(case: Case, ok: bool) !void {
    if (!ok) std.debug.print("reference vector {s} fails\n", .{case.name});
    try std.testing.expect(ok);
}

fn hexOf(bytes: []const u8) ![]u8 {
    const hex = try gpa.alloc(u8, 2 + bytes.len * 2);
    hex[0..2].* = "0x".*;
    _ = std.fmt.bufPrint(hex[2..], "{x}", .{bytes}) catch unreachable;
    return hex;
}

test "blob proofs verify exactly as the reference vectors say" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);

    var accepted: usize = 0;
    var rejected: usize = 0;
    for (cases.items) |case| {
        const verified = if (case.blobArray()) |blob|
            if (case.commitmentArray()) |commitment|
                if (case.proofArray()) |proof| blob_kzg.verify(blob, commitment, proof) else false
            else
                false
        else
            false;
        // The reference answers `true` for a verifying proof, `false` for one that does not verify
        // and null for malformed input; only a verifying proof is accepted.
        try expectCase(case, verified == std.mem.eql(u8, case.output, "true"));
        if (verified) accepted += 1 else rejected += 1;
    }
    try std.testing.expectEqual(9, accepted);
    try std.testing.expectEqual(20, rejected);
}

/// A blob of 4096 random field elements, each below 2^248.
fn randomBlob(seed: u64) *[blob_len]u8 {
    const blob = gpa.create([blob_len]u8) catch @panic("out of memory");
    var prng = std.Random.DefaultPrng.init(seed);
    prng.random().bytes(blob);
    for (0..blob_len / 32) |index| blob[index * 32] = 0;
    return blob;
}

test "a commitment and proof computed from a blob verify for that blob only" {
    const blob = randomBlob(1);
    defer gpa.destroy(blob);
    const other = randomBlob(2);
    defer gpa.destroy(other);
    const opening = support.kzg.open(blob);
    const other_opening = support.kzg.open(other);

    try std.testing.expect(blob_kzg.verify(blob, &opening.commitment, &opening.proof));

    // The blob, the commitment and the proof each have to be the ones of the opening.
    try std.testing.expect(!blob_kzg.verify(other, &opening.commitment, &opening.proof));
    try std.testing.expect(!blob_kzg.verify(blob, &other_opening.commitment, &opening.proof));
    try std.testing.expect(!blob_kzg.verify(blob, &opening.commitment, &other_opening.proof));
    var changed = blob.*;
    changed[blob_len - 1] ^= 1;
    try std.testing.expect(!blob_kzg.verify(&changed, &opening.commitment, &opening.proof));
}

test "challenges match the reference vectors" {
    var cases = try loadSuite("compute_challenge");
    defer freeCases(&cases);

    try std.testing.expect(cases.items.len >= 9);
    for (cases.items) |case| {
        const challenge = blob_kzg.computeChallenge(case.blobArray().?, case.commitmentArray().?);
        const hex = try hexOf(&challenge.toBytes());
        defer gpa.free(hex);
        try expectCase(case, std.mem.eql(u8, hex, case.output));
    }
}

test "the versioned hash of a commitment is 0x01 followed by its sha256 without the first byte" {
    // The compressed point at infinity: sha256 5f0657f3…4014.
    var infinity: [48]u8 = @splat(0);
    infinity[0] = 0xc0;
    const expected: [32]u8 = .{
        0x01, 0x06, 0x57, 0xf3, 0x75, 0x54, 0xc7, 0x81, 0x40, 0x2a, 0x22, 0x91, 0x7d, 0xee, 0x2f, 0x75,
        0xde, 0xf7, 0xab, 0x96, 0x6d, 0x7b, 0x77, 0x09, 0x05, 0x39, 0x8e, 0xba, 0x3c, 0x44, 0x40, 0x14,
    };
    try std.testing.expectEqualSlices(u8, &expected, &blob_kzg.versionedHash(&infinity));
}

/// A verifying case of the reference vectors.
fn validCase(cases: []const Case, name: []const u8) !Case {
    for (cases) |case| if (std.mem.eql(u8, case.name, name)) return case;
    return error.MissingCase;
}

fn expectRejected(what: []const u8, verified: bool) !void {
    if (verified) std.debug.print("a proof verified despite {s}\n", .{what});
    try std.testing.expect(!verified);
}

test "a proof verifies only for its own blob and commitment" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);
    const first = try validCase(cases.items, "verify_blob_kzg_proof_case_correct_proof_2");
    const second = try validCase(cases.items, "verify_blob_kzg_proof_case_correct_proof_3");
    const blob = first.blobArray().?;
    const commitment = first.commitmentArray().?;
    const proof = first.proofArray().?;
    try std.testing.expect(blob_kzg.verify(blob, commitment, proof));

    try expectRejected("the proof of another blob", blob_kzg.verify(blob, commitment, second.proofArray().?));
    try expectRejected("the commitment of another blob", blob_kzg.verify(blob, second.commitmentArray().?, proof));
    try expectRejected("the blob of another commitment", blob_kzg.verify(second.blobArray().?, commitment, proof));

    // A changed element: the low byte of an element, which stays a field element.
    const changed = try gpa.dupe(u8, blob);
    defer gpa.free(changed);
    changed[100 * 32 + 31] ^= 1;
    try expectRejected("a changed blob element", blob_kzg.verify(changed[0..blob_len], commitment, proof));

    var flipped_commitment = commitment.*;
    flipped_commitment[47] ^= 1;
    try expectRejected("a commitment with a flipped bit", blob_kzg.verify(blob, &flipped_commitment, proof));
    var flipped_proof = proof.*;
    flipped_proof[47] ^= 1;
    try expectRejected("a proof with a flipped bit", blob_kzg.verify(blob, commitment, &flipped_proof));
}
