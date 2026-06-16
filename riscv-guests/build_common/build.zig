//! Shared build helpers for the riscv-guests packages — deliberately only the **guest-agnostic**
//! bits: the pinned toolchain, the common freestanding rv64im target (every guest targets the same
//! ZkC profile), freestanding-linkage hygiene, the standalone-ELF link (entry stub + memory layout,
//! shared because every guest links the same way), and small utilities.
//!
//! Anything tied to a specific workload stays in the guest that uses it. In particular the zesu /
//! EVM-execution wiring (executor + SSZ module imports, native crypto linking) lives in
//! l2-execution's build.zig — the rollup and rollup-aggregation guests do NOT run the EVM (they do
//! KZG/compression and recursive proof verification), so that wiring is not common.
//!
//! Each guest pulls this in via a `build_common` path dependency in its build.zig.zon plus
//! `const common = @import("build_common");`. Never built on its own — `build()` is a no-op.

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

/// The freestanding rv64im target every guest builds for (the Linea ZkC interpreter profile:
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
}

/// build_common is consumed via `@import`, never built directly.
pub fn build(b: *std.Build) void {
    _ = b;
}
