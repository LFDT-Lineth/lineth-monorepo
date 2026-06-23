const protocol = @import("protocol/root.zig");
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const ext = @import("field/koalabear_ext.zig");
// TODO(new-sub-verifier): add import here

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
//  Nothing else changes: protocol.Spec, protocol.replay, and all existing
//  sub-verifiers are untouched.
// ─────────────────────────────────────────────────────────────────────────────

/// Compiled systems for every sub-verifier in the protocol.
/// One field per sub-verifier; each holds the comptime metadata for that query.
pub const Systems = struct {
    vanishing: vanishing.System,
    logderivativesum: logderivativesum.System = .{},
    // TODO(new-sub-verifier): add compiled system field here — step 2 above.
};

/// Proof is the verifier-visible transcript consumed by `verify` in one pass.
/// It is the verifier-ray analogue of prover-ray's `wiop.Proof`: a
/// self-contained bundle of exactly the data a verifier is entitled to see.
///
/// Protocol-level round messages (public columns + cells) are shared across
/// every sub-verifier. Sub-verifier-specific claim slices are routed only to
/// the verifier that registered them. Coins are not stored here — they are
/// re-derived deterministically by `protocol.replay` from the round messages.
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
    // TODO(new-sub-verifier): add claim fields here if needed — step 3 above.
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
    // Step 1 — replay transcript, derive all coins. `replay` comptime-validates
    // `spec` internal consistency and returns the stack-allocated coin array.
    const all_coins = try protocol.replay(spec, proof.rounds);

    // Step 2 — assemble the shared context routed to every sub-verifier.
    const ctx = protocol.Context{
        .all_coins = &all_coins,
        .rounds = proof.rounds,
    };

    // Step 3 — dispatch each sub-verifier with ctx + its own claims.
    try vanishing.verify(systems.vanishing, .{
        .ctx = ctx,
        .witness_claims = proof.witness_claims,
        .quotient_claims = proof.quotient_claims,
        .module_sizes = proof.module_sizes,
    });
    try logderivativesum.verify(systems.logderivativesum, ctx);
    // TODO(new-sub-verifier): dispatch here — step 4 above.
}
