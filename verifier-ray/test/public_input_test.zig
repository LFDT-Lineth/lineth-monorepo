const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const protocol = verifier_ray.protocol;
const public_input = protocol.public_input;

fn base(value: u32) protocol.Scalar {
    return .{ .base = field.Element.init(value) };
}

test "bindRoundMessages injects the separate public-input statement into transcript slots" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{ 3, 1 },
        .refs = &[_]public_input.CellRef{
            .{ .statement_index = 0, .round = 0, .index = 1 },
        },
    };
    const round0_cells = [_]protocol.Scalar{
        base(10),
        base(30),
    };
    const round1_cells = [_]protocol.Scalar{
        base(40),
    };
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &round0_cells },
        .{ .cells = &round1_cells },
    };
    const statement = [_]protocol.Scalar{
        base(20),
    };

    var bound = try public_input.bindRoundMessages(spec, &rounds, &statement);
    const merged = bound.rounds();

    try std.testing.expectEqual(@as(usize, 3), merged[0].cells.len);
    try std.testing.expectEqual(field.Element.init(10), merged[0].cells[0].base);
    try std.testing.expectEqual(field.Element.init(20), merged[0].cells[1].base);
    try std.testing.expectEqual(field.Element.init(30), merged[0].cells[2].base);
    try std.testing.expectEqual(field.Element.init(40), merged[1].cells[0].base);
}

test "bindRoundMessagesRuntime uses one flat bounded backing buffer" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{ 3, 1 },
        .refs = &[_]public_input.CellRef{
            .{ .statement_index = 0, .round = 0, .index = 1 },
        },
    };
    const round0_cells = [_]protocol.Scalar{ base(10), base(30) };
    const round1_cells = [_]protocol.Scalar{base(40)};
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &round0_cells },
        .{ .cells = &round1_cells },
    };
    const statement = [_]protocol.Scalar{base(20)};
    const limits: public_input.Limits = .{
        .round_count = 2,
        .max_cells_per_round = 3,
        .total_cells = 4,
    };
    var bound: public_input.RuntimeBoundRoundMessages(limits) = undefined;

    try public_input.bindRoundMessagesRuntime(limits, spec, &rounds, &statement, &bound);
    const merged = bound.rounds();

    try std.testing.expectEqual(@as(usize, 3), merged[0].cells.len);
    try std.testing.expectEqual(field.Element.init(10), merged[0].cells[0].base);
    try std.testing.expectEqual(field.Element.init(20), merged[0].cells[1].base);
    try std.testing.expectEqual(field.Element.init(30), merged[0].cells[2].base);
    try std.testing.expectEqual(field.Element.init(40), merged[1].cells[0].base);
}

test "bindRoundMessagesRuntime rejects total cells beyond its flat capacity" {
    const spec = public_input.Spec{ .round_cell_counts = &[_]usize{ 2, 2 } };
    const round_cells = [_]protocol.Scalar{ base(1), base(2) };
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &round_cells },
        .{ .cells = &round_cells },
    };
    const limits: public_input.Limits = .{
        .round_count = 2,
        .max_cells_per_round = 2,
        .total_cells = 3,
    };
    var bound: public_input.RuntimeBoundRoundMessages(limits) = undefined;

    try std.testing.expectError(
        error.InvalidSpec,
        public_input.bindRoundMessagesRuntime(limits, spec, &rounds, &.{}, &bound),
    );
}

test "bindRoundMessages rejects wrong public-input length" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{1},
        .refs = &[_]public_input.CellRef{
            .{ .statement_index = 0, .round = 0, .index = 0 },
        },
    };
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &[_]protocol.Scalar{} },
    };

    try std.testing.expectError(
        error.InvalidPublicInputCount,
        public_input.bindRoundMessages(spec, &rounds, &.{}),
    );
}

test "bindRoundMessages rejects proof cells smuggled into a public-input slot" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{2},
        .refs = &[_]public_input.CellRef{
            .{ .statement_index = 0, .round = 0, .index = 0 },
        },
    };
    const smuggled_cells = [_]protocol.Scalar{
        base(11),
        base(22),
    };
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &smuggled_cells },
    };
    const statement = [_]protocol.Scalar{
        base(33),
    };

    try std.testing.expectError(
        error.InvalidRoundCellCount,
        public_input.bindRoundMessages(spec, &rounds, &statement),
    );
}
