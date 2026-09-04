//! Constantine (Nim→C) precompile staticlib, built from the fork (which carries the
//! `defined(standalone)` gates, the embedded KZG SRS, and the rv64im-freestanding
//! nimble task natively). Exposes the two lazy-path artifacts l2-execution links:
//!   - riscv_staticlib: rv64im archive (fork task output + guest allocator object)
//!   - host_staticlib:  host archive for the FFI unit tests
//! Toolchain prerequisites (nim, nimble, clang) are installed by
//! `make -C riscv-guests install-constantine-deps`; tool locations are overridable via the
//! NIM / NIMBLE / LLVM_AR env vars (evaluated when the build script itself runs,
//! so they are not cached per step).

const std = @import("std");

const Toolchain = struct {
    nim: []const u8,
    nimble: []const u8,
    llvm_ar: []const u8,
};

fn envOr(b: *std.Build, key: []const u8, default: []const u8) []const u8 {
    return b.graph.environ_map.get(key) orelse default;
}

/// Locates nim, nimble and llvm-ar: env override, else PATH, else Homebrew's keg-only llvm
/// prefix (macOS, where llvm is deliberately not on PATH). Fatal with an actionable message
/// when a tool is missing — this runs at build-script evaluation, before any step executes.
fn resolveToolchain(b: *std.Build) Toolchain {
    // Homebrew's llvm is keg-only (not on PATH); /opt/homebrew/opt is its stable symlinked
    // prefix on Apple Silicon. Passed as an extra search dir — a nonexistent one is skipped.
    const extra: []const []const u8 = if (b.graph.host.result.os.tag == .macos)
        &.{"/opt/homebrew/opt/llvm/bin"}
    else
        &.{};
    return .{
        .nim = findTool(b, envOr(b, "NIM", "nim"), extra),
        .nimble = findTool(b, envOr(b, "NIMBLE", "nimble"), extra),
        .llvm_ar = findTool(b, envOr(b, "LLVM_AR", "llvm-ar"), extra),
    };
}

fn findTool(b: *std.Build, name: []const u8, extra: []const []const u8) []const u8 {
    return b.findProgram(&.{name}, extra) catch {
        std.debug.print(
            "error: required tool '{s}' not found; install it with: make -C riscv-guests install-constantine-deps\n",
            .{name},
        );
        std.process.exit(1);
    };
}

pub fn build(b: *std.Build) void {
    const tc = resolveToolchain(b);
    const ctt = b.dependency("constantine", .{});
    // The fork tree carries the standalone gates + shims; the rv64 nimble task compiles in
    // place (writing lib/ + nimcache/ under the fork), while the host nim compile redirects
    // --nimcache/--outdir into zig output dirs.
    const tree = ctt.path(".");

    b.addNamedLazyPath("riscv_staticlib", buildRiscv(b, tc, tree));
    b.addNamedLazyPath("host_staticlib", buildHost(b, tc, tree));
}

const NIM_BINDINGS = "bindings/lib_constantine.nim";

fn buildRiscv(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath) std.Build.LazyPath {
    // The fork owns the rv64im-freestanding compile (target flags, clang driver shim,
    // standalone headers, stdio backing, archive reindex) via its nimble task; here we
    // only drive it and merge the guest allocator in.
    const task = b.addSystemCommand(&.{ tc.nimble, "-y", "make_lib_riscv64_freestanding" });
    task.setName("nimble make_lib_riscv64_freestanding (rv64im)");
    task.setCwd(tree);
    // Cache invalidation comes from the dependency's .zon hash once it points at the pinned
    // repo url; the archive is an in-tree product consumed by the merge.
    const archive = tree.join(b.allocator, "lib/libconstantine.riscv64.a") catch @panic("oom");

    // The FBA-backed malloc shim, compiled with zig for rv64im (same soft-float feature set
    // as the l2-execution guest's mcpu, or lld rejects the mixed-ABI link).
    const allocator_obj = b.addObject(.{
        .name = "ctt_allocator_rv64",
        .root_module = b.createModule(.{
            .root_source_file = b.path("c_allocator.zig"),
            .target = b.resolveTargetQuery(.{
                .cpu_arch = .riscv64,
                .os_tag = .freestanding,
                .cpu_model = .{ .explicit = &std.Target.riscv.cpu.generic_rv64 },
                .cpu_features_add = std.Target.riscv.featureSet(&.{ .a, .c, .m, .zaamo, .zalrsc, .zicsr }),
            }),
            .optimize = .ReleaseSmall,
        }),
    });

    const merged = mergeArchive(b, tc, "constantine rv64", archive, allocator_obj.getEmittedBin(), "libguest_crypto_ctt.a");
    // Merge after the task (which produces the archive it consumes).
    merged.step.step.dependOn(&task.step);
    return merged.archive;
}

/// nim compile flags shared by both targets; the caller appends target flags and the
/// bindings compile arg.
fn nimCmd(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath, name: []const u8) *std.Build.Step.Run {
    const nim = b.addSystemCommand(&.{
        tc.nim,             "c",                  "--cc:clang",
        "--mm:arc",         "-d:useMalloc",       "--panics:on",
        "-d:CTT_ASM=false", "--threads:on",       "--noMain",
        "--app:staticlib",  "--nimMainPrefix:ctt_init_",
        "-d:release",       "-d:danger",          "--opt:size",
    });
    nim.setName(name);
    nim.setCwd(tree); // constantine/config.nims resolves --path:"." against cwd
    return nim;
}

fn buildHost(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath) std.Build.LazyPath {
    const nim = nimCmd(b, tc, tree, "nim compile constantine (host)");
    nim.addArgs(&.{ "--os:macosx", "--cc:clang" });
    _ = nim.addPrefixedOutputDirectoryArg("--outdir:", "host");
    const nimcache = nim.addPrefixedOutputDirectoryArg("--nimcache:", "host-nimcache");
    _ = nimcache; // declared as a cache output; the archive is the consumed artifact
    const archive = nim.addPrefixedOutputFileArg("--out:", "libconstantine.host.a");
    nim.addFileArg(tree.join(b.allocator, NIM_BINDINGS) catch @panic("oom"));

    const allocator_obj = b.addObject(.{
        .name = "ctt_allocator_host",
        .root_module = b.createModule(.{
            .root_source_file = b.path("c_allocator.zig"),
            .target = b.graph.host,
            .optimize = .ReleaseSmall,
        }),
    });

    return mergeArchive(b, tc, "constantine host", archive, allocator_obj.getEmittedBin(), "libguest_crypto_ctt_host.a").archive;
}

/// llvm-ar MRI merge: add the allocator object to the nim-built archive. The rv64 archive
/// arrives already reindexed by the fork's nimble task; the host archive is native, so its
/// nim index is valid as-is. llvm-ar reads the MRI script from stdin (`-M` takes no file
/// arg), so the heredoc below IS the script, with the zig-assigned paths spliced in as
/// $2..$4. Returns the Run step so callers can add ordering deps; `.archive` is the output.
const Merged = struct { step: *std.Build.Step.Run, archive: std.Build.LazyPath };

fn mergeArchive(b: *std.Build, tc: Toolchain, name: []const u8, lib: std.Build.LazyPath, allocator_obj: std.Build.LazyPath, out_name: []const u8) Merged {
    const merge = b.addSystemCommand(&.{ "sh", "-c",
        \\"$1" -M <<EOF
        \\create $3
        \\addlib $2
        \\addmod $4
        \\save
        \\end
        \\EOF
    , b.fmt("merge {s}", .{name}) });
    merge.setName(b.fmt("archive + merge {s}", .{name}));
    merge.addArgs(&.{tc.llvm_ar}); // $1
    merge.addFileArg(lib); // $2
    const out = merge.addOutputFileArg(out_name); // $3
    merge.addFileArg(allocator_obj); // $4
    return .{ .step = merge, .archive = out };
}
