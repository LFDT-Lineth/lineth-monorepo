const field = @import("../field/koalabear.zig");
const constants = @import("poseidon2_constants.zig");
const profiling = @import("../profiling.zig");
const r5_config = @import("r5_config");
const lineth_accel = if (r5_config.disable_accelerators) struct {} else @import("lineth_accelerators");

pub const Error = field.Error || error{InvalidInputLength};
pub const Digest = [8]field.Element;
pub const block_size = 8;
pub const digest_bytes = block_size * field.bytes;

const full_rounds = 6;
const partial_rounds = 21;
const total_rounds = full_rounds + partial_rounds;

pub fn zeroDigest() Digest {
    return zeroArray(block_size);
}

// In-place Merkle–Damgård compression: `state := compress(state, right)`.
//
// Both inputs are taken by pointer and the feed-forward result is written back
// through `state`, so the hot path (`MDHasher`) never copies a digest into or
// out of `compress` (no by-value arguments and return value). `state`
// is read into the local permutation buffer before it is overwritten, so it is
// fine for `state` to be the running hash; `right` must not alias `state`.
pub fn compressInPlace(state: *Digest, right: *const Digest) void {
    profiling.poseidon2Compress();
    // `align(8)` matches `zkvm_bytes_64`, letting `permutationAccel16` reinterpret
    // this buffer in place (see there). The array assignments lower to word stores,
    // unlike `@memcpy` on byte buffers which `ReleaseSmall` turns into a byte loop.
    var buf: [16]field.Element align(8) = undefined;
    // Element-wise copies lower to word loads/stores; `.* =` on the right half
    // otherwise regresses to a byte-wise `memcpy` under `ReleaseSmall`.
    inline for (0..block_size) |i| {
        buf[i] = state[i];
        buf[block_size + i] = right[i];
    }

    permutation(16, &buf);
    for (state, right, buf[block_size..]) |*dst, r, perm| {
        dst.* = r.add(perm);
    }
}

pub fn compress(left: Digest, right: Digest) Digest {
    var out = left;
    compressInPlace(&out, &right);
    return out;
}

pub fn compressSlices(left: []const field.Element, right: []const field.Element) Error!Digest {
    if (left.len != block_size or right.len != block_size) return Error.InvalidInputLength;
    return compress(left[0..block_size].*, right[0..block_size].*);
}

pub fn digestToBytes(digest: Digest) [digest_bytes]u8 {
    var out: [digest_bytes]u8 = undefined;
    for (digest, 0..) |limb, i| {
        const encoded = limb.toBytes();
        @memcpy(out[i * field.bytes .. (i + 1) * field.bytes], &encoded);
    }
    return out;
}

pub const MDHasher = struct {
    state: Digest,
    buffer: [block_size]field.Element,
    buffer_len: usize,

    pub fn init() MDHasher {
        return .{
            .state = zeroDigest(),
            .buffer = zeroArray(block_size),
            .buffer_len = 0,
        };
    }

    pub fn writeElement(self: *MDHasher, value: field.Element) void {
        self.buffer[self.buffer_len] = value;
        self.buffer_len += 1;
        if (self.buffer_len == block_size) {
            compressInPlace(&self.state, &self.buffer);
            self.buffer_len = 0;
        }
    }

    pub fn writeElements(self: *MDHasher, values: []const field.Element) void {
        var rest = values;
        // Fast path: when the buffer is empty, compress full blocks straight from
        // the input. This skips the per-element buffering branch and the
        // out-of-line `writeElement` call (8 of each per compression otherwise).
        if (self.buffer_len == 0) {
            while (rest.len >= block_size) {
                compressInPlace(&self.state, rest[0..block_size]);
                rest = rest[block_size..];
            }
        }
        for (rest) |value| {
            self.writeElement(value);
        }
    }

    pub fn writeBytes(self: *MDHasher, encoded: []const u8) Error!void {
        if (encoded.len % field.bytes != 0) return Error.InvalidInputLength;
        var offset: usize = 0;
        while (offset < encoded.len) : (offset += field.bytes) {
            self.writeElement(try field.Element.fromBytesCanonicalSlice(encoded[offset .. offset + field.bytes]));
        }
    }

    pub fn sumDigest(self: *MDHasher) Digest {
        if (self.buffer_len != 0) {
            var block: Digest = zeroArray(block_size);
            // Match prover-ray MDHasher: partial blocks are zero-left-padded.
            @memcpy(block[block_size - self.buffer_len ..], self.buffer[0..self.buffer_len]);
            compressInPlace(&self.state, &block);
            self.buffer_len = 0;
        }
        return self.state;
    }

    pub fn sumBytes(self: *MDHasher) [digest_bytes]u8 {
        return digestToBytes(self.sumDigest());
    }

    pub fn getState(self: MDHasher) Digest {
        var copy = self;
        return copy.sumDigest();
    }

    pub fn setState(self: *MDHasher, state: Digest) void {
        self.state = state;
        self.buffer_len = 0;
    }
};

pub fn hashElements(values: []const field.Element) Digest {
    var h = MDHasher.init();
    h.writeElements(values);
    return h.sumDigest();
}

pub fn permutation(comptime width: usize, state: *[width]field.Element) void {
    if (comptime !r5_config.disable_accelerators) {
        permutationAccel16(state);
    } else {
        permutationNative(width, state);
    }
}

fn permutationNative(comptime width: usize, state: *[width]field.Element) void {
    if (width != constants.width) @compileError("Poseidon2 Koalabear verifier constants support width 16");
    const round_keys = &constants.round_keys;

    matMulExternalInPlace(width, state);

    const half_full = full_rounds / 2;
    for (0..half_full) |round| {
        addRoundKey(width, state, round_keys, round, width);
        sBoxAll(width, state);
        matMulExternalInPlace(width, state);
    }

    for (half_full..half_full + partial_rounds) |round| {
        addRoundKey(width, state, round_keys, round, 1);
        state[0] = cube(state[0]);
        matMulInternalInPlace(width, state);
    }

    for (half_full + partial_rounds..total_rounds) |round| {
        addRoundKey(width, state, round_keys, round, width);
        sBoxAll(width, state);
        matMulExternalInPlace(width, state);
    }
}

// Delegate the width-16 permutation to the Lineth Poseidon2 accelerator opcode.
//
// `field.Element` is `extern struct { value: u32 }`, so `[16]Element` is exactly
// 16 little-endian canonical 32-bit words — the same layout the accelerator
// expects in `zkvm_bytes_64`. The bit casts are therefore plain reinterpretations.
fn permutationAccel16(state: *[constants.width]field.Element) void {
    // `[16]Element` is bit-identical to `zkvm_bytes_64` (16 LE u32 words = 64 bytes),
    // and the accelerator permits `input`/`output` to alias, so reinterpret the state
    // buffer in place. This avoids the two 64-byte copies (in/out staging) that the
    // previous `@bitCast` round-trip lowered to byte-wise `memcpy` on R5.
    // Safe because `compress` declares its state `align(8)`.
    const buf: *lineth_accel.zkvm_bytes_64 = @ptrCast(@alignCast(state));
    _ = lineth_accel.lineth_zkvm_poseidon2_permutation(buf, buf);
}

fn addRoundKey(
    comptime width: usize,
    state: *[width]field.Element,
    round_keys: *const [total_rounds][width]field.Element,
    round: usize,
    key_len: usize,
) void {
    for (0..key_len) |i| {
        state[i] = state[i].add(round_keys.*[round][i]);
    }
}

fn sBoxAll(comptime width: usize, state: *[width]field.Element) void {
    for (&state.*) |*limb| {
        limb.* = cube(limb.*);
    }
}

fn cube(value: field.Element) field.Element {
    return value.square().mul(value);
}

fn matMulM4InPlace(comptime width: usize, state: *[width]field.Element) void {
    inline for (0..width / 4) |chunk| {
        const offset = 4 * chunk;
        const t01 = state[offset].add(state[offset + 1]);
        const t23 = state[offset + 2].add(state[offset + 3]);
        const t0123 = t01.add(t23);
        const t01123 = t0123.add(state[offset + 1]);
        const t01233 = t0123.add(state[offset + 3]);

        state[offset + 3] = state[offset].double().add(t01233);
        state[offset + 1] = state[offset + 2].double().add(t01123);
        state[offset] = t01.add(t01123);
        state[offset + 2] = t23.add(t01233);
    }
}

fn matMulExternalInPlace(comptime width: usize, state: *[width]field.Element) void {
    matMulM4InPlace(width, state);

    var sums: [4]field.Element = zeroArray(4);
    inline for (0..width / 4) |chunk| {
        const offset = 4 * chunk;
        sums[0] = sums[0].add(state[offset]);
        sums[1] = sums[1].add(state[offset + 1]);
        sums[2] = sums[2].add(state[offset + 2]);
        sums[3] = sums[3].add(state[offset + 3]);
    }

    inline for (0..width / 4) |chunk| {
        const offset = 4 * chunk;
        state[offset] = state[offset].add(sums[0]);
        state[offset + 1] = state[offset + 1].add(sums[1]);
        state[offset + 2] = state[offset + 2].add(sums[2]);
        state[offset + 3] = state[offset + 3].add(sums[3]);
    }
}

fn zeroArray(comptime len: usize) [len]field.Element {
    var out: [len]field.Element = undefined;
    for (&out) |*limb| {
        limb.* = field.Element.zero();
    }
    return out;
}

// Precomputed 2^{-n} mod p for the KoalaBear field (p = 2_130_706_433 = 2^31 - 2^24 + 1).
const inv2Exp1: field.Element = .{ .value = 1_065_353_217 };
const inv2Exp2: field.Element = .{ .value = 1_598_029_825 };
const inv2Exp3: field.Element = .{ .value = 1_864_368_129 };
const inv2Exp4: field.Element = .{ .value = 1_997_537_281 };
const inv2Exp5: field.Element = .{ .value = 2_064_121_857 };
const inv2Exp6: field.Element = .{ .value = 2_097_414_145 };
const inv2Exp7: field.Element = .{ .value = 2_114_060_289 };
const inv2Exp8: field.Element = .{ .value = 2_122_383_361 };
const inv2Exp9: field.Element = .{ .value = 2_126_544_897 };
const inv2Exp24: field.Element = .{ .value = 2_130_706_306 };

fn matMulInternalInPlace(comptime width: usize, state: *[width]field.Element) void {
    var sum = state[0];
    inline for (1..width) |i| {
        sum = sum.add(state[i]);
    }

    state[0] = sum.sub(state[0].double());
    state[1] = sum.add(state[1]);
    state[2] = sum.add(state[2].double());
    state[3] = sum.add(state[3].mul(inv2Exp1));
    state[4] = sum.add(state[4].mul(.{ .value = 3 }));
    state[5] = sum.add(state[5].double().double());
    state[6] = sum.sub(state[6].mul(inv2Exp1));
    state[7] = sum.sub(state[7].mul(.{ .value = 3 }));
    state[8] = sum.sub(state[8].double().double());
    state[9] = sum.add(state[9].mul(inv2Exp8));

    switch (width) {
        16 => {
            state[10] = sum.add(state[10].mul(inv2Exp3));
            state[11] = sum.add(state[11].mul(inv2Exp24));
            state[12] = sum.sub(state[12].mul(inv2Exp8));
            state[13] = sum.sub(state[13].mul(inv2Exp3));
            state[14] = sum.sub(state[14].mul(inv2Exp4));
            state[15] = sum.sub(state[15].mul(inv2Exp24));
        },
        24 => {
            state[10] = sum.add(state[10].mul(inv2Exp2));
            state[11] = sum.add(state[11].mul(inv2Exp3));
            state[12] = sum.add(state[12].mul(inv2Exp4));
            state[13] = sum.add(state[13].mul(inv2Exp5));
            state[14] = sum.add(state[14].mul(inv2Exp6));
            state[15] = sum.add(state[15].mul(inv2Exp24));
            state[16] = sum.sub(state[16].mul(inv2Exp8));
            state[17] = sum.sub(state[17].mul(inv2Exp3));
            state[18] = sum.sub(state[18].mul(inv2Exp4));
            state[19] = sum.sub(state[19].mul(inv2Exp5));
            state[20] = sum.sub(state[20].mul(inv2Exp6));
            state[21] = sum.sub(state[21].mul(inv2Exp7));
            state[22] = sum.sub(state[22].mul(inv2Exp9));
            state[23] = sum.sub(state[23].mul(inv2Exp24));
        },
        else => unreachable,
    }
}
