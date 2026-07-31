//! Top-level FRI/PCS opening-proof verification.
//!
//! Ports prover-ray `crypto/koalabear/fri` `pcs.go` `Verify` (and its helpers
//! `authenticateInputQuery`, `bindInputTreeOpenings`, `checkClaimPointsOutOfDomain`,
//! `checkOpeningProofShape`, and the D=1 special case) into an allocation-free,
//! comptime-specialized verifier.
//!
//! The batch shapes and shift schedule are fixed at protocol-compile time, so
//! the canonical `layout` and the FRI `params` live in a comptime `System`.
//! From that `System` the verifier derives comptime capacity bounds
//! (`maxRounds`, `maxLevels`, `maxEntries`, `maxShifts`) and sizes every
//! per-query scratch buffer as a fixed stack array — nothing allocates.
//!
//! Deliberate deviations from the Go verifier (both documented in
//! docs/fri-pcs-plan.md):
//!   1. The final polynomial is Horner-evaluated at each query's folded domain
//!      point instead of IFFT-ing it to a codeword. Identical result, no FFT.
//!   2. Input trees are authenticated once PER BATCH (one InputTreeOpening per
//!      batch per query) rather than deduplicated across batches that happen to
//!      share a Merkle root. Still sound; only skips redundant-work avoidance.

const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const canonical = @import("../polynomial/canonical.zig");
const fiat_shamir = @import("../crypto/fiat_shamir.zig");
const params_mod = @import("params.zig");
const layout_mod = @import("layout.zig");
const tree = @import("tree.zig");
const paired_leaf = @import("paired_leaf.zig");
const fold_mod = @import("fold.zig");
const reconstruct = @import("reconstruct.zig");

pub const Octuplet = tree.Octuplet;

/// Locates one vanishing claim inside the flat `entry_claims`: opened column `g`
/// (canonical layout order) at its `shift`-th opened rotation. The vanishing
/// sub-verifier's witness/quotient claim `k` is exactly `entry_claims[entry][shift]`
/// — the same runtime cell prover-ray's PCS compiler opens (see the package doc).
/// Emitted by codegen from the LagrangeEval ↔ committed-column binding.
pub const ClaimRef = struct {
    entry: usize,
    shift: usize,
};

pub const Error = error{
    RootCountMismatch,
    InputOpeningCountMismatch,
    RoundRootCountMismatch,
    FinalPolyLengthMismatch,
    QueryPositionOutOfRange,
    RunningQueryCountMismatch,
    RunningLayerCountMismatch,
    RunningLayerShapeMismatch,
    ClaimShapeMismatch,
    ClaimPointInDomain,
    RowShapeMismatch,
    InputTreeAuthFailed,
    RunningLayerAuthFailed,
    LevelRoundExceedsSchedule,
    RoundZeroSelfMismatch,
    RoundZeroSiblingMismatch,
};

/// Where a committed batch's Merkle root is bound. A batch root MUST be tied to
/// the Fiat-Shamir transcript that derives zeta — otherwise a prover could open
/// against a forged root while zeta stays bound to the honest commitment, and
/// the opening would be authenticated at a point it is not committed to.
///
/// There are two provenances, mirroring prover-ray's single-source `collectRoots`
/// (which reads `rt.Commitments` — the exact octuplets `AdvanceRound` absorbs
/// into the transcript):
///   - `.round`: an interactive batch. Its root IS the oracle commitment absorbed
///     in round message `round_index` (the same one squeezed into zeta), so the
///     verifier reads it from `rounds[round_index]` rather than from the proof.
///   - `.precomputed`: the static precomputed batch. Its root is a compile-time
///     constant (like a verification key), fixed by codegen and trusted by
///     construction — it is never carried in a round message.
pub const BatchRoot = union(enum) {
    /// Index into `proof.rounds`; the batch root is that round's sole oracle
    /// commitment (`rounds[round_index].columns[0].oracle_commitment`).
    round: usize,
    /// Compile-time precomputed-batch root, emitted by codegen.
    precomputed: Octuplet,
};

/// Compile-time PCS description shared by prover and verifier codegen. `layout`
/// is the frozen canonical enumeration (see layout.zig); `params` the FRI
/// configuration. `num_batches` is the number of committed batches (one root
/// each). The `max*` bounds are derived from `layout`/`params` and size the
/// per-query stack scratch.
pub const System = struct {
    params: params_mod.Params,
    layout: layout_mod.Layout,
    /// Per-batch declared shapes (shapes[b][size_log2] = SizedShape). Same data
    /// `buildLayout` consumes; kept so row-shape checks use the full declared
    /// widths rather than inferring them from opened row indices.
    shapes: []const layout_mod.Shape,
    num_batches: usize,

    /// Routes each vanishing witness/quotient claim to its authenticated value in
    /// the flat `entry_claims`. `witness_map[k]` (resp. `quotient_map[k]`) is the
    /// `ClaimRef` for the vanishing sub-verifier's k-th witness (resp. quotient)
    /// claim. `verifier.verify` uses these to feed the *PCS-authenticated* claims
    /// into the vanishing check, so a prover cannot hand the two sub-verifiers
    /// different values for the same column. Length must equal the vanishing
    /// System's `total_witness_claims` / `total_quotient_claims`.
    witness_map: []const ClaimRef = &.{},
    quotient_map: []const ClaimRef = &.{},

    /// Per-batch root provenance, in canonical batch order (index == batch
    /// index). `verifier.verify` builds the authenticated `Inputs.roots` from
    /// this — reading interactive-batch roots out of the transcript-bound round
    /// oracle commitments and precomputed roots from the compile-time constant —
    /// so the root a batch is Merkle-authenticated against is provably the one
    /// zeta is bound to. Length MUST equal `num_batches`. Empty only for legacy
    /// callers that still pass `Inputs.roots` directly (deprecated; see
    /// `verifier.verify`).
    batch_roots: []const BatchRoot = &.{},

    /// Flat `all_coins` index of zeta — the shared LagrangeEval opening point,
    /// which is also the vanishing modules' eval coin. `verifier.verify` reads
    /// `all_coins[zeta_coin_index]` and passes it as `Inputs.zeta`, so the
    /// opening point is Fiat-Shamir-derived, never taken from the proof.
    zeta_coin_index: usize = 0,

    pub fn maxRounds(comptime self: System) usize {
        return self.params.numRounds();
    }
    pub fn maxLevels(comptime self: System) usize {
        return self.layout.len;
    }
    pub fn maxEntries(comptime self: System) usize {
        var m: usize = 0;
        for (self.layout) |bundle| {
            if (bundle.entries.len > m) m = bundle.entries.len;
        }
        return m;
    }
    pub fn maxShifts(comptime self: System) usize {
        var m: usize = 0;
        for (self.layout) |bundle| {
            for (bundle.entries) |entry| {
                if (entry.shifts.len > m) m = entry.shifts.len;
            }
        }
        return m;
    }
    /// Total opened columns across all bundles — the length of the flat
    /// `entry_claims` runtime slice the verifier consumes.
    pub fn totalEntries(comptime self: System) usize {
        var n: usize = 0;
        for (self.layout) |bundle| n += bundle.entries.len;
        return n;
    }
};

/// The running-FRI portion of an opening proof: the per-round layer roots
/// (T_1..T_{r-1}), the final polynomial coefficients, and, per query, the
/// running-layer Merkle branches (one per fold round 1..r-1).
pub const FriProof = struct {
    round_roots: []const Octuplet,
    final_poly: []const ext.Ext,
    /// running_queries[query][round-1] is the branch opening running layer
    /// `round` (rounds 1..numRounds-1) for `query`.
    running_queries: []const []const tree.Branch,
};

/// A complete PCS opening proof.
pub const Proof = struct {
    /// input_queries[query][batch] is the paired-leaf opening of `batch`'s tree
    /// for `query`.
    input_queries: []const []const paired_leaf.InputTreeOpening,
    fri: FriProof,
};

/// Runtime verification inputs (everything derived from the caller's transcript
/// replay plus the batch commitments).
///
/// `zeta` is the (single) opening point. It is NOT trusted from the proof: the
/// caller sets it to `all_coins[system.zeta_coin_index]`, the Fiat-Shamir coin
/// the vanishing sub-verifier also evaluates at. The fold challenges and query
/// positions are likewise not inputs — `verify` derives them from the live
/// transcript (see `deriveChallenges`), so a prover cannot choose them.
pub const Inputs = struct {
    /// One Merkle root per batch (index == batch index) to authenticate against.
    ///
    /// SECURITY: these roots MUST be bound to the Fiat-Shamir transcript that
    /// derived `zeta` — otherwise a prover can open against a forged root while
    /// zeta stays bound to the honest commitment. The full verifier
    /// (`verifier.verify`) enforces this by REBUILDING `roots` from
    /// `System.batch_roots` (the transcript-bound round oracle commitments) and
    /// ignoring whatever a proof supplies. Callers that invoke `pcs.verify`
    /// directly (e.g. the standalone FRI-engine tests) are responsible for
    /// passing transcript-bound roots themselves.
    roots: []const Octuplet,
    /// Claimed evaluations, one inner slice per opened column in canonical
    /// (flat) layout order; inner slice length == that column's shift count.
    /// entry_claims[g][j] is the claim at the g-th column's j-th shift.
    entry_claims: []const []const ext.Ext,
    zeta: ext.Ext,
};

/// The fold challenges and query positions the FRI verifier consumes, sized by
/// the System's comptime bounds so nothing allocates. Produced by
/// `replayWithTranscript` (the transcript-touching derivation) and consumed by
/// `verify` (pure arithmetic) — separating derivation from checking. `num_rounds`
/// records how many of `fold_alphas` are live for this proof's restricted schedule.
pub fn Challenges(comptime system: System) type {
    return struct {
        fold_alphas: [system.maxRounds()]ext.Ext = undefined,
        positions: [system.params.num_queries]usize = undefined,
        num_rounds: usize = 0,
    };
}

/// Derive the FRI fold challenges and query positions by continuing `transcript`
/// — the live Fiat-Shamir state left by `protocol.replayWithTranscript`
/// (mirroring prover-ray's `fs := rt.GetFS()`), so the proof cannot dictate them.
/// `transcript` is caller-owned and advanced in place. Only the round roots and
/// final polynomial are absorbed; the challenges are pure squeezes returned as a
/// value. This is the transcript-touching half of PCS verification; `verify`
/// then checks the proof against these challenges without touching the transcript.
pub fn replayWithTranscript(
    comptime system: System,
    transcript: *fiat_shamir.Transcript,
    proof: Proof,
) Challenges(system) {
    // The restricted schedule is a comptime property of the compiled System, so
    // resolve it at comptime; an invalid layout is a codegen bug caught here.
    const p = comptime system.params.restrictTo(layout_mod.maxSizeLog2(system.layout)) catch
        @compileError("PCS System layout exceeds its params' plaintext size");
    var ch = Challenges(system){};
    deriveChallenges(p, proof, transcript, &ch);
    return ch;
}

/// Verify an opening proof against `system` using pre-derived `challenges`
/// (from `replayWithTranscript`). `zeta` (in `inputs`) is the Fiat-Shamir opening
/// coin re-derived by the caller. This is the pure-checking half: it performs no
/// Fiat-Shamir squeezes, only authentication + reconstruction + fold arithmetic.
/// Returns on the first failure.
pub fn verify(
    comptime system: System,
    inputs: Inputs,
    proof: Proof,
    challenges: Challenges(system),
) (Error || params_mod.Error || fold_mod.Error || reconstruct.Error || paired_leaf.Error || tree.Error)!void {
    // Restrict the FRI schedule to the largest opened size so the fold count
    // tracks the witness, mirroring the prover. This is a comptime property of
    // the System (same restriction `replayWithTranscript` derives from).
    const p = comptime system.params.restrictTo(layout_mod.maxSizeLog2(system.layout)) catch
        @compileError("PCS System layout exceeds its params' plaintext size");
    const num_rounds = comptime p.numRounds();

    // ── Shape checks ─────────────────────────────────────────────────────────
    if (inputs.roots.len != system.num_batches) return Error.RootCountMismatch;
    if (inputs.entry_claims.len != comptime system.totalEntries()) return Error.ClaimShapeMismatch;
    checkEntryClaimShapes(system, inputs.entry_claims) catch return Error.ClaimShapeMismatch;
    try checkClaimPointsOutOfDomain(system, inputs.zeta);

    if (proof.input_queries.len != p.num_queries) return Error.InputOpeningCountMismatch;

    const want_round_roots: usize = if (num_rounds > 0) num_rounds - 1 else 0;
    if (proof.fri.round_roots.len != want_round_roots) return Error.RoundRootCountMismatch;
    if (proof.fri.final_poly.len != @as(usize, 1) << @intCast(p.log_final_poly_size)) {
        return Error.FinalPolyLengthMismatch;
    }
    if (proof.fri.running_queries.len != p.num_queries) return Error.RunningQueryCountMismatch;

    const codeword_size = @as(usize, 1) << @intCast(p.log_codeword_size);

    // ── Fold challenges + query positions (pre-derived from the transcript) ────
    const maxRounds = comptime system.maxRounds();
    const fold_alphas = challenges.fold_alphas[0..num_rounds];
    const query_positions = challenges.positions[0..p.num_queries];

    // ── Per-query verification ────────────────────────────────────────────────
    for (0..p.num_queries) |query_idx| {
        const s = query_positions[query_idx];
        if (s >= codeword_size) return Error.QueryPositionOutOfRange;

        var rq = fold_mod.ResolvedQuery(maxRounds){};

        // Final target: Horner-evaluate FinalPoly at the folded domain point
        // x = domainPoint(finalDomain, s >> numRounds). (Replaces the Go IFFT.)
        const final_log = p.log_codeword_size - num_rounds;
        const final_x = try params_mod.domainPointExt(@intCast(final_log), s >> @intCast(num_rounds));
        rq.final = canonical.evaluateExtAtExt(proof.fri.final_poly, final_x);

        const input_opening = proof.input_queries[query_idx];
        if (input_opening.len != system.num_batches) return Error.InputOpeningCountMismatch;

        // Authenticate each batch's input tree against its declared root.
        try authenticateInputQuery(system, p, input_opening, inputs.roots, s);

        // Running layers 1..numRounds-1: authenticate against the round root and
        // decode the (self, sibling) pair from the branch leaf + last sibling.
        try resolveRunningLayers(p, proof, query_idx, s, &rq);

        // Every level (including the round-0 top polynomial): authenticate row
        // shapes, then reconstruct its conjugate pair via the DEEP quotient.
        try resolveLevels(system, p, inputs, fold_alphas, input_opening, s, &rq);

        // D=1 (numRounds==0): checkFolds runs zero iterations, so tie the
        // round-0 pair to the final polynomial explicitly.
        if (num_rounds == 0) {
            const sib_final = canonical.evaluateExtAtExt(
                proof.fri.final_poly,
                try params_mod.domainPointExt(@intCast(p.log_codeword_size), s ^ 1),
            );
            if (!rq.aux[0].self.eql(rq.final)) return Error.RoundZeroSelfMismatch;
            if (!rq.aux[0].sibling.eql(sib_final)) return Error.RoundZeroSiblingMismatch;
        }

        // Fold recurrence for this single query.
        const one = [_]fold_mod.ResolvedQuery(maxRounds){rq};
        const pos = [_]usize{s};
        try fold_mod.checkFolds(maxRounds, p, &one, fold_alphas, &pos);
    }
}

/// Derive the FRI fold challenges and query positions from the live transcript.
/// Byte-faithful port of prover-ray `wiop/compilers/pcs/pcs.go` `verify`:
///
///   for root in round_roots: alpha = squeeze(); absorb(root)   // 8 base elems
///   alpha = squeeze()                                          // last, no root
///   absorb_ext(final_poly)
///   positions = randomManyIntegers(num_queries, codeword_size)
///
/// `alpha_DEEP` for a level is `fold_alphas[round]^2`, computed in resolveLevels;
/// only the raw fold challenges are squeezed here. Positions are drawn in
/// `[0, codeword_size)` where `codeword_size = 2^p.log_codeword_size` is the
/// restricted schedule's `effectiveN`.
fn deriveChallenges(
    p: params_mod.Params,
    proof: Proof,
    transcript: *fiat_shamir.Transcript,
    ch: anytype,
) void {
    const num_rounds = p.numRounds();
    ch.num_rounds = num_rounds;
    // The fold-alpha buffer is comptime-sized by the System's maxRounds. When a
    // protocol never folds (D=1, maxRounds==0) the buffer is a [0]ext.Ext, and
    // even a runtime-dead `ch.fold_alphas[i]` fails to compile ("cannot index
    // into empty array"). `cap` is comptime, so the round loop below is elided
    // from the D=1 instantiation rather than analyzed.
    const cap = @typeInfo(@TypeOf(ch.fold_alphas)).array.len;
    if (cap > 0) {
        // One challenge per intermediate layer root, absorbing the root between
        // squeezes.
        for (proof.fri.round_roots, 0..) |root, i| {
            ch.fold_alphas[i] = transcript.randomExt();
            transcript.updateElements(&root);
        }
    }

    // The final round's challenge is squeezed with no root after it. prover-ray
    // (`wiop/compilers/pcs/pcs.go` `verify`, line 331) squeezes this
    // UNCONDITIONALLY — including the D=1 (num_rounds == 0) case, where the round
    // loop above is empty. That squeeze mutates the transcript (via the safeguard
    // update after every squeeze) BEFORE `final_poly` is absorbed and the query
    // positions are drawn, so skipping it in D=1 would desynchronise the
    // positions from the reference. Squeeze it always; keep it only when a fold
    // slot exists (for D=1 there is no fold, so the value is discarded — but the
    // transcript side effect is what matters).
    const final_alpha = transcript.randomExt();
    if (cap > 0 and num_rounds > 0) ch.fold_alphas[num_rounds - 1] = final_alpha;

    transcript.updateExt(proof.fri.final_poly);

    const codeword_size = @as(usize, 1) << @intCast(p.log_codeword_size);
    transcript.randomManyIntegers(ch.positions[0..p.num_queries], codeword_size);
}

// ── helpers ──────────────────────────────────────────────────────────────────

fn checkEntryClaimShapes(comptime system: System, entry_claims: []const []const ext.Ext) !void {
    var g: usize = 0;
    inline for (system.layout) |bundle| {
        inline for (bundle.entries) |entry| {
            if (entry_claims[g].len != entry.shifts.len) return error.ClaimShapeMismatch;
            g += 1;
        }
    }
}

/// Every claim point of a size-N column is zeta·omega_N^shift; since
/// omega_N^N == 1, such a point lies in the size-N codeword domain iff zeta
/// itself does — independent of the shift. So test zeta once per distinct size.
/// Ports `checkClaimPointsOutOfDomain`.
fn checkClaimPointsOutOfDomain(comptime system: System, zeta: ext.Ext) Error!void {
    inline for (system.layout) |bundle| {
        // The size-N *codeword* domain has cardinality 2^(log_codeword_size -
        // roundForSize) = 2^(log_codeword_size - (log_plaintext_size -
        // size_log2)). Equivalent Go check: pointInDomain(zeta, encoder.Domain.Card).
        const round = system.params.log_plaintext_size - bundle.size_log2;
        const card = @as(u64, 1) << @intCast(system.params.log_codeword_size - round);
        if (pointInDomain(zeta, card)) return Error.ClaimPointInDomain;
    }
}

fn pointInDomain(point: ext.Ext, card: u64) bool {
    if (!point.isBase()) return false;
    const base = point.B0.a0;
    return base.pow(card).eql(field.Element.one());
}

fn authenticateInputQuery(
    comptime system: System,
    p: params_mod.Params,
    opening: []const paired_leaf.InputTreeOpening,
    roots: []const Octuplet,
    query_position: usize,
) (Error || paired_leaf.Error)!void {
    _ = system;
    const codeword_size = @as(usize, 1) << @intCast(p.log_codeword_size);
    for (opening, roots) |branch, root| {
        if (branch.leaves.len == 0 or branch.leaves[branch.leaves.len - 1] == null) {
            return Error.InputTreeAuthFailed;
        }
        const num_leaves = @as(usize, 1) << @intCast(branch.leaves.len);
        if (num_leaves > codeword_size or codeword_size % num_leaves != 0) return Error.InputTreeAuthFailed;
        const recovered = branch.recoverRoot(query_position / (codeword_size / num_leaves)) catch return Error.InputTreeAuthFailed;
        if (!octEql(recovered, root)) return Error.InputTreeAuthFailed;
    }
}

fn resolveRunningLayers(
    p: params_mod.Params,
    proof: Proof,
    query_idx: usize,
    s: usize,
    rq: anytype,
) (Error || fold_mod.Error || tree.Error)!void {
    const num_rounds = p.numRounds();
    if (num_rounds <= 1) return;
    const layers = proof.fri.running_queries[query_idx];
    if (layers.len != num_rounds - 1) return Error.RunningLayerCountMismatch;

    var j: u8 = 1;
    while (j < num_rounds) : (j += 1) {
        const branch = layers[j - 1];
        const root = proof.fri.round_roots[j - 1];

        // Exact branch-shape check, porting Go's `checkQueryLayerShape(...,
        // exactSiblings=true)` (fri.go checkBranchShape). For running layer j the
        // tree has `1 << (log_codeword_size - j)` leaves, so an authentic branch
        // carries exactly `log_codeword_size - j` siblings (and the same number of
        // aux siblings). `recoverRoot` alone only rejects branches *shorter* than
        // the query position needs; this pins the exact height so an over-long
        // forged branch is rejected before the (defence-in-depth) root check.
        const want_siblings: usize = @as(usize, p.log_codeword_size) - j;
        if (branch.siblings.len != want_siblings or branch.aux_siblings.len != want_siblings) {
            return Error.RunningLayerShapeMismatch;
        }

        const recovered = branch.recoverRoot(s >> @intCast(j)) catch return Error.RunningLayerAuthFailed;
        if (!octEql(recovered, root)) return Error.RunningLayerAuthFailed;
        if (branch.siblings.len == 0) return Error.RunningLayerAuthFailed;
        const self = try fold_mod.octupletToExt(branch.leaf);
        const sib = try fold_mod.octupletToExt(branch.siblings[branch.siblings.len - 1]);
        rq.rounds[j] = .{ .self = self, .sibling = sib };
    }
}

fn resolveLevels(
    comptime system: System,
    p: params_mod.Params,
    inputs: Inputs,
    fold_alphas: []const ext.Ext,
    input_opening: []const paired_leaf.InputTreeOpening,
    s: usize,
    rq: anytype,
) (Error || params_mod.Error || reconstruct.Error || paired_leaf.Error)!void {
    const num_rounds = p.numRounds();
    const maxShifts = comptime system.maxShifts();
    const maxEntries = comptime system.maxEntries();

    // Global (flat) column index into inputs.entry_claims, advanced in canonical
    // order as we walk bundles/entries.
    var g: usize = 0;
    inline for (system.layout) |bundle| {
        // `round` MUST come from the RESTRICTED schedule `p`, not the static
        // `system.params`: Go computes it via `roundForSize` on the restricted
        // receiver (`pcs.go` roundForSize + the `pcs = pcs.restrictTo(...)`
        // reassignment in Verify), i.e. round = maxSizeLog2(layout) - size_log2.
        // Using the unrestricted `system.params.log_plaintext_size` here would
        // make `round` too large by `log_plaintext_size - maxSizeLog2(layout)`
        // whenever that offset is nonzero, desyncing every level_log / level_pos
        // / fold-index and spuriously rejecting honest proofs. (The current
        // codegen always emits log_plaintext_size == maxSizeLog2(layout), so the
        // offset is 0 today — but the restricted frame is the correct one.)
        const round = p.log_plaintext_size - bundle.size_log2;
        if (round > num_rounds) return Error.LevelRoundExceedsSchedule;

        const level_log: u6 = @intCast(p.log_codeword_size - round);
        const level_pos = s >> @intCast(round);

        // alphaDeep for this level = (fold challenge of its own round)^2, or, at
        // the boundary round, the first power of the last fold challenge.
        // alphaDeep = (this round's own fold challenge)^2. At the boundary round
        // (round == numRounds, no challenge exists) use the first power of the
        // last fold challenge — distinct from round numRounds-1's square. With no
        // fold challenges at all (D=1), no level is ever folded and the constant
        // reconstruction is alpha-independent, so any value serves.
        var alpha_deep: ext.Ext = ext.Ext.one();
        if (round < fold_alphas.len) {
            alpha_deep = fold_alphas[round].square();
        } else if (fold_alphas.len > 0) {
            alpha_deep = fold_alphas[fold_alphas.len - 1];
        }

        // Validate row shapes and build resolved entries for self and sibling.
        var self_entries: [maxEntries]reconstruct.ResolvedEntry = undefined;
        var sib_entries: [maxEntries]reconstruct.ResolvedEntry = undefined;
        // Backing storage for each entry's claims (points + values).
        var claim_store: [maxEntries][maxShifts]reconstruct.Claim = undefined;

        const level_size = @as(usize, 1) << level_log;
        inline for (bundle.entries, 0..) |entry, ei| {
            const branch = input_opening[entry.batch_idx];
            const pair = try branch.pairAtLevel(level_size);
            // Shape check against the declared shape for this batch/size.
            const shape = comptime shapeFor(system, entry.batch_idx, entry.size_log2);
            if (!pair[0].matchesShape(shape.base_width, shape.ext_width) or
                !pair[1].matchesShape(shape.base_width, shape.ext_width))
            {
                return Error.RowShapeMismatch;
            }

            // Claims: (zeta·omega^shift, claimed value) for each shift.
            const claims = inputs.entry_claims[g + ei];
            for (entry.shifts, 0..) |shift, k| {
                claim_store[ei][k] = .{
                    .point = try reconstruct.shiftedPoint(entry.size_log2, shift, inputs.zeta),
                    .value = claims[k],
                };
            }
            const cs = claim_store[ei][0..entry.shifts.len];
            self_entries[ei] = .{ .row_value = reconstruct.rowValueForEntry(pair[0], entry), .claims = cs };
            sib_entries[ei] = .{ .row_value = reconstruct.rowValueForEntry(pair[1], entry), .claims = cs };
        }
        g += bundle.entries.len;

        const running_self = rq.rounds[round].self;
        const running_sib = rq.rounds[round].sibling;
        const x_self = try params_mod.domainPointExt(level_log, level_pos);
        const x_sib = try params_mod.domainPointExt(level_log, level_pos ^ 1);

        const self_val = try reconstruct.reconstructQueryValueAt(
            self_entries[0..bundle.entries.len],
            alpha_deep,
            x_self,
            running_self,
        );
        const sib_val = try reconstruct.reconstructQueryValueAt(
            sib_entries[0..bundle.entries.len],
            alpha_deep,
            x_sib,
            running_sib,
        );
        rq.aux[round] = .{ .self = self_val, .sibling = sib_val };
        rq.aux_present[round] = true;
    }
}

/// The declared shape for a (batch, size), read straight from the System's
/// per-batch shapes. Mirrors the Go check against `in.Shapes[batch][size]`.
fn shapeFor(comptime system: System, comptime batch_idx: usize, comptime size_log2: u8) layout_mod.SizedShape {
    comptime {
        const shape = system.shapes[batch_idx];
        if (size_log2 >= shape.len) return .{};
        return shape[size_log2];
    }
}

fn octEql(a: Octuplet, b: Octuplet) bool {
    for (a, b) |x, y| {
        if (!x.eql(y)) return false;
    }
    return true;
}
