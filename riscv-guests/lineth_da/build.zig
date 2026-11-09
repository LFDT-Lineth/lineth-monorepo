//! Builds the `lineth_da` package module (data-availability building blocks for the guests).
//! `zig build test -Dhost-tools=true` runs the native unit tests, and `-Dzkevm-fixtures=<dir>` adds
//! the reference tests over the EF zkevm corpus (what `make test` runs).

const std = @import("std");
const common = @import("build_common");

const Import = std.Build.Module.Import;

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

/// libzstd sources of the single-threaded compressor, linked into the native `zstd_encoder` module only.
const encoder_sources = [_][]const u8{
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

    // All guests target the same freestanding rv64im profile.
    const target = common.standardGuestTarget(b);
    // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    // ── Package module for consumers ─────────────────────────────────────
    const zstd = Zstd.init(b, target, optimize);
    const package = b.addModule("lineth_da", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = packageImports(b, target, optimize, zstd),
    });
    addPackageSources(b, package, zstd);

    // Consumers of the package build the guest side only.
    const host_tools = b.option(bool, "host-tools", "Build the native tests: fetches the reference vectors and builds the host crypto providers") orelse false;
    const zkevm_fixtures = b.option(std.Build.LazyPath, "zkevm-fixtures", "Directory holding the EF zkevm fixtures; enables the reference tests over its blocks");
    if (host_tools) addHostBuild(b, zstd, target, optimize, zkevm_fixtures);
}

/// libzstd for one target: the release's sources, its C declarations translated for Zig, and the
/// memory hooks over a Zig allocator that the decoder and the encoder share.
const Zstd = struct {
    source: *std.Build.Dependency,
    declarations: *std.Build.Module,
    memory: *std.Build.Module,

    /// Translates `zstd.h`, with the declarations of its static-linking section, and `zstd_errors.h`.
    /// The memory hooks are compiled at `optimize`.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) Zstd {
        const source = b.dependency("zstd", .{});
        const translated = b.addTranslateC(.{
            .root_source_file = b.addWriteFiles().add("zstd_c.h", "#include <zstd.h>\n#include <zstd_errors.h>\n"),
            .target = target,
            .optimize = .Debug,
        });
        translated.addIncludePath(source.path("lib"));
        translated.defineCMacro("ZSTD_STATIC_LINKING_ONLY", null);
        const declarations = translated.createModule();
        const memory = b.createModule(.{
            .root_source_file = b.path("src/zstd_memory.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{.{ .name = "zstd_c", .module = declarations }},
        });
        return .{ .source = source, .declarations = declarations, .memory = memory };
    }
};

/// The zesu dependency for one target, with the extern crypto backend.
fn zesuFor(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) *std.Build.Dependency {
    return b.dependency("zesu", .{
        .target = target,
        .optimize = optimize,
        .@"crypto-backend" = .@"extern",
    });
}

/// What `src/` imports by name, for one target: the zesu modules under the `zesu_*` names the
/// sources use, the declarations of libzstd and the memory hooks of the decoder.
fn packageImports(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, zstd: Zstd) []const Import {
    const zesu = zesuFor(b, target, optimize);
    return b.allocator.dupe(Import, &.{
        .{ .name = "zesu_accelerators", .module = zesu.module("accelerators") },
        .{ .name = "zesu_executor", .module = zesu.module("executor") },
        .{ .name = "zesu_input", .module = zesu.module("input") },
        .{ .name = "zesu_mpt", .module = zesu.module("mpt") },
        .{ .name = "zesu_rlp_decode", .module = zesu.module("rlp_decode") },
        .{ .name = "zstd_c", .module = zstd.declarations },
        .{ .name = "zstd_memory", .module = zstd.memory },
    }) catch @panic("OOM");
}

/// The package module for a native target, with its sources attached.
fn hostPackage(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, zstd: Zstd) *std.Build.Module {
    const package = common.createPackageModule(b, target, optimize, packageImports(b, target, optimize, zstd));
    addPackageSources(b, package, zstd);
    return package;
}

/// The native tests run on the host: a freestanding riscv64 target has no OS to run a
/// `std.testing` binary against. The test-side pieces (`support`), the zstd encoder
/// (`zstd_encoder`) and the reader of the EF fixtures' blocks (`ef_blocks`) are modules of the
/// package for the tests of its consumers; they are compiled at the package's `optimize`, where
/// ReleaseSmall becomes ReleaseSafe. When the package itself is built for a native target, the
/// encoder shares its libzstd declarations and memory hooks, which one compilation can hold only
/// once.
fn addHostBuild(
    b: *std.Build,
    package_zstd: Zstd,
    package_target: std.Build.ResolvedTarget,
    package_optimize: std.builtin.OptimizeMode,
    zkevm_fixtures: ?std.Build.LazyPath,
) void {
    const target = b.resolveTargetQuery(.{});
    const optimize: std.builtin.OptimizeMode = if (package_optimize == .ReleaseSmall) .ReleaseSafe else package_optimize;
    const zstd = if (package_target.result.os.tag == .freestanding) Zstd.init(b, target, optimize) else package_zstd;
    const package = hostPackage(b, target, .Debug, zstd);

    const encoder = b.addModule("zstd_encoder", .{
        .root_source_file = b.path("src/zstd_encoder.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "zstd_c", .module = zstd.declarations },
            .{ .name = "zstd_memory", .module = zstd.memory },
        },
    });
    encoder.addCSourceFiles(.{ .root = zstd.source.path("lib"), .files = &encoder_sources });

    // Frames that break the rules of the DA frame form, for the tests of the decoder.
    const zstd_frames = b.createModule(.{
        .root_source_file = b.path("test/support/zstd_frames.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{.{ .name = "zstd_c", .module = zstd.declarations }},
    });

    const support = b.addModule("support", .{
        .root_source_file = b.path("test/support/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    const ef_blocks = b.addModule("ef_blocks", .{
        .root_source_file = b.path("test/support/ef_blocks.zig"),
        .target = target,
        .optimize = optimize,
    });

    const shared: SharedModules = .{
        .encoder = encoder,
        .zstd_frames = zstd_frames,
        .support = support,
        .ef_blocks = ef_blocks,
        .optimize = optimize,
    };
    const readers = shared.readersOf(b, target, package);

    const native: common.NativeOptions = .{
        .target = target,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "zstd_encoder", .module = encoder },
            .{ .name = "zstd_frames", .module = zstd_frames },
            .{ .name = "support", .module = support },
            .{ .name = "ef_conflations", .module = readers.ef_conflations },
            .{ .name = "da_chunks", .module = readers.da_chunks },
        },
    };
    const test_step = b.step("test", "Run native Zig unit tests for the lineth_da package");
    const tests = common.addTests(b, test_step, native, .{});
    addKzgReferences(b, target, tests);
    const reference_tests = addReferenceTests(b, test_step, target, zstd, shared, zkevm_fixtures);
    linkHostCrypto(b, target, &.{ tests, reference_tests });
}

/// The test-side modules that are independent of the package, shared by every instance of it.
const SharedModules = struct {
    encoder: *std.Build.Module,
    zstd_frames: *std.Build.Module,
    support: *std.Build.Module,
    ef_blocks: *std.Build.Module,
    optimize: std.builtin.OptimizeMode,

    /// The test-side modules that read `package`, which are one per instance of it: the conflations
    /// of real blocks, and the chunks of a DA stream.
    fn readersOf(self: SharedModules, b: *std.Build, target: std.Build.ResolvedTarget, package: *std.Build.Module) struct {
        ef_conflations: *std.Build.Module,
        da_chunks: *std.Build.Module,
    } {
        return .{
            .ef_conflations = b.createModule(.{
                .root_source_file = b.path("test/support/ef_conflations.zig"),
                .target = target,
                .optimize = self.optimize,
                .imports = &.{
                    .{ .name = "lineth_da", .module = package },
                    .{ .name = "ef_blocks", .module = self.ef_blocks },
                },
            }),
            .da_chunks = b.createModule(.{
                .root_source_file = b.path("test/support/da_chunks.zig"),
                .target = target,
                .optimize = self.optimize,
                .imports = &.{
                    .{ .name = "lineth_da", .module = package },
                    .{ .name = "support", .module = self.support },
                    .{ .name = "zstd_encoder", .module = self.encoder },
                },
            }),
        };
    }
};

/// The official EIP-4844 reference vectors for the blob tests, wired in once their lazy dependency
/// is fetched.
fn addKzgReferences(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    tests: std.StringHashMap(*std.Build.Step.Compile),
) void {
    const c_kzg = b.lazyDependency("c_kzg_4844", .{}) orelse return;
    const paths = b.addOptions();
    paths.addOptionPath("tests_dir", c_kzg.path("tests"));
    const reference_vectors = b.createModule(.{
        .root_source_file = b.path("test/support/reference_vectors.zig"),
        .target = target,
        .optimize = .Debug,
        .imports = &.{.{ .name = "c_kzg_paths", .module = paths.createModule() }},
    });
    tests.get("test/blob_kzg_test.zig").?.root_module.addImport("reference_vectors", reference_vectors);
}

/// Every native test links the host build of the zkvm_* providers.
/// Optimized: the compiler's own linker crashes on a debug-built provider object.
fn linkHostCrypto(b: *std.Build, target: std.Build.ResolvedTarget, artifacts: []const std.StringHashMap(*std.Build.Step.Compile)) void {
    const accelerators = b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = .ReleaseSafe,
        .@"zkvm-provide" = true,
    });
    const crypto = common.HostCrypto.init(b, accelerators) orelse return;
    for (artifacts) |group| crypto.linkAll(group);
}

/// The `test/reference/*_test.zig` suites, run by `test` when `-Dzkevm-fixtures` is given. They are
/// compiled optimized: the corpus is gigabytes of JSON.
fn addReferenceTests(
    b: *std.Build,
    test_step: *std.Build.Step,
    target: std.Build.ResolvedTarget,
    zstd: Zstd,
    shared: SharedModules,
    zkevm_fixtures: ?std.Build.LazyPath,
) std.StringHashMap(*std.Build.Step.Compile) {
    const none = std.StringHashMap(*std.Build.Step.Compile).init(b.allocator);
    const fixtures = zkevm_fixtures orelse {
        if (b.pkg_hash.len == 0) std.log.warn("test/reference skipped: pass -Dzkevm-fixtures=<dir> (`make test` does)", .{});
        return none;
    };

    const paths = b.addOptions();
    paths.addOptionPath("amsterdam_blocks", fixtures.path(b, "blockchain_tests/for_amsterdam"));
    const package = hostPackage(b, target, .ReleaseSafe, zstd);
    const readers = shared.readersOf(b, target, package);
    const zesu = zesuFor(b, target, .ReleaseSafe);
    return common.addTests(b, test_step, .{
        .target = target,
        .optimize = .ReleaseSafe,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "ef_fixtures", .module = paths.createModule() },
            .{ .name = "ef_blocks", .module = shared.ef_blocks },
            .{ .name = "zesu_executor", .module = zesu.module("executor") },
            .{ .name = "zesu_mpt", .module = zesu.module("mpt") },
            .{ .name = "zstd_encoder", .module = shared.encoder },
            .{ .name = "zstd_frames", .module = shared.zstd_frames },
            .{ .name = "support", .module = shared.support },
            .{ .name = "ef_conflations", .module = readers.ef_conflations },
            .{ .name = "da_chunks", .module = readers.da_chunks },
        },
    }, .{ .dir = "test/reference" });
}

/// Wires the package module's embedded dictionary and the libzstd decoder behind its `zstd_decoder`
/// namespace. The freestanding target compiles the C library against the headers in
/// `src/freestanding_c`; every other target uses its libc.
fn addPackageSources(b: *std.Build, package: *std.Build.Module, zstd: Zstd) void {
    package.addAnonymousImport("zstd_dictionary_v2025_04_21", .{
        .root_source_file = b.path("dictionaries/v2025-04-21.bin"),
    });
    if (package.resolved_target.?.result.os.tag == .freestanding) {
        package.addIncludePath(b.path("src/freestanding_c"));
    } else {
        package.link_libc = true;
    }
    package.addCSourceFiles(.{ .root = zstd.source.path("lib"), .files = &decoder_sources, .flags = &decoder_flags });
}
