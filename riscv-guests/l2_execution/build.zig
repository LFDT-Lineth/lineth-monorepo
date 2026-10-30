const std = @import("std");
const common = @import("build_common");

const Import = std.Build.Module.Import;
const Compile = std.Build.Step.Compile;
const Tools = std.StringHashMap(*Compile);
const Secp256k1Backend = enum { std, constantine };

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    const options = Options.parse(b);
    addGuestProgram(b, options);
    if (!options.host_tools) return;

    const host = Host.init(b, options) orelse return;
    const tools = addHostTools(b, host);
    addWrapSteps(b, tools);
    addZkcSmokeSteps(b, tools, options);
    addRunnerSteps(b, tools);
    addHostTests(b, host);
    addReferenceSteps(b, host, tools, options);
}

/// Every `-D` option of this package, read once.
const Options = struct {
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    /// Keccak provider: the arithmetization wrapper (prover-accelerated custom op) or standard zig keccak.
    keccak_accel: bool,
    /// secp256k1 ecrecover/verify provider: standard zig or the Constantine staticlib.
    secp256k1_backend: Secp256k1Backend,
    fixtures_link: []const u8,
    zkc_smoke_input: []const u8,
    host_tools: bool,

    fn parse(b: *std.Build) Options {
        // All guests target the same freestanding rv64im profile.
        const target = common.standardGuestTarget(b);
        // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
        const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;
        const keccak_accel = b.option(bool, "keccak-accel", "Use the arithmetization keccak wrapper instead of standard zig keccak (default: unaccelerated)") orelse false;
        const secp256k1_backend = b.option(Secp256k1Backend, "secp256k1-backend", "secp256k1 ecrecover/verify provider: std (default) or constantine") orelse .std;
        const fixtures_link = b.option([]const u8, "execution-specs-fixtures-link", "Path where execution-specs zkevm fixtures are exposed") orelse "/tmp/execution-specs-json-fixtures/fixtures";
        const zkc_smoke_input = b.option([]const u8, "zkc-smoke-input", "Extended SSZ input for the ZkC smoke test") orelse "test/testdata/stateless_input.ssz";
        const host_tools = b.option(bool, "host-tools", "Build native test and helper targets") orelse false;
        return .{
            .target = target,
            .optimize = optimize,
            .keccak_accel = keccak_accel,
            .secp256k1_backend = secp256k1_backend,
            .fixtures_link = fixtures_link,
            .zkc_smoke_input = zkc_smoke_input,
            .host_tools = host_tools,
        };
    }
};

/// The accelerator package with its opt-in `zkvm_*` providers. The wrappers and every provider
/// module come from this one instance, so they share one set of sources and build options.
fn acceleratorsDependency(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    keccak_accel: bool,
    secp256k1_backend: Secp256k1Backend,
) *std.Build.Dependency {
    return b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = optimize,
        .@"zkvm-provide" = true,
        .@"keccak-accel" = keccak_accel,
        .@"secp256k1-backend" = secp256k1_backend,
    });
}

/// The self-contained rv64im guest ELF (execution, crypto, accelerators, input I/O).
fn addGuestProgram(b: *std.Build, options: Options) void {
    const zesu = b.dependency("zesu", .{
        .target = options.target,
        .optimize = options.optimize,
        .@"crypto-backend" = .@"extern",
    });
    const accelerators = acceleratorsDependency(b, options.target, options.optimize, options.keccak_accel, options.secp256k1_backend);
    const deps = Deps.init(b, options.target, options.optimize, zesu, accelerators);

    const package = common.createPackageModule(b, options.target, options.optimize, deps.package);
    common.addGuest(b, .{
        .name = "evm_execution_guest",
        .target = options.target,
        .optimize = options.optimize,
        .imports = &.{
            .{ .name = "l2_execution", .module = package },
            .{ .name = "core_serde", .module = deps.core_serde },
            .{ .name = "lineth_zkvm_accel", .module = accelerators.module("lineth_accelerators") },
            .{ .name = "zkvm_provide", .module = accelerators.module("zkvm_provide") },
        },
        .code_model = .medium,
    });
}

/// What native tests and tools share: build settings, the host crypto provider, the support module and the named test and fixture steps.
const Host = struct {
    native: common.NativeOptions,
    crypto: Crypto,
    zesu: *std.Build.Dependency,
    support: *std.Build.Module,
    test_step: *std.Build.Step,
    extended_vanilla_step: *std.Build.Step,
    prep_fixtures_step: *std.Build.Step,

    /// Null until the Constantine archive exists: it is fetched on demand and the build reruns once it is.
    fn init(b: *std.Build, options: Options) ?Host {
        const archive = b.dependency("guest_crypto", .{}).builder.named_lazy_paths.get("host_staticlib") orelse return null;
        // Host artifacts never build at ReleaseSmall: zig 0.16 -Oz miscompiles zesu's value-semantics
        // hot paths on aarch64 hosts (stack slots of by-value hash-map captures and `self` receivers
        // are recycled while still live), giving SIGSEGVs and wrong results across the EF zkevm
        // suite. Debug/ReleaseSafe/ReleaseFast pass; ReleaseSafe is the optimized mode CI runs. The
        // rv64im guest keeps `optimize` (ReleaseSmall) for the prover toolchain.
        const optimize: std.builtin.OptimizeMode =
            if (options.optimize == .ReleaseSmall) .ReleaseSafe else options.optimize;
        // Baseline CPU, not the build host's: `l2_execution_runner` ships in the prover-ray image,
        // so host-specific extensions (e.g. AVX-512 on the build machine) would SIGILL on older
        // or emulated (Rosetta) x86_64 hosts.
        const target = b.resolveTargetQuery(.{ .cpu_model = .baseline });
        const zlob = b.lazyDependency("zlob", .{ .target = target, .optimize = optimize }) orelse
            @panic("zlob dependency is required for native test and helper targets");
        const zesu = b.dependency("zesu", .{
            .target = target,
            .optimize = optimize,
            .@"crypto-backend" = .@"extern",
        });
        const accelerators = acceleratorsDependency(b, target, optimize, false, options.secp256k1_backend);
        const deps = Deps.init(b, target, optimize, zesu, accelerators);
        const package = common.createPackageModule(b, target, optimize, deps.package);
        const support = supportModule(b, target, optimize, deps, package, zlob);
        // The Constantine-backed `zkvm_*` provider the guest links, as an object for native roots.
        const provider = b.addObject(.{ .name = "zkvm_provide", .root_module = accelerators.module("zkvm_provide") });

        const test_step = b.step("test", "Run native Zig unit tests for the EVM execution guest");
        const extended_vanilla_step = b.step("extended-vanilla", "Reference-test guard: assert the dummy-wrapped extended guest (runL2Execution) agrees with the EF fixture's own expected validity over EF zkevm fixtures");
        const prep_fixtures_step = b.step("prep-execution-specs-json-fixtures", "Expose EF zkevm stateless fixtures for external runners");
        return .{
            // Tests run on the native host: a freestanding riscv64 target has no OS to run a
            // `std.testing` binary against.
            .native = .{
                .target = target,
                .optimize = optimize,
                .imports = concatImports(b, &.{ &deps.zesu, &.{
                    .{ .name = "l2_execution", .module = package },
                    .{ .name = "core_serde", .module = deps.core_serde },
                    .{ .name = "guest_crypto", .module = accelerators.module("guest_crypto") },
                    .{ .name = "secp256k1_std", .module = accelerators.module("secp256k1_std") },
                    .{ .name = "support", .module = support },
                    .{ .name = "zlob", .module = zlob.module("zlob") },
                } }),
            },
            .crypto = .{ .provider = provider, .constantine = archive },
            .zesu = zesu,
            .support = support,
            .test_step = test_step,
            .extended_vanilla_step = extended_vanilla_step,
            .prep_fixtures_step = prep_fixtures_step,
        };
    }
};

/// The `test/support` helpers shared by the suites and tools.
fn supportModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    deps: Deps,
    package: *std.Build.Module,
    zlob: *std.Build.Dependency,
) *std.Build.Module {
    const support = b.createModule(.{
        .root_source_file = b.path("test/support/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = concatImports(b, &.{ &deps.zesu, &.{
            .{ .name = "l2_execution", .module = package },
            .{ .name = "core_serde", .module = deps.core_serde },
            .{ .name = "guest_crypto", .module = deps.accelerators.module("guest_crypto") },
            .{ .name = "zlob", .module = zlob.module("zlob") },
        } }),
    });
    // Lazy: fetched when the helpers are built. The module is named "ssz.zig" (the dependency's own
    // b.addModule argument).
    if (b.lazyDependency("ssz", .{ .target = target, .optimize = optimize })) |ssz| {
        support.addImport("ssz", ssz.module("ssz.zig"));
    }
    return support;
}

/// Builds every `tools/*.zig` executable, each linked against the host crypto provider.
fn addHostTools(b: *std.Build, host: Host) Tools {
    const tools = common.addTools(b, host.native, .{});
    host.crypto.linkAll(tools);
    return tools;
}

/// Installs `l2_execution_wrap` beside the guest ELF and adds its run step. The tool wraps a vanilla
/// EF stateless-input .ssz into an extended input (zero l2MessageServiceAddress -> bridge
/// suppression) for hand-feeding one corpus input to the guest ELF under zkc.
fn addWrapSteps(b: *std.Build, tools: Tools) void {
    const wrap = tools.get("l2_execution_wrap").?;
    b.installArtifact(wrap);
    addRunStep(b, wrap, "l2-execution-wrap", "Run the native l2_execution_wrap tool (vanilla .ssz -> extended .ssz)");
}

/// Installs `zkc_reference_runner` and adds the `zkc-smoke` steps: one extended SSZ input under
/// ZkC, checked against the host machine. They need zkc and go on PATH; the Makefile sets
/// `-Dzkc-smoke-input` from INPUT.
fn addZkcSmokeSteps(b: *std.Build, tools: Tools, options: Options) void {
    const runner = tools.get("zkc_reference_runner").?;
    b.installArtifact(runner);

    const smoke = b.addRunArtifact(runner);
    smoke.addArg(options.zkc_smoke_input);
    finishZkcRun(b, smoke, true);
    b.step("zkc-smoke", "Run testdata .ssz under ZkC and compare it with the host machine").dependOn(&smoke.step);

    // An externally prepared production ELF.
    const external = b.addRunArtifact(runner);
    finishZkcRun(b, external, false);
    b.step("zkc-smoke-external", "Run an external guest ELF under ZkC and compare it with the host machine").dependOn(&external.step);
}

/// Adds `install-runner` (the native runner for the prover image, also part of the default install)
/// and the `l2-execution-runner` run step. The runner takes an arbitrary extended-input file, so it
/// does not depend on the EF fixtures.
fn addRunnerSteps(b: *std.Build, tools: Tools) void {
    const runner = tools.get("l2_execution_runner").?;
    const install = b.addInstallArtifact(runner, .{});
    b.getInstallStep().dependOn(&install.step);
    b.step("install-runner", "Install the native runner for the prover image").dependOn(&install.step);
    addRunStep(b, runner, "l2-execution-runner", "Run the native l2_execution_runner (SSZ extended-input file -> SSZ/JSON output on stdout)");
}

/// A named step running `exe` with the arguments after `--`.
fn addRunStep(b: *std.Build, exe: *Compile, name: []const u8, description: []const u8) void {
    const run = b.addRunArtifact(exe);
    if (b.args) |extra| run.addArgs(extra);
    b.step(name, description).dependOn(&run.step);
}

/// Completes a zkc runner invocation with the arithmetization Makefile (which defines the zkc run)
/// and the arguments after `--`; `installed_elf` runs the ELF from the install prefix.
fn finishZkcRun(b: *std.Build, run: *std.Build.Step.Run, installed_elf: bool) void {
    if (installed_elf) {
        run.addArg("--install-prefix");
        run.addArg(b.install_prefix);
    }
    run.addArg("--makefile");
    run.addFileArg(b.path("../../arithmetization/src/test/Makefile"));
    if (b.args) |extra| run.addArgs(extra);
    if (installed_elf) run.step.dependOn(b.getInstallStep());
}

/// The `test/*_test.zig` suites and the support root's own tests, with the corpus fixtures they embed.
fn addHostTests(b: *std.Build, host: Host) void {
    const suites = common.addTests(b, host.test_step, host.native, .{});
    host.crypto.linkAll(suites);
    host.crypto.linkAll(common.addTests(b, host.test_step, host.native, .{ .dir = "test/support", .suffix = "root.zig" }));
    addCryptoVectors(b, host.zesu, suites.get("test/guest_crypto_test.zig").?);
    addCorpusTests(b, host);
}

/// The suites under test/corpus/ read EF zkevm corpus vectors; they register once the lazy corpus
/// dependency is fetched.
fn addCorpusTests(b: *std.Build, host: Host) void {
    const fixtures = fixturesDependency(b) orelse return;
    embedFixtures(host.support, fixtures);
    host.crypto.linkAll(common.addTests(b, host.test_step, host.native, .{ .dir = "test/corpus" }));
}

/// The EF zkevm corpus; lazy, fetched before the tests build.
fn fixturesDependency(b: *std.Build) ?*std.Build.Dependency {
    return b.lazyDependency("execution_spec_tests_zkevm", .{});
}

/// Embeds two corpus vectors straight from the dependency tree (no committed copy). The empty-block
/// vector has no transactions, so only keccak is exercised of the native crypto backend.
fn embedFixtures(support: *std.Build.Module, fixtures: *std.Build.Dependency) void {
    support.addAnonymousImport("zkevm_stateless_block.json", .{
        .root_source_file = fixtures.path("blockchain_tests/for_amsterdam/amsterdam/eip7928_block_level_access_lists/block_access_lists/bal_empty_block_no_coinbase.json"),
    });
    support.addAnonymousImport("slotnum_distinct_per_block.json", .{
        .root_source_file = fixtures.path("blockchain_tests/for_amsterdam/amsterdam/eip7843_slotnum/slotnum/slotnum_distinct_per_block.json"),
    });
}

/// Embeds the guest_crypto vectors as `<file stem>_csv`: zesu's EIP-196/197/2537 testdata (CSV:
/// input,result,gas,notes; hex without 0x; empty notes = success row) and this package's own
/// ecrecover vectors.
fn addCryptoVectors(b: *std.Build, zesu: *std.Build.Dependency, guest_crypto_tests: *Compile) void {
    for (common.filesIn(zesu.builder, "src/evm/precompile/testdata", ".csv")) |csv| {
        guest_crypto_tests.root_module.addAnonymousImport(b.fmt("{s}_csv", .{std.fs.path.stem(csv)}), .{
            .root_source_file = zesu.path(csv),
        });
    }
    for (common.filesIn(b, "test/testdata", ".csv")) |csv| {
        guest_crypto_tests.root_module.addAnonymousImport(b.fmt("{s}_csv", .{std.fs.path.stem(csv)}), .{
            .root_source_file = b.path(csv),
        });
    }
}

/// The reference tests over the EF corpus: `extended-vanilla`, `reference-test-zkc` and the fixture link.
fn addReferenceSteps(b: *std.Build, host: Host, tools: Tools, options: Options) void {
    const fixtures = fixturesDependency(b) orelse return;
    const corpus = fixtures.path("blockchain_tests");

    // The dummy-wrapped extended input must get the fixture's own validity verdict from
    // `l2_execution.runL2Execution`; that one comparison exercises the delegated execution seam
    // end to end. Extra arguments after `--`, e.g. `-- --fork Amsterdam --limit 300`.
    const extended = b.addRunArtifact(tools.get("extended_vanilla_runner").?);
    extended.addDirectoryArg(corpus);
    if (b.args) |extra| extended.addArgs(extra);
    host.extended_vanilla_step.dependOn(&extended.step);

    // The corpus through the installed guest ELF under zkc (needs zkc and go on PATH).
    const zkc = b.addRunArtifact(tools.get("zkc_reference_runner").?);
    zkc.addDirectoryArg(corpus);
    finishZkcRun(b, zkc, true);
    b.step("reference-test-zkc", "Run the EF zkevm corpus through the guest ELF under zkc (needs zkc+go on PATH)").dependOn(&zkc.step);

    addFixturesLink(b, fixtures, host.prep_fixtures_step, options.fixtures_link);
}

/// Symlinks the EF fixtures at `link`, replacing any previous link.
fn addFixturesLink(b: *std.Build, fixtures: *std.Build.Dependency, step: *std.Build.Step, link: []const u8) void {
    const mkdir_parent = b.addSystemCommand(&.{ "mkdir", "-p", std.fs.path.dirname(link) orelse "." });
    const remove_old = b.addSystemCommand(&.{ "rm", "-rf", link });
    remove_old.step.dependOn(&mkdir_parent.step);

    const symlink = b.addSystemCommand(&.{ "ln", "-s" });
    symlink.addDirectoryArg(fixtures.path("."));
    symlink.addArg(link);
    symlink.step.dependOn(&remove_old.step);
    step.dependOn(&symlink.step);
}

/// What l2_execution's source files import by name, for one target.
const Deps = struct {
    /// zesu's exposed modules under the `zesu_*` names the sources use.
    zesu: [13]Import,
    core_serde: *std.Build.Module,
    accelerators: *std.Build.Dependency,
    /// Imports of the package module (`src/root.zig`).
    package: []const Import,

    fn init(
        b: *std.Build,
        target: std.Build.ResolvedTarget,
        optimize: std.builtin.OptimizeMode,
        zesu_dep: *std.Build.Dependency,
        accelerators: *std.Build.Dependency,
    ) Deps {
        const zesu = zesuImports(zesu_dep);
        const guest_common = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
        const core_serde = guest_common.module("core_serde");
        return .{
            .zesu = zesu,
            .core_serde = core_serde,
            .accelerators = accelerators,
            .package = concatImports(b, &.{ &zesu, &.{
                .{ .name = "guest_common", .module = guest_common.module("guest_common") },
                .{ .name = "core_serde", .module = core_serde },
            } }),
        };
    }
};

/// Pull zesu's exposed modules by name. Which crypto backend zesu uses is selected inside zesu by
/// target (freestanding leaves zkvm_* extern; native links real crypto) — that's zesu's concern.
fn zesuImports(zesu: *std.Build.Dependency) [13]Import {
    return .{
        .{ .name = "zesu_allocator", .module = zesu.module("zesu_allocator") },
        .{ .name = "zesu_executor", .module = zesu.module("executor") },
        .{ .name = "zesu_ssz_decode", .module = zesu.module("ssz_decode") },
        .{ .name = "zesu_ssz_output", .module = zesu.module("ssz_output") },
        .{ .name = "zesu_primitives", .module = zesu.module("primitives") },
        .{ .name = "zesu_mpt", .module = zesu.module("mpt") },
        .{ .name = "zesu_db", .module = zesu.module("db") },
        .{ .name = "zesu_context", .module = zesu.module("context") },
        .{ .name = "zesu_input", .module = zesu.module("input") },
        .{ .name = "zesu_hardfork", .module = zesu.module("hardfork") },
        .{ .name = "zesu_rlp_decode", .module = zesu.module("rlp_decode") },
        // The crypto-accelerator interface tx_signing.zig uses for ecrecover/keccak256; the
        // `executor` module does not re-export it.
        .{ .name = "zesu_accelerators", .module = zesu.module("accelerators") },
        .{ .name = "zesu_block_rlp_size", .module = zesu.module("block_rlp_size") },
    };
}

fn concatImports(b: *std.Build, parts: []const []const Import) []const Import {
    return std.mem.concat(b.allocator, Import, parts) catch @panic("OOM");
}

/// The host crypto provider native tests and tools link against: the Constantine-backed `zkvm_*`
/// object plus the Constantine archive.
const Crypto = struct {
    provider: *std.Build.Step.Compile,
    constantine: std.Build.LazyPath,

    fn link(crypto: Crypto, step: *std.Build.Step.Compile) void {
        step.root_module.addObject(crypto.provider);
        step.root_module.addObjectFile(crypto.constantine);
        step.root_module.link_libc = true;
    }

    fn linkAll(crypto: Crypto, artifacts: std.StringHashMap(*std.Build.Step.Compile)) void {
        var it = artifacts.valueIterator();
        while (it.next()) |artifact| crypto.link(artifact.*);
    }
};
