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
/// The canonical layout is NOT frozen at codegen time from one proving run:
/// it is RECONSTRUCTED at verify time from the comptime `System.columns`
/// (prover declaration order) plus the runtime `module_sizes`. This mirrors
/// prover-ray's own `GetLayout` (per-column size_log2 + running position) +
/// `canonicalLayout` (size DESC / batch ASC / base-then-ext / position ASC)
/// so ONE baked System verifies proofs of DIFFERENT dynamic-module sizes.
/// Stack-only: buffers are sized by the comptime envelope maxima
/// (`max_entries`, `max_size_log2`), with runtime lengths from the proof.
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
    MissingDynamicModuleSize,
    NonPowerOfTwoModuleSize,
    DynamicModuleSizeBelowMinimum,
    RestrictOutOfRange,
    LayoutOverflow,
};

/// A committed column's size source. `.static` bakes a comptime size_log2 (a
/// static-module column, whose padded size is fixed at compile time).
/// `.dynamic` names an index into the runtime `module_sizes` slice plus the
/// minimum runtime size_log2 this raw shift schedule is valid for. The
/// dynamic-module column's size_log2 = log2(module_sizes[idx]) varies per
/// proof, but proofs below `min_size_log2` are rejected because the baked raw
/// shift schedule would alias there. Mirrors how prover-ray's `GetLayout`
/// derives size_log2 from `col.Module.RuntimeSize(rt)`.
pub const SizeSource = union(enum) {
    static: u8,
    dynamic: struct {
        index: usize,
        min_size_log2: u8,
    },
};

/// One committed column, in prover DECLARATION order (batch-major, then
/// `round.Columns` order). This is the symbolic descriptor the verifier
/// reconstructs the canonical layout from — the size-independent invariants
/// (which batch, base/ext, shift schedule) are fixed; only `size` (hence the
/// column's bundle and position) varies per proof.
pub const ColumnDesc = struct {
    batch_idx: usize,
    is_ext: bool,
    size: SizeSource,
    /// Raw opening offsets. Offset o means this column is claimed at
    /// zeta * omega_N^(o mod N), N = 2^size_log2 the bundle domain size. Stored
    /// RAW (size-independent) — the verifier normalizes o mod N at the runtime
    /// size, so one baked System's shifts work at every dynamic size. May be
    /// negative (a back-shift).
    shifts: []const isize,
};

/// Routes one vanishing witness/quotient claim to its authenticated value.
/// `col_decl_idx` names a column by its declaration index (into
/// `System.columns`), resolved to a runtime entry index by the reconstruction;
/// `shift` is the slot within that column's shift schedule. Emitted by codegen
/// from the LagrangeEval ↔ committed-column binding.
pub const ClaimRef = struct {
    col_decl_idx: usize,
    shift: usize,
};

/// Where a committed batch's Merkle root is bound. A batch root MUST be tied to
/// the Fiat-Shamir transcript that derives zeta — otherwise a prover could open
/// against a forged root while zeta stays bound to the honest commitment.
pub const BatchRoot = union(enum) {
    /// Index into `proof.rounds`; the batch root is that round's sole oracle
    /// commitment.
    round: usize,
    /// Compile-time precomputed-batch root, emitted by codegen.
    precomputed: poseidon2.Digest,
};

/// The symbolic PCS descriptor, compiled once by codegen. The layout is
/// reconstructed at verify time from `columns` + `module_sizes` (see
/// `reconstruct`), so one System covers many dynamic sizes.
pub const System = struct {
    /// The ENVELOPE FRI params (prover-ray's static maxCommittableSizeLog2
    /// schedule), NOT restricted to any one proof. `reconstruct` restricts it
    /// to each proof's largest opened size via `restrictTo`.
    envelope_params: fri.Params,

    /// Every committed column, in prover declaration order.
    columns: []const ColumnDesc,

    /// Number of distinct committed batches.
    num_batches: usize,

    /// Per-batch root provenance, in canonical batch order (index == batch
    /// index). Length MUST equal `num_batches`.
    batch_roots: []const BatchRoot = &.{},

    /// Routes each vanishing witness/quotient claim to its authenticated value.
    witness_map: []const ClaimRef = &.{},
    quotient_map: []const ClaimRef = &.{},

    /// Flat `all_coins` index of zeta — the shared opening point.
    zeta_coin_index: ?usize = null,

    /// ENVELOPE bounds for stack buffers. `max_entries == columns.len` (the
    /// fixed number of opened columns); `max_size_log2 == envelope
    /// log_plaintext_size` (the buffer depth for rounds/bundles).
    max_entries: usize,
    max_size_log2: u8,
};

pub const OpeningProof = struct {
    /// input_queries[q][i] is query q's opening of the i-th distinct input
    /// tree (input-opening order: first batch encountered by the canonical
    /// size-descending layout, with equal Merkle roots deduplicated).
    input_queries: []const []const merkle.InputTreeOpening,
    fri_proof: fri.Proof,
};

pub const VerifyInput = struct {
    /// One Merkle root per committed batch, in canonical batch order
    /// (`batch_idx == roots[idx]`). `verify` reorders and deduplicates these
    /// into the proof's input-opening order, mirroring prover-ray's
    /// `inputOpeningRoots`.
    roots: []const poseidon2.Digest,
    /// Per-opened-column claimed evaluations, in reconstructed canonical layout
    /// order: `entry_claims[entry_idx][k]` is the claim for that entry's `k`-th
    /// shift. Jagged.
    entry_claims: []const []const ext.Ext,
    zeta: ext.Ext,
    fold_alphas: []const ext.Ext,
    /// The top-level DEEP batching challenge. When FRI has one or more fold
    /// rounds this equals the last entry of `fold_alphas`; when the largest
    /// opened size is 1 (num_rounds == 0), prover-ray still squeezes this final
    /// alpha and uses it as alpha_DEEP even though there are no fold alphas.
    deep_alpha: ext.Ext = ext.Ext.zero(),
    query_positions: []const usize,
    proof: OpeningProof,
    /// Runtime dynamic-module sizes, in canonical dynamic-module order (the
    /// same order `SizeSource.dynamic.index` indices into). Powers of two, and
    /// each one must satisfy the emitted `min_size_log2` bound for every column
    /// that references it.
    module_sizes: []const usize = &.{},
};

// =============================================================================
// Runtime layout reconstruction
// =============================================================================

/// The reconstructed canonical layout for one proof: envelope-max-sized stack
/// arrays plus runtime lengths. Every entry field is indexed by entry_idx in
/// canonical order (size DESC / batch ASC / base-then-ext / position ASC).
pub fn Reconstructed(comptime system: System) type {
    return struct {
        // At least 1 so a batch-free System (max_entries == 0, used only to reach
        // the transcript replay before PCS) still yields indexable buffers; the
        // runtime lengths (num_entries == 0) keep every loop empty.
        const cap = @max(system.max_entries, 1);

        /// restricted FRI params for THIS proof (envelope.restrictTo(top_size)).
        params: fri.Params,
        /// runtime number of opened columns (== system.columns.len).
        num_entries: usize,
        /// runtime max size_log2 across columns (== restricted log_plaintext_size).
        top_size: u8,

        // Per-entry arrays, canonical order, filled [0, num_entries).
        entry_size_log2: [cap]u8 = undefined,
        entry_batch: [cap]usize = undefined,
        entry_is_ext: [cap]bool = undefined,
        entry_row_idx: [cap]usize = undefined,
        entry_col_decl_idx: [cap]usize = undefined,

        /// col_to_entry[c] = the canonical entry index of column c (decl order).
        col_to_entry: [cap]usize = undefined,
    };
}

/// The proof's input-tree routing derived from the reconstructed layout and the
/// actual per-batch roots: distinct roots in input-opening order plus the
/// input-tree branch index each batch maps to.
pub fn InputRootRouting(comptime system: System) type {
    return struct {
        const Self = @This();
        const batch_cap = @max(system.num_batches, 1);

        distinct_count: usize = 0,
        roots: [batch_cap]poseidon2.Digest = undefined,
        index_by_batch: [batch_cap]usize = [_]usize{std.math.maxInt(usize)} ** batch_cap,

        pub fn distinctRoots(self: *const Self) []const poseidon2.Digest {
            return self.roots[0..self.distinct_count];
        }
    };
}

/// Reconstructs the canonical layout at verify time from `system.columns` and
/// runtime `module_sizes`, byte-faithfully mirroring prover-ray's
/// `GetLayout` (per-column size_log2 + running position within
/// (size_log2, is_ext) in declaration order) + `canonicalLayout` (size DESC /
/// batch ASC / base-then-ext / position ASC). Stack-only.
pub fn reconstruct(comptime system: System, module_sizes: []const usize) Error!Reconstructed(system) {
    const R = Reconstructed(system);
    var r: R = undefined;
    const num_cols = system.columns.len;
    if (num_cols > system.max_entries) return Error.LayoutOverflow;
    r.num_entries = num_cols;

    // Per-column size_log2, and per-column position within its
    // (size_log2, is_ext) bucket, in declaration order. Prover GetLayout keeps a
    // running BaseWidth/ExtWidth counter PER (batch, size_log2); here the counter
    // must also be per (batch, size_log2, is_ext), since canonicalLayout groups
    // by batch and each batch's positions restart at 0. We compute it directly
    // during enumeration instead (see below), but we still need per-column
    // size_log2 first.
    var col_size_log2: [@max(system.max_entries, 1)]u8 = undefined;
    // col_position[c] = number of earlier declaration-order columns sharing c's
    // (batch, size_log2, is_ext) bucket, computed via running per-bucket
    // counters in a single O(columns) pass (replaces an O(columns^2) rescan).
    var col_position: [@max(system.max_entries, 1)]usize = undefined;
    const num_batches_cap = @max(system.num_batches, 1);
    var bucket_count: [num_batches_cap][system.max_size_log2 + 1][2]usize =
        [_][system.max_size_log2 + 1][2]usize{[_][2]usize{[_]usize{ 0, 0 }} ** (system.max_size_log2 + 1)} ** num_batches_cap;
    var top_size: u8 = 0;
    for (system.columns, 0..) |col, c| {
        const sz: u8 = switch (col.size) {
            .static => |s| s,
            .dynamic => |dyn| blk: {
                if (dyn.index >= module_sizes.len) return Error.MissingDynamicModuleSize;
                const n = module_sizes[dyn.index];
                if (n == 0 or (n & (n - 1)) != 0) return Error.NonPowerOfTwoModuleSize;
                const sz: u8 = @intCast(std.math.log2_int(usize, n));
                if (sz < dyn.min_size_log2) return Error.DynamicModuleSizeBelowMinimum;
                break :blk sz;
            },
        };
        if (sz > system.max_size_log2) return Error.LayoutOverflow;
        col_size_log2[c] = sz;
        if (sz > top_size) top_size = sz;

        if (col.batch_idx >= system.num_batches) return Error.LayoutOverflow;
        const ext_idx: usize = if (col.is_ext) 1 else 0;
        const count = &bucket_count[col.batch_idx][sz][ext_idx];
        col_position[c] = count.*;
        count.* += 1;
    }
    r.top_size = top_size;

    // Canonical enumeration: size DESC, batch ASC (0..num_batches), base rows
    // then ext rows, position ASC (declaration order within a bucket). We assign
    // entry_idx as a running counter and record each entry's origin column.
    // Positions restart at 0 per (batch, size_log2, is_ext); col_position[c] was
    // precomputed above in a single O(columns) pass — exactly prover GetLayout's
    // running counter, restricted to one batch.
    var entry_idx: usize = 0;
    var size: i32 = @intCast(system.max_size_log2);
    while (size >= 0) : (size -= 1) {
        const size_u8: u8 = @intCast(size);
        var batch: usize = 0;
        while (batch < system.num_batches) : (batch += 1) {
            // Base rows first, then ext rows, each in declaration order.
            inline for ([_]bool{ false, true }) |want_ext| {
                for (system.columns, 0..) |col, c| {
                    if (col.batch_idx != batch) continue;
                    if (col.is_ext != want_ext) continue;
                    if (col_size_log2[c] != size_u8) continue;

                    if (entry_idx >= system.max_entries) return Error.LayoutOverflow;
                    r.entry_size_log2[entry_idx] = size_u8;
                    r.entry_batch[entry_idx] = batch;
                    r.entry_is_ext[entry_idx] = want_ext;
                    r.entry_row_idx[entry_idx] = col_position[c];
                    r.entry_col_decl_idx[entry_idx] = c;
                    r.col_to_entry[c] = entry_idx;
                    entry_idx += 1;
                }
            }
        }
    }
    if (entry_idx != num_cols) return Error.LayoutOverflow;

    r.params = system.envelope_params.restrictTo(top_size) catch return Error.RestrictOutOfRange;
    return r;
}

/// Mirrors prover-ray's `inputOpeningRoots`: walk the canonical entry order,
/// take each batch's root the first time that batch appears, and deduplicate
/// equal roots by value so batches sharing a Merkle root consume one input-tree
/// opening.
pub fn routeInputRoots(
    comptime system: System,
    recon: Reconstructed(system),
    batch_roots: []const poseidon2.Digest,
) Error!InputRootRouting(system) {
    if (batch_roots.len != system.num_batches) return Error.RootCountMismatch;

    var routing = InputRootRouting(system){};
    for (0..recon.num_entries) |entry_idx| {
        const batch_idx = recon.entry_batch[entry_idx];
        const root = batch_roots[batch_idx];
        const branch_idx = findRootIndex(routing.roots[0..routing.distinct_count], root) orelse blk: {
            if (routing.distinct_count >= system.num_batches) return Error.RootCountMismatch;
            const next = routing.distinct_count;
            routing.roots[next] = root;
            routing.distinct_count = next + 1;
            break :blk next;
        };
        routing.index_by_batch[batch_idx] = branch_idx;
    }
    return routing;
}

fn findRootIndex(roots: []const poseidon2.Digest, want: poseidon2.Digest) ?usize {
    for (roots, 0..) |root, i| {
        if (std.meta.eql(root, want)) return i;
    }
    return null;
}

// =============================================================================
// Fiat-Shamir challenge derivation
// =============================================================================

/// The FS-derived PCS challenges consumed by `verify`. Buffers are sized by the
/// ENVELOPE maxima (never allocate); runtime lengths come from the restricted
/// params. `fold_alphas` has capacity `max_size_log2` (envelope num_rounds max,
/// with log_final_poly_size assumed 0).
pub fn PcsChallenges(comptime system: System) type {
    const max_rounds = comptime @as(usize, system.max_size_log2) - system.envelope_params.log_final_poly_size + 1;
    return struct {
        fold_alphas: [max_rounds]ext.Ext = undefined,
        deep_alpha: ext.Ext = ext.Ext.zero(),
        query_positions: [system.envelope_params.num_queries]usize = undefined,
        num_rounds: usize = 0,

        pub fn foldAlphas(self: *const @This()) []const ext.Ext {
            return self.fold_alphas[0..self.num_rounds];
        }
    };
}

/// Derives the FRI fold challenges and query positions by continuing
/// `transcript`. Uses the RESTRICTED params (from `recon`) for round/position
/// counts and the codeword size, so the challenges match a proof of THIS size.
pub fn deriveChallenges(
    comptime system: System,
    recon: Reconstructed(system),
    transcript: *fiat_shamir.Transcript,
    fri_proof: fri.Proof,
) fri.Error!PcsChallenges(system) {
    const params = recon.params;
    const num_rounds = params.numRoundsRuntime();
    const want_round_roots: usize = if (num_rounds > 0) @as(usize, num_rounds) - 1 else 0;
    if (fri_proof.round_roots.len != want_round_roots) return fri.Error.InvalidRoundRootCount;

    var challenges = PcsChallenges(system){};
    challenges.num_rounds = num_rounds;

    // One challenge per intermediate layer root, absorbing the root between
    // squeezes.
    if (num_rounds > 0) {
        for (fri_proof.round_roots, 0..) |root, i| {
            challenges.fold_alphas[i] = transcript.randomExt();
            transcript.updateElements(&root);
        }
    }

    // Final round's challenge: squeezed UNCONDITIONALLY (matches prover-ray),
    // including D=1 where the loop above is empty.
    const final_alpha = transcript.randomExt();
    challenges.deep_alpha = final_alpha;
    if (num_rounds > 0) challenges.fold_alphas[num_rounds - 1] = final_alpha;

    transcript.updateExt(fri_proof.final_poly);

    const codeword_size = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(params.log_codeword_size));
    transcript.randomManyIntegersRuntime(challenges.query_positions[0..params.num_queries], codeword_size);
    return challenges;
}

// =============================================================================
// Verify
// =============================================================================

pub fn verify(comptime system: System, input: VerifyInput) Error!void {
    const recon = try reconstruct(system, input.module_sizes);
    const routing = try routeInputRoots(system, recon, input.roots);
    const params = recon.params;
    const num_entries = recon.num_entries;
    const num_rounds = params.numRoundsRuntime();

    if (input.entry_claims.len != num_entries) return Error.ClaimedValueCountMismatch;

    // Each opened column owns exactly `shifts.len` claimed values. Guarded by a
    // comptime len check so a column-free System (columns == &.{}) doesn't force
    // Zig to analyze indexing into an empty slice.
    if (comptime system.columns.len > 0) {
        for (0..num_entries) |e| {
            const shifts = system.columns[recon.entry_col_decl_idx[e]].shifts;
            if (input.entry_claims[e].len != shifts.len) return Error.ClaimedValueCountMismatch;
        }
    }

    // zeta==0 is only unsafe when some column is opened at more than one shift
    // (the rotations collapse). This is a fixed property of `columns`.
    if (comptime systemHasMultiShiftEntry(system)) {
        if (input.zeta.isZero()) return Error.ZetaZeroWithMultipleShifts;
    }

    // zeta must not land in any bundle's domain.
    for (0..num_entries) |e| {
        const round = recon.top_size - recon.entry_size_log2[e];
        const domain_log_size = params.log_codeword_size - round;
        if (pointInDomain(input.zeta, domain_log_size)) return Error.ClaimPointOnDomain;
    }

    if (input.proof.input_queries.len != params.num_queries) return Error.InputQueryCountMismatch;
    if (input.query_positions.len < params.num_queries) return Error.QueryPositionCountMismatch;
    try fri.checkOpeningProofShape(params, input.proof.fri_proof, input.fold_alphas, input.query_positions[0..params.num_queries]);

    // Envelope-max-sized stack buffers; runtime lengths use the restricted
    // num_rounds. `max_size_log2` bounds the envelope num_rounds
    // (log_final_poly_size == 0).
    const cap_rounds = comptime @as(usize, system.max_size_log2) + 1;
    var rounds_buf: [system.envelope_params.num_queries][cap_rounds]fri.Pair = undefined;
    var aux_buf: [system.envelope_params.num_queries][cap_rounds + 1]?fri.Pair = undefined;
    var final_buf: [system.envelope_params.num_queries]ext.Ext = undefined;
    var resolved: [system.envelope_params.num_queries]fri.ResolvedQuery = undefined;

    for (0..params.num_queries) |query_idx| {
        for (aux_buf[query_idx][0 .. num_rounds + 1]) |*slot| slot.* = null;
        for (rounds_buf[query_idx][0..num_rounds]) |*pair| pair.* = .{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() };

        const query_position = input.query_positions[query_idx];
        const opening = input.proof.input_queries[query_idx];
        try authenticateInputQuery(params, opening, routing, query_position);

        if (num_rounds > 0) {
            const running_query = input.proof.fri_proof.running_queries[query_idx];
            try fri.resolveRunningLayers(params, input.proof.fri_proof.round_roots, running_query, query_position, rounds_buf[query_idx][0..num_rounds]);
        }

        const final_point = fri.domainPointExt(params.log_codeword_size - num_rounds, query_position >> @as(std.math.Log2Int(usize), @intCast(num_rounds)));
        final_buf[query_idx] = canonical.evaluateExtAtExt(input.proof.fri_proof.final_poly, final_point);

        // Walk entries grouped by bundle (contiguous same-size runs in canonical
        // order). For each distinct size we bind the input-tree openings once and
        // reconstruct the DEEP quotient over that bundle's entries. Guarded by a
        // comptime column-count check so a column-free System doesn't force
        // analysis of indexing into an empty `system.columns`.
        var e0: usize = 0;
        while (comptime system.columns.len > 0) {
            if (e0 >= num_entries) break;
            const size_log2 = recon.entry_size_log2[e0];
            var e1 = e0;
            while (e1 < num_entries and recon.entry_size_log2[e1] == size_log2) e1 += 1;

            const round = recon.top_size - size_log2;
            const domain_log_size = params.log_codeword_size - round;
            const level_size = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(domain_log_size));

            try bindInputTreeOpenings(system, recon, routing, e0, e1, opening, level_size);

            const alpha_deep: ext.Ext = if (round < num_rounds)
                input.fold_alphas[round].square()
            else if (num_rounds > 0)
                input.fold_alphas[num_rounds - 1]
            else
                input.deep_alpha;

            const level_pos = query_position >> @as(std.math.Log2Int(usize), @intCast(round));
            const seed = seedPair(rounds_buf[query_idx][0..num_rounds], round, num_rounds);

            const self_val = try reconstructQueryValueAt(
                system,
                recon,
                routing,
                e0,
                e1,
                size_log2,
                opening,
                level_size,
                input.entry_claims,
                input.zeta,
                alpha_deep,
                fri.domainPointExt(domain_log_size, level_pos),
                false,
                seed.self,
            );
            const sib_val = try reconstructQueryValueAt(
                system,
                recon,
                routing,
                e0,
                e1,
                size_log2,
                opening,
                level_size,
                input.entry_claims,
                input.zeta,
                alpha_deep,
                fri.domainPointExt(domain_log_size, level_pos ^ 1),
                true,
                seed.sibling,
            );
            aux_buf[query_idx][round] = .{ .self = self_val, .sibling = sib_val };

            e0 = e1;
        }

        if (num_rounds == 0) {
            const pair = aux_buf[query_idx][0] orelse return Error.MissingTopLevelAux;
            const sib_final = fri.domainPointExt(params.log_codeword_size, query_position ^ 1);
            const sib_final_value = canonical.evaluateExtAtExt(input.proof.fri_proof.final_poly, sib_final);
            if (!pair.self.eql(final_buf[query_idx])) return Error.BoundaryFinalSelfMismatch;
            if (!pair.sibling.eql(sib_final_value)) return Error.BoundaryFinalSiblingMismatch;
        }

        resolved[query_idx] = .{
            .rounds = rounds_buf[query_idx][0..num_rounds],
            .aux = aux_buf[query_idx][0 .. num_rounds + 1],
            .final = final_buf[query_idx],
        };
    }

    try fri.checkFolds(params, resolved[0..params.num_queries], input.fold_alphas, input.query_positions);
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

fn systemHasMultiShiftEntry(comptime system: System) bool {
    comptime {
        for (system.columns) |col| {
            if (col.shifts.len > 1) return true;
        }
        return false;
    }
}

/// Authenticates every distinct input tree once per query against its known
/// root: prover-ray's `authenticateInputQuery`.
fn authenticateInputQuery(
    params: fri.Params,
    opening: []const merkle.InputTreeOpening,
    routing: anytype,
    query_position: usize,
) Error!void {
    if (opening.len != routing.distinct_count) return Error.InputTreeCountMismatch;

    const codeword_size = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(params.log_codeword_size));
    for (opening, routing.distinctRoots()) |branch, root| {
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

/// Per-batch (base, ext) widths within the bundle spanning canonical entries
/// [e0, e1). Computed at runtime from the reconstructed arrays.
fn bundleBatchWidths(recon: anytype, e0: usize, e1: usize, batch_idx: usize) struct { base: usize, ext: usize } {
    var base: usize = 0;
    var ext_width: usize = 0;
    var e = e0;
    while (e < e1) : (e += 1) {
        if (recon.entry_batch[e] != batch_idx) continue;
        if (recon.entry_is_ext[e]) ext_width += 1 else base += 1;
    }
    return .{ .base = base, .ext = ext_width };
}

/// Validates that each batch present in the bundle carries a conjugate pair
/// matching its declared (base, ext) width at `level_size`. Mirrors
/// prover-ray's `bindInputTreeOpenings`.
fn bindInputTreeOpenings(
    comptime system: System,
    recon: anytype,
    routing: anytype,
    e0: usize,
    e1: usize,
    opening: []const merkle.InputTreeOpening,
    level_size: usize,
) Error!void {
    // Distinct batches within the bundle, in first-declaration (entry) order.
    // At most system.num_batches distinct batches can ever appear.
    var seen: [@max(system.num_batches, 1)]usize = undefined;
    var count: usize = 0;
    var e = e0;
    outer: while (e < e1) : (e += 1) {
        const b = recon.entry_batch[e];
        for (seen[0..count]) |s| {
            if (s == b) continue :outer;
        }
        seen[count] = b;
        count += 1;

        const widths = bundleBatchWidths(recon, e0, e1, b);
        const branch_idx = routing.index_by_batch[b];
        const pair = try opening[branch_idx].pairAtLevel(level_size);
        if (pair[0].base.len != widths.base or pair[0].ext.len != widths.ext) return Error.RowShapeMismatch;
        if (pair[1].base.len != widths.base or pair[1].ext.len != widths.ext) return Error.ConjugateRowShapeMismatch;
    }
}

/// Combines the bundle's columns [e0, e1) with `running` at `x`, the same way
/// prover-ray's `Level.EvalsAt` does. Entries are walked highest-alphaDeep-power
/// first, which canonicalLayout's assignment makes simply reverse entry order.
/// Mirrors prover-ray's `reconstructQueryValueAt`.
fn reconstructQueryValueAt(
    comptime system: System,
    recon: anytype,
    routing: anytype,
    e0: usize,
    e1: usize,
    size_log2: u8,
    opening: []const merkle.InputTreeOpening,
    level_size: usize,
    entry_claims: []const []const ext.Ext,
    zeta: ext.Ext,
    alpha_deep: ext.Ext,
    x: ext.Ext,
    sibling: bool,
    running: ext.Ext,
) Error!ext.Ext {
    var value = running;
    var i = e1;
    while (i > e0) {
        i -= 1;
        const batch_idx = recon.entry_batch[i];
        const branch_idx = routing.index_by_batch[batch_idx];
        const pair = try opening[branch_idx].pairAtLevel(level_size);
        const row = if (sibling) pair[1] else pair[0];
        const row_idx = recon.entry_row_idx[i];
        const entry_value: ext.Ext = if (recon.entry_is_ext[i]) row.ext[row_idx] else ext.Ext.lift(row.base[row_idx]);

        const shifts = system.columns[recon.entry_col_decl_idx[i]].shifts;
        var term = ext.Ext.zero();
        for (shifts, 0..) |shift, k| {
            const point = shiftedPoint(size_log2, shift, zeta);
            const denom = x.sub(point);
            if (denom.isZero()) return Error.ClaimPointOnQueryPoint;
            const numerator = entry_value.sub(entry_claims[i][k]);
            term = term.add(numerator.mul(denom.inverse()));
        }
        value = value.mul(alpha_deep).add(term);
    }
    return value;
}

/// zeta * omega_N^(offset mod N), omega_N the generator of the size-2^size_log2
/// domain, N = 2^size_log2: prover-ray's `shiftedPoint` after normalizing the raw
/// offset mod the RUNTIME size (matching prover-ray's
/// `((offset % size) + size) % size`). omega_N^(offset mod N) == omega_N^offset,
/// so the reconstructed point equals the prover's regardless of normalization —
/// but the exponent must be reduced with the runtime N, which is exactly why the
/// raw offset (not a size-frozen normalization) is stored.
fn shiftedPoint(size_log2: u8, offset: isize, zeta: ext.Ext) ext.Ext {
    const order = @as(usize, 1) << @as(std.math.Log2Int(usize), @intCast(size_log2));
    const n: isize = @intCast(order);
    const shift: usize = @intCast(@mod(offset, n)); // @mod is always in [0, n)
    const base = field.rootOfUnityBy(order) catch unreachable;
    const rotation = base.pow(@as(u64, @intCast(shift)));
    return zeta.mulByBase(rotation);
}

/// Whether `point` lands in the size-2^log_size multiplicative subgroup.
/// Every domain point is a base-field root of unity, so an extension-valued
/// point that isn't itself a lifted base element can never coincide with one.
/// Mirrors prover-ray's `pointInDomain`. `log_size` is a RUNTIME value here
/// (derived from the reconstructed layout, not the comptime System), so the
/// exponentiation uses `pow`, not `powComptime`.
fn pointInDomain(point: ext.Ext, log_size: u8) bool {
    if (!point.isBase()) return false;
    const order: u64 = @as(u64, 1) << @as(std.math.Log2Int(u64), @intCast(log_size));
    const powered = point.B0.a0.pow(order);
    return powered.eql(field.Element.one());
}
