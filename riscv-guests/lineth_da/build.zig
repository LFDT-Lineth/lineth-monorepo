//! Builds the `lineth_da` package module (data-availability building blocks for the guests).
//! `zig build` links a freestanding rv64im smoke guest (`-Dsmoke` selects it), which proves the
//! package module and the libraries behind it build and link on the guest target;
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

    // All guests target the same freestanding rv64im profile.
    const target = common.standardGuestTarget(b);
    // Keep `-Doptimize` exposed to dependency consumers and default to ReleaseSmall.
    const optimize = b.option(std.builtin.OptimizeMode, "optimize", "Optimization mode (default: ReleaseSmall)") orelse .ReleaseSmall;

    // ── Package module for consumers and the smoke guest ─────────────────────────────────────
    const zstd = Zstd.init(b, target);
    const package = b.addModule("lineth_da", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = packageImports(b, target, optimize, zstd),
    });
    addPackageSources(b, package, zstd);
    if (target.result.os.tag == .freestanding) addSmokeGuest(b, package, target, optimize);

    // Consumers of the package build the guest side only.
    const host_tools = b.option(bool, "host-tools", "Build the native tests and tools: fetches the reference vectors and builds the host crypto providers") orelse false;
    const zkevm_fixtures = b.option(std.Build.LazyPath, "zkevm-fixtures", "Directory holding the EF zkevm fixtures; enables the reference tests over its blocks");
    if (host_tools) addHostBuild(b, zkevm_fixtures);
}

/// libzstd for one target: the release's sources and its C declarations translated for Zig.
const Zstd = struct {
    source: *std.Build.Dependency,
    declarations: *std.Build.Module,

    /// Translates `zstd.h`, with the declarations of its static-linking section, and `zstd_errors.h`.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget) Zstd {
        const source = b.dependency("zstd", .{});
        const translated = b.addTranslateC(.{
            .root_source_file = b.addWriteFiles().add("zstd_c.h", "#include <zstd.h>\n#include <zstd_errors.h>\n"),
            .target = target,
            .optimize = .Debug,
        });
        translated.addIncludePath(source.path("lib"));
        translated.defineCMacro("ZSTD_STATIC_LINKING_ONLY", null);
        return .{ .source = source, .declarations = translated.createModule() };
    }
};

/// What `src/` imports by name, for one target: the zesu modules under the `zesu_*` names the
/// sources use, the scalar-field arithmetic and the declarations of libzstd.
fn packageImports(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, zstd: Zstd) []const Import {
    const zesu = b.dependency("zesu", .{
        .target = target,
        .optimize = optimize,
        .@"crypto-backend" = .@"extern",
    });
    return b.allocator.dupe(Import, &.{
        .{ .name = "zesu_accelerators", .module = zesu.module("accelerators") },
        .{ .name = "zesu_executor", .module = zesu.module("executor") },
        .{ .name = "zesu_input", .module = zesu.module("input") },
        .{ .name = "zesu_mpt", .module = zesu.module("mpt") },
        .{ .name = "zesu_rlp_decode", .module = zesu.module("rlp_decode") },
        .{ .name = "bls_scalar", .module = scalarModule(b, target, optimize) },
        .{ .name = "zstd_c", .module = zstd.declarations },
    }) catch @panic("OOM");
}

/// The scalar-field arithmetic, optimized for speed in every build, with safety checks in a debug
/// one: it is the inner loop of the blob evaluation.
fn scalarModule(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) *std.Build.Module {
    return b.createModule(.{
        .root_source_file = b.path("src/bls_scalar.zig"),
        .target = target,
        .optimize = if (optimize == .Debug) .ReleaseSafe else .ReleaseFast,
    });
}

/// The package module for a native target, with its sources attached.
fn hostPackage(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode, zstd: Zstd) *std.Build.Module {
    const package = common.createPackageModule(b, target, optimize, packageImports(b, target, optimize, zstd));
    addPackageSources(b, package, zstd);
    return package;
}

/// The native tests and tools run on the host: a freestanding riscv64 target has no OS to run a
/// `std.testing` binary against.
fn addHostBuild(b: *std.Build, zkevm_fixtures: ?std.Build.LazyPath) void {
    const target = b.resolveTargetQuery(.{});
    const zstd = Zstd.init(b, target);
    const package = hostPackage(b, target, .Debug, zstd);

    const compressor = b.createModule(.{
        .root_source_file = b.path("test/zstd_compressor.zig"),
        .target = target,
        .optimize = .Debug,
        .imports = &.{.{ .name = "zstd_c", .module = zstd.declarations }},
    });
    compressor.addCSourceFiles(.{ .root = zstd.source.path("lib"), .files = &compressor_sources });

    const support = b.createModule(.{
        .root_source_file = b.path("test/support/root.zig"),
        .target = target,
        .optimize = .Debug,
    });

    const conflation_sample = b.createModule(.{
        .root_source_file = b.path("test/conflation_sample.zig"),
        .target = target,
        .optimize = .Debug,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "support", .module = support },
        },
    });

    const native: common.NativeOptions = .{
        .target = target,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "zstd_compressor", .module = compressor },
            .{ .name = "support", .module = support },
            .{ .name = "conflation_sample", .module = conflation_sample },
        },
    };
    const test_step = b.step("test", "Run native Zig unit tests for the lineth_da package");
    const tests = common.addTests(b, test_step, native, .{});
    addGoldenFrames(tests, zstd.source);
    const tools = common.addTools(b, native, .{});
    addKzgReferences(b, target, tests, tools, support);
    const reference_tests = addReferenceTests(b, test_step, target, zstd, zkevm_fixtures);
    linkHostCrypto(b, target, &.{ tests, tools, reference_tests });

    var tool_iter = tools.valueIterator();
    while (tool_iter.next()) |tool| b.installArtifact(tool.*);
}

/// Upstream's decompression regression frames, embedded in the zstd test root.
fn addGoldenFrames(tests: std.StringHashMap(*std.Build.Step.Compile), zstd_source: *std.Build.Dependency) void {
    for (golden_frames) |frame| {
        tests.get("test/zstd_test.zig").?.root_module.addAnonymousImport(frame.import_name, .{
            .root_source_file = zstd_source.path(frame.path),
        });
    }
}

/// The official EIP-4844 reference vectors for the blob tests and the blob_sample tool, and, with
/// the c-kzg-4844 library over blst, the commitments and proofs of the test support. Each is wired
/// in once its lazy dependency is fetched.
fn addKzgReferences(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    tests: std.StringHashMap(*std.Build.Step.Compile),
    tools: std.StringHashMap(*std.Build.Step.Compile),
    support: *std.Build.Module,
) void {
    const c_kzg = b.lazyDependency("c_kzg_4844", .{}) orelse return;
    const paths = b.addOptions();
    paths.addOptionPath("tests_dir", c_kzg.path("tests"));
    paths.addOptionPath("trusted_setup", c_kzg.path("src/trusted_setup.txt"));
    const paths_module = paths.createModule();
    const reference_vectors = b.createModule(.{
        .root_source_file = b.path("test/reference_vectors.zig"),
        .target = target,
        .optimize = .Debug,
        .imports = &.{.{ .name = "c_kzg_paths", .module = paths_module }},
    });
    tests.get("test/blob_kzg_test.zig").?.root_module.addImport("reference_vectors", reference_vectors);
    tools.get("blob_sample").?.root_module.addImport("reference_vectors", reference_vectors);

    const blst = b.lazyDependency("blst", .{}) orelse return;
    support.addImport("c_kzg", cKzgModule(b, target, c_kzg, blst));
    support.addImport("c_kzg_paths", paths_module);
}

/// Every native test and tool links the host build of the zkvm_* providers.
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
    return common.addTests(b, test_step, .{
        .target = target,
        .optimize = .ReleaseSafe,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "ef_fixtures", .module = paths.createModule() },
        },
    }, .{ .dir = "test/reference" });
}

/// c-kzg-4844 over blst, translated for Zig: the library that commits to blobs and proves them for
/// the native tests and tools. blst's portable build runs on every CPU of the target architecture.
fn cKzgModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    c_kzg: *std.Build.Dependency,
    blst: *std.Build.Dependency,
) *std.Build.Module {
    const translated = b.addTranslateC(.{
        .root_source_file = c_kzg.path("src/ckzg.h"),
        .target = target,
        .optimize = .Debug,
    });
    translated.addIncludePath(c_kzg.path("src"));
    translated.addIncludePath(blst.path("bindings"));
    const module = translated.createModule();
    module.addIncludePath(c_kzg.path("src"));
    module.addIncludePath(blst.path("bindings"));
    module.addCSourceFile(.{ .file = c_kzg.path("src/ckzg.c"), .flags = &.{ "-O2", "-fno-sanitize=undefined" } });
    const has_assembly = switch (target.result.cpu.arch) {
        .aarch64, .x86_64 => true,
        else => false,
    };
    // The C sources and the assembly select the same field-arithmetic variants by the portable macro.
    const blst_flags: []const []const u8 = if (has_assembly)
        &.{ "-O2", "-fno-sanitize=undefined", "-D__BLST_PORTABLE__" }
    else
        &.{ "-O2", "-fno-sanitize=undefined", "-D__BLST_PORTABLE__", "-D__BLST_NO_ASM__" };
    module.addCSourceFile(.{ .file = blst.path("src/server.c"), .flags = blst_flags });
    if (has_assembly) module.addCSourceFile(.{ .file = blst.path("build/assembly.S"), .flags = blst_flags });
    return module;
}

/// The smoke guests: freestanding rv64im programs that run one part of the package on the guest
/// target. Each is its own ELF, named `<smoke>_smoke`; `-Dsmoke` selects the one `zig build` links.
const Smoke = enum {
    /// Dictionary-based zstd decompression.
    zstd,
    /// Blob KZG verification and the blob's versioned hash.
    blob,
    /// Block truncation and the 254-bit packing of its payload.
    truncation,
    /// Verification of a DA stream of one conflation in one chunk.
    decompress,
};

fn addSmokeGuest(
    b: *std.Build,
    package: *std.Build.Module,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
) void {
    const smoke = b.option(Smoke, "smoke", "Smoke guest to link: zstd (default), blob, truncation or decompress") orelse .zstd;
    const accelerators = b.dependency("lineth_accelerators", .{
        .target = target,
        .optimize = optimize,
        .@"zkvm-provide" = true,
    });
    var imports: std.ArrayList(Import) = .empty;
    imports.appendSlice(b.allocator, &.{
        .{ .name = "lineth_da", .module = package },
        .{ .name = "lineth_accelerators", .module = accelerators.module("lineth_accelerators") },
    }) catch @panic("OOM");
    // The guests that reach sha256, keccak256, the KZG point evaluation or sender recovery define
    // the zkvm_* symbols the package declares.
    if (smoke != .zstd) imports.append(b.allocator, .{ .name = "zkvm_provide", .module = accelerators.module("zkvm_provide") }) catch @panic("OOM");

    common.addGuest(b, .{
        .name = b.fmt("{s}_smoke", .{@tagName(smoke)}),
        .root_source_file = b.fmt("test/smoke/{s}_smoke.zig", .{@tagName(smoke)}),
        .target = target,
        .optimize = optimize,
        .imports = imports.items,
        .code_model = .medium,
    });
}

/// Wires the package module's embedded dictionary and the libzstd decoder behind its `zstd`
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
