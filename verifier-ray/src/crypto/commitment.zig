const field = @import("../field/koalabear.zig");

pub const Commitment = [8]field.Element;

/// Flattening order: elements [0..8] in index order.
pub fn fromUints(v: [8]u32) Commitment {
    var out: Commitment = undefined;
    for (&out, v) |*dst, value| dst.* = field.Element.init(value);
    return out;
}

/// Flattening order: elements [0..8] in index order.
pub fn toUints(c: Commitment) [8]u32 {
    var out: [8]u32 = undefined;
    for (&out, c) |*dst, elem| dst.* = elem.value;
    return out;
}
