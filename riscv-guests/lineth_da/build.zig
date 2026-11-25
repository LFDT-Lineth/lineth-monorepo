//! Builds the `lineth_da` package module (data-availability building blocks for the guests).
//! `zig build test -Dhost-tools=true` runs the native unit tests, and `-Dzkevm-fixtures=<dir>` adds
//! the reference tests over the EF zkevm corpus and the host tests that take blocks from it (what
//! `make test` runs). With `-Dhost-tools=true`,
//! `host-lib` builds the shared library of the host layer.

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

/// libzstd's headers give its API default visibility; empty visibility macros and hidden visibility
/// keep libzstd out of the dynamic symbol table of the shared library.
const hidden_flags = [_][]const u8{ "-fvisibility=hidden", "-DZSTDLIB_VISIBLE=", "-DZSTDERRORLIB_VISIBLE=" };

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
        .imports = packageImports(b, target, optimize, zstd, true),
    });
    addPackageSources(b, package, zstd, &.{});
    b.modules.put(b.allocator, b.dupe("zesu_executor"), zesuFor(b, target, optimize, true).module("executor")) catch @panic("OOM");

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

/// How an instance of the package differs from the default one: the flags added to libzstd's decoder
/// sources, and whether zesu memoizes keccak256 in an unsynchronized global, which only pays off
/// inside a zkVM and which threads that hash concurrently have to leave out.
const PackageVariant = struct {
    c_flags: []const []const u8 = &.{},
    keccak_memo: bool = true,
};

/// The zesu dependency for one target, with the extern crypto backend. Its options select one
/// instance of zesu's modules, and a compilation can hold only one: the package and everything that
/// imports zesu next to it have to come from the same call. The default memo is no option, so that
/// instance is the one every other package of the guests builds.
fn zesuFor(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    keccak_memo: bool,
) *std.Build.Dependency {
    if (keccak_memo) return b.dependency("zesu", .{
        .target = target,
        .optimize = optimize,
        .@"crypto-backend" = .@"extern",
    });
    return b.dependency("zesu", .{
        .target = target,
        .optimize = optimize,
        .@"crypto-backend" = .@"extern",
        .@"keccak-memo" = false,
    });
}

/// What `src/` imports by name, for one target: the zesu modules under the `zesu_*` names the
/// sources use, the accelerator calls, the SSZ views of the witnesses, the declarations of libzstd and the memory hooks of the
/// decoder.
fn packageImports(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    zstd: Zstd,
    keccak_memo: bool,
) []const Import {
    const zesu = zesuFor(b, target, optimize, keccak_memo);
    const accelerators = b.dependency("lineth_accelerators", .{ .target = target, .optimize = optimize });
    const guest_common = b.dependency("guest_common", .{ .target = target, .optimize = optimize });
    return b.allocator.dupe(Import, &.{
        .{ .name = "guest_common", .module = guest_common.module("guest_common") },
        .{ .name = "zkvm_accelerators", .module = accelerators.module("zkvm_accelerators") },
        .{ .name = "zesu_executor", .module = zesu.module("executor") },
        .{ .name = "zesu_input", .module = zesu.module("input") },
        .{ .name = "zesu_mpt", .module = zesu.module("mpt") },
        .{ .name = "zesu_rlp_decode", .module = zesu.module("rlp_decode") },
        .{ .name = "zstd_c", .module = zstd.declarations },
        .{ .name = "zstd_memory", .module = zstd.memory },
    }) catch @panic("OOM");
}

/// The package module for a native target, with its sources attached.
fn hostPackage(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    zstd: Zstd,
    variant: PackageVariant,
) *std.Build.Module {
    const package = common.createPackageModule(b, target, optimize, packageImports(b, target, optimize, zstd, variant.keccak_memo));
    addPackageSources(b, package, zstd, variant.c_flags);
    return package;
}

/// The `zstd_encoder` module for a native target, with libzstd's compressor sources compiled with
/// `c_flags`. The tests of a consumer import it, so the module of the native tests is public.
fn addEncoder(
    b: *std.Build,
    public: bool,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    zstd: Zstd,
    c_flags: []const []const u8,
) *std.Build.Module {
    const options: std.Build.Module.CreateOptions = .{
        .root_source_file = b.path("host/zstd_encoder.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "zstd_c", .module = zstd.declarations },
            .{ .name = "zstd_memory", .module = zstd.memory },
        },
    };
    const encoder = if (public) b.addModule("zstd_encoder", options) else b.createModule(options);
    encoder.addCSourceFiles(.{ .root = zstd.source.path("lib"), .files = &encoder_sources, .flags = c_flags });
    return encoder;
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
    addHostLib(b);

    const target = b.resolveTargetQuery(.{});
    const optimize: std.builtin.OptimizeMode = if (package_optimize == .ReleaseSmall) .ReleaseSafe else package_optimize;
    const zstd = if (package_target.result.os.tag == .freestanding) Zstd.init(b, target, optimize) else package_zstd;
    const package = hostPackage(b, target, .Debug, zstd, .{});
    // The host layer's tests compress from several threads, so they run on zesu without its keccak memo.
    const host_package = hostPackage(b, target, .Debug, zstd, .{ .keccak_memo = false });

    const encoder = addEncoder(b, true, target, optimize, zstd, &.{});

    // Frames that break the rules of the DA frame form, for the tests of the decoder.
    const zstd_frames = b.createModule(.{
        .root_source_file = b.path("test/support/zstd_frames.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{.{ .name = "zstd_c", .module = zstd.declarations }},
    });

    const crypto = common.HostCrypto.forTarget(b, target, .{});
    const kzg_prover = kzgProverModule(b, target, optimize, crypto);

    const support = b.addModule("support", .{
        .root_source_file = b.path("test/support/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{.{ .name = "kzg_prover", .module = kzg_prover }},
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
    const host_readers = shared.readersOf(b, target, host_package);

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

    const host = hostModule(b, target, optimize, host_package, encoder, kzg_prover);
    const host_streams = b.createModule(.{
        .root_source_file = b.path("test/support/host_streams.zig"),
        .target = target,
        .optimize = optimize,
        .imports = &.{
            .{ .name = "da_chunks", .module = host_readers.da_chunks },
            .{ .name = "host", .module = host },
            .{ .name = "support", .module = support },
        },
    });
    const header = b.addTranslateC(.{
        .root_source_file = b.path("include/lineth_da.h"),
        .target = target,
        .optimize = .Debug,
    });
    const host_tests = common.addTests(b, test_step, .{
        .target = target,
        .imports = &.{
            .{ .name = "lineth_da", .module = host_package },
            .{ .name = "zstd_encoder", .module = encoder },
            .{ .name = "support", .module = support },
            .{ .name = "da_chunks", .module = host_readers.da_chunks },
            .{ .name = "host", .module = host },
            .{ .name = "kzg_prover", .module = kzg_prover },
            .{ .name = "host_streams", .module = host_streams },
            .{ .name = "lineth_da_h", .module = header.createModule() },
            .{ .name = "ef_fixtures", .module = fixturesModule(b, zkevm_fixtures) },
            .{ .name = "ef_conflations", .module = host_readers.ef_conflations },
        },
    }, .{ .dir = "test/host" });
    if (zkevm_fixtures == null and b.pkg_hash.len == 0) std.log.warn("test/host tests that read the EF corpus skipped: pass -Dzkevm-fixtures=<dir> (`make test` does)", .{});
    addKzgReferences(b, target, &.{ tests.get("test/blob_kzg_test.zig").?, host_tests.get("test/host/kzg_test.zig").? });
    const reference_tests = addReferenceTests(b, test_step, target, zstd, shared, zkevm_fixtures);
    linkHostCrypto(crypto, &.{ tests, host_tests, reference_tests });
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
    tests: []const *std.Build.Step.Compile,
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
    for (tests) |compile| compile.root_module.addImport("reference_vectors", reference_vectors);
}

/// Every native test links the host build of the zkvm_* providers.
fn linkHostCrypto(crypto: ?common.HostCrypto, artifacts: []const std.StringHashMap(*std.Build.Step.Compile)) void {
    const linked = crypto orelse return;
    for (artifacts) |group| linked.linkAll(group);
}

/// The module over the KZG prover for a native target, calling Constantine through its own
/// declarations. Those exist once Constantine is fetched, after which the build reruns.
fn kzgProverModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    crypto: ?common.HostCrypto,
) *std.Build.Module {
    const kzg_prover = b.createModule(.{
        .root_source_file = b.path("host/kzg_prover.zig"),
        .target = target,
        .optimize = optimize,
        .link_libc = true,
    });
    if (crypto) |host_crypto| kzg_prover.addImport("constantine_c", host_crypto.constantine_c);
    return kzg_prover;
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

    const package = hostPackage(b, target, .ReleaseSafe, zstd, .{});
    const readers = shared.readersOf(b, target, package);
    const zesu = zesuFor(b, target, .ReleaseSafe, true);
    return common.addTests(b, test_step, .{
        .target = target,
        .optimize = .ReleaseSafe,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "ef_fixtures", .module = fixturesModule(b, fixtures) },
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
/// `src/freestanding_c`; every other target uses its libc. `c_flags` are added to the flags of the
/// decoder sources.
fn addPackageSources(b: *std.Build, package: *std.Build.Module, zstd: Zstd, c_flags: []const []const u8) void {
    package.addAnonymousImport("zstd_dictionary_v2025_04_21", .{
        .root_source_file = b.path("dictionaries/v2025-04-21.bin"),
    });
    if (package.resolved_target.?.result.os.tag == .freestanding) {
        package.addIncludePath(b.path("src/freestanding_c"));
    } else {
        package.link_libc = true;
    }
    const flags = std.mem.concat(b.allocator, []const u8, &.{ &decoder_flags, c_flags }) catch @panic("OOM");
    package.addCSourceFiles(.{ .root = zstd.source.path("lib"), .files = &decoder_sources, .flags = flags });
}

/// The host layer for one native target: the module that is the root of the shared library, over a
/// package, an encoder and a KZG prover of that target.
fn hostModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    package: *std.Build.Module,
    encoder: *std.Build.Module,
    kzg_prover: *std.Build.Module,
) *std.Build.Module {
    return b.createModule(.{
        .root_source_file = b.path("host/root.zig"),
        .target = target,
        .optimize = optimize,
        .link_libc = true,
        .imports = &.{
            .{ .name = "lineth_da", .module = package },
            .{ .name = "zstd_encoder", .module = encoder },
            .{ .name = "kzg_prover", .module = kzg_prover },
        },
    });
}

/// The directory of the EF zkevm fixtures that the tests read, as a module; empty without the
/// fixtures.
fn fixturesModule(b: *std.Build, fixtures: ?std.Build.LazyPath) *std.Build.Module {
    const paths = b.addOptions();
    if (fixtures) |dir| {
        paths.addOptionPath("amsterdam_blocks", dir.path(b, "blockchain_tests/for_amsterdam"));
    } else {
        paths.addOption([]const u8, "amsterdam_blocks", "");
    }
    return paths.createModule();
}

/// The `host-lib` step builds the shared library of the host layer and installs it with its C
/// header. It is built for the machine that runs the build, on whose architecture the Constantine
/// archive is built: a baseline CPU model keeps the result independent of the CPU features of that
/// machine, and on Linux the glibc version is pinned.
fn addHostLib(b: *std.Build) void {
    const lib_step = b.step("host-lib", "Build the lineth_da shared library and install it with its C header");

    const host_os = b.graph.host.result.os.tag;
    var query: std.Target.Query = .{ .cpu_model = .baseline };
    if (host_os == .linux) {
        query.abi = .gnu;
        query.glibc_version = .{ .major = 2, .minor = 39, .patch = 0 };
    }
    const target = b.resolveTargetQuery(query);
    const optimize: std.builtin.OptimizeMode = .ReleaseSafe;
    const crypto = common.HostCrypto.forTarget(b, target, .{}) orelse return;

    // Its zesu is built without the keccak memo: compressors that run on several threads hash concurrently.
    const zstd = Zstd.init(b, target, optimize);
    const package = hostPackage(b, target, optimize, zstd, .{ .c_flags = &hidden_flags, .keccak_memo = false });
    const encoder = addEncoder(b, false, target, optimize, zstd, &hidden_flags);
    const library = hostModule(b, target, optimize, package, encoder, kzgProverModule(b, target, optimize, crypto));
    library.strip = true;
    const lib = b.addLibrary(.{ .name = "lineth_da", .linkage = .dynamic, .root_module = library });
    crypto.link(lib);
    if (host_os == .linux) {
        // The self-hosted linker ignores a version script and exports nothing, so lld links the library. The
        // script exports the `lineth_da_*` functions alone, which keeps the `zkvm_*`, `ctt_*` and libzstd
        // symbols from the other libraries of the process.
        lib.use_lld = true;
        lib.setVersionScript(b.path("host/lineth_da.map"));
        lib.link_gc_sections = true;
        lib.link_z_defs = true;
    }
    lib_step.dependOn(&b.addInstallArtifact(lib, .{}).step);
    lib_step.dependOn(&b.addInstallHeaderFile(b.path("include/lineth_da.h"), "lineth_da.h").step);
}
