const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const fieldvalue = @import("../field/value.zig");

pub const Error = field.Error || error{InvalidCardinality};

// Each evaluate* function has a runtime-domain form (the slice length gives n,
// so the vanishing-polynomial exponentiation uses the runtime pow) and a
// *Comptime form (n is comptime-known from the array length, so the vanishing
// exponentiation uses powComptime). For power-of-two domains powComptime emits
// an unrolled squaring chain with no runtime loop/branch/shift, which is ~3-4x
// cheaper than runtime pow (see bench/field_op_bench). Call sites whose domain
// size is statically known should prefer the *Comptime form; this mirrors the
// static/dynamic split in query/vanishing.zig's powModuleSize.
//
// The *Comptime forms additionally batch the per-point denominator inversions
// (point - omega_i)^-1 with Montgomery's trick: comptime n lets them stack-
// allocate an [n] scratch buffer, so n inversions collapse to one inversion plus
// ~3n multiplications. Since inversion costs ~30x a multiplication, this is a
// large win for the verifier's Lagrange-evaluation hot path. The runtime forms
// cannot batch (no allocator on the freestanding R5 target, and n can reach
// 2^24, so no fixed stack buffer) and keep the per-element inverse.

pub fn evaluateBaseAtBase(values: []const field.Element, point: field.Element) Error!field.Element {
    if (values.len == 0) return field.Element.zero();
    const n = try checkedCardinality(values.len);
    const vanishing = point.pow(n).sub(field.Element.one());
    return baseAtBase(values, point, n, vanishing);
}

pub fn evaluateBaseAtBaseComptime(
    comptime n: u32,
    values: *const [n]field.Element,
    point: field.Element,
) Error!field.Element {
    comptime checkComptimeCardinality(n);
    const vanishing = point.powComptime(n).sub(field.Element.one());
    const inv_n = field.Element.init(n).inverse();
    const omega = try field.rootOfUnityBy(n);

    // Collect denominators (point - omega^i). If point lands on a domain point
    // omega^i the polynomial interpolates to values[i] there, matching the
    // per-element form's early return.
    var denom: [n]field.Element = undefined;
    var omega_i = field.Element.one();
    for (0..n) |i| {
        if (point.eql(omega_i)) return values[i];
        denom[i] = point.sub(omega_i);
        omega_i = omega_i.mul(omega);
    }

    // One inversion for all n denominators.
    var inv_denom: [n]field.Element = undefined;
    fieldvalue.batchInvertBase(&inv_denom, &denom) catch unreachable;

    var sum = field.Element.zero();
    omega_i = field.Element.one();
    for (0..n) |i| {
        const weighted = omega_i.mul(inv_n).mul(values[i]);
        sum = sum.add(weighted.mul(inv_denom[i]));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

fn baseAtBase(
    values: []const field.Element,
    point: field.Element,
    n: u32,
    vanishing: field.Element,
) Error!field.Element {
    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();

    var omega_i = field.Element.one();
    var sum = field.Element.zero();
    for (values) |value| {
        if (point.eql(omega_i)) return value;
        const weighted = omega_i.mul(inv_n).mul(value);
        const inv_denom = point.sub(omega_i).inverse();
        sum = sum.add(weighted.mul(inv_denom));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

pub fn evaluateBaseAtExt(values: []const field.Element, point: ext.Ext) Error!ext.Ext {
    if (values.len == 0) return ext.Ext.zero();
    const n = try checkedCardinality(values.len);
    const vanishing = point.pow(n).sub(ext.Ext.one());
    return baseAtExt(values, point, n, vanishing);
}

pub fn evaluateBaseAtExtComptime(
    comptime n: u32,
    values: *const [n]field.Element,
    point: ext.Ext,
) Error!ext.Ext {
    comptime checkComptimeCardinality(n);
    const vanishing = point.powComptime(n).sub(ext.Ext.one());
    const inv_n = field.Element.init(n).inverse();
    const omega = try field.rootOfUnityBy(n);

    var denom: [n]ext.Ext = undefined;
    var omega_i = field.Element.one();
    for (0..n) |i| {
        const domain_point = ext.Ext.lift(omega_i);
        if (point.eql(domain_point)) return ext.Ext.lift(values[i]);
        denom[i] = point.sub(domain_point);
        omega_i = omega_i.mul(omega);
    }

    var inv_denom: [n]ext.Ext = undefined;
    fieldvalue.batchInvertExt(&inv_denom, &denom) catch unreachable;

    var sum = ext.Ext.zero();
    omega_i = field.Element.one();
    for (0..n) |i| {
        const weighted = omega_i.mul(inv_n).mul(values[i]);
        sum = sum.add(inv_denom[i].mulByBase(weighted));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

fn baseAtExt(
    values: []const field.Element,
    point: ext.Ext,
    n: u32,
    vanishing: ext.Ext,
) Error!ext.Ext {
    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();

    var omega_i = field.Element.one();
    var sum = ext.Ext.zero();
    for (values) |value| {
        const domain_point = ext.Ext.lift(omega_i);
        if (point.eql(domain_point)) return ext.Ext.lift(value);
        const weighted = omega_i.mul(inv_n).mul(value);
        const denom = point.sub(domain_point);
        sum = sum.add(denom.inverse().mulByBase(weighted));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

pub fn evaluateExtAtBase(values: []const ext.Ext, point: field.Element) Error!ext.Ext {
    if (values.len == 0) return ext.Ext.zero();
    const n = try checkedCardinality(values.len);
    const vanishing = point.pow(n).sub(field.Element.one());
    return extAtBase(values, point, n, vanishing);
}

pub fn evaluateExtAtBaseComptime(
    comptime n: u32,
    values: *const [n]ext.Ext,
    point: field.Element,
) Error!ext.Ext {
    comptime checkComptimeCardinality(n);
    const vanishing = point.powComptime(n).sub(field.Element.one());
    const inv_n = field.Element.init(n).inverse();
    const omega = try field.rootOfUnityBy(n);

    var denom: [n]field.Element = undefined;
    var omega_i = field.Element.one();
    for (0..n) |i| {
        if (point.eql(omega_i)) return values[i];
        denom[i] = point.sub(omega_i);
        omega_i = omega_i.mul(omega);
    }

    var inv_denom: [n]field.Element = undefined;
    fieldvalue.batchInvertBase(&inv_denom, &denom) catch unreachable;

    var sum = ext.Ext.zero();
    omega_i = field.Element.one();
    for (0..n) |i| {
        const weighted = values[i].mulByBase(omega_i.mul(inv_n));
        sum = sum.add(weighted.mulByBase(inv_denom[i]));
        omega_i = omega_i.mul(omega);
    }

    return sum.mulByBase(vanishing);
}

fn extAtBase(
    values: []const ext.Ext,
    point: field.Element,
    n: u32,
    vanishing: field.Element,
) Error!ext.Ext {
    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();

    var omega_i = field.Element.one();
    var sum = ext.Ext.zero();
    for (values) |value| {
        if (point.eql(omega_i)) return value;
        const weighted = value.mulByBase(omega_i.mul(inv_n));
        const inv_denom = point.sub(omega_i).inverse();
        sum = sum.add(weighted.mulByBase(inv_denom));
        omega_i = omega_i.mul(omega);
    }

    return sum.mulByBase(vanishing);
}

pub fn evaluateExtAtExt(values: []const ext.Ext, point: ext.Ext) Error!ext.Ext {
    if (values.len == 0) return ext.Ext.zero();
    const n = try checkedCardinality(values.len);
    const vanishing = point.pow(n).sub(ext.Ext.one());
    return extAtExt(values, point, n, vanishing);
}

pub fn evaluateExtAtExtComptime(
    comptime n: u32,
    values: *const [n]ext.Ext,
    point: ext.Ext,
) Error!ext.Ext {
    comptime checkComptimeCardinality(n);
    const vanishing = point.powComptime(n).sub(ext.Ext.one());
    const inv_n = field.Element.init(n).inverse();
    const omega = try field.rootOfUnityBy(n);

    var denom: [n]ext.Ext = undefined;
    var omega_i = field.Element.one();
    for (0..n) |i| {
        const domain_point = ext.Ext.lift(omega_i);
        if (point.eql(domain_point)) return values[i];
        denom[i] = point.sub(domain_point);
        omega_i = omega_i.mul(omega);
    }

    var inv_denom: [n]ext.Ext = undefined;
    fieldvalue.batchInvertExt(&inv_denom, &denom) catch unreachable;

    var sum = ext.Ext.zero();
    omega_i = field.Element.one();
    for (0..n) |i| {
        const weighted = values[i].mulByBase(omega_i.mul(inv_n));
        sum = sum.add(weighted.mul(inv_denom[i]));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

fn extAtExt(
    values: []const ext.Ext,
    point: ext.Ext,
    n: u32,
    vanishing: ext.Ext,
) Error!ext.Ext {
    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();

    var omega_i = field.Element.one();
    var sum = ext.Ext.zero();
    for (values) |value| {
        const domain_point = ext.Ext.lift(omega_i);
        if (point.eql(domain_point)) return value;
        const weighted = value.mulByBase(omega_i.mul(inv_n));
        const denom = point.sub(domain_point);
        sum = sum.add(weighted.mul(denom.inverse()));
        omega_i = omega_i.mul(omega);
    }

    return vanishing.mul(sum);
}

// checkComptimeCardinality is the comptime analogue of checkedCardinality: it
// validates a comptime domain size at compile time, turning an invalid size
// into a compile error rather than a runtime error.
fn checkComptimeCardinality(comptime n: u32) void {
    if (!field.isPowerOfTwo(n)) {
        @compileError("Lagrange domain size must be a power of two");
    }
    if (field.log2PowerOfTwo(n) > field.max_order_root) {
        @compileError("Lagrange domain size exceeds supported KoalaBear root-of-unity order");
    }
}

fn checkedCardinality(cardinality: usize) Error!u32 {
    if (!field.isPowerOfTwo(cardinality)) {
        return error.InvalidCardinality;
    }
    if (field.log2PowerOfTwo(cardinality) > field.max_order_root) {
        return error.InvalidCardinality;
    }
    // Koalabear domains are bounded by max_order_root, so the validated length
    // always fits in u32; truncate keeps the field-sized conversion explicit.
    return @truncate(cardinality);
}
