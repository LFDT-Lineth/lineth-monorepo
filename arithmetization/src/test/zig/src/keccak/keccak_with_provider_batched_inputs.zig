const wrappers = @import("wrappers");

const custom_std = wrappers.custom_std;
const keccak = wrappers.keccak_provide;

extern var _in_start: u8;

const KECCAK256_PADDED_BITS: usize = 5440;
const KECCAK256_PADDED_BYTES: usize = KECCAK256_PADDED_BITS / 8;
const LENGTH_FIELD_BYTES: usize = 8;
const OUTPUT_BYTES: usize = 32;
const VECTOR_BYTES: usize = KECCAK256_PADDED_BYTES + LENGTH_FIELD_BYTES + OUTPUT_BYTES;

export fn main() noreturn {
    const input: [*]const u8 = @ptrFromInt(@intFromPtr(&_in_start));

    var vector_index: usize = 0;
    while (true) : (vector_index += 1) {
        const base = vector_index * VECTOR_BYTES;
        const vector = input[base .. base + VECTOR_BYTES];
        if (isZero(vector)) {
            custom_std.exit(0);
        }

        const padded = input[base .. base + KECCAK256_PADDED_BYTES];
        const len_start = base + KECCAK256_PADDED_BYTES;
        const msg_len_bits = readU64Little(input[len_start .. len_start + LENGTH_FIELD_BYTES]);
        const expected_start = len_start + LENGTH_FIELD_BYTES;
        const expected = input[expected_start .. expected_start + OUTPUT_BYTES];

        var msg_buf: [KECCAK256_PADDED_BYTES]u8 = undefined;
        const msg_len = extractLeftPaddedMessage(padded, msg_len_bits, &msg_buf) orelse custom_std.exit(2);

        var output_hash: keccak.zkvm_keccak256_hash = undefined;
        const data: [*c]const u8 = &msg_buf;
        const output: [*c]keccak.zkvm_keccak256_hash = &output_hash;
        if (keccak.zkvm_keccak256(data, msg_len, output) != .ZKVM_EOK) {
            custom_std.exit(1);
        }

        if (!digestEq(&output_hash.data, expected)) {
            custom_std.exit(1);
        }
    }
}

fn extractLeftPaddedMessage(
    padded: []const u8,
    msg_len_bits: u64,
    out: *[KECCAK256_PADDED_BYTES]u8,
) ?usize {
    if (msg_len_bits > KECCAK256_PADDED_BITS) {
        return null;
    }

    const len: usize = @intCast(msg_len_bits);
    if (len == 0) {
        return 0;
    }

    const skip = KECCAK256_PADDED_BITS - len;
    var acc: u8 = 0;
    var acc_bits: u4 = 0;
    var out_len: usize = 0;
    var bit_i = skip;
    while (bit_i < skip + len) : (bit_i += 1) {
        const byte_idx = bit_i / 8;
        const bit_in_byte: u3 = @intCast(7 - (bit_i % 8));
        const bit = (padded[byte_idx] >> bit_in_byte) & 1;
        acc = (acc << 1) | bit;
        acc_bits += 1;
        if (acc_bits == 8) {
            out[out_len] = acc;
            out_len += 1;
            acc = 0;
            acc_bits = 0;
        }
    }

    if (acc_bits > 0) {
        const shift: u3 = @intCast(8 - acc_bits);
        out[out_len] = acc << shift;
        out_len += 1;
    }

    return out_len;
}

fn readU64Little(bytes: []const u8) u64 {
    var value: u64 = 0;
    var i: usize = 0;
    while (i < LENGTH_FIELD_BYTES) : (i += 1) {
        value |= @as(u64, bytes[i]) << @intCast(i * 8);
    }
    return value;
}

fn digestEq(computed: *const [OUTPUT_BYTES]u8, expected: []const u8) bool {
    var ok = true;
    var i: usize = 0;
    while (i < OUTPUT_BYTES) : (i += 1) {
        ok = ok and computed[i] == expected[i];
    }
    return ok;
}

fn isZero(bytes: []const u8) bool {
    for (bytes) |byte| {
        if (byte != 0) {
            return false;
        }
    }
    return true;
}
