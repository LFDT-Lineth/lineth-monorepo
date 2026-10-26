//! Reads the generated aggregator pair image as a real
//! `verifier.AggregatorInput` and runs the two-proof aggregation end-to-end.
//!
//! The fixture is `testdata/riscv_proof_pair_image.bin`, written by
//! codegen/generate-aggregator-image: the SAME honest riscv proof that backs
//! `testdata/riscv_proof_image.bin`, encoded twice into one pair image (the
//! prover is deterministic, so a second proving run would yield this identical
//! proof anyway). This is the aggregator's cross-language end-to-end check: Go
//! writes the pair layout (16-byte pointer header + two relocated sub-images),
//! Zig maps and rebases it, and `verifier.verifyPair` accepts both proofs plus
//! their public-input consistency against the real compiled system.
//!
//! The fixture carries the production RISC-V system's registered public input.
//! Differing-statement coverage is pinned by verify_pair_test.zig using two
//! independently valid synthetic proofs.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;
const relocation = verifier_ray.image_relocation;

// The base address at which the pair image was encoded.
// Must match GuestBase in prover-ray/wiop/proofserialization/layout.go.
const encoded_base: usize = 0x08800000;

const image_path = "testdata/riscv_proof_pair_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const prot_write: c_int = 2;
const map_private: c_int = 2;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn munmap(address: *anyopaque, length: usize) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

/// Maps the pair image privately at any address the OS picks, at its encoded
/// layout. Writes are copy-on-write and cannot modify the stored pair image.
fn mapPairImage() ![]u8 {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;
    const img_len: usize = @intCast(image_len);

    const p = mmap(null, img_len, prot_read | prot_write, map_private, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MmapFailed;
    const buf: [*]u8 = @ptrCast(p);
    return buf[0..img_len];
}

/// main.zig's native pair loader: relocate, validate at the mapped layout,
/// and form the typed pointers from the validated offsets.
fn loadPairImage() !struct { input: verifier.AggregatorInput, base: usize } {
    const img = try mapPairImage();
    const base = @intFromPtr(img.ptr);
    try relocation.rebasePair(img.ptr, img.len, encoded_base, base);
    const pair = try relocation.validatePair(img.ptr, img.len, base);
    return .{ .input = .{ .a = @ptrFromInt(base + pair.a), .b = @ptrFromInt(base + pair.b) }, .base = base };
}

test "a Go-encoded aggregator pair image verifies both proofs and their consistency" {
    const loaded = loadPairImage() catch |err| switch (err) {
        // The fixture is generated (not committed — it is above Git hosting
        // size limits); `make generate-testdata` produces it before test runs.
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };
    const input = loaded.input;

    // The header must point at two distinct in-image roots, laid out
    // header-then-A-then-B.
    try std.testing.expectEqual(loaded.base + relocation.pair_header_len, @intFromPtr(input.a));
    try std.testing.expect(@intFromPtr(input.b) > @intFromPtr(input.a));
    try std.testing.expect(input.a.proof.rounds.len > 0);
    try std.testing.expect(input.b.proof.rounds.len > 0);

    try verifier.verifyPair(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.a.*,
        input.b.*,
    );
}

test "pair validation confines each member to its own region" {
    // At the encoded layout (the bytes the R5 guest sees), point one member's
    // public_inputs slice at the other member's: each pointer is inside the
    // pair image but outside its own member, so validation must reject both.
    const img = mapPairImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };
    defer _ = munmap(@ptrCast(img.ptr), img.len);
    const pair = try relocation.validatePair(img.ptr, img.len, encoded_base);

    const header = @offsetOf(verifier.VerifyInput, "public_inputs");
    const a_header = img[pair.a + header ..][0..8];
    const b_header = img[pair.b + header ..][0..8];
    const a_pointer = std.mem.readInt(u64, a_header, .little);
    const b_pointer = std.mem.readInt(u64, b_header, .little);

    // A reaching forward into B.
    std.mem.writeInt(u64, a_header, b_pointer, .little);
    try std.testing.expectError(error.PointerPastImage, relocation.validatePair(img.ptr, img.len, encoded_base));
    std.mem.writeInt(u64, a_header, a_pointer, .little);

    // B reaching back into A. B's region starts at its own root, so this is
    // rejected as before-image; only the case above pins the split point.
    std.mem.writeInt(u64, b_header, a_pointer, .little);
    try std.testing.expectError(error.PointerBeforeImage, relocation.validatePair(img.ptr, img.len, encoded_base));
    std.mem.writeInt(u64, b_header, b_pointer, .little);

    _ = try relocation.validatePair(img.ptr, img.len, encoded_base);
}
