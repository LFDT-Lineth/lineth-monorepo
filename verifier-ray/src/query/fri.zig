const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const poseidon2 = @import("../crypto/poseidon2.zig");
const merkle = @import("../crypto/merkle.zig");

/// The low-degree-test core: the pure FRI fold recurrence plus running-layer
/// Merkle authentication, ported from prover-ray's `checkOpeningProofShape`,
/// the running-layer loop inside `pcs.Verify`, and `checkFolds`.
///
/// This module never sees a claim, a batching weight, or a committed row: it
/// consumes already-authenticated-and-reconstructed `ResolvedQuery` records.
/// The PCS/DEEP layer (input-tree authentication and the per-level Horner
/// reconstruction) supplies those records; see `query/pcs.zig`.
pub const Error = merkle.Error || error{
    InvalidRoundRootCount,
    InvalidRunningQueryCount,
    InvalidFinalPolyLength,
    InsufficientFoldAlphas,
    InsufficientPositions,
    PositionOutOfRange,
    InvalidRunningLayerShape,
    InvalidResolvedQueryCount,
    MerkleProofInvalid,
    NonCanonicalLeaf,
    FoldMismatch,
    FinalPolyMismatch,
    BoundaryAuxNotConstant,
};

/// FRI configuration shared by every check in this module. Comptime, since it
/// is fixed at protocol-compile time and shared with the prover. Mirrors
/// prover-ray's `fri.Params`, restricted to what the low-degree-test core
/// needs; the DEEP/PCS layer carries its own encoder schedule on top.
pub const Params = struct {
    /// log2 of the codeword domain size (`fri.Params.LogCodewordSize`).
    log_codeword_size: u8,
    /// log2 of the plaintext polynomial size (`fri.Params.LogPlainTextSize`).
    log_plaintext_size: u8,
    /// log2 of the number of coefficients the final polynomial reveals
    /// (`fri.Params.logFinalPolySize`, private in Go); this length is the
    /// enforced low-degree bound on the final layer.
    log_final_poly_size: u8 = 0,
    /// Number of independent FRI queries.
    num_queries: usize,

    /// Number of folding rounds: `fri.Params.numRounds()`.
    pub fn numRounds(comptime self: Params) u8 {
        comptime {
            if (self.log_final_poly_size > self.log_plaintext_size) {
                @compileError("fri: log_final_poly_size exceeds log_plaintext_size");
            }
            if (self.log_plaintext_size >= self.log_codeword_size) {
                @compileError("fri: log_plaintext_size must be less than log_codeword_size");
            }
            if (self.num_queries == 0) {
                @compileError("fri: num_queries must be nonzero, or every proof verifies");
            }
        }
        return self.log_plaintext_size - self.log_final_poly_size;
    }
};

/// One fold round's conjugate pair (self, sibling): prover-ray's `inputPair`.
pub const Pair = struct {
    self: ext.Ext,
    sibling: ext.Ext,
};

/// One query's fully authenticated and reconstructed fold inputs: prover-ray's
/// `resolvedQuery`. `rounds[0]` is never read: round 0 always introduces the
/// top level (see `NewProverState`'s "there being no round -1 to fold a real
/// codeword from"), so `aux[0]` is always present and supersedes it.
pub const ResolvedQuery = struct {
    /// rounds[j] = the running codeword's (self, sibling) pair authenticated
    /// at round j; filled by `resolveRunningLayers` for j in [1, num_rounds).
    /// Length num_rounds, checked by `checkFolds`.
    rounds: []const Pair,
    /// aux[j] = the level pair introduced at round j (reconstructed by the
    /// DEEP/PCS layer), if any; when present it supersedes rounds[j]. Length
    /// num_rounds + 1: a level may be introduced at the boundary round
    /// (index num_rounds, one past the last fold), authenticated but never
    /// folded -- see `checkFolds`, which also checks this length.
    aux: []const ?Pair,
    /// The final polynomial evaluated at this query's final-domain point.
    final: ext.Ext,
};

/// The running-layer proof data `checkOpeningProofShape` / `checkFolds`
/// consume: prover-ray's `fri.Proof`, restricted to the running-layer path.
/// The input-tree openings live in the PCS `OpeningProof`, not here.
pub const Proof = struct {
    /// Merkle roots for the running polynomial's committed layers
    /// T_1..T_{num_rounds-1}; length `num_rounds - 1`.
    round_roots: []const poseidon2.Digest,
    /// The final polynomial's revealed coefficients; length
    /// `1 << log_final_poly_size`.
    final_poly: []const ext.Ext,
    /// running_queries[k][j-1] is query k's Merkle branch authenticating
    /// round j's leaf/conjugate pair, for j in [1, num_rounds).
    running_queries: []const []const merkle.Branch,
};

/// Validates `proof`'s structure against `params` and the challenge/position
/// counts before any authentication or reconstruction runs, so a malformed
/// proof can never cause an out-of-bounds access later. Mirrors prover-ray's
/// `checkOpeningProofShape`.
///
/// `fold_alphas` is read only for its length here — checking that at least
/// `num_rounds` challenges are available — never for arithmetic; the fold
/// challenges themselves are consumed later, by `checkFolds`.
pub fn checkOpeningProofShape(
    comptime params: Params,
    proof: Proof,
    fold_alphas: []const ext.Ext,
    positions: []const usize,
) Error!void {
    const num_rounds = params.numRounds();
    const want_round_roots: u8 = if (num_rounds > 0) num_rounds - 1 else 0;
    if (proof.round_roots.len != want_round_roots) return Error.InvalidRoundRootCount;
    if (proof.running_queries.len != params.num_queries) return Error.InvalidRunningQueryCount;
    if (proof.final_poly.len != (@as(usize, 1) << params.log_final_poly_size)) return Error.InvalidFinalPolyLength;
    if (fold_alphas.len < num_rounds) return Error.InsufficientFoldAlphas;
    if (positions.len < params.num_queries) return Error.InsufficientPositions;

    const codeword_size = @as(usize, 1) << params.log_codeword_size;
    for (proof.running_queries, positions[0..params.num_queries]) |query_branches, position| {
        if (query_branches.len != want_round_roots) return Error.InvalidRunningQueryCount;
        if (position >= codeword_size) return Error.PositionOutOfRange;
    }
}

/// Authenticates one query's running-layer branches against `round_roots` and
/// decodes each round's leaf/deepest-sibling pair into `rounds[1..num_rounds)`.
/// `rounds[0]` is left untouched: round 0 always introduces the top level (see
/// `ResolvedQuery`), so its running pair is never read. Mirrors the
/// running-layer loop inside prover-ray's `pcs.Verify`.
///
/// Validates every input length itself, since `rounds` is a caller-allocated
/// output buffer `checkOpeningProofShape` never sees.
pub fn resolveRunningLayers(
    comptime params: Params,
    round_roots: []const poseidon2.Digest,
    query_branches: []const merkle.Branch,
    position: usize,
    rounds: []Pair,
) Error!void {
    const num_rounds = params.numRounds();
    if (num_rounds == 0) return; // no running layers at all (D=1)
    if (rounds.len != num_rounds) return Error.InvalidRunningLayerShape;
    if (query_branches.len != num_rounds - 1) return Error.InvalidRunningLayerShape;
    if (round_roots.len != num_rounds - 1) return Error.InvalidRunningLayerShape;

    for (1..@as(usize, num_rounds)) |j| {
        const branch = query_branches[j - 1];
        const want_siblings = params.log_codeword_size - j;
        if (branch.siblings.len != want_siblings) return Error.InvalidRunningLayerShape;

        const recovered = try branch.recoverRoot(position >> @as(u6, @intCast(j)));
        if (!poseidon2.eql(recovered, round_roots[j - 1])) return Error.MerkleProofInvalid;

        rounds[j] = .{
            .self = try octupletToExt(branch.leaf),
            .sibling = try octupletToExt(branch.siblings[branch.siblings.len - 1]),
        };
    }
}

/// Verifies the FRI fold recurrence for every query against values the caller
/// has already authenticated and reconstructed: pure arithmetic, no Merkle
/// proof or row ever passes through it. Mirrors prover-ray's `checkFolds`,
/// including the boundary-round check (a level introduced past the last fold
/// round must have a constant DEEP quotient).
///
/// At each round j, the fold point's inverse is squared from the previous
/// round's rather than recomputed from scratch: x_{j+1} = x_j^2 under the
/// domain-halving map, so 1/x_{j+1} = (1/x_j)^2. Only round 0's inverse is
/// computed directly, from the query position's (bit-reversed) domain point.
pub fn checkFolds(
    comptime params: Params,
    resolved: []const ResolvedQuery,
    fold_alphas: []const ext.Ext,
    positions: []const usize,
) Error!void {
    const num_rounds = params.numRounds();
    if (resolved.len != params.num_queries) return Error.InvalidResolvedQueryCount;
    if (fold_alphas.len < num_rounds) return Error.InsufficientFoldAlphas;
    if (positions.len < resolved.len) return Error.InsufficientPositions;

    const generator = comptime fullDomainGenerator(params);
    for (resolved, positions[0..resolved.len]) |rq, s| {
        if (rq.rounds.len != num_rounds) return Error.InvalidRunningLayerShape;
        if (rq.aux.len != @as(usize, num_rounds) + 1) return Error.InvalidRunningLayerShape;

        var x_inv = domainPoint(params.log_codeword_size, generator, s).inverse();

        for (0..num_rounds) |j| {
            var pair = rq.rounds[j];
            if (rq.aux[j]) |level_pair| pair = level_pair;

            // fold: (self + sib)/2 + alpha * (self - sib)/x, halved once at
            // the end rather than twice (see prover-ray's checkFolds).
            var sum = pair.self.add(pair.sibling);
            var diff = pair.self.sub(pair.sibling);
            diff = diff.mulByBase(x_inv);
            diff = diff.mul(fold_alphas[j]);
            sum = sum.add(diff);
            sum = sum.mulByBase(inv_two);

            if (j < num_rounds - 1) {
                if (!sum.eql(rq.rounds[j + 1].self)) return Error.FoldMismatch;
            } else if (!sum.eql(rq.final)) return Error.FinalPolyMismatch;

            x_inv = x_inv.square();
        }

        // A level introduced at the boundary round (index num_rounds) is
        // authenticated but never folded: its batched DEEP quotient must
        // evaluate identically at both conjugate positions. The num_rounds
        // == 0 case (no rounds at all) is handled entirely in query/pcs.zig,
        // against the final polynomial directly rather than this check.
        if (num_rounds > 0) {
            if (rq.aux[num_rounds]) |pair| {
                if (!pair.self.eql(pair.sibling)) return Error.BoundaryAuxNotConstant;
            }
        }
    }
}

// 2^{-1} mod p (p = 2_130_706_433 = 2^31 - 2^24 + 1). Duplicated from
// crypto/poseidon2.zig's inv2Exp1, which is private to that module.
const inv_two: field.Element = .{ .value = 1_065_353_217 };

fn fullDomainGenerator(comptime params: Params) field.Element {
    return field.rootOfUnityBy(@as(usize, 1) << params.log_codeword_size) catch
        @compileError("fri: log_codeword_size exceeds the supported KoalaBear root-of-unity order");
}

/// x = g^{bitrev_{log_size}(position)}, where g generates the size-2^log_size
/// subgroup. Matches prover-ray's `domainPoint`: the codeword is stored
/// bit-reversed so that FRI conjugate pairs land at adjacent positions.
/// `log_size` is comptime at this function's only call site (derived from
/// `params.log_codeword_size`), which is what lets `bitReverse` fold its
/// shift amount at compile time below.
fn domainPoint(comptime log_size: u8, generator: field.Element, position: usize) field.Element {
    return generator.pow(@as(u64, bitReverse(position, log_size)));
}

fn bitReverse(value: usize, comptime width: u8) usize {
    if (width == 0) return 0;
    // field.rootOfUnityBy bounds every domain log-size to <= max_order_root
    // (24), so the low `width` bits of `value` always fit in a u32 with room
    // to spare.
    const v: u32 = @intCast(value);
    const reversed: u32 = @bitReverse(v);
    return reversed >> (32 - width);
}

/// Converts a Poseidon2 digest into an extension element. Expects
/// coordinates 6 and 7 to be zero, matching prover-ray's `octupletToExt`.
fn octupletToExt(digest: poseidon2.Digest) Error!ext.Ext {
    if (!digest[6].isZero() or !digest[7].isZero()) return Error.NonCanonicalLeaf;
    return .{
        .B0 = .{ .a0 = digest[0], .a1 = digest[1] },
        .B1 = .{ .a0 = digest[2], .a1 = digest[3] },
        .B2 = .{ .a0 = digest[4], .a1 = digest[5] },
    };
}
