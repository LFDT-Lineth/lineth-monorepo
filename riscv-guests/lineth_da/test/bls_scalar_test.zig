const std = @import("std");

const bls_scalar = @import("bls_scalar");

const Fr = bls_scalar.Fr;

const modulus = bls_scalar.modulus;

fn bytesOf(value: u256) [32]u8 {
    var bytes: [32]u8 = undefined;
    std.mem.writeInt(u256, &bytes, value, .big);
    return bytes;
}

fn valueOf(fr: Fr) u256 {
    return std.mem.readInt(u256, &fr.toBytes(), .big);
}

fn element(value: u256) Fr {
    return Fr.fromBytes(&bytesOf(value)).?;
}

fn mulMod(a: u256, b: u256) u256 {
    return @intCast(@as(u512, a) * b % modulus);
}

fn powMod(base: u256, exponent: u256) u256 {
    var result: u256 = 1;
    var power = base;
    var rest = exponent;
    while (rest > 0) : (rest >>= 1) {
        if (rest & 1 == 1) result = mulMod(result, power);
        power = mulMod(power, power);
    }
    return result;
}

/// Values near the edges of the field and random ones.
fn samples(random: std.Random, out: []u256) void {
    const edges = [_]u256{ 0, 1, 2, modulus - 1, modulus - 2, modulus / 2, modulus / 2 + 1, 1 << 64, 1 << 192, (1 << 254) - 1 };
    for (out, 0..) |*slot, i| {
        slot.* = if (i < edges.len) edges[i] else random.int(u256) % modulus;
    }
}

test "the modulus is the BLS12-381 scalar field order" {
    // r = x^4 - x^2 + 1 for the BLS parameter x = -0xd201000000010000.
    const x: u256 = 0xd201000000010000;
    try std.testing.expectEqual(x * x * x * x - x * x + 1, modulus);
}

test "an element reads back as the value it was made from" {
    var prng = std.Random.DefaultPrng.init(1);
    var values: [64]u256 = undefined;
    samples(prng.random(), &values);
    for (values) |value| try std.testing.expectEqual(value, valueOf(element(value)));
}

test "values at or above the modulus are not field elements" {
    try std.testing.expect(Fr.fromBytes(&bytesOf(modulus - 1)) != null);
    for ([_]u256{ modulus, modulus + 1, 1 << 255, std.math.maxInt(u256) }) |value| {
        try std.testing.expect(Fr.fromBytes(&bytesOf(value)) == null);
        try std.testing.expect(!Fr.isCanonical(&bytesOf(value)));
    }
    try std.testing.expect(Fr.isCanonical(&bytesOf(modulus - 1)));
}

test "reduction takes any 256-bit value modulo r" {
    var prng = std.Random.DefaultPrng.init(2);
    for ([_]u256{ modulus, modulus + 1, 2 * modulus - 1, 2 * modulus, std.math.maxInt(u256) }) |value| {
        try std.testing.expectEqual(value % modulus, valueOf(Fr.fromBytesReduced(&bytesOf(value))));
    }
    for (0..200) |_| {
        const value = prng.random().int(u256);
        try std.testing.expectEqual(value % modulus, valueOf(Fr.fromBytesReduced(&bytesOf(value))));
    }
}

test "addition, subtraction, negation and multiplication agree with integer arithmetic" {
    var prng = std.Random.DefaultPrng.init(3);
    var values: [40]u256 = undefined;
    samples(prng.random(), &values);
    for (values) |a| {
        for (values) |b| {
            const fa = element(a);
            const fb = element(b);
            try std.testing.expectEqual(@as(u256, @intCast((@as(u512, a) + b) % modulus)), valueOf(fa.add(fb)));
            try std.testing.expectEqual(@as(u256, @intCast((@as(u512, a) + modulus - b) % modulus)), valueOf(fa.sub(fb)));
            try std.testing.expectEqual(mulMod(a, b), valueOf(fa.mul(fb)));
        }
        try std.testing.expectEqual(@as(u256, @intCast((modulus - a) % modulus)), valueOf(element(a).negate()));
        try std.testing.expectEqual(mulMod(a, a), valueOf(element(a).square()));
    }
}

test "small integers convert to the elements with their value" {
    for ([_]u64{ 0, 1, 7, 4096, std.math.maxInt(u64) }) |value| {
        try std.testing.expectEqual(@as(u256, value), valueOf(Fr.fromInt(value)));
    }
}

test "zero and one are the additive and multiplicative identities" {
    try std.testing.expect(Fr.zero.isZero());
    try std.testing.expect(!Fr.one.isZero());
    try std.testing.expectEqual(@as(u256, 1), valueOf(Fr.one));
    const a = element(0x1234567890abcdef);
    try std.testing.expect(a.add(Fr.zero).eql(a));
    try std.testing.expect(a.mul(Fr.one).eql(a));
    try std.testing.expect(a.sub(a).isZero());
}

test "powers agree with integer exponentiation" {
    var prng = std.Random.DefaultPrng.init(4);
    for (0..20) |_| {
        const base = prng.random().int(u256) % modulus;
        const exponent = prng.random().int(u256);
        try std.testing.expectEqual(powMod(base, exponent), valueOf(element(base).pow(exponent)));
    }
    try std.testing.expect(element(5).pow(0).eql(Fr.one));
}

test "the inverse multiplies to one, and zero has none" {
    var prng = std.Random.DefaultPrng.init(5);
    var values: [30]u256 = undefined;
    samples(prng.random(), &values);
    for (values) |value| {
        if (value == 0) continue;
        try std.testing.expect(element(value).mul(element(value).inverse()).eql(Fr.one));
    }
    try std.testing.expect(Fr.zero.inverse().isZero());
}

test "7 generates the 4096th roots of unity the blob domain uses" {
    // 7 is a non-residue, so its power of (r - 1) / 4096 has the full order 4096: it is a root of
    // unity whose 2048th power is -1.
    const root = Fr.fromInt(7).pow((modulus - 1) / 4096);
    try std.testing.expect(root.pow(4096).eql(Fr.one));
    try std.testing.expect(root.pow(2048).eql(Fr.one.negate()));
}
