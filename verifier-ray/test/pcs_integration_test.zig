const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixtures = @import("test_pcs_integration");

const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;
const vanishing = verifier_ray.query.vanishing;
const pcs = verifier_ray.query.pcs;
const merkle = verifier_ray.crypto.merkle;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;

fn toDigest(o: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, o) |*dst, v| dst.* = field.Element.init(v);
    return out;
}

fn toExt(e: [6]u32) ext.Ext {
    return .{
        .B0 = .{ .a0 = field.Element.init(e[0]), .a1 = field.Element.init(e[1]) },
        .B1 = .{ .a0 = field.Element.init(e[2]), .a1 = field.Element.init(e[3]) },
        .B2 = .{ .a0 = field.Element.init(e[4]), .a1 = field.Element.init(e[5]) },
    };
}

fn toRowOpening(allocator: std.mem.Allocator, r: fixtures.RowOpeningData) !merkle.RowOpening {
    const base = try allocator.alloc(field.Element, r.base.len);
    for (base, r.base) |*dst, v| dst.* = field.Element.init(v);
    const ext_vals = try allocator.alloc(ext.Ext, r.ext.len);
    for (ext_vals, r.ext) |*dst, v| dst.* = toExt(v);
    return .{ .base = base, .ext = ext_vals };
}

fn toInputTreeOpening(allocator: std.mem.Allocator, o: fixtures.InputTreeOpeningData) !merkle.InputTreeOpening {
    const siblings = try allocator.alloc(poseidon2.Digest, o.siblings.len);
    for (siblings, o.siblings) |*dst, s| dst.* = toDigest(s);
    const leaves = try allocator.alloc(?merkle.RowPair, o.leaves.len);
    for (leaves, o.leaves) |*dst, l| {
        dst.* = if (l) |pair| .{ try toRowOpening(allocator, pair[0]), try toRowOpening(allocator, pair[1]) } else null;
    }
    return .{ .siblings = siblings, .leaves = leaves };
}

fn toProof(allocator: std.mem.Allocator, p: fixtures.OpeningProofData) !pcs.OpeningProof {
    const input_queries = try allocator.alloc([]const merkle.InputTreeOpening, p.input_queries.len);
    for (input_queries, p.input_queries) |*dst, iq| {
        const openings = try allocator.alloc(merkle.InputTreeOpening, iq.len);
        for (openings, iq) |*d, o| d.* = try toInputTreeOpening(allocator, o);
        dst.* = openings;
    }

    const round_roots = try allocator.alloc(poseidon2.Digest, p.fri_proof.round_roots.len);
    for (round_roots, p.fri_proof.round_roots) |*dst, r| dst.* = toDigest(r);

    const final_poly = try allocator.alloc(ext.Ext, p.fri_proof.final_poly.len);
    for (final_poly, p.fri_proof.final_poly) |*dst, v| dst.* = toExt(v);

    const running_queries = try allocator.alloc([]const merkle.Branch, p.fri_proof.running_queries.len);
    for (running_queries, p.fri_proof.running_queries) |*dst, rq| {
        const branches = try allocator.alloc(merkle.Branch, rq.len);
        for (branches, rq) |*d, b| {
            const siblings = try allocator.alloc(poseidon2.Digest, b.siblings.len);
            for (siblings, b.siblings) |*d2, s| d2.* = toDigest(s);
            d.* = .{ .leaf = toDigest(b.leaf), .siblings = siblings };
        }
        dst.* = branches;
    }

    return .{
        .input_queries = input_queries,
        .fri_proof = .{ .round_roots = round_roots, .final_poly = final_poly, .running_queries = running_queries },
    };
}

const spec = protocol.Spec{
    .round_coin_counts = &[_]usize{ 0, 1 },
    .round_coin_offsets = &[_]usize{ 0, 0 },
    .total_round_coins = 1,
};

const Corruption = enum { none, round_root, claimed_value };

fn runCase(
    allocator: std.mem.Allocator,
    comptime system: pcs.System,
    case: fixtures.IntegrationCase,
    corrupt: Corruption,
) !void {
    var round_root = case.round_root;
    if (corrupt == .round_root) round_root[0] +%= 1;
    const columns = try allocator.alloc(protocol.ColumnMessage, 1);
    columns[0] = .{ .oracle_commitment = toDigest(round_root) };
    const rounds = try allocator.alloc(protocol.RoundMessage, 1);
    rounds[0] = .{ .columns = columns, .cells = &.{} };

    const claimed_values = try allocator.alloc(ext.Ext, case.claimed_values.len);
    for (claimed_values, case.claimed_values) |*dst, v| dst.* = toExt(v);
    if (corrupt == .claimed_value) claimed_values[0] = claimed_values[0].add(ext.Ext.lift(field.Element.one()));

    const systems = verifier.Systems{
        .vanishing = vanishing.System{ .modules = &.{}, .total_witness_claims = 1 },
        .pcs = system,
    };

    try verifier.verify(spec, systems, .{
        .rounds = rounds,
        .witness_claims = &.{},
        .quotient_claims = &.{},
        .pcs_opening = .{
            .claimed_values = claimed_values,
            .proof = try toProof(allocator, case.proof),
        },
    });
}

test "verifier.verify authenticates a real PCS opening bound to a round commitment" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    inline for (fixtures.integration_cases) |case| {
        try runCase(allocator, case.system, case, .none);
    }
}

test "verifier.verify rejects a round commitment that does not match the PCS root" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    inline for (fixtures.integration_cases) |case| {
        try std.testing.expectError(error.MerkleProofInvalid, runCase(allocator, case.system, case, .round_root));
    }
}

test "verifier.verify rejects a claimed value that does not match the opening" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    inline for (fixtures.integration_cases) |case| {
        try std.testing.expectError(error.BoundaryFinalSelfMismatch, runCase(allocator, case.system, case, .claimed_value));
    }
}
