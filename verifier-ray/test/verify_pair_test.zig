const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;
const ext = verifier_ray.field.koalabear_ext;
const field = verifier_ray.field.koalabear;

// Tests for `verifier.verifyPair`, the two-proof aggregation entry point:
// both proofs must verify against the same compiled system AND carry the same
// public-input statement.
//
// The adversarial case that only verifyPair can catch is two INDIVIDUALLY
// VALID proofs about different statements. The generated public-input fixture
// (OpenedCellPublicInput) carries exactly that pair: its honest proof opens the
// cell value 30, its alt proof — honest for the same baked system — opens 31.
// Neither `verify` call rejects either proof; only the consistency check can.

// Returns whether the two statements agree, mirroring the rule verifyPair
// enforces, so the sweeps below can branch on the fixture shape.
fn statementsAgree(a: verifier.PublicInput, b: verifier.PublicInput) bool {
    verifier.checkPublicInputConsistency(a, b) catch return false;
    return true;
}

test "a proof paired with itself verifies" {
    inline for (0..vf.case_count) |i| {
        const case = comptime vf.get(i);
        const input = vf.getInput(i);
        verifier.verifyPair(case.spec, case.systems, input, input) catch |err| {
            std.debug.print("pair case {d} ({s}) unexpectedly failed: {s}\n", .{ i, case.name, @errorName(err) });
            return err;
        };
    }
}

test "a pair containing a tampered proof is rejected" {
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasFailing(i)) {
            checked += 1;
            const case = comptime vf.get(i);
            const honest = vf.getInput(i);
            const tampered = vf.getInputFailing(i);
            // Both orders: the tampered member must sink the pair no matter
            // which slot it occupies.
            if (verifier.verifyPair(case.spec, case.systems, honest, tampered)) |_| {
                std.debug.print("pair case {d} ({s}) accepted (honest, tampered)\n", .{ i, case.name });
                return error.TamperedProofAccepted;
            } else |_| {}
            if (verifier.verifyPair(case.spec, case.systems, tampered, honest)) |_| {
                std.debug.print("pair case {d} ({s}) accepted (tampered, honest)\n", .{ i, case.name });
                return error.TamperedProofAccepted;
            } else |_| {}
        }
    }
    try std.testing.expect(checked > 0);
}

test "two individually valid proofs with different statements are rejected" {
    // Sweep every case carrying a second honest proof (alt). Where the two
    // statements agree (the multi-size cases: both statements are empty), the
    // pair must verify — two DIFFERENT honest proofs are a legitimate pair.
    // Where they disagree (OpenedCellPublicInput: 30 vs 31), the pair must be
    // rejected with InconsistentPublicInputs even though — as pinned by
    // verifier_test.zig's alt sweep — each proof verifies on its own.
    var rejected: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasAlt(i)) {
            const case = comptime vf.get(i);
            const honest = vf.getInput(i);
            const alt = vf.getInputAlt(i);
            if (statementsAgree(honest.public_inputs, alt.public_inputs)) {
                verifier.verifyPair(case.spec, case.systems, honest, alt) catch |err| {
                    std.debug.print(
                        "pair case {d} ({s}) consistent alt pair failed: {s}\n",
                        .{ i, case.name, @errorName(err) },
                    );
                    return err;
                };
            } else {
                rejected += 1;
                try std.testing.expectError(
                    error.InconsistentPublicInputs,
                    verifier.verifyPair(case.spec, case.systems, honest, alt),
                );
                try std.testing.expectError(
                    error.InconsistentPublicInputs,
                    verifier.verifyPair(case.spec, case.systems, alt, honest),
                );
            }
        }
    }
    // Guard against the sweep silently losing its adversarial member: the
    // public-input alt fixture must keep carrying a differing statement.
    try std.testing.expect(rejected > 0);
}

test "two independently proven proofs with the same non-trivial statement verify as a pair" {
    // The multi-size cases (DynamicFibonacciMultiSize/TwoModules) only agree
    // trivially — both statements are empty, so equality holds vacuously.
    // This sweeps every case carrying an altSameStatement fixture: an
    // independently re-proven proof (different witness rounds/openings, NOT a
    // copy of honest) whose public-input statement genuinely matches honest's
    // non-empty statement. Confirms verifyPair accepts a pair on real
    // statement equality, not just on both sides being empty.
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasAltSameStatement(i)) {
            const case = comptime vf.get(i);
            const honest = vf.getInput(i);
            const altSame = vf.getInputAltSameStatement(i);
            try std.testing.expect(honest.public_inputs.len > 0);
            try std.testing.expect(statementsAgree(honest.public_inputs, altSame.public_inputs));
            checked += 1;
            verifier.verifyPair(case.spec, case.systems, honest, altSame) catch |err| {
                std.debug.print(
                    "pair case {d} ({s}) same-statement pair failed: {s}\n",
                    .{ i, case.name, @errorName(err) },
                );
                return err;
            };
        }
    }
    // Guard against the sweep silently losing its positive member.
    try std.testing.expect(checked > 0);
}

test "consistency: statements of different lengths are rejected" {
    const one = [_]protocol.Scalar{.{ .base = field.Element.init(7) }};
    try std.testing.expectError(
        error.InconsistentPublicInputs,
        verifier.checkPublicInputConsistency(&one, &.{}),
    );
}

test "pair validates both statement lengths before comparing elements" {
    const no_public_inputs_index = 0;
    const no_public_inputs_case = comptime vf.get(no_public_inputs_index);
    const no_public_inputs = vf.getInput(no_public_inputs_index);
    try std.testing.expectEqual(@as(usize, 0), no_public_inputs_case.systems.public_input.refs.len);

    const one = [_]protocol.Scalar{.{ .base = field.Element.init(1) }};
    const two = [_]protocol.Scalar{.{ .base = field.Element.init(2) }};
    var invalid_a = no_public_inputs;
    invalid_a.public_inputs = &one;
    var invalid_b = no_public_inputs;
    invalid_b.public_inputs = &two;

    // If consistency ran first, these different elements would instead return
    // InconsistentPublicInputs. The compiled statement count takes precedence.
    try std.testing.expectError(
        error.InvalidPublicInputCount,
        verifier.verifyPair(no_public_inputs_case.spec, no_public_inputs_case.systems, invalid_a, invalid_b),
    );

    const one_public_input_index = 62;
    const one_public_input_case = comptime vf.get(one_public_input_index);
    const valid = vf.getInput(one_public_input_index);
    try std.testing.expectEqual(@as(usize, 1), one_public_input_case.systems.public_input.refs.len);
    var missing = valid;
    missing.public_inputs = &.{};

    // Exercise each side independently so neither can reach the consistency
    // comparison or proof verification with an invalid statement length.
    try std.testing.expectError(
        error.InvalidPublicInputCount,
        verifier.verifyPair(one_public_input_case.spec, one_public_input_case.systems, missing, valid),
    );
    try std.testing.expectError(
        error.InvalidPublicInputCount,
        verifier.verifyPair(one_public_input_case.spec, one_public_input_case.systems, valid, missing),
    );
}

test "consistency: equality is over field values, not wire encoding" {
    // The same value may travel base-encoded in one statement and lifted into
    // the extension field in the other; they are the same statement.
    const as_base = [_]protocol.Scalar{.{ .base = field.Element.init(42) }};
    const as_ext = [_]protocol.Scalar{.{ .ext = ext.Ext.lift(field.Element.init(42)) }};
    try verifier.checkPublicInputConsistency(&as_base, &as_ext);

    const other = [_]protocol.Scalar{.{ .base = field.Element.init(43) }};
    try std.testing.expectError(
        error.InconsistentPublicInputs,
        verifier.checkPublicInputConsistency(&as_base, &other),
    );
}

test "empty statements are trivially consistent" {
    try verifier.checkPublicInputConsistency(&.{}, &.{});
}
