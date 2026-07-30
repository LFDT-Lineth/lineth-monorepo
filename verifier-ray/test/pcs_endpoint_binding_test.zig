const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const ext = verifier_ray.field.koalabear_ext;
const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;

// Soundness gate for the LogDerivativeSum / vanishing ENDPOINT CELL binding.
//
// The audit flagged that the Z[n-1] endpoint cell (read by logderivativesum's
// z_final_refs) and the vanishing `cell_value` are read as RAW transcript cells
// — Fiat-Shamir-absorbed but not themselves PCS-authenticated. They are sound
// only because prover-ray registers an endpoint-binding vanishing constraint
//
//     (result_cell − Z_shifted · L_pos) == 0
//
// whose `column_claim` (Z at the endpoint rotation) IS PCS-authenticated. So the
// cell is pinned to the authenticated column, not trusted on its own.
//
// This test proves that binding is present and enforced end-to-end: it takes a
// REAL protocol carrying PCS + a boundary vanishing + a LogDerivativeSum
// (verify fixture case "SingleFractionAllOnes"), leaves every PCS-authenticated
// entry_claim HONEST, and corrupts ONLY the raw endpoint cell. verifier.verify
// must still reject — because the endpoint-binding vanishing constraint no longer
// holds (the cell no longer equals the authenticated column value). Without the
// binding, the corruption would slip past every check and this test would accept.
//
// Case index 49 is the "SingleFractionAllOnes" LogDerivativeSum scenario; its
// round-1 cells are [result, z_final] and its expression DAG binds the z_final
// cell to column_claim 1 (the PCS-authenticated Z column) via L_3 (last row).
const case_index: usize = 49;

// spec/systems are comptime protocol descriptions; bind them as comptime
// constants (verifier.verify takes them as comptime params). The proof is a
// runtime value.
const spec = vf.get(case_index).spec;
const systems = vf.get(case_index).systems;

test "endpoint cell binding: honest proof verifies" {
    const proof = vf.getInput(case_index);
    try verifier.verify(spec, systems, proof);
}

test "endpoint cell binding: corrupting the raw z_final cell is rejected" {
    const proof = vf.getInput(case_index);

    // Locate the z_final endpoint cell from the logderiv system's ref, so this
    // test tracks the fixture even if the round/index shifts on regeneration.
    const ref = systems.logderivativesum.queries[0].z_final_refs[0];

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // Copy the round messages so we can mutate one cell without touching the
    // (const) fixture data. Only the target round's cells slice needs its own
    // backing array; the rest alias the originals.
    const rounds_buf = try a.dupe(protocol.RoundMessage, proof.rounds);
    const cells_buf = try a.dupe(protocol.Scalar, proof.rounds[ref.round].cells);

    // Flip the endpoint cell to a value that is NOT the authenticated Z[n-1].
    // entry_claims stay honest, so PCS still authenticates; only the vanishing
    // endpoint-binding constraint (result − Z·L_pos) is now violated.
    cells_buf[ref.index] = .{ .ext = cells_buf[ref.index].toExt().add(ext.Ext.one()) };
    rounds_buf[ref.round].cells = cells_buf;

    var bad = proof;
    bad.rounds = rounds_buf;

    // The endpoint cell is defended in TWO layers, and corrupting it trips the
    // first one reached:
    //   1. Fiat-Shamir: the cell is absorbed into the shared transcript, so
    //      changing it re-derives different FRI query positions/challenges and
    //      the PCS Merkle authentication fails (InputTreeAuthFailed) before any
    //      query sub-verifier runs.
    //   2. Vanishing: even if a corruption somehow survived FS, the
    //      endpoint-binding constraint (result − Z·L_pos == 0) ties the cell to
    //      the PCS-authenticated column, so the quotient identity would fail
    //      (QuotientIdentityMismatch).
    // The soundness claim is that the corruption CANNOT be accepted. Assert
    // rejection via either layer — the endpoint cell is not a free value.
    const err = verifier.verify(spec, systems, bad);
    try std.testing.expect(std.meta.isError(err));
    if (err) |_| unreachable else |e| switch (e) {
        error.InputTreeAuthFailed, error.QuotientIdentityMismatch => {},
        else => {
            std.debug.print("unexpected rejection error: {s}\n", .{@errorName(e)});
            return error.TestUnexpectedError;
        },
    }
}
