const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = field.Error || error{InvalidCardinality};

pub fn evaluateBaseAtBase(values: []const field.Element, point: field.Element) Error!field.Element {
    if (values.len == 0) return field.Element.zero();
    const n = try checkedCardinality(values.len);

    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();
    const vanishing = point.pow(n).sub(field.Element.one());

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

    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();
    const vanishing = point.pow(n).sub(ext.Ext.one());

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

    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();
    const vanishing = point.pow(n).sub(field.Element.one());

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

    const omega = try field.rootOfUnityBy(values.len);
    const inv_n = field.Element.init(n).inverse();
    const vanishing = point.pow(n).sub(ext.Ext.one());

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
