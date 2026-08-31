const protocol = @import("protocol/root.zig");
const public_input_mod = protocol.public_input;
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const grandproduct = @import("query/grandproduct.zig");
const rowlimit = @import("query/rowlimit.zig");
const pcs = @import("query/pcs.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const ext = @import("field/koalabear_ext.zig");
const profiling = @import("profiling.zig");

/// Compiled verifier metadata for the protocol: the public-input transcript
/// layout plus one field per sub-verifier.
pub const Systems = struct {
    public_input: public_input_mod.Spec = .{},
    vanishing: vanishing.System,
    logderivativesum: logderivativesum.System = .{},
    /// Grand-product boundary checks: permutation arguments (Result == 1) and
    /// message-bus handles (Result == expected accumulator). The running-product
    /// Z-column recurrence and row-0 boundary are ordinary vanishing constraints,
    /// already covered by `vanishing`; this sub-verifier closes the remaining
    /// ∏Z[n-1] == Result and Result == expected identities.
    grandproduct: grandproduct.System = .{},
    /// Lookup row-limit checks. Independent of vanishing/logderivativesum: it
    /// bounds dynamic-module runtime sizes rather than checking a polynomial
    /// identity, so it needs only `proof.module_sizes`, never claims or coins.
    rowlimit: rowlimit.System = .{},
    /// FRI/PCS opening verifier. MANDATORY: there is no "PCS-disabled" protocol.
    /// `verify` runs PCS first — deriving the opening coins (zeta, fold
    /// challenges, query positions) from the shared Fiat-Shamir transcript and
    /// authenticating the committed claims — then feeds those *authenticated*
    /// claims into the vanishing check. So a prover can neither choose the
    /// challenges nor hand the two sub-verifiers different claim values.
    pcs: pcs.System,
};

/// Flat public-input statement in prover-ray registration order. Each entry is
/// the verifier-visible scalar value of one cell registered via
/// `System.RegisterPublicInputs` on the prover side.
pub const PublicInput = []const protocol.Scalar;

/// Proof is the verifier-visible transcript consumed by `verify` in one pass.
/// It is the verifier-ray analogue of prover-ray's `wiop.Proof`: a
/// self-contained bundle of exactly the data a verifier is entitled to see.
///
/// Protocol-level round messages (commitments + NON-public-input cells) are
/// shared across every sub-verifier. Registered public-input cells are omitted
/// from `rounds[*].cells` and carried separately in `VerifyInput.public_inputs`,
/// mirroring prover-ray's `Proof` + `PublicInput` split. Sub-verifier-specific
/// claim slices are routed only to the verifier that registered them. Coins are
/// not stored here — they are re-derived deterministically by
/// `protocol.replayWithTranscript` from the bound round messages.
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
///
/// It also does NOT carry `entry_claims`: those claimed evaluations are
/// ordinary `LagrangeEval.EvaluationClaims` cells, already transcript-bound in
/// `rounds[*].cells`. `verify` reconstructs the `pcs.verify`-shaped
/// `entry_claims` array locally, by reading each opened column's claim cells
/// out of `rounds[*].cells` via the per-column `claim_cells` table on
/// `pcs.System.columns` (see `pcs.ColumnDesc.claim_cells`).
pub const PcsOpening = struct {
    proof: pcs.OpeningProof,
};

/// A bundled verifier input: prover-ray-style proof plus the separate flat
/// public-input statement, for callers (native mmap / R5 linked-memory /
/// embedded-rodata loaders) that need the two loaded and passed around as one
/// value. `verify` itself takes `proof` and `public_inputs` as separate
/// parameters — pass `input.proof, input.public_inputs`.
/// `proof.rounds[*].cells` omits public-input cells; `public_inputs` supplies
/// them in registration order.
pub const VerifyInput = struct {
    proof: Proof,
    public_inputs: PublicInput = &.{},
};

/// The root of an aggregator pair image: two pointers to independently encoded
/// `VerifyInput`s in the same input region. This is what the aggregator guest
/// casts `_in_start` to (and the native aggregator mmaps), the two-proof
/// analogue of casting the region to a single `*const VerifyInput`.
///
/// The pointers are absolute addresses written by the encoder
/// (verifier-ray/codegen's `EncodeAggregatorPair`), so dereferencing needs no
/// arithmetic and no fix-up pass — the same zero-cost-decode contract as the
/// single-proof image, with the same cost: the image is only valid at the base
/// it was encoded for. Layout is pinned in `proof_abi.zig`.
pub const AggregatorInput = struct {
    a: *const VerifyInput,
    b: *const VerifyInput,
};

/// Verifies TWO proofs of the same compiled protocol and checks that their
/// public-input statements agree. This is the aggregation-shaped entry point:
/// a caller (e.g. a recursive R5 guest) that attests to a pair of proofs must
/// also attest that both proofs speak about the same statement — otherwise the
/// pair "both verify" claim is meaningless, as each proof could be about a
/// different instance.
///
/// The consistency rule is element-wise equality of the two flat statements
/// (see `checkPublicInputConsistency`). It runs BEFORE either verification:
/// it is O(statement length) while each `verify` is the expensive part, so an
/// inconsistent pair is rejected without paying for two full verifications.
///
/// Both proofs are checked against the SAME comptime `spec`/`systems` by
/// construction — there is deliberately no two-system variant: cross-system
/// public-input consistency has no meaning while statement layouts can differ.
pub fn verifyPair(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    a: VerifyInput,
    b: VerifyInput,
) !void {
    try checkPublicInputConsistency(a.public_inputs, b.public_inputs);
    try verify(spec, systems, a.proof, a.public_inputs);
    try verify(spec, systems, b.proof, b.public_inputs);
}

/// The public-input consistency rule for a proof pair: the two flat statements
/// must be equal element-wise, in prover-ray registration order. Values are
/// compared through `Scalar.toExt` so a base-encoded scalar equals its lifted
/// extension encoding — equality is over field values, not wire representation.
///
/// Length inequality is also `InconsistentPublicInputs`: each statement's
/// length-vs-spec check belongs to `verify` (via `bindRoundMessages`); here two
/// different lengths simply cannot be the same statement.
pub fn checkPublicInputConsistency(
    a: PublicInput,
    b: PublicInput,
) error{InconsistentPublicInputs}!void {
    if (a.len != b.len) return error.InconsistentPublicInputs;
    for (a, b) |scalar_a, scalar_b| {
        if (!scalar_a.toExt().eql(scalar_b.toExt())) return error.InconsistentPublicInputs;
    }
}

/// Verifies a proof against the compiled protocol in three steps:
///
///   1. Replay   — absorb every round message into the shared Fiat-Shamir
///                 transcript and squeeze all coins deterministically.
///   2. Route    — wrap coins and bound round messages in a `protocol.Context`
///                 that every sub-verifier can read without owning the transcript.
///   3. Dispatch — call each sub-verifier with the shared context and its own
///                 claim slice. Sub-verifiers are independent of each other.
///
/// `spec` carries the protocol-level coin routing. `systems` carries the
/// public-input transcript layout plus one compiled system per sub-verifier.
/// This is the only place in the codebase that knows the full verifier
/// metadata bundle.
pub fn verify(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    proof: Proof,
    public_inputs: PublicInput,
) !void {
    comptime if (systems.public_input.round_cell_counts.len != spec.round_coin_counts.len - 1)
        @compileError("verifier: public_input.round_cell_counts must have one entry per replayed round");

    profiling.reset();
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.verify_start, 0);

    var bound_rounds = try public_input_mod.bindRoundMessages(systems.public_input, proof.rounds, public_inputs);
    const rounds = bound_rounds.rounds();

    // Step 1 — replay transcript, derive all coins. The transcript is owned here
    // and threaded by pointer: `protocol` absorbs the round messages + squeezes
    // the protocol coins, leaving it at the state a transcript-continuing
    // sub-verifier (PCS, below) resumes from. `replayWithTranscript`
    // comptime-validates `spec` internal consistency and returns the
    // stack-allocated coin array.
    var transcript = fiat_shamir.Transcript.init();
    const all_coins = try protocol.replayWithTranscript(&transcript, spec, rounds, proof.module_sizes);
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.transcript_done, 0);

    // Step 2 — assemble the shared context routed to every sub-verifier.
    const ctx = protocol.Context{
        .all_coins = &all_coins,
        .rounds = rounds,
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
    try resolveRoots(pcs_system.batch_roots, rounds, &bound_roots);

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

    // Every claimed evaluation is an ordinary `LagrangeEval.EvaluationClaims`
    // cell, transcript-bound in `ctx.rounds`. The per-column `claim_cells`
    // table (parallel to `shifts`) names exactly which (round, index) cell
    // backs each (column, shift) claim. This array is stack-local to this call
    // and passed straight into `pcs.verify` below, so its lifetime is fine:
    // nothing here escapes past `verify` returning.
    var entry_claims_buf: pcs.EntryClaims(pcs_system) = .{};
    try pcs.buildEntryClaims(pcs_system, recon, ctx, &entry_claims_buf);
    const entry_claims = entry_claims_buf.slice();

    const pcs_challenges = try pcs.deriveChallenges(pcs_system, recon, &transcript, opening.proof.fri_proof);
    try pcs.verify(pcs_system, .{
        .roots = &bound_roots,
        .entry_claims = entry_claims,
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
    try routeClaims(pcs_system, recon, pcs_system.witness_map, entry_claims, &derived_witness);
    try routeClaims(pcs_system, recon, pcs_system.quotient_map, entry_claims, &derived_quotient);

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

    try grandproduct.verify(systems.grandproduct, ctx);

    try rowlimit.verify(systems.rowlimit, proof.module_sizes);

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
/// interactive batch's root is the commitment of the round message it names
/// (the same octuplet absorbed to derive zeta); a precomputed batch's root is
/// the compile-time constant. This is the verifier-ray analogue of prover-ray's
/// `collectRoots` reading `rt.Commitments`, so the root a batch is authenticated
/// against is provably the one zeta is bound to.
///
/// `batch_roots.len` must equal `out.len` (== num_batches). A `.round` entry must
/// name a round message that exists and carries a commitment; otherwise the
/// PCS/protocol metadata disagree — surfaced as an error rather than an
/// out-of-bounds panic or a silently mis-bound root.
///
/// prover-ray columns never travel raw (see wiop_prove_verify.go's Proof
/// comment): a round is either committed, in which case its message carries
/// exactly that one root, or it isn't, in which case `commitment` is null. So
/// `RoundMessage.commitment == null` below directly reflects "this round has no
/// batch root," not an ambiguous case to disambiguate at runtime.
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
                slot.* = rounds[round_index].commitment orelse return error.BatchRootMismatch;
            },
        }
    }
}
