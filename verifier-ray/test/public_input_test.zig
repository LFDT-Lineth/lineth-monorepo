const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const protocol = verifier_ray.protocol;
const public_input = protocol.public_input;

fn base(value: u32) protocol.Scalar {
    return .{ .base = field.Element.init(value) };
}

/// The extension-field form of a base value: same value, extension tag.
fn baseAsExt(value: u32) protocol.Scalar {
    return .{ .ext = verifier_ray.field.koalabear_ext.Ext.lift(field.Element.init(value)) };
}

test "bindRoundMessages injects the separate public-input statement into transcript slots" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{ 3, 1 },
        .cell_ext_bits = &[_]u64{0x0},
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

test "bindRoundMessages rejects wrong public-input length" {
    const spec = public_input.Spec{
        .round_cell_counts = &[_]usize{1},
        .cell_ext_bits = &[_]u64{0x0},
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
        .cell_ext_bits = &[_]u64{0x0},
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

// Round 0 holds a base cell, an extension cell and a base public input; round 1
// one extension cell. Flat indices 1 and 3 are extension: bits 0b1010.
const mixed_spec = public_input.Spec{
    .round_cell_counts = &[_]usize{ 3, 1 },
    .cell_ext_bits = &[_]u64{0b1010},
    .refs = &[_]public_input.CellRef{
        .{ .statement_index = 0, .round = 0, .index = 2 },
    },
};

fn bindMixed(round0: [2]protocol.Scalar, round1: protocol.Scalar, statement: protocol.Scalar) public_input.Error!void {
    const rounds = [_]protocol.RoundMessage{
        .{ .cells = &round0 },
        .{ .cells = &[_]protocol.Scalar{round1} },
    };
    _ = try public_input.bindRoundMessages(mixed_spec, &rounds, &[_]protocol.Scalar{statement});
}

test "bindRoundMessages accepts cells carrying their declared field" {
    try bindMixed(.{ base(1), baseAsExt(2) }, baseAsExt(4), base(3));
}

test "bindRoundMessages rejects a base cell re-sent in extension form" {
    try std.testing.expectError(
        error.CellFieldMismatch,
        bindMixed(.{ baseAsExt(1), baseAsExt(2) }, baseAsExt(4), base(3)),
    );
}

test "bindRoundMessages rejects an extension cell sent as base" {
    try std.testing.expectError(
        error.CellFieldMismatch,
        bindMixed(.{ base(1), base(2) }, baseAsExt(4), base(3)),
    );
    try std.testing.expectError(
        error.CellFieldMismatch,
        bindMixed(.{ base(1), baseAsExt(2) }, base(4), base(3)),
    );
}

test "bindRoundMessages rejects a public input not carrying its declared field" {
    try std.testing.expectError(
        error.CellFieldMismatch,
        bindMixed(.{ base(1), baseAsExt(2) }, baseAsExt(4), baseAsExt(3)),
    );
}
