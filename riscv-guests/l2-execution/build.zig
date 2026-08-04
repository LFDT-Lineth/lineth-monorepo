const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // All guests target the same freestanding rv64im ZkC profile (shared helper).
    const target = common.standardGuestTarget(b);

    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed — consumers of the exposed zkvm_provide module set it through `b.dependency(..., .{ .optimize = ... })`
    // — while still defaulting to ReleaseSmall. (standardOptimizeOption with preferred_optimize_mode would swap
    //`-Doptimize` for `-Drelease`, breaking the dependency pass-through.)
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    // Keccak provider: standard zig keccak (zesu stdlibs_accel) by default; the
    // arithmetization keccak wrapper (prover-accelerated custom op) when opted in
    // with -Dkeccak-accel=true. Read by zkvm_provide.zig at comptime.
    const keccak_accel = b.option(bool, "keccak-accel", "Use the arithmetization keccak wrapper instead of standard zig keccak (default: standard)") orelse false;
    const execution_specs_fixtures_link = b.option([]const u8, "execution-specs-fixtures-link", "Path where execution-specs zkevm fixtures are exposed") orelse "/tmp/execution-specs-json-fixtures/fixtures";
    const guest_options = b.addOptions();
    guest_options.addOption(bool, "keccak_accel", keccak_accel);

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
    //   • zesu_zkvm_stdlibs — zesu-zkvm's stdlibs_accel: in-guest software precompiles that
    //     zkvm_provide.zig exports as the zkvm_* symbols zesu references;
    //   • zesu_crypto_backend — zesu's own native crypto backend (the handful of its precompiles
    //     with no C-library dependency: modexp, RIPEMD-160), standing in for the two of those
    //     zesu_zkvm_stdlibs leaves as unconditional-failure stubs;
    //   • lineth_zkvm_accel — Lineth accelerator wrappers (keccak today): the only actually
    //     prover-accelerated (custom opcode / circuit) source in this file — zkvm_* the prover
    //     accelerates at execution rather than at link time, so the ELF stays fully resolved;
    //   • linea_zkvm_io — zesu-zkvm's zkvm_io: satisfies the standards `read_input` by reading the
    //     memory-mapped `_in_start` (the input slot is the proving system's detail, kept out of the
    //     guest; `_in_start` is supplied by the linker script).
    const zesu_guest = b.dependency("zesu", .{ .target = target, .optimize = optimize });
    const zesu_zkvm = b.dependency("zesu_zkvm", .{});
    const zesu_zkvm_stdlibs_src = zesu_zkvm.path("linea/src/runtime/stdlibs_accel.zig"); // also imported by the native stdlibs test below
    const zesu_zkvm_stdlibs_mod = b.createModule(.{
        .root_source_file = zesu_zkvm_stdlibs_src,
        .target = target,
        .optimize = optimize,
    });
    const lineth_accel_mod = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize }).module("lineth_accelerators");

    const modexp_impl_mod = b.createModule(.{
        .root_source_file = zesu_guest.path("src/crypto/backends/modexp_impl.zig"),
        .target = target,
        .optimize = optimize,
    });
    modexp_impl_mod.addImport("zesu_allocator", zesu_guest.module("zesu_allocator"));
    const ripemd160_impl_mod = b.createModule(.{
        .root_source_file = zesu_guest.path("src/crypto/backends/ripemd160_impl.zig"),
        .target = target,
        .optimize = optimize,
    });
    const zesu_crypto_backend_mod = b.createModule(.{
        .root_source_file = b.path("src/zesu_crypto_backend.zig"),
        .target = target,
        .optimize = optimize,
    });
    zesu_crypto_backend_mod.addImport("zesu_modexp_impl", modexp_impl_mod);
    zesu_crypto_backend_mod.addImport("zesu_ripemd160_impl", ripemd160_impl_mod);

    // Expose the precompile providers (zkvm_provide.zig) as a standalone module so other packages
    // can link the SAME exported zkvm_* symbols this guest uses
    const provide_mod = b.addModule("zkvm_provide", .{
        .root_source_file = b.path("src/zkvm_provide.zig"),
        .target = target,
        .optimize = optimize,
    });
    provide_mod.addImport("zesu_zkvm_stdlibs", zesu_zkvm_stdlibs_mod);
    provide_mod.addImport("lineth_zkvm_accel", lineth_accel_mod);
    provide_mod.addImport("zesu_crypto_backend", zesu_crypto_backend_mod);
    provide_mod.addOptions("build_options", guest_options);

    const linea_io_mod = b.createModule(.{
        .root_source_file = zesu_zkvm.path("linea/src/zkvm_io.zig"),
        .target = target,
        .optimize = optimize,
    });

    // The extended wire format's SSZ codec, built for the SAME riscv64/optimize pair as the guest
    // itself (mirrors the native l2_execution_ssz_mod below, for the test host). `l2_execution.zig`
    // is pulled into `evm_execution_guest.zig` via a plain relative import, not a separate module:
    // a separate module would double-claim `execution.zig`, which both files import. The native
    // `guest_mod` used by the native test never needs this wiring — Zig's lazy analysis skips it
    // since `guestMain` (the only caller) isn't `@export`-ed for that target.
    const l2_execution_ssz_guest_mod = b.createModule(.{
        .root_source_file = b.path("src/l2_execution_ssz.zig"),
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
    guest_module.addImport("zesu_zkvm_stdlibs", zesu_zkvm_stdlibs_mod);
    guest_module.addImport("lineth_zkvm_accel", lineth_accel_mod);
    guest_module.addImport("zesu_crypto_backend", zesu_crypto_backend_mod);
    guest_module.addImport("linea_zkvm_io", linea_io_mod);
    guest_module.addImport("l2_execution_ssz", l2_execution_ssz_guest_mod);
    guest_module.addOptions("build_options", guest_options); // keccak_accel flag, read in zkvm_provide.zig
    common.clearFreestandingNativeLinkage(b, guest_module);
    common.installGuestElf(b, guest_module, gp_name);

    // ── Native test ───────────────────────────────────────────────────────────
    // Runs `execution.executeStatelessInputWithLogs` (the log-preserving seam `l2_execution.zig`
    // delegates per-block execution to) against a real execution-spec-tests zkevm SSZ fixture on
    // the host, asserting it computes the SAME pre/post/receipts roots as zesu's own vanilla
    // `executor.executeStatelessInput` — i.e. adding the log-preserving path doesn't change
    // validation outcomes. Links zesu's full native crypto backend; linea adds the library search
    // path so it links on macOS. The committed fixture is an empty block (only keccak), but the
    // full backend is linked so the suite can grow to tx-bearing fixtures (ecrecover/curves)
    // without further build changes.
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
    const extended_vanilla_step = b.step("extended-vanilla", "Reference-test guard: assert the dummy-wrapped extended guest (runL2Execution) agrees with the EF fixture's own expected validity over EF zkevm fixtures");
    const prep_fixtures_step = b.step("prep-execution-specs-json-fixtures", "Expose EF zkevm stateless fixtures for external runners");

    // Integration smoke test for the delegated precompiles: verifies zesu-zkvm's stdlibs_accel
    // imports and that its ecrecover round-trips (the in-guest precompiles delegate to it). std +
    // the dependency only — no fixtures, no native crypto libs.
    const stdlibs_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/stdlibs_accel_test.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    stdlibs_tests.root_module.addImport("zesu_zkvm_stdlibs", b.createModule(.{
        .root_source_file = zesu_zkvm_stdlibs_src,
        .target = native_target,
        .optimize = host_optimize,
    }));
    test_step.dependOn(&b.addRunArtifact(stdlibs_tests).step);

    const l2_execution_ssz_mod = b.createModule(.{
        .root_source_file = b.path("src/l2_execution_ssz.zig"),
        .target = native_target,
        .optimize = host_optimize,
    });
    const l2_execution_ssz_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_ssz_test.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    l2_execution_ssz_tests.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
    test_step.dependOn(&b.addRunArtifact(l2_execution_ssz_tests).step);

    // ── l2-execution guest logic (src/l2_execution.zig) unit tests ──────────────────────────────────
    // These `zig build test` UNIT TESTS run only on the native target — like the l2_execution_ssz
    // codec tests above, and like every other `b.addTest` artifact in this file. That's a standard
    // Zig constraint, not specific to this module: a freestanding riscv64 target has no OS to run a
    // `std.testing` binary against, so test binaries are always compiled and run for the native host.
    // The `l2_execution.zig` LOGIC ITSELF is not native-only — it's compiled into the riscv64 guest
    // ELF too, reached via `evm_execution_guest.zig`'s relative import inside `guestMain` (see the
    // module-wiring comment near `l2_execution_ssz_guest_mod` above).
    //
    // These tests exercise the Linea-layer logic — the FTX rolling hash, dynamicChainConfigHash,
    // hashAddressList/hashDigestList, the L1->L2 bridge storage-slot math, L2->L1 message
    // extraction, forced-transaction dispatch, and the witness-backed MPT account/storage reads —
    // against hand-built fixtures and Python-computed expected values (see Readme.md §6.3/§6.5/§2.1).
    // Needs the full zesu import set (MPT, executor types/tx-decode, primitives, accelerators for
    // ecrecover) plus the sibling `l2_execution_ssz` module.
    const l2_execution_mod = b.createModule(.{
        .root_source_file = b.path("src/l2_execution.zig"),
        .target = native_target,
        .optimize = host_optimize,
    });
    addExecutionImports(l2_execution_mod, native_imports);
    l2_execution_mod.addImport("l2_execution_ssz", l2_execution_ssz_mod);

    const l2_execution_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_test.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    l2_execution_tests.root_module.addImport("l2_execution", l2_execution_mod);
    l2_execution_tests.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
    l2_execution_tests.root_module.addImport("zesu_executor", native_imports.executor);
    l2_execution_tests.root_module.addImport("zesu_mpt", native_imports.mpt);
    l2_execution_tests.root_module.addImport("zesu_input", native_imports.input);
    l2_execution_tests.root_module.addImport("zesu_primitives", native_imports.primitives);
    l2_execution_tests.root_module.addImport("zesu_allocator", native_imports.allocator);
    l2_execution_tests.root_module.addImport("zesu_accelerators", native_imports.accelerators);
    linkNativeZesuCrypto(l2_execution_tests, native_target, native_crypto);
    test_step.dependOn(&b.addRunArtifact(l2_execution_tests).step);

    // ── l2-execution JSON output shape (test/l2_execution_json.zig) ─────────────────────────────────
    // Native-only, pure std + the sibling `l2_execution_ssz` module (no zesu dependency): asserts
    // `encodeOutputJson`'s field names/order/hex format agree byte-for-byte with the Python
    // reference codec's `proof_io_v1.encode_response`, using the same golden values as the
    // committed `getZkL2ExecutionProofV1.response.json` fixture. Lives in `test/`, not `src/`: the
    // guest ELF always emits SSZ (see `evm_execution_guest.zig`'s doc comment), so this codec is
    // reachable only from native host tooling (`l2-execution-runner --json` and the test below).
    const l2_execution_json_mod = b.createModule(.{
        .root_source_file = b.path("test/l2_execution_json.zig"),
        .target = native_target,
        .optimize = host_optimize,
    });
    l2_execution_json_mod.addImport("l2_execution_ssz", l2_execution_ssz_mod);

    const l2_execution_json_tests = b.addTest(.{
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_json_test.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    l2_execution_json_tests.root_module.addImport("l2_execution_json", l2_execution_json_mod);
    l2_execution_json_tests.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
    test_step.dependOn(&b.addRunArtifact(l2_execution_json_tests).step);

    // ── Vanilla-input dummy-fill wrap (test/vanilla_wrap.zig) ───────────────────────────────────────
    // Wraps a vanilla EF `SszStatelessInput` into an extended `L2ExecutionProofPrivateInput` with
    // dummy rollup fields, so the extended guest can run against the same EF corpus the vanilla guest
    // runs on. Needs `zesu_ssz_decode` (to read the vanilla input's chain_id/fee_recipient) and the
    // sibling `l2_execution_ssz` module (to build + encode the wrapper). Lives in `test/`, not `src/`:
    // it's never reachable from the guest ELF's compile graph, only from the two native host
    // consumers below (`l2-execution-wrap` and `extended-vanilla-runner`).
    const vanilla_wrap_mod = b.createModule(.{
        .root_source_file = b.path("test/vanilla_wrap.zig"),
        .target = native_target,
        .optimize = host_optimize,
    });
    vanilla_wrap_mod.addImport("zesu_ssz_decode", native_imports.ssz_decode);
    vanilla_wrap_mod.addImport("l2_execution_ssz", l2_execution_ssz_mod);

    // ── Vanilla StatelessInput SSZ encoder module (test/stateless_input_encode.zig) ─────────────────
    // Test-only SSZ encoder for zesu's vanilla StatelessInput — the byte-level inverse of
    // zesu_ssz_decode's decode, which ships with no matching encoder of its own. Wired as a shared
    // named module (not a bare relative import) since two independent test roots use it: its own
    // round-trip/golden tests below, and the conflation-plan DSL, which needs it to produce each
    // fabricated payload's stateless_input_ssz bytes. Mirrors how `vanilla_wrap_mod` above is shared
    // across two consumers.
    const stateless_input_encode_mod = b.createModule(.{
        .root_source_file = b.path("test/stateless_input_encode.zig"),
        .target = native_target,
        .optimize = host_optimize,
    });
    stateless_input_encode_mod.addImport("zesu_input", native_imports.input);

    // ── `l2-execution-wrap` native host tool ────────────────────────────────────────────────────────
    // Wraps a vanilla EF stateless-input .ssz into an extended L2ExecutionProofPrivateInput .ssz
    // (zero l2MessageServiceAddress -> bridge suppression), so the ZkC harness can feed the extended
    // guest the same EF corpus the vanilla guest ran on. Installed to zig-out/bin so `make compile`
    // (plain `zig build`) produces it alongside the guest ELF; the Go harness invokes it per input.
    const l2_execution_wrap_exe = b.addExecutable(.{
        .name = "l2-execution-wrap",
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_wrap.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    l2_execution_wrap_exe.root_module.addImport("vanilla_wrap", vanilla_wrap_mod);
    linkNativeZesuCrypto(l2_execution_wrap_exe, native_target, native_crypto);
    b.installArtifact(l2_execution_wrap_exe);

    const run_l2_execution_wrap_step = b.step(
        "l2-execution-wrap",
        "Run the native l2-execution-wrap tool (vanilla .ssz -> extended .ssz)",
    );
    const run_l2_execution_wrap = b.addRunArtifact(l2_execution_wrap_exe);
    if (b.args) |extra| run_l2_execution_wrap.addArgs(extra);
    run_l2_execution_wrap_step.dependOn(&run_l2_execution_wrap.step);

    // ── `l2-execution-runner` native host tool ──────────────────────────────────────────────────────
    // Standalone host executable: SSZ extended-input file in, SSZ (default) or JSON (`--json`)
    // output on stdout. See test/l2_execution_runner.zig's doc comment for why the SSZ/JSON toggle
    // lives here rather than in the freestanding guest. Not gated behind the lazy
    // execution-spec-tests fixtures below — it takes an arbitrary input file, not the EF corpus.
    const l2_execution_runner_exe = b.addExecutable(.{
        .name = "l2-execution-runner",
        .root_module = b.createModule(.{
            .root_source_file = b.path("test/l2_execution_runner.zig"),
            .target = native_target,
            .optimize = host_optimize,
        }),
    });
    l2_execution_runner_exe.root_module.addImport("l2_execution", l2_execution_mod);
    l2_execution_runner_exe.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
    l2_execution_runner_exe.root_module.addImport("l2_execution_json", l2_execution_json_mod);
    linkNativeZesuCrypto(l2_execution_runner_exe, native_target, native_crypto);
    b.installArtifact(l2_execution_runner_exe);

    const run_l2_execution_runner_step = b.step(
        "l2-execution-runner",
        "Run the native l2-execution-runner (SSZ extended-input file -> SSZ/JSON output on stdout)",
    );
    const run_l2_execution_runner = b.addRunArtifact(l2_execution_runner_exe);
    if (b.args) |extra| run_l2_execution_runner.addArgs(extra);
    run_l2_execution_runner_step.dependOn(&run_l2_execution_runner.step);

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
        // Direct zesu imports so the test can decode the SSZ fixture and run zesu's vanilla
        // executeStatelessInput itself, to assert executeStatelessInputWithLogs (src/execution.zig)
        // computes the SAME roots on the same input.
        tests.root_module.addImport("zesu_executor", native_imports.executor);
        tests.root_module.addImport("zesu_ssz_decode", native_imports.ssz_decode);
        tests.root_module.addImport("zesu_allocator", native_imports.allocator);
        tests.root_module.addImport("zesu_mpt", native_imports.mpt);
        linkNativeZesuCrypto(tests, native_target, native_crypto);

        test_step.dependOn(&b.addRunArtifact(tests).step);

        // ── Vanilla StatelessInput SSZ encoder (test/stateless_input_encode.zig) unit tests ────────
        // Test-only SSZ encoder for zesu's vanilla StatelessInput — the byte-level inverse of
        // zesu_ssz_decode's decode, which ships with no matching encoder of its own. A round-trip
        // test builds a StatelessInput as readable Zig and checks it survives encode-then-decode; a
        // golden test decodes this same fixture and checks re-encoding it reproduces the original
        // bytes exactly. Needs the same zesu_input/zesu_ssz_decode imports as the vanilla-wrap
        // wiring, plus the fixtures module already built for the guest smoke test above.
        const stateless_input_encode_tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/stateless_input_encode_test.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        stateless_input_encode_tests.root_module.addImport("zesu_input", native_imports.input);
        stateless_input_encode_tests.root_module.addImport("zesu_ssz_decode", native_imports.ssz_decode);
        stateless_input_encode_tests.root_module.addImport("evm_execution_fixtures", fixtures_mod);
        stateless_input_encode_tests.root_module.addImport("stateless_input_encode", stateless_input_encode_mod);
        linkNativeZesuCrypto(stateless_input_encode_tests, native_target, native_crypto);
        test_step.dependOn(&b.addRunArtifact(stateless_input_encode_tests).step);

        // ── Conflation-plan test DSL parity guard (test/conflation_plan_parity_test.zig) ────────────
        // Proves `conflation_plan.zig`'s `StubEngine` (the l2-execution guest's execution-seam stub)
        // is a faithful stand-in for the real per-block execution seam on real data, and smoke-tests
        // the `ConflationPlan` DSL itself end to end through the guest's real conflation logic.
        // `conflation_plan.zig` is pulled in by relative import from the test file, so every import
        // it needs (the full zesu set its real-MPT/real-header derivation uses, `l2_execution` for
        // the seam, `l2_execution_ssz` for the envelope codec, and the shared
        // `stateless_input_encode` module) is wired directly on this root module.
        const conflation_plan_parity_tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/conflation_plan_parity_test.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        conflation_plan_parity_tests.root_module.addImport("l2_execution", l2_execution_mod);
        conflation_plan_parity_tests.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
        conflation_plan_parity_tests.root_module.addImport("zesu_executor", native_imports.executor);
        conflation_plan_parity_tests.root_module.addImport("zesu_mpt", native_imports.mpt);
        conflation_plan_parity_tests.root_module.addImport("zesu_input", native_imports.input);
        conflation_plan_parity_tests.root_module.addImport("zesu_primitives", native_imports.primitives);
        conflation_plan_parity_tests.root_module.addImport("zesu_allocator", native_imports.allocator);
        conflation_plan_parity_tests.root_module.addImport("zesu_rlp_decode", native_imports.rlp_decode);
        conflation_plan_parity_tests.root_module.addImport("zesu_ssz_decode", native_imports.ssz_decode);
        conflation_plan_parity_tests.root_module.addImport("stateless_input_encode", stateless_input_encode_mod);
        conflation_plan_parity_tests.root_module.addImport("evm_execution_fixtures", fixtures_mod);
        linkNativeZesuCrypto(conflation_plan_parity_tests, native_target, native_crypto);
        test_step.dependOn(&b.addRunArtifact(conflation_plan_parity_tests).step);

        // ── Conflation-plan range scenario suite (test/l2_execution_range_test.zig) ───────────────
        // One rich happy-path scenario asserting the full 16-field public input plus preimages,
        // and twelve one-mutation rejection scenarios, both built on the `ConflationPlan` DSL —
        // driving the guest's real conflation logic (`l2_execution.runL2ExecutionWithEngine`)
        // through `StubEngine` instead of live EVM execution. Needs exactly the import set
        // `conflation_plan.zig` itself needs, since it's pulled in by relative import from the
        // test root, same as the parity test above.
        const l2_execution_range_tests = b.addTest(.{
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/l2_execution_range_test.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        l2_execution_range_tests.root_module.addImport("l2_execution", l2_execution_mod);
        l2_execution_range_tests.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
        l2_execution_range_tests.root_module.addImport("zesu_executor", native_imports.executor);
        l2_execution_range_tests.root_module.addImport("zesu_mpt", native_imports.mpt);
        l2_execution_range_tests.root_module.addImport("zesu_input", native_imports.input);
        l2_execution_range_tests.root_module.addImport("zesu_primitives", native_imports.primitives);
        l2_execution_range_tests.root_module.addImport("zesu_rlp_decode", native_imports.rlp_decode);
        l2_execution_range_tests.root_module.addImport("stateless_input_encode", stateless_input_encode_mod);
        linkNativeZesuCrypto(l2_execution_range_tests, native_target, native_crypto);
        test_step.dependOn(&b.addRunArtifact(l2_execution_range_tests).step);

        // ── extended-vs-fixture validity reference-test guard (permanent) ──
        // The single reference-test runner for the extended guest: wraps the vanilla EF input into a
        // dummy-filled extended input (vanilla_wrap.wrapVanillaAsExtended, single payload, empty
        // FTX, zero l2_message_service_address) and asserts l2_execution.runL2Execution — the
        // extended guest's actual Linea-layer logic, delegating per-block execution to
        // `execution.executeStatelessInputWithLogs` — agrees with the EF fixture's OWN expected
        // validity verdict (not a second, independently re-run implementation — see
        // extended_vanilla_runner.zig's header comment for why).
        // Once wrapped, every other Linea-layer check (conflation invariants, FTX dispatch, bridge
        // reads, message extraction) is either trivially satisfied or suppressed, so this single
        // comparison already exercises the delegated execution seam (including the hand-copied
        // header-chain preamble and EIP-7928 BAL path) end-to-end — a separate seam-only runner
        // would be redundant with it. Pass-through extra args after `--`, e.g.
        // `zig build extended-vanilla -- --fork Amsterdam --limit 300`.
        const extended_vanilla_runner_exe = b.addExecutable(.{
            .name = "extended-vanilla-runner",
            .root_module = b.createModule(.{
                .root_source_file = b.path("test/extended_vanilla_runner.zig"),
                .target = native_target,
                .optimize = host_optimize,
            }),
        });
        // NOT `evm_execution_guest` (guest_mod): that module relative-imports `l2_execution.zig`
        // inside `guestMain`, so combining it with `l2_execution_mod` as a second, separately-rooted
        // module in the same compile unit is a Zig module-graph conflict ("file exists in modules
        // 'l2_execution' and 'evm_execution_guest'") — the same constraint documented above for
        // `l2_execution_ssz_guest_mod`.
        extended_vanilla_runner_exe.root_module.addImport("l2_execution", l2_execution_mod);
        extended_vanilla_runner_exe.root_module.addImport("l2_execution_ssz", l2_execution_ssz_mod);
        extended_vanilla_runner_exe.root_module.addImport("vanilla_wrap", vanilla_wrap_mod);
        linkNativeZesuCrypto(extended_vanilla_runner_exe, native_target, native_crypto);

        const run_extended_vanilla = b.addRunArtifact(extended_vanilla_runner_exe);
        run_extended_vanilla.addArg("--fixtures");
        run_extended_vanilla.addDirectoryArg(fixtures_dep.path("blockchain_tests"));
        if (b.args) |extra| run_extended_vanilla.addArgs(extra);
        extended_vanilla_step.dependOn(&run_extended_vanilla.step);

        const fixtures_parent = std.fs.path.dirname(execution_specs_fixtures_link) orelse ".";
        const mkdir_fixtures_parent = b.addSystemCommand(&.{ "mkdir", "-p", fixtures_parent });
        const remove_old_fixtures_link = b.addSystemCommand(&.{ "rm", "-rf", execution_specs_fixtures_link });
        remove_old_fixtures_link.step.dependOn(&mkdir_fixtures_parent.step);

        const link_fixtures = b.addSystemCommand(&.{ "ln", "-s" });
        link_fixtures.addDirectoryArg(fixtures_dep.path("."));
        link_fixtures.addArg(execution_specs_fixtures_link);
        link_fixtures.step.dependOn(&remove_old_fixtures_link.step);
        prep_fixtures_step.dependOn(&link_fixtures.step);
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
    // Log-preserving seam (src/execution.zig) additions: everything executor/main.zig's
    // executeStatelessInput/executeBlockStateless preamble touches that isn't already reachable
    // through the `executor` module's public re-exports. `primitives` isn't exposed by name in
    // zesu's build.zig comments but IS added via the same expose=true addModule() call as the rest
    // of this list — needed for the SpecId/isEnabledIn/KECCAK_EMPTY the copied BAL validation uses.
    primitives: *std.Build.Module,
    mpt: *std.Build.Module,
    db: *std.Build.Module,
    context: *std.Build.Module,
    input: *std.Build.Module,
    hardfork: *std.Build.Module,
    rlp_decode: *std.Build.Module,
    // The crypto-accelerator interface tx_signing.zig uses for ecrecover/keccak256. Not re-exported
    // by the `executor` module (tx_signing.zig is one of its private submodules), so
    // l2_execution.zig's own sender-recovery port imports it directly.
    accelerators: *std.Build.Module,
};

/// Pull zesu's exposed modules by name. Which crypto backend zesu uses is selected inside zesu by
/// target (freestanding leaves zkvm_* extern; native links real crypto) — that's zesu's concern.
fn zesuImports(zesu: *std.Build.Dependency) ZesuImports {
    return .{
        .allocator = zesu.module("zesu_allocator"),
        .executor = zesu.module("executor"),
        .ssz_decode = zesu.module("ssz_decode"),
        .ssz_output = zesu.module("ssz_output"),
        .primitives = zesu.module("primitives"),
        .mpt = zesu.module("mpt"),
        .db = zesu.module("db"),
        .context = zesu.module("context"),
        .input = zesu.module("input"),
        .hardfork = zesu.module("hardfork"),
        .rlp_decode = zesu.module("rlp_decode"),
        .accelerators = zesu.module("accelerators"),
    };
}

fn addExecutionImports(module: *std.Build.Module, imports: ZesuImports) void {
    module.addImport("zesu_allocator", imports.allocator);
    module.addImport("zesu_executor", imports.executor);
    module.addImport("zesu_ssz_decode", imports.ssz_decode);
    module.addImport("zesu_ssz_output", imports.ssz_output);
    module.addImport("zesu_primitives", imports.primitives);
    module.addImport("zesu_mpt", imports.mpt);
    module.addImport("zesu_db", imports.db);
    module.addImport("zesu_context", imports.context);
    module.addImport("zesu_input", imports.input);
    module.addImport("zesu_hardfork", imports.hardfork);
    module.addImport("zesu_rlp_decode", imports.rlp_decode);
    module.addImport("zesu_accelerators", imports.accelerators);
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
