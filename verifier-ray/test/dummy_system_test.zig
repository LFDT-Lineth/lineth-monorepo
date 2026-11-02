//! End-to-end verification tests for the compiled dummy system and its
//! Go-generated single proof image (`testdata/dummy_proof_image.bin`).
//!
//! Unlike a custom verifier stub, the dummy system is compiled through
//! `compilers.CompileFull` in `prover-ray/backend/devmode` and emitted by
//! `codegen/generate-dummy-system` as a standard `verifier-ray` system
//! (`testdata/generated/dummy_system.zig`). These tests verify that
//! `verifier.verify` accepts honest dummy proofs and rejects mutated
//! statements/cells using the exact same Zig verifier code as the production
//! RISC-V system.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const dummy_system = @import("dummy_system");

const verifier = verifier_ray.verifier;
const relocation = verifier_ray.image_relocation;
const field = verifier_ray.field.koalabear;

const encoded_base: usize = 0x08800000;
const single_image_path = "testdata/dummy_proof_image.bin";

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

fn mapFile(path: [*:0]const u8) ![]u8 {
    const fd = open(path, o_rdonly);
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

test "dummy_system: generated spec and systems match 40-cell public-input layout" {
    const spec = dummy_system.system_0_spec;
    const systems = dummy_system.system_0_systems;

    try std.testing.expectEqual(@as(usize, 40), systems.public_input.refs.len);
    try std.testing.expectEqual(@as(usize, 1), systems.vanishing.modules.len);
    try std.testing.expectEqual(@as(usize, 0), spec.dynamic_module_count);
}

test "dummy_system: single proof image verifies and rejects mutated public input" {
    const img = mapFile(single_image_path) catch |err| switch (err) {
        error.ImageMissing => return error.SkipZigTest,
        else => return err,
    };
    defer _ = munmap(@ptrCast(img.ptr), img.len);

    const base = @intFromPtr(img.ptr);
    relocation.rebase(img.ptr, img.len, encoded_base, base);
    try relocation.validateImage(img.ptr, img.len, base);

    const input: *verifier.VerifyInput = @ptrCast(@alignCast(img.ptr));
    try std.testing.expectEqual(@as(usize, 40), input.public_inputs.len);

    try verifier.verify(
        dummy_system.system_0_spec,
        dummy_system.system_0_systems,
        input.proof,
        input.public_inputs,
    );

    // Mutating a public input must fail the vanishing quotient check.
    const pub_mut: [*]verifier_ray.protocol.Scalar = @constCast(input.public_inputs.ptr);
    const saved = pub_mut[0];
    pub_mut[0] = .{ .base = saved.base.add(field.Element.one()) };
    defer pub_mut[0] = saved;

    try std.testing.expectError(
        error.QuotientIdentityMismatch,
        verifier.verify(
            dummy_system.system_0_spec,
            dummy_system.system_0_systems,
            input.proof,
            input.public_inputs,
        ),
    );
}
