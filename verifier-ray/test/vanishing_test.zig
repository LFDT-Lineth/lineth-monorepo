const std = @import("std");
const verifier_ray = @import("verifier_ray");
const fixtures = @import("test_vanishing");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const runtime = verifier_ray.runtime;
const vanishing = verifier_ray.query.vanishing;

test "vanishing quotient honest scenarios match prover-ray" {
    try std.testing.expect(fixtures.scenarios.len > 0);
    inline for (fixtures.scenarios) |case| {
        const system = case.system;
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const input = try proofInput(arena.allocator(), case.honest);
        try vanishing.verify(system, input);
    }
}

test "vanishing quotient invalid scenarios fail identity" {
    var invalid_case_count: usize = 0;
    inline for (fixtures.scenarios) |case| {
        const invalid = case.invalid orelse continue;
        invalid_case_count += 1;
        const system = case.system;
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const input = try proofInput(arena.allocator(), invalid);
        try std.testing.expectError(error.QuotientIdentityMismatch, vanishing.verify(system, input));
    }
    try std.testing.expect(invalid_case_count > 0);
}

test "dynamic vanishing module sizes are required and validated" {
    comptime var dynamic_case_count: usize = 0;
    var wrong_size_failures: usize = 0;
    inline for (fixtures.scenarios) |case| {
        const system = case.system;
        if (system.dynamic_module_count == 0) continue;
        dynamic_case_count += 1;

        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();

        const valid = try proofInput(arena.allocator(), case.honest);
        try vanishing.verify(system, valid);

        var missing = valid;
        missing.module_sizes = &.{};
        try std.testing.expectError(error.MissingDynamicModuleSize, vanishing.verify(system, missing));

        var zero_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        zero_sizes[0] = 0;
        var zero = valid;
        zero.module_sizes = zero_sizes;
        try std.testing.expectError(error.InvalidModuleSize, vanishing.verify(system, zero));

        var non_power_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        non_power_sizes[0] = 7;
        var non_power = valid;
        non_power.module_sizes = non_power_sizes;
        try std.testing.expectError(error.InvalidModuleSize, vanishing.verify(system, non_power));

        var wrong_sizes = try arena.allocator().dupe(usize, valid.module_sizes);
        wrong_sizes[0] = if (wrong_sizes[0] == 16) 8 else 16;
        var wrong = valid;
        wrong.module_sizes = wrong_sizes;
        vanishing.verify(system, wrong) catch |err| {
            if (err == error.QuotientIdentityMismatch) wrong_size_failures += 1 else return err;
        };
    }
    try std.testing.expect(dynamic_case_count > 0);
    try std.testing.expect(wrong_size_failures > 0);
}

fn proofInput(allocator: std.mem.Allocator, proof: fixtures.VanishingProofView) !vanishing.CheckInput {
    const witness_claims = try allocator.alloc(ext.Ext, proof.witness_claims.len);
    for (proof.witness_claims, 0..) |claim, i| {
        witness_claims[i] = uintsToExt(claim);
    }

    const quotient_claims = try allocator.alloc(ext.Ext, proof.quotient_claims.len);
    for (proof.quotient_claims, 0..) |claim, i| {
        quotient_claims[i] = uintsToExt(claim);
    }

    const rounds = try allocator.alloc(runtime.RoundMessage, proof.rounds.len);
    for (proof.rounds, 0..) |round, i| {
        rounds[i] = try roundMessage(allocator, round);
    }

    return .{
        .rounds = rounds,
        .witness_claims = witness_claims,
        .quotient_claims = quotient_claims,
        .module_sizes = proof.module_sizes,
    };
}

fn roundMessage(allocator: std.mem.Allocator, round: fixtures.RuntimeTraceRound) !runtime.RoundMessage {
    var column_count: usize = 0;
    for (round.columns) |column| {
        column_count += switch (column) {
            .oracle => |commitments| commitments.len,
            .public_base, .public_ext => 1,
        };
    }

    const columns = try allocator.alloc(runtime.ColumnMessage, column_count);
    var next_column: usize = 0;
    for (round.columns) |column| {
        switch (column) {
            .oracle => |commitments| {
                for (commitments) |commitment| {
                    columns[next_column] = .{ .oracle_commitment = uintsToCommitment(commitment) };
                    next_column += 1;
                }
            },
            .public_base => |values| {
                const converted = try allocator.alloc(field.Element, values.len);
                for (values, 0..) |value, i| {
                    converted[i] = field.Element.init(value);
                }
                columns[next_column] = .{ .public_column = .{ .base = converted } };
                next_column += 1;
            },
            .public_ext => |values| {
                const converted = try allocator.alloc(ext.Ext, values.len);
                for (values, 0..) |value, i| {
                    converted[i] = uintsToExt(value);
                }
                columns[next_column] = .{ .public_column = .{ .ext = converted } };
                next_column += 1;
            },
        }
    }

    const cells = try allocator.alloc(runtime.Scalar, round.cells.len);
    for (round.cells, 0..) |cell, i| {
        cells[i] = switch (cell) {
            .base => |value| .{ .base = field.Element.init(value) },
            .ext => |value| .{ .ext = uintsToExt(value) },
        };
    }

    return .{ .columns = columns, .cells = cells };
}

fn uintsToExt(limbs: [6]u32) ext.Ext {
    return ext.Ext.fromUints(limbs[0], limbs[1], limbs[2], limbs[3], limbs[4], limbs[5]);
}

fn uintsToCommitment(limbs: [8]u32) runtime.Commitment {
    var out: runtime.Commitment = undefined;
    for (&out, limbs) |*dst, limb| {
        dst.* = field.Element.init(limb);
    }
    return out;
}
