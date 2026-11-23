const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // All guests target the same freestanding rv64im profile.
    const target = common.standardGuestTarget(b);
    // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;
    const host_tools = b.option(bool, "host-tools", "Build the devmode prover and the native tests") orelse false;

    _ = b.addModule("lineth_recursion", recursionModule(b, target, optimize));
    if (!host_tools) return;

    const prover_paths = b.addOptions();
    prover_paths.addOptionPath("devmode_prove", addDevmodeProver(b));
    _ = b.addModule("devmode_prover", proverModule(b, target, optimize, prover_paths));

    const native_target = b.resolveTargetQuery(.{});
    const test_step = b.step("test", "Run native Zig tests for the recursive verifier");
    _ = common.addTests(b, test_step, .{
        .target = native_target,
        .imports = &.{
            .{ .name = "lineth_recursion", .module = b.createModule(recursionModule(b, native_target, .Debug)) },
            .{ .name = "devmode_prover", .module = b.createModule(proverModule(b, native_target, .Debug, prover_paths)) },
        },
    }, .{});
}

/// The generated devmode system of verifier-ray, relative to this package.
const devmode_system_source = "../../verifier-ray/testdata/generated/devmode_system.zig";

/// The verifier of the zkVM guest is built for the R5 target with its pure-Zig Poseidon2; every
/// other target builds the verifier for the host. The riscv system option is the one the verifier
/// package requires; the devmode system is the one this package verifies against.
fn verifierRay(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) *std.Build.Dependency {
    const devmode_system_path = b.pathFromRoot(devmode_system_source);
    if (target.result.cpu.arch == .riscv64) {
        return b.dependency("verifier_ray", .{
            .target = target,
            .optimize = optimize,
            .r5 = true,
            .@"disable-accelerators" = true,
            .@"riscv-system" = devmode_system_path,
        });
    }
    return b.dependency("verifier_ray", .{ .target = target, .optimize = optimize, .@"riscv-system" = devmode_system_path });
}

fn recursionModule(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) std.Build.Module.CreateOptions {
    const verifier_ray = verifierRay(b, target, optimize);
    return .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = b.allocator.dupe(std.Build.Module.Import, &.{
            .{ .name = "verifier_ray", .module = verifier_ray.module("verifier_ray") },
            .{ .name = "devmode_system", .module = verifier_ray.module("devmode_system") },
        }) catch @panic("OOM"),
    };
}

fn proverModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    prover_paths: *std.Build.Step.Options,
) std.Build.Module.CreateOptions {
    return .{
        .root_source_file = b.path("test/support/devmode_prover.zig"),
        .target = target,
        .optimize = optimize,
        .imports = b.allocator.dupe(std.Build.Module.Import, &.{
            .{ .name = "prover_paths", .module = prover_paths.createModule() },
            .{ .name = "verifier_ray", .module = verifierRay(b, target, optimize).module("verifier_ray") },
        }) catch @panic("OOM"),
    };
}

/// The prover-ray command (`prover`, run as `prover devmode-prove`) built for the host: the program
/// the tests and tools hand the devmode proofs they need to. `go` rebuilds it on every run and caches
/// what did not change.
fn addDevmodeProver(b: *std.Build) std.Build.LazyPath {
    const go = b.addSystemCommand(&.{ "go", "build", "-o" });
    const binary = go.addOutputFileArg("prover");
    go.addArg("./cmd/prover");
    go.setCwd(b.path("../../prover-ray"));
    go.has_side_effects = true;
    return binary;
}
