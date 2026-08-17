const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub fn evaluateBaseAtBase(coefficients: []const field.Element, point: field.Element) field.Element {
    var acc = field.Element.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(coefficients[i]);
    }
    return acc;
}

pub fn evaluateBaseAtExt(coefficients: []const field.Element, point: ext.Ext) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(ext.Ext.lift(coefficients[i]));
    }
    return acc;
}

pub fn evaluateExtAtBase(coefficients: []const ext.Ext, point: field.Element) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mulByBase(point).add(coefficients[i]);
    }
    return acc;
}

pub fn evaluateExtAtExt(coefficients: []const ext.Ext, point: ext.Ext) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(coefficients[i]);
    }
    return acc;
}
