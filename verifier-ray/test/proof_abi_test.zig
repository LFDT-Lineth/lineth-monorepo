//! Pins the discriminant placement of the proof types' tagged unions and
//! optionals, which `@offsetOf` cannot express and which `src/proof_abi.zig`
//! therefore cannot assert at comptime.
//!
//! prover-ray's encoder writes these discriminants by hand, so their byte offset
//! is part of the wire contract. Their numeric values are pinned at comptime in
//! `src/proof_abi.zig`. See `prover-ray/docs/proof-serialization.md`.
//!
//! The checks read only the discriminant byte and compare it against the value's
//! own active tag. They deliberately avoid diffing whole values: Zig does not
//! define what assignment writes to a union's padding, so a byte-level diff of
//! two variants is not reproducible.

const std = @import("std");
const verifier_ray = @import("verifier_ray");

const value = verifier_ray.field.value;
const base = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const merkle = verifier_ray.crypto.merkle;
const commitment = verifier_ray.crypto.commitment;
const protocol = verifier_ray.protocol;

/// Asserts that the byte at `offset` holds `v`'s active discriminant.
///
/// Called with two variants whose tags differ, this locates the discriminant:
/// a byte that tracks the active tag across distinct variants is the tag, and
/// `@sizeOf` (pinned in `proof_abi.zig`) fixes the rest of the layout around it.
fn expectTagByte(comptime T: type, v: T, offset: usize) !void {
    const want: u8 = @intFromEnum(std.meta.activeTag(v));
    const got = std.mem.asBytes(&v)[offset];
    try std.testing.expectEqual(want, got);
}

test "Scalar discriminant byte is at offset 24" {
    try expectTagByte(value.Scalar, .{ .base = base.Element.zero() }, 24);
    try expectTagByte(value.Scalar, .{ .ext = ext.Ext.zero() }, 24);
}

test "Vector discriminant byte is at offset 16" {
    try expectTagByte(value.Vector, .{ .base = &.{} }, 16);
    try expectTagByte(value.Vector, .{ .ext = &.{} }, 16);
}

test "ColumnMessage discriminant byte is at offset 32" {
    try expectTagByte(
        protocol.ColumnMessage,
        .{ .oracle_commitment = std.mem.zeroes(commitment.Commitment) },
        32,
    );
    try expectTagByte(
        protocol.ColumnMessage,
        .{ .public_column = .{ .base = &.{} } },
        32,
    );
}

test "optional RowPair has_value byte is at offset 64" {
    const empty = merkle.RowOpening{ .base = &.{}, .ext = &.{} };

    const absent: ?merkle.RowPair = null;
    try std.testing.expectEqual(@as(u8, 0), std.mem.asBytes(&absent)[64]);

    const present: ?merkle.RowPair = .{ empty, empty };
    try std.testing.expectEqual(@as(u8, 1), std.mem.asBytes(&present)[64]);
}

test "empty slices carry a non-null pointer" {
    // The encoder must reproduce this: a null pointer in a []const T is UB even
    // at length zero, so an all-zero slice header is not a valid empty slice.
    const empty: []const base.Element = &.{};
    try std.testing.expectEqual(@as(usize, 0), empty.len);
    try std.testing.expect(@intFromPtr(empty.ptr) != 0);
}

test "proof ABI comptime assertions are analyzed" {
    // src/proof_abi.zig is assertion-only; referencing it forces its comptime
    // block to run as part of the test build as well as the library build.
    _ = verifier_ray.proof_abi;
}
