//! Reads an image produced by prover-ray's encoder as a real `verifier.VerifyInput`.
//!
//! Every other check on the format is indirect: `proof_abi.zig` asserts Zig's
//! layout against pinned numbers, and prover-ray's tests assert its encoder
//! against its own copy of those numbers. Both sides can agree with the pins and
//! still disagree with each other, and prover-ray's round-trip cannot see it —
//! its encoder and decoder share the same constants, so a symmetric error keeps
//! them consistent while the image disagrees with this verifier.
//!
//! This is the one test where a byte written by Go is interpreted by the actual
//! Zig type, with no Zig-side parsing in between: mmap, cast, read.
//!
//! The fixture is `testdata/proof_image.bin`, written by prover-ray's
//! `TestVerifierRayImageIsUpToDate`, which fails if the file goes stale. Values are raw u32
//! limbs, not field arithmetic results, so both sides compare identical numbers.

const std = @import("std");
const verifier_ray = @import("verifier_ray");

const verifier = verifier_ray.verifier;

/// The address the image is relocated for. Pointers in the image are absolute,
/// so it can only be read here.
///
/// prover-ray's abi_agreement_test.go must use the same constant. It is not the
/// production GuestBase (0x08800000) because macOS refuses MAP_FIXED in the low
/// address space; 0x400000000 maps on both hosts.
const fixture_base: usize = 0x400000000;

const image_path = "testdata/proof_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const map_fixed: c_int = 0x10;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn close(fd: c_int) c_int;

/// Maps the fixture image at `fixture_base` and casts it, exactly as
/// `loadR5Input` casts the guest's input region: no parsing, no fix-up.
fn mapFixtureImage() !*const verifier.VerifyInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    // Length is rounded up to a page; the image is smaller and the tail is unread.
    const p = mmap(@ptrFromInt(fixture_base), 1 << 16, prot_read, map_private | map_fixed, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return @ptrCast(@alignCast(p));
}

fn expectDigest(actual: anytype, comptime first: u32) !void {
    inline for (0..8) |i| {
        try std.testing.expectEqual(first + @as(u32, i), actual[i].value);
    }
}

fn expectExt(actual: anytype, comptime first: u32) !void {
    try std.testing.expectEqual(first + 0, actual.B0.a0.value);
    try std.testing.expectEqual(first + 1, actual.B0.a1.value);
    try std.testing.expectEqual(first + 2, actual.B1.a0.value);
    try std.testing.expectEqual(first + 3, actual.B1.a1.value);
    try std.testing.expectEqual(first + 4, actual.B2.a0.value);
    try std.testing.expectEqual(first + 5, actual.B2.a1.value);
}

test "a Go-encoded image reads as a verifier.VerifyInput" {
    const input = mapFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        // Some sandboxes refuse MAP_FIXED. Skipping is right: the format is not
        // broken, the environment cannot host the test.
        error.MapFixedUnavailable => return error.SkipZigTest,
    };

    const proof = &input.proof;

    // ---- the flat public-input statement, separate from the round cells ------
    try std.testing.expectEqual(@as(usize, 2), input.public_inputs.len);
    switch (input.public_inputs[0]) {
        .base => |b| try std.testing.expectEqual(@as(u32, 201), b.value),
        .ext => return error.WrongCellVariant,
    }
    switch (input.public_inputs[1]) {
        .base => return error.WrongCellVariant,
        .ext => |e| try expectExt(e, 211),
    }

    // ---- rounds --------------------------------------------------------------
    try std.testing.expectEqual(@as(usize, 3), proof.rounds.len);

    const r0 = proof.rounds[0];
    // A committed round carries only its Merkle root, as an optional.
    const root = r0.commitment orelse return error.MissingCommitment;
    try expectDigest(root, 10);
    try std.testing.expectEqual(@as(usize, 2), r0.cells.len);
    // The discriminant must survive: same 24-byte payload, different variant.
    switch (r0.cells[0]) {
        .base => |b| try std.testing.expectEqual(@as(u32, 100), b.value),
        .ext => return error.WrongCellVariant,
    }
    switch (r0.cells[1]) {
        .base => return error.WrongCellVariant,
        .ext => |e| try expectExt(e, 200),
    }

    // A round that commits nothing: the presence flag must read as absent.
    const r1 = proof.rounds[1];
    try std.testing.expect(r1.commitment == null);
    try std.testing.expectEqual(@as(usize, 1), r1.cells.len);
    switch (r1.cells[0]) {
        .base => |b| try std.testing.expectEqual(@as(u32, 31), b.value),
        .ext => return error.WrongCellVariant,
    }

    // An empty round: zero-length slices must be readable, and their pointers
    // non-null, which is why the encoder never writes a null there.
    const r2 = proof.rounds[2];
    try std.testing.expect(r2.commitment == null);
    try std.testing.expectEqual(@as(usize, 0), r2.cells.len);
    try std.testing.expect(@intFromPtr(r2.cells.ptr) != 0);

    // ---- module sizes --------------------------------------------------------
    try std.testing.expectEqual(@as(usize, 2), proof.module_sizes.len);
    try std.testing.expectEqual(@as(usize, 8), proof.module_sizes[0]);
    try std.testing.expectEqual(@as(usize, 16), proof.module_sizes[1]);

    // ---- PCS input-tree openings --------------------------------------------
    // No entry_claims to read here any more: PcsOpening no longer carries them
    // as a separate serialized field, so the fixture no longer has a standalone
    // 50/60-valued jagged array to assert on. The verifier reconstructs the
    // claimed evaluations itself at verify time by reading ordinary round cells
    // (see verifier.zig's `verify`) — already covered by this test's own
    // round-cell assertions above.
    const queries = proof.pcs_opening.proof.input_queries;
    const input_caps = proof.pcs_opening.proof.input_caps;
    try std.testing.expectEqual(@as(usize, 1), input_caps.len);
    try std.testing.expectEqual(@as(usize, 1), input_caps[0].nodes.len);
    try expectDigest(input_caps[0].nodes[0], 60);
    try std.testing.expectEqual(@as(usize, 1), input_caps[0].tables.len);
    try std.testing.expectEqual(@as(u8, 5), input_caps[0].tables[0].size_log2);
    try std.testing.expectEqual(@as(usize, 1), input_caps[0].tables[0].rows.len);
    try std.testing.expectEqual(@as(usize, 2), input_caps[0].tables[0].rows[0].base.len);
    try std.testing.expectEqual(@as(u32, 68), input_caps[0].tables[0].rows[0].base[0].value);
    try expectExt(input_caps[0].tables[0].rows[0].ext[0], 70);

    try std.testing.expectEqual(@as(usize, 1), queries.len);
    try std.testing.expectEqual(@as(usize, 1), queries[0].len);

    const opening = queries[0][0];
    try std.testing.expectEqual(@as(usize, 1), opening.siblings.len);
    try expectDigest(opening.siblings[0], 70);

    try std.testing.expectEqual(@as(usize, 2), opening.leaves.len);
    try std.testing.expect(opening.leaves[0] == null); // the null level
    const pair = opening.leaves[1] orelse return error.MissingLeaf;
    try std.testing.expectEqual(@as(usize, 2), pair[0].base.len);
    try std.testing.expectEqual(@as(u32, 80), pair[0].base[0].value);
    try std.testing.expectEqual(@as(u32, 81), pair[0].base[1].value);
    try std.testing.expectEqual(@as(usize, 1), pair[0].ext.len);
    try expectExt(pair[0].ext[0], 90);
    try std.testing.expectEqual(@as(u32, 82), pair[1].base[0].value);
    try std.testing.expectEqual(@as(usize, 0), pair[1].ext.len);

    // ---- FRI proof -----------------------------------------------------------
    const fri_proof = proof.pcs_opening.proof.fri_proof;
    try std.testing.expectEqual(@as(usize, 1), fri_proof.round_roots.len);
    try expectDigest(fri_proof.round_roots[0], 110);
    try std.testing.expectEqual(@as(usize, 1), fri_proof.round_caps.len);
    try std.testing.expectEqual(@as(usize, 1), fri_proof.round_caps[0].nodes.len);
    try expectDigest(fri_proof.round_caps[0].nodes[0], 150);
    try std.testing.expectEqual(@as(usize, 2), fri_proof.round_caps[0].aux.len);
    try expectDigest(fri_proof.round_caps[0].aux[0] orelse return error.MissingAux, 160);
    try std.testing.expect(fri_proof.round_caps[0].aux[1] == null);
    try std.testing.expectEqual(@as(usize, 1), fri_proof.final_poly.len);
    try expectExt(fri_proof.final_poly[0], 120);

    try std.testing.expectEqual(@as(usize, 1), fri_proof.running_queries.len);
    try std.testing.expectEqual(@as(usize, 1), fri_proof.running_queries[0].len);
    const branch = fri_proof.running_queries[0][0];
    // Branch is the type whose fields are NOT in declaration order, so reading
    // both correctly is the sharpest check that the encoder targeted the layout
    // rather than the source.
    try std.testing.expectEqual(@as(usize, 1), branch.siblings.len);
    try expectDigest(branch.siblings[0], 130);
    try expectDigest(branch.leaf, 140);
}
