//! Reads the committed honest-proof image as a real `verifier.VerifyInput`.
//!
//! The fixture is `testdata/riscv_proof_image.bin`, generated
//! from the same real `arithmetization/src/main/riscv/main.zkc` proof path
//! (proving `zkc_r5.AllInOneGuestELF`, which exercises the full RV64I +
//! M-extension + custom-precompile surface in a single witness) that emits
//! `testdata/generated/riscv_system.zig`. This is the
//! cross-language end-to-end check: Go writes the native layout bytes, Zig
//! mmaps and casts them directly, then the real verifier accepts the proof
//! against the real compiled system.
//!
//! Deliberately a distinct file from `testdata/proof_image.bin`, which is
//! prover-ray's `TestVerifierRayImageIsUpToDate` fixture: a small synthetic
//! `VerifyInput` at a different base address, for a cross-language ABI-
//! agreement check unrelated to this real end-to-end proof. The two must not
//! share a path — each writer would silently clobber the other's fixture with
//! content the other's reader can't decode.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;

// The base address at which riscv_proof_image.bin was encoded.
// Must match GuestBase in prover-ray/wiop/proofserialization/layout.go.
const encoded_base: usize = 0x08800000;

const image_path = "testdata/riscv_proof_image.bin";

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

fn readU64(buf: [*]const u8, offset: usize) u64 {
    return std.mem.readInt(u64, buf[offset..][0..8], .little);
}

fn loadFixtureImage() !*const verifier.VerifyInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;
    const img_len: usize = @intCast(image_len);

    // Map the file privately at any address the OS picks. Pointer rewrites are
    // copy-on-write and cannot modify the stored proof image.
    const p = mmap(null, img_len, prot_read | prot_write, map_private, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MmapFailed;

    const buf: [*]u8 = @ptrCast(p);
    const rounds_header = @offsetOf(verifier.VerifyInput, "proof") +
        @offsetOf(verifier.Proof, "rounds");
    const stored_rounds_pointer = readU64(buf, rounds_header);

    verifier_ray.image_relocation.rebase(buf, img_len, encoded_base, @intFromPtr(p));

    const rounds_offset = stored_rounds_pointer - encoded_base;
    try std.testing.expectEqual(
        @as(u64, @intCast(@intFromPtr(p))) + rounds_offset,
        readU64(buf, rounds_header),
    );

    // A fresh mapping must still see the original pointer bytes, proving the
    // relocation dirtied only the private mapping rather than the file.
    const stored = mmap(null, img_len, prot_read, map_private, fd, 0);
    if (@intFromPtr(stored) == map_failed) return error.MmapFailed;
    defer _ = munmap(stored, img_len);
    try std.testing.expectEqual(stored_rounds_pointer, readU64(@ptrCast(stored), rounds_header));

    return @ptrCast(@alignCast(p));
}

test "a Go-encoded honest proof image verifies against the real riscv system" {
    const input = loadFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };

    try std.testing.expect(input.proof.rounds.len > 0);
    try std.testing.expect(input.proof.pcs_opening.proof.input_queries.len > 0);

    try verifier.verify(
        riscv_system.system_0_spec,
        riscv_system.system_0_systems,
        input.proof,
        input.public_inputs,
    );
}
