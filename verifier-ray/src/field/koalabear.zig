pub const bytes: usize = 4;
pub const modulus: u32 = 2_130_706_433;
pub const multiplicative_gen: u32 = 3;
pub const max_order_root: usize = 24;
pub const root_of_unity: u32 = 1_791_270_792;
pub const mont_constant: u32 = 33_554_430;
pub const mont_constant_inv: u32 = 1_057_030_144;

pub const Error = error{NonCanonicalEncoding};

/// KoalaBear base-field element stored as a canonical representative modulo `modulus`.
///
/// This is an `extern struct` because field elements are embedded in verifier
/// inputs that can be cast directly from bytes. The stable C-compatible field
/// layout keeps the native and R5 input representations identical.
pub const Element = extern struct {
    value: u32,

    pub fn init(raw: u64) Element {
        return .{ .value = @as(u32, @intCast(raw % modulus)) };
    }

    pub fn zero() Element {
        return .{ .value = 0 };
    }

    pub fn one() Element {
        return .{ .value = 1 };
    }

    pub fn fromCanonical(value: u32) Error!Element {
        if (value >= modulus) return Error.NonCanonicalEncoding;
        return .{ .value = value };
    }

    pub fn fromBytesCanonical(encoded: [bytes]u8) Error!Element {
        return fromCanonical(readU32BigEndian(encoded));
    }

    pub fn fromBytesCanonicalSlice(encoded: []const u8) Error!Element {
        if (encoded.len != bytes) return Error.NonCanonicalEncoding;
        return fromBytesCanonical(.{ encoded[0], encoded[1], encoded[2], encoded[3] });
    }

    pub fn fromBytesWide(encoded: []const u8) Element {
        var acc: u64 = 0;
        for (encoded) |byte| {
            acc = ((acc << 8) + byte) % modulus;
        }
        return init(acc);
    }

    pub fn toBytes(self: Element) [bytes]u8 {
        return writeU32BigEndian(self.value);
    }

    pub fn eql(self: Element, rhs: Element) bool {
        return self.value == rhs.value;
    }

    pub fn isZero(self: Element) bool {
        return self.value == 0;
    }

    pub fn add(self: Element, rhs: Element) Element {
        return .{ .value = (self.value + rhs.value) % modulus };
    }

    pub fn sub(self: Element, rhs: Element) Element {
        return .{ .value = (self.value + modulus - rhs.value) % modulus };
    }

    pub fn neg(self: Element) Element {
        if (self.isZero()) return self;
        return .{ .value = modulus - self.value };
    }

    pub fn double(self: Element) Element {
        return self.add(self);
    }

    pub fn halve(self: Element) Element {
        if ((self.value & 1) == 0) return .{ .value = self.value >> 1 };
        return .{ .value = @as(u32, @intCast((@as(u64, self.value) + modulus) >> 1)) };
    }

    pub fn mul(self: Element, rhs: Element) Element {
        return init(@as(u64, self.value) * @as(u64, rhs.value));
    }

    pub fn square(self: Element) Element {
        return self.mul(self);
    }

    // The exponent is u64, not u32. On the rv64 target a u32 loop variable makes
    // LLVM re-narrow `exp` every iteration: `exp >>= 1` lowers to `srliw` (the
    // 32-bit word shift) and the `exp != 0` test needs a `sext.w` before it, so
    // the loop carries two extra instructions per bit. A u64 exponent lowers the
    // shift to a plain `srli` with no per-iteration `sext.w`. u64 is also the
    // natural width: the exponents are domain sizes / positions (usize at the
    // call sites), so it avoids a narrowing @intCast. The values, bounded by
    // max_order_root, fit in u32 too.
    pub fn pow(self: Element, exponent: u64) Element {
        var result = Element.one();
        var base = self;
        var exp = exponent;
        while (exp != 0) : (exp >>= 1) {
            if ((exp & 1) == 1) {
                result = result.mul(base);
            }
            base = base.square();
        }
        return result;
    }

    pub fn powComptime(self: Element, comptime exponent: usize) Element {
        var result = Element.one();
        var b = self;
        comptime var exp = exponent;
        inline while (exp != 0) : (exp >>= 1) {
            if ((exp & 1) == 1) result = result.mul(b);
            b = b.square();
        }
        return result;
    }

    pub fn inverse(self: Element) Element {
        if (self.isZero()) unreachable;
        // Fermat: x^(p-2) = x^-1. For KoalaBear p = 2^31 - 2^24 + 1, so
        //   p - 2 = 0x7EFFFFFF = bits 0..23 set, bit 24 clear, bits 25..30 set,
        // i.e. a run of 24 ones, a gap, then a run of 6 ones:
        //   p - 2 = (2^24 - 1) + (2^6 - 1) * 2^25.
        // We build the two all-ones runs from the doubling identity
        //   x^(2^(j+k) - 1) = (x^(2^j - 1))^(2^k) * x^(2^k - 1)
        // doubling the run length each step (2 -> 4 -> 6 -> 12 -> 24) so the
        // 6-ones block is reused to grow the 24-ones block, then combine the two
        // runs with one final shift+multiply. 48 squarings + 6 multiplications.
        const b1 = self; // x^(2^1  - 1)
        const b2 = sqn(b1, 1).mul(b1); // x^(2^2  - 1)
        const b4 = sqn(b2, 2).mul(b2); // x^(2^4  - 1)
        const b6 = sqn(b4, 2).mul(b2); // x^(2^6  - 1)  (top run, bits 25..30)
        const b12 = sqn(b6, 6).mul(b6); // x^(2^12 - 1)
        const b24 = sqn(b12, 12).mul(b12); // x^(2^24 - 1)  (low run, bits 0..23)
        // x^(p-2) = (x^(2^6 - 1))^(2^25) * x^(2^24 - 1)
        return sqn(b6, 25).mul(b24);
    }

    // Square self n times.
    fn sqn(self: Element, comptime n: usize) Element {
        var t = self;
        inline for (0..n) |_| t = t.square();
        return t;
    }

    pub fn div(self: Element, rhs: Element) Element {
        return self.mul(rhs.inverse());
    }

    pub fn mul2ExpNegN(self: Element, n: u32) Element {
        if (n > 32) unreachable;
        var result = self;
        var i: u32 = 0;
        while (i < n) : (i += 1) {
            result = result.halve();
        }
        return result;
    }
};

pub fn zero() Element {
    return Element.zero();
}

pub fn one() Element {
    return Element.one();
}

pub fn rootOfUnityBy(cardinality: usize) Error!Element {
    if (!isPowerOfTwo(cardinality)) {
        return Error.NonCanonicalEncoding;
    }
    const log_n = log2PowerOfTwo(cardinality);
    if (log_n > max_order_root) return Error.NonCanonicalEncoding;

    var result = Element.init(root_of_unity);
    var i: usize = log_n;
    while (i < max_order_root) : (i += 1) {
        result = result.square();
    }
    return result;
}

pub fn isPowerOfTwo(value: usize) bool {
    return value != 0 and (value & (value - 1)) == 0;
}

pub fn log2PowerOfTwo(value: usize) usize {
    if (!isPowerOfTwo(value)) unreachable;
    var n = value;
    var result: usize = 0;
    while (n > 1) : (n >>= 1) {
        result += 1;
    }
    return result;
}

fn readU32BigEndian(encoded: [bytes]u8) u32 {
    return @as(u32, encoded[3]) |
        (@as(u32, encoded[2]) << 8) |
        (@as(u32, encoded[1]) << 16) |
        (@as(u32, encoded[0]) << 24);
}

fn writeU32BigEndian(value: u32) [bytes]u8 {
    return .{
        @as(u8, @intCast((value >> 24) & 0xff)),
        @as(u8, @intCast((value >> 16) & 0xff)),
        @as(u8, @intCast((value >> 8) & 0xff)),
        @as(u8, @intCast(value & 0xff)),
    };
}
