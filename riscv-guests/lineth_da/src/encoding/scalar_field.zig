//! The scalar field of BLS12-381, the field of EIP-4844 blob elements, over Constantine's
//! `ctt_bls12_381_fr_*` functions, which the archive of the guest crypto providers defines. An
//! element is a `bls12_381_fr`: Constantine's fully reduced Montgomery form over machine words.

const std = @import("std");

/// r = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001.
pub const modulus: u256 = 0x73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001;

extern fn ctt_bls12_381_fr_unmarshalBE(dst: *Fr, src: [*]const u8, src_len: usize) bool;
extern fn ctt_bls12_381_fr_marshalBE(dst: [*]u8, dst_len: usize, src: *const Fr) bool;
extern fn ctt_bls12_381_fr_is_zero(a: *const Fr) usize;
extern fn ctt_bls12_381_fr_set_zero(a: *Fr) void;
extern fn ctt_bls12_381_fr_set_one(a: *Fr) void;
extern fn ctt_bls12_381_fr_sum(r: *Fr, a: *const Fr, b: *const Fr) void;
extern fn ctt_bls12_381_fr_diff(r: *Fr, a: *const Fr, b: *const Fr) void;
extern fn ctt_bls12_381_fr_prod(r: *Fr, a: *const Fr, b: *const Fr) void;
extern fn ctt_bls12_381_fr_square_in_place(a: *Fr) void;
extern fn ctt_bls12_381_fr_inv(r: *Fr, a: *const Fr) void;

pub const Fr = extern struct {
    /// `CTT_WORDS_REQUIRED(255)` machine words.
    limbs: [(255 + @bitSizeOf(usize) - 1) / @bitSizeOf(usize)]usize,

    pub fn zero() Fr {
        var element: Fr = undefined;
        ctt_bls12_381_fr_set_zero(&element);
        return element;
    }

    pub fn one() Fr {
        var element: Fr = undefined;
        ctt_bls12_381_fr_set_one(&element);
        return element;
    }

    /// The element with the canonical big-endian encoding `bytes`, or null when `bytes` encodes a
    /// value of at least the modulus.
    pub fn fromBytes(bytes: *const [32]u8) ?Fr {
        var element: Fr = undefined;
        if (!ctt_bls12_381_fr_unmarshalBE(&element, bytes, bytes.len)) return null;
        return element;
    }

    /// Whether `bytes` is the big-endian encoding of a value below the modulus.
    pub fn isCanonical(bytes: *const [32]u8) bool {
        return std.mem.readInt(u256, bytes, .big) < modulus;
    }

    /// The big-endian integer `bytes` reduced modulo r.
    pub fn fromBytesReduced(bytes: *const [32]u8) Fr {
        // 2^256 < 3r, so two subtractions reduce any 256-bit value.
        var value = std.mem.readInt(u256, bytes, .big);
        while (value >= modulus) value -= modulus;
        var reduced: [32]u8 = undefined;
        std.mem.writeInt(u256, &reduced, value, .big);
        return fromBytes(&reduced).?;
    }

    /// The canonical big-endian encoding.
    pub fn toBytes(self: Fr) [32]u8 {
        var bytes: [32]u8 = undefined;
        const marshalled = ctt_bls12_381_fr_marshalBE(&bytes, bytes.len, &self);
        std.debug.assert(marshalled);
        return bytes;
    }

    pub fn isZero(self: Fr) bool {
        return ctt_bls12_381_fr_is_zero(&self) != 0;
    }

    pub fn add(self: Fr, other: Fr) Fr {
        var sum: Fr = undefined;
        ctt_bls12_381_fr_sum(&sum, &self, &other);
        return sum;
    }

    pub fn sub(self: Fr, other: Fr) Fr {
        var difference: Fr = undefined;
        ctt_bls12_381_fr_diff(&difference, &self, &other);
        return difference;
    }

    pub fn mul(self: Fr, other: Fr) Fr {
        var product: Fr = undefined;
        ctt_bls12_381_fr_prod(&product, &self, &other);
        return product;
    }

    pub fn squareInPlace(self: *Fr) void {
        ctt_bls12_381_fr_square_in_place(self);
    }

    /// The multiplicative inverse; zero for zero.
    pub fn inverse(self: Fr) Fr {
        var inverted: Fr = undefined;
        ctt_bls12_381_fr_inv(&inverted, &self);
        return inverted;
    }
};
