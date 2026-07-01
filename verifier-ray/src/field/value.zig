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
