//! Builds Constantine static archives for the guest and host FFI tests.
//! Tool locations are overridable with `NIM`, `NIMBLE`, and `LLVM_AR`.

const std = @import("std");

const Toolchain = struct {
    nim: []const u8,
    nimble: []const u8,
    llvm_ar: []const u8,
};

fn envOr(b: *std.Build, key: []const u8, default: []const u8) []const u8 {
    return b.graph.environ_map.get(key) orelse default;
}

/// Locates `nim`, `nimble`, and `llvm-ar`.
fn resolveToolchain(b: *std.Build) Toolchain {
    // Homebrew's LLVM is keg-only on macOS, so add its stable prefix explicitly.
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
    // Build the rv64 archive in place and the host archive into Zig output dirs.
    const tree = ctt.path(".");

    b.addNamedLazyPath("riscv_staticlib", buildRiscv(b, tc, tree));
    b.addNamedLazyPath("host_staticlib", buildHost(b, tc, tree));
}

const NIM_BINDINGS = "bindings/lib_constantine.nim";

fn buildRiscv(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath) std.Build.LazyPath {
    // Run the fork's rv64im-freestanding build and merge in the guest allocator.
    const task = b.addSystemCommand(&.{ tc.nimble, "-y", "make_lib_riscv64_freestanding" });
    task.setName("nimble make_lib_riscv64_freestanding (rv64im)");
    task.setCwd(tree);
    // The pinned dependency hash covers cache invalidation.
    const archive = tree.join(b.allocator, "lib/libconstantine.riscv64.a") catch @panic("oom");

    // Match the guest's rv64im soft-float ABI.
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
    // Merge after the task that produces the archive.
    merged.step.step.dependOn(&task.step);
    return merged.archive;
}

/// Shared `nim c` flags.
fn nimCmd(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath, name: []const u8) *std.Build.Step.Run {
    const nim = b.addSystemCommand(&.{
        tc.nim,             "c",                  "--cc:clang",
        "--mm:arc",         "-d:useMalloc",       "--panics:on",
        "-d:CTT_ASM=false", "--threads:on",       "--noMain",
        "--app:staticlib",  "--nimMainPrefix:ctt_init_",
        "-d:release",       "-d:danger",          "--opt:size",
    });
    nim.setName(name);
    nim.setCwd(tree); // `config.nims` resolves relative paths from cwd
    return nim;
}

fn buildHost(b: *std.Build, tc: Toolchain, tree: std.Build.LazyPath) std.Build.LazyPath {
    const nim = nimCmd(b, tc, tree, "nim compile constantine (host)");
    // The host archive backs the FFI unit test and its embedded KZG context.
    nim.addArgs(&.{ "--os:macosx", "--cc:clang", "-d:CTT_EMBEDDED_KZG" });
    _ = nim.addPrefixedOutputDirectoryArg("--outdir:", "host");
    const nimcache = nim.addPrefixedOutputDirectoryArg("--nimcache:", "host-nimcache");
    _ = nimcache; // declared as a cache output
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

/// Merges the allocator object into the Nim-built archive with an `llvm-ar` MRI script.
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
