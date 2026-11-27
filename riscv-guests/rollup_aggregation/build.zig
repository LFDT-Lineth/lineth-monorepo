const std = @import("std");
const common = @import("build_common");

const Import = std.Build.Module.Import;
const Compile = std.Build.Step.Compile;

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const options = Options.parse(b);
    addGuestProgram(b, options);
    if (!options.host_tools) return;

    const crypto = common.HostCrypto.forTarget(b, options.host_target, options.crypto) orelse return;
    const host = Host.init(b, options.host_target);
    addHostTests(b, host, crypto);
    addFixtureStep(b, host, crypto);
}

/// Every `-D` option of this package, read once.
const Options = struct {
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    crypto: common.CryptoOptions,
    host_tools: bool,
    host_target: std.Build.ResolvedTarget,

    fn parse(b: *std.Build) Options {
        // All guests target the same freestanding rv64im profile.
        const target = common.standardGuestTarget(b);
        // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
        const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;
        const host_tools = b.option(bool, "host-tools", "Build native test and helper targets") orelse false;
        return .{
            .target = target,
            .optimize = optimize,
            .crypto = common.CryptoOptions.parse(b),
            .host_tools = host_tools,
            .host_target = b.resolveTargetQuery(.{}),
        };
    }
};

/// What the sources of the package import by name, for one target.
const Deps = struct {
    guest_common: *std.Build.Module,
    core_serde: *std.Build.Module,
    zkvm_accelerators: *std.Build.Module,
    lineth_recursion: *std.Build.Dependency,
    /// Imports of the package module (`src/root.zig`).
    package: []const Import,

    /// `host_tools` also builds the test-side modules `lineth_recursion` exposes to its consumers.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, host_tools: bool) Deps {
        const guest_common = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
        const lineth_recursion = b.dependency("lineth_recursion", .{ .target = target, .optimize = optimize, .@"host-tools" = host_tools });
        // The same instance `guest_common` imports, so one compilation holds the module once.
        const zkvm_accelerators = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("zkvm_accelerators");
        return .{
            .guest_common = guest_common.module("guest_common"),
            .core_serde = guest_common.module("core_serde"),
            .zkvm_accelerators = zkvm_accelerators,
            .lineth_recursion = lineth_recursion,
            .package = b.allocator.dupe(Import, &.{
                .{ .name = "guest_common", .module = guest_common.module("guest_common") },
                .{ .name = "core_serde", .module = guest_common.module("core_serde") },
                .{ .name = "zkvm_accelerators", .module = zkvm_accelerators },
                .{ .name = "lineth_recursion", .module = lineth_recursion.module("lineth_recursion") },
            }) catch @panic("OOM"),
        };
    }
};

/// The self-contained rv64im guest ELF (aggregation statement, recursion, crypto, input I/O).
fn addGuestProgram(b: *std.Build, options: Options) void {
    const accelerators = common.acceleratorsDependency(b, options.target, options.optimize, options.crypto);
    const deps = Deps.init(b, options.target, options.optimize, false);
    const package = common.createPackageModule(b, options.target, options.optimize, deps.package);
    common.addGuest(b, .{
        .name = "rollup_aggregation_guest",
        .target = options.target,
        .optimize = options.optimize,
        .imports = &.{
            .{ .name = "rollup_aggregation", .module = package },
            .{ .name = "lineth_accelerators", .module = accelerators.module("lineth_accelerators") },
            .{ .name = "zkvm_provide", .module = accelerators.module("zkvm_provide") },
        },
        .code_model = .medium,
    });
}

/// What the native tests and tools share: the modules they import.
const Host = struct {
    native: common.NativeOptions,

    fn init(b: *std.Build, target: std.Build.ResolvedTarget) Host {
        const deps = Deps.init(b, target, .Debug, true);
        const package = common.createPackageModule(b, target, .Debug, deps.package);
        const devmode_prover = deps.lineth_recursion.module("devmode_prover");
        const scenario = b.createModule(.{
            .root_source_file = b.path("test/support/scenario.zig"),
            .target = target,
            .optimize = .Debug,
            .imports = &.{
                .{ .name = "rollup_aggregation", .module = package },
                .{ .name = "core_serde", .module = deps.core_serde },
                .{ .name = "devmode_prover", .module = devmode_prover },
            },
        });
        return .{
            // Tests run on the native host: a freestanding riscv64 target has no OS to run a
            // `std.testing` binary against.
            .native = .{
                .target = target,
                .optimize = .Debug,
                .imports = b.allocator.dupe(Import, &.{
                    .{ .name = "rollup_aggregation", .module = package },
                    .{ .name = "core_serde", .module = deps.core_serde },
                    .{ .name = "guest_common", .module = deps.guest_common },
                    .{ .name = "scenario", .module = scenario },
                    .{ .name = "zkvm_accelerators", .module = deps.zkvm_accelerators },
                }) catch @panic("OOM"),
            },
        };
    }
};

/// The `test/*_test.zig` suites; the sample suite embeds the committed fixture.
fn addHostTests(b: *std.Build, host: Host, crypto: common.HostCrypto) void {
    const test_step = b.step("test", "Run native Zig tests for the rollup-aggregation guest");
    const tests = common.addTests(b, test_step, host.native, .{});
    crypto.linkAll(tests);
    const sample = tests.get("test/sample_fixture_test.zig").?.root_module;
    sample.addAnonymousImport("sample.request.ssz", .{ .root_source_file = b.path("test/testdata/sample.request.ssz") });
    sample.addAnonymousImport("sample.output.bin", .{ .root_source_file = b.path("test/testdata/sample.output.bin") });
}

/// Runs `gen_fixture` with the arguments after `--`.
fn addFixtureStep(b: *std.Build, host: Host, crypto: common.HostCrypto) void {
    const tools = common.addTools(b, host.native, .{});
    const gen_fixture: *Compile = tools.get("gen_fixture").?;
    crypto.link(gen_fixture);
    const run = b.addRunArtifact(gen_fixture);
    if (b.args) |args| run.addArgs(args);
    b.step("gen-fixture", "Write a sample RollupAggregationInput and its expected output to the paths after `--`").dependOn(&run.step);
}
