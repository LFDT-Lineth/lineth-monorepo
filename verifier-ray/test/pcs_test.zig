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
// A single static column at size 2^2 makes reconstruct()'s top_size == 2, so the
// restricted params match the intended { log_codeword_size = 4,
// log_plaintext_size = 2, num_queries = 3 } (numRounds = 2, one running-layer
// root). The envelope is that same params (restrictTo is a no-op).
const challenge_system = pcs.System{
    .envelope_params = .{ .log_codeword_size = 4, .log_plaintext_size = 2, .num_queries = 3 },
    .columns = &.{
        .{ .batch_idx = 0, .is_ext = false, .size = .{ .static = 2 }, .shifts = &[_]isize{0} },
    },
    .num_batches = 1,
    .max_entries = 1,
    .max_size_log2 = 2,
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
    const recon = try pcs.reconstruct(challenge_system, &.{});
    var transcript = fiat_shamir.Transcript.init();
    const challenges = try pcs.deriveChallenges(challenge_system, recon, &transcript, challengeFriProof(1));
    try std.testing.expectEqual(@as(usize, 2), challenges.foldAlphas().len);
    try std.testing.expectEqual(@as(usize, 3), challenges.query_positions.len);
    // Query positions are reduced into the codeword domain (2^4 = 16).
    for (challenges.query_positions) |p| try std.testing.expect(p < 16);
}

test "deriveChallenges is deterministic for the same transcript and proof" {
    const recon = try pcs.reconstruct(challenge_system, &.{});
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    const a = try pcs.deriveChallenges(challenge_system, recon, &t1, challengeFriProof(7));
    const b = try pcs.deriveChallenges(challenge_system, recon, &t2, challengeFriProof(7));
    for (a.foldAlphas(), b.foldAlphas()) |x, y| try std.testing.expect(x.eql(y));
    try std.testing.expectEqualSlices(usize, &a.query_positions, &b.query_positions);
}

test "deriveChallenges depends on the absorbed transcript state" {
    // Two transcripts diverging before derivation must yield different
    // challenges: they are a function of the live Fiat-Shamir state, not just
    // the proof. (Absorb one differing element up front.)
    const recon = try pcs.reconstruct(challenge_system, &.{});
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    t1.updateExt(&.{ext.Ext.fromUints(.{ 1, 0, 0, 0, 0, 0 })});
    t2.updateExt(&.{ext.Ext.fromUints(.{ 2, 0, 0, 0, 0, 0 })});
    const a = try pcs.deriveChallenges(challenge_system, recon, &t1, challengeFriProof(9));
    const b = try pcs.deriveChallenges(challenge_system, recon, &t2, challengeFriProof(9));
    var any_alpha_differs = false;
    for (a.foldAlphas(), b.foldAlphas()) |x, y| {
        if (!x.eql(y)) any_alpha_differs = true;
    }
    try std.testing.expect(any_alpha_differs);
}

// ── STEP 1 byte-faithfulness anchor: runtime layout reconstruction ────────────
//
// These pin `pcs.reconstruct` against a HAND-COMPUTED expected canonical layout,
// exercising the varying-size property (one comptime System, two runtime sizes)
// and the static-column case (whose reconstruction must equal the old frozen
// layout the codegen emits at that single size).

// 2 batches. Batch 0 owns one DYNAMIC base column (module_sizes[0]) and one
// STATIC base column at size 2^2. Batch 1 owns one STATIC ext column at size 2^3.
// Declaration order: [dyn(b0), static2(b0), static3ext(b1)].
const recon_system = pcs.System{
    .envelope_params = .{ .log_codeword_size = 6, .log_plaintext_size = 5, .num_queries = 1 },
    .columns = &.{
        .{ .batch_idx = 0, .is_ext = false, .size = .{ .dynamic = 0 }, .shifts = &[_]isize{0} },
        .{ .batch_idx = 0, .is_ext = false, .size = .{ .static = 2 }, .shifts = &[_]isize{ 0, 1 } },
        .{ .batch_idx = 1, .is_ext = true, .size = .{ .static = 3 }, .shifts = &[_]isize{0} },
    },
    .num_batches = 2,
    .max_entries = 3,
    .max_size_log2 = 5,
};

test "reconstruct: dynamic column at size 2^2 (== a static col), canonical order" {
    // module_sizes[0] = 4 -> dyn col size_log2 = 2. Sizes present: {3 (b1 ext),
    // 2 (b0 dyn base, b0 static base)}. top_size = 3.
    // Canonical order (size DESC / batch ASC / base-then-ext / pos ASC):
    //   size 3: batch1 ext pos0  -> entry 0  (col decl 2)
    //   size 2: batch0 base pos0 -> entry 1  (col decl 0, the dyn col)
    //   size 2: batch0 base pos1 -> entry 2  (col decl 1, the static-2 col)
    const recon = try pcs.reconstruct(recon_system, &[_]usize{4});
    try std.testing.expectEqual(@as(usize, 3), recon.num_entries);
    try std.testing.expectEqual(@as(u8, 3), recon.top_size);
    try std.testing.expectEqual(@as(usize, 2), recon.distinct_count);

    // entry 0: batch1 ext size3 pos0, from col decl 2
    try std.testing.expectEqual(@as(u8, 3), recon.entry_size_log2[0]);
    try std.testing.expectEqual(@as(usize, 1), recon.entry_batch[0]);
    try std.testing.expect(recon.entry_is_ext[0]);
    try std.testing.expectEqual(@as(usize, 0), recon.entry_row_idx[0]);
    try std.testing.expectEqual(@as(usize, 2), recon.entry_col_decl_idx[0]);

    // entry 1: batch0 base size2 pos0, from col decl 0 (dyn)
    try std.testing.expectEqual(@as(u8, 2), recon.entry_size_log2[1]);
    try std.testing.expectEqual(@as(usize, 0), recon.entry_batch[1]);
    try std.testing.expect(!recon.entry_is_ext[1]);
    try std.testing.expectEqual(@as(usize, 0), recon.entry_row_idx[1]);
    try std.testing.expectEqual(@as(usize, 0), recon.entry_col_decl_idx[1]);

    // entry 2: batch0 base size2 pos1, from col decl 1 (static-2)
    try std.testing.expectEqual(@as(u8, 2), recon.entry_size_log2[2]);
    try std.testing.expectEqual(@as(usize, 1), recon.entry_row_idx[2]);
    try std.testing.expectEqual(@as(usize, 1), recon.entry_col_decl_idx[2]);

    // reverse map
    try std.testing.expectEqual(@as(usize, 1), recon.col_to_entry[0]);
    try std.testing.expectEqual(@as(usize, 2), recon.col_to_entry[1]);
    try std.testing.expectEqual(@as(usize, 0), recon.col_to_entry[2]);

    // index_by_batch: first entry is batch1 -> branch 0, then batch0 -> branch 1.
    try std.testing.expectEqual(@as(usize, 1), recon.index_by_batch[0]);
    try std.testing.expectEqual(@as(usize, 0), recon.index_by_batch[1]);

    // restricted params: top_size=3 -> offset=2, codeword 6-2=4, plaintext 3.
    try std.testing.expectEqual(@as(u8, 4), recon.params.log_codeword_size);
    try std.testing.expectEqual(@as(u8, 3), recon.params.log_plaintext_size);
}

test "reconstruct: same System, LARGER dynamic size changes bundle + top_size" {
    // module_sizes[0] = 16 -> dyn col size_log2 = 4, now the LARGEST size.
    // Sizes present: {4 (b0 dyn base), 3 (b1 ext), 2 (b0 static base)}. top_size=4.
    // Canonical order:
    //   size 4: batch0 base pos0 -> entry 0 (col decl 0, dyn)
    //   size 3: batch1 ext  pos0 -> entry 1 (col decl 2)
    //   size 2: batch0 base pos0 -> entry 2 (col decl 1, static-2)
    const recon = try pcs.reconstruct(recon_system, &[_]usize{16});
    try std.testing.expectEqual(@as(u8, 4), recon.top_size);

    try std.testing.expectEqual(@as(usize, 0), recon.entry_col_decl_idx[0]);
    try std.testing.expectEqual(@as(u8, 4), recon.entry_size_log2[0]);
    try std.testing.expectEqual(@as(usize, 2), recon.entry_col_decl_idx[1]);
    try std.testing.expectEqual(@as(usize, 1), recon.entry_col_decl_idx[2]);

    // The dyn column now occupies entry 0 (was entry 1 at the smaller size) — the
    // whole point: one baked System, layout is a runtime function of the size.
    try std.testing.expectEqual(@as(usize, 0), recon.col_to_entry[0]);

    // index_by_batch: first entry batch0 -> branch 0, then batch1 -> branch 1.
    try std.testing.expectEqual(@as(usize, 0), recon.index_by_batch[0]);
    try std.testing.expectEqual(@as(usize, 1), recon.index_by_batch[1]);

    // restricted params: top_size=4 -> offset=1, codeword 6-1=5, plaintext 4.
    try std.testing.expectEqual(@as(u8, 5), recon.params.log_codeword_size);
    try std.testing.expectEqual(@as(u8, 4), recon.params.log_plaintext_size);
}

test "reconstruct: rejects non-power-of-two and missing dynamic sizes" {
    try std.testing.expectError(error.NonPowerOfTwoModuleSize, pcs.reconstruct(recon_system, &[_]usize{6}));
    try std.testing.expectError(error.MissingDynamicModuleSize, pcs.reconstruct(recon_system, &.{}));
}
