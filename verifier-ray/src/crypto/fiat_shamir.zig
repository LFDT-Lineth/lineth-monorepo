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

    pub fn randomExt(self: *Transcript) ext.Ext {
        const challenge = self.randomDigest();
        return .{
            .B0 = .{ .a0 = challenge[0], .a1 = challenge[1] },
            .B1 = .{ .a0 = challenge[2], .a1 = challenge[3] },
            .B2 = .{ .a0 = challenge[4], .a1 = challenge[5] },
        };
    }

    /// Fills `out` with integers reduced into `[0, upper_bound)`, consuming one
    /// Poseidon2 digest at a time and taking its eight base limbs in order.
    /// `upper_bound` must be a power of two, so masking with `upper_bound - 1`
    /// is uniform and equivalent to `% upper_bound` — this mirrors prover-ray's
    /// `RandomManyIntegers` expression line-for-line. We
    /// derive FRI query positions and must reproduce the prover's transcript
    /// exactly. Squeezing continues from the current transcript state, so callers
    /// must have absorbed everything up to the query-derivation point.
    pub fn randomManyIntegers(self: *Transcript, out: []usize, comptime upper_bound: usize) void {
        comptime {
            if (upper_bound == 0 or (upper_bound & (upper_bound - 1)) != 0)
                @compileError("fiat_shamir.randomManyIntegers: upper_bound must be a non-zero power of two");
        }
        self.randomManyIntegersRuntime(out, upper_bound);
    }

    /// Runtime `upper_bound` variant, for the PCS layer whose codeword size is a
    /// runtime function of the restricted FRI params. `upper_bound` must still be
    /// a non-zero power of two (the FRI codeword size always is; the reconstructed
    /// params guarantee it). Byte-identical squeeze order to the comptime form.
    pub fn randomManyIntegersRuntime(self: *Transcript, out: []usize, upper_bound: usize) void {
        var i: usize = 0;
        while (i < out.len) {
            const digest = self.randomDigest();
            for (digest) |element| {
                out[i] = @as(usize, element.value) & (upper_bound - 1);
                i += 1;
                if (i >= out.len) break;
            }
        }
    }
};
