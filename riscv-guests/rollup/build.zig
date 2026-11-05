const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im profile every guest targets (shared helper).
    const target = common.standardGuestTarget(b);

    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed while still defaulting to ReleaseSmall — mirrors l2-execution's own build.zig.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    // Production proving config knob, same contract as l2-execution's flag of the same name. The
    // stub's keccak use is comptime-only, so today the flag only keeps the CI/production
    // invocation stable; the real guest implementation wires it to the arithmetization keccak
    // accelerator backend.
    _ = b.option(bool, "keccak-accel", "Use the arithmetization keccak wrapper instead of standard zig keccak (default: unaccelerated)") orelse false;

    // ── Guest: statically-linked rv64im ELF ───────────────────────────────────────────────────
    // Two path dependencies: guest_common for the SSZ codec primitives, lineth_accelerators for
    // the zkVM io-interface (read_input/write_output).
    const guest_common_mod = b.dependency("guest_common", .{ .target = target, .optimize = optimize }).module("guest_common");
    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const package_mod = common.createPackageModule(b, target, optimize, &.{
        .{ .name = "guest_common", .module = guest_common_mod },
    });
    common.addGuest(b, .{
        .name = "rollup_guest",
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "rollup", .module = package_mod },
            .{ .name = "lineth_accelerators", .module = lineth_accel_mod },
        },
    });

    // ── Native tests ───────────────────────────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against. The entry point
    // (`src/main.zig`) is the only caller of `lineth_accelerators`' io, so the native package
    // module needs `guest_common` alone.
    const native_target = b.resolveTargetQuery(.{});
    const guest_common_native_mod = b.dependency("guest_common", .{ .target = native_target, .optimize = .Debug }).module("guest_common");
    const package_native_mod = common.createPackageModule(b, native_target, .Debug, &.{
        .{ .name = "guest_common", .module = guest_common_native_mod },
    });

    // Shared test-only sample data (`sampleInput`/`repeat32`/`repeat20`), used by the native tests
    // and the fixture generator — one definition of a valid sample input.
    const support_mod = b.createModule(.{
        .root_source_file = b.path("test/support.zig"),
        .target = native_target,
        .optimize = .Debug,
        .imports = &.{.{ .name = "rollup", .module = package_native_mod }},
    });
    const native: common.NativeOptions = .{
        .target = native_target,
        .imports = &.{
            .{ .name = "rollup", .module = package_native_mod },
            .{ .name = "support", .module = support_mod },
        },
    };

    const test_step = b.step("test", "Run native Zig unit tests for the rollup guest");
    _ = common.addTests(b, test_step, native, .{});
    const tools = common.addTools(b, native, .{});

    // ── Fixture generation ─────────────────────────────────────────────────────────────────────
    // Prints a framed sample input as raw bytes on stdout, from the same `sampleInput` the tests use —
    // `make exec`'s guest input is this package's own encoder output, not an externally-produced
    // or separately-checked-in fixture. `require-input` in this guest's Makefile invokes this step
    // to (re)generate the default INPUT on every `make exec`/`debug`, so nothing here is checked
    // into git and it can never silently drift from what the current wire format produces.
    const gen_fixture_step = b.step("gen-fixture", "Print a sample RollupProofPrivateInput as framed raw bytes on stdout");
    gen_fixture_step.dependOn(&b.addRunArtifact(tools.get("gen_fixture").?).step);
}
