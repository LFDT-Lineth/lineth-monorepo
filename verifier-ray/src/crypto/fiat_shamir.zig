const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const field_value = @import("../field/value.zig");
const poseidon2 = @import("poseidon2.zig");

pub const Transcript = struct {
    hasher: poseidon2.MDHasher,

    pub fn init() Transcript {
        return .{ .hasher = poseidon2.MDHasher.init() };
    }

    pub fn updateElement(self: *Transcript, value: field.Element) void {
        self.hasher.writeElement(value);
    }

    pub fn updateElements(self: *Transcript, values: field_value.ElementSlice) void {
        self.hasher.writeElements(values);
    }

    pub fn updateExt(self: *Transcript, values: field_value.ExtSlice) void {
        for (values) |ext_value| {
            self.hasher.writeElements(&.{ ext_value.B0.a0, ext_value.B0.a1, ext_value.B1.a0, ext_value.B1.a1, ext_value.B2.a0, ext_value.B2.a1 });
        }
    }

    pub fn absorbVector(self: *Transcript, vector: field_value.Vector) void {
        switch (vector) {
            .base => |values| self.updateElements(values),
            .ext => |values| self.updateExt(values),
        }
    }

    pub fn absorbScalar(self: *Transcript, scalar: field_value.Scalar) void {
        switch (scalar) {
            .base => |scalar_value| self.updateElement(scalar_value),
            .ext => |scalar_value| self.updateExt(&.{scalar_value}),
        }
    }

    pub fn randomDigest(self: *Transcript) poseidon2.Digest {
        const challenge = self.hasher.sumDigest();
        self.updateElement(field.Element.zero());
        return challenge;
    }

    pub fn randomExt(self: *Transcript) ext.Ext {
        const challenge = self.randomDigest();
        return .{
            .B0 = .{ .a0 = challenge[0], .a1 = challenge[1] },
            .B1 = .{ .a0 = challenge[2], .a1 = challenge[3] },
            .B2 = .{ .a0 = challenge[4], .a1 = challenge[5] },
        };
    }

    pub fn compressionCount(self: *const Transcript) usize {
        return self.hasher.compression_count;
    }
};
