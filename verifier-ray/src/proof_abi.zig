//! Pinned in-memory layout of the proof types.
//!
//! `verifier.Proof` is not parsed — it is cast straight out of the input region
//! (`main.zig`'s `loadR5Input`/`loadNativeInput`), so prover-ray's encoder has to
//! reproduce this module's exact byte layout. See
//! `prover-ray/wiop/proofserialization/README.md`.
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

// ============================================================================
// Failure diagnostics
//
// These assertions fire on people who did not write them and who have no reason
// to know what a "proof image" is. So a failure has to explain what broke, why
// it broke, what it would break downstream, and exactly what to do — including
// the field order to use. Terse "expected X, got Y" is not enough here.
// ============================================================================

/// The shared explanation attached to every drift message.
const why =
    \\
    \\WHY THIS FIRED
    \\  Zig's `auto` struct layout stable-sorts fields by alignment, descending
    \\  (equal alignments keep declaration order). So ANY of these moves offsets:
    \\    - adding a field, removing one, or changing a field's type
    \\    - reordering the declaration
    \\    - declaring a lower-alignment field before a higher-alignment one
    \\    - a Zig upgrade that changes the layout rule or a slice's representation
    \\
    \\WHAT IT BREAKS
    \\  prover-ray serializes a proof by writing these exact byte offsets, and the
    \\  verifier casts its input region straight to *const Proof — there is no
    \\  parsing step and no runtime check. So a layout change does NOT fail fast:
    \\  the cast still succeeds, the verifier reads misaligned garbage, and you get
    \\  an unrelated-looking failure somewhere deep in verification. This assertion
    \\  exists to convert that into the build error you are reading now.
    \\
;

/// Lists the type's fields in actual memory order, as
/// "name: align A, size S -> offset O".
///
/// Deliberately reports offset order rather than a re-derived "ideal" order: the
/// compiler lays fields out align-descending, so memory order is ALWAYS a valid
/// align-descending declaration order. Re-deriving one would be a guess, and for
/// equal alignments it could only echo whatever is declared now — which, when
/// that declaration is the thing that broke, would be advice to cement the bug.
fn layoutInMemoryOrder(comptime T: type) []const u8 {
    const fields = @typeInfo(T).@"struct".fields;
    if (fields.len == 0) return "      (no fields)\n";

    comptime var order: [fields.len]usize = undefined;
    for (0..fields.len) |i| order[i] = i;
    comptime var i: usize = 1;
    inline while (i < fields.len) : (i += 1) {
        comptime var j: usize = i;
        inline while (j > 0 and
            @offsetOf(T, fields[order[j - 1]].name) > @offsetOf(T, fields[order[j]].name)) : (j -= 1)
        {
            const tmp = order[j - 1];
            order[j - 1] = order[j];
            order[j] = tmp;
        }
    }

    comptime var out: []const u8 = "";
    inline for (order) |k| {
        const f = fields[k];
        out = out ++ std.fmt.comptimePrint(
            "      {s}: align {d}, size {d}  -> offset {d}\n",
            .{ f.name, @alignOf(f.type), @sizeOf(f.type), @offsetOf(T, f.name) },
        );
    }
    return out;
}

/// True when every field shares one alignment, so the compiler's align-descending
/// sort is a no-op and memory order is exactly declaration order. That changes the
/// advice: there is no "declare it align-descending" fix, only "put the fields
/// back in the order the format expects".
fn allFieldsSameAlignment(comptime T: type) bool {
    const fields = @typeInfo(T).@"struct".fields;
    if (fields.len < 2) return true;
    inline for (fields) |f| {
        if (@alignOf(f.type) != @alignOf(fields[0].type)) return false;
    }
    return true;
}

/// Shows the current layout, labelled as current — never as a target.
fn currentLayoutOf(comptime T: type) []const u8 {
    if (@typeInfo(T) != .@"struct") return "";
    return "\n     CURRENT layout of " ++ @typeName(T) ++ " (what you have right now):\n\n" ++
        layoutInMemoryOrder(T);
}

/// The reorder-specific advice, which depends on whether alignments differ.
fn orderingAdvice(comptime T: type) []const u8 {
    if (allFieldsSameAlignment(T)) {
        return "     Every field here has the SAME alignment, so the compiler does not\n" ++
            "     reorder anything: memory order is exactly your declaration order.\n" ++
            "     So this is a plain declaration reorder (or an inserted field) —\n" ++
            "     move the fields back so each lands on its pinned offset.\n";
    }
    return "     Fields here have MIXED alignments and the compiler sorts them\n" ++
        "     align-descending, so declaring a lower-alignment field ahead of a\n" ++
        "     higher-alignment one silently moves both. Declare them in the memory\n" ++
        "     order shown above and source order will match the layout.\n";
}

/// Asserts a type's size and alignment.
fn expectSize(comptime T: type, comptime size: usize, comptime alignment: usize) void {
    if (@sizeOf(T) != size) {
        @compileError(std.fmt.comptimePrint(
            "\n\nPROOF ABI DRIFT: @sizeOf({s}) is {d}, but the proof format is pinned to {d}.\n" ++
                why ++
                "\nHOW TO FIX\n" ++
                "  A size change almost always means A FIELD WAS ADDED (or removed, or\n" ++
                "  retyped). That is why this fired. Decide which applies:\n" ++
                "\n" ++
                "  a) Did you mean to change {s}? Then this file is not the only place to\n" ++
                "     update. In the SAME change, also update:\n" ++
                "       - the pinned {d} below,\n" ++
                "       - prover-ray's proof encoder (it writes these offsets by hand),\n" ++
                "       - prover-ray/wiop/proofserialization/README.md section 6 (layout table).\n" ++
                "     Shipping the struct change alone silently invalidates every proof\n" ++
                "     produced afterwards.\n" ++
                "\n" ++
                "  b) Did you not mean to? Revert the field change.\n" ++
                currentLayoutOf(T),
            .{ @typeName(T), @sizeOf(T), size, @typeName(T), size },
        ));
    }
    if (@alignOf(T) != alignment) {
        @compileError(std.fmt.comptimePrint(
            "\n\nPROOF ABI DRIFT: @alignOf({s}) is {d}, but the proof format is pinned to {d}.\n" ++
                why ++
                "\nHOW TO FIX\n" ++
                "  Alignment changes when the widest field changes. The encoder pads to\n" ++
                "  {d}; at {d} every following field in the image is misplaced. Either\n" ++
                "  revert the field change, or update this pin, prover-ray's encoder and\n" ++
                "  prover-ray/wiop/proofserialization/README.md section 6 together.\n",
            .{ @typeName(T), @alignOf(T), alignment, alignment, @alignOf(T) },
        ));
    }
}

/// Asserts a struct field's byte offset.
fn expectField(comptime T: type, comptime name: []const u8, comptime offset: usize) void {
    if (@offsetOf(T, name) != offset) {
        @compileError(std.fmt.comptimePrint(
            "\n\nPROOF ABI DRIFT: field \"{s}\" of {s} is at byte offset {d}, but the\n" ++
                "proof format is pinned to offset {d}. The encoder will write \"{s}\" at {d}\n" ++
                "while the verifier reads it from {d}.\n" ++
                why ++
                "\nHOW TO FIX\n" ++
                "  a) Did you mean to change {s}? Then this file is not the only place to\n" ++
                "     update. In the SAME change, also update:\n" ++
                "       - the pinned offset below,\n" ++
                "       - prover-ray's proof encoder (it writes these offsets by hand),\n" ++
                "       - prover-ray/wiop/proofserialization/README.md section 6 (layout table).\n" ++
                "     Shipping the struct change alone silently invalidates every proof\n" ++
                "     produced afterwards.\n" ++
                "\n" ++
                "  b) Did you not mean to? Then get \"{s}\" back to offset {d}:\n" ++
                currentLayoutOf(T) ++
                "\n" ++ orderingAdvice(T),
            .{
                name, @typeName(T), @offsetOf(T, name), offset,
                name, offset,       @offsetOf(T, name), @typeName(T),
                name, offset,
            },
        ));
    }
}

/// Asserts a tagged union variant's numeric discriminant. The encoder writes
/// these as raw bytes, so reordering a union's variants is a wire change.
fn expectTag(comptime T: type, comptime variant: std.meta.Tag(T), comptime tag: usize) void {
    if (@intFromEnum(variant) != tag) {
        @compileError(std.fmt.comptimePrint(
            "\n\nPROOF ABI DRIFT: {s}.{s} has discriminant {d}, but the proof format is\n" ++
                "pinned to {d}.\n" ++
                "\nWHY THIS FIRED\n" ++
                "  A tagged union's discriminant is its variant's position in the\n" ++
                "  declaration, so INSERTING OR REORDERING VARIANTS RENUMBERS THEM.\n" ++
                "  prover-ray writes this number into the image as a raw byte.\n" ++
                "\nWHAT IT BREAKS\n" ++
                "  Nothing crashes. The verifier reads the wrong variant of a value it\n" ++
                "  is otherwise happy to accept — e.g. a base field element interpreted\n" ++
                "  as an extension element — and fails much later, or not at all.\n" ++
                "\nHOW TO FIX\n" ++
                "  a) Adding a variant? Append it AFTER the existing ones so the current\n" ++
                "     discriminants keep their values.\n" ++
                "  b) Genuinely renumbering? Update the pin below, prover-ray's encoder,\n" ++
                "     and prover-ray/wiop/proofserialization/README.md section 6 in the same change.\n",
            .{ @typeName(T), @tagName(variant), @intFromEnum(variant), tag },
        ));
    }
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
    // A committed round is represented solely by its Merkle root; columns never
    // travel raw, so there is no ColumnMessage union any more.
    expectSize(protocol.RoundMessage, 56, 8);
    expectField(protocol.RoundMessage, "cells", 0);
    expectField(protocol.RoundMessage, "commitment", 16);
    expectSize(?protocol.Commitment, 36, 4);

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

    expectSize(verifier.PcsOpening, 64, 8);
    expectField(verifier.PcsOpening, "proof", 0);

    // ---- root ----------------------------------------------------------------
    // VerifyInput, NOT Proof, is what the loaders cast the input region to, so it
    // is what must sit at image offset 0. Pinning only Proof was not enough: when
    // the root became VerifyInput, Proof stayed 112 bytes and unchanged, so every
    // assertion here still passed while the encoder was targeting the wrong root.
    expectSize(verifier.VerifyInput, 112, 8);
    expectField(verifier.VerifyInput, "proof", 0);
    expectField(verifier.VerifyInput, "public_inputs", 96);

    expectSize(verifier.Proof, 96, 8);
    expectField(verifier.Proof, "rounds", 0);
    expectField(verifier.Proof, "module_sizes", 16);
    expectField(verifier.Proof, "pcs_opening", 32);

    // ---- tagged unions and optionals ----------------------------------------
    // Sizes and discriminant values here; discriminant byte offsets, which
    // @offsetOf cannot express, in test/proof_abi_test.zig.
    expectSize(value.Scalar, 28, 4);
    expectTag(value.Scalar, .base, 0);
    expectTag(value.Scalar, .ext, 1);

    expectSize(?merkle.RowPair, 72, 8);
}
