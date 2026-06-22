const base = @import("koalabear.zig");
const ext = @import("koalabear_ext.zig");

pub const ElementSlice = []const base.Element;
pub const ExtSlice = []const ext.Ext;

pub const Vector = union(enum) {
    base: ElementSlice,
    ext: ExtSlice,
};

pub const Scalar = union(enum) {
    base: base.Element,
    ext: ext.Ext,

    // TODO: consider short-circuiting when all scalars are base elements to avoid lifting overhead.
    pub fn toExt(self: Scalar) ext.Ext {
        return switch (self) {
            .base => |b| ext.Ext.lift(b),
            .ext => |e| e,
        };
    }
};

pub fn ElementArray(comptime len: usize) type {
    return [len]base.Element;
}

pub fn ExtArray(comptime len: usize) type {
    return [len]ext.Ext;
}

pub fn allZero(values: ElementSlice) bool {
    for (values) |value| {
        if (!value.isZero()) return false;
    }
    return true;
}

pub fn allZeroExt(values: ExtSlice) bool {
    for (values) |value| {
        if (!value.isZero()) return false;
    }
    return true;
}

pub fn batchInvertBase(out: []base.Element, values: []const base.Element) error{LengthMismatch}!void {
    if (out.len != values.len) return error.LengthMismatch;
    for (values, out) |value, *dst| {
        dst.* = if (value.isZero()) base.Element.zero() else value.inverse();
    }
}

pub fn batchInvertExt(out: []ext.Ext, values: []const ext.Ext) error{LengthMismatch}!void {
    if (out.len != values.len) return error.LengthMismatch;
    for (values, out) |value, *dst| {
        dst.* = if (value.isZero()) ext.Ext.zero() else value.inverse();
    }
}
