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
const fixture_map = @import("fixture_map.zig");

const verifier = verifier_ray.verifier;

const fixture_base: usize = 0x08800000;
const image_path = "testdata/riscv_proof_image.bin";

test "a Go-encoded honest proof image verifies against the real riscv system" {
    const input = fixture_map.mapFixtureImage(verifier.VerifyInput, image_path, fixture_base) catch |err| switch (err) {
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
