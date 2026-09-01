const protocol = @import("../protocol/root.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const multiset_hashing = @import("../crypto/multiset_hashing.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    MissingRoundCommitment,
    ContributionMismatch,
} || protocol.CellError;

/// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
/// round is the proof.rounds index (0-based); index is the position within
/// that round's cells slice. Mirrors grandproduct.ScalarRef /
/// logderivativesum.ScalarRef.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

/// One round contributing to the shared-randomness sponge preimage. Mirrors
/// prover-ray's `for i := range rt.CurrentRound().ID { if
/// !rt.System.Rounds[i].HasCommitment { continue } ... }` loop: a round with
/// has_commitment == false carries no Octuplet to hash and is skipped
/// entirely, in round order.
pub const Round = struct {
    round: usize,
    has_commitment: bool,
};

/// System is the compiled metadata for a single
/// messagebus.SharedRandomnessContributionChecker: the ordered rounds whose
/// commitments feed the sponge, and the transcript cells carrying the claimed
/// multiset-hash contribution, one per limb (`multiset_hashing.size` limbs).
pub const System = struct {
    rounds: []const Round = &.{},
    contribution_refs: []const ScalarRef = &.{},
};

/// Verifies that the shared-randomness contribution public-input cells equal
/// `multisethashing.Hash` of the Poseidon2 Merkle-Damgård sponge digest over
/// every committed round preceding the message-bus coin round — the
/// verifier-side counterpart to prover-ray's `sharedRandomnessContribution` /
/// `SharedRandomnessContributionChecker.Check`.
///
/// Both the round commitments and the claimed-contribution cells are read
/// from `ctx` (the adversary's transcript), never from a baked-in
/// honest-prover value: `ctx.rounds[r].commitment` is the transcript-bound
/// Merkle root for round r (or null if that round never committed), and
/// `system.contribution_refs` name the (round, index) cells the claimed
/// digest limbs occupy — already merged from the public-input statement into
/// `ctx.rounds[*].cells` by `verifier.verify`'s call to `bindRoundMessages`
/// before any sub-verifier runs.
///
/// A `System{}` zero value (no rounds, no contribution_refs) verifies
/// trivially: a protocol compiled without
/// messagebus.CompileOptions.SharedRandomness registers no checker and has
/// nothing for this sub-verifier to enforce.
pub fn verify(comptime system: System, ctx: protocol.Context) Error!void {
    if (system.contribution_refs.len == 0) return;
    if (system.contribution_refs.len != multiset_hashing.size)
        @compileError("shared_randomness: contribution_refs must match multiset_hashing.size");

    var hasher = poseidon2.MDHasher.init();
    inline for (system.rounds) |round| {
        if (!round.has_commitment) continue;
        if (round.round >= ctx.rounds.len) return error.MissingRoundCommitment;
        const commitment = ctx.rounds[round.round].commitment orelse return error.MissingRoundCommitment;
        hasher.writeElements(&commitment);
    }
    const digest = hasher.sumDigest();
    const contribution = multiset_hashing.hash(digest);

    inline for (system.contribution_refs, 0..) |ref, i| {
        const claimed = (try ctx.cell(ref.round, ref.index)).toExt();
        if (!claimed.eql(ext.Ext.lift(contribution[i]))) return error.ContributionMismatch;
    }
}
