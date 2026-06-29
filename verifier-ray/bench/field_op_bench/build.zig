const std = @import("std");
const common = @import("build_common");

pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
    const target = common.standardGuestTarget(b);

    const koalabear_mod = b.addModule("koalabear", .{
        .root_source_file = b.path("../../src/field/koalabear.zig"),
        .target = target,
        .optimize = .ReleaseSmall,
        .strip = true,
    });

    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = .ReleaseSmall,
        .strip = true,
        .imports = &.{
            .{ .name = "koalabear", .module = koalabear_mod },
        },
    });

    // Link the statically-linked rv64im ELF with the shared entry stub (start.s)
    // + rv64im memory layout + dead-section GC.
    common.installGuestElf(b, main_mod, "field-op-bench");
}
