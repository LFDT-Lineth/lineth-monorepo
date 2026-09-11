const merkle = @import("crypto/merkle.zig");
const protocol = @import("protocol/types.zig");
const fri = @import("query/fri.zig");
const pcs = @import("query/pcs.zig");
const verifier = @import("verifier.zig");

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

    /// OpeningProof lives at Proof.pcs_opening + PcsOpening.proof.
    const opening = @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    const input_queries = opening + @offsetOf(pcs.OpeningProof, "input_queries");

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
    const final_poly = fri_proof + @offsetOf(fri.Proof, "final_poly");
    const running_queries = fri_proof + @offsetOf(fri.Proof, "running_queries");

    const branch = @sizeOf(merkle.Branch);
    const branch_siblings = @offsetOf(merkle.Branch, "siblings");
};

/// Patches every slice-pointer in a VerifyInput image from `encoded_base +
/// offset` to `mapped_base + offset`, in place.
///
/// The walk is structural, never scanning for pointer-shaped bytes, so lengths
/// and field values are never touched.
///
/// Call this after loading an image at an address other than the one it was
/// encoded for: the macOS fallback in main.zig and the fixture loader in
/// riscv_proof_image_test.zig both do.
pub fn rebase(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) void {
    const delta: i64 = @as(i64, @intCast(mapped_base)) - @as(i64, @intCast(encoded_base));

    // Returns the patched pointer's offset within the image, for walking into
    // whatever it points at.
    const patchPtr = struct {
        fn f(image: [*]u8, len: usize, off: usize, enc_base: usize, d: i64) usize {
            if (off + abi.slice_hdr > len) return 0;
            const old_ptr = readU64(image, off);
            if (old_ptr == 0) return 0; // null / empty-slice sentinel
            const new_ptr = @as(u64, @intCast(@as(i64, @intCast(old_ptr)) + d));
            writeU64(image, off, new_ptr);
            return @intCast(@as(i64, @intCast(old_ptr - enc_base)));
        }
    }.f;

    const sliceLen = struct {
        fn f(image: [*]u8, hdr: usize) usize {
            return @intCast(readU64(image, hdr + abi.slice_len_off));
        }
    }.f;

    _ = patchPtr(img, img_len, abi.public_inputs, encoded_base, delta);

    const rounds_ptr = patchPtr(img, img_len, abi.rounds, encoded_base, delta);
    const n_rounds = sliceLen(img, abi.rounds);
    _ = patchPtr(img, img_len, abi.module_sizes, encoded_base, delta);

    for (0..n_rounds) |i| {
        const rm = rounds_ptr + i * abi.round_message;
        _ = patchPtr(img, img_len, rm + abi.round_cells, encoded_base, delta);
    }

    const iq_outer_ptr = patchPtr(img, img_len, abi.input_queries, encoded_base, delta);
    const n_iq = sliceLen(img, abi.input_queries);
    for (0..n_iq) |i| {
        const inner_hdr = iq_outer_ptr + i * abi.slice_hdr;
        const iq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_base, delta);
        const n_ito = sliceLen(img, inner_hdr);
        for (0..n_ito) |j| {
            const ito_off = iq_inner_ptr + j * abi.ito;
            _ = patchPtr(img, img_len, ito_off + abi.ito_siblings, encoded_base, delta);

            // leaves holds ?RowPair inline, so elements are walked at that
            // stride rather than dereferenced.
            const leaves_hdr = ito_off + abi.ito_leaves;
            const leaves_ptr = patchPtr(img, img_len, leaves_hdr, encoded_base, delta);
            const n_leaves = sliceLen(img, leaves_hdr);
            for (0..n_leaves) |k| {
                const leaf_off = leaves_ptr + k * abi.opt_row_pair;
                if (leaf_off + abi.row_pair_flag + 1 > img_len) continue;
                if (img[leaf_off + abi.row_pair_flag] == 0) continue; // absent
                for (0..2) |r| {
                    const row = leaf_off + r * abi.row_opening;
                    _ = patchPtr(img, img_len, row + abi.row_base, encoded_base, delta);
                    _ = patchPtr(img, img_len, row + abi.row_ext, encoded_base, delta);
                }
            }
        }
    }

    _ = patchPtr(img, img_len, abi.round_roots, encoded_base, delta);
    _ = patchPtr(img, img_len, abi.final_poly, encoded_base, delta);

    const rq_outer_ptr = patchPtr(img, img_len, abi.running_queries, encoded_base, delta);
    const n_rq = sliceLen(img, abi.running_queries);
    for (0..n_rq) |i| {
        const inner_hdr = rq_outer_ptr + i * abi.slice_hdr;
        const rq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_base, delta);
        const n_br = sliceLen(img, inner_hdr);
        for (0..n_br) |j| {
            const br = rq_inner_ptr + j * abi.branch;
            _ = patchPtr(img, img_len, br + abi.branch_siblings, encoded_base, delta);
        }
    }
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
