const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const ext = verifier_ray.field.koalabear_ext;
const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;

// End-to-end rejection of a corrupted LogDerivativeSum AVERAGE CELL.
//
// The average cell C of a Z column (read by logderivativesum's entries, which
// check Σ n·C == Result) is a RAW transcript cell: Fiat-Shamir-absorbed, but
// not itself PCS-authenticated. It is defended in two layers:
//   1. Fiat-Shamir: C is absorbed into the transcript, so changing it
//      re-derives different challenges and query positions, and the PCS
//      Merkle authentication fails (MerkleProofInvalid) before any query
//      sub-verifier runs.
//   2. The cyclic recurrence zNum − (Z − Z_shifted + C)·zDen == 0, whose column
//      claims ARE PCS-authenticated: summed over the rows, it makes n·C the
//      column's total, so a C that is not would fail the quotient identity
//      (QuotientIdentityMismatch).
//
// This test takes a REAL protocol (verify fixture case
// "SingleFractionAllOnes"), corrupts ONLY the raw average cell, and checks
// that the full verifier rejects. In practice layer 1 rejects first, so this
// test does not exercise layer 2: it would pass even if the recurrence did not
// read C. Layer 2 is checked in isolation, with the honest coins, by
// vanishing_test.zig ("average cell is bound by the cyclic recurrence").
//
// Resolve the fixture index by NAME so fixture regeneration (which shifts case
// ordering) never silently points this test at the wrong scenario; a missing
// scenario is a comptime error.
const case_index: usize = caseIndexByName("SingleFractionAllOnes");

fn caseIndexByName(comptime name: []const u8) usize {
    comptime {
        for (0..vf.case_count) |i| {
            if (std.mem.eql(u8, vf.get(i).name, name)) return i;
        }
        @compileError("pcs_average_binding_test: fixture case '" ++ name ++ "' not found; regenerate testdata or update the test");
    }
}

// spec/systems are comptime protocol descriptions; bind them as comptime
// constants (verifier.verify takes them as comptime params). The proof is a
// runtime value.
const spec = vf.get(case_index).spec;
const systems = vf.get(case_index).systems;

test "average cell binding: honest proof verifies" {
    const input = vf.getInput(case_index);
    try verifier.verify(spec, systems, input.proof, input.public_inputs);
}

test "average cell binding: corrupting the raw average cell is rejected" {
    const input = vf.getInput(case_index);

    // Locate the average cell from the logderiv system's ref, so this test
    // tracks the fixture even if the round/index shifts on regeneration.
    const ref = systems.logderivativesum.queries[0].entries[0].average_ref;

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // Copy the round messages so we can mutate one cell without touching the
    // (const) fixture data. Only the target round's cells slice needs its own
    // backing array; the rest alias the originals.
    const rounds_buf = try a.dupe(protocol.RoundMessage, input.proof.rounds);
    const cells_buf = try a.dupe(protocol.Scalar, input.proof.rounds[ref.round].cells);

    // Flip the average cell to a value that is NOT the column's total over n.
    // Every other proof value stays honest; the changed cell alters the
    // transcript, and the recurrence that reads it no longer holds.
    cells_buf[ref.index] = .{ .ext = cells_buf[ref.index].toExt().add(ext.Ext.one()) };
    rounds_buf[ref.round].cells = cells_buf;

    var bad = input;
    bad.proof.rounds = rounds_buf;

    // Either layer may reject (see the header); in practice Fiat–Shamir does.
    const err = verifier.verify(spec, systems, bad.proof, bad.public_inputs);
    try std.testing.expect(std.meta.isError(err));
    if (err) |_| unreachable else |e| switch (e) {
        // Layer 1 (Fiat-Shamir): the corrupted cell re-derives different query
        // positions, so PCS Merkle authentication fails against the honest roots.
        // Layer 2 (vanishing): the recurrence ties the cell to the
        // PCS-authenticated column, so the quotient identity fails.
        error.MerkleProofInvalid, error.QuotientIdentityMismatch => {},
        else => {
            std.debug.print("unexpected rejection error: {s}\n", .{@errorName(e)});
            return error.TestUnexpectedError;
        },
    }
}
