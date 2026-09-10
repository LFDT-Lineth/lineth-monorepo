/// image_relocation patches every slice-pointer in a VerifyInput image from its
/// encoded guest address (encoded_base + offset) to the equivalent host address
/// (mapped_base + offset). The image layout is fully determined by the
/// proofserialization layout constants mirrored in proof_abi.zig; every []const T
/// header is a {ptr: u64le, len: u64le} pair and only the ptr field needs
/// adjustment. The walk is typed, never scanning raw bytes, so non-pointer u64s
/// (lengths, field values) are never touched.
///
/// Call this after loading an image into an anonymous mmap at an address other
/// than its encoded base — the macOS fallback in main.zig and the test fixture
/// loader in riscv_proof_image_test.zig both use it.
pub fn rebase(img: [*]u8, img_len: usize, encoded_base: usize, mapped_base: usize) void {
    const delta: i64 = @as(i64, @intCast(mapped_base)) - @as(i64, @intCast(encoded_base));

    // Patch a single slice-pointer at byte offset `off` in the image.
    // Returns the payload offset within the image (old_ptr - encoded_base),
    // which callers use to walk into the pointed-to data.
    const patchPtr = struct {
        fn f(image: [*]u8, len: usize, off: usize, enc_base: usize, d: i64) usize {
            if (off + 16 > len) return 0;
            const old_ptr = readU64(image, off);
            if (old_ptr == 0) return 0; // null / empty-slice sentinel
            const new_ptr = @as(u64, @intCast(@as(i64, @intCast(old_ptr)) + d));
            writeU64(image, off, new_ptr);
            return @intCast(@as(i64, @intCast(old_ptr - enc_base)));
        }
    }.f;

    const sliceLen = struct {
        fn f(image: [*]u8, off: usize) usize {
            return @intCast(readU64(image, off + 8));
        }
    }.f;

    // VerifyInput: proof @ 0 (96 bytes), public_inputs @ 96 ([]Scalar — no nested ptrs)
    _ = patchPtr(img, img_len, 96, encoded_base, delta);

    // Proof: rounds @ 0, module_sizes @ 16, pcs_opening @ 32
    const rounds_ptr = patchPtr(img, img_len, 0, encoded_base, delta);
    const n_rounds = sliceLen(img, 0);
    _ = patchPtr(img, img_len, 16, encoded_base, delta); // module_sizes.ptr (scalar)

    // RoundMessage: 56 bytes, cells @ 0 ([]Scalar — no nested ptrs)
    for (0..n_rounds) |i| {
        _ = patchPtr(img, img_len, rounds_ptr + i * 56, encoded_base, delta);
    }

    // PcsOpening.proof (OpeningProof): input_queries @ 32, fri_proof @ 48
    // input_queries: [][]InputTreeOpening
    const iq_outer_ptr = patchPtr(img, img_len, 32 + 0, encoded_base, delta);
    const n_iq = sliceLen(img, 32 + 0);
    for (0..n_iq) |i| {
        const inner_hdr = iq_outer_ptr + i * 16;
        const iq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_base, delta);
        const n_ito = sliceLen(img, inner_hdr);
        // InputTreeOpening: siblings @ 0, leaves @ 16
        for (0..n_ito) |j| {
            const ito_off = iq_inner_ptr + j * 32;
            _ = patchPtr(img, img_len, ito_off + 0, encoded_base, delta); // siblings.ptr
            // leaves: []?RowPair — inline 72-byte values, not pointers, so the
            // elements are walked at that stride with no dereference. Each is
            // [2]RowOpening then a presence flag at +64; RowOpening is
            // {base: []Scalar @0, ext: []Scalar @16}.
            const leaves_ptr = patchPtr(img, img_len, ito_off + 16, encoded_base, delta);
            const n_leaves = sliceLen(img, ito_off + 16);
            for (0..n_leaves) |k| {
                const leaf_off = leaves_ptr + k * 72;
                if (leaf_off + 65 > img_len) continue;
                if (img[leaf_off + 64] == 0) continue; // absent ?RowPair
                // RowPair[0]: base.ptr @0, ext.ptr @16
                _ = patchPtr(img, img_len, leaf_off + 0, encoded_base, delta);
                _ = patchPtr(img, img_len, leaf_off + 16, encoded_base, delta);
                // RowPair[1]: base.ptr @32, ext.ptr @48
                _ = patchPtr(img, img_len, leaf_off + 32, encoded_base, delta);
                _ = patchPtr(img, img_len, leaf_off + 48, encoded_base, delta);
            }
        }
    }

    // FriProof: round_roots @ 48 (scalar), final_poly @ 64 (scalar), running_queries @ 80
    _ = patchPtr(img, img_len, 48 + 0, encoded_base, delta);
    _ = patchPtr(img, img_len, 48 + 16, encoded_base, delta);

    // running_queries: [][]Branch; Branch: siblings @ 0, leaf @ 16 (inline Digest)
    const rq_outer_ptr = patchPtr(img, img_len, 48 + 32, encoded_base, delta);
    const n_rq = sliceLen(img, 48 + 32);
    for (0..n_rq) |i| {
        const inner_hdr = rq_outer_ptr + i * 16;
        const rq_inner_ptr = patchPtr(img, img_len, inner_hdr, encoded_base, delta);
        const n_br = sliceLen(img, inner_hdr);
        for (0..n_br) |j| {
            _ = patchPtr(img, img_len, rq_inner_ptr + j * 48, encoded_base, delta);
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
