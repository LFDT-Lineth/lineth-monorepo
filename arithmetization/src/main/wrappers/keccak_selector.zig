const zesu_accel = @import("zesu_zkvm_accel"); // zesu-zkvm's pure-Zig precompile backend (stdlibs_accel)
const linea_accel = @import("wrappers"); // Linea accelerator wrappers (source paths wired in build.zig)
const build_options = @import("build_options"); // keccak_accel: standard zig keccak vs Linea wrapper

comptime {
    if (build_options.keccak_accel) {
        @export(&linea_accel.keccak.zkvm_keccak256, .{ .name = "zkvm_keccak256" });
    } else {
        @export(&keccak256, .{ .name = "zkvm_keccak256" });
    }
}

// Standard zig keccak (std.crypto via stdlibs_accel); used unless -Dkeccak-accel selects the wrapper.
fn keccak256(data: [*]const u8, len: usize, output: *[32]u8) callconv(.c) i32 {
    zesu_accel.keccak256(data[0..len], output);
    return 0;
}
