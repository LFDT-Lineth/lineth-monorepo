//! The scalar field of BLS12-381 by plain integer arithmetic, written without the package under test.

/// r = x^4 - x^2 + 1 for the BLS parameter x = -0xd201000000010000.
pub const modulus: u256 = blk: {
    const x: u256 = 0xd201000000010000;
    break :blk x * x * x * x - x * x + 1;
};

pub fn mulMod(a: u256, b: u256) u256 {
    return @intCast(@as(u512, a) * b % modulus);
}

pub fn powMod(base: u256, exponent: u256) u256 {
    var result: u256 = 1;
    var power = base;
    var rest = exponent;
    while (rest > 0) : (rest >>= 1) {
        if (rest & 1 == 1) result = mulMod(result, power);
        power = mulMod(power, power);
    }
    return result;
}
