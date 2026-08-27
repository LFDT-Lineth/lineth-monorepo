//! Verifies every honest guest ELF's real proof against the real riscv system,
//! not just the one guest (`ExitZeroGuestELF`) committed as
//! `testdata/riscv_proof_image.bin`.
//!
//! Unlike that committed fixture, these images are NOT checked in:
//! `testdata/README.md` asks fixtures there to stay small and deterministic,
//! and 10 honest proofs at ~52MB apiece would be ~525MB of binary fixtures for
//! no benefit over reusing the one guest already committed. Instead,
//! `codegen/generate-riscv-guest-proofs` (run via `make
//! generate-riscv-guest-proofs`, or directly) writes one
//! `riscv_proof_image_<guest>.bin` per guest plus a `manifest.txt` naming them
//! into a gitignored scratch directory; this test reads that manifest at
//! runtime (there is no comptime guest list here to keep in sync — the
//! manifest is the single source of truth) and calls `verifier.verify()` on
//! every image it names. If the scratch directory hasn't been generated yet,
//! the test skips rather than failing, exactly like `proof_image_test.zig`
//! skips on a missing fixture.
//!
//! All images verify against the same `testdata/generated/riscv_system.zig`:
//! `main.zkc`'s circuit doesn't depend on the guest, only the proof does.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;

const scratch_dir = "testdata/scratch/riscv-guest-proofs";
const manifest_path = scratch_dir ++ "/manifest.txt";

// Not the same fixed address proof_image_test.zig uses: that test's own
// ~52MB MAP_FIXED_NOREPLACE mapping (0x08800000..~0x0bc70000, and it never
// unmaps) may still be live in this process (test order is randomized), so a
// nearby address here would spuriously report MapFixedUnavailable for an
// unrelated reason. 0x20000000 leaves that whole region, and every image
// mapped here (also ~52MB, and this test's own mapGuestImage does unmap
// between guests), comfortably clear.
const fixture_base: usize = 0x20000000;

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
const map_fixed_noreplace: c_int = 0x10 | 0x100000;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn read(fd: c_int, buf: [*]u8, count: usize) isize;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn munmap(address: *anyopaque, length: usize) c_int;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

// Reads the whole manifest into a fixed buffer. Comfortably larger than the
// current 10-guest manifest (~120 bytes); grows in step with
// HonestRiscvGuests if that list ever grows substantially.
const manifest_buf_cap = 4096;

fn readManifest(buf: *[manifest_buf_cap]u8) ![]const u8 {
    const fd = open(manifest_path, o_rdonly);
    if (fd < 0) return error.ManifestMissing;
    defer _ = close(fd);

    const n = read(fd, buf, buf.len);
    if (n < 0) return error.ManifestReadFailed;
    return buf[0..@intCast(n)];
}

fn mapGuestImage(path_z: [*:0]const u8) !struct { input: *const verifier.VerifyInput, len: usize } {
    const fd = open(path_z, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;

    const p = mmap(@ptrFromInt(fixture_base), @intCast(image_len), prot_read, map_private | map_fixed_noreplace, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return .{ .input = @ptrCast(@alignCast(p)), .len = @intCast(image_len) };
}

test "every honest guest's real proof verifies against the real riscv system" {
    var manifest_buf: [manifest_buf_cap]u8 = undefined;
    const manifest = readManifest(&manifest_buf) catch |err| switch (err) {
        error.ManifestMissing => return error.SkipZigTest,
        error.ManifestReadFailed => return error.SkipZigTest,
    };

    var checked: usize = 0;
    var lines = std.mem.tokenizeScalar(u8, manifest, '\n');
    while (lines.next()) |guest_name| {
        var path_buf: [256]u8 = undefined;
        const path_z = try std.fmt.bufPrintZ(&path_buf, scratch_dir ++ "/riscv_proof_image_{s}.bin", .{guest_name});

        const mapped = mapGuestImage(path_z) catch |err| switch (err) {
            error.ImageMissing => {
                std.debug.print("skipping {s}: image missing (stale manifest?)\n", .{guest_name});
                continue;
            },
            error.MapFixedUnavailable => return error.SkipZigTest,
        };
        defer _ = munmap(@ptrCast(@constCast(mapped.input)), mapped.len);
        const input = mapped.input;

        try std.testing.expect(input.proof.rounds.len > 0);

        verifier.verify(
            riscv_system.system_0_spec,
            riscv_system.system_0_systems,
            input.proof,
            input.public_inputs,
        ) catch |err| {
            std.debug.print("verify failed for guest {s}: {}\n", .{ guest_name, err });
            return err;
        };
        checked += 1;
    }

    try std.testing.expect(checked > 0);
}
