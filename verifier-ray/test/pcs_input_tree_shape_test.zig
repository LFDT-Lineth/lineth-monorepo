const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const merkle = verifier_ray.crypto.merkle;
const poseidon2 = verifier_ray.crypto.poseidon2;
const pcs = verifier_ray.query.pcs;
const verifier = verifier_ray.verifier;

// End-to-end regression for input-tree routing: every opened batch gets its
// own input tree, even when two batches commit the same table and so share a
// Merkle root. The PCS used to merge such batches into one input tree; that
// made the proof's shape depend on the data, and the fixed-shape (gnark)
// verifier could not follow it.
//
// The fixture case "AllZeroFilter" has two batches with equal roots. Identical
// trees have identical caps, which is how this test finds the twins. It checks
// that the honest proof (one tree per batch) verifies, and that the old
// deduplicated shape (the second twin's cap and per-query openings dropped) is
// rejected.
//
// Resolve the fixture index by NAME so fixture regeneration (which shifts case
// ordering) never silently points this test at the wrong scenario; a missing
// scenario is a comptime error.
const case_index: usize = caseIndexByName("AllZeroFilter");

fn caseIndexByName(comptime name: []const u8) usize {
    comptime {
        for (0..vf.case_count) |i| {
            if (std.mem.eql(u8, vf.get(i).name, name)) return i;
        }
        @compileError("pcs_input_tree_shape_test: fixture case '" ++ name ++ "' not found; regenerate testdata or update the test");
    }
}

const spec = vf.get(case_index).spec;
const systems = vf.get(case_index).systems;

fn sameCap(a: pcs.InputCap, b: pcs.InputCap) bool {
    if (a.nodes.len != b.nodes.len) return false;
    for (a.nodes, b.nodes) |x, y| {
        if (!poseidon2.eql(x, y)) return false;
    }
    return true;
}

/// Index of the second of two input trees with identical caps, i.e. the tree a
/// root-deduplicating prover would have omitted.
fn twinTreeIndex(caps: []const pcs.InputCap) ?usize {
    for (caps, 0..) |cap, j| {
        for (caps[0..j]) |earlier| {
            if (sameCap(earlier, cap)) return j;
        }
    }
    return null;
}

fn without(comptime T: type, a: std.mem.Allocator, items: []const T, drop: usize) ![]const T {
    const res = try a.alloc(T, items.len - 1);
    @memcpy(res[0..drop], items[0..drop]);
    @memcpy(res[drop..], items[drop + 1 ..]);
    return res;
}

test "input trees: twin batches each get their own tree and verify" {
    const input = vf.getInput(case_index);
    const opening = input.proof.pcs_opening.proof;

    // Guard: the fixture must actually have twin trees, or this test checks
    // nothing about deduplication.
    try std.testing.expect(twinTreeIndex(opening.input_caps) != null);
    for (opening.input_queries) |query| {
        try std.testing.expectEqual(opening.input_caps.len, query.len);
    }

    try verifier.verify(spec, systems, input.proof, input.public_inputs);
}

test "input trees: the old deduplicated shape is rejected" {
    const input = vf.getInput(case_index);
    const opening = input.proof.pcs_opening.proof;
    const twin = twinTreeIndex(opening.input_caps) orelse return error.FixtureHasNoTwinTrees;

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // Drop the twin's cap and its opening in every query; everything else
    // stays honest.
    const queries = try a.alloc([]const merkle.InputTreeOpening, opening.input_queries.len);
    for (opening.input_queries, queries) |query, *out| {
        out.* = try without(merkle.InputTreeOpening, a, query, twin);
    }

    var bad = input;
    bad.proof.pcs_opening.proof.input_caps = try without(pcs.InputCap, a, opening.input_caps, twin);
    bad.proof.pcs_opening.proof.input_queries = queries;

    try std.testing.expectError(
        error.InputTreeCountMismatch,
        verifier.verify(spec, systems, bad.proof, bad.public_inputs),
    );
}
