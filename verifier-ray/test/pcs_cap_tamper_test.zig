const std = @import("std");
const verifier_ray = @import("verifier_ray");
const vf = @import("test_verify");

const verifier = verifier_ray.verifier;
const pcs = verifier_ray.query.pcs;
const merkle = verifier_ray.crypto.merkle;
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;

// Cap-authentication soundness, end to end through `verifier.verify`.
//
// A Merkle cap is the frontier a PCS opening authenticates its queries
// against. Query branches only climb to the frontier nodes they land on; every
// other node (and every row of an input cap table) is bound to the committed
// root only by recomputing the root from the whole cap. These tests take
// honest verify.zig proofs and tamper each cap node, each table row and each
// auxiliary digest in turn — so the unqueried ones are covered without
// re-deriving the transcript's query positions — and require every tampered
// proof to be rejected.
//
// The cases are chosen for shape: Fibonacci and SizeThirtyTwoCubic carry more
// cap nodes than their 4 queries can reach, ConditionalLookupShape carries
// input cap tables.

fn caseIndex(comptime name: []const u8) usize {
    comptime {
        @setEvalBranchQuota(100_000);
        for (0..vf.case_count) |i| {
            if (std.mem.eql(u8, vf.get(i).name, name)) return i;
        }
        @compileError("no verify.zig case named " ++ name);
    }
}

fn verifyProof(comptime i: usize, proof: verifier.Proof) !void {
    const case = comptime vf.get(i);
    try verifier.verify(case.spec, case.systems, proof, vf.getInput(i).public_inputs);
}

fn expectRejected(comptime i: usize, proof: verifier.Proof, what: []const u8) !void {
    verifyProof(i, proof) catch return;
    std.debug.print("case {s} accepted a proof with {s}\n", .{ comptime vf.get(i).name, what });
    return error.TamperedProofAccepted;
}

fn bump(d: poseidon2.Digest) poseidon2.Digest {
    var out = d;
    out[0] = out[0].add(field.Element.one());
    return out;
}

fn withInputCaps(proof: verifier.Proof, caps: []const pcs.InputCap) verifier.Proof {
    var out = proof;
    out.pcs_opening.proof.input_caps = caps;
    return out;
}

fn withRoundCaps(proof: verifier.Proof, caps: []const merkle.MerkleCap) verifier.Proof {
    var out = proof;
    out.pcs_opening.proof.fri_proof.round_caps = caps;
    return out;
}

fn tamperEveryInputCapNode(comptime name: []const u8) !usize {
    const i = comptime caseIndex(name);
    const honest = vf.getInput(i).proof;
    try verifyProof(i, honest);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    var tampered: usize = 0;
    const honest_caps = honest.pcs_opening.proof.input_caps;
    for (honest_caps, 0..) |cap, c| {
        for (0..cap.nodes.len) |k| {
            const caps = try allocator.dupe(pcs.InputCap, honest_caps);
            const nodes = try allocator.dupe(poseidon2.Digest, cap.nodes);
            nodes[k] = bump(nodes[k]);
            caps[c].nodes = nodes;
            try expectRejected(i, withInputCaps(honest, caps), "a tampered input cap node");
            tampered += 1;
        }
    }
    return tampered;
}

fn tamperEveryRoundCapNode(comptime name: []const u8) !usize {
    const i = comptime caseIndex(name);
    const honest = vf.getInput(i).proof;
    try verifyProof(i, honest);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    var tampered: usize = 0;
    const honest_caps = honest.pcs_opening.proof.fri_proof.round_caps;
    for (honest_caps, 0..) |cap, c| {
        for (0..cap.nodes.len) |k| {
            const caps = try allocator.dupe(merkle.MerkleCap, honest_caps);
            const nodes = try allocator.dupe(poseidon2.Digest, cap.nodes);
            nodes[k] = bump(nodes[k]);
            caps[c].nodes = nodes;
            try expectRejected(i, withRoundCaps(honest, caps), "a tampered running cap node");
            tampered += 1;
        }
    }
    return tampered;
}

fn tamperEveryRoundCapAux(comptime name: []const u8) !usize {
    const i = comptime caseIndex(name);
    const honest = vf.getInput(i).proof;
    try verifyProof(i, honest);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    var tampered: usize = 0;
    const honest_caps = honest.pcs_opening.proof.fri_proof.round_caps;
    for (honest_caps, 0..) |cap, c| {
        for (0..cap.aux.len) |k| {
            const caps = try allocator.dupe(merkle.MerkleCap, honest_caps);
            const aux = try allocator.dupe(?poseidon2.Digest, cap.aux);
            // An absent aux digest gets a plausible-looking one (a cap node);
            // a present one is perturbed.
            aux[k] = if (aux[k]) |d| bump(d) else cap.nodes[0];
            caps[c].aux = aux;
            try expectRejected(i, withRoundCaps(honest, caps), "a tampered running cap auxiliary digest");
            tampered += 1;
        }
    }
    return tampered;
}

test "verify rejects every tampered input cap node" {
    var tampered: usize = 0;
    tampered += try tamperEveryInputCapNode("Fibonacci");
    tampered += try tamperEveryInputCapNode("SizeThirtyTwoCubic");
    tampered += try tamperEveryInputCapNode("ConditionalLookupShape");
    try std.testing.expect(tampered > 0);
}

test "verify rejects every tampered input cap table row" {
    const i = comptime caseIndex("ConditionalLookupShape");
    const honest = vf.getInput(i).proof;
    try verifyProof(i, honest);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    var tampered: usize = 0;
    const honest_caps = honest.pcs_opening.proof.input_caps;
    for (honest_caps, 0..) |cap, c| {
        for (cap.tables, 0..) |table, t| {
            for (table.rows, 0..) |row, r| {
                var tampered_row = row;
                if (row.base.len > 0) {
                    const base = try allocator.dupe(field.Element, row.base);
                    base[0] = base[0].add(field.Element.one());
                    tampered_row.base = base;
                } else if (row.ext.len > 0) {
                    const values = try allocator.dupe(@TypeOf(row.ext[0]), row.ext);
                    values[0].B0.a0 = values[0].B0.a0.add(field.Element.one());
                    tampered_row.ext = values;
                } else continue;

                const rows = try allocator.dupe(merkle.RowOpening, table.rows);
                rows[r] = tampered_row;
                const tables = try allocator.dupe(pcs.InputCapTable, cap.tables);
                tables[t].rows = rows;
                const caps = try allocator.dupe(pcs.InputCap, honest_caps);
                caps[c].tables = tables;
                try expectRejected(i, withInputCaps(honest, caps), "a tampered input cap table row");
                tampered += 1;
            }
        }
    }
    try std.testing.expect(tampered > 0);
}

// Revealed tables must match the reconstructed shape position by position
// (`size_log2` in shape order) and in count. That per-position check is what
// rejects reordered tables; no verify.zig proof reveals two tables in one cap,
// so it is exercised by relabelling, dropping and duplicating a table instead.
test "verify rejects input cap tables that do not match the revealed shape" {
    const i = comptime caseIndex("ConditionalLookupShape");
    const honest = vf.getInput(i).proof;
    try verifyProof(i, honest);

    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    var tampered: usize = 0;
    const honest_caps = honest.pcs_opening.proof.input_caps;
    for (honest_caps, 0..) |cap, c| {
        if (cap.tables.len == 0) continue;

        const relabelled = try allocator.dupe(pcs.InputCapTable, cap.tables);
        relabelled[0].size_log2 +%= 1;
        var caps = try allocator.dupe(pcs.InputCap, honest_caps);
        caps[c].tables = relabelled;
        try expectRejected(i, withInputCaps(honest, caps), "a relabelled input cap table");

        caps = try allocator.dupe(pcs.InputCap, honest_caps);
        caps[c].tables = cap.tables[1..];
        try expectRejected(i, withInputCaps(honest, caps), "a missing input cap table");

        const duplicated = try allocator.alloc(pcs.InputCapTable, cap.tables.len + 1);
        @memcpy(duplicated[0..cap.tables.len], cap.tables);
        duplicated[cap.tables.len] = cap.tables[0];
        caps = try allocator.dupe(pcs.InputCap, honest_caps);
        caps[c].tables = duplicated;
        try expectRejected(i, withInputCaps(honest, caps), "a duplicated input cap table");

        tampered += 3;
    }
    try std.testing.expect(tampered > 0);
}

test "verify rejects every tampered running cap node" {
    var tampered: usize = 0;
    tampered += try tamperEveryRoundCapNode("Fibonacci");
    tampered += try tamperEveryRoundCapNode("SizeThirtyTwoCubic");
    try std.testing.expect(tampered > 0);
}

test "verify rejects every tampered running cap auxiliary digest" {
    var tampered: usize = 0;
    tampered += try tamperEveryRoundCapAux("Fibonacci");
    tampered += try tamperEveryRoundCapAux("SizeThirtyTwoCubic");
    try std.testing.expect(tampered > 0);
}
