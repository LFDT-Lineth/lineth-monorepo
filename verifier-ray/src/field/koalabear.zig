pub const bytes: usize = 4;
pub const modulus: u32 = 2_130_706_433;
pub const multiplicative_gen: u32 = 3;
pub const max_order_root: usize = 24;
pub const root_of_unity: u32 = 1_791_270_792;
pub const mont_constant: u32 = 33_554_430;
pub const mont_constant_inv: u32 = 1_057_030_144;

pub const Error = error{NonCanonicalEncoding};

pub const Element = struct {
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
        const sum = @as(u64, self.value) + @as(u64, rhs.value);
        if (sum >= modulus) return .{ .value = @as(u32, @intCast(sum - modulus)) };
        return .{ .value = @as(u32, @intCast(sum)) };
    }

    pub fn sub(self: Element, rhs: Element) Element {
        if (self.value >= rhs.value) {
            return .{ .value = self.value - rhs.value };
        }
        return .{ .value = modulus - (rhs.value - self.value) };
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

    pub fn inverse(self: Element) Element {
        if (self.isZero()) unreachable;
        return self.pow(modulus - 2);
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
