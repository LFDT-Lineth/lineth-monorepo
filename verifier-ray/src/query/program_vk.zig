const std = @import("std");
const protocol = @import("../protocol/root.zig");
const field = @import("../field/koalabear.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");

pub const Error = error{
    MissingRoundCommitment,
    ProgramVKMismatch,
    ProgramVKNotBaseField,
} || protocol.CellError;

/// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
/// round is the proof.rounds index (0-based); index is the position within
/// that round's cells slice. Mirrors shared_randomness.ScalarRef.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

/// The round whose PCS commitment is the program VK. Mirrors prover-ray's
/// `rt.Commitments[zkcdriver.ProgramRound]`. `has_commitment == false` means
/// that round committed no column, in which case the Go map lookup yields the
/// zero Octuplet and the prover publishes that — so the verifier must too.
pub const Round = struct {
    round: usize = 0,
    has_commitment: bool = false,
};

/// System is the compiled metadata for a single risc5.ProgramVKChecker: the
/// round whose commitment is the program VK, and the transcript cells carrying
/// the claimed VK, one per limb of the commitment.
pub const System = struct {
    commitment_round: Round = .{},
    vk_refs: []const ScalarRef = &.{},
};

/// Verifies that the program-VK public-input cells equal the PCS commitment to
/// the program round, limb by limb — the verifier-side counterpart to
/// prover-ray's `risc5.ProgramVKChecker.Check`.
///
/// Both the commitment and the claimed-VK cells are read from `ctx` (the
/// adversary's transcript), never from a baked-in honest-prover value:
/// `ctx.rounds[r].commitment` is the transcript-bound Merkle root for round r
/// (or null if that round never committed), and `system.vk_refs` name the
/// (round, index) cells the claimed VK limbs occupy — already merged from the
/// public-input statement into `ctx.rounds[*].cells` by `verifier.verify`'s
/// call to `bindRoundMessages` before any sub-verifier runs.
///
/// A `System{}` zero value (no vk_refs) verifies trivially: a protocol that
/// does not register the program VK has nothing for this sub-verifier to
/// enforce.
pub fn verify(system: System, ctx: protocol.Context) Error!void {
    if (system.vk_refs.len == 0) return;

    var commitment: poseidon2.Digest = @splat(field.Element.zero());
    // Codegen emits exactly one ref per commitment limb (see
    // BuildProgramVKSystem), so this is an invariant, not a proof-dependent
    // condition. It stays as an assert because the loop below indexes
    // `commitment` by the same counter.
    std.debug.assert(system.vk_refs.len == commitment.len);
    if (system.commitment_round.has_commitment) {
        if (system.commitment_round.round >= ctx.rounds.len) return error.MissingRoundCommitment;
        commitment = ctx.rounds[system.commitment_round.round].commitment orelse
            return error.MissingRoundCommitment;
    }

    for (system.vk_refs, 0..) |ref, i| {
        // The VK limbs are base-field by protocol contract: prover-ray's
        // programVKCell panics on an extension cell. Reject an ext-encoded limb
        // here too rather than lifting it.
        const claimed = switch (try ctx.cell(ref.round, ref.index)) {
            .base => |b| b,
            .ext => return error.ProgramVKNotBaseField,
        };
        if (!claimed.eql(commitment[i])) return error.ProgramVKMismatch;
    }
}
