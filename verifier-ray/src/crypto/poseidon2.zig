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

// Digest is a plain [8]field.Element alias (arrays hold no declarations of
// their own in Zig), so this is a free function rather than a Digest method.
// Exists so callers needing digest equality (crypto.merkle, query.fri) don't
// each need std.meta.eql -- see the callers' own reasoning for why avoiding
// std matters there.
pub fn eql(a: Digest, b: Digest) bool {
    for (a, b) |x, y| {
        if (!x.eql(y)) return false;
    }
    return true;
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

// The width-16 permutation in software, on u64 lanes with lazy reduction.
//
// KoalaBear's p is below 2^31, so a lane can hold a sum of many reduced values
// before it must be reduced: the external matrix reduces each output once
// (inputs < p, outputs < 35p), the internal matrix computes sum + D_i * x_i
// with D_i folded into [0, p) (sum < 16p, product < p^2, total < 2^63), and the
// S-box adds the round key and cubes with two reductions (x + k < 2p < 2^32, so
// both products fit in 64 bits). Every lane is < p on entry to each round.
// All rounds are unrolled with their keys as constants: under zkc each
// instruction is a cycle, and the per-round calls and loops of a
// straightforward version cost more than the arithmetic in partial rounds.
fn permutationNative(comptime width: usize, state: *[width]field.Element) void {
    if (width != constants.width) @compileError("Poseidon2 Koalabear verifier constants support width 16");
    // The unrolled rounds are comptime loops: 27 rounds of up to 16 lanes.
    @setEvalBranchQuota(20_000);
    var s: [width]u64 = undefined;
    inline for (0..width) |i| s[i] = state[i].value;

    matMulExternal(&s);
    const half_full = full_rounds / 2;
    inline for (0..half_full) |round| fullRound(&s, round);
    inline for (half_full..half_full + partial_rounds) |round| partialRound(&s, round);
    inline for (half_full + partial_rounds..total_rounds) |round| fullRound(&s, round);

    inline for (0..width) |i| state[i] = .{ .value = @intCast(s[i]) };
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

fn zeroArray(comptime len: usize) [len]field.Element {
    var out: [len]field.Element = undefined;
    for (&out) |*limb| {
        limb.* = field.Element.zero();
    }
    return out;
}

const p: u64 = field.modulus;

// (x + key)^3 mod p, for x < p.
inline fn sBox(x: u64, comptime key: u64) u64 {
    const t = x + key;
    const t2 = (t * t) % p;
    return (t2 * t) % p;
}

inline fn fullRound(s: *[constants.width]u64, comptime round: usize) void {
    inline for (0..constants.width) |i| s[i] = sBox(s[i], constants.round_keys[round][i].value);
    matMulExternal(s);
}

inline fn partialRound(s: *[constants.width]u64, comptime round: usize) void {
    s[0] = sBox(s[0], constants.round_keys[round][0].value);
    matMulInternal(s);
}

// The external matrix: circ(2 M4, M4, M4, M4), as M4 on each block of four
// followed by adding the four column sums. Lanes < p on entry and on exit.
inline fn matMulExternal(s: *[constants.width]u64) void {
    inline for (0..constants.width / 4) |chunk| {
        const o = 4 * chunk;
        const t01 = s[o] + s[o + 1];
        const t23 = s[o + 2] + s[o + 3];
        const t0123 = t01 + t23;
        const t01123 = t0123 + s[o + 1];
        const t01233 = t0123 + s[o + 3];
        const x0 = s[o];
        const x2 = s[o + 2];
        s[o + 3] = 2 * x0 + t01233;
        s[o + 1] = 2 * x2 + t01123;
        s[o] = t01 + t01123;
        s[o + 2] = t23 + t01233;
    }
    var sums: [4]u64 = .{ 0, 0, 0, 0 };
    inline for (0..constants.width / 4) |chunk| {
        inline for (0..4) |j| sums[j] += s[4 * chunk + j];
    }
    inline for (0..constants.width) |i| s[i] = (s[i] + sums[i % 4]) % p;
}

// Precomputed 2^{-n} mod p for the KoalaBear field (p = 2_130_706_433 = 2^31 - 2^24 + 1).
const inv2Exp1: u64 = 1_065_353_217;
const inv2Exp3: u64 = 1_864_368_129;
const inv2Exp4: u64 = 1_997_537_281;
const inv2Exp8: u64 = 2_122_383_361;
const inv2Exp24: u64 = 2_130_706_306;

// The internal matrix is 1 + diag(D): out_i = sum + D_i * x_i. D_i is
// -2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16,
// -1/2^24, each taken mod p so that subtraction becomes addition.
const internal_diag = [constants.width]u64{
    p - 2,        1,            2,            inv2Exp1,
    3,            4,            p - inv2Exp1, p - 3,
    p - 4,        inv2Exp8,     inv2Exp3,     inv2Exp24,
    p - inv2Exp8, p - inv2Exp3, p - inv2Exp4, p - inv2Exp24,
};

inline fn matMulInternal(s: *[constants.width]u64) void {
    var sum: u64 = 0;
    inline for (0..constants.width) |i| sum += s[i];
    inline for (0..constants.width) |i| {
        s[i] = if (internal_diag[i] == 1) (sum + s[i]) % p else (sum + internal_diag[i] * s[i]) % p;
    }
}
