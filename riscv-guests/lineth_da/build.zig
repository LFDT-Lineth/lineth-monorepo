//! Builds the `lineth_da` package module (data-availability building blocks for the guests).
//! `zig build` links a freestanding rv64im smoke guest (`-Dsmoke` selects it), which proves the
//! package module and the libraries behind it build and link on the guest target; `zig build test`
//! runs the native unit tests.

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
        .imports = packageImports(b, target, optimize),
    });
    addPackageSources(b, package, zstd_source);
    if (target.result.os.tag == .freestanding) addSmokeGuest(b, package, target, optimize);

    // Consumers of the package build the guest side only.
    const host_tools = b.option(bool, "host-tools", "Build the native tests and tools: fetches the reference vectors and builds the host crypto providers") orelse false;
    if (host_tools) addHostBuild(b, zstd_source);
}

/// What `src/` imports by name, for one target: the zesu modules under the `zesu_*` names the
/// sources use, and the scalar-field arithmetic.
fn packageImports(b: *std.Build, target: std.Build.ResolvedTarget, optimize: std.builtin.OptimizeMode) []const Import {
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

/// The native tests and tools. Like every other `b.addTest` artifact in this tree, these run on
/// the native host: a freestanding riscv64 target has no OS to run a `std.testing` binary against.
fn addHostBuild(b: *std.Build, zstd_source: *std.Build.Dependency) void {
    const target = b.resolveTargetQuery(.{});
    const imports = packageImports(b, target, .Debug);
    const package = common.createPackageModule(b, target, .Debug, imports);
    addPackageSources(b, package, zstd_source);

    const compressor = b.createModule(.{
        .root_source_file = b.path("test/zstd_compressor.zig"),
        .target = target,
        .optimize = .Debug,
    });
    compressor.addCSourceFiles(.{ .root = zstd_source.path("lib"), .files = &compressor_sources });

    const support = b.createModule(.{
        .root_source_file = b.path("test/support/root.zig"),
        .target = target,
        .optimize = .Debug,
    });

    const native: common.NativeOptions = .{
        .target = target,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "bls_scalar", .module = importNamed(imports, "bls_scalar") },
            .{ .name = "zstd_compressor", .module = compressor },
            .{ .name = "support", .module = support },
        },
    };
    const test_step = b.step("test", "Run native Zig unit tests for the lineth_da package");
    const tests = common.addTests(b, test_step, native, .{});
    for (golden_frames) |frame| {
        tests.get("test/zstd_test.zig").?.root_module.addAnonymousImport(frame.import_name, .{
            .root_source_file = zstd_source.path(frame.path),
        });
    }
    if (b.lazyDependency("c_kzg_4844", .{})) |c_kzg| {
        const vectors = b.addOptions();
        vectors.addOptionPath("tests_dir", c_kzg.path("tests"));
        tests.get("test/blob_kzg_test.zig").?.root_module.addImport("kzg_vectors", vectors.createModule());
    }

    const tools = common.addTools(b, native, .{});

    // The tests and tools that reach sha256, keccak256, the KZG point evaluation or sender
    // recovery link the host build of the zkvm_* providers.
    if (Crypto.init(b, target)) |crypto| {
        crypto.linkAll(tests);
        crypto.link(tools.get("truncate_block").?);
    }

    var tool_iter = tools.valueIterator();
    while (tool_iter.next()) |tool| b.installArtifact(tool.*);
}

fn importNamed(imports: []const Import, name: []const u8) *std.Build.Module {
    for (imports) |import| if (std.mem.eql(u8, import.name, name)) return import.module;
    unreachable;
}

/// The host build of the zkvm_* providers: the Constantine-backed provider module as an object and
/// the Constantine archive it needs, linked into native roots.
const Crypto = struct {
    provider: *std.Build.Step.Compile,
    constantine: std.Build.LazyPath,

    /// Null until the Constantine archive exists: it is fetched on demand and the build reruns once it is.
    fn init(b: *std.Build, target: std.Build.ResolvedTarget) ?Crypto {
        const constantine = b.dependency("guest_crypto", .{}).builder.named_lazy_paths.get("host_staticlib") orelse return null;
        // Optimized: the compiler's own linker crashes on a debug-built provider object.
        const accelerators = b.dependency("lineth_accelerators", .{
            .target = target,
            .optimize = .ReleaseSafe,
            .@"zkvm-provide" = true,
        });
        return .{
            .provider = b.addObject(.{ .name = "zkvm_provide", .root_module = accelerators.module("zkvm_provide") }),
            .constantine = constantine,
        };
    }

    fn link(crypto: Crypto, artifact: *std.Build.Step.Compile) void {
        artifact.root_module.addObject(crypto.provider);
        artifact.root_module.addObjectFile(crypto.constantine);
        artifact.root_module.link_libc = true;
    }

    fn linkAll(crypto: Crypto, artifacts: std.StringHashMap(*std.Build.Step.Compile)) void {
        var it = artifacts.valueIterator();
        while (it.next()) |artifact| crypto.link(artifact.*);
    }
};

/// The smoke guests: freestanding rv64im programs that run one part of the package on the guest
/// target. Each is its own ELF, named `<smoke>_smoke`; `-Dsmoke` selects the one `zig build` links.
const Smoke = enum {
    /// Dictionary-based zstd decompression.
    zstd,
    /// Blob KZG verification and the blob's versioned hash.
    blob,
    /// Block truncation and the 254-bit packing of its payload.
    truncation,
};

fn addSmokeGuest(
    b: *std.Build,
    package: *std.Build.Module,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
) void {
    const smoke = b.option(Smoke, "smoke", "Smoke guest to link: zstd (default), blob or truncation") orelse .zstd;
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

    const module = b.createModule(.{
        .root_source_file = b.path(b.fmt("test/smoke/{s}_smoke.zig", .{@tagName(smoke)})),
        .target = target,
        .optimize = optimize,
        .imports = imports.items,
        .code_model = .medium,
    });
    common.clearFreestandingNativeLinkage(b, module);
    common.installGuestElf(b, module, b.fmt("{s}_smoke", .{@tagName(smoke)}));
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
