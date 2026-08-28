//! Pins the discriminant placement of the proof types' tagged unions and
//! optionals, which `@offsetOf` cannot express and which `src/proof_abi.zig`
//! therefore cannot assert at comptime.
//!
//! prover-ray's encoder writes these discriminants by hand, so their byte offset
//! is part of the wire contract. Their numeric values are pinned at comptime in
//! `src/proof_abi.zig`. See `prover-ray/wiop/proofserialization/README.md`.
//!
//! The checks read only the discriminant byte and compare it against the value's
//! own active tag. They deliberately avoid diffing whole values: Zig does not
//! define what assignment writes to a union's padding, so a byte-level diff of
//! two variants is not reproducible.
//!
//! On failure these print a full diagnosis rather than "expected 1, found 12" —
//! whoever trips this will not have the serialization format in mind.

const std = @import("std");
const verifier_ray = @import("verifier_ray");

const value = verifier_ray.field.value;
const base = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const merkle = verifier_ray.crypto.merkle;
const commitment = verifier_ray.crypto.commitment;
const protocol = verifier_ray.protocol;

const Failure = error{ProofAbiDrift};

/// Explains a discriminant that moved, then fails.
fn reportTagDrift(
    comptime T: type,
    variant_name: []const u8,
    offset: usize,
    want: u8,
    got: u8,
    bytes: []const u8,
) Failure {
    std.debug.print(
        \\
        \\================================================================
        \\PROOF ABI DRIFT: {s} discriminant is not where the format expects
        \\================================================================
        \\
        \\  Variant .{s} should put its discriminant byte at offset {d}, holding
        \\  the value {d}. The byte at offset {d} is actually {d}.
        \\
        \\  Raw bytes of the value ({d} bytes total):
        \\    {x}
        \\
        \\WHY THIS FIRED
        \\  Zig does not specify where a tagged union or optional keeps its
        \\  discriminant. It is placed after the payload today, but adding a
        \\  variant, changing a payload's size or alignment, or upgrading Zig can
        \\  all move it.
        \\
        \\WHAT IT BREAKS
        \\  prover-ray writes this discriminant into the proof image by hand, at a
        \\  hardcoded offset. verifier-ray casts the image straight to a proof —
        \\  no parsing, no runtime check. If the offset moves, the verifier reads
        \\  the discriminant out of payload bytes: it silently picks the wrong
        \\  variant, e.g. treating a base field element as an extension element,
        \\  and fails somewhere unrelated. This test exists to catch that here.
        \\
        \\HOW TO FIX
        \\  a) Did you change {s} on purpose? Then update, in the same change:
        \\       - the expected offset in test/proof_abi_test.zig (this file),
        \\       - the size and tag pins in src/proof_abi.zig,
        \\       - prover-ray's proof encoder, which writes the discriminant,
        \\       - prover-ray/wiop/proofserialization/README.md section 6 (layout table).
        \\     Shipping the type change alone silently invalidates every proof
        \\     produced afterwards.
        \\
        \\  b) Did you not mean to? The usual cause is a new variant whose payload
        \\     is larger than the existing ones, which grows the union and pushes
        \\     the discriminant back. Check what changed about {s}'s payloads.
        \\
        \\
    , .{
        @typeName(T), variant_name, offset,    want,
        offset,       got,          bytes.len, bytes,
        @typeName(T), @typeName(T),
    });
    return Failure.ProofAbiDrift;
}

/// Asserts that the byte at `offset` holds `v`'s active discriminant.
///
/// Called with two variants whose tags differ, this locates the discriminant:
/// a byte that tracks the active tag across distinct variants is the tag, and
/// `@sizeOf` (pinned in `proof_abi.zig`) fixes the rest of the layout around it.
fn expectTagByte(comptime T: type, v: T, offset: usize) !void {
    const want: u8 = @intFromEnum(std.meta.activeTag(v));
    const bytes = std.mem.asBytes(&v);
    const got = bytes[offset];
    if (got != want) {
        return reportTagDrift(T, @tagName(std.meta.activeTag(v)), offset, want, got, bytes);
    }
}

test "Scalar discriminant byte is at offset 24" {
    try expectTagByte(value.Scalar, .{ .base = base.Element.zero() }, 24);
    try expectTagByte(value.Scalar, .{ .ext = ext.Ext.zero() }, 24);
}

test "optional Commitment has_value byte is at offset 32" {
    // A committed round carries only its Merkle root, as an optional rather than
    // a column union, so this flag is what the encoder writes per round.
    const absent: ?commitment.Commitment = null;
    try std.testing.expectEqual(@as(u8, 0), std.mem.asBytes(&absent)[32]);

    const present: ?commitment.Commitment = std.mem.zeroes(commitment.Commitment);
    try std.testing.expectEqual(@as(u8, 1), std.mem.asBytes(&present)[32]);
}

test "optional RowPair has_value byte is at offset 64" {
    const empty = merkle.RowOpening{ .base = &.{}, .ext = &.{} };

    const absent: ?merkle.RowPair = null;
    const absent_byte = std.mem.asBytes(&absent)[64];
    if (absent_byte != 0) {
        return reportTagDrift(?merkle.RowPair, "null", 64, 0, absent_byte, std.mem.asBytes(&absent));
    }

    const present: ?merkle.RowPair = .{ empty, empty };
    const present_byte = std.mem.asBytes(&present)[64];
    if (present_byte != 1) {
        return reportTagDrift(?merkle.RowPair, "non-null", 64, 1, present_byte, std.mem.asBytes(&present));
    }
}

test "empty slices carry a non-null pointer" {
    const empty: []const base.Element = &.{};
    try std.testing.expectEqual(@as(usize, 0), empty.len);
    if (@intFromPtr(empty.ptr) == 0) {
        std.debug.print(
            \\
            \\================================================================
            \\PROOF ABI DRIFT: an empty slice now has a null pointer
            \\================================================================
            \\
            \\WHY THIS MATTERS
            \\  prover-ray's encoder writes slice headers by hand. A []const T holds
            \\  a non-optional [*]const T, so a null pointer is undefined behaviour
            \\  even at length zero. The encoder therefore emits a non-null dummy
            \\  pointer for empty slices, mirroring what Zig itself does.
            \\
            \\  This test asserts that Zig still does that. If it now emits a null
            \\  pointer for empty slice literals, the encoder's convention should be
            \\  revisited to match — see the determinism rules in
            \\  prover-ray/wiop/proofserialization/README.md section 9.
            \\
            \\
        , .{});
        return Failure.ProofAbiDrift;
    }
}

test "proof ABI comptime assertions are analyzed" {
    // src/proof_abi.zig is assertion-only; referencing it forces its comptime
    // block to run as part of the test build as well as the library build.
    _ = verifier_ray.proof_abi;
}
