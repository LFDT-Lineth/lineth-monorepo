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
            self.hasher.writeElements(&value.limbs);
        }
    }

    pub fn randomField(self: *Transcript) poseidon2.Digest {
        const challenge = self.hasher.sumElement();
        self.updateElement(field.Element.zero());
        return challenge;
    }

    pub fn randomExt(self: *Transcript) ext.Ext {
        const challenge = self.randomField();
        return .{ .limbs = .{
            challenge[0],
            challenge[1],
            challenge[2],
            challenge[3],
        } };
    }

    pub fn state(self: Transcript) poseidon2.Digest {
        return self.hasher.getState();
    }

    pub fn setState(self: *Transcript, digest: poseidon2.Digest) void {
        self.hasher.setState(digest);
    }

    pub const challengeExt = randomExt;
};
