const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;
pub const zkvm_bytes_32 = types.zkvm_bytes_32;
pub const zkvm_bytes_64 = types.zkvm_bytes_64;

pub const zkvm_secp256k1_hash = zkvm_bytes_32;
pub const zkvm_secp256k1_signature = zkvm_bytes_64;
pub const zkvm_secp256k1_pubkey = zkvm_bytes_64;

pub fn zkvm_secp256k1_verify(
    msg: [*c]const zkvm_secp256k1_hash,
    sig: [*c]const zkvm_secp256k1_signature,
    pubkey: [*c]const zkvm_secp256k1_pubkey,
    verified: [*c]bool,
) callconv(.c) zkvm_status {
    if (msg == null or sig == null or pubkey == null or verified == null) {
        lineth_std.panic();
    }

    asm volatile (
        \\.insn r 0x2b, 0b000, 0b0000001, %[verified], %[msg], %[sig]
        :
        : [verified] "r" (@intFromPtr(verified)),
          [msg] "r" (@intFromPtr(msg)),
          [sig] "r" (@intFromPtr(sig)),
        : .{ .memory = true });
    return .ZKVM_EOK;
}
