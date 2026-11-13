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
    const prover = addDevmodeProver(b);
    const unit = Host.init(b, options.host_target, .Debug, null, prover);
    const tools = addHostTools(b, unit, crypto);
    addHostTests(b, options, unit, crypto, prover);
    addFixtureStep(b, tools);
    addFetchLineaBlocksStep(b, tools);
    addZkcSmokeStep(b, tools);
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
        const zkevm_fixtures = b.option(std.Build.LazyPath, "zkevm-fixtures", "Directory holding the EF zkevm fixtures; enables the test/reference suites");
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
    zkvm_accelerators: *std.Build.Module,
    verifier_ray: *std.Build.Module,
    lineth_da: *std.Build.Dependency,
    /// Imports of the package module (`src/root.zig`).
    package: []const Import,

    /// `host_tools` also builds the test-side modules `lineth_da` exposes to its consumers.
    /// `guest` builds the verifier for the zkVM, with its pure-Zig Poseidon2.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, host_tools: bool, guest: bool) Deps {
        const guest_common = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
        const lineth_da = if (host_tools)
            b.dependency("lineth_da", .{ .target = target, .optimize = optimize, .@"host-tools" = true })
        else
            b.dependency("lineth_da", .{ .target = target, .optimize = optimize });
        // The same instance `lineth_da` and `guest_common` import, so one compilation holds the module once.
        const zkvm_accelerators = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("zkvm_accelerators");
        // The riscv system option is the one the verifier package requires; the devmode system is
        // the one this package verifies against.
        const devmode_system_path = b.pathFromRoot(devmode_system_source);
        const verifier_ray = if (guest)
            b.dependency("verifier_ray", .{
                .target = target,
                .optimize = optimize,
                .r5 = true,
                .@"disable-accelerators" = true,
                .@"riscv-system" = devmode_system_path,
            })
        else
            b.dependency("verifier_ray", .{ .target = target, .optimize = optimize, .@"riscv-system" = devmode_system_path });
        return .{
            .guest_common = guest_common.module("guest_common"),
            .core_serde = guest_common.module("core_serde"),
            .zkvm_accelerators = zkvm_accelerators,
            .verifier_ray = verifier_ray.module("verifier_ray"),
            .lineth_da = lineth_da,
            .package = b.allocator.dupe(Import, &.{
                .{ .name = "guest_common", .module = guest_common.module("guest_common") },
                .{ .name = "core_serde", .module = guest_common.module("core_serde") },
                .{ .name = "lineth_da", .module = lineth_da.module("lineth_da") },
                .{ .name = "zkvm_accelerators", .module = zkvm_accelerators },
                .{ .name = "verifier_ray", .module = verifier_ray.module("verifier_ray") },
                .{ .name = "devmode_system", .module = verifier_ray.module("devmode_system") },
            }) catch @panic("OOM"),
        };
    }
};

/// The generated devmode system of verifier-ray, relative to this package.
const devmode_system_source = "../../verifier-ray/testdata/generated/devmode_system.zig";

/// The self-contained rv64im guest ELF (rollup statement, data-availability verification, crypto,
/// input I/O).
fn addGuestProgram(b: *std.Build, options: Options) void {
    const accelerators = common.acceleratorsDependency(b, options.target, options.optimize, options.crypto);
    const deps = Deps.init(b, options.target, options.optimize, false, true);
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

/// The devmode prover of verifier-ray (`generate-devmode-system`, run with `-prove`) built for the
/// host: the program the tests and tools hand the proofs they need to. `go` rebuilds it on every run
/// and caches what did not change.
fn addDevmodeProver(b: *std.Build) std.Build.LazyPath {
    const go = b.addSystemCommand(&.{ "go", "build", "-o" });
    const binary = go.addOutputFileArg("generate-devmode-system");
    go.addArg("./codegen/generate-devmode-system");
    go.setCwd(b.path("../../verifier-ray"));
    go.has_side_effects = true;
    return binary;
}

/// What the native tests and tools of one optimize mode share: the modules they import.
const Host = struct {
    native: common.NativeOptions,

    /// `fixtures` is the directory of the EF zkevm fixtures the `ef_fixtures` module names.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, fixtures: ?std.Build.LazyPath, prover: std.Build.LazyPath) Host {
        const deps = Deps.init(b, target, optimize, true, false);
        const package = common.createPackageModule(b, target, optimize, deps.package);
        const da_support = deps.lineth_da.module("support");
        const zstd_encoder = deps.lineth_da.module("zstd_encoder");
        const ef_blocks = deps.lineth_da.module("ef_blocks");
        const paths = b.addOptions();
        if (fixtures) |dir| {
            paths.addOptionPath("amsterdam_blocks", dir.path(b, "blockchain_tests/for_amsterdam"));
        } else {
            paths.addOption([]const u8, "amsterdam_blocks", "");
        }
        paths.addOptionPath("linea_window", b.path("test/testdata/linea_mainnet_27434569_27434718.json.gz"));
        const ef_fixtures = paths.createModule();
        const sample_input = b.createModule(.{
            .root_source_file = b.path("test/support/sample_input.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "rollup", .module = package },
                .{ .name = "core_serde", .module = deps.core_serde },
                .{ .name = "guest_common", .module = deps.guest_common },
            },
        });
        const prover_paths = b.addOptions();
        prover_paths.addOptionPath("devmode_prove", prover);
        const devmode_prover = b.createModule(.{
            .root_source_file = b.path("test/support/devmode_prover.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "prover_paths", .module = prover_paths.createModule() },
                .{ .name = "rollup", .module = package },
                .{ .name = "verifier_ray", .module = deps.verifier_ray },
            },
        });
        const incompressible_blocks = b.createModule(.{
            .root_source_file = b.path("test/support/incompressible_blocks.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "ef_blocks", .module = ef_blocks },
                .{ .name = "lineth_da", .module = deps.lineth_da.module("lineth_da") },
                .{ .name = "zesu_executor", .module = deps.lineth_da.module("zesu_executor") },
            },
        });
        const scenario = b.createModule(.{
            .root_source_file = b.path("test/support/scenario.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "rollup", .module = package },
                .{ .name = "core_serde", .module = deps.core_serde },
                .{ .name = "sample_input", .module = sample_input },
                .{ .name = "devmode_prover", .module = devmode_prover },
                .{ .name = "lineth_da", .module = deps.lineth_da.module("lineth_da") },
                .{ .name = "da_support", .module = da_support },
                .{ .name = "zstd_encoder", .module = zstd_encoder },
                .{ .name = "ef_blocks", .module = ef_blocks },
                .{ .name = "ef_fixtures", .module = ef_fixtures },
                .{ .name = "incompressible_blocks", .module = incompressible_blocks },
            },
        });
        return .{
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
                    .{ .name = "zesu_executor", .module = deps.lineth_da.module("zesu_executor") },
                    .{ .name = "sample_input", .module = sample_input },
                    .{ .name = "scenario", .module = scenario },
                    .{ .name = "devmode_prover", .module = devmode_prover },
                    .{ .name = "ef_blocks", .module = ef_blocks },
                    .{ .name = "ef_fixtures", .module = ef_fixtures },
                    .{ .name = "zkvm_accelerators", .module = deps.zkvm_accelerators },
                }) catch @panic("OOM"),
            },
        };
    }
};

/// Every `tools/*.zig` executable. `gen_fixture` runs the statement and `fetch_linea_blocks`
/// recovers senders; both link the host crypto provider.
fn addHostTools(b: *std.Build, host: Host, crypto: common.HostCrypto) Tools {
    const tools = common.addTools(b, host.native, .{});
    crypto.link(tools.get("gen_fixture").?);
    crypto.link(tools.get("fetch_linea_blocks").?);
    return tools;
}

/// The `test/*_test.zig` suites, and with `-Dzkevm-fixtures` the `test/reference/*_test.zig` suites
/// over the Amsterdam blocks of the EF zkevm fixtures. The reference suites are compiled optimized:
/// the corpus is gigabytes of JSON.
fn addHostTests(b: *std.Build, options: Options, unit: Host, crypto: common.HostCrypto, prover: std.Build.LazyPath) void {
    const test_step = b.step("test", "Run native Zig tests for the rollup guest");
    crypto.linkAll(common.addTests(b, test_step, unit.native, .{}));

    const fixtures = options.zkevm_fixtures orelse {
        if (b.pkg_hash.len == 0) std.log.warn("test/reference skipped: pass -Dzkevm-fixtures=<dir> (`make test` does)", .{});
        return;
    };
    const reference = Host.init(b, options.host_target, .ReleaseSafe, fixtures, prover);
    crypto.linkAll(common.addTests(b, test_step, reference.native, .{ .dir = "test/reference" }));
}

/// Runs `gen_fixture` with the arguments after `--`.
fn addFixtureStep(b: *std.Build, tools: Tools) void {
    const gen_fixture = b.addRunArtifact(tools.get("gen_fixture").?);
    if (b.args) |args| gen_fixture.addArgs(args);
    b.step("gen-fixture", "Write a sample RollupProofPrivateInput and its expected output to the paths after `--`").dependOn(&gen_fixture.step);
}

/// Runs `fetch_linea_blocks` with the arguments after `--`.
fn addFetchLineaBlocksStep(b: *std.Build, tools: Tools) void {
    const fetch_linea_blocks = b.addRunArtifact(tools.get("fetch_linea_blocks").?);
    if (b.args) |args| fetch_linea_blocks.addArgs(args);
    b.step("fetch-linea-blocks", "Rebuild the Linea mainnet window from the public RPC into the directory after `--`").dependOn(&fetch_linea_blocks.step);
}

/// Runs the guest ELF under zkc and compares its output with the native statement's; the options
/// of `zkc_smoke` follow `--`.
fn addZkcSmokeStep(b: *std.Build, tools: Tools) void {
    const zkc_smoke = b.addRunArtifact(tools.get("zkc_smoke").?);
    if (b.args) |args| zkc_smoke.addArgs(args);
    b.step("zkc-smoke", "Run a guest ELF under zkc and compare its output with the native statement's").dependOn(&zkc_smoke.step);
}
