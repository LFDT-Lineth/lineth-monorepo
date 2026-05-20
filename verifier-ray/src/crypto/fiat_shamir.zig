const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("poseidon2.zig");

pub const Transcript = struct {
    hasher: poseidon2.MDHasher,

    pub fn init() Transcript {
        return .{ .hasher = poseidon2.MDHasher.init() };
    }

    pub fn updateElement(self: *Transcript, value: field.Element) void {
        self.hasher.writeElement(value);
    }

    pub fn updateElements(self: *Transcript, values: []const field.Element) void {
        self.hasher.writeElements(values);
    }

    pub fn updateExt(self: *Transcript, values: []const ext.Ext) void {
        for (values) |value| {
            self.hasher.writeElements(&.{ value.B0.a0, value.B0.a1, value.B1.a0, value.B1.a1, value.B2.a0, value.B2.a1 });
        }
    }

    pub fn randomField(self: *Transcript) poseidon2.Digest {
        const challenge = self.hasher.sumElement();
        self.updateElement(field.Element.zero());
        return challenge;
    }

    pub fn randomExt(self: *Transcript) ext.Ext {
        const challenge = self.randomField();
        return .{
            .B0 = .{ .a0 = challenge[0], .a1 = challenge[1] },
            .B1 = .{ .a0 = challenge[2], .a1 = challenge[3] },
            .B2 = .{ .a0 = challenge[4], .a1 = challenge[5] },
        };
    }

    pub fn state(self: Transcript) poseidon2.Digest {
        return self.hasher.getState();
    }

    pub fn setState(self: *Transcript, digest: poseidon2.Digest) void {
        self.hasher.setState(digest);
    }
};
