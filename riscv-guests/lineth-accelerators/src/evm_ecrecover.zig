const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;
pub const zkvm_bytes_32 = types.zkvm_bytes_32;
pub const zkvm_bytes_64 = types.zkvm_bytes_64;

pub const zkvm_secp256k1_hash = zkvm_bytes_32;
pub const zkvm_secp256k1_signature = zkvm_bytes_64;
pub const zkvm_secp256k1_pubkey = zkvm_bytes_64;

pub fn zkvm_secp256k1_ecrecover(
    msg: [*c]const zkvm_secp256k1_hash,
    sig: [*c]const zkvm_secp256k1_signature,
    recid: u8,
    output: [*c]zkvm_secp256k1_pubkey,
) callconv(.c) zkvm_status {
    if (msg == null or sig == null or output == null) {
        lineth_std.panic();
    }

    _ = recid;

    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0000001, %[out], %[msg], %[sig]
        :
        : [out] "r" (@intFromPtr(output)),
          [msg] "r" (@intFromPtr(msg)),
          [sig] "r" (@intFromPtr(sig)),
        : .{ .memory = true });
    return .ZKVM_EOK;
}
