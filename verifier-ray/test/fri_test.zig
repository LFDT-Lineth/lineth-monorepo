const std = @import("std");
const verifier_ray = @import("verifier_ray");
const merkle_fixtures = @import("test_fri_vectors");
const fold_fixtures = @import("fri_fold_cases.zig");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const merkle = verifier_ray.crypto.merkle;
const fri = verifier_ray.query.fri;

// Merkle cases are generated live from prover-ray's exported Tree/Branch API
// (see testdata/generate/fri/main.go); regenerate via `make generate-testdata`.
//
// Fold cases are FROZEN (test/fri_fold_cases.zig), not generated: prover-ray's
// FRI internals (Level/quotientColumn/EvalsAt) changed shape in a way that
// broke the previous hand-built-Level generator, and none of the exported
// surface can currently rebuild an equivalent "pure fold, zero DEEP quotient"
// scenario. The two honest cases here are a mechanical (scripted) conversion
// of the last-known-good generated vectors; the three rejection cases are
// derived from them at test time by corrupting one field, mirroring exactly
// what the old generator did (corruptRunningSibling/corruptAux/corruptFinal
// in the now-deleted prover-ray/crypto/koalabear/fri/vectors_gen_test.go).
//
// This means these two cases catch Zig regressions but not prover-ray
// drift -- see the file header of fri_fold_cases.zig. Follow-up: restore
// real generation once quotientColumn/Level's construction is exported.

// ─── crypto.merkle: vectors from prover-ray's exported Tree API ────────────

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

fn runMerkleCase(allocator: std.mem.Allocator, case: merkle_fixtures.MerkleCase) !void {
    const branch = merkle.Branch{
        .leaf = toDigest(case.leaf),
        .siblings = try toDigests(allocator, case.siblings),
    };
    const recovered = try branch.recoverRoot(case.index);
    const matches = poseidon2.eql(recovered, toDigest(case.root));
    try std.testing.expectEqual(case.expect_match, matches);
}

test "merkle branches from prover-ray vectors" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    try std.testing.expect(merkle_fixtures.merkle_cases.len > 0);
    for (merkle_fixtures.merkle_cases) |case| {
        runMerkleCase(allocator, case) catch |err| {
            std.debug.print("merkle case '{s}' failed: {}\n", .{ case.name, err });
            return err;
        };
    }
}

test "merkle branch with no siblings is rejected before any hashing" {
    // A pure shape check: no tree needed, so hand-written rather than
    // generated (unlike the other merkle cases, which come from a real tree).
    const branch = merkle.Branch{ .leaf = poseidon2.zeroDigest(), .siblings = &.{} };
    try std.testing.expectError(error.EmptyBranch, branch.recoverRoot(0));
}

// ─── query.fri: frozen vectors from a real multi-round, multi-level proof ──

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

fn toPair(p: fold_fixtures.RawPair) fri.Pair {
    return .{ .self = toExt(p.self), .sibling = toExt(p.sibling) };
}

fn toOptPairs(allocator: std.mem.Allocator, ps: []const ?fold_fixtures.RawPair) ![]?fri.Pair {
    const out = try allocator.alloc(?fri.Pair, ps.len);
    for (out, ps) |*dst, p| dst.* = if (p) |v| toPair(v) else null;
    return out;
}

fn toBranch(allocator: std.mem.Allocator, b: fold_fixtures.RawBranch) !merkle.Branch {
    return .{ .leaf = toDigest(b.leaf), .siblings = try toDigests(allocator, b.siblings) };
}

fn toBranches(allocator: std.mem.Allocator, bs: []const fold_fixtures.RawBranch) ![]merkle.Branch {
    const out = try allocator.alloc(merkle.Branch, bs.len);
    for (out, bs) |*dst, b| dst.* = try toBranch(allocator, b);
    return out;
}

// wrongExt/wrongDigest stand in for "some value that must not match the
// honest one"; the exact value is immaterial as long as it differs (every
// honest value corrupted below happens to be all-zero).
fn wrongExt() ext.Ext {
    return ext.Ext.lift(field.Element.init(999_999));
}

fn wrongDigest() poseidon2.Digest {
    var out = poseidon2.zeroDigest();
    out[0] = field.Element.init(999_999);
    return out;
}

const Corruption = enum { none, running_sibling, aux, final };

fn runFoldCase(allocator: std.mem.Allocator, comptime params: fri.Params, case: fold_fixtures.FoldCase, corrupt: Corruption) !void {
    const fold_alphas = try toExts(allocator, case.fold_alphas);
    const round_roots = try toDigests(allocator, case.round_roots);
    const final_poly = try toExts(allocator, case.final_poly);
    const running_branches = try toBranches(allocator, case.running_branches);
    const positions = try allocator.dupe(usize, &.{case.position});

    if (corrupt == .running_sibling) {
        // running_branches[0].siblings is []const Digest (Branch's own field
        // type), so mutating it in place isn't legal; duplicate first.
        const siblings = try allocator.dupe(poseidon2.Digest, running_branches[0].siblings);
        siblings[siblings.len - 1] = wrongDigest();
        running_branches[0] = .{ .leaf = running_branches[0].leaf, .siblings = siblings };
    }
    if (corrupt == .final) {
        final_poly[0] = wrongExt();
    }

    const proof = fri.Proof{
        .round_roots = round_roots,
        .final_poly = final_poly,
        .running_queries = &.{running_branches},
    };

    try fri.checkOpeningProofShape(params, proof, fold_alphas, positions);

    // rounds[0] is never written (round 0 always introduces a level), so
    // initialize it rather than leaving allocator garbage.
    const rounds = try allocator.alloc(fri.Pair, params.numRounds());
    @memset(rounds, .{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() });
    const running_result = fri.resolveRunningLayers(params, round_roots, running_branches, case.position, rounds);

    if (corrupt == .running_sibling) {
        try std.testing.expectError(error.MerkleProofInvalid, running_result);
        return;
    }
    try running_result;

    const want_rounds = case.expected_rounds;
    try std.testing.expectEqual(rounds.len, want_rounds.len);
    for (rounds[1..], want_rounds[1..]) |got, want| {
        const want_pair = toPair(want);
        try std.testing.expect(got.self.eql(want_pair.self));
        try std.testing.expect(got.sibling.eql(want_pair.sibling));
    }

    const aux = try toOptPairs(allocator, case.aux);
    if (corrupt == .aux) {
        aux[0].?.self = wrongExt();
    }

    const resolved = [_]fri.ResolvedQuery{.{ .rounds = rounds, .aux = aux, .final = final_poly[0] }};
    const fold_result = fri.checkFolds(params, &resolved, fold_alphas, positions);

    switch (corrupt) {
        .aux => try std.testing.expectError(error.FoldMismatch, fold_result),
        .final => try std.testing.expectError(error.FinalPolyMismatch, fold_result),
        .running_sibling => unreachable, // returned above
        .none => try fold_result,
    }
}

test "fri fold cases from frozen prover-ray vectors" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    try std.testing.expect(fold_fixtures.fold_cases.len > 0);
    inline for (fold_fixtures.fold_cases) |case| {
        runFoldCase(allocator, case.params, case, .none) catch |err| {
            std.debug.print("fold case '{s}' failed: {}\n", .{ case.name, err });
            return err;
        };
    }
}

// The three rejection paths, derived by corrupting the first honest case
// (mirrors the deleted generator's corruptRunningSibling/corruptAux/
// corruptFinal, which only ever derived from "single_level_3rounds").
test "fri fold cases reject a corrupted running sibling, aux, or final poly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const base = fold_fixtures.fold_cases[0];
    inline for (.{ Corruption.running_sibling, Corruption.aux, Corruption.final }) |corrupt| {
        runFoldCase(allocator, base.params, base, corrupt) catch |err| {
            std.debug.print("corrupted case '{s}' failed: {}\n", .{ @tagName(corrupt), err });
            return err;
        };
    }
}

test "resolveRunningLayers and checkFolds reject undersized buffers" {
    const params = fold_fixtures.fold_cases[0].params;
    var rounds: [1]fri.Pair = undefined;
    const branches: [0]merkle.Branch = .{};
    const roots: [0]poseidon2.Digest = .{};
    try std.testing.expectError(
        error.InvalidRunningLayerShape,
        fri.resolveRunningLayers(params, &roots, &branches, 0, &rounds),
    );

    var aux: [1]?fri.Pair = .{null};
    const resolved = [_]fri.ResolvedQuery{.{ .rounds = &rounds, .aux = &aux, .final = ext.Ext.zero() }};
    const fold_alphas: [3]ext.Ext = .{ ext.Ext.zero(), ext.Ext.zero(), ext.Ext.zero() };
    const positions: [1]usize = .{0};
    try std.testing.expectError(
        error.InvalidRunningLayerShape,
        fri.checkFolds(params, &resolved, &fold_alphas, &positions),
    );
}

test "checkFolds rejects a resolved-query count under params.num_queries" {
    const params = fold_fixtures.fold_cases[0].params;
    const empty: [0]fri.ResolvedQuery = .{};
    const fold_alphas: [3]ext.Ext = .{ ext.Ext.zero(), ext.Ext.zero(), ext.Ext.zero() };
    const positions: [0]usize = .{};
    try std.testing.expectError(
        error.InvalidResolvedQueryCount,
        fri.checkFolds(params, &empty, &fold_alphas, &positions),
    );
}
