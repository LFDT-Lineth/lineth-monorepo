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
    // The case only verifyPair can catch. Sweep every case carrying an
    // altDifferentStatement fixture: a genuinely re-proven proof, same size,
    // whose statement differs from honest's. Rejected in either order.
    var rejected: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasAltDifferentStatement(i)) {
            const case = comptime vf.get(i);
            const honest = vf.getInput(i);
            const alt = vf.getInputAltDifferentStatement(i);
            // Pin the fixture's shape so the sweep cannot pass vacuously.
            try std.testing.expect(!statementsAgree(honest.public_inputs, alt.public_inputs));
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
    try std.testing.expect(rejected > 0);
}

test "two proofs of one statement at different dynamic sizes verify as a pair" {
    // The claim is the public-input statement and nothing else (see "Aggregate
    // statement" in verifier.zig): proof-side data such as module_sizes is
    // validated per proof and never compared across the pair. hasAlt is the
    // multi-size predicate, so every alt here has a different size vector.
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasAlt(i)) {
            const case = comptime vf.get(i);
            const honest = vf.getInput(i);
            const alt = vf.getInputAlt(i);
            try std.testing.expect(statementsAgree(honest.public_inputs, alt.public_inputs));
            try std.testing.expect(!std.mem.eql(usize, honest.proof.module_sizes, alt.proof.module_sizes));
            checked += 1;
            verifier.verifyPair(case.spec, case.systems, honest, alt) catch |err| {
                std.debug.print(
                    "pair case {d} ({s}) different-size pair failed: {s}\n",
                    .{ i, case.name, @errorName(err) },
                );
                return err;
            };
        }
    }
    try std.testing.expect(checked > 0);
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

/// Index of the first fixture case whose compiled statement has exactly
/// `count` public inputs, so the tests below do not depend on fixture order.
fn firstCaseWithPublicInputs(comptime count: usize) usize {
    @setEvalBranchQuota(100_000);
    for (0..vf.case_count) |i| {
        if (vf.get(i).systems.public_input.refs.len == count) return i;
    }
    @compileError(std.fmt.comptimePrint("no fixture case has {d} public inputs", .{count}));
}

test "pair validates both statement lengths before comparing elements" {
    const no_public_inputs_index = comptime firstCaseWithPublicInputs(0);
    const no_public_inputs_case = comptime vf.get(no_public_inputs_index);
    const no_public_inputs = vf.getInput(no_public_inputs_index);

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

    const one_public_input_index = comptime firstCaseWithPublicInputs(1);
    const one_public_input_case = comptime vf.get(one_public_input_index);
    const valid = vf.getInput(one_public_input_index);
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

test "pair statement encoding binds normalized public inputs" {
    const as_base = [_]protocol.Scalar{.{ .base = field.Element.init(42) }};
    const as_ext = [_]protocol.Scalar{.{ .ext = ext.Ext.lift(field.Element.init(42)) }};
    var base_encoded: [verifier.pairStatementEncodedLen(1)]u8 = undefined;
    var ext_encoded: [verifier.pairStatementEncodedLen(1)]u8 = undefined;
    try verifier.encodePairStatement(&as_base, &base_encoded);
    try verifier.encodePairStatement(&as_ext, &ext_encoded);
    try std.testing.expectEqualSlices(u8, &base_encoded, &ext_encoded);
}

test "pair statement encoding matches the documented byte layout" {
    // Pins the "Aggregate statement" spec in verifier.zig byte for byte, WITHOUT
    // going through Ext.toBytes — so a change to either the spec text or the
    // encoder (endianness, limb order, prefix width) fails here rather than
    // silently diverging from what an outer verifier would rebuild.
    //
    // Statement: two scalars. X[0] = 42 (base, lifted: only limb B0.a0 set).
    // X[1] = an extension element with every limb distinct, to pin limb order.
    const x1 = ext.Ext{
        .B0 = .{ .a0 = field.Element.init(0x01020304), .a1 = field.Element.init(0x11121314) },
        .B1 = .{ .a0 = field.Element.init(0x21222324), .a1 = field.Element.init(0x31323334) },
        .B2 = .{ .a0 = field.Element.init(0x41424344), .a1 = field.Element.init(0x51525354) },
    };
    const statement = [_]protocol.Scalar{ .{ .base = field.Element.init(42) }, .{ .ext = x1 } };
    var encoded: [verifier.pairStatementEncodedLen(2)]u8 = undefined;
    try verifier.encodePairStatement(&statement, &encoded);

    const expected = "lineth-verifier-pair-v1" ++
        // BE64(n = 2)
        "\x00\x00\x00\x00\x00\x00\x00\x02" ++
        // enc(42): B0.a0 big-endian, remaining five limbs zero
        "\x00\x00\x00\x2a" ++ "\x00" ** 20 ++
        // enc(x1): B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1, each big-endian
        "\x01\x02\x03\x04" ++ "\x11\x12\x13\x14" ++
        "\x21\x22\x23\x24" ++ "\x31\x32\x33\x34" ++
        "\x41\x42\x43\x44" ++ "\x51\x52\x53\x54";
    try std.testing.expectEqualSlices(u8, expected, &encoded);
}

test "empty statements are trivially consistent" {
    try verifier.checkPublicInputConsistency(&.{}, &.{});
}

test "module sizes must match the dynamic module count exactly" {
    // Entries past `dynamic_module_count` never reach the transcript, so
    // accepting them would let one honest proof take several encodings.
    // `verifyPair` inherits the check through `verify`, in both slots.
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        const case = comptime vf.get(i);
        if (comptime case.spec.dynamic_module_count > 0) {
            checked += 1;
            const honest = vf.getInput(i);
            const sizes = honest.proof.module_sizes;

            var extended_sizes: [case.spec.dynamic_module_count + 1]usize = undefined;
            @memcpy(extended_sizes[0..sizes.len], sizes);
            extended_sizes[sizes.len] = sizes[0];
            var extra = honest;
            extra.proof.module_sizes = &extended_sizes;
            try std.testing.expectError(
                error.UnexpectedDynamicModuleSize,
                verifier.verify(case.spec, case.systems, extra.proof, extra.public_inputs),
            );
            try std.testing.expectError(
                error.UnexpectedDynamicModuleSize,
                verifier.verifyPair(case.spec, case.systems, extra, extra),
            );

            var missing = honest;
            missing.proof.module_sizes = sizes[0 .. sizes.len - 1];
            try std.testing.expectError(
                error.MissingDynamicModuleSize,
                verifier.verify(case.spec, case.systems, missing.proof, missing.public_inputs),
            );
        }
    }
    try std.testing.expect(checked > 0);
}
