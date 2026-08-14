const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
    const target = common.standardGuestTarget(b);
    const optimize: std.builtin.OptimizeMode = .ReleaseSmall;

    // Symbols are kept (strip = false) so PC-level profiles from the zkc trace
    // can be attributed to functions; see run.go's -pcprof flag. Symbol tables
    // do not enter guest memory, so cycle counts are unaffected.
    const strip = false;

    // zstd decode touches no crypto, so accelerators are irrelevant here; kept
    // off by default only to match bench_compress's convention of measuring
    // the plain path unless asked otherwise.
    const disable_accelerators = b.option(
        bool,
        "disable-accelerators",
        "Disable Lineth zkVM accelerator wrappers (unused by this benchmark)",
    ) orelse true;

    const accel_dep = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const accel_mod = accel_dep.module("lineth_accelerators");

    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("../../src/lib.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
    });
    // src/crypto/poseidon2.zig imports `lineth_accelerators` only when
    // r5_config.disable_accelerators is false, so the import must be supplied on
    // exactly the same condition or the module fails to resolve. See
    // bench_compress/build.zig for the full explanation (#3455).
    if (!disable_accelerators) {
        verifier_mod.addImport("lineth_accelerators", accel_mod);
    }

    const r5_options = b.addOptions();
    r5_options.addOption(bool, "is_r5_zkvm", true);
    r5_options.addOption(bool, "disable_accelerators", disable_accelerators);
    verifier_mod.addOptions("r5_config", r5_options);

    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", false);
    profiling_opts.addOption(bool, "is_r5_marks", true);
    verifier_mod.addOptions("profiling_config", profiling_opts);

    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = optimize,
        .strip = strip,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
            .{ .name = "lineth_accelerators", .module = accel_mod },
        },
    });

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s)
    // + rv64im memory layout + dead-section GC.
    // zstd's decode-only C sources, compiled for the same freestanding rv64im
    // target as the Zig guest. No libc is linked: vendor/shim supplies the
    // <string.h> and <assert.h> that zstd_deps.h includes, and a static DCtx
    // avoids malloc entirely.
    main_mod.addIncludePath(b.path("vendor/zstd"));
    main_mod.addIncludePath(b.path("vendor/zstd/common"));
    main_mod.addIncludePath(b.path("vendor/shim"));
    main_mod.addCSourceFiles(.{
        .root = b.path("vendor/zstd"),
        .files = &.{
            "common/debug.c",
            "common/entropy_common.c",
            "common/error_private.c",
            "common/fse_decompress.c",
            "common/xxhash.c",
            "common/zstd_common.c",
            "decompress/huf_decompress.c",
            "decompress/zstd_ddict.c",
            "decompress/zstd_decompress.c",
            "decompress/zstd_decompress_block.c",
        },
        .flags = &.{
            "-std=c99",
            "-DNDEBUG",
            "-DDEBUGLEVEL=0",
            "-DZSTD_LEGACY_SUPPORT=0",
            // The tuned Huffman decoder loop is amd64 assembly; the C fallback
            // is what a RISC-V target would use in any case.
            "-DZSTD_DISABLE_ASM",
            // Single-threaded: leaves pool.c/threading.c out of the build.
            "-DZSTD_TRACE=0",
        },
    });

    common.installGuestElf(b, main_mod, "bench-zstd-c");
}
