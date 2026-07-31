const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;
const vanishing = verifier_ray.query.vanishing;
const pcs = verifier_ray.query.pcs;
const ext = verifier_ray.field.koalabear_ext;

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

// A degenerate PCS system/opening: no batches, no layout, no claims. `verify`
// reaches replayWithTranscript (which errors here on the bad round count) before
// touching PCS, so an empty system suffices. num_batches == 0 makes resolveRoots
// fill a zero-length roots array.
const empty_pcs_system = pcs.System{
    .params = .{ .log_codeword_size = 1, .log_plaintext_size = 0, .num_queries = 1 },
    .layout = &.{},
    .num_batches = 0,
    .zeta_coin_index = 0,
};
const empty_pcs_opening = verifier.PcsOpening{
    .entry_claims = &.{},
    .proof = .{ .input_queries = &.{}, .fri_proof = .{ .round_roots = &.{}, .final_poly = &.{ext.Ext.zero()}, .running_queries = &.{} } },
};
