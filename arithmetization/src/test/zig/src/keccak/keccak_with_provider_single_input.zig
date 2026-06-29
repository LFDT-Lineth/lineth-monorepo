const wrappers = @import("wrappers");

const custom_std = wrappers.custom_std;
const keccak = wrappers.keccak_provide;

extern var _in_start: u8;

const LENGTH_FIELD_BYTES: usize = 8;

export fn main() noreturn {
    const input: [*]const u8 = @ptrFromInt(@intFromPtr(&_in_start));
    const msg_len = readU64Little(input[0..LENGTH_FIELD_BYTES]);
    const data: [*c]const u8 = @ptrFromInt(@intFromPtr(input) + LENGTH_FIELD_BYTES);

    var output_hash: keccak.zkvm_keccak256_hash = undefined;
    const output: [*c]keccak.zkvm_keccak256_hash = &output_hash;

    if (keccak.zkvm_keccak256(data, msg_len, output) != .ZKVM_EOK) {
        custom_std.exit(1);
    }

    const output_byte: *volatile u8 = @ptrCast(&output_hash.data[0]);
    _ = output_byte.*;

    custom_std.exit(0);
}

fn readU64Little(bytes: []const u8) usize {
    var value: u64 = 0;
    var i: usize = 0;
    while (i < LENGTH_FIELD_BYTES) : (i += 1) {
        value |= @as(u64, bytes[i]) << @intCast(i * 8);
    }
    return @intCast(value);
}
