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
                    std.debug.print("pair case {d} ({s}) consistent alt pair failed: {s}\n", .{ i, case.name, @errorName(err) });
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

test "consistency: statements of different lengths are rejected" {
    const one = [_]protocol.Scalar{.{ .base = field.Element.init(7) }};
    try std.testing.expectError(
        error.InconsistentPublicInputs,
        verifier.checkPublicInputConsistency(&one, &.{}),
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
