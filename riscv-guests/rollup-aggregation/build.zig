const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile every guest targets (shared helper).
    const target = common.standardGuestTarget(b);

    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed while still defaulting to ReleaseSmall — mirrors l2-execution's own build.zig.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    // Production proving config knob, same contract as l2-execution's flag of the same name. The
    // stub's keccak use is comptime-only, so today the flag only keeps the CI/production
    // invocation stable; the real guest implementation wires it to the arithmetization keccak
    // accelerator backend.
    _ = b.option(bool, "keccak-accel", "Use the arithmetization keccak wrapper instead of standard zig keccak (default: standard)") orelse false;

    const gp_name = "rollup_aggregation_guest";

    // ── Guest: statically-linked rv64im ELF ───────────────────────────────────────────────────
    // Two path dependencies: guest_common for the SSZ codec primitives, lineth_accelerators for
    // the zkVM io-interface (read_input/write_output).
    const guest_common_mod = b.dependency("guest_common", .{ .target = target, .optimize = optimize }).module("guest_common");
    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const rollup_aggregation_ssz_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_ssz.zig"),
        .target = target,
        .optimize = optimize,
    });
    rollup_aggregation_ssz_mod.addImport("guest_common", guest_common_mod);

    // `rollup_aggregation_guest.zig` relatively imports its sibling `rollup_aggregation.zig`/
    // `guest_errors.zig` (same module, no separate wiring needed); only the two path
    // dependencies above are named imports.
    const guest_module = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_guest.zig"),
        .target = target,
        .optimize = optimize,
    });
    guest_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_mod);
    guest_module.addImport("lineth_accelerators", lineth_accel_mod);
    common.clearFreestandingNativeLinkage(b, guest_module);
    common.installGuestElf(b, guest_module, gp_name);

    // ── Native tests ───────────────────────────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against. Only
    // `rollup_aggregation_ssz`/`rollup_aggregation` are needed natively —
    // `rollup_aggregation_guest.zig`'s `guestMain` (the only caller of
    // `lineth_accelerators`) is never referenced by a test, so Zig's lazy analysis never compiles
    // that path for the native target (mirrors l2-execution's own `linea_zkvm_io`/`zkvm_provide`
    // omission from its native `guest_mod`).
    const native_target = b.resolveTargetQuery(.{});
    const guest_common_native_mod = b.dependency("guest_common", .{ .target = native_target, .optimize = .Debug }).module("guest_common");

    const rollup_aggregation_ssz_native_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation_ssz.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    rollup_aggregation_ssz_native_mod.addImport("guest_common", guest_common_native_mod);

    const rollup_aggregation_native_mod = b.createModule(.{
        .root_source_file = b.path("src/rollup_aggregation.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    rollup_aggregation_native_mod.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);

    // Shared test-only sample data (`sampleInput`/`repeat32`/`repeat20`), used by both native test
    // binaries below and by `tools/gen_fixture.zig` — one definition of a valid sample input.
    const support_mod = b.createModule(.{
        .root_source_file = b.path("test/support.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    support_mod.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);

    const test_step = b.step("test", "Run native Zig unit tests for the rollup-aggregation guest");

    const rollup_aggregation_ssz_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/rollup_aggregation_ssz_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    rollup_aggregation_ssz_tests.root_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);
    rollup_aggregation_ssz_tests.root_module.addImport("support.zig", support_mod);
    test_step.dependOn(&b.addRunArtifact(rollup_aggregation_ssz_tests).step);

    const rollup_aggregation_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/rollup_aggregation_test.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    rollup_aggregation_tests.root_module.addImport("rollup_aggregation", rollup_aggregation_native_mod);
    rollup_aggregation_tests.root_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);
    rollup_aggregation_tests.root_module.addImport("support.zig", support_mod);
    test_step.dependOn(&b.addRunArtifact(rollup_aggregation_tests).step);

    // ── Fixture generation ─────────────────────────────────────────────────────────────────────
    // Prints a framed sample input as hex on stdout, from the same `sampleInput` the tests use —
    // `make exec`'s guest input is this package's own encoder output, not an externally-produced
    // or separately-checked-in fixture. `require-input` in this guest's Makefile invokes this step
    // to (re)generate the default INPUT on every `make exec`/`debug`, so nothing here is checked
    // into git and it can never silently drift from what the current wire format produces.
    const gen_fixture = b.addExecutable(.{
        .name = "gen_fixture",
        .root_module = b.createModule(.{
            .root_source_file = b.path("tools/gen_fixture.zig"),
            .target = native_target,
            .optimize = .Debug,
        }),
    });
    gen_fixture.root_module.addImport("rollup_aggregation_ssz", rollup_aggregation_ssz_native_mod);
    gen_fixture.root_module.addImport("support.zig", support_mod);
    const gen_fixture_step = b.step("gen-fixture", "Print a sample RollupAggregationProofPrivateInput as framed raw bytes on stdout");
    gen_fixture_step.dependOn(&b.addRunArtifact(gen_fixture).step);
}
