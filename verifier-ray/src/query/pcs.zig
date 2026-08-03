const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const merkle = @import("../crypto/merkle.zig");
const fiat_shamir = @import("../crypto/fiat_shamir.zig");
const fri = @import("fri.zig");

/// The PCS/DEEP-quotient layer: input-tree authentication, per-level Horner
/// DEEP-quotient reconstruction, and the boundary-round / D=1 special cases,
/// ported from prover-ray's `pcs.go` `PCS.Verify`. Produces the
/// `fri.ResolvedQuery` records `fri.checkFolds` needs.
///
/// Unlike prover-ray, `System.layout` is not recomputed per proof from raw
/// `Shape`/`BatchShifts` inputs: it is the already-compiled canonical layout
/// (prover-ray's `canonicalLayout` output), supplied as comptime data by
/// codegen -- mirroring how `query/vanishing.zig`'s `System.modules` already
/// bakes in static shape. A `SizedShape{BaseWidth,ExtWidth}` is exactly "how
/// many base/ext entries this bundle has for a batch", so it is derived from
/// the layout rather than carried as a separate, possibly-inconsistent input.
pub const Error = merkle.Error || fri.Error || error{
    RootCountMismatch,
    ClaimedValueCountMismatch,
    ZetaZeroWithMultipleShifts,
    ClaimPointOnDomain,
    InputQueryCountMismatch,
    QueryPositionCountMismatch,
    InputTreeCountMismatch,
    InputTreeShapeMismatch,
    RowShapeMismatch,
    ConjugateRowShapeMismatch,
    ClaimPointOnQueryPoint,
    MissingTopLevelAux,
    BoundaryFinalSelfMismatch,
    BoundaryFinalSiblingMismatch,
};

/// One committed column's opening schedule within a size bundle: prover-ray's
/// `deepEntry`, minus `AlphaPower` (implied by array position -- entries
/// combine highest-index first, i.e. reverse array order) and minus a
/// separate row-count input (this entry IS the (batch, size, row)
/// declaration the shape check needs).
pub const DeepEntry = struct {
    batch_idx: usize,
    is_ext: bool,
    row_idx: usize,
    /// Index of this opened column into `VerifyInput.entry_claims`, in canonical
    /// layout order (bundles high→low size, then entries within a bundle). This
    /// entry owns the whole inner slice `entry_claims[entry_idx]`, whose length
    /// equals `shifts.len` (one claimed value per opened rotation).
    entry_idx: usize,
    /// Shift s means this row is claimed at zeta * omega_N^s, omega_N the
    /// generator of the size-2^size_log2 (bundle) domain.
    shifts: []const usize,
};

pub const SizeBundle = struct {
    size_log2: u8,
    entries: []const DeepEntry,
};

/// Locates one vanishing claim inside the flat `entry_claims`: opened column
/// `entry` (canonical layout order) at its `shift`-th opened rotation. The
/// vanishing sub-verifier's k-th witness/quotient claim is exactly
/// `entry_claims[entry][shift]` — the same runtime cell prover-ray's PCS compiler
/// opens. Emitted by codegen from the LagrangeEval ↔ committed-column binding so
/// the higher-level verifier can feed PCS-*authenticated* claims into vanishing.
pub const ClaimRef = struct {
    entry: usize,
    shift: usize,
};

/// Where a committed batch's Merkle root is bound. A batch root MUST be tied to
/// the Fiat-Shamir transcript that derives zeta — otherwise a prover could open
/// against a forged root while zeta stays bound to the honest commitment, and
/// the opening would be authenticated at a point it is not committed to.
///
/// Two provenances, mirroring prover-ray's single-source `collectRoots` (which
/// reads `rt.Commitments` — the exact octuplets the protocol replay absorbs into
/// the transcript):
///   - `.round`: an interactive batch. Its root IS the oracle commitment absorbed
///     in round message `round_index` (the same one squeezed into zeta), read
///     from `proof.rounds[round_index]` rather than the proof's opening payload.
///   - `.precomputed`: the static precomputed batch. Its root is a compile-time
///     constant (like a verification key), fixed by codegen and trusted by
///     construction — never carried in a round message.
pub const BatchRoot = union(enum) {
    /// Index into `proof.rounds`; the batch root is that round's sole oracle
    /// commitment (`rounds[round_index].columns[0].oracle_commitment`).
    round: usize,
    /// Compile-time precomputed-batch root, emitted by codegen.
    precomputed: poseidon2.Digest,
};

/// The canonical layout, compiled once by codegen from a protocol's batch
/// shapes and shift schedules: prover-ray's `layout`. Sizes appear in
/// descending order, matching `canonicalLayout`.
pub const System = struct {
    params: fri.Params,
    layout: []const SizeBundle,

    /// Number of distinct committed batches referenced by `layout` — the length
    /// of the authenticated roots slice `verify` consumes and of `batch_roots`.
    num_batches: usize,

    /// Per-batch root provenance, in canonical batch order (index == batch
    /// index). `verifier.verify` builds the authenticated batch roots from this —
    /// reading interactive-batch roots out of the transcript-bound round oracle
    /// commitments and precomputed roots from the compile-time constant — so the
    /// root a batch is Merkle-authenticated against is provably the one zeta is
    /// bound to. Length MUST equal `num_batches`.
    ///
    /// Empty (`&.{}`) only for legacy callers that pass `VerifyInput.roots`
    /// directly (the standalone engine tests); the full `verifier.verify` path
    /// requires it.
    batch_roots: []const BatchRoot = &.{},

    /// Routes each vanishing witness/quotient claim to its authenticated value in
    /// the flat `entry_claims`. `witness_map[k]` (resp. `quotient_map[k]`) is the
    /// `ClaimRef` for the vanishing sub-verifier's k-th witness (resp. quotient)
    /// claim. `verifier.verify` uses these to feed the *PCS-authenticated* claims
    /// into the vanishing check, so a prover cannot hand the two sub-verifiers
    /// different values for the same column. Lengths must equal the vanishing
    /// System's `total_witness_claims` / `total_quotient_claims`. Empty for the
    /// standalone engine tests (which never route into vanishing).
    witness_map: []const ClaimRef = &.{},
    quotient_map: []const ClaimRef = &.{},

    /// Flat `all_coins` index of zeta — the shared opening point, also the
    /// vanishing modules' eval coin. `verifier.verify` reads
    /// `all_coins[zeta_coin_index]` and passes it as the opening point, so zeta is
    /// Fiat-Shamir-derived, never taken from the proof. Required when PCS is
    /// enabled: `null` makes `verifier.verify` fail at comptime rather than
    /// silently selecting coin 0 (a mis-binding that would break soundness).
    zeta_coin_index: ?usize = null,
};

pub const OpeningProof = struct {
    /// input_queries[q][i] is query q's opening of the i-th distinct input
    /// tree (first-declaration order among `system.layout`'s batches).
    input_queries: []const []const merkle.InputTreeOpening,
    fri_proof: fri.Proof,
};

pub const VerifyInput = struct {
    /// One root per distinct batch index referenced in `system.layout`, in
    /// first-declaration order (prover-ray's `inputOpeningRoots`, computed
    /// here at comptime instead of at verify time since batch identity is
    /// already static).
    roots: []const poseidon2.Digest,
    /// Per-opened-column claimed evaluations, in canonical layout order:
    /// `entry_claims[e.entry_idx][k]` is the claim for entry `e`'s `k`-th shift,
    /// so `entry_claims[e.entry_idx].len == e.shifts.len`. Jagged rather than a
    /// flat slice so a `ClaimRef{entry, shift}` (used by the higher-level
    /// verifier to feed PCS-authenticated claims to other sub-verifiers) is a
    /// direct double-index with no offset arithmetic.
    entry_claims: []const []const ext.Ext,
    zeta: ext.Ext,
    fold_alphas: []const ext.Ext,
    query_positions: []const usize,
    proof: OpeningProof,
};

/// The Fiat-Shamir–derived PCS challenges consumed by `verify`: the per-round
/// fold challenges and the FRI query positions. Both are sized by the System's
/// comptime `params`, so nothing allocates. Produced by `deriveChallenges` (the
/// transcript-touching half) and passed to `verify` (pure arithmetic) — this
/// separation lets the caller thread one transcript through replay → PCS.
pub fn PcsChallenges(comptime system: System) type {
    const num_rounds = comptime system.params.numRounds();
    return struct {
        fold_alphas: [num_rounds]ext.Ext = undefined,
        query_positions: [system.params.num_queries]usize = undefined,
    };
}

/// Derives the FRI fold challenges and query positions by continuing
/// `transcript` — the live Fiat-Shamir state `protocol.replayWithTranscript`
/// left after squeezing the protocol coins (mirroring prover-ray's
/// `fs := rt.GetFS()`), so the proof cannot dictate them. Only the running-layer
/// roots and the final polynomial are absorbed; the challenges are pure squeezes
/// returned as a value. `transcript` is caller-owned and advanced in place.
///
/// This is the transcript-touching counterpart of `verify`, which then checks
/// the proof against these challenges without touching the transcript.
///
/// `fri_proof.round_roots` must hold exactly `num_rounds - 1` intermediate-layer
/// roots (the shape `verify` later re-checks). This is validated up front rather
/// than trusted: the absorb loop indexes the fixed-size `fold_alphas` buffer by
/// root position, so an over-long `round_roots` would otherwise write past it —
/// a stack overflow in release builds, where bounds checks are disabled.
pub fn deriveChallenges(
    comptime system: System,
    transcript: *fiat_shamir.Transcript,
    fri_proof: fri.Proof,
) fri.Error!PcsChallenges(system) {
    const num_rounds = comptime system.params.numRounds();
    const want_round_roots = if (num_rounds > 0) num_rounds - 1 else 0;
    if (fri_proof.round_roots.len != want_round_roots) return fri.Error.InvalidRoundRootCount;

    var challenges = PcsChallenges(system){};

    // One challenge per intermediate layer root, absorbing the root between
    // squeezes. When a protocol never folds (num_rounds == 0) `fold_alphas` is a
    // [0]ext.Ext, so this loop is elided at comptime rather than indexing into an
    // empty array.
    if (comptime num_rounds > 0) {
        for (fri_proof.round_roots, 0..) |root, i| {
            challenges.fold_alphas[i] = transcript.randomExt();
            transcript.updateElements(&root);
        }
    }

    // The final round's challenge is squeezed with no root after it. prover-ray
    // (`wiop/compilers/pcs/pcs.go` `verify`) squeezes this UNCONDITIONALLY,
    // including the D=1 (num_rounds == 0) case where the loop above is empty. The
    // squeeze mutates the transcript (the safeguard update after every squeeze)
    // BEFORE `final_poly` is absorbed and the query positions are drawn, so
    // skipping it for D=1 would desynchronise the positions from the prover.
    // Squeeze it always; keep it only when a fold slot exists (for D=1 there is
    // none, so the value is discarded — the transcript side effect is the point).
    const final_alpha = transcript.randomExt();
    if (comptime num_rounds > 0) challenges.fold_alphas[num_rounds - 1] = final_alpha;

    transcript.updateExt(fri_proof.final_poly);

    const codeword_size = comptime @as(usize, 1) << @intCast(system.params.log_codeword_size);
    transcript.randomManyIntegers(&challenges.query_positions, codeword_size);
    return challenges;
}

pub fn verify(comptime system: System, input: VerifyInput) Error!void {
    const params = system.params;
    const info = comptime computeLayoutInfo(system.layout);

    if (input.roots.len != info.distinct_count) return Error.RootCountMismatch;
    if (input.entry_claims.len != info.total_entries) return Error.ClaimedValueCountMismatch;
    // Each opened column owns exactly `shifts.len` claimed values. Checking the
    // per-entry shape up front lets `reconstructQueryValueAt` index
    // `entry_claims[entry_idx][k]` without a bounds check in the hot loop.
    inline for (system.layout) |bundle| {
        inline for (bundle.entries) |entry| {
            if (input.entry_claims[entry.entry_idx].len != entry.shifts.len) return Error.ClaimedValueCountMismatch;
        }
    }
    if (comptime systemHasMultiShiftEntry(system)) {
        if (input.zeta.isZero()) return Error.ZetaZeroWithMultipleShifts;
    }

    inline for (system.layout) |bundle| {
        const round = comptime roundForSize(params, bundle.size_log2);
        const domain_log_size = params.log_codeword_size - round;
        if (pointInDomain(input.zeta, domain_log_size)) return Error.ClaimPointOnDomain;
    }

    if (input.proof.input_queries.len != params.num_queries) return Error.InputQueryCountMismatch;
    if (input.query_positions.len < params.num_queries) return Error.QueryPositionCountMismatch;
    try fri.checkOpeningProofShape(params, input.proof.fri_proof, input.fold_alphas, input.query_positions[0..params.num_queries]);

    const num_rounds = comptime params.numRounds();
    var rounds_buf: [params.num_queries][if (num_rounds > 0) num_rounds else 1]fri.Pair = undefined;
    var aux_buf: [params.num_queries][num_rounds + 1]?fri.Pair = undefined;
    var final_buf: [params.num_queries]ext.Ext = undefined;
    var resolved: [params.num_queries]fri.ResolvedQuery = undefined;

    for (0..params.num_queries) |query_idx| {
        @memset(&aux_buf[query_idx], null);
        for (&rounds_buf[query_idx]) |*pair| pair.* = .{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() };

        const query_position = input.query_positions[query_idx];
        const opening = input.proof.input_queries[query_idx];
        try authenticateInputQuery(params, opening, input.roots, info, query_position);

        if (num_rounds > 0) {
            const running_query = input.proof.fri_proof.running_queries[query_idx];
            try fri.resolveRunningLayers(params, input.proof.fri_proof.round_roots, running_query, query_position, rounds_buf[query_idx][0..num_rounds]);
        }

        final_buf[query_idx] = fri.domainPointExt(params.log_codeword_size - num_rounds, query_position >> num_rounds);
        final_buf[query_idx] = evalFinalPoly(input.proof.fri_proof.final_poly, final_buf[query_idx]);

        inline for (system.layout) |bundle| {
            const round = comptime roundForSize(params, bundle.size_log2);
            const domain_log_size = params.log_codeword_size - round;
            const level_size = @as(usize, 1) << domain_log_size;

            try bindInputTreeOpenings(bundle, opening, info, level_size);

            // fold_alphas.len == num_rounds exactly (checkOpeningProofShape
            // above), so this matches prover-ray's round < len(foldAlphas).
            const alpha_deep: ext.Ext = if (round < num_rounds)
                input.fold_alphas[round].square()
            else if (num_rounds > 0)
                input.fold_alphas[num_rounds - 1]
            else
                ext.Ext.zero();

            const level_pos = query_position >> round;
            const seed = seedPair(rounds_buf[query_idx][0..num_rounds], round, num_rounds);

            const self_val = try reconstructQueryValueAt(
                bundle,
                opening,
                info,
                level_size,
                input.entry_claims,
                input.zeta,
                alpha_deep,
                fri.domainPointExt(domain_log_size, level_pos),
                false,
                seed.self,
            );
            const sib_val = try reconstructQueryValueAt(
                bundle,
                opening,
                info,
                level_size,
                input.entry_claims,
                input.zeta,
                alpha_deep,
                fri.domainPointExt(domain_log_size, level_pos ^ 1),
                true,
                seed.sibling,
            );
            aux_buf[query_idx][round] = .{ .self = self_val, .sibling = sib_val };
        }

        if (comptime num_rounds == 0) {
            const pair = aux_buf[query_idx][0] orelse return Error.MissingTopLevelAux;
            const sib_final = fri.domainPointExt(params.log_codeword_size, query_position ^ 1);
            const sib_final_value = evalFinalPoly(input.proof.fri_proof.final_poly, sib_final);
            if (!pair.self.eql(final_buf[query_idx])) return Error.BoundaryFinalSelfMismatch;
            if (!pair.sibling.eql(sib_final_value)) return Error.BoundaryFinalSiblingMismatch;
        }

        resolved[query_idx] = .{
            .rounds = rounds_buf[query_idx][0..num_rounds],
            .aux = &aux_buf[query_idx],
            .final = final_buf[query_idx],
        };
    }

    try fri.checkFolds(params, &resolved, input.fold_alphas, input.query_positions);
}

/// Evaluates the revealed final-polynomial coefficients at `x` via Horner's
/// method, rather than expanding the whole final-domain codeword via FFT
/// (prover-ray's approach): the verifier only ever needs one or two point
/// evaluations per query, never the full codeword.
fn evalFinalPoly(coeffs: []const ext.Ext, x: ext.Ext) ext.Ext {
    var acc = ext.Ext.zero();
    var i = coeffs.len;
    while (i != 0) {
        i -= 1;
        acc = acc.mul(x).add(coeffs[i]);
    }
    return acc;
}

/// The running-codeword seed for a level introduced at `round`: zero at
/// round 0 (no round -1 to fold from) and at the boundary round `num_rounds`
/// (one past the last fold, never itself folded), otherwise the
/// already-authenticated running pair. Mirrors `rq.Rounds[round]` always
/// being valid in prover-ray, where `Rounds` carries an explicit zero-seeded
/// slot at both ends.
fn seedPair(rounds: []const fri.Pair, round: u8, num_rounds: u8) fri.Pair {
    if (round == 0 or round == num_rounds) return .{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() };
    return rounds[round];
}

/// Maps a bundle's size to its introduction round: prover-ray's
/// `roundForSize`. Comptime, since the layout (and hence every bundle's
/// size) is fixed protocol configuration, not proof data -- an out-of-range
/// size here is a codegen bug, not something a malicious prover controls.
fn roundForSize(comptime params: fri.Params, comptime size_log2: u8) u8 {
    comptime {
        if (size_log2 > params.log_plaintext_size) @compileError("fri: pcs: size exceeds params schedule");
        const round = params.log_plaintext_size - size_log2;
        if (round > params.numRounds()) @compileError("fri: pcs: level introduced past the boundary round");
        return round;
    }
}

/// Everything about `System.layout` that isn't already explicit in the
/// layout itself: the batch-dedup mapping (prover-ray's `inputOpeningRoots`,
/// done here by index since batch_idx is the identity of one commitment --
/// codegen never assigns two distinct indices to the same commitment, so
/// this always agrees with prover-ray's by-root-value dedup) and the total
/// flattened claim count. One comptime walk over the layout, so neither can
/// silently disagree with the layout describing them.
const LayoutInfo = struct {
    index_by_batch: []const usize,
    distinct_count: usize,
    /// Number of opened columns across the whole layout — the length of
    /// `VerifyInput.entry_claims`.
    total_entries: usize,
};

fn computeLayoutInfo(comptime layout: []const SizeBundle) LayoutInfo {
    comptime {
        var num_batches: usize = 0;
        for (layout) |bundle| {
            for (bundle.entries) |entry| {
                if (entry.batch_idx >= num_batches) num_batches = entry.batch_idx + 1;
            }
        }

        var index_by_batch: [num_batches]usize = undefined;
        var assigned: [num_batches]bool = [_]bool{false} ** num_batches;
        var distinct_count: usize = 0;
        var total_entries: usize = 0;
        for (layout) |bundle| {
            for (bundle.entries) |entry| {
                if (!assigned[entry.batch_idx]) {
                    assigned[entry.batch_idx] = true;
                    index_by_batch[entry.batch_idx] = distinct_count;
                    distinct_count += 1;
                }
                total_entries += 1;
            }
        }
        const final_index_by_batch = index_by_batch;
        return .{ .index_by_batch = &final_index_by_batch, .distinct_count = distinct_count, .total_entries = total_entries };
    }
}

fn systemHasMultiShiftEntry(comptime system: System) bool {
    comptime {
        for (system.layout) |bundle| {
            for (bundle.entries) |entry| {
                if (entry.shifts.len > 1) return true;
            }
        }
        return false;
    }
}

/// Distinct batch indices appearing in `bundle`, in first-declaration order.
/// Entries for one batch are contiguous within a bundle (canonicalLayout
/// walks batches in declaration order, then rows within a batch), so this is
/// exactly `bindInputTreeOpenings`'s per-batch (not per-row) shape check.
fn distinctBatchesInBundle(comptime bundle: SizeBundle) []const usize {
    comptime {
        var buf: [bundle.entries.len]usize = undefined;
        var count: usize = 0;
        outer: for (bundle.entries) |entry| {
            for (buf[0..count]) |seen| {
                if (seen == entry.batch_idx) continue :outer;
            }
            buf[count] = entry.batch_idx;
            count += 1;
        }
        const result = buf[0..count];
        return result;
    }
}

fn bundleBatchWidths(comptime bundle: SizeBundle, comptime batch_idx: usize) struct { base: usize, ext: usize } {
    comptime {
        var base: usize = 0;
        var ext_width: usize = 0;
        for (bundle.entries) |entry| {
            if (entry.batch_idx != batch_idx) continue;
            if (entry.is_ext) ext_width += 1 else base += 1;
        }
        return .{ .base = base, .ext = ext_width };
    }
}

/// Authenticates every distinct input tree once per query against its known
/// root: prover-ray's `authenticateInputQuery`. Uses the opened branch's own
/// declared depth (`leaves.len`), exactly like prover-ray -- a branch with a
/// depth mismatching the actual committed tree fails the root comparison
/// regardless, so no separate declared-shape input is needed here.
fn authenticateInputQuery(
    comptime params: fri.Params,
    opening: []const merkle.InputTreeOpening,
    roots: []const poseidon2.Digest,
    comptime info: LayoutInfo,
    query_position: usize,
) Error!void {
    if (opening.len != info.distinct_count or roots.len != info.distinct_count) return Error.InputTreeCountMismatch;

    const codeword_size = @as(usize, 1) << params.log_codeword_size;
    for (opening, roots) |branch, root| {
        const num_levels = branch.leaves.len;
        // `num_levels` is attacker-controlled (the proof's leaves slice length).
        // Bound it to the codeword depth BEFORE the shift below: a tree can have
        // at most `log_codeword_size` levels, and without this guard a
        // `num_levels >= @bitSizeOf(Log2Int(usize))` value would truncate in the
        // `@intCast` and yield a bogus `num_leaves` (e.g. 1) that slips past the
        // divisibility check. In ReleaseSmall/Fast (R5) there is no @intCast trap.
        if (num_levels == 0 or num_levels > params.log_codeword_size) return Error.InputTreeShapeMismatch;
        const num_leaves = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(num_levels));
        if (num_leaves > codeword_size or codeword_size % num_leaves != 0) return Error.InputTreeShapeMismatch;
        const recovered = try branch.recoverRoot(query_position / (codeword_size / num_leaves));
        if (!std.meta.eql(recovered, root)) return Error.MerkleProofInvalid;
    }
}

/// Validates that each batch present in `bundle` carries a conjugate pair
/// matching its declared (base, ext) width at `level_size`, for both rows of
/// the pair -- the conjugate is unread by the fold today but is still
/// transmitted, so an unvalidated conjugate would be a malleable proof.
/// Mirrors prover-ray's `bindInputTreeOpenings`.
fn bindInputTreeOpenings(
    comptime bundle: SizeBundle,
    opening: []const merkle.InputTreeOpening,
    comptime info: LayoutInfo,
    level_size: usize,
) Error!void {
    const batches = comptime distinctBatchesInBundle(bundle);
    inline for (batches) |batch_idx| {
        const widths = comptime bundleBatchWidths(bundle, batch_idx);
        const branch_idx = info.index_by_batch[batch_idx];
        const pair = try opening[branch_idx].pairAtLevel(level_size);
        if (pair[0].base.len != widths.base or pair[0].ext.len != widths.ext) return Error.RowShapeMismatch;
        if (pair[1].base.len != widths.base or pair[1].ext.len != widths.ext) return Error.ConjugateRowShapeMismatch;
    }
}

/// Combines `bundle`'s columns with `running` (this level's own round's
/// running-codeword value) at `x`, the same way prover-ray's `Level.EvalsAt`
/// combines a level's columns with the prover's running codeword. Entries
/// are walked highest-alphaDeep-power first, which canonicalLayout's
/// assignment makes simply the reverse array order. Mirrors prover-ray's
/// `reconstructQueryValueAt`.
fn reconstructQueryValueAt(
    comptime bundle: SizeBundle,
    opening: []const merkle.InputTreeOpening,
    comptime info: LayoutInfo,
    level_size: usize,
    entry_claims: []const []const ext.Ext,
    zeta: ext.Ext,
    alpha_deep: ext.Ext,
    x: ext.Ext,
    sibling: bool,
    running: ext.Ext,
) Error!ext.Ext {
    var value = running;
    comptime var i = bundle.entries.len;
    inline while (i > 0) {
        i -= 1;
        const entry = bundle.entries[i];
        const branch_idx = info.index_by_batch[entry.batch_idx];
        const pair = try opening[branch_idx].pairAtLevel(level_size);
        const row = if (sibling) pair[1] else pair[0];
        const entry_value: ext.Ext = if (entry.is_ext) row.ext[entry.row_idx] else ext.Ext.lift(row.base[entry.row_idx]);

        var term = ext.Ext.zero();
        inline for (entry.shifts, 0..) |shift, k| {
            const point = shiftedPoint(bundle.size_log2, shift, zeta);
            const denom = x.sub(point);
            if (denom.isZero()) return Error.ClaimPointOnQueryPoint;
            const numerator = entry_value.sub(entry_claims[entry.entry_idx][k]);
            term = term.add(numerator.mul(denom.inverse()));
        }
        value = value.mul(alpha_deep).add(term);
    }
    return value;
}

/// zeta * omega_N^shift, omega_N the generator of the size-2^size_log2
/// domain: prover-ray's `shiftedPoint`. `size_log2` and `shift` are comptime
/// (both come from the layout), so the rotation itself is a compile-time
/// constant -- no runtime exponentiation.
fn shiftedPoint(comptime size_log2: u8, comptime shift: usize, zeta: ext.Ext) ext.Ext {
    const rotation = comptime (field.rootOfUnityBy(@as(usize, 1) << size_log2) catch
        @compileError("fri: pcs: size_log2 exceeds the supported KoalaBear root-of-unity order")).powComptime(shift);
    return zeta.mulByBase(rotation);
}

/// Whether `point` lands in the size-2^log_size multiplicative subgroup.
/// Every domain point is a base-field root of unity, so an extension-valued
/// point that isn't itself a lifted base element can never coincide with
/// one. Mirrors prover-ray's `pointInDomain`.
fn pointInDomain(point: ext.Ext, log_size: u8) bool {
    if (!point.isBase()) return false;
    const shift: std.math.Log2Int(u64) = @intCast(log_size);
    const powered = point.B0.a0.pow(@as(u64, 1) << shift);
    return powered.eql(field.Element.one());
}
