const protocol = @import("protocol/root.zig");
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const pcs = @import("query/pcs.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const ext = @import("field/koalabear_ext.zig");
const profiling = @import("profiling.zig");

/// Compiled systems for every sub-verifier in the protocol.
/// One field per sub-verifier; each holds the comptime metadata for that query.
pub const Systems = struct {
    vanishing: vanishing.System,
    logderivativesum: logderivativesum.System = .{},
    /// FRI/PCS opening verifier. MANDATORY: there is no "PCS-disabled" protocol.
    /// `verify` runs PCS first — deriving the opening coins (zeta, fold
    /// challenges, query positions) from the shared Fiat-Shamir transcript and
    /// authenticating the committed claims — then feeds those *authenticated*
    /// claims into the vanishing check. So a prover can neither choose the
    /// challenges nor hand the two sub-verifiers different claim values.
    pcs: pcs.System,
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
    /// Per-module domain sizes for dynamically-sized vanishing modules.
    /// Must be populated when the compiled system has dynamic modules;
    /// defaults to an empty slice, which produces `MissingDynamicModuleSize`
    /// if any dynamic module is present.
    module_sizes: []const usize = &.{},
    /// The PCS opening. MANDATORY: the vanishing witness/quotient claims are
    /// re-sliced from its authenticated `entry_claims` (there is no "raw claims"
    /// alternative — that is exactly the gap this closes). Carries only the data
    /// the verifier is entitled to trust: the claimed evaluations and the opening
    /// proof. The batch roots and the opening COINS (zeta, fold challenges, query
    /// positions) are NOT here — `verify` rebuilds/derives them.
    pcs_opening: PcsOpening,
};

/// The prover-supplied half of a PCS opening (everything except the values the
/// verifier reconstructs on its own). See `Proof.pcs_opening`.
///
/// It deliberately does NOT carry `roots`: the batch Merkle roots are rebuilt by
/// `verify` from `pcs.System.batch_roots` — the transcript-bound round oracle
/// commitments (and compile-time precomputed roots) — never from the proof. If a
/// prover could supply roots here, it could open against a forged root while zeta
/// stays bound to the honest commitment. Coins (zeta, fold challenges, query
/// positions) are likewise absent and derived by `verify`.
pub const PcsOpening = struct {
    /// Per-opened-column claimed evaluations, jagged `[entry][shift]` in
    /// canonical layout order (see `pcs.VerifyInput.entry_claims`).
    entry_claims: []const []const ext.Ext,
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
    const all_coins = try protocol.replayWithTranscript(&transcript, spec, proof.rounds, proof.module_sizes);
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.transcript_done, 0);

    // Step 2 — assemble the shared context routed to every sub-verifier.
    const ctx = protocol.Context{
        .all_coins = &all_coins,
        .rounds = proof.rounds,
    };

    // Step 3 — PCS first. It continues the SAME transcript to derive the opening
    // challenges — zeta (the shared opening point), the FRI fold challenges, and
    // the query positions — then authenticates the committed claims. Deriving the
    // challenges here (not reading them from the proof) is the whole point: the
    // prover cannot choose the Fiat-Shamir challenges.
    const pcs_system = systems.pcs;
    const opening = proof.pcs_opening;

    // Rebuild the per-batch Merkle roots from their transcript-bound
    // provenance (round oracle commitments + compile-time precomputed roots),
    // NOT from the proof. `pcs.verify` will reorder/deduplicate these into the
    // proof's input-opening order, so the root each batch is authenticated
    // against is provably the same octuplet zeta is bound to. Mirrors
    // prover-ray's `collectRoots` + `inputOpeningRoots`.
    var bound_roots: [pcs_system.num_batches]poseidon2.Digest = undefined;
    try resolveRoots(pcs_system.batch_roots, proof.rounds, &bound_roots);

    // zeta is the Fiat-Shamir opening coin, never proof-supplied. Requiring the
    // index at comptime turns a mis-configured PCS system into a build error
    // instead of a silent wrong-coin selection.
    const zeta_index = comptime pcs_system.zeta_coin_index orelse
        @compileError("pcs: System.zeta_coin_index must be set");

    // Reconstruct the canonical PCS layout for THIS proof's dynamic sizes: one
    // baked comptime System verifies proofs of different module sizes because the
    // bundle placement / entry order / restricted params are a runtime function
    // of `module_sizes` (mirroring prover-ray's GetLayout + canonicalLayout +
    // restrictTo). deriveChallenges needs the restricted params for the fold /
    // query-position counts.
    const recon = try pcs.reconstruct(pcs_system, proof.module_sizes);

    const pcs_challenges = try pcs.deriveChallenges(pcs_system, recon, &transcript, opening.proof.fri_proof);
    try pcs.verify(pcs_system, .{
        .roots = &bound_roots,
        .entry_claims = opening.entry_claims,
        .zeta = all_coins[zeta_index],
        .fold_alphas = pcs_challenges.foldAlphas(),
        .deep_alpha = pcs_challenges.deep_alpha,
        .query_positions = pcs_challenges.query_positions[0..recon.params.num_queries],
        .proof = opening.proof,
        .module_sizes = proof.module_sizes,
    });

    // Route each PCS-authenticated entry_claim to the vanishing claim slot that
    // consumes it (same committed column at zeta, per the codegen-emitted maps),
    // so the vanishing check runs on values the FRI opening just proved — never
    // on raw proof-supplied claims. This closes the "feed the two sub-verifiers
    // different values for the same column" gap.
    var derived_witness: [systems.vanishing.total_witness_claims]ext.Ext = undefined;
    var derived_quotient: [systems.vanishing.total_quotient_claims]ext.Ext = undefined;
    try routeClaims(pcs_system, recon, pcs_system.witness_map, opening.entry_claims, &derived_witness);
    try routeClaims(pcs_system, recon, pcs_system.quotient_map, opening.entry_claims, &derived_quotient);

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
    // TODO(profiling): add a final verify_done marker once more phases run after logderivativesum.
}

/// Fills `out[k]` with the authenticated claim each `map` entry points at:
/// `out[k] = entry_claims[recon.col_to_entry[map[k].col_decl_idx]][map[k].shift]`.
/// `map.len` must equal `out.len` (the vanishing System's claim total) and every
/// ClaimRef must be in range, else the PCS/vanishing metadata disagree — a
/// codegen bug, surfaced as an error rather than an out-of-bounds panic.
fn routeClaims(
    comptime system: pcs.System,
    recon: pcs.Reconstructed(system),
    map: []const pcs.ClaimRef,
    entry_claims: []const []const ext.Ext,
    out: []ext.Ext,
) error{ClaimMapMismatch}!void {
    if (map.len != out.len) return error.ClaimMapMismatch;
    for (map, out) |ref, *slot| {
        // A ClaimRef names a column by its declaration index; the runtime
        // reconstruction resolves it to the canonical entry that column occupies
        // for THIS proof's sizes.
        if (ref.col_decl_idx >= system.columns.len) return error.ClaimMapMismatch;
        const entry = recon.col_to_entry[ref.col_decl_idx];
        if (entry >= entry_claims.len) return error.ClaimMapMismatch;
        const col = entry_claims[entry];
        if (ref.shift >= col.len) return error.ClaimMapMismatch;
        slot.* = col[ref.shift];
    }
}

/// Fills `out[b]` with batch `b`'s authenticated Merkle root, resolved from its
/// transcript-bound provenance (`batch_roots[b]`) — NOT from the proof. An
/// interactive batch's root is the sole oracle commitment of the round message it
/// names (the same octuplet absorbed to derive zeta); a precomputed batch's root
/// is the compile-time constant. This is the verifier-ray analogue of prover-ray's
/// `collectRoots` reading `rt.Commitments`, so the root a batch is authenticated
/// against is provably the one zeta is bound to.
///
/// `batch_roots.len` must equal `out.len` (== num_batches). A `.round` entry must
/// name a round message that exists and carries exactly one oracle commitment;
/// otherwise the PCS/protocol metadata disagree — surfaced as an error rather than
/// an out-of-bounds panic or a silently mis-bound root.
///
/// A committed round can never carry public columns alongside its commitment:
/// prover-ray's `hideCommittedColumns` (wiop/compilers/pcs/pcs.go) panics at
/// compile time if a committed round holds a `VisibilityPublic` column, and
/// otherwise rewrites all of that round's columns to `VisibilityInternal`
/// before the transcript ever absorbs them. So a committed round's message is
/// provably root-only — `cols.len != 1` below is asserting that invariant, not
/// over-rejecting a legal mixed-visibility round.
fn resolveRoots(
    batch_roots: []const pcs.BatchRoot,
    rounds: []const protocol.RoundMessage,
    out: []poseidon2.Digest,
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
