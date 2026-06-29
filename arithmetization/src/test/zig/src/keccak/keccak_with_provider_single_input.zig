//! Single call to non-accelerated keccak.
const lineth_accel = @import("lineth_zkvm_accel"); // hash type + zkvm_exit
const provide = @import("zkvm_provide"); // defines (exports) the zkvm_* symbols

// Force zkvm_provide's comptime @export block to emit zkvm_keccak256 into this ELF.
comptime {
    _ = provide;
}

// The exported C-ABI keccak provider; resolved from zkvm_provide above.
extern fn zkvm_keccak256(data: [*c]const u8, len: usize, output: [*c]lineth_accel.zkvm_keccak256_hash) lineth_accel.zkvm_status;

extern var _in_start: u8;

const LENGTH_FIELD_BYTES: usize = 8;

export fn main() noreturn {
    const input: [*]const u8 = @ptrFromInt(@intFromPtr(&_in_start));
    const msg_len = readU64Little(input[0..LENGTH_FIELD_BYTES]);
    const data: [*c]const u8 = @ptrFromInt(@intFromPtr(input) + LENGTH_FIELD_BYTES);

    var output_hash: lineth_accel.zkvm_keccak256_hash = undefined;
    const output: [*c]lineth_accel.zkvm_keccak256_hash = &output_hash;

    if (zkvm_keccak256(data, msg_len, output) != .ZKVM_EOK) {
        lineth_accel.zkvm_exit(1);
    }

    const output_byte: *volatile u8 = @ptrCast(&output_hash.data[0]);
    _ = output_byte.*;

    lineth_accel.zkvm_exit(0);
}

fn readU64Little(bytes: []const u8) usize {
    var value: u64 = 0;
    var i: usize = 0;
    while (i < LENGTH_FIELD_BYTES) : (i += 1) {
        value |= @as(u64, bytes[i]) << @intCast(i * 8);
    }
    return @intCast(value);
}
