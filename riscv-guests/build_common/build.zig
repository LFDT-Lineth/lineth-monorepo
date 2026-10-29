//! Shared build helpers for the riscv-guests packages — deliberately only the **guest-agnostic**
//! bits: the pinned toolchain, the common freestanding rv64im target (every guest targets the same
//! ZkC profile), freestanding-linkage hygiene, the package layout (`src/root.zig` is the package
//! module, `src/main.zig` the guest entry point), the standalone-ELF link (entry stub + memory
//! layout, shared because every guest links the same way), and small utilities.
//!
//! Anything tied to a specific workload stays in the guest that uses it. In particular the zesu /
//! EVM-execution wiring (executor + SSZ module imports, native crypto linking) lives in
//! l2-execution's build.zig — the rollup and rollup-aggregation guests do NOT run the EVM (they do
//! KZG/compression and recursive proof verification), so that wiring is not common.
//!
//! Each guest pulls this in via a `build_common` path dependency in its build.zig.zon plus
//! `const common = @import("build_common");`. Never built on its own — `build()` only registers the
//! formatting steps, like every package.

const std = @import("std");
const builtin = @import("builtin");

/// The one Zig toolchain pinned for all guests (kept in sync with riscv-guests/.zigversion).
pub const required_zig_version = "0.16.0";

/// Abort unless the active Zig matches the pinned toolchain.
pub fn requireZigVersion() void {
    if (!std.mem.eql(u8, builtin.zig_version_string, required_zig_version)) {
        std.debug.print(
            "riscv-guests requires Zig {s}; found {s}\n",
            .{ required_zig_version, builtin.zig_version_string },
        );
        @panic("unsupported Zig version");
    }
}

/// The freestanding rv64im target every guest builds for (the Lineth ZkC interpreter profile:
/// base RV64I + M, soft-float, no A/C/D/F/Zicsr). Overridable on the CLI like any standard target.
pub fn standardGuestTarget(b: *std.Build) std.Build.ResolvedTarget {
    return b.standardTargetOptions(.{
        .default_target = .{
            .cpu_arch = .riscv64,
            .cpu_model = .{ .explicit = &std.Target.riscv.cpu.generic_rv64 },
            .cpu_features_add = std.Target.riscv.featureSet(&.{.m}),
            .cpu_features_sub = std.Target.riscv.featureSet(&.{ .a, .c, .d, .f, .zicsr, .zaamo, .zalrsc }),
            .os_tag = .freestanding,
            .abi = .none,
        },
    });
}

/// The package module: `src/root.zig` for one target, plus the package's declared dependency
/// imports. Files under `src/` import each other by relative path, so this one module reaches every
/// source file; callers create it once per target they build for.
pub fn createPackageModule(
    b: *std.Build,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    imports: []const std.Build.Module.Import,
) *std.Build.Module {
    return b.createModule(.{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
        .imports = imports,
    });
}

pub const GuestOptions = struct {
    /// Name of the installed ELF.
    name: []const u8,
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode,
    /// Modules `src/main.zig` names: the package module and any dependency it uses directly.
    imports: []const std.Build.Module.Import,
    code_model: std.builtin.CodeModel = .default,
};

/// Builds the guest ELF from the entry point `src/main.zig` and installs it via `installGuestElf`.
/// Call after the package module and its dependency imports are fully wired: the freestanding
/// linkage pass walks the module graph as it stands.
pub fn addGuest(b: *std.Build, options: GuestOptions) void {
    const entry = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = options.target,
        .optimize = options.optimize,
        .imports = options.imports,
        .code_model = options.code_model,
    });
    clearFreestandingNativeLinkage(b, entry);
    installGuestElf(b, entry, options.name);
}

/// Package-relative paths of the files directly in `sub_dir` of `b`'s package whose names end in
/// `suffix`, sorted. A missing `sub_dir` yields an empty list.
pub fn filesIn(b: *std.Build, sub_dir: []const u8, suffix: []const u8) []const []const u8 {
    const io = b.graph.io;
    var dir = b.build_root.handle.openDir(io, sub_dir, .{ .iterate = true }) catch |err| switch (err) {
        error.FileNotFound => return &.{},
        else => std.debug.panic("cannot open {s}: {s}", .{ sub_dir, @errorName(err) }),
    };
    defer dir.close(io);

    var paths: std.ArrayList([]const u8) = .empty;
    var it = dir.iterate();
    while (it.next(io) catch |err| std.debug.panic("cannot scan {s}: {s}", .{ sub_dir, @errorName(err) })) |entry| {
        if (entry.kind != .file or !std.mem.endsWith(u8, entry.name, suffix)) continue;
        paths.append(b.allocator, b.pathJoin(&.{ sub_dir, entry.name })) catch @panic("OOM");
    }
    std.mem.sort([]const u8, paths.items, {}, struct {
        fn lessThan(_: void, lhs: []const u8, rhs: []const u8) bool {
            return std.mem.lessThan(u8, lhs, rhs);
        }
    }.lessThan);
    return paths.items;
}

/// How native (host) roots are built: the target and optimize mode they compile for, plus the
/// modules every root imports.
pub const NativeOptions = struct {
    target: std.Build.ResolvedTarget,
    optimize: std.builtin.OptimizeMode = .Debug,
    imports: []const std.Build.Module.Import = &.{},
};

pub const TestsOptions = struct {
    dir: []const u8 = "test",
    suffix: []const u8 = "_test.zig",
};

pub const ToolsOptions = struct {
    dir: []const u8 = "tools",
};

/// Every `<dir>/*<suffix>` file as a native test root, run by `step`; returned keyed by root path.
/// Callers add any link inputs to the returned artifacts.
pub fn addTests(
    b: *std.Build,
    step: *std.Build.Step,
    native: NativeOptions,
    layout: TestsOptions,
) std.StringHashMap(*std.Build.Step.Compile) {
    var tests = std.StringHashMap(*std.Build.Step.Compile).init(b.allocator);
    for (filesIn(b, layout.dir, layout.suffix)) |root| {
        const compile = b.addTest(.{ .root_module = nativeRoot(b, root, native) });
        step.dependOn(&b.addRunArtifact(compile).step);
        tests.put(root, compile) catch @panic("OOM");
    }
    return tests;
}

/// Every `<dir>/*.zig` file as a native executable named after its file stem; returned keyed by
/// name. Nothing is installed or run: callers wire the install and run steps they want.
pub fn addTools(
    b: *std.Build,
    native: NativeOptions,
    layout: ToolsOptions,
) std.StringHashMap(*std.Build.Step.Compile) {
    var tools = std.StringHashMap(*std.Build.Step.Compile).init(b.allocator);
    for (filesIn(b, layout.dir, ".zig")) |root| {
        const name = std.fs.path.stem(root);
        const exe = b.addExecutable(.{ .name = name, .root_module = nativeRoot(b, root, native) });
        tools.put(name, exe) catch @panic("OOM");
    }
    return tools;
}

fn nativeRoot(b: *std.Build, root_file: []const u8, native: NativeOptions) *std.Build.Module {
    return b.createModule(.{
        .root_source_file = b.path(root_file),
        .target = native.target,
        .optimize = native.optimize,
        .imports = native.imports,
    });
}

pub fn parseAddress(comptime name: []const u8, value: []const u8) u64 {
    return std.fmt.parseInt(u64, value, 0) catch @panic("invalid -D" ++ name ++ " value");
}

/// Clear native libc/libcpp linkage across a module graph — freestanding rv64im guests must not
/// link a host libc, but imported modules may default to it.
pub fn clearFreestandingNativeLinkage(b: *std.Build, module: *std.Build.Module) void {
    var seen = std.AutoHashMap(*std.Build.Module, void).init(b.allocator);
    clearFreestandingNativeLinkageInner(&seen, module);
}

fn clearFreestandingNativeLinkageInner(
    seen: *std.AutoHashMap(*std.Build.Module, void),
    module: *std.Build.Module,
) void {
    if (seen.get(module) != null) return;
    seen.put(module, {}) catch @panic("OOM");

    if (@hasField(std.Build.Module, "link_libc")) {
        module.link_libc = false;
    }
    if (@hasField(std.Build.Module, "link_libcpp")) {
        module.link_libcpp = false;
    }

    var imports = module.import_table.iterator();
    while (imports.next()) |entry| {
        clearFreestandingNativeLinkageInner(seen, entry.value_ptr.*);
    }
}

// Standalone-ELF link inputs, shared by every guest: the entry stub (sets sp, calls main) and the
// rv64im memory layout. @embedFile-d into the build runner from build_common's own directory, so the
// helper below needs no path-dependency gymnastics to reach them (b.path would resolve against the
// *guest*, not here).
const entry_stub_source = @embedFile("start.s");
const linker_script_source = @embedFile("linker_script.ld");

/// Links a guest into its default deliverable: a **statically-linked rv64im ELF**, which is the
/// zkvm-standards riscv-target artifact ("Object Format: ELF, statically linked"):
/// https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md
///
/// There is deliberately no relocatable-`.o` path: a `.o` is not statically linked, and the ZkC
/// interpreter loads a finished ELF (via ELF→JSON, which reads PT_LOAD segments + an entry point) —
/// it does not perform a final link. Every guest links the same way, so the entry stub and memory
/// layout live here and are shared.
///
/// `root_module` must be the guest's fully-wired root module (all imports added, freestanding linkage
/// cleared, `code_model` set). On top of that this helper adds the pieces a final link needs:
///   • the entry stub (start.s: set sp from the linker script, then call main),
///   • the rv64im memory layout (linker_script.ld: stack @0, program @0x00800000, input @0x08800000,
///     heap @0x48800000 — kept in sync with arithmetization/src/test per PR #3332),
///   • dead-section GC, so the linked ELF drops unreferenced code.
/// Zig links its own soft-float compiler_rt (`__udivti3` / `mem*`) automatically.
///
/// Installed by the DEFAULT build (`zig build` → zig-out/bin/<name>); also reachable via the `elf`
/// step alias. NOTE: linker_script.ld's IN origin assumes the default `-Din-origin`
/// (0x08800000); a guest built with a custom offset needs a matching linker script.
pub fn installGuestElf(b: *std.Build, root_module: *std.Build.Module, guest_name: []const u8) void {
    const wf = b.addWriteFiles();
    const exe = b.addExecutable(.{ .name = guest_name, .root_module = root_module });
    exe.root_module.addAssemblyFile(wf.add("start.s", entry_stub_source));
    exe.setLinkerScript(wf.add("linker_script.ld", linker_script_source));
    exe.link_gc_sections = true;

    const install = b.addInstallArtifact(exe, .{});
    b.getInstallStep().dependOn(&install.step); // default `zig build` yields the statically-linked ELF
    b.step("elf", "Alias for the default build — the statically-linked ZkC ELF").dependOn(&install.step);

    const release_dir = b.option([]const u8, "release-dir", "Directory for the release ELF") orelse "zig-out/release";
    const program_id_file = b.option([]const u8, "program-id-file", "Path to write the ELF Program ID") orelse "zig-out/program-id";
    const release_guest = b.option([]const u8, "release-guest", "Guest name for the release ELF") orelse "";
    const release_version = b.option([]const u8, "release-version", "Bare release version for the release ELF") orelse "";
    const program_id_tool = b.addExecutable(.{
        .name = "program-id",
        .root_module = b.createModule(.{
            .root_source_file = b.dependency("build_common", .{}).path("program_id.zig"),
            .target = b.resolveTargetQuery(.{}),
            .optimize = .Debug,
        }),
    });
    const program_id = b.addRunArtifact(program_id_tool);
    program_id.addFileArg(exe.getEmittedBin());
    program_id.addArg(release_dir);
    program_id.addArg(program_id_file);
    program_id.addArg(release_guest);
    program_id.addArg(release_version);
    program_id.stdio = .inherit;
    program_id.has_side_effects = true;
    b.step("program-id", "Stage the versioned guest ELF with its Keccak-256 Program ID").dependOn(&program_id.step);
}

/// Registers `zig build fmt` (rewrite in place) and `zig build fmt-check` (verify only) over every
/// Zig source in the calling package. `zig fmt` has no recursive exclude, so the package's own
/// `zig-pkg/` (fetched dependencies) and `zig-out/` are excluded by exact path.
pub fn addFmtSteps(b: *std.Build) void {
    const exclude_paths: []const []const u8 = &.{ "zig-pkg", "zig-out" };
    const fmt = b.addFmt(.{ .paths = &.{"."}, .exclude_paths = exclude_paths });
    const fmt_check = b.addFmt(.{ .paths = &.{"."}, .exclude_paths = exclude_paths, .check = true });
    b.step("fmt", "Format this package's Zig sources in place").dependOn(&fmt.step);
    b.step("fmt-check", "Verify this package's Zig sources are formatted").dependOn(&fmt_check.step);
}

/// build_common is consumed via `@import`; its only step of its own is formatting.
pub fn build(b: *std.Build) void {
    addFmtSteps(b);
}
