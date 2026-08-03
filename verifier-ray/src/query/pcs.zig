const std = @import("std");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const merkle = @import("../crypto/merkle.zig");
const canonical = @import("../polynomial/canonical.zig");
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
    /// Offset into `VerifyInput.claimed_values`; this entry owns
    /// `claimed_values[claim_offset..][0..shifts.len]`.
    claim_offset: usize,
    /// Shift s means this row is claimed at zeta * omega_N^s, omega_N the
    /// generator of the size-2^size_log2 (bundle) domain.
    shifts: []const usize,
};

pub const SizeBundle = struct {
    size_log2: u8,
    entries: []const DeepEntry,
};

/// The canonical layout, compiled once by codegen from a protocol's batch
/// shapes and shift schedules: prover-ray's `layout`. Sizes appear in
/// descending order, matching `canonicalLayout`.
///
/// Carries no `log_plaintext_size`: it is `layout`'s largest bundle size, so
/// `params` derives it rather than codegen supplying a field that could
/// disagree. Mirrors prover-ray's `restrictToOpenings`, which performs the
/// same shrink at runtime because its `PCS` outlives any one proof.
pub const System = struct {
    log_codeword_size: u8,
    log_final_poly_size: u8 = 0,
    num_queries: usize,
    layout: []const SizeBundle,
    /// Index into `protocol.Context.all_coins` of the opening point zeta.
    zeta_coin_index: usize = 0,

    /// The `fri.Params` this system verifies against.
    pub fn params(comptime self: System) fri.Params {
        return .{
            .log_codeword_size = self.log_codeword_size,
            .log_plaintext_size = comptime maxSizeLog2(self.layout),
            .log_final_poly_size = self.log_final_poly_size,
            .num_queries = self.num_queries,
        };
    }
};

fn maxSizeLog2(comptime layout: []const SizeBundle) u8 {
    comptime {
        if (layout.len == 0) @compileError("fri: pcs: System.layout must have at least one size bundle");
        var max_size: u8 = layout[0].size_log2;
        for (layout[1..]) |bundle| {
            if (bundle.size_log2 > max_size) max_size = bundle.size_log2;
        }
        return max_size;
    }
}

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
    /// Flattened claims; entry e reads
    /// `claimed_values[e.claim_offset..][0..e.shifts.len]`.
    claimed_values: []const ext.Ext,
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
    const params = comptime system.params();
    return struct {
        fold_alphas: [params.numRounds()]ext.Ext = undefined,
        query_positions: [params.num_queries]usize = undefined,
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
    const params = comptime system.params();
    const num_rounds = comptime params.numRounds();
    const want_round_roots = if (num_rounds > 0) num_rounds - 1 else 0;
    if (fri_proof.round_roots.len != want_round_roots) return fri.Error.InvalidRoundRootCount;

    var challenges = PcsChallenges(system){};

    // One challenge per intermediate layer root, absorbing the root between
    // squeezes; then the final round's challenge, with no root after it. When a
    // protocol never folds (num_rounds == 0) the buffer is empty and this whole
    // block is elided at comptime rather than indexing into a [0]ext.Ext.
    if (comptime num_rounds > 0) {
        for (fri_proof.round_roots, 0..) |root, i| {
            challenges.fold_alphas[i] = transcript.randomExt();
            transcript.updateElements(&root);
        }
        challenges.fold_alphas[num_rounds - 1] = transcript.randomExt();
    }

    transcript.updateExt(fri_proof.final_poly);

    const codeword_size = comptime @as(usize, 1) << @intCast(params.log_codeword_size);
    transcript.randomManyIntegers(&challenges.query_positions, codeword_size);
    return challenges;
}

pub fn verify(comptime system: System, input: VerifyInput) Error!void {
    const params = comptime system.params();
    const info = comptime computeLayoutInfo(system.layout);

    if (input.roots.len != info.distinct_count) return Error.RootCountMismatch;
    if (input.claimed_values.len != info.total_claims) return Error.ClaimedValueCountMismatch;
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

        const final_point = fri.domainPointExt(params.log_codeword_size - num_rounds, query_position >> num_rounds);
        final_buf[query_idx] = canonical.evaluateExtAtExt(input.proof.fri_proof.final_poly, final_point);

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
                input.claimed_values,
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
                input.claimed_values,
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
            const sib_final_value = canonical.evaluateExtAtExt(input.proof.fri_proof.final_poly, sib_final);
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
    total_claims: usize,
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
        var total_claims: usize = 0;
        for (layout) |bundle| {
            for (bundle.entries) |entry| {
                if (!assigned[entry.batch_idx]) {
                    assigned[entry.batch_idx] = true;
                    index_by_batch[entry.batch_idx] = distinct_count;
                    distinct_count += 1;
                }
                total_claims += entry.shifts.len;
            }
        }
        const final_index_by_batch = index_by_batch;
        return .{ .index_by_batch = &final_index_by_batch, .distinct_count = distinct_count, .total_claims = total_claims };
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
        // num_levels is proof-controlled: bounding it before the shift keeps
        // an oversized value from overflow-trapping the cast, and makes
        // num_leaves <= codeword_size follow.
        if (num_levels == 0 or num_levels > params.log_codeword_size) return Error.InputTreeShapeMismatch;
        const num_leaves = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(num_levels));
        if (codeword_size % num_leaves != 0) return Error.InputTreeShapeMismatch;
        const recovered = try branch.recoverRoot(query_position / (codeword_size / num_leaves));
        if (!poseidon2.eql(recovered, root)) return Error.MerkleProofInvalid;
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
    claimed_values: []const ext.Ext,
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
            const numerator = entry_value.sub(claimed_values[entry.claim_offset + k]);
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
/// one. Mirrors prover-ray's `pointInDomain`. `log_size` is comptime so the
/// exponent is too, letting `powComptime` resolve its square-and-multiply
/// schedule at compile time.
fn pointInDomain(point: ext.Ext, comptime log_size: u8) bool {
    if (!point.isBase()) return false;
    const powered = point.B0.a0.powComptime(@as(usize, 1) << log_size);
    return powered.eql(field.Element.one());
}
