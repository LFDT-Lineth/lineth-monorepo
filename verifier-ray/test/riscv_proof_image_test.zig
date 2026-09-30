//! Reads the committed honest-proof image as a real `verifier.VerifyInput`.
//!
//! The fixture is `testdata/riscv_proof_image.bin`, generated from the same
//! real `arithmetization/src/main/riscv/main.zkc` proof path (proving
//! `zkc_r5.AllInOneGuestELF`) that emits `testdata/generated/riscv_system.zig`.
//! Go writes the pointer-free guest image (`EncodeGuest`); this test decodes
//! it and the real verifier accepts the proof against the real compiled system.
//!
//! Deliberately a distinct file from `testdata/proof_image.bin`, which is
//! prover-ray's `TestVerifierRayImageIsUpToDate` fixture: a small synthetic
//! `VerifyInput` in the cast layout, for a cross-language ABI-agreement check
//! unrelated to this real end-to-end proof. The two must not share a path —
//! each writer would silently clobber the other's fixture with content the
//! other's reader can't decode.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;
const proof_guest = verifier_ray.proof_guest;

const image_path = "testdata/riscv_proof_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn munmap(address: *anyopaque, length: usize) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

fn mix(x: *u64, v: u32) void {
    x.* = x.* *% 0x9e3779b185ebca87 +% v;
}

fn mixU8(x: *u64, v: u8) void {
    mix(x, v);
}

fn mixDig(x: *u64, d: verifier_ray.crypto.poseidon2.Digest) void {
    for (d) |limb| mix(x, limb.value);
}

fn mixExt(x: *u64, e: verifier_ray.field.koalabear_ext.Ext) void {
    mix(x, e.B0.a0.value);
    mix(x, e.B0.a1.value);
    mix(x, e.B1.a0.value);
    mix(x, e.B1.a1.value);
    mix(x, e.B2.a0.value);
    mix(x, e.B2.a1.value);
}

fn mixScalar(x: *u64, s: verifier_ray.field.value.Scalar) void {
    switch (s) {
        .base => |b| {
            mixU8(x, 0);
            mix(x, b.value);
            mix(x, 0);
            mix(x, 0);
            mix(x, 0);
            mix(x, 0);
            mix(x, 0);
        },
        .ext => |e| {
            mixU8(x, 1);
            mixExt(x, e);
        },
    }
}

fn mixRow(x: *u64, row: verifier_ray.crypto.merkle.RowOpening) void {
    mix(x, @intCast(row.base.len));
    for (row.base) |e| mix(x, e.value);
    mix(x, @intCast(row.ext.len));
    for (row.ext) |e| mixExt(x, e);
}

/// Same mix as prover-ray's TestEncodeGuest_RiscvFixture. A base scalar's high
/// limbs are the cast image's padding, which this decoder does not keep.
fn imageFingerprint(input: verifier.VerifyInput) u64 {
    var x: u64 = 0x6a09e667f3bcc909;
    const opening = input.proof.pcs_opening.proof;
    for (input.proof.rounds) |round| {
        mix(&x, @intCast(round.cells.len));
        for (round.cells) |cell| mixScalar(&x, cell);
        if (round.commitment) |c| {
            mixU8(&x, 1);
            mixDig(&x, c);
        } else mixU8(&x, 0);
    }
    mix(&x, @intCast(input.proof.module_sizes.len));
    for (input.proof.module_sizes) |n| {
        mix(&x, @truncate(n));
        mix(&x, @truncate(n >> 32));
    }
    for (input.public_inputs) |s| mixScalar(&x, s);
    for (opening.input_queries) |query| {
        mix(&x, @intCast(query.len));
        for (query) |tree| {
            mix(&x, @intCast(tree.siblings.len));
            for (tree.siblings) |d| mixDig(&x, d);
            mix(&x, @intCast(tree.leaves.len));
            for (tree.leaves) |leaf| {
                if (leaf) |pair| {
                    mixU8(&x, 1);
                    mixRow(&x, pair[0]);
                    mixRow(&x, pair[1]);
                } else mixU8(&x, 0);
            }
        }
    }
    for (opening.input_caps) |cap| {
        mix(&x, @intCast(cap.nodes.len));
        for (cap.nodes) |d| mixDig(&x, d);
        mix(&x, @intCast(cap.tables.len));
        for (cap.tables) |table| {
            mixU8(&x, table.size_log2);
            mix(&x, @intCast(table.rows.len));
            for (table.rows) |row| mixRow(&x, row);
        }
    }
    const fri_proof = opening.fri_proof;
    mix(&x, @intCast(fri_proof.round_roots.len));
    for (fri_proof.round_roots) |d| mixDig(&x, d);
    mix(&x, @intCast(fri_proof.round_caps.len));
    for (fri_proof.round_caps) |cap| {
        mix(&x, @intCast(cap.nodes.len));
        for (cap.nodes) |d| mixDig(&x, d);
        mix(&x, @intCast(cap.aux.len));
        for (cap.aux) |aux| {
            if (aux) |d| {
                mixU8(&x, 1);
                mixDig(&x, d);
            } else mixU8(&x, 0);
        }
    }
    mix(&x, @intCast(fri_proof.final_poly.len));
    for (fri_proof.final_poly) |e| mixExt(&x, e);
    mix(&x, @intCast(fri_proof.running_queries.len));
    for (fri_proof.running_queries) |query| {
        mix(&x, @intCast(query.len));
        for (query) |branch| {
            mix(&x, @intCast(branch.siblings.len));
            for (branch.siblings) |d| mixDig(&x, d);
            mixDig(&x, branch.leaf);
        }
    }
    return x;
}

fn loadFixtureImage(allocator: std.mem.Allocator) !verifier.VerifyInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;
    const img_len: usize = @intCast(image_len);

    const mapped = mmap(null, img_len, prot_read, map_private, fd, 0);
    if (@intFromPtr(mapped) == map_failed) return error.MmapFailed;
    defer _ = munmap(mapped, img_len);

    const file_bytes: [*]const u8 = @ptrCast(mapped);
    // The same fixed buffer the guest loaders use. Decoding into it is what
    // proves the smoke-test binary has room for this proof.
    const buf = try allocator.alloc(u8, proof_guest.decode_buffer_len);
    var fba = std.heap.FixedBufferAllocator.init(buf);
    return proof_guest.decode(fba.allocator(), file_bytes[0..img_len]);
}

test "a Go-encoded honest proof image verifies against the real riscv system" {
    var arena_impl = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena_impl.deinit();
    const input = loadFixtureImage(arena_impl.allocator()) catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };

    try std.testing.expectEqual(@as(usize, 5), input.proof.rounds.len);
    try std.testing.expectEqual(@as(usize, 97), input.proof.module_sizes.len);
    try std.testing.expectEqual(@as(usize, 337), input.public_inputs.len);
    try std.testing.expectEqual(@as(usize, 229), input.proof.pcs_opening.proof.input_queries.len);
    try std.testing.expectEqual(@as(usize, 15), input.proof.pcs_opening.proof.fri_proof.round_roots.len);
    try std.testing.expectEqual(@as(u64, 0x39688720aeec5408), imageFingerprint(input));

    try verifier.verify(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.proof,
        input.public_inputs,
    );
}
