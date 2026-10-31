//! Arithmetic in the BLS12-381 scalar field, the field of EIP-4844 blob elements. Elements are held
//! in Montgomery form over four 64-bit limbs, always fully reduced.

const std = @import("std");

/// r = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001.
pub const modulus: u256 = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001;

const Limbs = [4]u64;

const modulus_limbs = limbsOf(modulus);

/// -r^-1 mod 2^64.
const inverse_negated: u64 = blk: {
    var inverse: u64 = 1;
    for (0..6) |_| inverse *%= 2 -% modulus_limbs[0] *% inverse;
    break :blk 0 -% inverse;
};

/// 2^256 mod r: the Montgomery form of one.
const one_limbs = limbsOf(@intCast((@as(u512, 1) << 256) % modulus));

/// 2^512 mod r: multiplying by it converts into Montgomery form.
const to_montgomery_limbs = blk: {
    const one: u512 = (@as(u512, 1) << 256) % modulus;
    break :blk limbsOf(@intCast((one * one) % modulus));
};

fn limbsOf(value: u256) Limbs {
    return .{ @truncate(value), @truncate(value >> 64), @truncate(value >> 128), @truncate(value >> 192) };
}

fn lessThanModulus(limbs: Limbs) bool {
    var i: usize = 4;
    while (i > 0) {
        i -= 1;
        if (limbs[i] != modulus_limbs[i]) return limbs[i] < modulus_limbs[i];
    }
    return false;
}

fn subtractModulus(limbs: Limbs) Limbs {
    var out: Limbs = undefined;
    var borrow: u1 = 0;
    inline for (0..4) |i| {
        const first = @subWithOverflow(limbs[i], modulus_limbs[i]);
        const second = @subWithOverflow(first[0], borrow);
        out[i] = second[0];
        borrow = first[1] | second[1];
    }
    return out;
}

fn readBigEndian(bytes: *const [32]u8) Limbs {
    return .{
        std.mem.readInt(u64, bytes[24..32], .big),
        std.mem.readInt(u64, bytes[16..24], .big),
        std.mem.readInt(u64, bytes[8..16], .big),
        std.mem.readInt(u64, bytes[0..8], .big),
    };
}

/// Montgomery product a * b / 2^256 mod r, for operands below r.
fn montgomeryMultiply(a: Limbs, b: Limbs) Limbs {
    var t: [5]u64 = @splat(0);
    inline for (0..4) |i| {
        var carry: u64 = 0;
        inline for (0..4) |j| {
            const wide = @as(u128, a[j]) * b[i] + t[j] + carry;
            t[j] = @truncate(wide);
            carry = @truncate(wide >> 64);
        }
        t[4] = carry;

        const factor = t[0] *% inverse_negated;
        carry = @truncate((@as(u128, factor) * modulus_limbs[0] + t[0]) >> 64);
        inline for (1..4) |j| {
            const wide = @as(u128, factor) * modulus_limbs[j] + t[j] + carry;
            t[j - 1] = @truncate(wide);
            carry = @truncate(wide >> 64);
        }
        t[3] = t[4] +% carry;
        t[4] = 0;
    }
    const result: Limbs = .{ t[0], t[1], t[2], t[3] };
    return if (lessThanModulus(result)) result else subtractModulus(result);
}

pub const Fr = struct {
    limbs: Limbs,

    pub const zero: Fr = .{ .limbs = @splat(0) };
    pub const one: Fr = .{ .limbs = one_limbs };

    /// The element with the canonical big-endian encoding `bytes`, or null when `bytes` encodes a
    /// value of at least the modulus.
    pub fn fromBytes(bytes: *const [32]u8) ?Fr {
        const limbs = readBigEndian(bytes);
        if (!lessThanModulus(limbs)) return null;
        return .{ .limbs = montgomeryMultiply(limbs, to_montgomery_limbs) };
    }

    /// Whether `bytes` is the big-endian encoding of a value below the modulus.
    pub fn isCanonical(bytes: *const [32]u8) bool {
        return lessThanModulus(readBigEndian(bytes));
    }

    /// The big-endian integer `bytes` reduced modulo r.
    pub fn fromBytesReduced(bytes: *const [32]u8) Fr {
        var limbs = readBigEndian(bytes);
        // 2^256 < 3r, so two subtractions reduce any 256-bit value.
        while (!lessThanModulus(limbs)) limbs = subtractModulus(limbs);
        return .{ .limbs = montgomeryMultiply(limbs, to_montgomery_limbs) };
    }

    pub fn fromInt(value: u64) Fr {
        return .{ .limbs = montgomeryMultiply(.{ value, 0, 0, 0 }, to_montgomery_limbs) };
    }

    /// The canonical big-endian encoding.
    pub fn toBytes(self: Fr) [32]u8 {
        const limbs = montgomeryMultiply(self.limbs, .{ 1, 0, 0, 0 });
        var bytes: [32]u8 = undefined;
        std.mem.writeInt(u64, bytes[0..8], limbs[3], .big);
        std.mem.writeInt(u64, bytes[8..16], limbs[2], .big);
        std.mem.writeInt(u64, bytes[16..24], limbs[1], .big);
        std.mem.writeInt(u64, bytes[24..32], limbs[0], .big);
        return bytes;
    }

    pub fn isZero(self: Fr) bool {
        return (self.limbs[0] | self.limbs[1] | self.limbs[2] | self.limbs[3]) == 0;
    }

    pub fn add(self: Fr, other: Fr) Fr {
        var sum: Limbs = undefined;
        var carry: u1 = 0;
        inline for (0..4) |i| {
            const first = @addWithOverflow(self.limbs[i], other.limbs[i]);
            const second = @addWithOverflow(first[0], carry);
            sum[i] = second[0];
            carry = first[1] | second[1];
        }
        return .{ .limbs = if (lessThanModulus(sum)) sum else subtractModulus(sum) };
    }

    pub fn sub(self: Fr, other: Fr) Fr {
        var difference: Limbs = undefined;
        var borrow: u1 = 0;
        inline for (0..4) |i| {
            const first = @subWithOverflow(self.limbs[i], other.limbs[i]);
            const second = @subWithOverflow(first[0], borrow);
            difference[i] = second[0];
            borrow = first[1] | second[1];
        }
        if (borrow == 0) return .{ .limbs = difference };
        var carry: u1 = 0;
        inline for (0..4) |i| {
            const first = @addWithOverflow(difference[i], modulus_limbs[i]);
            const second = @addWithOverflow(first[0], carry);
            difference[i] = second[0];
            carry = first[1] | second[1];
        }
        return .{ .limbs = difference };
    }

    pub fn mul(self: Fr, other: Fr) Fr {
        return .{ .limbs = montgomeryMultiply(self.limbs, other.limbs) };
    }

    pub fn square(self: Fr) Fr {
        return self.mul(self);
    }

    /// self^exponent.
    pub fn pow(self: Fr, exponent: u256) Fr {
        var result = one;
        var bit: usize = 256;
        while (bit > 0) {
            bit -= 1;
            result = result.square();
            if ((exponent >> @intCast(bit)) & 1 == 1) result = result.mul(self);
        }
        return result;
    }

    /// The multiplicative inverse; zero for zero.
    pub fn inverse(self: Fr) Fr {
        return self.pow(modulus - 2);
    }
};
