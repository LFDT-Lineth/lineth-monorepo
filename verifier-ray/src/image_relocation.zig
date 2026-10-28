const std = @import("std");
const builtin = @import("builtin");
const merkle = @import("crypto/merkle.zig");
const field = @import("field/koalabear.zig");
const ext = @import("field/koalabear_ext.zig");
const poseidon2 = @import("crypto/poseidon2.zig");
const protocol = @import("protocol/types.zig");
const fri = @import("query/fri.zig");
const pcs = @import("query/pcs.zig");
const verifier = @import("verifier.zig");

comptime {
    // encoded_pointer/encoded_base/mapped_base are all treated as one 64-bit
    // address space below; every supported target (x86_64, aarch64, R5/RV64)
    // has a 64-bit usize, so no width-narrowing cast is needed or checked for.
    if (@bitSizeOf(usize) != 64) @compileError("image_relocation assumes a 64-bit usize");
    // The image is little-endian and `validateElements` reads its limbs as
    // native words.
    if (builtin.cpu.arch.endian() != .little) @compileError("image_relocation assumes a little-endian target");
}

pub const Error = error{
    PointerBeforeImage,
    PointerPastImage,
    AddressOverflow,
    /// The image does not start at a `VerifyInput`-aligned address.
    MisalignedImage,
};

/// Returns an encoded absolute pointer's byte offset within its image.
pub fn imageOffset(encoded_pointer: u64, encoded_base: usize) Error!usize {
    if (encoded_pointer < encoded_base) return error.PointerBeforeImage;
    return encoded_pointer - encoded_base;
}

/// Strides and offsets of the image layout, derived from the types so that a
/// field added to any of them moves this walk with it. Nothing downstream would
/// catch a stale number: a wrong offset patches the wrong bytes and leaves real
/// pointers at their encoded addresses, and the image still casts cleanly.
const abi = struct {
    /// {ptr, len}, both usize; only ptr needs adjusting.
    const slice_hdr = @sizeOf([]const u8);
    const slice_len_off = slice_hdr / 2;

    const public_inputs = @offsetOf(verifier.VerifyInput, "public_inputs");
    const rounds = @offsetOf(verifier.Proof, "rounds");
    const module_sizes = @offsetOf(verifier.Proof, "module_sizes");

    const round_message = @sizeOf(protocol.RoundMessage);
    const round_cells = @offsetOf(protocol.RoundMessage, "cells");
    const round_commitment = @offsetOf(protocol.RoundMessage, "commitment");

    /// OpeningProof lives at Proof.pcs_opening + PcsOpening.proof.
    const opening = @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    const input_queries = opening + @offsetOf(pcs.OpeningProof, "input_queries");
    const input_caps = opening + @offsetOf(pcs.OpeningProof, "input_caps");

    const input_cap = @sizeOf(pcs.InputCap);
    const input_cap_nodes = @offsetOf(pcs.InputCap, "nodes");
    const input_cap_tables = @offsetOf(pcs.InputCap, "tables");

    const input_cap_table = @sizeOf(pcs.InputCapTable);
    const input_cap_table_rows = @offsetOf(pcs.InputCapTable, "rows");

    const ito = @sizeOf(merkle.InputTreeOpening);
    const ito_siblings = @offsetOf(merkle.InputTreeOpening, "siblings");
    const ito_leaves = @offsetOf(merkle.InputTreeOpening, "leaves");

    const opt_row_pair = @sizeOf(?merkle.RowPair);
    /// An optional's discriminant offset is not derivable (@offsetOf rejects
    /// optionals), so this assumes the flag follows the payload.
    /// test/proof_abi_test.zig pins the real offset against a null and a
    /// non-null value.
    const row_pair_flag = @sizeOf(merkle.RowPair);
    const row_opening = @sizeOf(merkle.RowOpening);
    const row_base = @offsetOf(merkle.RowOpening, "base");
    const row_ext = @offsetOf(merkle.RowOpening, "ext");

    const fri_proof = opening + @offsetOf(pcs.OpeningProof, "fri_proof");
    const round_roots = fri_proof + @offsetOf(fri.Proof, "round_roots");
    const round_caps = fri_proof + @offsetOf(fri.Proof, "round_caps");
    const final_poly = fri_proof + @offsetOf(fri.Proof, "final_poly");
    const running_queries = fri_proof + @offsetOf(fri.Proof, "running_queries");

    const merkle_cap = @sizeOf(merkle.MerkleCap);
    const merkle_cap_nodes = @offsetOf(merkle.MerkleCap, "nodes");
    const merkle_cap_aux = @offsetOf(merkle.MerkleCap, "aux");

    const branch = @sizeOf(merkle.Branch);
    const branch_siblings = @offsetOf(merkle.Branch, "siblings");
};

/// Patches every slice-pointer in a VerifyInput image from `encoded_base +
/// offset` to `mapped_base + offset`, in place.
///
/// The walk is structural, never scanning for pointer-shaped bytes: it writes
/// only the pointer words of the headers it reaches.
///
/// Call this after loading an image at an address other than the one it was
/// encoded for: the native loader in main.zig and the fixture loader in
/// riscv_proof_image_test.zig both do.
///
/// The walk is memory-safe on arbitrary bytes: every read and write is bounded
/// to the image, every element count is clamped to what fits, and a pointer
/// that is not inside the encoded image is left as is. It does not make the
/// result trustworthy — overlapping slices can make a patch land on bytes that
/// are also read as data or as a length — so an untrusted image must pass
/// `validateImage(img, img_len, mapped_base)` AFTER this, at the layout the
/// verifier will actually read.
pub fn rebase(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) void {
    const w = Walk{ .img = img, .len = img_len, .encoded_base = encoded_base, .mapped_base = mapped_base };

    _ = w.patch(abi.public_inputs, 0);
    _ = w.patch(abi.module_sizes, 0);

    if (w.patch(abi.rounds, abi.round_message)) |rounds| {
        for (0..rounds.len) |i| {
            _ = w.patch(rounds.offset + i * abi.round_message + abi.round_cells, 0);
        }
    }

    if (w.patch(abi.input_caps, abi.input_cap)) |caps| {
        for (0..caps.len) |i| {
            const cap = caps.offset + i * abi.input_cap;
            _ = w.patch(cap + abi.input_cap_nodes, 0);
            const tables = w.patch(cap + abi.input_cap_tables, abi.input_cap_table) orelse continue;
            for (0..tables.len) |j| {
                const table = tables.offset + j * abi.input_cap_table;
                const rows = w.patch(table + abi.input_cap_table_rows, abi.row_opening) orelse continue;
                for (0..rows.len) |k| w.patchRow(rows.offset + k * abi.row_opening);
            }
        }
    }

    if (w.patch(abi.input_queries, abi.slice_hdr)) |queries| {
        for (0..queries.len) |i| {
            const openings = w.patch(queries.offset + i * abi.slice_hdr, abi.ito) orelse continue;
            for (0..openings.len) |j| {
                const opening = openings.offset + j * abi.ito;
                _ = w.patch(opening + abi.ito_siblings, 0);

                // leaves holds ?RowPair inline, so elements are walked at that
                // stride rather than dereferenced.
                const leaves = w.patch(opening + abi.ito_leaves, abi.opt_row_pair) orelse continue;
                for (0..leaves.len) |k| {
                    const leaf = leaves.offset + k * abi.opt_row_pair;
                    if (img[leaf + abi.row_pair_flag] == 0) continue; // absent
                    for (0..2) |r| w.patchRow(leaf + r * abi.row_opening);
                }
            }
        }
    }

    _ = w.patch(abi.round_roots, 0);

    if (w.patch(abi.round_caps, abi.merkle_cap)) |caps| {
        for (0..caps.len) |i| {
            const cap = caps.offset + i * abi.merkle_cap;
            _ = w.patch(cap + abi.merkle_cap_nodes, 0);
            _ = w.patch(cap + abi.merkle_cap_aux, 0);
        }
    }

    _ = w.patch(abi.final_poly, 0);

    if (w.patch(abi.running_queries, abi.slice_hdr)) |queries| {
        for (0..queries.len) |i| {
            const branches = w.patch(queries.offset + i * abi.slice_hdr, abi.branch) orelse continue;
            for (0..branches.len) |j| {
                _ = w.patch(branches.offset + j * abi.branch + abi.branch_siblings, 0);
            }
        }
    }
}

/// One `rebase` pass over one image.
const Walk = struct {
    img: [*]u8,
    len: usize,
    encoded_base: usize,
    mapped_base: usize,

    /// Relocates the slice header at `header` and returns its payload region,
    /// with the element count clamped to whole `stride`-sized elements that
    /// fit in the image (a `stride` of 0 means the payload is not walked).
    /// Returns null, leaving the header untouched, when the header is not in
    /// the image or its pointer is not inside the encoded image — including a
    /// pointer an earlier visit already relocated, so a header reached twice
    /// is patched once whenever the two ranges are disjoint.
    fn patch(w: Walk, header: usize, stride: usize) ?Region {
        if (header > w.len or w.len - header < abi.slice_hdr) return null;
        const pointer = readU64(w.img, header);
        if (pointer < w.encoded_base or pointer - w.encoded_base > w.len) return null;
        const offset: usize = pointer - w.encoded_base;
        writeU64(w.img, header, w.mapped_base + offset);
        if (stride == 0) return null;
        const count = readU64(w.img, header + abi.slice_len_off);
        return .{ .offset = offset, .len = @min(count, (w.len - offset) / stride) };
    }

    /// Relocates both slices of a `merkle.RowOpening` at `row`.
    fn patchRow(w: Walk, row: usize) void {
        _ = w.patch(row + abi.row_base, 0);
        _ = w.patch(row + abi.row_ext, 0);
    }
};

/// Byte-for-byte layout of the aggregator pair image header: two absolute
/// little-endian u64 pointers, matching `EncodeAggregatorPair`'s header write
/// and `verifier.AggregatorInput`.
pub const pair_header_len = 2 * @sizeOf(u64);

/// Offsets of an aggregator pair image's two `VerifyInput` members.
pub const PairOffsets = struct {
    a: usize,
    b: usize,
};

/// Reads and checks an aggregator pair header without trusting it: the one
/// definition of a well-formed header, shared by `validatePair` and
/// `rebasePair` (and through them every loader in main.zig).
///
/// Proof A starts right after the header, as `EncodeAggregatorPair` writes
/// it; proof B follows A's root, aligned for a `VerifyInput`, with room for
/// its own root. Returns the members' offsets within the image.
pub fn parsePairHeader(img: [*]const u8, img_len: usize, encoded_base: usize) Error!PairOffsets {
    const root_len = @sizeOf(verifier.VerifyInput);
    try requireRange(0, pair_header_len, img_len);
    const a = try imageOffset(readU64(img, 0), encoded_base);
    const b = try imageOffset(readU64(img, @sizeOf(u64)), encoded_base);
    if (a != pair_header_len) return error.PointerPastImage;
    if (b < a + root_len or b % @alignOf(verifier.VerifyInput) != 0) return error.PointerPastImage;
    try requireRange(b, root_len, img_len);
    return .{ .a = a, .b = b };
}

/// Validates an untrusted aggregator pair image: its header
/// (`parsePairHeader`) and each member with `validateImage`, each bounded to
/// its own region so neither can reach into the other.
pub fn validatePair(img: [*]const u8, img_len: usize, encoded_base: usize) Error!PairOffsets {
    const pair = try parsePairHeader(img, img_len, encoded_base);
    try validateImage(img + pair.a, pair.b - pair.a, encoded_base + pair.a);
    try validateImage(img + pair.b, img_len - pair.b, encoded_base + pair.b);
    return pair;
}

/// Rebases an aggregator pair image (see `verifier.AggregatorInput`): rebases
/// each `VerifyInput` member, then patches the two header pointers.
/// `EncodeAggregatorPair` encodes each member with `Encode` at its own absolute
/// start address, so each is walked relative to that start rather than to the
/// pair image's base. Like `rebase`, the result is untrusted: validate it with
/// `validatePair(img, img_len, mapped_base)` afterwards.
pub fn rebasePair(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) Error!void {
    const pair = try parsePairHeader(img, img_len, encoded_base);
    rebase(img + pair.a, pair.b - pair.a, encoded_base + pair.a, mapped_base + pair.a);
    rebase(img + pair.b, img_len - pair.b, encoded_base + pair.b, mapped_base + pair.b);
    writeU64(img, 0, mapped_base + pair.a);
    writeU64(img, @sizeOf(u64), mapped_base + pair.b);
}

const Region = struct {
    offset: usize,
    len: usize,
};

/// Validates an encoded VerifyInput without constructing Zig slices or tagged
/// unions from untrusted bytes.
pub fn validateImage(img: [*]const u8, img_len: usize, encoded_base: usize) Error!void {
    // Every offset below is checked against its type's alignment relative to
    // the image start. An aligned `img` makes the limb reads here aligned; an
    // aligned `encoded_base` makes the pointers the verifier dereferences
    // (`encoded_base + offset`) aligned.
    if (@intFromPtr(img) % @alignOf(verifier.VerifyInput) != 0 or
        encoded_base % @alignOf(verifier.VerifyInput) != 0) return error.MisalignedImage;
    try requireRange(0, @sizeOf(verifier.VerifyInput), img_len);
    try validateScalars(img, img_len, try resolveSlice(img, img_len, abi.public_inputs, @sizeOf(protocol.Scalar), @alignOf(protocol.Scalar), encoded_base));
    _ = try resolveSlice(img, img_len, abi.module_sizes, @sizeOf(usize), @alignOf(usize), encoded_base);

    const rounds = try resolveSlice(img, img_len, abi.rounds, abi.round_message, @alignOf(protocol.RoundMessage), encoded_base);
    for (0..rounds.len) |i| {
        const round = rounds.offset + i * abi.round_message;
        try validateOptionalDigest(img, img_len, round + abi.round_commitment);
        try validateScalars(img, img_len, try resolveSlice(img, img_len, round + abi.round_cells, @sizeOf(protocol.Scalar), @alignOf(protocol.Scalar), encoded_base));
    }

    const input_caps = try resolveSlice(img, img_len, abi.input_caps, abi.input_cap, @alignOf(pcs.InputCap), encoded_base);
    for (0..input_caps.len) |i| {
        const cap = input_caps.offset + i * abi.input_cap;
        try validateElements(img, img_len, try resolveSlice(img, img_len, cap + abi.input_cap_nodes, @sizeOf(poseidon2.Digest), @alignOf(poseidon2.Digest), encoded_base), @sizeOf(poseidon2.Digest));
        const tables = try resolveSlice(img, img_len, cap + abi.input_cap_tables, abi.input_cap_table, @alignOf(pcs.InputCapTable), encoded_base);
        for (0..tables.len) |j| {
            const table = tables.offset + j * abi.input_cap_table;
            try validateRows(img, img_len, try resolveSlice(img, img_len, table + abi.input_cap_table_rows, abi.row_opening, @alignOf(merkle.RowOpening), encoded_base), encoded_base);
        }
    }

    const queries = try resolveSlice(img, img_len, abi.input_queries, abi.slice_hdr, @alignOf([]const merkle.InputTreeOpening), encoded_base);
    for (0..queries.len) |i| {
        const openings = try resolveSlice(img, img_len, queries.offset + i * abi.slice_hdr, abi.ito, @alignOf(merkle.InputTreeOpening), encoded_base);
        for (0..openings.len) |j| {
            const opening = openings.offset + j * abi.ito;
            try validateElements(img, img_len, try resolveSlice(img, img_len, opening + abi.ito_siblings, @sizeOf(poseidon2.Digest), @alignOf(poseidon2.Digest), encoded_base), @sizeOf(poseidon2.Digest));
            const leaves = try resolveSlice(img, img_len, opening + abi.ito_leaves, abi.opt_row_pair, @alignOf(?merkle.RowPair), encoded_base);
            for (0..leaves.len) |k| {
                const leaf = leaves.offset + k * abi.opt_row_pair;
                const flag = img[leaf + abi.row_pair_flag];
                if (flag > 1) return error.PointerPastImage;
                if (flag == 1) try validateRows(img, img_len, .{ .offset = leaf, .len = 2 }, encoded_base);
            }
        }
    }

    try validateElements(img, img_len, try resolveSlice(img, img_len, abi.round_roots, @sizeOf(poseidon2.Digest), @alignOf(poseidon2.Digest), encoded_base), @sizeOf(poseidon2.Digest));
    const caps = try resolveSlice(img, img_len, abi.round_caps, abi.merkle_cap, @alignOf(merkle.MerkleCap), encoded_base);
    for (0..caps.len) |i| {
        const cap = caps.offset + i * abi.merkle_cap;
        try validateElements(img, img_len, try resolveSlice(img, img_len, cap + abi.merkle_cap_nodes, @sizeOf(poseidon2.Digest), @alignOf(poseidon2.Digest), encoded_base), @sizeOf(poseidon2.Digest));
        const aux = try resolveSlice(img, img_len, cap + abi.merkle_cap_aux, @sizeOf(?poseidon2.Digest), @alignOf(?poseidon2.Digest), encoded_base);
        for (0..aux.len) |j| try validateOptionalDigest(img, img_len, aux.offset + j * @sizeOf(?poseidon2.Digest));
    }
    try validateElements(img, img_len, try resolveSlice(img, img_len, abi.final_poly, @sizeOf(ext.Ext), @alignOf(ext.Ext), encoded_base), @sizeOf(ext.Ext));

    const running = try resolveSlice(img, img_len, abi.running_queries, abi.slice_hdr, @alignOf([]const merkle.Branch), encoded_base);
    for (0..running.len) |i| {
        const branches = try resolveSlice(img, img_len, running.offset + i * abi.slice_hdr, abi.branch, @alignOf(merkle.Branch), encoded_base);
        for (0..branches.len) |j| {
            const branch = branches.offset + j * abi.branch;
            try validateElements(img, img_len, try resolveSlice(img, img_len, branch + abi.branch_siblings, @sizeOf(poseidon2.Digest), @alignOf(poseidon2.Digest), encoded_base), @sizeOf(poseidon2.Digest));
            try validateElements(img, img_len, .{ .offset = branch + @offsetOf(merkle.Branch, "leaf"), .len = 1 }, @sizeOf(poseidon2.Digest));
        }
    }
}

fn resolveSlice(img: [*]const u8, img_len: usize, header: usize, elem_size: usize, alignment: usize, encoded_base: usize) Error!Region {
    try requireRange(header, abi.slice_hdr, img_len);
    const pointer = readU64(img, header);
    if (pointer < encoded_base) return error.PointerBeforeImage;
    const offset: usize = @intCast(pointer - encoded_base);
    const len: usize = @intCast(readU64(img, header + abi.slice_len_off));
    if (offset % alignment != 0) return error.PointerPastImage;
    const extent = std.math.mul(usize, len, elem_size) catch return error.AddressOverflow;
    try requireRange(offset, extent, img_len);
    return .{ .offset = offset, .len = len };
}

fn validateRows(img: [*]const u8, img_len: usize, rows: Region, encoded_base: usize) Error!void {
    for (0..rows.len) |i| {
        const row = rows.offset + i * abi.row_opening;
        try validateElements(img, img_len, try resolveSlice(img, img_len, row + abi.row_base, @sizeOf(field.Element), @alignOf(field.Element), encoded_base), @sizeOf(field.Element));
        try validateElements(img, img_len, try resolveSlice(img, img_len, row + abi.row_ext, @sizeOf(ext.Ext), @alignOf(ext.Ext), encoded_base), @sizeOf(ext.Ext));
    }
}

fn validateScalars(img: [*]const u8, img_len: usize, scalars: Region) Error!void {
    for (0..scalars.len) |i| {
        const scalar = scalars.offset + i * @sizeOf(protocol.Scalar);
        if (img[scalar + @sizeOf(ext.Ext)] > 1) return error.PointerPastImage;
        try validateElements(img, img_len, .{ .offset = scalar, .len = 1 }, @sizeOf(ext.Ext));
    }
}

fn validateOptionalDigest(img: [*]const u8, img_len: usize, offset: usize) Error!void {
    try requireRange(offset, @sizeOf(?poseidon2.Digest), img_len);
    const flag = img[offset + @sizeOf(poseidon2.Digest)];
    if (flag > 1) return error.PointerPastImage;
    if (flag == 1) try validateElements(img, img_len, .{ .offset = offset, .len = 1 }, @sizeOf(poseidon2.Digest));
}

fn validateElements(img: [*]const u8, img_len: usize, values: Region, stride: usize) Error!void {
    const extent = std.math.mul(usize, values.len, stride) catch return error.AddressOverflow;
    try requireRange(values.offset, extent, img_len);
    const count = extent / @sizeOf(field.Element);
    // Each caller's offset is a `resolveSlice`-checked slice start or a field
    // offset within one, of a type at least as aligned as `field.Element`, and
    // `validateImage` checked the image start: one aligned load per limb
    // rather than four byte loads, on the guest's hottest validation loop.
    const limbs: [*]const u32 = @ptrCast(@alignCast(img + values.offset));
    for (limbs[0..count]) |limb| {
        if (limb >= field.modulus) return error.PointerPastImage;
    }
}

fn requireRange(offset: usize, len: usize, image_len: usize) Error!void {
    const end = std.math.add(usize, offset, len) catch return error.AddressOverflow;
    if (end > image_len) return error.PointerPastImage;
}

fn readU64(img: [*]const u8, off: usize) u64 {
    return @as(u64, img[off]) |
        (@as(u64, img[off + 1]) << 8) |
        (@as(u64, img[off + 2]) << 16) |
        (@as(u64, img[off + 3]) << 24) |
        (@as(u64, img[off + 4]) << 32) |
        (@as(u64, img[off + 5]) << 40) |
        (@as(u64, img[off + 6]) << 48) |
        (@as(u64, img[off + 7]) << 56);
}

fn writeU64(img: [*]u8, off: usize, v: u64) void {
    img[off + 0] = @truncate(v);
    img[off + 1] = @truncate(v >> 8);
    img[off + 2] = @truncate(v >> 16);
    img[off + 3] = @truncate(v >> 24);
    img[off + 4] = @truncate(v >> 32);
    img[off + 5] = @truncate(v >> 40);
    img[off + 6] = @truncate(v >> 48);
    img[off + 7] = @truncate(v >> 56);
}
