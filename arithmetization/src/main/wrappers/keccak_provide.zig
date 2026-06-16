const custom_std = @import("custom_std.zig");
const zesu_accel = @import("zesu_zkvm_accel");
const linea_accel = @import("keccak.zig");
const build_options = @import("build_options"); // keccak_accel: standard zig keccak vs Linea wrapper

pub const zkvm_status = linea_accel.zkvm_status;
pub const zkvm_keccak256_hash = linea_accel.zkvm_keccak256_hash;

pub const zkvm_keccak256 = if (build_options.keccak_accel) linea_accel.zkvm_keccak256 else zesu_keccak256;

comptime {
    @export(&zkvm_keccak256, .{ .name = "zkvm_keccak256" });
}

fn zesu_keccak256(data: [*c]const u8, len: usize, output: [*c]zkvm_keccak256_hash) callconv(.c) zkvm_status {
    if (data == null or output == null) {
        custom_std.panic();
    }

    const data_ptr: [*]const u8 = @ptrFromInt(@intFromPtr(data));
    const output_ptr: *zkvm_keccak256_hash = @ptrFromInt(@intFromPtr(output));
    zesu_accel.keccak256(data_ptr[0..len], &output_ptr.data);
    return .ZKVM_EOK;
}
