//! Builds the `lineth_da` package module (data-availability building blocks for the guests).
//! `zig build` links the freestanding rv64im smoke guest, which proves the package module and the C
//! library behind its `zstd` namespace build and link without a C runtime; `zig build test` runs
//! the native unit tests.

const std = @import("std");
const common = @import("build_common");

/// libzstd sources of the decoder: dictionary-based decompression and everything it links against.
const decoder_sources = [_][]const u8{
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
};

/// One section per function and per datum, so the linker keeps only what the guest calls; the
/// portable decoder loops, so no assembly source has to be linked on x86-64.
const decoder_flags = [_][]const u8{ "-ffunction-sections", "-fdata-sections", "-DZSTD_DISABLE_ASM" };

/// Upstream's decompression regression frames, embedded in the zstd test root under their import
/// names: the first four must decode, the last two must be rejected.
const golden_frames = [_]struct { import_name: []const u8, path: []const u8 }{
    .{ .import_name = "golden_block_128k", .path = "tests/golden-decompression/block-128k.zst" },
    .{ .import_name = "golden_empty_block", .path = "tests/golden-decompression/empty-block.zst" },
    .{ .import_name = "golden_rle_first_block", .path = "tests/golden-decompression/rle-first-block.zst" },
    .{ .import_name = "golden_zero_seq_2b", .path = "tests/golden-decompression/zeroSeq_2B.zst" },
    .{ .import_name = "golden_error_off0", .path = "tests/golden-decompression-errors/off0.bin.zst" },
    .{ .import_name = "golden_error_zero_seq_extraneous", .path = "tests/golden-decompression-errors/zeroSeq_extraneous.zst" },
};

/// libzstd sources of the single-threaded compressor, linked into native tests and tools only.
const compressor_sources = [_][]const u8{
    "compress/fse_compress.c",
    "compress/hist.c",
    "compress/huf_compress.c",
    "compress/zstd_compress.c",
    "compress/zstd_compress_literals.c",
    "compress/zstd_compress_sequences.c",
    "compress/zstd_compress_superblock.c",
    "compress/zstd_double_fast.c",
    "compress/zstd_fast.c",
    "compress/zstd_lazy.c",
    "compress/zstd_ldm.c",
    "compress/zstd_opt.c",
};

pub fn build(b: *std.Build) void {
    common.requireZigVersion();

    // Same freestanding rv64im ZkC profile as every guest (shared helper); a consumer that passes
    // its own target via b.dependency overrides this default.
    const target = common.standardGuestTarget(b);
    // Use b.option directly (not standardOptimizeOption) so the `-Doptimize` enum option stays
    // exposed — consumers set it through `b.dependency(..., .{ .optimize = ... })` — while still
    // defaulting to ReleaseSmall.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    const zstd_source = b.dependency("zstd", .{});

    // ── Package module for consumers and the smoke guest ─────────────────────────────────────
    const package = b.addModule("lineth_da", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    addPackageSources(b, package, zstd_source);
    if (target.result.os.tag == .freestanding) addSmokeGuest(b, package, target, optimize);

    // ── Native tests and tools ───────────────────────────────────────────────────────────────
    // Like every other `b.addTest` artifact in this tree, these run on the native host: a
    // freestanding riscv64 target has no OS to run a `std.testing` binary against.
    const native_target = b.resolveTargetQuery(.{});
    const package_native = common.createPackageModule(b, native_target, .Debug, &.{});
    addPackageSources(b, package_native, zstd_source);

    const compressor = b.createModule(.{
        .root_source_file = b.path("test/zstd_compressor.zig"),
        .target = native_target,
        .optimize = .Debug,
    });
    compressor.addCSourceFiles(.{ .root = zstd_source.path("lib"), .files = &compressor_sources });

    const native: common.NativeOptions = .{
        .target = native_target,
        .imports = &.{
            .{ .name = "lineth_da", .module = package_native },
            .{ .name = "zstd_compressor", .module = compressor },
        },
    };
    const test_step = b.step("test", "Run native Zig unit tests for the lineth_da package");
    const tests = common.addTests(b, test_step, native, .{});
    for (golden_frames) |frame| {
        tests.get("test/zstd_test.zig").?.root_module.addAnonymousImport(frame.import_name, .{
            .root_source_file = zstd_source.path(frame.path),
        });
    }

    const tools = common.addTools(b, native, .{});
    var tool_iter = tools.valueIterator();
    while (tool_iter.next()) |tool| b.installArtifact(tool.*);
}

/// The statically-linked rv64im smoke guest: the package module and libzstd linked without a C runtime.
fn addSmokeGuest(
    b: *std.Build,
    package: *std.Build.Module,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
) void {
    const accelerators = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const smoke = b.createModule(.{
        .root_source_file = b.path("test/smoke/zstd_smoke.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "lineth_accelerators", .module = accelerators.module("lineth_accelerators") },
        },
    });
    common.clearFreestandingNativeLinkage(b, smoke);
    common.installGuestElf(b, smoke, "zstd_smoke");
}

/// Wires the package module's embedded dictionary and the libzstd decoder behind its `zstd`
/// namespace. The freestanding target compiles the C library against the headers in
/// `src/freestanding_c`; every other target uses its libc.
fn addPackageSources(b: *std.Build, package: *std.Build.Module, zstd_source: *std.Build.Dependency) void {
    package.addAnonymousImport("zstd_dictionary_v2025_04_21", .{
        .root_source_file = b.path("dictionaries/v2025-04-21.bin"),
    });
    if (package.resolved_target.?.result.os.tag == .freestanding) {
        package.addIncludePath(b.path("src/freestanding_c"));
    } else {
        package.link_libc = true;
    }
    package.addCSourceFiles(.{ .root = zstd_source.path("lib"), .files = &decoder_sources, .flags = &decoder_flags });
}
