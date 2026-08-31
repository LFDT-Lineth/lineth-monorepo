//! Reads the generated aggregator pair image as a real
//! `verifier.AggregatorInput` and runs the two-proof aggregation end-to-end.
//!
//! The fixture is `testdata/riscv_proof_pair_image_test.bin`, written by
//! codegen/generate-riscv-system: the SAME honest riscv proof that backs
//! `testdata/riscv_proof_image.bin`, encoded twice into one pair image (the
//! prover is deterministic, so a second proving run would yield this identical
//! proof anyway). This is the aggregator's cross-language end-to-end check: Go
//! writes the pair layout (16-byte pointer header + two relocated sub-images),
//! Zig mmaps and casts it, and `verifier.verifyPair` accepts both proofs plus
//! their public-input consistency against the real compiled system.
//!
//! The riscv system currently registers no public inputs, so the consistency
//! check here passes on two empty statements; the adversarial consistency
//! coverage (differing statements between two individually valid proofs) is
//! pinned by verify_pair_test.zig on the synthetic public-input fixtures.
//!
//! The fixture is deliberately NOT based at GuestBase:
//! riscv_proof_image_test.zig's fixture occupies GuestBase in this same test
//! process, and a pair image based there would always overlap it. This base
//! mirrors generate-riscv-system's `pairTestBase`.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const riscv_system = @import("riscv_system");

const verifier = verifier_ray.verifier;

const fixture_base: usize = 0x50000000;
const image_path = "testdata/riscv_proof_pair_image_test.bin";

const o_rdonly: c_int = 0;
const prot_read: c_int = 1;
const map_private: c_int = 2;
// MAP_FIXED_NOREPLACE, for the same reason as riscv_proof_image_test.zig:
// plain MAP_FIXED would silently clobber whatever test order/ASLR already
// placed at fixture_base; _NOREPLACE turns that into a skip instead.
const map_fixed_noreplace: c_int = 0x10 | 0x100000;
const seek_end: c_int = 2;
const map_failed = ~@as(usize, 0);

extern fn open(path: [*:0]const u8, flags: c_int) c_int;
extern fn mmap(address: ?*anyopaque, length: usize, prot: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn close(fd: c_int) c_int;
extern fn lseek(fd: c_int, offset: i64, whence: c_int) i64;

fn mapFixtureImage() !*const verifier.AggregatorInput {
    const fd = open(image_path, o_rdonly);
    if (fd < 0) return error.ImageMissing;
    defer _ = close(fd);

    const image_len = lseek(fd, 0, seek_end);
    if (image_len <= 0) return error.ImageMissing;

    const p = mmap(@ptrFromInt(fixture_base), @intCast(image_len), prot_read, map_private | map_fixed_noreplace, fd, 0);
    if (@intFromPtr(p) == map_failed) return error.MapFixedUnavailable;

    return @ptrCast(@alignCast(p));
}

test "a Go-encoded aggregator pair image verifies both proofs and their consistency" {
    const input = mapFixtureImage() catch |err| switch (err) {
        // The fixture is generated (not committed — it is above Git hosting
        // size limits); `make generate-testdata` produces it before test runs.
        error.ImageMissing => return error.SkipZigTest,
        // The environment refuses the fixed mapping, or fixture_base was
        // already occupied in this process — an environment/test-ordering
        // limitation, not a pair-image format failure.
        error.MapFixedUnavailable => return error.SkipZigTest,
    };

    // The header must point at two distinct in-image roots, laid out
    // header-then-A-then-B.
    try std.testing.expect(@intFromPtr(input.a) == fixture_base + 16);
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
