//! Reads the committed honest-proof image as a real `verifier.VerifyInput`.
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
//! The fixture is `testdata/proof_image.bin`, written by
//! `codegen/abicheck`'s `TestVerifierRayImageIsUpToDate`, which fails if the file
//! goes stale. Values are raw u32
//! limbs, not field arithmetic results, so both sides compare identical numbers.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;

/// The address the image is relocated for. Pointers in the image are absolute,
/// so it can only be read here.
///
/// codegen/abicheck's abi_agreement_test.go must use the same constant. It is not the
/// production GuestBase (0x08800000) because macOS refuses MAP_FIXED in the low
/// address space; 0x400000000 maps on both hosts.
const fixture_base: usize = 0x400000000;


const image_path = "testdata/proof_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const map_fixed: c_int = 0x10;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

fn mapFixtureImage() !*const verifier.VerifyInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;

    const p = mmap(@ptrFromInt(fixture_base), @intCast(image_len), prot_read, map_private | map_fixed, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return @ptrCast(@alignCast(p));
}

test "a Go-encoded honest proof image verifies against the real riscv system" {
    const input = mapFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        // Some environments refuse low-address MAP_FIXED mappings. That is an
        // environment limitation, not a proof-image format failure.
        error.MapFixedUnavailable => return error.SkipZigTest,
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
