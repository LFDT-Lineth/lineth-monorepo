const field = @import("../field/koalabear.zig");
const constants = @import("poseidon2_constants.zig");

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

pub fn compress(left: Digest, right: Digest) Digest {
    var state: [16]field.Element = undefined;
    @memcpy(state[0..block_size], &left);
    @memcpy(state[block_size..], &right);

    var out = right;
    permutation(16, &state);
    for (&out, state[block_size..]) |*dst, state_limb| {
        dst.* = dst.add(state_limb);
    }
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
    compression_count: usize,

    pub fn init() MDHasher {
        return .{
            .state = zeroDigest(),
            .buffer = zeroArray(block_size),
            .buffer_len = 0,
            .compression_count = 0,
        };
    }

    pub fn writeElement(self: *MDHasher, value: field.Element) void {
        self.buffer[self.buffer_len] = value;
        self.buffer_len += 1;
        if (self.buffer_len == block_size) {
            self.state = compress(self.state, self.buffer);
            self.buffer_len = 0;
            self.compression_count += 1;
        }
    }

    pub fn writeElements(self: *MDHasher, values: []const field.Element) void {
        for (values) |value| {
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
            self.state = compress(self.state, block);
            self.buffer_len = 0;
            self.compression_count += 1;
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
        self.compression_count = 0;
    }
};

pub fn hashElements(values: []const field.Element) Digest {
    var h = MDHasher.init();
    h.writeElements(values);
    return h.sumDigest();
}

fn permutation(comptime width: usize, state: *[width]field.Element) void {
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
    for (0..width / 4) |chunk| {
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
    for (0..width / 4) |chunk| {
        const offset = 4 * chunk;
        sums[0] = sums[0].add(state[offset]);
        sums[1] = sums[1].add(state[offset + 1]);
        sums[2] = sums[2].add(state[offset + 2]);
        sums[3] = sums[3].add(state[offset + 3]);
    }

    for (0..width / 4) |chunk| {
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

fn matMulInternalInPlace(comptime width: usize, state: *[width]field.Element) void {
    var sum = state[0];
    for (state[1..]) |limb| {
        sum = sum.add(limb);
    }

    state[0] = sum.sub(state[0].double());
    state[1] = sum.add(state[1]);
    state[2] = sum.add(state[2].double());
    state[3] = sum.add(state[3].halve());
    state[4] = sum.add(state[4].mul(field.Element.init(3)));
    state[5] = sum.add(state[5].double().double());
    state[6] = sum.sub(state[6].halve());
    state[7] = sum.sub(state[7].mul(field.Element.init(3)));
    state[8] = sum.sub(state[8].double().double());
    state[9] = sum.add(state[9].mul2ExpNegN(8));

    switch (width) {
        16 => {
            state[10] = sum.add(state[10].mul2ExpNegN(3));
            state[11] = sum.add(state[11].mul2ExpNegN(24));
            state[12] = sum.sub(state[12].mul2ExpNegN(8));
            state[13] = sum.sub(state[13].mul2ExpNegN(3));
            state[14] = sum.sub(state[14].mul2ExpNegN(4));
            state[15] = sum.sub(state[15].mul2ExpNegN(24));
        },
        24 => {
            state[10] = sum.add(state[10].mul2ExpNegN(2));
            state[11] = sum.add(state[11].mul2ExpNegN(3));
            state[12] = sum.add(state[12].mul2ExpNegN(4));
            state[13] = sum.add(state[13].mul2ExpNegN(5));
            state[14] = sum.add(state[14].mul2ExpNegN(6));
            state[15] = sum.add(state[15].mul2ExpNegN(24));
            state[16] = sum.sub(state[16].mul2ExpNegN(8));
            state[17] = sum.sub(state[17].mul2ExpNegN(3));
            state[18] = sum.sub(state[18].mul2ExpNegN(4));
            state[19] = sum.sub(state[19].mul2ExpNegN(5));
            state[20] = sum.sub(state[20].mul2ExpNegN(6));
            state[21] = sum.sub(state[21].mul2ExpNegN(7));
            state[22] = sum.sub(state[22].mul2ExpNegN(9));
            state[23] = sum.sub(state[23].mul2ExpNegN(24));
        },
        else => unreachable,
    }
}
