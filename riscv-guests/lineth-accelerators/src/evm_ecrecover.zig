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

    // The RISC-V R-type custom instruction carries only three register operands
    // (out, msg, sig), so `recid` cannot be passed as its own scalar. Pack the
    // signature and recovery id into one contiguous 65-byte buffer, r ‖ s ‖ recid,
    // and hand its address to the accelerator (which reads recid at sig+64).
    var sig_recid: [65]u8 align(8) = undefined;
    const sig_bytes: [*]const u8 = @ptrCast(sig);
    var i: usize = 0;
    while (i < 64) : (i += 1) {
        sig_recid[i] = sig_bytes[i];
    }
    sig_recid[64] = recid;

    // The accelerator writes Qx || Qy || ok into this private response. Keeping
    // it separate from `output` lets the C API leave caller memory untouched
    // when recovery fails.
    var response: [65]u8 align(8) = undefined;
    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0000001, %[response], %[msg], %[sig]
        :
        : [response] "r" (@intFromPtr(&response)),
          [msg] "r" (@intFromPtr(msg)),
          [sig] "r" (@intFromPtr(&sig_recid)),
        : .{ .memory = true });

    if (response[64] == 0) {
        return .ZKVM_EFAIL;
    }
    const output_bytes: [*]u8 = @ptrCast(output);
    i = 0;
    while (i < 64) : (i += 1) {
        output_bytes[i] = response[i];
    }
    return .ZKVM_EOK;
}
