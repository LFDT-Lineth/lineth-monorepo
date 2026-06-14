const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // All guests target the same freestanding rv64im ZkC profile (shared helper).
    const target = common.standardGuestTarget(b);

    const optimize = b.standardOptimizeOption(.{
        .preferred_optimize_mode = .ReleaseSmall,
    });

    const gp_name = "evm_execution_guest";
    const source = "src/evm_execution_guest.zig";

    // ── Guest: statically-linked rv64im ELF ───────────────────────────────────
    // The zkvm-standards riscv-target deliverable is "ELF, statically linked" (RV64I+M+Zicclsm, LP64
    // soft-float): https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md
    // So the default build links a self-contained ELF the ZkC interpreter loads (via ELF→JSON). There
    // is no relocatable `.o`: a `.o` is not statically linked, and the interpreter loads a finished ELF
    // rather than performing a final link. The shared entry stub + memory layout + compiler_rt/GC
    // plumbing live in build_common.installGuestElf; here we wire the guest's root module:
    //   • zesu executor + SSZ modules — the execution logic;
    //   • zesu_zkvm_accel — zesu-zkvm's stdlibs_accel: in-guest software precompiles that
    //     zkvm_provide.zig exports as the zkvm_* symbols zesu references;
    //   • linea_zkvm_accel — Linea accelerator wrappers (keccak today): zkvm_* the prover accelerates
    //     at execution rather than at link time, so the ELF stays fully resolved;
    //   • linea_zkvm_io — zesu-zkvm's zkvm_io: satisfies the standards `read_input` by reading the
    //     memory-mapped `_in_start` (the input slot is the proving system's detail, kept out of the
    //     guest; `_in_start` is supplied by the linker script).
    const zesu_guest = b.dependency("zesu", .{ .target = target, .optimize = optimize });
    const zesu_zkvm = b.dependency("zesu_zkvm", .{});
    const zesu_accel_src = zesu_zkvm.path("linea/src/runtime/stdlibs_accel.zig"); // also imported by the native accel test below
    const zesu_accel_mod = b.createModule(.{
        .root_source_file = zesu_accel_src,
        .target = target,
        .optimize = optimize,
    });
    const linea_accel_mod = b.createModule(.{
        .root_source_file = b.path("../../arithmetization/src/wrappers/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    const linea_io_mod = b.createModule(.{
        .root_source_file = zesu_zkvm.path("linea/src/zkvm_io.zig"),
        .target = target,
        .optimize = optimize,
    });

    const guest_module = b.createModule(.{
        .root_source_file = b.path(source),
        .target = target,
        .optimize = optimize,
    });
    guest_module.code_model = .medium;
    addExecutionImports(guest_module, zesuImports(zesu_guest));
    guest_module.addImport("zesu_zkvm_accel", zesu_accel_mod);
    guest_module.addImport("linea_zkvm_accel", linea_accel_mod);
    guest_module.addImport("linea_zkvm_io", linea_io_mod);
    common.clearFreestandingNativeLinkage(b, guest_module);
    common.installGuestElf(b, guest_module, gp_name);

    // ── Native test ───────────────────────────────────────────────────────────
    // Runs the thin wrapper (vanilla zesu stateless execution) on the host against a real
    // execution-spec-tests zkevm SSZ fixture, asserting the serialized validation result matches —
    // the same end-to-end check as zesu's zkevm-blockchain-test-runner. Links zesu's full native
    // crypto backend; linea adds the library search path so it links on macOS. The
    // committed fixture is an empty block (only keccak), but the full backend is linked so the suite
    // can grow to tx-bearing fixtures (ecrecover/curves) without further build changes.
    //
    // Host artifacts never build at ReleaseSmall: zig 0.16 (stable and dev.3153) -Oz miscompiles
    // zesu's value-semantics hot paths on aarch64 hosts — stack slots of by-value hash-map captures
    // (`if (m.get(k)) |v|` + iterate) and by-value `self` receivers are recycled while still live,
    // yielding SIGSEGVs/wrong results across the EF zkevm suite (first hit: BaTracker.computeHash,
    // zesu transition.zig). Debug/ReleaseSafe/ReleaseFast pass (23,264/23,264 blocks). ReleaseSafe
    // is the optimized mode CI runs for host tests; the rv64im guest object above keeps `optimize`
    // (ReleaseSmall) for the prover toolchain.
    const host_optimize: std.builtin.OptimizeMode =
        if (optimize == .ReleaseSmall) .ReleaseSafe else optimize;
    const native_target = b.resolveTargetQuery(.{});
    const native_crypto = resolveNativeCrypto(b, native_target);
    const zesu_native = b.dependency("zesu", .{ .target = native_target, .optimize = host_optimize });
    const native_imports = zesuImports(zesu_native);

    const guest_mod = b.createModule(.{
        .root_source_file = b.path(source),
        .target = native_target,
        .optimize = host_optimize,
    });
    addExecutionImports(guest_mod, native_imports);

    const test_step = b.step("test", "Run native Zig unit tests for the EVM execution guest");
    const spec_step = b.step("spec-tests", "Run the guest against all EF zkevm stateless fixtures (host)");

    // Integration smoke test for the delegated precompiles: verifies zesu-zkvm's stdlibs_accel
    // imports and that its ecrecover round-trips (the in-guest precompiles delegate to it). std +
    // the dependency only — no fixtures, no native crypto libs.
    const accel_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/stdlibs_accel_test.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    accel_tests.root_module.addImport("zesu_zkvm_accel", b.createModule(.{
        .root_source_file = zesu_accel_src,
        .target = native_target,
        .optimize = host_optimize,
    }));
    test_step.dependOn(&b.addRunArtifact(accel_tests).step);

    // The SSZ fixture comes from the execution-spec-tests zkevm dependency (lazy: only fetched when
    // this test is built). An empty-block vector → no transactions → no secp256k1/curve precompiles,
    // so the full native crypto backend is linked but only keccak is exercised.
    const fixture_rel = "blockchain_tests/for_amsterdam/amsterdam/eip7928_block_level_access_lists/block_access_lists/bal_empty_block_no_coinbase.json";
    if (b.lazyDependency("execution_spec_tests_zkevm", .{})) |fixtures_dep| {
        const fixtures_mod = b.createModule(.{
            .root_source_file = b.path("test/evm_execution_fixtures.zig"),
            .target = native_target,
            .optimize = host_optimize,
        });
        // Embed the chosen fixture straight from the dependency tree (no committed copy).
        fixtures_mod.addAnonymousImport("zkevm_stateless_block.json", .{
            .root_source_file = fixtures_dep.path(fixture_rel),
        });

        const tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/evm_execution_guest_test.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        tests.root_module.addImport("evm_execution_guest", guest_mod);
        tests.root_module.addImport("evm_execution_fixtures", fixtures_mod);
        linkNativeZesuCrypto(tests, native_target, native_crypto);

        test_step.dependOn(&b.addRunArtifact(tests).step);

        // ── Spec-test runner ────────────────────────────────────────────────────
        // Standalone host executable that walks the WHOLE zkevm fixture tree and runs every block
        // through this guest (mirrors zesu's zkevm-blockchain-test-runner). Fixtures come from the
        // same lazy dependency — no curl, no embedding; `zig build spec-tests` passes the
        // blockchain_tests/ directory as --fixtures. Pass-through extra args after `--`, e.g.
        // `zig build spec-tests -- --fork Amsterdam -x`.
        const spec_runner_exe = b.addExecutable(.{
            .name = "evm-execution-spec-runner",
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/evm_spec_runner.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        spec_runner_exe.root_module.addImport("evm_execution_guest", guest_mod);
        linkNativeZesuCrypto(spec_runner_exe, native_target, native_crypto);

        const run_spec = b.addRunArtifact(spec_runner_exe);
        run_spec.addArg("--fixtures");
        run_spec.addDirectoryArg(fixtures_dep.path("blockchain_tests"));
        if (b.args) |extra| run_spec.addArgs(extra);
        spec_step.dependOn(&run_spec.step);
    }
}

// ── zesu / EVM-execution wiring (l2-execution-specific) ───────────────────────────────────────────
// These are NOT in build_common: only guests that run the EVM via zesu need them. The rollup and
// rollup-aggregation guests (KZG/compression + recursive proof verification) do not.

const ZesuImports = struct {
    allocator: *std.Build.Module,
    executor: *std.Build.Module,
    ssz_decode: *std.Build.Module,
    ssz_output: *std.Build.Module,
};

/// Pull zesu's exposed modules by name. Which crypto backend zesu uses is selected inside zesu by
/// target (freestanding leaves zkvm_* extern; native links real crypto) — that's zesu's concern.
fn zesuImports(zesu: *std.Build.Dependency) ZesuImports {
    return .{
        .allocator = zesu.module("zesu_allocator"),
        .executor = zesu.module("executor"),
        .ssz_decode = zesu.module("ssz_decode"),
        .ssz_output = zesu.module("ssz_output"),
    };
}

fn addExecutionImports(module: *std.Build.Module, imports: ZesuImports) void {
    module.addImport("zesu_allocator", imports.allocator);
    module.addImport("zesu_executor", imports.executor);
    module.addImport("zesu_ssz_decode", imports.ssz_decode);
    module.addImport("zesu_ssz_output", imports.ssz_output);
}

const NativeCrypto = struct {
    include_path: []const u8,
    lib_path: []const u8,
    blst_path: []const u8,
    mcl_path: []const u8,
    is_linux: bool,
};

fn resolveNativeCrypto(b: *std.Build, target: std.Build.ResolvedTarget) NativeCrypto {
    _ = target;
    const default_prefix = if (b.graph.host.result.os.tag == .linux) "/usr/local" else "/opt/homebrew";
    const prefix = b.option([]const u8, "crypto-prefix", "Native crypto dependency prefix") orelse default_prefix;
    const lib_path = b.fmt("{s}/lib", .{prefix});
    return .{
        .include_path = b.fmt("{s}/include", .{prefix}),
        .lib_path = lib_path,
        .blst_path = b.fmt("{s}/libblst.a", .{lib_path}),
        .mcl_path = b.fmt("{s}/libmcl.a", .{lib_path}),
        .is_linux = b.graph.host.result.os.tag == .linux,
    };
}

/// Links the full native crypto backing zesu's native accelerator: secp256k1 (ecrecover), OpenSSL
/// (P-256), blst (BLS12-381 + KZG), mcl (BN254). No-op for freestanding targets, whose crypto is the
/// in-guest zkvm_* symbols. zesu sets the C include path itself, so here we only add the library
/// search path + the libraries.
fn linkNativeZesuCrypto(
    step: *std.Build.Step.Compile,
    target: std.Build.ResolvedTarget,
    crypto: NativeCrypto,
) void {
    if (target.result.os.tag == .freestanding) return;

    addCompileIncludePath(step, .{ .cwd_relative = crypto.include_path });
    addCompileLibraryPath(step, .{ .cwd_relative = crypto.lib_path });

    linkCompileSystemLibrary(step, "c");
    if (target.result.os.tag != .windows) {
        linkCompileSystemLibrary(step, "m");
    }
    linkCompileSystemLibrary(step, "secp256k1");
    linkCompileSystemLibrary(step, "ssl");
    linkCompileSystemLibrary(step, "crypto");
    step.root_module.addObjectFile(.{ .cwd_relative = crypto.blst_path });
    if (crypto.is_linux) {
        linkCompileSystemLibrary(step, "mcl");
    } else {
        step.root_module.addObjectFile(.{ .cwd_relative = crypto.mcl_path });
        step.root_module.link_libcpp = true;
    }
}

fn addCompileIncludePath(step: *std.Build.Step.Compile, path: std.Build.LazyPath) void {
    if (@hasDecl(std.Build.Step.Compile, "addIncludePath")) {
        step.addIncludePath(path);
    } else {
        step.root_module.addIncludePath(path);
    }
}

fn addCompileLibraryPath(step: *std.Build.Step.Compile, path: std.Build.LazyPath) void {
    if (@hasDecl(std.Build.Step.Compile, "addLibraryPath")) {
        step.addLibraryPath(path);
    } else {
        step.root_module.addLibraryPath(path);
    }
}

fn linkCompileSystemLibrary(step: *std.Build.Step.Compile, name: []const u8) void {
    if (@hasDecl(std.Build.Step.Compile, "linkSystemLibrary")) {
        step.linkSystemLibrary(name);
    } else {
        step.root_module.linkSystemLibrary(name, .{});
    }
}
