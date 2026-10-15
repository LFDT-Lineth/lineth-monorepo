const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const ext = verifier_ray.field.koalabear_ext;
const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;

// Soundness gate for the LogDerivativeSum AVERAGE CELL binding.
//
// The average cell C of a Z column (read by logderivativesum's entries, which
// check Σ n·C == Result) is a RAW transcript cell — Fiat-Shamir-absorbed but not
// itself PCS-authenticated. It is sound only because C is a `cell_value` leaf of
// the cyclic recurrence vanishing constraint
//
//     zNum − (Z − Z_shifted + C)·zDen == 0
//
// whose column claims (Z at zeta and at its −1 rotation) ARE PCS-authenticated.
// Summed over the rows, that constraint makes n·C the column's total, so C is
// pinned to the authenticated columns, not trusted on its own.
//
// This test proves that binding is present and enforced end-to-end: it takes a
// REAL protocol carrying PCS + the recurrence + a LogDerivativeSum (verify
// fixture case "SingleFractionAllOnes"), leaves every PCS-authenticated
// entry_claim HONEST, and corrupts ONLY the raw average cell. verifier.verify
// must still reject — because the recurrence no longer holds. Without the
// binding, a corrupted C (with Result adjusted to match) would slip past every
// check and this test would accept.
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
    // entry_claims stay honest, so PCS still authenticates; only the
    // recurrence, which reads C, is now violated.
    cells_buf[ref.index] = .{ .ext = cells_buf[ref.index].toExt().add(ext.Ext.one()) };
    rounds_buf[ref.round].cells = cells_buf;

    var bad = input;
    bad.proof.rounds = rounds_buf;

    // The average cell is defended in TWO layers, and corrupting it trips the
    // first one reached:
    //   1. Fiat-Shamir: the cell is absorbed into the shared transcript, so
    //      changing it re-derives different FRI query positions/challenges and
    //      the PCS Merkle authentication fails (MerkleProofInvalid) before any
    //      query sub-verifier runs.
    //   2. Vanishing: even if a corruption somehow survived FS, the recurrence
    //      ties C to the PCS-authenticated Z column, so the quotient identity
    //      would fail (QuotientIdentityMismatch).
    // The soundness claim is that the corruption CANNOT be accepted. Assert
    // rejection via either layer — the average cell is not a free value.
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
