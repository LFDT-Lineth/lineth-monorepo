const protocol = @import("protocol/root.zig");
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const pcs = @import("pcs/verify.zig");
const ext = @import("field/koalabear_ext.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const profiling = @import("profiling.zig");
// TODO(new-sub-verifier): add import here — step 1 below.

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
//  Nothing else changes: protocol.Spec, protocol.replayWithTranscript, and all
//  existing sub-verifiers are untouched.
// ─────────────────────────────────────────────────────────────────────────────

/// Compiled systems for every sub-verifier in the protocol.
/// One field per sub-verifier; each holds the comptime metadata for that query.
pub const Systems = struct {
    vanishing: vanishing.System,
    logderivativesum: logderivativesum.System = .{},
    /// FRI/PCS opening-proof verifier. Mandatory and non-defaulted: PCS is what
    /// authenticates the committed claims the query sub-verifiers consume, so a
    /// protocol has no verifiable shape without it. Omitting this field is a
    /// compile error, which is deliberate — there is no "PCS-disabled" System.
    pcs: pcs.System,
    // TODO(new-sub-verifier): add compiled system field here — step 2 above.
};

/// Proof is the verifier-visible transcript consumed by `verify` in one pass.
/// It is the verifier-ray analogue of prover-ray's `wiop.Proof`: a
/// self-contained bundle of exactly the data a verifier is entitled to see.
///
/// Protocol-level round messages (public columns + cells) are shared across
/// every sub-verifier. Sub-verifier-specific claim slices are routed only to
/// the verifier that registered them. Coins are not stored here — they are
/// re-derived deterministically by `protocol.replayWithTranscript` from the
/// round messages.
pub const Proof = struct {
    rounds: []const protocol.RoundMessage,
    /// Per-module domain sizes for dynamically-sized vanishing modules.
    /// Must be populated when the compiled system has dynamic modules;
    /// defaults to an empty slice, which produces `MissingDynamicModuleSize`
    /// if any dynamic module is present.
    module_sizes: []const usize = &.{},
    /// The PCS opening the vanishing witness/quotient claims are re-sliced from.
    /// There is no "raw claims" alternative: claims must be PCS-authenticated,
    /// so the two sub-verifiers provably consume the same values. This makes
    /// "feed PCS and vanishing different values" unrepresentable.
    claims: PcsClaims,
    // TODO(new-sub-verifier): add claim fields here if needed — step 3 above.
};

/// The proof-carried PCS data: the claimed evaluations and the opening proof.
///
/// It deliberately does NOT carry `roots` or `zeta`. Those are the two values a
/// prover could otherwise choose to unbind the opening from the transcript:
///   - `roots` — the Merkle trust anchor — is built by `verify` from the
///     transcript-bound `System.batch_roots` (the round oracle commitments that
///     were absorbed to derive the challenges), never supplied here.
///   - `zeta` — the opening point — is the Fiat-Shamir coin
///     `all_coins[system.zeta_coin_index]`, likewise built by `verify`.
/// So the whole malleable-input surface is removed at the type level rather than
/// checked: a proof simply has no field for either. That leaves `entry_claims`
/// and the opening proof itself, both authenticated by the FRI opening against
/// the transcript-bound roots — nothing a prover carries here can make a false
/// opening verify. (The low-level `pcs.Inputs` still bundles all three, because
/// the FRI engine is protocol-agnostic; `verify` assembles it below.)
pub const PcsClaims = struct {
    /// Claimed evaluations, one inner slice per opened column in canonical (flat)
    /// layout order; inner slice length == that column's shift count.
    entry_claims: []const []const ext.Ext,
    proof: pcs.Proof,
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

    // Step 1 — replay transcript, derive all protocol coins. The transcript is
    // owned here and threaded by pointer through each phase: `protocol` absorbs
    // the round messages + squeezes the protocol coins, leaving it at the state
    // a transcript-continuing sub-verifier (PCS) resumes from. `protocol` stays
    // FRI-agnostic — the FRI squeeze schedule lives in `pcs.replayWithTranscript`.
    var transcript = fiat_shamir.Transcript.init();
    const all_coins = try protocol.replayWithTranscript(&transcript, spec, proof.rounds, proof.module_sizes);
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.transcript_done, 0);

    // Step 2 — assemble the shared context routed to every sub-verifier.
    const ctx = protocol.Context{
        .all_coins = &all_coins,
        .rounds = proof.rounds,
    };

    // Step 3 — dispatch each sub-verifier with ctx + its own claims.
    // TODO(new-sub-verifier): add dispatch call here — step 4 above.

    // PCS runs first: it authenticates the committed claims the query
    // sub-verifiers below consume. PCS is mandatory (see `Systems.pcs`), so
    // there is no trust-input fallback — the claims always come from a checked
    // opening.
    //
    // The vanishing witness/quotient claims are re-derived from the
    // PCS-authenticated `entry_claims` (never read from `proof`), closing the
    // gap where a prover could feed PCS and vanishing different values for the
    // same column. Storage for the derived claims is comptime-bounded by the
    // vanishing System's claim totals, so nothing allocates.
    var derived_witness: [systems.vanishing.total_witness_claims]ext.Ext = undefined;
    var derived_quotient: [systems.vanishing.total_quotient_claims]ext.Ext = undefined;

    // Assemble the low-level `pcs.Inputs` from three transcript-bound sources —
    // none taken from the proof's own copy — so a prover has no field to choose:
    //   - roots: the batch Merkle roots, rebuilt from `System.batch_roots` (the
    //     round oracle commitments absorbed to derive the challenges). If a batch
    //     were authenticated against a proof-supplied root instead, a prover could
    //     open against a forged root while zeta stays bound to the honest
    //     commitment. Storage is comptime-bounded by num_batches — no allocation.
    //   - zeta: the Fiat-Shamir opening coin `all_coins[zeta_coin_index]`.
    //   - entry_claims: the only proof-carried field, authenticated by the FRI
    //     opening against those transcript-bound roots.
    var bound_roots: [systems.pcs.num_batches]pcs.Octuplet = undefined;
    try resolveRoots(systems.pcs.batch_roots, proof.rounds, &bound_roots);
    const inputs = pcs.Inputs{
        .roots = &bound_roots,
        .entry_claims = proof.claims.entry_claims,
        .zeta = all_coins[systems.pcs.zeta_coin_index],
    };

    // Derive the FRI challenges by continuing the SAME transcript (PCS owns its
    // squeeze schedule), then check the proof against them. Derivation and
    // checking are separate calls: the first touches the transcript, the second
    // is pure arithmetic.
    const pcs_coins = pcs.replayWithTranscript(systems.pcs, &transcript, proof.claims.proof);
    try pcs.verify(systems.pcs, inputs, proof.claims.proof, pcs_coins);

    // Route each authenticated entry_claim to the vanishing claim slot that
    // consumes it (same column at zeta, per the codegen-emitted maps).
    try routeClaims(systems.pcs.witness_map, inputs.entry_claims, &derived_witness);
    try routeClaims(systems.pcs.quotient_map, inputs.entry_claims, &derived_quotient);

    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.vanishing_start, 0);
    try vanishing.verify(systems.vanishing, .{
        .ctx = ctx,
        .witness_claims = &derived_witness,
        .quotient_claims = &derived_quotient,
        .module_sizes = proof.module_sizes,
    });
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.vanishing_done, 0);

    try logderivativesum.verify(systems.logderivativesum, ctx);

    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.logderivativesum_done, profiling.snapshot().poseidon2_compress);
    // TODO(new-sub-verifier): dispatch here — step 4 above.
    // TODO(profiling): add a final verify_done marker once more phases run after logderivativesum.
}

/// Fills `out` with the authenticated claim each `map` entry points at:
/// `out[k] = entry_claims[map[k].entry][map[k].shift]`. `map.len` must equal
/// `out.len` (the vanishing System's claim total) and every ClaimRef must be in
/// range, else the PCS/vanishing metadata disagree — a codegen bug, surfaced as
/// an error rather than an out-of-bounds panic.
fn routeClaims(
    map: []const pcs.ClaimRef,
    entry_claims: []const []const ext.Ext,
    out: []ext.Ext,
) error{ClaimMapMismatch}!void {
    if (map.len != out.len) return error.ClaimMapMismatch;
    for (map, out) |ref, *slot| {
        if (ref.entry >= entry_claims.len) return error.ClaimMapMismatch;
        const col = entry_claims[ref.entry];
        if (ref.shift >= col.len) return error.ClaimMapMismatch;
        slot.* = col[ref.shift];
    }
}

/// Fills `out[b]` with batch `b`'s authenticated Merkle root, resolved from its
/// transcript-bound provenance (`batch_roots[b]`) — NOT from the proof. An
/// interactive batch's root is the sole oracle commitment of the round message
/// it names (the same octuplet absorbed to derive zeta); a precomputed batch's
/// root is the compile-time constant. This is the verifier-ray analogue of
/// prover-ray's `collectRoots` reading `rt.Commitments`, so the root a batch is
/// authenticated against is provably the one zeta is bound to.
///
/// `batch_roots.len` must equal `out.len` (== num_batches). A `.round` entry must
/// name a round message that exists and carries exactly one oracle commitment;
/// otherwise the PCS/protocol metadata disagree — surfaced as an error, not an
/// out-of-bounds panic or a silently mis-bound root.
fn resolveRoots(
    batch_roots: []const pcs.BatchRoot,
    rounds: []const protocol.RoundMessage,
    out: []pcs.Octuplet,
) error{BatchRootMismatch}!void {
    if (batch_roots.len != out.len) return error.BatchRootMismatch;
    for (batch_roots, out) |br, *slot| {
        switch (br) {
            .precomputed => |root| slot.* = root,
            .round => |round_index| {
                if (round_index >= rounds.len) return error.BatchRootMismatch;
                const cols = rounds[round_index].columns;
                // A committed interactive round carries exactly one oracle
                // commitment (its batch Merkle root); anything else is a metadata
                // mismatch, not an honest proof.
                if (cols.len != 1) return error.BatchRootMismatch;
                switch (cols[0]) {
                    .oracle_commitment => |root| slot.* = root,
                    .public_column => return error.BatchRootMismatch,
                }
            },
        }
    }
}
