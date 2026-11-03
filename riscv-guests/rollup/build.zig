const std = @import("std");
const common = @import("build_common");

const Import = std.Build.Module.Import;
const Compile = std.Build.Step.Compile;
const Tools = std.StringHashMap(*Compile);

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const options = Options.parse(b);
    addGuestProgram(b, options);
    if (!options.host_tools) return;

    const crypto = common.HostCrypto.forTarget(b, options.host_target, options.crypto) orelse return;
    const unit = Host.init(b, options.host_target, .Debug);
    const tools = addHostTools(b, unit, crypto);
    addHostTests(b, unit, crypto);
    addFixtureStep(b, tools);
    addZkcSmokeStep(b, tools);
    addReferenceStep(b, options, crypto);
}

/// Every `-D` option of this package, read once.
const Options = struct {
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    crypto: common.CryptoOptions,
    host_tools: bool,
    /// Directory holding the EF zkevm fixtures; its `blockchain_tests/` is the corpus.
    zkevm_fixtures: ?std.Build.LazyPath,
    host_target: std.Build.ResolvedTarget,

    fn parse(b: *std.Build) Options {
        // All guests target the same freestanding rv64im profile.
        const target = common.standardGuestTarget(b);
        // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
        const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;
        const host_tools = b.option(bool, "host-tools", "Build native test and helper targets") orelse false;
        const zkevm_fixtures = b.option(std.Build.LazyPath, "zkevm-fixtures", "Directory holding the EF zkevm fixtures; enables the reference-test step");
        return .{
            .target = target,
            .optimize = optimize,
            .crypto = common.CryptoOptions.parse(b),
            .host_tools = host_tools,
            .zkevm_fixtures = zkevm_fixtures,
            .host_target = b.resolveTargetQuery(.{}),
        };
    }
};

/// What the sources of the package import by name, for one target.
const Deps = struct {
    guest_common: *std.Build.Module,
    core_serde: *std.Build.Module,
    lineth_da: *std.Build.Dependency,
    /// Imports of the package module (`src/root.zig`).
    package: []const Import,

    /// `host_tools` also builds the test-side modules `lineth_da` exposes to its consumers.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, host_tools: bool) Deps {
        const guest_common = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
        const lineth_da = if (host_tools)
            b.dependency("lineth_da", .{ .target = target, .optimize = optimize, .@"host-tools" = true })
        else
            b.dependency("lineth_da", .{ .target = target, .optimize = optimize });
        return .{
            .guest_common = guest_common.module("guest_common"),
            .core_serde = guest_common.module("core_serde"),
            .lineth_da = lineth_da,
            .package = b.allocator.dupe(Import, &.{
                .{ .name = "guest_common", .module = guest_common.module("guest_common") },
                .{ .name = "core_serde", .module = guest_common.module("core_serde") },
                .{ .name = "lineth_da", .module = lineth_da.module("lineth_da") },
            }) catch @panic("OOM"),
        };
    }
};

/// The self-contained rv64im guest ELF (rollup statement, data-availability verification, crypto,
/// input I/O).
fn addGuestProgram(b: *std.Build, options: Options) void {
    const accelerators = common.acceleratorsDependency(b, options.target, options.optimize, options.crypto);
    const deps = Deps.init(b, options.target, options.optimize, false);
    const package = common.createPackageModule(b, options.target, options.optimize, deps.package);
    common.addGuest(b, .{
        .name = "rollup_guest",
        .target = options.target,
        .optimize = options.optimize,
        .imports = &.{
            .{ .name = "rollup", .module = package },
            .{ .name = "lineth_accelerators", .module = accelerators.module("lineth_accelerators") },
            .{ .name = "zkvm_provide", .module = accelerators.module("zkvm_provide") },
        },
        .code_model = .medium,
    });
}

/// What the native tests and tools of one optimize mode share: the modules they import.
const Host = struct {
    native: common.NativeOptions,
    lineth_da: *std.Build.Dependency,

    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) Host {
        const deps = Deps.init(b, target, optimize, true);
        const package = common.createPackageModule(b, target, optimize, deps.package);
        const da_support = deps.lineth_da.module("support");
        const zstd_compressor = deps.lineth_da.module("zstd_compressor");
        const support = b.createModule(.{
            .root_source_file = b.path("test/support.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "rollup", .module = package },
                .{ .name = "core_serde", .module = deps.core_serde },
            },
        });
        const scenario = b.createModule(.{
            .root_source_file = b.path("test/scenario.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "rollup", .module = package },
                .{ .name = "core_serde", .module = deps.core_serde },
                .{ .name = "lineth_da", .module = deps.lineth_da.module("lineth_da") },
                .{ .name = "da_support", .module = da_support },
                .{ .name = "zstd_compressor", .module = zstd_compressor },
            },
        });
        return .{
            .lineth_da = deps.lineth_da,
            // Tests run on the native host: a freestanding riscv64 target has no OS to run a
            // `std.testing` binary against.
            .native = .{
                .target = target,
                .optimize = optimize,
                .imports = b.allocator.dupe(Import, &.{
                    .{ .name = "rollup", .module = package },
                    .{ .name = "core_serde", .module = deps.core_serde },
                    .{ .name = "guest_common", .module = deps.guest_common },
                    .{ .name = "lineth_da", .module = deps.lineth_da.module("lineth_da") },
                    .{ .name = "support", .module = support },
                    .{ .name = "scenario", .module = scenario },
                }) catch @panic("OOM"),
            },
        };
    }
};

/// Every `tools/*.zig` executable. `gen_fixture` runs the statement and links the host crypto
/// provider.
fn addHostTools(b: *std.Build, host: Host, crypto: common.HostCrypto) Tools {
    const tools = common.addTools(b, host.native, .{});
    crypto.link(tools.get("gen_fixture").?);
    return tools;
}

/// The `test/*_test.zig` suites.
fn addHostTests(b: *std.Build, host: Host, crypto: common.HostCrypto) void {
    const test_step = b.step("test", "Run native Zig unit tests for the rollup guest");
    crypto.linkAll(common.addTests(b, test_step, host.native, .{}));
}

/// Runs `gen_fixture` with the arguments after `--`.
fn addFixtureStep(b: *std.Build, tools: Tools) void {
    const gen_fixture = b.addRunArtifact(tools.get("gen_fixture").?);
    if (b.args) |args| gen_fixture.addArgs(args);
    b.step("gen-fixture", "Write a sample RollupProofPrivateInput and its expected output to the paths after `--`").dependOn(&gen_fixture.step);
}

/// Runs the guest ELF under zkc and compares its output with the native statement's; the options
/// of `zkc_smoke` follow `--`.
fn addZkcSmokeStep(b: *std.Build, tools: Tools) void {
    const zkc_smoke = b.addRunArtifact(tools.get("zkc_smoke").?);
    if (b.args) |args| zkc_smoke.addArgs(args);
    b.step("zkc-smoke", "Run a guest ELF under zkc and compare its output with the native statement's").dependOn(&zkc_smoke.step);
}

/// The `test/reference/*_test.zig` suites over the Amsterdam blocks of the EF zkevm fixtures, run
/// by the `reference-test` step, which registers when `-Dzkevm-fixtures` is given. They are
/// compiled optimized: the corpus is gigabytes of JSON.
fn addReferenceStep(b: *std.Build, options: Options, crypto: common.HostCrypto) void {
    const fixtures = options.zkevm_fixtures orelse return;
    const paths = b.addOptions();
    paths.addOptionPath("amsterdam_blocks", fixtures.path(b, "blockchain_tests/for_amsterdam"));

    const host = Host.init(b, options.host_target, .ReleaseSafe);
    const imports = std.mem.concat(b.allocator, Import, &.{
        host.native.imports,
        &.{
            .{ .name = "ef_fixtures", .module = paths.createModule() },
            .{ .name = "ef_blocks", .module = host.lineth_da.module("ef_blocks") },
        },
    }) catch @panic("OOM");
    const step = b.step("reference-test", "Run the rollup guest over consecutive blocks of the EF zkevm fixtures");
    crypto.linkAll(common.addTests(b, step, .{
        .target = host.native.target,
        .optimize = host.native.optimize,
        .imports = imports,
    }, .{ .dir = "test/reference" }));
}
