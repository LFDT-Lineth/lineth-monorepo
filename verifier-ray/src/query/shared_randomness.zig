const protocol = @import("../protocol/root.zig");
const field = @import("../field/koalabear.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const multiset_hashing = @import("../crypto/multiset_hashing.zig");

pub const Error = error{
    MissingRoundCommitment,
    ContributionMismatch,
    ContributionNotBaseField,
} || protocol.CellError;

/// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
/// round is the proof.rounds index (0-based); index is the position within
/// that round's cells slice. Mirrors grandproduct.ScalarRef /
/// logderivativesum.ScalarRef.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

/// The round whose PCS commitment is the hash preimage. Mirrors prover-ray's
/// `com := rt.Commitments[rt.CurrentRound().ID]`: the coin round's own
/// commitment, and nothing else. `has_commitment == false` means that round
/// committed no column, in which case the Go map lookup yields the zero
/// Octuplet and the prover hashes that — so the verifier must too.
pub const Round = struct {
    round: usize = 0,
    has_commitment: bool = false,
};

/// System is the compiled metadata for a single
/// messagebus.SharedRandomnessContributionChecker: the round whose commitment
/// is hashed, and the transcript cells carrying the claimed multiset-hash
/// contribution, one per limb (`multiset_hashing.size` limbs).
pub const System = struct {
    commitment_round: Round = .{},
    contribution_refs: []const ScalarRef = &.{},
};

/// Verifies that the shared-randomness contribution public-input cells equal
/// `multisethashing.Hash` of the message-bus coin round's own PCS commitment —
/// the verifier-side counterpart to prover-ray's
/// `sharedRandomnessContribution` / `SharedRandomnessContributionChecker.Check`.
///
/// Both the commitment and the claimed-contribution cells are read from `ctx`
/// (the adversary's transcript), never from a baked-in honest-prover value:
/// `ctx.rounds[r].commitment` is the transcript-bound Merkle root for round r
/// (or null if that round never committed), and `system.contribution_refs` name
/// the (round, index) cells the claimed digest limbs occupy — already merged
/// from the public-input statement into `ctx.rounds[*].cells` by
/// `verifier.verify`'s call to `bindRoundMessages` before any sub-verifier runs.
///
/// A `System{}` zero value (no contribution_refs) verifies trivially: a
/// protocol compiled without messagebus.CompileOptions.SharedRandomness
/// registers no checker and has nothing for this sub-verifier to enforce.
pub fn verify(comptime system: System, ctx: protocol.Context) Error!void {
    if (system.contribution_refs.len == 0) return;
    if (system.contribution_refs.len != multiset_hashing.size)
        @compileError("shared_randomness: contribution_refs must match multiset_hashing.size");

    // A round that committed no column has no Octuplet to hash; prover-ray's
    // `rt.Commitments[...]` map lookup yields the zero value there, so hash
    // zeroes rather than erroring, or the two sides disagree.
    var commitment: poseidon2.Digest = @splat(field.Element.zero());
    if (system.commitment_round.has_commitment) {
        if (system.commitment_round.round >= ctx.rounds.len) return error.MissingRoundCommitment;
        commitment = ctx.rounds[system.commitment_round.round].commitment orelse
            return error.MissingRoundCommitment;
    }
    const contribution = multiset_hashing.hash(commitment);

    inline for (system.contribution_refs, 0..) |ref, i| {
        // The contribution limbs are base-field by protocol contract:
        // prover-ray's messagebus.contributionCell panics on an extension
        // cell, and the sibling gammaDigest path likewise rejects
        // ext-encoded cells. Reject an ext-encoded limb here too rather than
        // lifting it via toExt(), which would erase the base/ext distinction
        // and accept an encoding the protocol is meant to forbid.
        const claimed = switch (try ctx.cell(ref.round, ref.index)) {
            .base => |b| b,
            .ext => return error.ContributionNotBaseField,
        };
        if (!claimed.eql(contribution[i])) return error.ContributionMismatch;
    }
}
