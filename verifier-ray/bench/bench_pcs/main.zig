const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixture = @import("large_pcs_fixture");
const accel = @import("lineth_accelerators");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const merkle = verifier_ray.crypto.merkle;
const pcs = verifier_ray.query.pcs;
const profiling = verifier_ray.profiling;

// Conversion is deliberately outside the measured region. It models the
// verifier receiving an already-decoded proof, while keeping the benchmark
// fixture compact and generated from the prover's exported API.
var arena_storage: [16 * 1024 * 1024]u8 align(16) = undefined;

fn toDigest(o: [8]u32) poseidon2.Digest {
    var out: poseidon2.Digest = undefined;
    for (&out, o) |*dst, value| dst.* = field.Element.init(value);
    return out;
}

fn toDigests(allocator: std.mem.Allocator, os: []const [8]u32) ![]poseidon2.Digest {
    const out = try allocator.alloc(poseidon2.Digest, os.len);
    for (out, os) |*dst, value| dst.* = toDigest(value);
    return out;
}

fn toExt(e: [6]u32) ext.Ext {
    return .{
        .B0 = .{ .a0 = field.Element.init(e[0]), .a1 = field.Element.init(e[1]) },
        .B1 = .{ .a0 = field.Element.init(e[2]), .a1 = field.Element.init(e[3]) },
        .B2 = .{ .a0 = field.Element.init(e[4]), .a1 = field.Element.init(e[5]) },
    };
}

fn toExts(allocator: std.mem.Allocator, values: []const [6]u32) ![]ext.Ext {
    const out = try allocator.alloc(ext.Ext, values.len);
    for (out, values) |*dst, value| dst.* = toExt(value);
    return out;
}

fn toExtsJagged(allocator: std.mem.Allocator, rows: []const []const [6]u32) ![]const []const ext.Ext {
    const out = try allocator.alloc([]const ext.Ext, rows.len);
    for (out, rows) |*dst, row| dst.* = try toExts(allocator, row);
    return out;
}

fn toRowOpening(allocator: std.mem.Allocator, row: fixture.RowOpeningData) !merkle.RowOpening {
    const base = try allocator.alloc(field.Element, row.base.len);
    for (base, row.base) |*dst, value| dst.* = field.Element.init(value);
    return .{ .base = base, .ext = try toExts(allocator, row.ext) };
}

fn toRowPair(allocator: std.mem.Allocator, pair: fixture.RowPairData) !merkle.RowPair {
    return .{ try toRowOpening(allocator, pair[0]), try toRowOpening(allocator, pair[1]) };
}

fn toInputTreeOpening(allocator: std.mem.Allocator, opening: fixture.InputTreeOpeningData) !merkle.InputTreeOpening {
    const leaves = try allocator.alloc(?merkle.RowPair, opening.leaves.len);
    for (leaves, opening.leaves) |*dst, leaf| dst.* = if (leaf) |pair| try toRowPair(allocator, pair) else null;
    return .{ .siblings = try toDigests(allocator, opening.siblings), .leaves = leaves };
}

fn toInputQueries(allocator: std.mem.Allocator, queries: []const []const fixture.InputTreeOpeningData) ![]const []const merkle.InputTreeOpening {
    const out = try allocator.alloc([]const merkle.InputTreeOpening, queries.len);
    for (out, queries) |*dst, query| {
        const trees = try allocator.alloc(merkle.InputTreeOpening, query.len);
        for (trees, query) |*tree, opening| tree.* = try toInputTreeOpening(allocator, opening);
        dst.* = trees;
    }
    return out;
}

fn toInputCaps(allocator: std.mem.Allocator, caps: []const fixture.InputCapData) ![]const pcs.InputCap {
    const out = try allocator.alloc(pcs.InputCap, caps.len);
    for (out, caps) |*dst, cap| {
        const tables = try allocator.alloc(pcs.InputCapTable, cap.tables.len);
        for (tables, cap.tables) |*table_dst, table| {
            const rows = try allocator.alloc(merkle.RowOpening, table.rows.len);
            for (rows, table.rows) |*row_dst, row| row_dst.* = try toRowOpening(allocator, row);
            table_dst.* = .{ .size_log2 = table.size_log2, .rows = rows };
        }
        dst.* = .{ .root = toDigest(cap.root), .nodes = try toDigests(allocator, cap.nodes), .tables = tables };
    }
    return out;
}

fn toMerkleCaps(allocator: std.mem.Allocator, caps: []const fixture.MerkleCapData) ![]const merkle.MerkleCap {
    const out = try allocator.alloc(merkle.MerkleCap, caps.len);
    for (out, caps) |*dst, cap| {
        const aux = try allocator.alloc(?poseidon2.Digest, cap.aux.len);
        for (aux, cap.aux) |*aux_dst, value| aux_dst.* = if (value) |digest| toDigest(digest) else null;
        dst.* = .{ .nodes = try toDigests(allocator, cap.nodes), .aux = aux };
    }
    return out;
}

fn toBranch(allocator: std.mem.Allocator, branch: fixture.BranchData) !merkle.Branch {
    return .{ .leaf = toDigest(branch.leaf), .siblings = try toDigests(allocator, branch.siblings) };
}

fn toRunningQueries(allocator: std.mem.Allocator, queries: []const []const fixture.BranchData) ![]const []const merkle.Branch {
    const out = try allocator.alloc([]const merkle.Branch, queries.len);
    for (out, queries) |*dst, query| {
        const branches = try allocator.alloc(merkle.Branch, query.len);
        for (branches, query) |*branch_dst, branch| branch_dst.* = try toBranch(allocator, branch);
        dst.* = branches;
    }
    return out;
}

fn toVerifyInput(allocator: std.mem.Allocator, case: fixture.PcsCase) !pcs.VerifyInput {
    return .{
        .roots = try toDigests(allocator, case.roots),
        .entry_claims = try toExtsJagged(allocator, case.entry_claims),
        .zeta = toExt(case.zeta),
        .fold_alphas = try toExts(allocator, case.fold_alphas),
        .query_positions = try allocator.dupe(usize, case.query_positions),
        .proof = .{
            .input_queries = try toInputQueries(allocator, case.proof.input_queries),
            .input_caps = try toInputCaps(allocator, case.proof.input_caps),
            .fri_proof = .{
                .round_roots = try toDigests(allocator, case.proof.fri_proof.round_roots),
                .round_caps = try toMerkleCaps(allocator, case.proof.fri_proof.round_caps),
                .final_poly = try toExts(allocator, case.proof.fri_proof.final_poly),
                .running_queries = try toRunningQueries(allocator, case.proof.fri_proof.running_queries),
            },
        },
    };
}

pub export fn main() noreturn {
    var allocator_state = std.heap.FixedBufferAllocator.init(&arena_storage);
    const input = toVerifyInput(allocator_state.allocator(), fixture.large_case) catch accel.zkvm_exit(1);

    profiling.markR5Value(10, 0);
    pcs.verify(fixture.large_case.system, input) catch accel.zkvm_exit(1);
    profiling.markR5Value(11, 1);
    accel.zkvm_exit(0);
}
