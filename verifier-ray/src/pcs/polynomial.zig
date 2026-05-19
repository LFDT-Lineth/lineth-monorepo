const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{LengthMismatch};

pub fn evaluateBaseCanonical(coefficients: []const field.Element, point: field.Element) field.Element {
    var acc = field.Element.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(coefficients[i]);
    }
    return acc;
}

pub fn evaluateBaseCanonicalAtExt(coefficients: []const field.Element, point: ext.Ext) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(ext.Ext.lift(coefficients[i]));
    }
    return acc;
}

pub fn evaluateExtCanonicalAtBase(coefficients: []const ext.Ext, point: field.Element) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mulByBase(point).add(coefficients[i]);
    }
    return acc;
}

pub fn evaluateExtCanonical(coefficients: []const ext.Ext, point: ext.Ext) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coefficients.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(point).add(coefficients[i]);
    }
    return acc;
}

pub fn evaluateBaseCanonicalBatch(
    out: []field.Element,
    polys: []const []const field.Element,
    point: field.Element,
) Error!void {
    if (out.len != polys.len) return Error.LengthMismatch;
    for (polys, out) |poly, *dst| {
        dst.* = evaluateBaseCanonical(poly, point);
    }
}

pub fn evaluateBaseCanonicalBatchAtExt(
    out: []ext.Ext,
    polys: []const []const field.Element,
    point: ext.Ext,
) Error!void {
    if (out.len != polys.len) return Error.LengthMismatch;
    for (polys, out) |poly, *dst| {
        dst.* = evaluateBaseCanonicalAtExt(poly, point);
    }
}

pub fn evaluateExtCanonicalBatchAtBase(
    out: []ext.Ext,
    polys: []const []const ext.Ext,
    point: field.Element,
) Error!void {
    if (out.len != polys.len) return Error.LengthMismatch;
    for (polys, out) |poly, *dst| {
        dst.* = evaluateExtCanonicalAtBase(poly, point);
    }
}

pub fn evaluateExtCanonicalBatch(
    out: []ext.Ext,
    polys: []const []const ext.Ext,
    point: ext.Ext,
) Error!void {
    if (out.len != polys.len) return Error.LengthMismatch;
    for (polys, out) |poly, *dst| {
        dst.* = evaluateExtCanonical(poly, point);
    }
}

pub const evaluateHorner = evaluateBaseCanonical;
