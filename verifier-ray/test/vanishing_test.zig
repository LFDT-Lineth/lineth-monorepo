const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixtures = @import("test_vanishing");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const protocol = verifier_ray.protocol;
const public_input = protocol.public_input;
const vanishing = verifier_ray.query.vanishing;
const logderivativesum = verifier_ray.query.logderivativesum;
const commitment_mod = verifier_ray.crypto.commitment;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;

test "vanishing quotient honest scenarios match prover-ray" {
    try std.testing.expect(fixtures.scenarios.len > 0);
    inline for (fixtures.scenarios) |case| {
        const spec = case.spec;
        const system = case.system;
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const proof = try buildProofData(arena.allocator(), case.honest);
        var bound = try public_input.bindRoundMessages(case.public_input, proof.proof_rounds, proof.public_inputs);
        const rounds = bound.rounds();
        var transcript = fiat_shamir.Transcript.init();
        const coins = try protocol.replayWithTranscript(&transcript, spec, rounds, proof.module_sizes);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = rounds };
        try vanishing.verify(system, .{
            .ctx = ctx,
            .witness_claims = proof.witness_claims,
            .quotient_claims = proof.quotient_claims,
            .module_sizes = proof.module_sizes,
        });
        // The same honest proof must satisfy the LogDerivativeSum boundary
        // checks (final-sum == Result). Empty logderiv systems pass trivially.
        try logderivativesum.verify(case.logderiv, ctx);
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
        var bound = try public_input.bindRoundMessages(case.public_input, proof.proof_rounds, proof.public_inputs);
        const rounds = bound.rounds();
        var transcript = fiat_shamir.Transcript.init();
        const coins = try protocol.replayWithTranscript(&transcript, spec, rounds, proof.module_sizes);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = rounds };
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
        var bound = try public_input.bindRoundMessages(case.public_input, valid.proof_rounds, valid.public_inputs);
        const rounds = bound.rounds();
        var transcript = fiat_shamir.Transcript.init();
        const coins = try protocol.replayWithTranscript(&transcript, spec, rounds, valid.module_sizes);
        const ctx = protocol.Context{ .all_coins = &coins, .rounds = rounds };

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

// dynSelectorSystem builds a single-bucket DYNAMIC module (size from
// module_sizes[0]) whose sole vanishing is the bare selector L_position — the
// dynamic analogue of the static fixture in the in-domain test above, for
// exercising the runtime position bounds check against a proof-supplied size.
fn dynSelectorSystem(comptime position: i32) vanishing.System {
    const S = struct {
        const expressions = [_]vanishing.ExprNode{.{ .lagrange_selector = position }};
        const vanishings = [_]vanishing.Vanishing{.{ .expression = 0 }};
        const buckets = [_]vanishing.Bucket{.{ .ratio = 1, .vanishings = &vanishings, .quotient_claim_offset = 0 }};
        const modules = [_]vanishing.Module{.{
            .size = .{ .dynamic = 0 },
            .expressions = &expressions,
            .buckets = &buckets,
            .witness_claim_offset = 0,
            .merge_coin_index = 0,
            .eval_coin_index = 1,
        }};
    };
    return .{ .modules = &S.modules, .dynamic_module_count = 1, .total_witness_claims = 0, .total_quotient_claims = 1 };
}

// dynCancelledSystem is dynSelectorSystem's analogue for the cancellation
// path: a constant-1 vanishing carrying one cancelled position, so
// cancellationAtPoint (not evalLagrangeSelector) resolves `position` against
// the proof-supplied size.
fn dynCancelledSystem(comptime position: i32) vanishing.System {
    const S = struct {
        const expressions = [_]vanishing.ExprNode{.{ .constant = field.Element.init(1) }};
        const cancelled = [_]i32{position};
        const vanishings = [_]vanishing.Vanishing{.{ .expression = 0, .cancelled_positions = &cancelled }};
        const buckets = [_]vanishing.Bucket{.{ .ratio = 1, .vanishings = &vanishings, .quotient_claim_offset = 0 }};
        const modules = [_]vanishing.Module{.{
            .size = .{ .dynamic = 0 },
            .expressions = &expressions,
            .buckets = &buckets,
            .witness_claim_offset = 0,
            .merge_coin_index = 0,
            .eval_coin_index = 1,
        }};
    };
    return .{ .modules = &S.modules, .dynamic_module_count = 1, .total_witness_claims = 0, .total_quotient_claims = 1 };
}

test "lagrange selector rejects positions outside [-n, n) for dynamic modules" {
    // module_sizes is proof-supplied: a hostile size can push a codegen-baked
    // position out of the addressable range [-n, n). Below n = 4, an
    // off-domain eval coin r = 2, and a zero quotient claim.
    const n = 4;
    const sizes = [_]usize{n};
    const quotient_claims = [_]ext.Ext{ext.Ext.zero()};
    const off_domain = ext.Ext.lift(field.Element.init(2)); // 2 is not a 4th root of unity
    const all_coins = [_]ext.Ext{ ext.Ext.one(), off_domain };
    const ctx = protocol.Context{ .all_coins = &all_coins, .rounds = &.{} };
    const input = vanishing.CheckInput{
        .ctx = ctx,
        .witness_claims = &.{},
        .quotient_claims = &quotient_claims,
        .module_sizes = &sizes,
    };

    // position == n: one past the last row. Without the bounds check the
    // root-of-unity exponentiation would silently reduce it mod n and evaluate
    // L_0 — a different selector than the constraint declares.
    try std.testing.expectError(
        error.LagrangeSelectorPositionOutOfRange,
        vanishing.verify(dynSelectorSystem(n), input),
    );

    // position == -n-1: one below the first addressable end-relative row.
    // Without the bounds check normalizePosition's usize subtraction (n - n-1)
    // underflows.
    try std.testing.expectError(
        error.LagrangeSelectorPositionOutOfRange,
        vanishing.verify(dynSelectorSystem(-n - 1), input),
    );

    // position == -n: the valid boundary (resolves to row 0). It must clear
    // the bounds check and proceed to the ordinary identity check, which fails
    // here (L_0(2) != 0 while the quotient claim is zero) — confirming the
    // rejections above were specifically the bounds guard.
    try std.testing.expectError(
        error.QuotientIdentityMismatch,
        vanishing.verify(dynSelectorSystem(-n), input),
    );
}

test "cancelled positions get the same dynamic bounds check" {
    const n = 4;
    const sizes = [_]usize{n};
    const quotient_claims = [_]ext.Ext{ext.Ext.zero()};
    const off_domain = ext.Ext.lift(field.Element.init(2));
    const all_coins = [_]ext.Ext{ ext.Ext.one(), off_domain };
    const ctx = protocol.Context{ .all_coins = &all_coins, .rounds = &.{} };
    const input = vanishing.CheckInput{
        .ctx = ctx,
        .witness_claims = &.{},
        .quotient_claims = &quotient_claims,
        .module_sizes = &sizes,
    };

    try std.testing.expectError(
        error.LagrangeSelectorPositionOutOfRange,
        vanishing.verify(dynCancelledSystem(n), input),
    );
    try std.testing.expectError(
        error.LagrangeSelectorPositionOutOfRange,
        vanishing.verify(dynCancelledSystem(-n - 1), input),
    );
    // Valid boundary: -n resolves to row 0, C(r) = r - 1 = 1 at r = 2, so the
    // identity check runs (and fails against the zero quotient claim).
    try std.testing.expectError(
        error.QuotientIdentityMismatch,
        vanishing.verify(dynCancelledSystem(-n), input),
    );
}

const ProofData = struct {
    proof_rounds: []const protocol.RoundMessage,
    public_inputs: []const protocol.Scalar,
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
    const proof_rounds = try buildRounds(allocator, proof, round_cells);
    const public_inputs = try buildScalars(allocator, proof.public_inputs);

    return .{
        .proof_rounds = proof_rounds,
        .public_inputs = public_inputs,
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

fn buildScalars(allocator: std.mem.Allocator, cells: []const fixtures.RuntimeTraceCell) ![]const protocol.Scalar {
    const out = try allocator.alloc(protocol.Scalar, cells.len);
    for (cells, 0..) |cell, i| {
        out[i] = switch (cell) {
            .base => |v| .{ .base = field.Element.init(v) },
            .ext => |v| .{ .ext = ext.Ext.fromUints(v) },
        };
    }
    return out;
}

fn buildRounds(allocator: std.mem.Allocator, proof: fixtures.VanishingProofView, round_cells: []const []const protocol.Scalar) ![]const protocol.RoundMessage {
    const rounds = try allocator.alloc(protocol.RoundMessage, proof.rounds.len);
    for (proof.rounds, 0..) |round, i| {
        rounds[i] = buildRoundMessage(round, round_cells[i]);
    }
    return rounds;
}

fn buildRoundMessage(round: fixtures.RuntimeTraceRound, cells: []const protocol.Scalar) protocol.RoundMessage {
    const commitment = if (round.commitment) |c| commitment_mod.fromUints(c) else null;
    return .{ .commitment = commitment, .cells = cells };
}
