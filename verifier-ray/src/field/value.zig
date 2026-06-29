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
    for (values) |value| if (!value.isZero()) return false;
    return true;
}

pub fn allZeroExt(values: ExtSlice) bool {
    for (values) |value| if (!value.isZero()) return false;
    return true;
}

// Montgomery batch inversion: invert all nonzero `values` with a single field
// inversion plus ~3 multiplications per element, instead of one inversion each
// (inversion costs ~30x a multiplication, so this is a large win for N > 1).
//
// Let p_i = v_0 * v_1 * ... * v_i be the running product over the nonzero
// values. One inverse gives (p_last)^-1; walking back, each v_i^-1 is recovered
// as (running inverse) * (prefix product before i), then the running inverse is
// multiplied by v_i to drop it. Zero inputs have no inverse: they map to zero in
// `out` and are skipped so they do not collapse the running product.
fn batchInvert(comptime T: type, out: []T, values: []const T) error{LengthMismatch}!void {
    if (out.len != values.len) return error.LengthMismatch;
    if (values.len == 0) return;

    // Forward pass: out[i] holds the product of all nonzero values up to and
    // including i (carrying the previous prefix across zeros unchanged).
    var running = T.one();
    var any_nonzero = false;
    for (values, out) |value, *dst| {
        dst.* = running;
        if (!value.isZero()) {
            running = running.mul(value);
            any_nonzero = true;
        }
    }

    // Single inversion of the full product (skip if every value was zero).
    if (!any_nonzero) {
        for (out) |*dst| dst.* = T.zero();
        return;
    }
    var inv = running.inverse();

    // Backward pass: peel each nonzero value off the running inverse. out[i]
    // already holds the prefix product before i, so v_i^-1 = inv * out[i].
    var i: usize = values.len;
    while (i > 0) {
        i -= 1;
        if (values[i].isZero()) {
            out[i] = T.zero();
        } else {
            out[i] = inv.mul(out[i]);
            inv = inv.mul(values[i]);
        }
    }
}

pub fn batchInvertBase(out: []base.Element, values: []const base.Element) error{LengthMismatch}!void {
    return batchInvert(base.Element, out, values);
}

pub fn batchInvertExt(out: []ext.Ext, values: []const ext.Ext) error{LengthMismatch}!void {
    return batchInvert(ext.Ext, out, values);
}
