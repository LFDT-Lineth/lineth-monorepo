const std = @import("std");
const protocol = @import("protocol/root.zig");
const public_input_mod = protocol.public_input;
const vanishing = @import("query/vanishing.zig");
const logderivativesum = @import("query/logderivativesum.zig");
const grandproduct = @import("query/grandproduct.zig");
const rowlimit = @import("query/rowlimit.zig");
const shared_randomness = @import("query/shared_randomness.zig");
const pcs = @import("query/pcs.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const ext = @import("field/koalabear_ext.zig");
const profiling = @import("profiling.zig");

// ============================================================================
// Aggregate statement
//
// Let S = (spec, systems) be the compiled inner verifier the aggregator guest
// is built against (riscv_system.zig, a comptime constant of the ELF), and
// n = systems.public_input.refs.len. A STATEMENT is
//
//     X ∈ F_{p^6}^n
//
// the flat public-input vector in prover-ray registration order, every scalar
// lifted to the extension field (`Scalar.toExt`). The inner relation is
//
//     R_S(X, π)  ⇔  verify(S, π, X) accepts
//
// X is absorbed into π's Fiat-Shamir transcript by `bindRoundMessages`, so a
// proof is a proof of exactly one X. The relation `verifyPair` decides is
//
//     R_pair(X; πA, πB)  ⇔  R_S(X, πA) ∧ R_S(X, πB)
//
// X is the whole claim; πA and πB are witnesses. Everything that travels in a
// proof rather than in the statement — `module_sizes`, round messages,
// openings — is private to its own verification and is NOT compared across the
// pair: two accepted proofs of X with different module sizes are a valid pair.
// πA == πB is accepted too; the claim is "X is proven", not "proven twice".
//
// The guest exposes exactly this claim through its output:
//
//     out = pair_statement_domain || BE64(n) || enc(X[0]) || … || enc(X[n-1])
//
// with enc = `Ext.toBytes`: the field's canonical encoding, 6 BIG-endian u32
// limbs (B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1), each < p. Every integer in
// `out` is big-endian; this is deliberately the opposite of the little-endian
// proof image, because `out` reuses the canonical field bytes that golden
// tests and Poseidon2 already pin. The domain tag versions the encoding,
// BE64(n) makes it prefix-free, and the lift to the extension field makes
// `out` injective on the same equality relation as
// `checkPublicInputConsistency` — one claim, one encoding.
// test/verify_pair_test.zig pins the exact bytes independently of `toBytes`.
//
// The rest of the claim is bound by the OUTER proof, not by these bytes:
//   - S: the aggregator ELF embeds it, so the outer program commitment
//     identifies S. A different inner system is a different outer program.
//   - πA, πB: read from the guest input region, never exposed.
//   - how the zkVM commits to `out` (raw `guest_output` bytes, or their keccak
//     in `guest_output_hash`) is the outer system's choice; it does not change
//     the claim, only its representation as outer public inputs.
//
// So an accepted outer proof reads: "there exist two proofs, each accepted by
// verifier S, of the one statement X whose canonical encoding is `out`."
//
// For the production RISC-V system n = 17: the inner guest's keccak output
// digest (8 u32 words, 16 limbs) followed by the message-bus accumulator cell.
// The pair claim is therefore "two accepted inner executions share this output
// digest and bus state". A chained aggregation (X_A ≠ X_B linked by a
// predicate, out = f(X_A, X_B)) is a new relation and must take a new domain
// tag.
// ============================================================================

pub const pair_statement_domain = "lineth-verifier-pair-v1";

pub fn pairStatementEncodedLen(comptime count: usize) usize {
    return pair_statement_domain.len + @sizeOf(u64) + count * ext.bytes;
}

/// Canonical encoding of the aggregate statement (see the header above).
/// Scalars are normalized to extension-field values so this encoding uses the
/// same equality relation as `checkPublicInputConsistency`.
pub fn encodePairStatement(statement: PublicInput, out: []u8) !void {
    if (out.len != pair_statement_domain.len + @sizeOf(u64) + statement.len * ext.bytes)
        return error.InvalidPairStatementBuffer;
    @memcpy(out[0..pair_statement_domain.len], pair_statement_domain);
    std.mem.writeInt(u64, out[pair_statement_domain.len..][0..@sizeOf(u64)], statement.len, .big);
    for (statement, 0..) |scalar, i| {
        const encoded = scalar.toExt().toBytes();
        @memcpy(out[pair_statement_domain.len + @sizeOf(u64) + i * ext.bytes ..][0..ext.bytes], &encoded);
    }
}

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
    /// Shared-randomness contribution check: this shard's public-input
    /// contribution digest must equal the Poseidon2 sponge hash over every
    /// committed round preceding the message-bus coin round. Verifier-side
    /// counterpart to prover-ray's messagebus.SharedRandomnessContributionChecker.
    /// Empty (`.{}`) for a protocol compiled with the messagebus
    /// WithoutSharedRandomness opt-out, in which case the check is a no-op.
    shared_randomness: shared_randomness.System = .{},
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
/// `VerifyInput`s in the same input region, the two-proof analogue of the
/// single `*const VerifyInput` at the start of a proof image.
///
/// The pointers are absolute addresses written by the encoder
/// (verifier-ray/codegen's `EncodeAggregatorPair`) for one base, so the image
/// is only valid at that base. The header is never cast straight out of
/// untrusted memory: both pair loaders in `main.zig` run
/// `image_relocation.validatePair` on the image at the address it will be read
/// from, which reads the two words as integers, checks them
/// (`parsePairHeader`) and validates each member, and only then form these
/// typed pointers. Layout is pinned in `proof_abi.zig`.
pub const AggregatorInput = struct {
    a: *const VerifyInput,
    b: *const VerifyInput,
};

/// Decides R_pair (see "Aggregate statement" above): verifies TWO proofs of
/// the same compiled protocol and checks that their public-input statements
/// agree. This is the aggregation-shaped entry point: a caller (e.g. a
/// recursive R5 guest) that attests to a pair of proofs must also attest that
/// both proofs speak about the same statement — otherwise the pair "both
/// verify" claim is meaningless, as each proof could be about a different
/// instance.
///
/// The consistency rule is element-wise equality of the two flat statements
/// (see `checkPublicInputConsistency`) and nothing else: proof-side data such
/// as `module_sizes` is validated per proof by `verify` and is not part of the
/// claim. The rule runs BEFORE either verification: it is O(statement length)
/// while each `verify` is the expensive part, so an inconsistent pair is
/// rejected without paying for two full verifications.
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
    var workspace: Workspace(systems.public_input) = undefined;
    return verifyPairWithWorkspace(spec, systems, a, b, &workspace);
}

/// `verifyPair` with caller-owned scratch (see `Workspace`). The two
/// verifications run one after the other, so they share one workspace.
pub fn verifyPairWithWorkspace(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    a: VerifyInput,
    b: VerifyInput,
    workspace: *Workspace(systems.public_input),
) !void {
    // Validate both untrusted slice lengths before comparing any elements. The
    // consistency walk must not let a pair image choose its runtime cost (or
    // trigger an out-of-bounds read) before verify's normal count check runs.
    if (a.public_inputs.len != systems.public_input.refs.len or
        b.public_inputs.len != systems.public_input.refs.len)
    {
        return error.InvalidPublicInputCount;
    }
    try checkPublicInputConsistency(a.public_inputs, b.public_inputs);
    try verifyWithWorkspace(spec, systems, a.proof, a.public_inputs, workspace);
    try verifyWithWorkspace(spec, systems, b.proof, b.public_inputs, workspace);
}

/// The public-input consistency rule for a proof pair: the two flat statements
/// must be equal element-wise, in prover-ray registration order. Values are
/// compared through `Scalar.toExt` so a base-encoded scalar equals its lifted
/// extension encoding — equality is over field values, not wire representation.
///
/// Length inequality is also `InconsistentPublicInputs`: two different lengths
/// cannot be the same statement. Checking each length against the compiled
/// spec is the caller's job — `verifyPairWithWorkspace` does it before calling
/// this, and `verify` does it again via `bindRoundMessages`.
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
/// Scratch for the bound-round-message workspace, which must live across
/// transcript replay and PCS verification. It holds every round cell, so the R5
/// entry point owns one in `.bss` rather than letting it sit in `verify`'s
/// stack frame — the guest's linker stack is a fixed 8 MiB.
pub fn Workspace(comptime spec: public_input_mod.Spec) type {
    return struct {
        bound_rounds: public_input_mod.BoundRoundMessages(spec) = undefined,
    };
}

pub fn verify(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    proof: Proof,
    public_inputs: PublicInput,
) !void {
    var workspace: Workspace(systems.public_input) = undefined;
    return verifyWithWorkspace(spec, systems, proof, public_inputs, &workspace);
}

pub fn verifyWithWorkspace(
    comptime spec: protocol.Spec,
    comptime systems: Systems,
    proof: Proof,
    public_inputs: PublicInput,
    workspace: *Workspace(systems.public_input),
) !void {
    comptime if (systems.public_input.round_cell_counts.len != spec.round_coin_counts.len - 1)
        @compileError("verifier: public_input.round_cell_counts must have one entry per replayed round");

    profiling.reset();
    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.verify_start, 0);

    try public_input_mod.bindRoundMessagesInto(systems.public_input, &workspace.bound_rounds, proof.rounds, public_inputs);
    const rounds = workspace.bound_rounds.rounds();

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

    // The column manifest (which columns the prover elided from each committed
    // batch as zero / duplicate) is read from the same transcript-bound round
    // cells zeta was derived from; `reconstructWithManifest` validates it.
    var manifest: pcs.Manifest(pcs_system) = .{};
    try pcs.readManifest(pcs_system, ctx, &manifest);

    // Reconstruct the canonical PCS layout for THIS proof's dynamic sizes and
    // manifest: one baked comptime System verifies proofs of different module
    // sizes because the bundle placement / entry order / restricted params are
    // a runtime function of `module_sizes` (mirroring prover-ray's GetLayout +
    // canonicalLayout + restrictTo). deriveChallenges needs the restricted
    // params for the fold / query-position counts.
    const recon = try pcs.reconstructWithManifest(pcs_system, proof.module_sizes, manifest.slice());

    // Every claimed evaluation is an ordinary `LagrangeEval.EvaluationClaims`
    // cell, transcript-bound in `ctx.rounds`. The per-column `claim_cells`
    // table (parallel to `shifts`) names exactly which (round, index) cell
    // backs each (column, shift) claim. This array is stack-local to this call
    // and passed straight into `pcs.verify` below, so its lifetime is fine:
    // nothing here escapes past `verify` returning.
    var entry_claims_buf: pcs.EntryClaims(pcs_system) = .{};
    try pcs.buildEntryClaims(pcs_system, &recon, ctx, &entry_claims_buf);
    const entry_claims = entry_claims_buf.slice();

    const pcs_challenges = try pcs.deriveChallenges(pcs_system, &recon, &transcript, opening.proof.fri_proof);
    try pcs.verify(pcs_system, .{
        .roots = &bound_roots,
        .entry_claims = entry_claims,
        .zeta = all_coins[zeta_index],
        .fold_alphas = pcs_challenges.foldAlphas(),
        .deep_alpha = pcs_challenges.deep_alpha,
        .query_positions = pcs_challenges.query_positions[0..recon.params.num_queries],
        .proof = opening.proof,
        .module_sizes = proof.module_sizes,
        .manifest = manifest.slice(),
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

    try logderivativesum.verify(systems.logderivativesum, ctx, proof.module_sizes);

    if (comptime profiling.r5_marks) profiling.markR5Value(profiling.Mark.logderivativesum_done, profiling.snapshot().poseidon2_compress);

    try grandproduct.verify(systems.grandproduct, ctx);

    try rowlimit.verify(systems.rowlimit, proof.module_sizes);

    try shared_randomness.verify(systems.shared_randomness, ctx);

    // TODO(profiling): add a final verify_done marker once more phases run after logderivativesum.
}

/// Fills `out[k]` with the authenticated claim each `map` entry points at:
/// `out[k] = pcs.routedClaim(map[k].col_decl_idx, map[k].shift)` — the entry's
/// claim for a Present column, zero for a Zero column, the target's claim for
/// an alias (see pcs.zig "Column manifest"). `map.len` must equal `out.len`
/// (the vanishing System's claim total) and every ClaimRef must be in range,
/// else the PCS/vanishing metadata disagree — a codegen bug, surfaced as an
/// error rather than an out-of-bounds panic.
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
        // for THIS proof's sizes and manifest.
        slot.* = try pcs.routedClaim(system, &recon, entry_claims, ref.col_decl_idx, ref.shift);
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
