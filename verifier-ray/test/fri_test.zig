const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const merkle = verifier_ray.crypto.merkle;
const fri = verifier_ray.query.fri;

// Test vectors generated from prover-ray: crypto.merkle's tree-walk is
// checked against roots and branches from prover-ray's own
// newCompleteBinaryTree and Poseidon2, and query.fri's fold arithmetic is
// checked against a real multi-round, multi-level FRI proof at non-trivial
// (bit-reversed, mixed-parity) query positions. Regenerate via
// `make generate-testdata`; see
// prover-ray/crypto/koalabear/fri/vectors_gen_test.go.
const fixtures_json = @import("test_fri_vectors").raw;

// ─── JSON wire types ─────────────────────────────────────────────────────────
//
// Field/Ext elements travel as their canonical uint64 representation;
// Octuplets and Exts are fixed-length arrays in the same coordinate order
// prover-ray's generator emits (Ext: [B0.a0, B0.a1, B1.a0, B1.a1, B2.a0,
// B2.a1]).

const JsonOctuplet = [8]u64;
const JsonExt = [6]u64;

const JsonBranch = struct {
    leaf: JsonOctuplet,
    siblings: []JsonOctuplet,
};

const JsonPair = struct {
    self: JsonExt,
    sibling: JsonExt,
};

const JsonMerkleCase = struct {
    name: []const u8,
    leaf: JsonOctuplet,
    siblings: []JsonOctuplet,
    index: usize,
    root: JsonOctuplet,
    expect_match: bool,
    expect_error: []const u8 = "",
};

// log_codeword_size/num_rounds/log_final_poly_size/num_queries record the
// Params a case was generated under; runFoldCase checks them against the
// hardcoded comptime fri.Params it runs the case with.
const JsonFoldCase = struct {
    name: []const u8,
    log_codeword_size: u8,
    num_rounds: u8,
    log_final_poly_size: u8,
    num_queries: usize,
    fold_alphas: []JsonExt,
    round_roots: []JsonOctuplet,
    final_poly: []JsonExt,
    position: usize,
    running_branches: []JsonBranch,
    expected_rounds: []JsonPair,
    aux: []?JsonPair,
    expect_running_error: []const u8 = "",
    expect_fold_error: []const u8 = "",
};

const Fixtures = struct {
    merkle_cases: []JsonMerkleCase,
    fold_cases: []JsonFoldCase,
};

fn loadFixtures(allocator: std.mem.Allocator) !Fixtures {
    return std.json.parseFromSliceLeaky(Fixtures, allocator, fixtures_json, .{});
}

// ─── JSON -> verifier_ray value conversions ──────────────────────────────────

fn toDigest(o: JsonOctuplet) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, o) |*dst, v| dst.* = field.Element.init(v);
    return out;
}

fn toDigests(allocator: std.mem.Allocator, os: []JsonOctuplet) ![]poseidon2.Digest {
    const out = try allocator.alloc(poseidon2.Digest, os.len);
    for (out, os) |*dst, o| dst.* = toDigest(o);
    return out;
}

fn toExt(e: JsonExt) ext.Ext {
    return .{
        .B0 = .{ .a0 = field.Element.init(e[0]), .a1 = field.Element.init(e[1]) },
        .B1 = .{ .a0 = field.Element.init(e[2]), .a1 = field.Element.init(e[3]) },
        .B2 = .{ .a0 = field.Element.init(e[4]), .a1 = field.Element.init(e[5]) },
    };
}

fn toExts(allocator: std.mem.Allocator, es: []JsonExt) ![]ext.Ext {
    const out = try allocator.alloc(ext.Ext, es.len);
    for (out, es) |*dst, e| dst.* = toExt(e);
    return out;
}

fn toPair(p: JsonPair) fri.Pair {
    return .{ .self = toExt(p.self), .sibling = toExt(p.sibling) };
}

fn toOptPairs(allocator: std.mem.Allocator, ps: []?JsonPair) ![]?fri.Pair {
    const out = try allocator.alloc(?fri.Pair, ps.len);
    for (out, ps) |*dst, p| dst.* = if (p) |v| toPair(v) else null;
    return out;
}

fn toBranch(allocator: std.mem.Allocator, b: JsonBranch) !merkle.Branch {
    return .{ .leaf = toDigest(b.leaf), .siblings = try toDigests(allocator, b.siblings) };
}

fn toBranches(allocator: std.mem.Allocator, bs: []JsonBranch) ![]merkle.Branch {
    const out = try allocator.alloc(merkle.Branch, bs.len);
    for (out, bs) |*dst, b| dst.* = try toBranch(allocator, b);
    return out;
}

// Maps a fixture's error-name string to the corresponding error value.
fn mapError(name: []const u8) fri.Error {
    if (std.mem.eql(u8, name, "MerkleProofInvalid")) return error.MerkleProofInvalid;
    if (std.mem.eql(u8, name, "FoldMismatch")) return error.FoldMismatch;
    if (std.mem.eql(u8, name, "FinalPolyMismatch")) return error.FinalPolyMismatch;
    if (std.mem.eql(u8, name, "InvalidRunningLayerShape")) return error.InvalidRunningLayerShape;
    std.debug.panic("fri_test: unrecognized expected error name '{s}'", .{name});
}

fn mapMerkleError(name: []const u8) merkle.Error {
    if (std.mem.eql(u8, name, "EmptyBranch")) return error.EmptyBranch;
    std.debug.panic("fri_test: unrecognized expected merkle error name '{s}'", .{name});
}

// ─── crypto.merkle: vectors from prover-ray's newCompleteBinaryTree ─────────

fn runMerkleCase(allocator: std.mem.Allocator, case: JsonMerkleCase) !void {
    const branch = merkle.Branch{
        .leaf = toDigest(case.leaf),
        .siblings = try toDigests(allocator, case.siblings),
    };

    if (case.expect_error.len > 0) {
        try std.testing.expectError(mapMerkleError(case.expect_error), branch.recoverRoot(case.index));
        return;
    }

    const recovered = try branch.recoverRoot(case.index);
    const matches = std.meta.eql(recovered, toDigest(case.root));
    try std.testing.expectEqual(case.expect_match, matches);
}

test "merkle branches from prover-ray vectors" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const fixtures = try loadFixtures(allocator);
    try std.testing.expect(fixtures.merkle_cases.len > 0);
    for (fixtures.merkle_cases) |case| {
        runMerkleCase(allocator, case) catch |err| {
            std.debug.print("merkle case '{s}' failed: {}\n", .{ case.name, err });
            return err;
        };
    }
}

// ─── query.fri: vectors from a real multi-round, multi-level FRI proof ─────
//
// Every fold case shares this shape: domain size 16 (log_codeword_size = 4),
// 3 folding rounds.
const fold_params: fri.Params = .{
    .log_codeword_size = 4,
    .num_rounds = 3,
    .log_final_poly_size = 0,
    .num_queries = 1,
};

fn runFoldCase(allocator: std.mem.Allocator, comptime params: fri.Params, case: JsonFoldCase) !void {
    try std.testing.expectEqual(case.log_codeword_size, params.log_codeword_size);
    try std.testing.expectEqual(case.num_rounds, params.num_rounds);
    try std.testing.expectEqual(case.log_final_poly_size, params.log_final_poly_size);
    try std.testing.expectEqual(case.num_queries, params.num_queries);

    const fold_alphas = try toExts(allocator, case.fold_alphas);
    const round_roots = try toDigests(allocator, case.round_roots);
    const final_poly = try toExts(allocator, case.final_poly);
    const running_branches = try toBranches(allocator, case.running_branches);
    const positions = try allocator.dupe(usize, &.{case.position});

    const proof = fri.Proof{
        .round_roots = round_roots,
        .final_poly = final_poly,
        .running_queries = &.{running_branches},
    };

    try fri.checkOpeningProofShape(params, proof, fold_alphas, positions);

    const rounds = try allocator.alloc(fri.Pair, params.num_rounds);
    const running_result = fri.resolveRunningLayers(params, round_roots, running_branches, case.position, rounds);

    if (case.expect_running_error.len > 0) {
        try std.testing.expectError(mapError(case.expect_running_error), running_result);
        return;
    }
    try running_result;

    const want_rounds = case.expected_rounds;
    for (rounds[1..], want_rounds[1..]) |got, want| {
        const want_pair = toPair(want);
        try std.testing.expect(got.self.eql(want_pair.self));
        try std.testing.expect(got.sibling.eql(want_pair.sibling));
    }

    const aux = try toOptPairs(allocator, case.aux);
    const resolved = [_]fri.ResolvedQuery{.{ .rounds = rounds, .aux = aux, .final = final_poly[0] }};
    const fold_result = fri.checkFolds(params, &resolved, fold_alphas, positions);

    if (case.expect_fold_error.len > 0) {
        try std.testing.expectError(mapError(case.expect_fold_error), fold_result);
        return;
    }
    try fold_result;
}

test "fri fold cases from prover-ray vectors" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const fixtures = try loadFixtures(allocator);
    try std.testing.expect(fixtures.fold_cases.len > 0);
    for (fixtures.fold_cases) |case| {
        runFoldCase(allocator, fold_params, case) catch |err| {
            std.debug.print("fold case '{s}' failed: {}\n", .{ case.name, err });
            return err;
        };
    }
}
