const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixtures = @import("test_pcs_vectors");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const merkle = verifier_ray.crypto.merkle;
const pcs = verifier_ray.query.pcs;
const fri = verifier_ray.query.fri;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;

// Test vectors generated from prover-ray's real PCS.Commit/AddOpening/
// NewProverState/Open/Verify pipeline, called through prover-ray's exported
// surface only (see testdata/generate/main.go's PCS section): the claimed
// values are computed independently (not via the unexported
// pcs.shiftedPoint), and every honest case is self-checked against
// prover-ray's own exported pcs.Verify before being emitted here. Regenerate
// via `make generate-testdata`.

fn toDigest(o: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, o) |*dst, v| dst.* = field.Element.init(v);
    return out;
}

fn toDigests(allocator: std.mem.Allocator, os: []const [8]u32) ![]poseidon2.Digest {
    const out = try allocator.alloc(poseidon2.Digest, os.len);
    for (out, os) |*dst, o| dst.* = toDigest(o);
    return out;
}

fn toExt(e: [6]u32) ext.Ext {
    return .{
        .B0 = .{ .a0 = field.Element.init(e[0]), .a1 = field.Element.init(e[1]) },
        .B1 = .{ .a0 = field.Element.init(e[2]), .a1 = field.Element.init(e[3]) },
        .B2 = .{ .a0 = field.Element.init(e[4]), .a1 = field.Element.init(e[5]) },
    };
}

fn toExts(allocator: std.mem.Allocator, es: []const [6]u32) ![]ext.Ext {
    const out = try allocator.alloc(ext.Ext, es.len);
    for (out, es) |*dst, e| dst.* = toExt(e);
    return out;
}

fn toExtsJagged(allocator: std.mem.Allocator, rows: []const []const [6]u32) ![]const []const ext.Ext {
    const out = try allocator.alloc([]const ext.Ext, rows.len);
    for (out, rows) |*dst, row| dst.* = try toExts(allocator, row);
    return out;
}

fn toRowOpening(allocator: std.mem.Allocator, r: fixtures.RowOpeningData) !merkle.RowOpening {
    const base = try allocator.alloc(field.Element, r.base.len);
    for (base, r.base) |*dst, v| dst.* = field.Element.init(v);
    return .{ .base = base, .ext = try toExts(allocator, r.ext) };
}

fn toRowPair(allocator: std.mem.Allocator, p: fixtures.RowPairData) !merkle.RowPair {
    return .{ try toRowOpening(allocator, p[0]), try toRowOpening(allocator, p[1]) };
}

fn toInputTreeOpening(allocator: std.mem.Allocator, o: fixtures.InputTreeOpeningData) !merkle.InputTreeOpening {
    const leaves = try allocator.alloc(?merkle.RowPair, o.leaves.len);
    for (leaves, o.leaves) |*dst, l| dst.* = if (l) |v| try toRowPair(allocator, v) else null;
    return .{ .siblings = try toDigests(allocator, o.siblings), .leaves = leaves };
}

fn toInputQueries(allocator: std.mem.Allocator, qs: []const []const fixtures.InputTreeOpeningData) ![]const []const merkle.InputTreeOpening {
    const out = try allocator.alloc([]const merkle.InputTreeOpening, qs.len);
    for (out, qs) |*dst, q| {
        const trees = try allocator.alloc(merkle.InputTreeOpening, q.len);
        for (trees, q) |*t, jo| t.* = try toInputTreeOpening(allocator, jo);
        dst.* = trees;
    }
    return out;
}

fn toBranch(allocator: std.mem.Allocator, b: fixtures.BranchData) !merkle.Branch {
    return .{ .leaf = toDigest(b.leaf), .siblings = try toDigests(allocator, b.siblings) };
}

fn toRunningQueries(allocator: std.mem.Allocator, qs: []const []const fixtures.BranchData) ![]const []const merkle.Branch {
    const out = try allocator.alloc([]const merkle.Branch, qs.len);
    for (out, qs) |*dst, q| {
        const branches = try allocator.alloc(merkle.Branch, q.len);
        for (branches, q) |*b, jb| b.* = try toBranch(allocator, jb);
        dst.* = branches;
    }
    return out;
}

fn mapPcsError(name: []const u8) pcs.Error {
    if (std.mem.eql(u8, name, "BoundaryAuxNotConstant")) return error.BoundaryAuxNotConstant;
    if (std.mem.eql(u8, name, "BoundaryFinalSelfMismatch")) return error.BoundaryFinalSelfMismatch;
    if (std.mem.eql(u8, name, "BoundaryFinalSiblingMismatch")) return error.BoundaryFinalSiblingMismatch;
    if (std.mem.eql(u8, name, "ClaimPointOnDomain")) return error.ClaimPointOnDomain;
    if (std.mem.eql(u8, name, "ClaimPointOnQueryPoint")) return error.ClaimPointOnQueryPoint;
    if (std.mem.eql(u8, name, "MerkleProofInvalid")) return error.MerkleProofInvalid;
    if (std.mem.eql(u8, name, "RowShapeMismatch")) return error.RowShapeMismatch;
    if (std.mem.eql(u8, name, "ConjugateRowShapeMismatch")) return error.ConjugateRowShapeMismatch;
    if (std.mem.eql(u8, name, "FoldMismatch")) return error.FoldMismatch;
    if (std.mem.eql(u8, name, "FinalPolyMismatch")) return error.FinalPolyMismatch;
    std.debug.panic("pcs_test: unrecognized expected error name '{s}'", .{name});
}

// system is a separate comptime parameter (not read off a plain-parameter
// `case`) because pcs.verify requires a comptime System: the enclosing
// `inline for` makes `case.system` comptime-known at the call site, but a
// regular function parameter would lose that. Mirrors vanishing_test.zig's
// own pattern of calling verify() directly against a comptime-extracted
// `system`/`spec` rather than through a plain-parameter helper.
fn runPCSCase(allocator: std.mem.Allocator, comptime system: pcs.System, case: fixtures.PcsCase) !void {
    const input = pcs.VerifyInput{
        .roots = try toDigests(allocator, case.roots),
        .entry_claims = try toExtsJagged(allocator, case.entry_claims),
        .zeta = toExt(case.zeta),
        .fold_alphas = try toExts(allocator, case.fold_alphas),
        .query_positions = try allocator.dupe(usize, case.query_positions),
        .proof = .{
            .input_queries = try toInputQueries(allocator, case.proof.input_queries),
            .fri_proof = .{
                .round_roots = try toDigests(allocator, case.proof.fri_proof.round_roots),
                .final_poly = try toExts(allocator, case.proof.fri_proof.final_poly),
                .running_queries = try toRunningQueries(allocator, case.proof.fri_proof.running_queries),
            },
        },
    };

    const result = pcs.verify(system, input);
    if (case.expect_verify_error.len > 0) {
        try std.testing.expectError(mapPcsError(case.expect_verify_error), result);
        return;
    }
    try result;
}

test "pcs verify cases from prover-ray vectors" {
    try std.testing.expect(fixtures.pcs_cases.len > 0);
    inline for (fixtures.pcs_cases) |case| {
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        runPCSCase(arena.allocator(), case.system, case) catch |err| {
            std.debug.print("pcs case '{s}' failed: {}\n", .{ case.name, err });
            return err;
        };
    }
}

// ── PCS challenge derivation ──────────────────────────────────────────────────
//
// `pcs.deriveChallenges` squeezes the FRI fold challenges + query positions from
// a caller-owned transcript. There is no golden vector for these (the pcs.zig
// fixtures carry synthetic challenges, not transcript-derived ones), so these
// tests pin the properties that must hold regardless of the exact values:
// correct shape, determinism, and sensitivity to the absorbed transcript state.

// numRounds = log_plaintext_size - log_final_poly_size = 2, so fold_alphas has
// length 2 and the FRI proof carries numRounds-1 = 1 running-layer root.
const challenge_system = pcs.System{
    .params = .{ .log_codeword_size = 4, .log_plaintext_size = 2, .num_queries = 3 },
    .layout = &.{},
    .num_batches = 0,
};

fn challengeDigest(seed: u32) poseidon2.Digest {
    var d: poseidon2.Digest = undefined;
    for (&d, 0..) |*limb, i| limb.* = field.Element.init(seed +% @as(u32, @intCast(i)));
    return d;
}

// A well-shaped FRI proof for `challenge_system`: exactly num_rounds-1 == 1
// running-layer root.
fn challengeFriProof(root_seed: u32) fri.Proof {
    const S = struct {
        var round_roots: [1]poseidon2.Digest = undefined;
        var final_poly = [_]ext.Ext{ext.Ext.zero()};
    };
    S.round_roots[0] = challengeDigest(root_seed);
    return .{ .round_roots = &S.round_roots, .final_poly = &S.final_poly, .running_queries = &.{} };
}

test "deriveChallenges produces the comptime-sized shape" {
    var transcript = fiat_shamir.Transcript.init();
    const challenges = try pcs.deriveChallenges(challenge_system, &transcript, challengeFriProof(1));
    try std.testing.expectEqual(@as(usize, 2), challenges.fold_alphas.len);
    try std.testing.expectEqual(@as(usize, 3), challenges.query_positions.len);
    // Query positions are reduced into the codeword domain (2^4 = 16).
    for (challenges.query_positions) |p| try std.testing.expect(p < 16);
}

test "deriveChallenges is deterministic for the same transcript and proof" {
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    const a = try pcs.deriveChallenges(challenge_system, &t1, challengeFriProof(7));
    const b = try pcs.deriveChallenges(challenge_system, &t2, challengeFriProof(7));
    for (a.fold_alphas, b.fold_alphas) |x, y| try std.testing.expect(x.eql(y));
    try std.testing.expectEqualSlices(usize, &a.query_positions, &b.query_positions);
}

test "deriveChallenges depends on the absorbed transcript state" {
    // Two transcripts diverging before derivation must yield different
    // challenges: they are a function of the live Fiat-Shamir state, not just
    // the proof. (Absorb one differing element up front.)
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    t1.updateExt(&.{ext.Ext.fromUints(.{ 1, 0, 0, 0, 0, 0 })});
    t2.updateExt(&.{ext.Ext.fromUints(.{ 2, 0, 0, 0, 0, 0 })});
    const a = try pcs.deriveChallenges(challenge_system, &t1, challengeFriProof(9));
    const b = try pcs.deriveChallenges(challenge_system, &t2, challengeFriProof(9));
    var any_alpha_differs = false;
    for (a.fold_alphas, b.fold_alphas) |x, y| {
        if (!x.eql(y)) any_alpha_differs = true;
    }
    try std.testing.expect(any_alpha_differs);
}
