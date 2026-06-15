//! Golden-vector conformance tests for verifier-ray.
//!
//! The vectors are generated from prover-ray and encoded as plain integers in
//! `test_vectors`. The helpers in this file convert that generated data into
//! verifier-ray field/runtime types while keeping the expected values close to
//! the source vectors.

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vectors = @import("test_vectors");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const commitment_mod = verifier_ray.crypto.commitment;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const poseidon2 = verifier_ray.crypto.poseidon2;
const poly_lagrange = verifier_ray.polynomial.lagrange;
const poly_canonical = verifier_ray.polynomial.canonical;
const protocol = verifier_ray.protocol;

test "runtime visibility tags match prover-ray" {
    try std.testing.expectEqual(vectors.prover_visibility_oracle, @intFromEnum(protocol.Visibility.oracle));
    try std.testing.expectEqual(vectors.prover_visibility_public, @intFromEnum(protocol.Visibility.public));
}

test "koalabear base field matches prover-ray golden cases" {
    for (vectors.field_cases) |case| {
        const a = field.Element.init(case.a);
        const b = field.Element.init(case.b);

        try expectElem(a.add(b), case.add);
        try expectElem(a.sub(b), case.sub);
        try expectElem(a.mul(b), case.mul);
        try expectElem(a.square(), case.square_a);
        try expectElem(a.neg(), case.neg_a);
        try std.testing.expectEqualSlices(u8, &case.bytes_a, &a.toBytes());

        if (case.has_inv_a) {
            try expectElem(a.inverse(), case.inv_a);
        }
        if (case.has_div_ab) {
            try expectElem(a.div(b), case.div_ab);
        }
    }
}

test "koalabear extension field matches prover-ray golden cases" {
    for (vectors.ext_cases) |case| {
        const a = ext.Ext.fromUints(case.a);
        const b = ext.Ext.fromUints(case.b);
        const scalar = field.Element.init(case.scalar);

        try expectExt(a.add(b), ext.Ext.fromUints(case.add));
        try expectExt(a.sub(b), ext.Ext.fromUints(case.sub));
        try expectExt(a.mul(b), ext.Ext.fromUints(case.mul));
        try expectExt(a.square(), ext.Ext.fromUints(case.square_a));
        try expectExt(a.neg(), ext.Ext.fromUints(case.neg_a));
        try expectExt(a.mulByBase(scalar), ext.Ext.fromUints(case.mul_by_base));
        try std.testing.expectEqualSlices(u8, &case.bytes_a, &a.toBytes());

        if (case.has_inv_a) {
            try expectExt(a.inverse(), ext.Ext.fromUints(case.inv_a));
        }
    }
}

test "canonical polynomial evaluation matches prover-ray golden cases" {
    for (vectors.canonical_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(poly_canonical.evaluateBaseAtBase(coeffs[0..case.coeffs.len], field.Element.init(case.point)), case.expected);
    }

    for (vectors.canonical_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            poly_canonical.evaluateBaseAtExt(coeffs[0..case.coeffs.len], ext.Ext.fromUints(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }

    for (vectors.canonical_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            poly_canonical.evaluateExtAtBase(coeffs[0..case.coeffs.len], field.Element.init(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }

    for (vectors.canonical_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            poly_canonical.evaluateExtAtExt(coeffs[0..case.coeffs.len], ext.Ext.fromUints(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }
}

test "lagrange polynomial evaluation matches prover-ray golden cases" {
    for (vectors.lagrange_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(try poly_lagrange.evaluateBaseAtBase(coeffs[0..case.coeffs.len], field.Element.init(case.point)), case.expected);
    }

    for (vectors.lagrange_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            try poly_lagrange.evaluateBaseAtExt(coeffs[0..case.coeffs.len], ext.Ext.fromUints(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }

    for (vectors.lagrange_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try poly_lagrange.evaluateExtAtBase(coeffs[0..case.coeffs.len], field.Element.init(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }

    for (vectors.lagrange_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try poly_lagrange.evaluateExtAtExt(coeffs[0..case.coeffs.len], ext.Ext.fromUints(case.point)),
            ext.Ext.fromUints(case.expected),
        );
    }
}

test "lagrange evaluation returns domain value at roots of unity" {
    const base_values = [_]field.Element{
        field.Element.init(3),
        field.Element.init(1),
        field.Element.init(4),
        field.Element.init(1),
    };
    const ext_values = [_]ext.Ext{
        ext.Ext.fromUints(.{ 3, 1, 4, 1, 5, 9 }),
        ext.Ext.fromUints(.{ 5, 9, 2, 6, 5, 3 }),
        ext.Ext.fromUints(.{ 5, 3, 5, 8, 9, 7 }),
        ext.Ext.fromUints(.{ 9, 7, 9, 3, 2, 3 }),
    };

    const omega = try field.rootOfUnityBy(base_values.len);
    var domain_point = field.Element.one();
    for (base_values, ext_values) |base_value, ext_value| {
        try expectElem(try poly_lagrange.evaluateBaseAtBase(&base_values, domain_point), base_value.value);
        try expectExt(try poly_lagrange.evaluateBaseAtExt(&base_values, ext.Ext.lift(domain_point)), ext.Ext.lift(base_value));
        try expectExt(try poly_lagrange.evaluateExtAtBase(&ext_values, domain_point), ext_value);
        try expectExt(try poly_lagrange.evaluateExtAtExt(&ext_values, ext.Ext.lift(domain_point)), ext_value);
        domain_point = domain_point.mul(omega);
    }
}

test "poseidon2 compression and merkle-damgard match prover-ray golden cases" {
    for (vectors.poseidon_compress_cases) |case| {
        try expectDigest(poseidon2.compress(digest(case.left), digest(case.right)), case.expected);
    }

    for (vectors.poseidon_md_cases) |case| {
        var h = poseidon2.MDHasher.init();
        var message: [32]field.Element = undefined;
        fillElems(&message, case.message);
        h.writeElements(message[0..case.message.len]);
        try expectDigest(h.sumDigest(), case.expected);

        const expected_compressions = (case.message.len + poseidon2.block_size - 1) / poseidon2.block_size;
        try std.testing.expectEqual(expected_compressions, h.compression_count);
    }
}

test "fiat-shamir transcript matches prover-ray golden cases" {
    for (vectors.fiat_shamir_cases) |case| {
        var transcript = fiat_shamir.Transcript.init();

        var base_updates: [32]field.Element = undefined;
        fillElems(&base_updates, case.base_updates);
        transcript.updateElements(base_updates[0..case.base_updates.len]);

        var ext_updates: [16]ext.Ext = undefined;
        fillExts(&ext_updates, case.ext_updates);
        transcript.updateExt(ext_updates[0..case.ext_updates.len]);

        try expectDigest(transcript.randomDigest(), case.random_field);
        try expectExt(transcript.randomExt(), ext.Ext.fromUints(case.random_ext));
    }
}

test "transcript derives round coins matching prover-ray golden vectors" {
    for (vectors.runtime_trace_cases) |case| {
        // protocol.replay is parametric on a comptime Spec; the golden vectors
        // carry runtime coin counts, so drive fiat_shamir.Transcript directly
        // and compare against the prover-ray expected values.
        var transcript = fiat_shamir.Transcript.init();
        for (case.rounds) |round_case| {
            var backing = TraceRoundBacking{};
            const message = try backing.fill(round_case, false);
            for (message.columns) |entry| {
                switch (entry) {
                    .oracle_commitment => |c| transcript.updateElements(&c),
                    .public_column => |col| transcript.absorbVector(col),
                }
            }
            for (message.cells) |cell| transcript.absorbScalar(cell);
            for (round_case.expected_coins) |expected| {
                try expectExt(transcript.randomExt(), ext.Ext.fromUints(expected));
            }
        }
    }
}

test "tampered round message produces different downstream coins" {
    const case = vectors.runtime_trace_cases[0];
    var transcript = fiat_shamir.Transcript.init();
    var backing = TraceRoundBacking{};
    const message = try backing.fill(case.rounds[0], true);
    for (message.columns) |entry| {
        switch (entry) {
            .oracle_commitment => |c| transcript.updateElements(&c),
            .public_column => |col| transcript.absorbVector(col),
        }
    }
    for (message.cells) |cell| transcript.absorbScalar(cell);
    try std.testing.expect(case.rounds[0].expected_coins.len > 0);
    const got = transcript.randomExt();
    try std.testing.expect(!got.eql(ext.Ext.fromUints(case.rounds[0].expected_coins[0])));
}

/// Convert a generated Poseidon digest fixture into field elements.
fn digest(values: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, values) |*dst, value| {
        dst.* = field.Element.init(value);
    }
    return out;
}

/// Fill an existing buffer with generated base-field values.
fn fillElems(out: []field.Element, values: []const u32) void {
    for (values, 0..) |value, i| {
        out[i] = field.Element.init(value);
    }
}

/// Fill an existing buffer with generated extension-field values.
fn fillExts(out: []ext.Ext, values: []const [6]u32) void {
    for (values, 0..) |value, i| {
        out[i] = ext.Ext.fromUints(value);
    }
}

/// Compare a field element to its generated integer representation.
fn expectElem(actual: field.Element, expected: u32) !void {
    try std.testing.expectEqual(expected, actual.value);
}

/// Compare extension-field values using the field's equality helper.
fn expectExt(actual: ext.Ext, expected: ext.Ext) !void {
    try std.testing.expect(actual.eql(expected));
}

/// Compare a Poseidon digest to its generated integer representation.
fn expectDigest(actual: poseidon2.Digest, expected: [8]u32) !void {
    for (actual, expected) |actual_limb, expected_limb| {
        try expectElem(actual_limb, expected_limb);
    }
}

const trace_dimensions = traceDimensions(vectors.runtime_trace_cases);
const max_trace_columns = trace_dimensions.columns;
const max_trace_commitments = trace_dimensions.commitments;
const max_trace_values = trace_dimensions.values;
const max_trace_cells = trace_dimensions.cells;
const max_trace_coins = trace_dimensions.coins;

/// Maximum scratch-buffer sizes needed to replay all generated runtime traces.
const TraceDimensions = struct {
    columns: usize = 0,
    commitments: usize = 0,
    message_columns: usize = 0,
    values: usize = 0,
    cells: usize = 0,
    coins: usize = 0,
};

/// Derive runtime trace backing sizes from the generated vectors at comptime.
fn traceDimensions(comptime cases: anytype) TraceDimensions {
    var dimensions = TraceDimensions{};
    for (cases) |case| {
        for (case.rounds) |round| {
            dimensions.columns = @max(dimensions.columns, round.columns.len);
            dimensions.cells = @max(dimensions.cells, round.cells.len);
            dimensions.coins = @max(dimensions.coins, round.expected_coins.len);
            var commitments: usize = 0;
            var message_columns: usize = 0;
            for (round.columns) |column| {
                switch (column) {
                    .oracle => |commitments_for_column| {
                        commitments += commitments_for_column.len;
                        message_columns += commitments_for_column.len;
                    },
                    .public_base => |values| {
                        message_columns += 1;
                        dimensions.values = @max(dimensions.values, values.len);
                    },
                    .public_ext => |values| {
                        message_columns += 1;
                        dimensions.values = @max(dimensions.values, values.len);
                    },
                }
            }
            dimensions.commitments = @max(dimensions.commitments, commitments);
            dimensions.message_columns = @max(dimensions.message_columns, message_columns);
        }
    }
    return dimensions;
}

/// Owns the scratch buffers used by a runtime round message.
///
/// `RoundMessage` stores slices into this backing, so callers must keep the
/// backing alive until the runtime has absorbed the message.
const TraceRoundBacking = struct {
    oracle_commitments: [max_trace_commitments]protocol.Commitment = undefined,
    columns: [trace_dimensions.message_columns]protocol.ColumnMessage = undefined,
    cells: [max_trace_cells]protocol.Scalar = undefined,
    base_values: [max_trace_columns][max_trace_values]field.Element = undefined,
    ext_values: [max_trace_columns][max_trace_values]ext.Ext = undefined,

    /// Convert one generated trace round into the verifier runtime message shape.
    fn fill(self: *TraceRoundBacking, round_case: vectors.RuntimeTraceRound, tamper_first_absorb: bool) !protocol.RoundMessage {
        try std.testing.expect(round_case.columns.len <= max_trace_columns);
        try std.testing.expect(round_case.cells.len <= max_trace_cells);

        var tampered = false;
        var oracle_commitment_count: usize = 0;
        var column_count: usize = 0;
        for (round_case.columns, 0..) |column_case, i| {
            switch (column_case) {
                .oracle => |commitments| {
                    try std.testing.expect(commitments.len <= max_trace_commitments);
                    for (commitments) |c| {
                        self.oracle_commitments[oracle_commitment_count] = commitment_mod.fromUints(c);
                        if (tamper_first_absorb and !tampered) {
                            self.oracle_commitments[oracle_commitment_count][0] = field.Element.init(self.oracle_commitments[oracle_commitment_count][0].value ^ 1);
                            tampered = true;
                        }
                        self.columns[column_count] = .{ .oracle_commitment = self.oracle_commitments[oracle_commitment_count] };
                        oracle_commitment_count += 1;
                        column_count += 1;
                    }
                },
                .public_base => |values| {
                    try std.testing.expect(values.len <= max_trace_values);
                    fillElems(&self.base_values[i], values);
                    if (tamper_first_absorb and !tampered and values.len != 0) {
                        self.base_values[i][0] = field.Element.init(self.base_values[i][0].value ^ 1);
                        tampered = true;
                    }
                    self.columns[column_count] = .{ .public_column = .{ .base = self.base_values[i][0..values.len] } };
                    column_count += 1;
                },
                .public_ext => |values| {
                    try std.testing.expect(values.len <= max_trace_values);
                    fillExts(&self.ext_values[i], values);
                    if (tamper_first_absorb and !tampered and values.len != 0) {
                        self.ext_values[i][0].B0.a0 = field.Element.init(self.ext_values[i][0].B0.a0.value ^ 1);
                        tampered = true;
                    }
                    self.columns[column_count] = .{ .public_column = .{ .ext = self.ext_values[i][0..values.len] } };
                    column_count += 1;
                },
            }
        }

        for (round_case.cells, 0..) |cell_case, i| {
            self.cells[i] = switch (cell_case) {
                .base => |base_value| .{ .base = field.Element.init(base_value) },
                .ext => |ext_value| .{ .ext = ext.Ext.fromUints(ext_value) },
            };
        }

        return .{
            .columns = self.columns[0..column_count],
            .cells = self.cells[0..round_case.cells.len],
        };
    }
};
