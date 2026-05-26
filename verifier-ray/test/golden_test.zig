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
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const poseidon2 = verifier_ray.crypto.poseidon2;
const lagrange = verifier_ray.pcs.lagrange;
const polynomial = verifier_ray.pcs.polynomial;
const runtime = verifier_ray.runtime;

test "koalabear base field matches prover-ray golden cases" {
    for (vectors.field_cases) |case| {
        const a = elem(case.a);
        const b = elem(case.b);

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
        const a = uintsToExt(case.a);
        const b = uintsToExt(case.b);
        const scalar = elem(case.scalar);

        try expectExt(a.add(b), uintsToExt(case.add));
        try expectExt(a.sub(b), uintsToExt(case.sub));
        try expectExt(a.mul(b), uintsToExt(case.mul));
        try expectExt(a.square(), uintsToExt(case.square_a));
        try expectExt(a.neg(), uintsToExt(case.neg_a));
        try expectExt(a.mulByBase(scalar), uintsToExt(case.mul_by_base));
        try std.testing.expectEqualSlices(u8, &case.bytes_a, &a.toBytes());

        if (case.has_inv_a) {
            try expectExt(a.inverse(), uintsToExt(case.inv_a));
        }
    }
}

test "canonical polynomial evaluation matches prover-ray golden cases" {
    for (vectors.canonical_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(polynomial.evaluateBaseCanonical(coeffs[0..case.coeffs.len], elem(case.point)), case.expected);
    }

    for (vectors.canonical_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateBaseCanonicalAtExt(coeffs[0..case.coeffs.len], uintsToExt(case.point)),
            uintsToExt(case.expected),
        );
    }

    for (vectors.canonical_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateExtCanonicalAtBase(coeffs[0..case.coeffs.len], elem(case.point)),
            uintsToExt(case.expected),
        );
    }

    for (vectors.canonical_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            polynomial.evaluateExtCanonical(coeffs[0..case.coeffs.len], uintsToExt(case.point)),
            uintsToExt(case.expected),
        );
    }
}

test "lagrange polynomial evaluation matches prover-ray golden cases" {
    for (vectors.lagrange_base_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectElem(try lagrange.evaluateBaseAtBase(coeffs[0..case.coeffs.len], elem(case.point)), case.expected);
    }

    for (vectors.lagrange_base_at_ext_cases) |case| {
        var coeffs: [16]field.Element = undefined;
        fillElems(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateBaseAtExt(coeffs[0..case.coeffs.len], uintsToExt(case.point)),
            uintsToExt(case.expected),
        );
    }

    for (vectors.lagrange_ext_at_base_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateExtAtBase(coeffs[0..case.coeffs.len], elem(case.point)),
            uintsToExt(case.expected),
        );
    }

    for (vectors.lagrange_ext_cases) |case| {
        var coeffs: [16]ext.Ext = undefined;
        fillExts(&coeffs, case.coeffs);
        try expectExt(
            try lagrange.evaluateExtAtExt(coeffs[0..case.coeffs.len], uintsToExt(case.point)),
            uintsToExt(case.expected),
        );
    }
}

test "lagrange evaluation returns domain value at roots of unity" {
    const base_values = [_]field.Element{
        elem(3),
        elem(1),
        elem(4),
        elem(1),
    };
    const ext_values = [_]ext.Ext{
        .{ .B0 = .{ .a0 = elem(3), .a1 = elem(1) }, .B1 = .{ .a0 = elem(4), .a1 = elem(1) }, .B2 = .{ .a0 = elem(5), .a1 = elem(9) } },
        .{ .B0 = .{ .a0 = elem(5), .a1 = elem(9) }, .B1 = .{ .a0 = elem(2), .a1 = elem(6) }, .B2 = .{ .a0 = elem(5), .a1 = elem(3) } },
        .{ .B0 = .{ .a0 = elem(5), .a1 = elem(3) }, .B1 = .{ .a0 = elem(5), .a1 = elem(8) }, .B2 = .{ .a0 = elem(9), .a1 = elem(7) } },
        .{ .B0 = .{ .a0 = elem(9), .a1 = elem(7) }, .B1 = .{ .a0 = elem(9), .a1 = elem(3) }, .B2 = .{ .a0 = elem(2), .a1 = elem(3) } },
    };

    const omega = try field.rootOfUnityBy(base_values.len);
    var domain_point = field.Element.one();
    for (base_values, ext_values) |base_value, ext_value| {
        try expectElem(try lagrange.evaluateBaseAtBase(&base_values, domain_point), base_value.value);
        try expectExt(try lagrange.evaluateBaseAtExt(&base_values, ext.Ext.lift(domain_point)), ext.Ext.lift(base_value));
        try expectExt(try lagrange.evaluateExtAtBase(&ext_values, domain_point), ext_value);
        try expectExt(try lagrange.evaluateExtAtExt(&ext_values, ext.Ext.lift(domain_point)), ext_value);
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
        try expectExt(transcript.randomExt(), uintsToExt(case.random_ext));
    }
}

test "runtime round coin derivation matches prover-ray scripted protocol" {
    for (vectors.runtime_trace_cases) |case| {
        var rt = runtime.Runtime.initWithRoundCount(case.rounds.len);
        var coins: [max_trace_coins]runtime.Coin = undefined;

        // The trace was generated by prover-ray's WIOP runtime. Replaying the
        // same absorbs here proves the Zig runtime derives the same round coins.
        for (case.rounds[0 .. case.rounds.len - 1], 0..) |round_case, round_index| {
            var backing = TraceRoundBacking{};
            const message = try backing.fill(round_case, false);
            // all the random coins generated via zig runtime
            const got = try rt.advanceRoundWithMessage(round_index, message, &coins);
            try std.testing.expectEqual(round_case.expected_coins.len, got.len);
            for (got, round_case.expected_coins) |actual, expected| {
                try expectExt(actual, uintsToExt(expected));
            }
        }
    }
}

test "runtime downstream coin diverges after tampered absorb" {
    const case = vectors.runtime_trace_cases[0];
    var rt = runtime.Runtime.initWithRoundCount(case.rounds.len);
    var backing = TraceRoundBacking{};
    const message = try backing.fill(case.rounds[0], true);
    var coins: [max_trace_coins]runtime.Coin = undefined;

    const got = try rt.advanceRoundWithMessage(0, message, &coins);
    try std.testing.expect(got.len > 0);
    try std.testing.expect(!got[0].eql(uintsToExt(case.rounds[0].expected_coins[0])));
}

/// Convert a generated base-field integer into the concrete field element type.
fn elem(value: u32) field.Element {
    return field.Element.init(value);
}

/// Convert six generated base-field limbs into one KoalaBear extension value.
fn uintsToExt(limbs: [6]u32) ext.Ext {
    return ext.Ext.fromUints(limbs[0], limbs[1], limbs[2], limbs[3], limbs[4], limbs[5]);
}

/// Convert a generated Poseidon digest fixture into field elements.
fn digest(values: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, values) |*dst, value| {
        dst.* = elem(value);
    }
    return out;
}

/// Fill an existing buffer with generated base-field values.
fn fillElems(out: []field.Element, values: []const u32) void {
    for (values, 0..) |value, i| {
        out[i] = elem(value);
    }
}

/// Fill an existing buffer with generated extension-field values.
fn fillExts(out: []ext.Ext, values: []const [6]u32) void {
    for (values, 0..) |value, i| {
        out[i] = uintsToExt(value);
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
const max_trace_values = trace_dimensions.values;
const max_trace_cells = trace_dimensions.cells;
const max_trace_coins = trace_dimensions.coins;

/// Maximum scratch-buffer sizes needed to replay all generated runtime traces.
const TraceDimensions = struct {
    columns: usize = 0,
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
            for (round.columns) |column| {
                dimensions.values = @max(dimensions.values, column.base_values.len);
                dimensions.values = @max(dimensions.values, column.ext_values.len);
            }
        }
    }
    return dimensions;
}

/// Owns the scratch buffers used by a runtime round message.
///
/// `RoundMessage` stores slices into this backing, so callers must keep the
/// backing alive until the runtime has absorbed the message.
const TraceRoundBacking = struct {
    columns: [max_trace_columns]runtime.ColumnAssignment = undefined,
    cells: [max_trace_cells]runtime.Scalar = undefined,
    base_values: [max_trace_columns][max_trace_values]field.Element = undefined,
    ext_values: [max_trace_columns][max_trace_values]ext.Ext = undefined,

    /// Convert one generated trace round into the verifier runtime message shape.
    fn fill(self: *TraceRoundBacking, round_case: vectors.RuntimeTraceRound, tamper_first_absorb: bool) !runtime.RoundMessage {
        try std.testing.expect(round_case.columns.len <= max_trace_columns);
        try std.testing.expect(round_case.cells.len <= max_trace_cells);

        var tampered = false;
        var column_count: usize = 0;
        for (round_case.columns, 0..) |column_case, i| {
            if (column_case.visibility == 0) continue;
            try std.testing.expect(column_case.is_assigned);

            const assignment: runtime.Vector = if (column_case.is_ext) assignment: {
                try std.testing.expect(column_case.ext_values.len <= max_trace_values);
                fillExts(&self.ext_values[i], column_case.ext_values);
                break :assignment .{ .ext = self.ext_values[i][0..column_case.ext_values.len] };
            } else assignment: {
                try std.testing.expect(column_case.base_values.len <= max_trace_values);
                fillElems(&self.base_values[i], column_case.base_values);
                if (tamper_first_absorb and !tampered and column_case.base_values.len != 0) {
                    // The lsb of the first value is bit flipped to simulate a tampered absorb, which should cause downstream coins to diverge.
                    self.base_values[i][0] = elem(self.base_values[i][0].value ^ 1);
                    tampered = true;
                }
                break :assignment .{ .base = self.base_values[i][0..column_case.base_values.len] };
            };

            self.columns[column_count] = .{
                .visibility = @enumFromInt(column_case.visibility),
                .assignment = assignment,
            };
            column_count += 1;
        }

        for (round_case.cells, 0..) |cell_case, i| {
            try std.testing.expect(cell_case.is_assigned);
            self.cells[i] = if (cell_case.is_ext)
                .{ .ext = uintsToExt(cell_case.ext_value) }
            else
                .{ .base = elem(cell_case.base_value) };
        }

        return .{
            .columns = self.columns[0..column_count],
            .cells = self.cells[0..round_case.cells.len],
            .next_round_coin_count = round_case.expected_coins.len,
        };
    }
};
