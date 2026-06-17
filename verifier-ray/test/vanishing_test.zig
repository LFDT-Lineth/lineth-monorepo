const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixtures = @import("test_vanishing");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const protocol = verifier_ray.protocol;
const vanishing = verifier_ray.query.vanishing;
const commitment_mod = verifier_ray.crypto.commitment;

test "vanishing quotient honest scenarios match prover-ray" {
    try std.testing.expect(fixtures.scenarios.len > 0);
    inline for (fixtures.scenarios) |case| {
        const spec = case.spec;
        const system = case.system;
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const proof = try buildProofData(arena.allocator(), case.honest);
        const coins = try protocol.replay(spec, proof.rounds);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = proof.rounds };
        try vanishing.verify(system, .{
            .ctx = ctx,
            .witness_claims = proof.witness_claims,
            .quotient_claims = proof.quotient_claims,
            .module_sizes = proof.module_sizes,
        });
    }
}

test "vanishing quotient invalid scenarios fail identity" {
    var invalid_case_count: usize = 0;
    inline for (fixtures.scenarios) |case| {
        const invalid = case.invalid orelse continue;
        invalid_case_count += 1;
        const spec = case.spec;
        const system = case.system;
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const proof = try buildProofData(arena.allocator(), invalid);
        const coins = try protocol.replay(spec, proof.rounds);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = proof.rounds };
        try std.testing.expectError(
            error.QuotientIdentityMismatch,
            vanishing.verify(system, .{
                .ctx = ctx,
                .witness_claims = proof.witness_claims,
                .quotient_claims = proof.quotient_claims,
                .module_sizes = proof.module_sizes,
            }),
        );
    }
    try std.testing.expect(invalid_case_count > 0);
}

test "dynamic vanishing module sizes are required and validated" {
    comptime var dynamic_case_count: usize = 0;
    var wrong_size_failures: usize = 0;
    inline for (fixtures.scenarios) |case| {
        const spec = case.spec;
        const system = case.system;
        if (system.dynamic_module_count == 0) continue;
        dynamic_case_count += 1;

        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();

        const valid = try buildProofData(arena.allocator(), case.honest);
        const coins = try protocol.replay(spec, valid.rounds);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = valid.rounds };

        try vanishing.verify(system, .{ .ctx = ctx, .witness_claims = valid.witness_claims, .quotient_claims = valid.quotient_claims, .module_sizes = valid.module_sizes });

        try std.testing.expectError(
            error.MissingDynamicModuleSize,
            vanishing.verify(system, .{ .ctx = ctx, .witness_claims = valid.witness_claims, .quotient_claims = valid.quotient_claims, .module_sizes = &.{} }),
        );

        var zero_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        zero_sizes[0] = 0;
        try std.testing.expectError(
            error.InvalidModuleSize,
            vanishing.verify(system, .{ .ctx = ctx, .witness_claims = valid.witness_claims, .quotient_claims = valid.quotient_claims, .module_sizes = zero_sizes }),
        );

        var non_power_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        non_power_sizes[0] = 7;
        try std.testing.expectError(
            error.InvalidModuleSize,
            vanishing.verify(system, .{ .ctx = ctx, .witness_claims = valid.witness_claims, .quotient_claims = valid.quotient_claims, .module_sizes = non_power_sizes }),
        );

        var wrong_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        wrong_sizes[0] = if (wrong_sizes[0] == 16) 8 else 16;
        vanishing.verify(system, .{ .ctx = ctx, .witness_claims = valid.witness_claims, .quotient_claims = valid.quotient_claims, .module_sizes = wrong_sizes }) catch |err| {
            if (err == error.QuotientIdentityMismatch) wrong_size_failures += 1 else return err;
        };
    }
    try std.testing.expect(dynamic_case_count > 0);
    // Some constraints may trivially vanish at multiple domain sizes (P(r) = 0,
    // Q(r) = 0 simultaneously), so not every case necessarily produces a
    // mismatch. Assert at least one case does to confirm the check is live.
    try std.testing.expect(wrong_size_failures > 0);
}

test "lagrange selector rejects an in-domain evaluation coin" {
    // A minimal static module of size 4 whose sole vanishing is the bare
    // selector L_1. The Fiat-Shamir eval coin is never on-domain in practice,
    // so the golden fixtures cannot reach the guard; build the degenerate input
    // by hand and feed r = ω^1 directly.
    const n = 4;
    const position = 1;
    const expressions = [_]vanishing.ExprNode{.{ .lagrange_selector = position }};
    const vanishings = [_]vanishing.Vanishing{.{ .expression = 0 }};
    const buckets = [_]vanishing.Bucket{.{ .ratio = 1, .vanishings = &vanishings, .quotient_claim_offset = 0 }};
    const modules = [_]vanishing.Module{.{
        .size = .{ .static = n },
        .expressions = &expressions,
        .buckets = &buckets,
        .witness_claim_offset = 0,
        .merge_coin_index = 0,
        .eval_coin_index = 1,
    }};
    const system = vanishing.System{ .modules = &modules, .total_witness_claims = 0, .total_quotient_claims = 1 };

    const omega = try field.rootOfUnityBy(n);
    const on_domain = ext.Ext.lift(omega.pow(position)); // r = ω^position
    const quotient_claims = [_]ext.Ext{ext.Ext.zero()};

    // In-domain coin: the r − ω^position denominator vanishes, so the guard
    // must reject rather than silently dividing by zero (the field's 1/0 = 0).
    {
        const all_coins = [_]ext.Ext{ ext.Ext.one(), on_domain };
        const ctx = protocol.Context{ .all_coins = &all_coins, .rounds = &.{} };
        try std.testing.expectError(
            error.LagrangeSelectorInDomain,
            vanishing.verify(system, .{ .ctx = ctx, .witness_claims = &.{}, .quotient_claims = &quotient_claims }),
        );
    }

    // Control: an out-of-domain coin must clear the guard and proceed to the
    // ordinary identity check (which fails here, confirming the earlier error
    // was specifically the in-domain guard and not something structural).
    {
        const off_domain = ext.Ext.lift(field.Element.init(2)); // 2 is not a 4th root of unity
        const all_coins = [_]ext.Ext{ ext.Ext.one(), off_domain };
        const ctx = protocol.Context{ .all_coins = &all_coins, .rounds = &.{} };
        try std.testing.expectError(
            error.QuotientIdentityMismatch,
            vanishing.verify(system, .{ .ctx = ctx, .witness_claims = &.{}, .quotient_claims = &quotient_claims }),
        );
    }
}

const ProofData = struct {
    rounds: []const protocol.RoundMessage,
    witness_claims: []const ext.Ext,
    quotient_claims: []const ext.Ext,
    module_sizes: []const usize,
};

fn buildProofData(allocator: std.mem.Allocator, proof: fixtures.VanishingProofView) !ProofData {
    const witness_claims = try allocator.alloc(ext.Ext, proof.witness_claims.len);
    for (proof.witness_claims, 0..) |claim, i| witness_claims[i] = ext.Ext.fromUints(claim);

    const quotient_claims = try allocator.alloc(ext.Ext, proof.quotient_claims.len);
    for (proof.quotient_claims, 0..) |claim, i| quotient_claims[i] = ext.Ext.fromUints(claim);

    const round_cells = try buildRoundCells(allocator, proof);
    const rounds = try buildRounds(allocator, proof, round_cells);

    return .{
        .rounds = rounds,
        .witness_claims = witness_claims,
        .quotient_claims = quotient_claims,
        .module_sizes = proof.module_sizes,
    };
}

fn buildRoundCells(allocator: std.mem.Allocator, proof: fixtures.VanishingProofView) ![]const []const protocol.Scalar {
    const round_cells = try allocator.alloc([]const protocol.Scalar, proof.rounds.len);
    for (proof.rounds, 0..) |round, i| {
        const cells = try allocator.alloc(protocol.Scalar, round.cells.len);
        for (round.cells, 0..) |cell, j| {
            cells[j] = switch (cell) {
                .base => |v| .{ .base = field.Element.init(v) },
                .ext => |v| .{ .ext = ext.Ext.fromUints(v) },
            };
        }
        round_cells[i] = cells;
    }
    return round_cells;
}

fn buildRounds(allocator: std.mem.Allocator, proof: fixtures.VanishingProofView, round_cells: []const []const protocol.Scalar) ![]const protocol.RoundMessage {
    const rounds = try allocator.alloc(protocol.RoundMessage, proof.rounds.len);
    for (proof.rounds, 0..) |round, i| {
        rounds[i] = try buildRoundMessage(allocator, round, round_cells[i]);
    }
    return rounds;
}

fn buildRoundMessage(allocator: std.mem.Allocator, round: fixtures.RuntimeTraceRound, cells: []const protocol.Scalar) !protocol.RoundMessage {
    var column_count: usize = 0;
    for (round.columns) |column| {
        column_count += switch (column) {
            .oracle => |commitments| commitments.len,
            .public_base, .public_ext => 1,
        };
    }

    const columns = try allocator.alloc(protocol.ColumnMessage, column_count);
    var next_column: usize = 0;
    for (round.columns) |column| {
        switch (column) {
            .oracle => |commitments| {
                for (commitments) |commitment| {
                    columns[next_column] = .{ .oracle_commitment = commitment_mod.fromUints(commitment) };
                    next_column += 1;
                }
            },
            .public_base => |values| {
                const converted = try allocator.alloc(field.Element, values.len);
                for (values, 0..) |value, i| converted[i] = field.Element.init(value);
                columns[next_column] = .{ .public_column = .{ .base = converted } };
                next_column += 1;
            },
            .public_ext => |values| {
                const converted = try allocator.alloc(ext.Ext, values.len);
                for (values, 0..) |value, i| converted[i] = ext.Ext.fromUints(value);
                columns[next_column] = .{ .public_column = .{ .ext = converted } };
                next_column += 1;
            },
        }
    }

    return .{ .columns = columns, .cells = cells };
}
