const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;
const vanishing = verifier_ray.query.vanishing;
const logderivativesum = verifier_ray.query.logderivativesum;
const pcs = verifier_ray.query.pcs;
const ext = verifier_ray.field.koalabear_ext;
const field = verifier_ray.field.koalabear;

// Tests for `verifier.verify`, the top-level entry point. Two layers:
//   1. The end-to-end sweep below drives every generated fixture case through
//      the compilers.CompileFull pipeline (Go, gen-time) → real proof →
//      serialized verify.zig → verifier.verify chain. Honest proofs must
//      verify; tampered ones must be rejected.
//   2. The round-count guard needs no fixture: `verify` must reject a proof
//      whose round count disagrees with the compiled spec, during transcript
//      replay (before PCS runs).
//
// (The PCS-authenticated challenge derivation `verify` relies on is pinned
// separately in pcs_test.zig.)

test "all fixture cases: honest proofs verify end-to-end" {
    inline for (0..vf.case_count) |i| {
        const case = comptime vf.get(i);
        const input = vf.getInput(i);
        verifier.verify(case.spec, case.systems, input.proof, input.public_inputs) catch |err| {
            std.debug.print("case {d} ({s}) unexpectedly failed: {s}\n", .{ i, case.name, @errorName(err) });
            return err;
        };
    }
}

test "all fixture cases: tampered proofs are rejected" {
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasFailing(i)) {
            checked += 1;
            const case = comptime vf.get(i);
            const input = vf.getInputFailing(i);
            const res = verifier.verify(case.spec, case.systems, input.proof, input.public_inputs);
            if (res) |_| {
                std.debug.print("case {d} ({s}) accepted a tampered proof\n", .{ i, case.name });
                return error.TamperedProofAccepted;
            } else |_| {}
        }
    }
    // Guard against the sweep silently checking nothing (e.g. if hasFailing
    // regressed to all-false): at least the vanishing scenarios carry failing
    // inputs.
    try std.testing.expect(checked > 0);
}

// False-statement (malicious-prover) sub-verifier coverage. The shared
// .invalid fixtures above are TAMPERED proofs — a valid proof with a byte
// flipped after the fact — so they are internally inconsistent and get
// rejected at the PCS/vanishing stage, BEFORE later sub-verifiers
// (grandproduct, logderivativesum) run. They cannot demonstrate that those
// sub-verifiers are actually wired into verifier.Systems.
//
// The .invalidVerify fixtures are different in kind: not tampered, but
// SELF-CONSISTENT proofs of a FALSE statement, produced by running the real
// prover end-to-end on a witness that violates the relation (e.g. an A/B pair
// that is not a permutation). Such a proof is well-formed — PCS- and
// vanishing-valid — so the earlier stages have nothing to catch, and it is
// rejected specifically by the sub-verifier that checks the mathematical
// claim. Asserting the EXACT error pins which sub-verifier fires — e.g. the
// grandproduct permutation fixture must fail with error.ResultMismatch —
// proving that sub-verifier is exercised as the point of rejection.
test "self-consistent false-statement fixtures fail in the targeted sub-verifier" {
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasFailingVerify(i)) {
            checked += 1;
            const case = comptime vf.get(i);
            const input = vf.getInputFailingVerify(i);
            const expected = comptime vf.failingVerifyError(i);
            const res = verifier.verify(case.spec, case.systems, input.proof, input.public_inputs);
            if (res) |_| {
                // A well-formed proof of a false statement was accepted: the
                // targeted sub-verifier did not fire (its wiring is dead).
                std.debug.print("case {d} ({s}): self-consistent false-statement proof was ACCEPTED\n", .{ i, case.name });
                return error.FalseStatementAccepted;
            } else |err| {
                const got = @errorName(err);
                if (!std.mem.eql(u8, got, expected)) {
                    std.debug.print("case {d} ({s}): expected {s}, got {s}\n", .{ i, case.name, expected, got });
                    return error.UnexpectedSubVerifierError;
                }
            }
        }
    }
    try std.testing.expect(checked > 0);
}

fn caseIndex(comptime name: []const u8) usize {
    inline for (0..vf.case_count) |i| {
        if (comptime std.mem.eql(u8, vf.get(i).name, name)) return i;
    }
    @compileError("no verify fixture case named " ++ name);
}

const rs_blowup_cases = [_][]const u8{
    "DynamicFibonacciMultiSizeBlowup4",
    "DynamicFibonacciMultiSizeBlowup8",
    "DynamicFibonacciMultiSizeBlowup16",
};

fn withLogPlaintextSize(comptime systems: verifier.Systems, comptime log_plaintext_size: u8) verifier.Systems {
    var s = systems;
    s.pcs.envelope_params.log_plaintext_size = log_plaintext_size;
    return s;
}

// RS blowup is a per-System PCS option (#4069). The honest sweeps above verify
// each DynamicFibonacciMultiSizeBlowup* proof under its own System; here the
// SAME System with only the rate changed (log_plaintext_size +-1; query count
// and cap height untouched) must reject it, so a verifier that ignored the
// baked rate could not pass.
test "rs blowup: a proof is rejected under the same System at another rate" {
    inline for (rs_blowup_cases) |name| {
        const i = comptime caseIndex(name);
        const case = comptime vf.get(i);
        const input = vf.getInput(i);
        const log_plaintext = comptime case.systems.pcs.envelope_params.log_plaintext_size;
        inline for ([_]i16{ -1, 1 }) |delta| {
            const other = comptime withLogPlaintextSize(case.systems, @intCast(@as(i16, log_plaintext) + delta));
            try std.testing.expectError(error.InvalidCap, verifier.verify(case.spec, other, input.proof, input.public_inputs));
        }
    }
}

// The largest committable size shrinks with the blowup (the field's 2-adicity
// caps the codeword at 2^24), so blowup 8 and 16 must reject a dynamic size
// that blowup 2 and 4 still accept. The honest fixtures stay far below the
// bound, so claim one power of two above it. Blowup 4 shares blowup 2's
// max_size_log2, so only 8 and 16 apply.
test "rs blowup: a dynamic size above the rate's committable bound is rejected" {
    const base = comptime vf.get(caseIndex("DynamicFibonacciMultiSize"));
    inline for (rs_blowup_cases[1..]) |name| {
        const i = comptime caseIndex(name);
        const case = comptime vf.get(i);
        const input = vf.getInput(i);
        const max_size_log2 = comptime case.systems.pcs.max_size_log2;
        comptime std.debug.assert(max_size_log2 < base.systems.pcs.max_size_log2);
        const sizes = [_]usize{@as(usize, 1) << (max_size_log2 + 1)};
        var proof = input.proof;
        proof.module_sizes = &sizes;
        try std.testing.expectError(error.LayoutOverflow, verifier.verify(case.spec, case.systems, proof, input.public_inputs));
    }
}

test "multi-size: one baked System verifies proofs of two dynamic sizes" {
    // The runtime-size-reconstructed PCS layout: a committed-dynamic-column
    // protocol is proven at two DIFFERENT dynamic-module sizes, and BOTH proofs
    // verify against the SAME comptime System (case.systems). The verifier
    // reconstructs each proof's canonical layout from the shared ColumnDesc list
    // + the proof's own module_sizes, so the bundle placement / entry order /
    // restricted FRI params adapt per size. Without the reconstruction, the alt
    // (larger) size would land in a different bundle and fail.
    var checked: usize = 0;
    inline for (0..vf.case_count) |i| {
        if (comptime vf.hasAlt(i)) {
            checked += 1;
            const case = comptime vf.get(i);
            // Primary size.
            const input = vf.getInput(i);
            verifier.verify(case.spec, case.systems, input.proof, input.public_inputs) catch |err| {
                std.debug.print("multi-size case {d} ({s}) primary failed: {s}\n", .{ i, case.name, @errorName(err) });
                return err;
            };
            // Alternate size — SAME System. hasAlt means "different size", so
            // pin that the fixture really is one: a same-size proof here would
            // let this sweep pass without exercising layout reconstruction.
            const alt_input = vf.getInputAlt(i);
            try std.testing.expect(!std.mem.eql(usize, input.proof.module_sizes, alt_input.proof.module_sizes));
            verifier.verify(case.spec, case.systems, alt_input.proof, alt_input.public_inputs) catch |err| {
                std.debug.print("multi-size case {d} ({s}) alt size failed: {s}\n", .{ i, case.name, @errorName(err) });
                return err;
            };
        }
    }
    try std.testing.expect(checked > 0);
}

// Note: there is no "empty protocol" verify test — PCS is mandatory, so `verify`
// always indexes a zeta coin, which a zero-coin spec cannot provide.
test "verify rejects proof with wrong round count" {
    const spec = protocol.Spec{
        .round_coin_counts = &[_]usize{ 0, 1 },
        .round_coin_offsets = &[_]usize{ 0, 0 },
        .total_round_coins = 1,
    };
    const systems = verifier.Systems{
        .public_input = .{ .round_cell_counts = &[_]usize{0} },
        .vanishing = vanishing.System{ .modules = &.{} },
        .pcs = empty_pcs_system,
    };
    try std.testing.expectError(
        error.InvalidRoundCount,
        verifier.verify(spec, systems, .{
            .rounds = &.{},
            .pcs_opening = empty_pcs_opening,
        }, &.{}),
    );
}

// Robustness: a sub-verifier reading a transcript cell whose (round, index) ref
// (trusted, from the compiled System) points past the PROOF's actual rounds/cells
// slice must return CellRefOutOfRange, not read out of bounds. In R5 ReleaseSmall
// builds bounds checks are off, so an unbounded proof-driven index would be an OOB
// read; Context.cell() must guard it. Exercised directly on the logderiv
// sub-verifier (the same Context.cell path vanishing's cell_value uses).
test "cell ref past the proof's cells slice is rejected, not read OOB" {
    // A query whose result_ref points at cell index 3, but the round supplies
    // only one cell.
    const query = logderivativesum.Query{
        .entries = &.{},
        .result_ref = .{ .round = 0, .index = 3 },
    };
    const system = logderivativesum.System{ .queries = &.{query} };

    const cells = [_]protocol.Scalar{.{ .base = field.Element.init(1) }};
    const rounds = [_]protocol.RoundMessage{.{ .cells = &cells }};
    const ctx = protocol.Context{ .all_coins = &.{}, .rounds = &rounds };

    try std.testing.expectError(error.CellRefOutOfRange, logderivativesum.verify(system, ctx, &.{}));

    // And a ref past the rounds slice entirely.
    const query2 = logderivativesum.Query{
        .entries = &.{},
        .result_ref = .{ .round = 5, .index = 0 },
    };
    const system2 = logderivativesum.System{ .queries = &.{query2} };
    try std.testing.expectError(error.CellRefOutOfRange, logderivativesum.verify(system2, ctx, &.{}));
}

// A degenerate PCS system/opening: no batches, no layout, no claims. `verify`
// reaches replayWithTranscript (which errors here on the bad round count) before
// touching PCS, so an empty system suffices. num_batches == 0 makes resolveRoots
// fill a zero-length roots array.
const empty_pcs_system = pcs.System{
    .envelope_params = .{ .log_codeword_size = 1, .log_plaintext_size = 0, .num_queries = 1, .merkle_cap_height = 0 },
    .columns = &.{},
    .num_batches = 0,
    .max_entries = 0,
    .max_size_log2 = 0,
    .zeta_coin_index = 0,
};
const empty_pcs_opening = verifier.PcsOpening{
    .proof = .{ .input_queries = &.{}, .fri_proof = .{ .round_roots = &.{}, .final_poly = &.{ext.Ext.zero()}, .running_queries = &.{} } },
};
