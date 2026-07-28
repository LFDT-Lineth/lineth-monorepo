const std = @import("std");
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
            // Absorb the six base limbs directly. Routing through writeElements
            // (even by reinterpreting the slice) de-inlines these stores and
            // measured slower; building a throwaway 6-element array is also waste.
            self.hasher.writeElement(ext_value.B0.a0);
            self.hasher.writeElement(ext_value.B0.a1);
            self.hasher.writeElement(ext_value.B1.a0);
            self.hasher.writeElement(ext_value.B1.a1);
            self.hasher.writeElement(ext_value.B2.a0);
            self.hasher.writeElement(ext_value.B2.a1);
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

    /// The current transcript state as a digest (flushing any pending buffer
    /// into a copy; the live transcript is unchanged). Mirrors prover-ray
    /// `fiatshamir.FiatShamir.State` / `MDHasher.GetStateOctuplet`. Used to
    /// snapshot the transcript for a fixture and reseed it in a test.
    pub fn getState(self: Transcript) poseidon2.Digest {
        return self.hasher.getState();
    }

    /// Replaces the transcript state with `state` and clears the pending buffer.
    /// Mirrors prover-ray `fiatshamir.FiatShamir.SetState` /
    /// `MDHasher.SetStateOctuplet`. Reseeds a transcript from a snapshot so a
    /// standalone fixture can be verified without replaying a full protocol.
    pub fn setState(self: *Transcript, state: poseidon2.Digest) void {
        self.hasher.setState(state);
    }

    pub fn randomExt(self: *Transcript) ext.Ext {
        const challenge = self.randomDigest();
        return .{
            .B0 = .{ .a0 = challenge[0], .a1 = challenge[1] },
            .B1 = .{ .a0 = challenge[2], .a1 = challenge[3] },
            .B2 = .{ .a0 = challenge[4], .a1 = challenge[5] },
        };
    }

    /// Samples `out.len` integers uniformly in `[0, upper_bound)` from the
    /// transcript, filling `out`. Byte-faithful port of prover-ray
    /// `fiatshamir.FiatShamir.RandomManyIntegers`: each `randomDigest` squeeze
    /// yields eight base elements, and each is reduced `element.value %
    /// upper_bound` (the element is already a canonical representative, matching
    /// Go's `c[j].Bits()[0]`). Squeezes are drawn until `out` is full; a partial
    /// final digest discards its unused coordinates. `upper_bound` must be a
    /// non-zero power of two (prover-ray panics otherwise); the caller guarantees
    /// this from the FRI codeword size, so it is a debug assertion here.
    pub fn randomManyIntegers(self: *Transcript, out: []usize, upper_bound: usize) void {
        std.debug.assert(upper_bound != 0 and (upper_bound & (upper_bound - 1)) == 0);
        var i: usize = 0;
        while (i < out.len) {
            const digest = self.randomDigest();
            for (digest) |element| {
                out[i] = @as(usize, element.value) % upper_bound;
                i += 1;
                if (i >= out.len) break;
            }
        }
    }
};
