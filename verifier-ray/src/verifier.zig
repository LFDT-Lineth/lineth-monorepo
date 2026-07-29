const protocol = @import("protocol/root.zig");
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const pcs = @import("query/pcs.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const ext = @import("field/koalabear_ext.zig");
// TODO(new-sub-verifier): add import here — step 1 below.
const profiling = @import("profiling.zig");

// ── Adding a new sub-verifier ─────────────────────────────────────────────────
//
//  This file is the only place that needs to change. Steps, in order:
//
//  1. Import the new query module at the top of this file:
//       const sub_verifier = @import("query/sub_verifier.zig");
//
//  2. Add its compiled system to `Systems`:
//       pub const Systems = struct {
//           vanishing:   vanishing.System,
//           sub_verifier: sub_verifier.System,   // ← add
//       };
//
//  3. Add its proof claims to `Proof`:
//       pub const Proof = struct {
//           ...
//           sub_verifier_claims: []const ext.Ext,   // ← add
//       };
//     Some sub-verifiers need no extra proof data and can omit this step.
//
//  4. Add a dispatch call in `verify` step 3 — ctx is already built:
//       try sub_verifier.verify(systems.sub_verifier, .{
//           .ctx    = ctx,
//           .claims = proof.sub_verifier_claims,
//       });
//
//  Nothing else changes: protocol.Spec, protocol.replayWithTranscript, and all existing
//  sub-verifiers are untouched.
// ─────────────────────────────────────────────────────────────────────────────

/// Compiled systems for every sub-verifier in the protocol.
/// One field per sub-verifier; each holds the comptime metadata for that query.
pub const Systems = struct {
    vanishing: vanishing.System,
    logderivativesum: logderivativesum.System = .{},
    /// FRI/PCS opening verifier. Optional: `null` for protocols with no
    /// polynomial commitment (the pre-PCS shape, still used by many test
    /// fixtures). When present, `verify` derives the PCS opening coins (zeta,
    /// fold challenges, query positions) from the shared Fiat-Shamir transcript
    /// and checks the opening — the coins are never taken from the proof.
    pcs: ?pcs.System = null,
    // TODO(new-sub-verifier): add compiled system field here — step 2 above.
};

/// Proof is the verifier-visible transcript consumed by `verify` in one pass.
/// It is the verifier-ray analogue of prover-ray's `wiop.Proof`: a
/// self-contained bundle of exactly the data a verifier is entitled to see.
///
/// Protocol-level round messages (public columns + cells) are shared across
/// every sub-verifier. Sub-verifier-specific claim slices are routed only to
/// the verifier that registered them. Coins are not stored here — they are
/// re-derived deterministically by `protocol.replayWithTranscript` from the round messages.
pub const Proof = struct {
    rounds: []const protocol.RoundMessage,
    // vanishing claims
    witness_claims: []const ext.Ext,
    quotient_claims: []const ext.Ext,
    /// Per-module domain sizes for dynamically-sized vanishing modules.
    /// Must be populated when the compiled system has dynamic modules;
    /// defaults to an empty slice, which produces `MissingDynamicModuleSize`
    /// if any dynamic module is present.
    module_sizes: []const usize = &.{},
    /// The PCS opening, present iff `Systems.pcs` is set. Carries only the data
    /// the verifier is entitled to trust from the prover — the batch roots, the
    /// claimed evaluations, and the opening proof. The opening COINS (zeta, fold
    /// challenges, query positions) are NOT here: `verify` derives them from the
    /// transcript so the prover cannot choose them.
    pcs_opening: ?PcsOpening = null,
    // TODO(new-sub-verifier): add claim fields here if needed — step 3 above.
};

/// The prover-supplied half of a PCS opening (everything except the
/// Fiat-Shamir-derived coins). See `Proof.pcs_opening`.
pub const PcsOpening = struct {
    roots: []const poseidon2.Digest,
    claimed_values: []const ext.Ext,
    proof: pcs.OpeningProof,
};

/// Verifies a proof against the compiled protocol in three steps:
///
///   1. Replay   — absorb every round message into the shared Fiat-Shamir
///                 transcript and squeeze all coins deterministically.
///   2. Route    — wrap coins and bound round messages in a `protocol.Context`
///                 that every sub-verifier can read without owning the transcript.
///   3. Dispatch — call each sub-verifier with the shared context and its own
///                 claim slice. Sub-verifiers are independent of each other.
///
/// `spec` carries the protocol-level coin routing (shared across all
/// sub-verifiers). `systems` holds one compiled system per sub-verifier.
/// This is the only place in the codebase that knows the full list of
/// sub-verifiers.
pub fn verify(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    proof: Proof,
) !void {
    profiling.reset();
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.verify_start, 0);

    // Step 1 — replay transcript, derive all coins. The transcript is owned here
    // and threaded by pointer: `protocol` absorbs the round messages + squeezes
    // the protocol coins, leaving it at the state a transcript-continuing
    // sub-verifier (PCS, below) resumes from. `replayWithTranscript`
    // comptime-validates `spec` internal consistency and returns the
    // stack-allocated coin array.
    var transcript = fiat_shamir.Transcript.init();
    const all_coins = try protocol.replayWithTranscript(&transcript, spec, proof.rounds);
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.transcript_done, 0);

    // Step 2 — assemble the shared context routed to every sub-verifier.
    const ctx = protocol.Context{
        .all_coins = &all_coins,
        .rounds = proof.rounds,
    };

    // Step 3 — dispatch each sub-verifier with ctx + its own claims.
    // TODO(new-sub-verifier): add dispatch call here — step 4 above.

    // PCS (when present): continue the SAME transcript to derive the opening
    // challenges — zeta (the shared opening point), the FRI fold challenges, and
    // the query positions — then check the opening. Deriving them here (not
    // reading them from the proof) is the whole point: the prover cannot choose
    // the Fiat-Shamir challenges. Challenge derivation touches the transcript;
    // the check is pure arithmetic.
    if (comptime systems.pcs) |pcs_system| {
        const opening = proof.pcs_opening orelse return error.MissingPcsOpening;
        const pcs_challenges = pcs.deriveChallenges(pcs_system, &transcript, opening.proof.fri_proof);
        try pcs.verify(pcs_system, .{
            .roots = opening.roots,
            .claimed_values = opening.claimed_values,
            .zeta = all_coins[pcs_system.zeta_coin_index],
            .fold_alphas = &pcs_challenges.fold_alphas,
            .query_positions = &pcs_challenges.query_positions,
            .proof = opening.proof,
        });
    }

    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.vanishing_start, 0);
    try vanishing.verify(systems.vanishing, .{
        .ctx = ctx,
        .witness_claims = proof.witness_claims,
        .quotient_claims = proof.quotient_claims,
        .module_sizes = proof.module_sizes,
    });
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.vanishing_done, 0);

    try logderivativesum.verify(systems.logderivativesum, ctx);

    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.logderivativesum_done, profiling.snapshot().poseidon2_compress);
    // TODO(new-sub-verifier): dispatch here — step 4 above.
    // TODO(profiling): add a final verify_done marker once more phases run after logderivativesum.
}
