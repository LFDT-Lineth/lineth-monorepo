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
//      the full compileFullPipeline (Go, gen-time) → real proof → serialized
//      verify.zig → verifier.verify chain. Honest proofs must verify; tampered
//      ones must be rejected.
//   2. The round-count guard needs no fixture: `verify` must reject a proof
//      whose round count disagrees with the compiled spec, during transcript
//      replay (before PCS runs).
//
// (The PCS-authenticated challenge derivation `verify` relies on is pinned
// separately in pcs_test.zig.)

test "all fixture cases: honest proofs verify end-to-end" {
    inline for (0..vf.case_count) |i| {
        const case = comptime vf.get(i);
        verifier.verify(case.spec, case.systems, vf.getInput(i)) catch |err| {
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
            const proof = vf.getInputFailing(i);
            const err = verifier.verify(case.spec, case.systems, proof);
            if (!std.meta.isError(err)) {
                std.debug.print("case {d} ({s}) accepted a tampered proof\n", .{ i, case.name });
                return error.TamperedProofAccepted;
            }
        }
    }
    // Guard against the sweep silently checking nothing (e.g. if hasFailing
    // regressed to all-false): at least the vanishing scenarios carry failing
    // inputs.
    try std.testing.expect(checked > 0);
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
            verifier.verify(case.spec, case.systems, vf.getInput(i)) catch |err| {
                std.debug.print("multi-size case {d} ({s}) primary failed: {s}\n", .{ i, case.name, @errorName(err) });
                return err;
            };
            // Alternate size — SAME System.
            verifier.verify(case.spec, case.systems, vf.getInputAlt(i)) catch |err| {
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
        .vanishing = vanishing.System{ .modules = &.{} },
        .pcs = empty_pcs_system,
    };
    try std.testing.expectError(
        error.InvalidRoundCount,
        verifier.verify(spec, systems, .{
            .rounds = &.{},
            .pcs_opening = empty_pcs_opening,
        }),
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
        .z_final_refs = &.{},
        .result_ref = .{ .round = 0, .index = 3 },
    };
    const system = logderivativesum.System{ .queries = &.{query} };

    const cells = [_]protocol.Scalar{.{ .base = field.Element.init(1) }};
    const rounds = [_]protocol.RoundMessage{.{ .cells = &cells }};
    const ctx = protocol.Context{ .all_coins = &.{}, .rounds = &rounds };

    try std.testing.expectError(error.CellRefOutOfRange, logderivativesum.verify(system, ctx));

    // And a ref past the rounds slice entirely.
    const query2 = logderivativesum.Query{
        .z_final_refs = &.{},
        .result_ref = .{ .round = 5, .index = 0 },
    };
    const system2 = logderivativesum.System{ .queries = &.{query2} };
    try std.testing.expectError(error.CellRefOutOfRange, logderivativesum.verify(system2, ctx));
}

// A degenerate PCS system/opening: no batches, no layout, no claims. `verify`
// reaches replayWithTranscript (which errors here on the bad round count) before
// touching PCS, so an empty system suffices. num_batches == 0 makes resolveRoots
// fill a zero-length roots array.
const empty_pcs_system = pcs.System{
    .envelope_params = .{ .log_codeword_size = 1, .log_plaintext_size = 0, .num_queries = 1 },
    .columns = &.{},
    .num_batches = 0,
    .max_entries = 0,
    .max_size_log2 = 0,
    .zeta_coin_index = 0,
};
const empty_pcs_opening = verifier.PcsOpening{
    .entry_claims = &.{},
    .proof = .{ .input_queries = &.{}, .fri_proof = .{ .round_roots = &.{}, .final_poly = &.{ext.Ext.zero()}, .running_queries = &.{} } },
};
