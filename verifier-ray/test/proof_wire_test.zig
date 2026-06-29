const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const proof_wire = verifier_ray.proof_wire;

test "proof wire decodes valid proof claims without allocation" {
    const encoded = [_]u8{
        1, 0, 0, 0, // round_count
        1, 0, 0, 0, // round 0 column_count
        1, // column tag: public base
        2, 0, 0, 0, // public base length
        9, 0, 0, 0, // public base value 0
        10, 0, 0, 0, // public base value 1
        0, 0, 0, 0, // round 0 cell_count
        1, 0, 0, 0, // witness_claim_count
        1, 0, 0, 0, // witness_claim_0: ext claim with 6 u32 limbs
        2, 0, 0, 0,
        3, 0, 0, 0,
        4, 0, 0, 0,
        5, 0, 0, 0,
        6, 0, 0, 0,
        0, 0, 0, 0, // quotient_claim_count
        0, 0, 0, 0, // module_size_count
    };
    const layout = comptime layoutWithEncodedSize(.{
        .round_count = 1,
        .column_count = 1,
        .cell_count = 0,
        .public_base_value_count = 2,
        .public_ext_value_count = 0,
        .witness_claim_count = 1,
        .quotient_claim_count = 0,
        .module_size_count = 0,
        .encoded_size = 0,
    }, &encoded);
    var backing: proof_wire.Backing(layout) = .{};

    const proof = try backing.decode(&encoded);

    try std.testing.expectEqual(layout.encoded_size, encoded.len);
    try std.testing.expectEqual(@as(usize, 1), proof.rounds.len);
    try std.testing.expectEqual(@as(usize, 1), proof.rounds[0].columns.len);
    try std.testing.expectEqual(@as(usize, 0), proof.rounds[0].cells.len);
    const public_column = proof.rounds[0].columns[0].public_column;
    try std.testing.expectEqual(@as(usize, 2), public_column.base.len);
    try std.testing.expect(public_column.base[0].eql(field.Element.init(9)));
    try std.testing.expect(public_column.base[1].eql(field.Element.init(10)));
    try std.testing.expectEqual(@as(usize, 1), proof.witness_claims.len);
    try std.testing.expect(proof.witness_claims[0].eql(ext.Ext.fromUints(.{ 1, 2, 3, 4, 5, 6 })));
    try std.testing.expectEqual(@as(usize, 0), proof.quotient_claims.len);
    try std.testing.expectEqual(@as(usize, 0), proof.module_sizes.len);
}

test "proof wire rejects trailing input" {
    const encoded = [_]u8{
        0, 0, 0, 0, // round_count
        0, 0, 0, 0, // witness_claim_count
        0, 0, 0, 0, // quotient_claim_count
        0, 0, 0, 0, // module_size_count
        0,
    };
    const layout = comptime layoutWithEncodedSize(.{
        .round_count = 0,
        .column_count = 0,
        .cell_count = 0,
        .public_base_value_count = 0,
        .public_ext_value_count = 0,
        .witness_claim_count = 0,
        .quotient_claim_count = 0,
        .module_size_count = 0,
        .encoded_size = 0,
    }, &encoded);
    var backing: proof_wire.Backing(layout) = .{};

    try std.testing.expectError(error.TrailingInput, backing.decode(&encoded));
}

// layoutWithEncodedSize computes the encoded_size from the encoded input
fn layoutWithEncodedSize(comptime layout: proof_wire.Layout, comptime encoded: []const u8) proof_wire.Layout {
    var out = layout;
    out.encoded_size = encoded.len;
    return out;
}
