//! Pinned in-memory layout of the proof types.
//!
//! `verifier.Proof` is not parsed — it is cast straight out of the input region
//! (`main.zig`'s `loadR5Input`/`loadNativeInput`), so prover-ray's encoder has to
//! reproduce this module's exact byte layout. See
//! `prover-ray/docs/proof-serialization.md`.
//!
//! Nothing here changes any layout; it asserts the layout the encoder targets, so
//! that a field reordering, a new field, or a Zig upgrade that shifts a slice's
//! representation fails THIS build loudly instead of silently invalidating every
//! encoded proof. A wrong image still casts cleanly and would otherwise surface as
//! an unrelated verification failure, which is the worst way to find out.
//!
//! Zig's `auto` layout is deliberately unspecified — `extern struct` rejects
//! slices outright ("slices have no guaranteed in-memory representation") — so
//! these numbers are observed, not guaranteed. That is exactly why they are
//! asserted rather than assumed.
//!
//! Observed rule (Zig 0.16, identical on aarch64 and riscv64): fields are
//! **stable-sorted by alignment, descending**. Equal alignments keep declaration
//! order, which is why every proof struct made only of slices (all align 8) lays
//! out exactly as declared.
//!
//! CONVENTION for the types pinned here: **declare fields in descending
//! alignment order.** Then declaration order always equals memory order and there
//! is nothing left for the compiler to reorder. Mixing a `[N]Element` (align 4)
//! ahead of a slice (align 8) is what made `merkle.Branch` disagree with its own
//! declaration before it was reordered.
//!
//! Discriminant VALUES are pinned below. Their byte OFFSETS cannot be expressed
//! with `@offsetOf`, so they are pinned by `test/proof_abi_test.zig` instead.

const std = @import("std");

const base = @import("field/koalabear.zig");
const ext = @import("field/koalabear_ext.zig");
const value = @import("field/value.zig");
const merkle = @import("crypto/merkle.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const protocol = @import("protocol/types.zig");
const fri = @import("query/fri.zig");
const pcs = @import("query/pcs.zig");
const verifier = @import("verifier.zig");

/// Asserts a type's size and alignment, naming the actual values on failure.
fn expectSize(comptime T: type, comptime size: usize, comptime alignment: usize) void {
    if (@sizeOf(T) != size) @compileError(std.fmt.comptimePrint(
        "proof ABI drift: @sizeOf({s}) is {d}, pinned at {d} — prover-ray's encoder must be updated in lockstep",
        .{ @typeName(T), @sizeOf(T), size },
    ));
    if (@alignOf(T) != alignment) @compileError(std.fmt.comptimePrint(
        "proof ABI drift: @alignOf({s}) is {d}, pinned at {d}",
        .{ @typeName(T), @alignOf(T), alignment },
    ));
}

/// Asserts a tagged union variant's numeric discriminant. The encoder writes
/// these values as raw bytes, so reordering a union's variants is a wire change.
fn expectTag(comptime T: type, comptime variant: std.meta.Tag(T), comptime tag: usize) void {
    if (@intFromEnum(variant) != tag) @compileError(std.fmt.comptimePrint(
        "proof ABI drift: {s}.{s} has discriminant {d}, pinned at {d} — reordering union variants is a wire change",
        .{ @typeName(T), @tagName(variant), @intFromEnum(variant), tag },
    ));
}

/// Asserts a struct field's byte offset.
fn expectField(comptime T: type, comptime name: []const u8, comptime offset: usize) void {
    if (@offsetOf(T, name) != offset) @compileError(std.fmt.comptimePrint(
        "proof ABI drift: @offsetOf({s}, \"{s}\") is {d}, pinned at {d}",
        .{ @typeName(T), name, @offsetOf(T, name), offset },
    ));
}

comptime {
    // ---- primitives ----------------------------------------------------------
    // A slice is two words, {ptr, len}, with no capacity field. The encoder lays
    // each payload out directly behind its header on the strength of this.
    expectSize([]const u8, 16, 8);
    expectSize(base.Element, 4, 4);
    expectSize(ext.Ext, 24, 4);
    expectField(ext.Ext, "B0", 0);
    expectField(ext.Ext, "B1", 8);
    expectField(ext.Ext, "B2", 16);
    expectSize(poseidon2.Digest, 32, 4);

    // ---- round messages ------------------------------------------------------
    expectSize(protocol.RoundMessage, 32, 8);
    expectField(protocol.RoundMessage, "columns", 0);
    expectField(protocol.RoundMessage, "cells", 16);

    // ---- PCS input-tree openings --------------------------------------------
    expectSize(merkle.RowOpening, 32, 8);
    expectField(merkle.RowOpening, "base", 0);
    expectField(merkle.RowOpening, "ext", 16);

    expectSize(merkle.RowPair, 64, 8);

    expectSize(merkle.InputTreeOpening, 32, 8);
    expectField(merkle.InputTreeOpening, "siblings", 0);
    expectField(merkle.InputTreeOpening, "leaves", 16);

    // ---- running-layer branches ---------------------------------------------
    // Declared align-descending (slice before digest array), so these offsets
    // match the declaration. Swapping the two fields back would silently keep
    // this same layout while making the source read the other way round.
    expectSize(merkle.Branch, 48, 8);
    expectField(merkle.Branch, "siblings", 0);
    expectField(merkle.Branch, "leaf", 16);

    // ---- FRI / PCS proof -----------------------------------------------------
    expectSize(fri.Proof, 48, 8);
    expectField(fri.Proof, "round_roots", 0);
    expectField(fri.Proof, "final_poly", 16);
    expectField(fri.Proof, "running_queries", 32);

    expectSize(pcs.OpeningProof, 64, 8);
    expectField(pcs.OpeningProof, "input_queries", 0);
    expectField(pcs.OpeningProof, "fri_proof", 16);

    expectSize(verifier.PcsOpening, 80, 8);
    expectField(verifier.PcsOpening, "entry_claims", 0);
    expectField(verifier.PcsOpening, "proof", 16);

    // ---- root ----------------------------------------------------------------
    // Must stay at image offset 0: the loaders cast the input region's base
    // address directly to *const Proof.
    expectSize(verifier.Proof, 112, 8);
    expectField(verifier.Proof, "rounds", 0);
    expectField(verifier.Proof, "module_sizes", 16);
    expectField(verifier.Proof, "pcs_opening", 32);

    // ---- tagged unions and optionals ----------------------------------------
    // Sizes and discriminant values here; discriminant byte offsets, which
    // @offsetOf cannot express, in test/proof_abi_test.zig.
    expectSize(value.Scalar, 28, 4);
    expectTag(value.Scalar, .base, 0);
    expectTag(value.Scalar, .ext, 1);

    expectSize(value.Vector, 24, 8);
    expectTag(value.Vector, .base, 0);
    expectTag(value.Vector, .ext, 1);

    expectSize(protocol.ColumnMessage, 40, 8);
    expectTag(protocol.ColumnMessage, .oracle_commitment, 0);
    expectTag(protocol.ColumnMessage, .public_column, 1);

    expectSize(?merkle.RowPair, 72, 8);
}
