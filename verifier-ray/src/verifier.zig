const protocol = @import("protocol/root.zig");
const vanishing = @import("query/vanishing.zig");
const ext = @import("field/koalabear_ext.zig");
// TODO(new-sub-verifier): add import here — step 1 below.

// ── Adding a new sub-verifier (e.g. logderiv, rangecheck) ────────────────────
//
//  This file is the only place that needs to change. Four steps, in order:
//
//  1. Import the new query module at the top of this file:
//       const logderiv = @import("query/logderiv.zig");
//
//  2. Add its compiled system to `Systems`:
//       pub const Systems = struct {
//           vanishing: vanishing.System,
//           logderiv:  logderiv.System,   // ← add
//       };
//
//  3. Add its runtime claim fields to `ProofData`:
//       pub const ProofData = struct {
//           ...
//           logderiv_claims: []const ext.Ext,   // ← add
//       };
//
//  4. Add a dispatch call in `verify` step 3 — ctx is already built:
//       try logderiv.verify(systems.logderiv, .{
//           .ctx    = ctx,
//           .claims = proof.logderiv_claims,
//       });
//
//  Nothing else changes: protocol.Spec, protocol.replay, and all existing
//  sub-verifiers are untouched.
// ─────────────────────────────────────────────────────────────────────────────

/// Compiled systems for every sub-verifier in the protocol.
/// One field per sub-verifier; each holds the comptime metadata for that query.
pub const Systems = struct {
    vanishing: vanishing.System,
    // TODO(new-sub-verifier): add field here — step 2 above.
};

/// All proof data consumed by the verifier in one pass.
///
/// Protocol-level round messages are shared across every sub-verifier.
/// Sub-verifier-specific claim slices are routed only to the verifier that
/// registered them.
pub const ProofData = struct {
    rounds: []const protocol.RoundMessage,
    // vanishing claims
    witness_claims: []const ext.Ext,
    quotient_claims: []const ext.Ext,
    /// Per-module domain sizes for dynamically-sized vanishing modules.
    /// Must be populated when the compiled system has dynamic modules;
    /// defaults to an empty slice, which produces `MissingDynamicModuleSize`
    /// if any dynamic module is present.
    module_sizes: []const usize = &.{},
    // TODO(new-sub-verifier): add claim fields here — step 3 above.
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
    proof: ProofData,
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
    // TODO(new-sub-verifier): add dispatch call here — step 4 above.
    try vanishing.verify(systems.vanishing, .{
        .ctx = ctx,
        .witness_claims = proof.witness_claims,
        .quotient_claims = proof.quotient_claims,
        .module_sizes = proof.module_sizes,
    });
}
