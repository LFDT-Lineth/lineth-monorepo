const protocol = @import("protocol/root.zig");
const verifier = @import("verifier.zig");
const field = @import("field/koalabear.zig");
const ext = @import("field/koalabear_ext.zig");
const commitment = @import("crypto/commitment.zig");

pub const Error = error{
    InvalidRoundCount,
    InvalidColumnCount,
    InvalidCellCount,
    InvalidWitnessClaimCount,
    InvalidQuotientClaimCount,
    InvalidModuleSizeCount,
    InvalidColumnTag,
    InvalidScalarTag,
    InvalidPublicBaseValueCount,
    InvalidPublicExtValueCount,
    TrailingInput,
    UnexpectedEndOfInput,
};

/// Layout describes the exact shape of one serialized proof input. The Go
/// generator emits one value per verifier fixture case, and `Backing(layout)`
/// uses it to allocate fixed arrays at comptime instead of using an allocator
/// while decoding runtime input bytes.
pub const Layout = struct {
    round_count: usize,
    column_count: usize,
    cell_count: usize,
    public_base_value_count: usize,
    public_ext_value_count: usize,
    witness_claim_count: usize,
    quotient_claim_count: usize,
    module_size_count: usize,
    encoded_size: usize,
};

const column_oracle_commitment: u8 = 0;
const column_public_base: u8 = 1;
const column_public_ext: u8 = 2;

const scalar_base: u8 = 0;
const scalar_ext: u8 = 1;

/// Backing returns a case-specialized storage type for a proof-wire input. The
/// decoded `verifier.Proof` contains slices into this storage, so the backing
/// value must outlive the verifier call.
pub fn Backing(comptime layout: Layout) type {
    return struct {
        const Self = @This();

        proof: verifier.Proof = undefined,
        rounds: [layout.round_count]protocol.RoundMessage = undefined,
        columns: [layout.column_count]protocol.ColumnMessage = undefined,
        cells: [layout.cell_count]protocol.Scalar = undefined,
        public_base_values: [layout.public_base_value_count]field.Element = undefined,
        public_ext_values: [layout.public_ext_value_count]ext.Ext = undefined,
        witness_claims: [layout.witness_claim_count]ext.Ext = undefined,
        quotient_claims: [layout.quotient_claim_count]ext.Ext = undefined,
        module_sizes: [layout.module_size_count]usize = undefined,

        /// decode parses one proof-wire byte slice into this backing storage
        /// and returns a verifier.Proof view over the populated arrays.
        pub fn decode(self: *Self, encoded: []const u8) Error!*const verifier.Proof {
            if (encoded.len != layout.encoded_size) return error.UnexpectedEndOfInput;

            var reader = Reader{ .input = encoded };

            const round_count = try reader.readLen();
            if (round_count != self.rounds.len) return error.InvalidRoundCount;

            var column_offset: usize = 0;
            var cell_offset: usize = 0;
            var public_base_offset: usize = 0;
            var public_ext_offset: usize = 0;

            for (&self.rounds) |*round| {
                const column_start = column_offset;
                const round_column_count = try reader.readLen();
                if (column_offset + round_column_count > self.columns.len) return error.InvalidColumnCount;
                for (self.columns[column_offset..][0..round_column_count]) |*column| {
                    column.* = try self.readColumn(&reader, &public_base_offset, &public_ext_offset);
                }
                column_offset += round_column_count;

                const cell_start = cell_offset;
                const round_cell_count = try reader.readLen();
                if (cell_offset + round_cell_count > self.cells.len) return error.InvalidCellCount;
                for (self.cells[cell_offset..][0..round_cell_count]) |*cell| {
                    cell.* = try reader.readScalar();
                }
                cell_offset += round_cell_count;

                round.* = .{
                    .columns = self.columns[column_start..column_offset],
                    .cells = self.cells[cell_start..cell_offset],
                };
            }

            if (column_offset != self.columns.len) return error.InvalidColumnCount;
            if (cell_offset != self.cells.len) return error.InvalidCellCount;
            if (public_base_offset != self.public_base_values.len) return error.InvalidPublicBaseValueCount;
            if (public_ext_offset != self.public_ext_values.len) return error.InvalidPublicExtValueCount;

            const witness_claim_count = try reader.readLen();
            if (witness_claim_count != self.witness_claims.len) return error.InvalidWitnessClaimCount;
            for (&self.witness_claims) |*claim| {
                claim.* = try reader.readExt();
            }

            const quotient_claim_count = try reader.readLen();
            if (quotient_claim_count != self.quotient_claims.len) return error.InvalidQuotientClaimCount;
            for (&self.quotient_claims) |*claim| {
                claim.* = try reader.readExt();
            }

            const module_size_count = try reader.readLen();
            if (module_size_count != self.module_sizes.len) return error.InvalidModuleSizeCount;
            for (&self.module_sizes) |*size| {
                size.* = try reader.readLen();
            }

            if (reader.remaining() != 0) return error.TrailingInput;

            self.proof = .{
                .rounds = &self.rounds,
                .witness_claims = &self.witness_claims,
                .quotient_claims = &self.quotient_claims,
                .module_sizes = &self.module_sizes,
            };
            return &self.proof;
        }

        fn readColumn(
            self: *Self,
            reader: *Reader,
            public_base_offset: *usize,
            public_ext_offset: *usize,
        ) Error!protocol.ColumnMessage {
            return switch (try reader.readU8()) {
                column_oracle_commitment => .{ .oracle_commitment = try reader.readCommitment() },
                column_public_base => blk: {
                    const len = try reader.readLen();
                    if (public_base_offset.* + len > self.public_base_values.len) return error.InvalidPublicBaseValueCount;
                    const start = public_base_offset.*;
                    for (self.public_base_values[start..][0..len]) |*value| {
                        value.* = try reader.readField();
                    }
                    public_base_offset.* += len;
                    break :blk .{ .public_column = .{ .base = self.public_base_values[start..public_base_offset.*] } };
                },
                column_public_ext => blk: {
                    const len = try reader.readLen();
                    if (public_ext_offset.* + len > self.public_ext_values.len) return error.InvalidPublicExtValueCount;
                    const start = public_ext_offset.*;
                    for (self.public_ext_values[start..][0..len]) |*value| {
                        value.* = try reader.readExt();
                    }
                    public_ext_offset.* += len;
                    break :blk .{ .public_column = .{ .ext = self.public_ext_values[start..public_ext_offset.*] } };
                },
                else => error.InvalidColumnTag,
            };
        }
    };
}

const Reader = struct {
    input: []const u8,
    offset: usize = 0,

    fn remaining(self: Reader) usize {
        return self.input.len - self.offset;
    }

    fn readU8(self: *Reader) Error!u8 {
        if (self.remaining() < 1) return error.UnexpectedEndOfInput;
        const value = self.input[self.offset];
        self.offset += 1;
        return value;
    }

    fn readU32(self: *Reader) Error!u32 {
        if (self.remaining() < 4) return error.UnexpectedEndOfInput;
        const bytes = self.input[self.offset..][0..4];
        self.offset += 4;
        return @as(u32, bytes[0]) |
            (@as(u32, bytes[1]) << 8) |
            (@as(u32, bytes[2]) << 16) |
            (@as(u32, bytes[3]) << 24);
    }

    fn readLen(self: *Reader) Error!usize {
        return @intCast(try self.readU32());
    }

    fn readField(self: *Reader) Error!field.Element {
        return field.Element.init(try self.readU32());
    }

    fn readExt(self: *Reader) Error!ext.Ext {
        return ext.Ext.fromUints(.{
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
        });
    }

    fn readCommitment(self: *Reader) Error!commitment.Commitment {
        return commitment.fromUints(.{
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
            try self.readU32(),
        });
    }

    fn readScalar(self: *Reader) Error!protocol.Scalar {
        return switch (try self.readU8()) {
            scalar_base => .{ .base = try self.readField() },
            scalar_ext => .{ .ext = try self.readExt() },
            else => error.InvalidScalarTag,
        };
    }
};
