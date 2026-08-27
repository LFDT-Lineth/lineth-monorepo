//! Reads the committed honest-proof image as a real `verifier.VerifyInput`.
//!
//! The fixture is `testdata/riscv_proof_image.bin`, generated from the same real
//! `arithmetization/src/main/riscv/main.zkc` proof path that emits
//! `testdata/generated/riscv_system.zig`. This is the cross-language end-to-end
//! check: Go writes the native layout bytes, Zig mmaps and casts them directly,
//! then the real verifier accepts the proof against the real compiled system.
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

const fixture_base: usize = 0x08800000;
const image_path = "testdata/riscv_proof_image.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
// MAP_FIXED_NOREPLACE (not MAP_FIXED): this test shares a process with every
// other test in test/all.zig, and Zig randomizes test order per run, so
// whatever else has already been placed in the address space by the time
// this test runs varies run to run. Plain MAP_FIXED would silently overlap
// (and corrupt) anything already mapped at fixture_base, producing a
// non-reproducible verify() outcome keyed to test order/ASLR rather than to
// the (proof, public_inputs) actually under test. _NOREPLACE fails the
// syscall instead — surfaced below as MapFixedUnavailable, the same skip path
// already used for the "environment refuses low-address mappings" case.
const map_fixed_noreplace: c_int = 0x10 | 0x100000;
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

    const p = mmap(@ptrFromInt(fixture_base), @intCast(image_len), prot_read, map_private | map_fixed_noreplace, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return @ptrCast(@alignCast(p));
}

test "a Go-encoded honest proof image verifies against the real riscv system" {
    const input = mapFixtureImage() catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        // Either the environment refuses low-address fixed mappings outright,
        // or (MAP_FIXED_NOREPLACE) fixture_base was already occupied by
        // something else in this test binary's process — an environment/test-
        // ordering limitation, not a proof-image format failure.
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
