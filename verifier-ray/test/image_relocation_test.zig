const std = @import("std");
const verifier_ray = @import("verifier_ray");

const verifier = verifier_ray.verifier;
const merkle = verifier_ray.crypto.merkle;
const fri = verifier_ray.query.fri;
const pcs = verifier_ray.query.pcs;

const slice_ptr_offset = 0;
const slice_len_offset = @sizeOf(usize);

fn putSlice(image: []u8, header: usize, pointer: usize, len: usize) void {
    std.mem.writeInt(u64, image[header + slice_ptr_offset ..][0..8], @intCast(pointer), .little);
    std.mem.writeInt(u64, image[header + slice_len_offset ..][0..8], @intCast(len), .little);
}

fn expectPointer(image: []const u8, header: usize, expected: usize) !void {
    try std.testing.expectEqual(
        @as(u64, @intCast(expected)),
        std.mem.readInt(u64, image[header + slice_ptr_offset ..][0..8], .little),
    );
}

test "image relocation rebases every Merkle cap slice" {
    const encoded_base: usize = 0x08800000;

    const opening = @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    const input_caps_header = opening + @offsetOf(pcs.OpeningProof, "input_caps");
    const fri_proof = opening + @offsetOf(pcs.OpeningProof, "fri_proof");
    const round_caps_header = fri_proof + @offsetOf(fri.Proof, "round_caps");

    const input_caps = 144;
    const input_cap_0_nodes = 208;
    const input_cap_0_tables = 240;
    const input_cap_1_nodes = 288;
    const input_cap_1_tables = 320;
    const input_cap_0_table_0_rows = 344;
    const input_cap_0_table_1_rows = 408;
    const input_cap_1_table_0_rows = 440;
    const row_payloads = [_][2]usize{
        .{ 472, 476 },
        .{ 500, 504 },
        .{ 528, 532 },
        .{ 556, 560 },
    };
    const round_caps = 584;
    const round_cap_0_nodes = 648;
    const round_cap_0_aux = 680;
    const round_cap_1_nodes = 716;

    var image: [760]u8 = @splat(0);
    const mapped_base = @intFromPtr(&image);

    putSlice(&image, input_caps_header, encoded_base + input_caps, 2);

    const input_cap_0 = input_caps;
    putSlice(&image, input_cap_0 + @offsetOf(pcs.InputCap, "nodes"), encoded_base + input_cap_0_nodes, 1);
    putSlice(&image, input_cap_0 + @offsetOf(pcs.InputCap, "tables"), encoded_base + input_cap_0_tables, 2);

    const input_cap_1 = input_caps + @sizeOf(pcs.InputCap);
    putSlice(&image, input_cap_1 + @offsetOf(pcs.InputCap, "nodes"), encoded_base + input_cap_1_nodes, 1);
    putSlice(&image, input_cap_1 + @offsetOf(pcs.InputCap, "tables"), encoded_base + input_cap_1_tables, 1);

    const input_cap_0_table_0 = input_cap_0_tables;
    const input_cap_0_table_1 = input_cap_0_tables + @sizeOf(pcs.InputCapTable);
    const input_cap_1_table_0 = input_cap_1_tables;
    putSlice(
        &image,
        input_cap_0_table_0 + @offsetOf(pcs.InputCapTable, "rows"),
        encoded_base + input_cap_0_table_0_rows,
        2,
    );
    putSlice(
        &image,
        input_cap_0_table_1 + @offsetOf(pcs.InputCapTable, "rows"),
        encoded_base + input_cap_0_table_1_rows,
        1,
    );
    putSlice(
        &image,
        input_cap_1_table_0 + @offsetOf(pcs.InputCapTable, "rows"),
        encoded_base + input_cap_1_table_0_rows,
        1,
    );

    const row_offsets = [_]usize{
        input_cap_0_table_0_rows,
        input_cap_0_table_0_rows + @sizeOf(merkle.RowOpening),
        input_cap_0_table_1_rows,
        input_cap_1_table_0_rows,
    };
    for (row_offsets, row_payloads) |row, payloads| {
        putSlice(&image, row + @offsetOf(merkle.RowOpening, "base"), encoded_base + payloads[0], 1);
        putSlice(&image, row + @offsetOf(merkle.RowOpening, "ext"), encoded_base + payloads[1], 1);
    }

    putSlice(&image, round_caps_header, encoded_base + round_caps, 2);
    const round_cap_0 = round_caps;
    putSlice(&image, round_cap_0 + @offsetOf(merkle.MerkleCap, "nodes"), encoded_base + round_cap_0_nodes, 1);
    putSlice(&image, round_cap_0 + @offsetOf(merkle.MerkleCap, "aux"), encoded_base + round_cap_0_aux, 1);

    const round_cap_1 = round_caps + @sizeOf(merkle.MerkleCap);
    putSlice(&image, round_cap_1 + @offsetOf(merkle.MerkleCap, "nodes"), encoded_base + round_cap_1_nodes, 1);
    // Go encodes empty slices as a non-null pointer to the image base.
    putSlice(&image, round_cap_1 + @offsetOf(merkle.MerkleCap, "aux"), encoded_base, 0);

    verifier_ray.image_relocation.rebase(&image, image.len, encoded_base, mapped_base);

    try expectPointer(&image, input_caps_header, mapped_base + input_caps);
    try expectPointer(&image, input_cap_0 + @offsetOf(pcs.InputCap, "nodes"), mapped_base + input_cap_0_nodes);
    try expectPointer(&image, input_cap_0 + @offsetOf(pcs.InputCap, "tables"), mapped_base + input_cap_0_tables);
    try expectPointer(&image, input_cap_1 + @offsetOf(pcs.InputCap, "nodes"), mapped_base + input_cap_1_nodes);
    try expectPointer(&image, input_cap_1 + @offsetOf(pcs.InputCap, "tables"), mapped_base + input_cap_1_tables);
    try expectPointer(
        &image,
        input_cap_0_table_0 + @offsetOf(pcs.InputCapTable, "rows"),
        mapped_base + input_cap_0_table_0_rows,
    );
    try expectPointer(
        &image,
        input_cap_0_table_1 + @offsetOf(pcs.InputCapTable, "rows"),
        mapped_base + input_cap_0_table_1_rows,
    );
    try expectPointer(
        &image,
        input_cap_1_table_0 + @offsetOf(pcs.InputCapTable, "rows"),
        mapped_base + input_cap_1_table_0_rows,
    );
    for (row_offsets, row_payloads) |row, payloads| {
        try expectPointer(&image, row + @offsetOf(merkle.RowOpening, "base"), mapped_base + payloads[0]);
        try expectPointer(&image, row + @offsetOf(merkle.RowOpening, "ext"), mapped_base + payloads[1]);
    }

    try expectPointer(&image, round_caps_header, mapped_base + round_caps);
    try expectPointer(&image, round_cap_0 + @offsetOf(merkle.MerkleCap, "nodes"), mapped_base + round_cap_0_nodes);
    try expectPointer(&image, round_cap_0 + @offsetOf(merkle.MerkleCap, "aux"), mapped_base + round_cap_0_aux);
    try expectPointer(&image, round_cap_1 + @offsetOf(merkle.MerkleCap, "nodes"), mapped_base + round_cap_1_nodes);
    try expectPointer(&image, round_cap_1 + @offsetOf(merkle.MerkleCap, "aux"), mapped_base);
}

test "pair relocation rebases the header and each sub-image against its own start" {
    const relocation = verifier_ray.image_relocation;
    const encoded_base: usize = 0x08800000;
    const hdr = relocation.pair_header_len;
    // Each sub-image is a zeroed VerifyInput (every other slice null) whose
    // public_inputs points just past it.
    const payload = @sizeOf(verifier.VerifyInput);
    const sub_len = payload + 32;
    const public_inputs_header = @offsetOf(verifier.VerifyInput, "public_inputs");

    var image: [hdr + 2 * sub_len]u8 align(8) = @splat(0);
    const mapped_base = @intFromPtr(&image);
    const subs = [_]usize{ hdr, hdr + sub_len };
    for (subs, 0..) |sub, i| {
        std.mem.writeInt(u64, image[i * 8 ..][0..8], encoded_base + sub, .little);
        putSlice(image[sub..], public_inputs_header, encoded_base + sub + payload, 1);
    }

    try relocation.rebasePair(&image, image.len, encoded_base, mapped_base);

    for (subs, 0..) |sub, i| {
        try std.testing.expectEqual(
            @as(u64, mapped_base + sub),
            std.mem.readInt(u64, image[i * 8 ..][0..8], .little),
        );
        try expectPointer(image[sub..], public_inputs_header, mapped_base + sub + payload);
    }
}

test "pair header rejects malformed pointers" {
    const relocation = verifier_ray.image_relocation;
    const encoded_base: usize = 0x08800000;
    const hdr = relocation.pair_header_len;
    const root_len = @sizeOf(verifier.VerifyInput);
    // Large enough for both roots, so every rejection below comes from the
    // pointer it targets rather than from the image being too short.
    const b_ok = hdr + root_len;
    // Slack past B's root, so a misaligned B is rejected for its alignment
    // rather than for its root not fitting.
    var image: [b_ok + root_len + 16]u8 align(8) = @splat(0);

    const Case = struct { a: u64, b: u64, err: relocation.Error };
    const cases = [_]Case{
        // A must start right after the header.
        .{ .a = encoded_base + hdr + 8, .b = encoded_base + b_ok, .err = error.PointerPastImage },
        .{ .a = encoded_base - 8, .b = encoded_base + b_ok, .err = error.PointerBeforeImage },
        // B must not overlap A's root, must be aligned, and its root must fit.
        .{ .a = encoded_base + hdr, .b = encoded_base + hdr + 8, .err = error.PointerPastImage },
        .{ .a = encoded_base + hdr, .b = encoded_base + b_ok + 4, .err = error.PointerPastImage },
        .{ .a = encoded_base + hdr, .b = encoded_base + b_ok + 24, .err = error.PointerPastImage },
        .{ .a = encoded_base + hdr, .b = encoded_base - 8, .err = error.PointerBeforeImage },
    };
    for (cases) |case| {
        std.mem.writeInt(u64, image[0..8], case.a, .little);
        std.mem.writeInt(u64, image[8..16], case.b, .little);
        const before = image;
        try std.testing.expectError(case.err, relocation.parsePairHeader(&image, image.len, encoded_base));
        try std.testing.expectError(case.err, relocation.rebasePair(&image, image.len, encoded_base, @intFromPtr(&image)));
        // A rejected header leaves the image untouched.
        try std.testing.expectEqualSlices(u8, &before, &image);
    }

    // B near the top of the address space must not wrap the root-fits check.
    // Only a base below one root's size leaves room for such an offset.
    std.mem.writeInt(u64, image[0..8], hdr, .little);
    std.mem.writeInt(u64, image[8..16], std.math.maxInt(u64) - 7, .little);
    try std.testing.expectError(error.AddressOverflow, relocation.parsePairHeader(&image, image.len, 0));

    std.mem.writeInt(u64, image[0..8], encoded_base + hdr, .little);
    std.mem.writeInt(u64, image[8..16], encoded_base + b_ok, .little);
    const pair = try relocation.parsePairHeader(&image, image.len, encoded_base);
    try std.testing.expectEqual(hdr, pair.a);
    try std.testing.expectEqual(b_ok, pair.b);
    try std.testing.expectError(error.PointerPastImage, relocation.parsePairHeader(&image, hdr - 1, encoded_base));
}

test "relocation patches a header reachable twice only once" {
    // Two outer slices share one payload of inner slice headers. Nothing in
    // the format forbids that, so the walk reaches the shared inner header
    // twice; patching it again would push it past the mapping.
    const encoded_base: usize = 0x08800000;
    const opening = @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    const input_queries_header = opening + @offsetOf(pcs.OpeningProof, "input_queries");
    const fri_proof = opening + @offsetOf(pcs.OpeningProof, "fri_proof");
    const running_queries_header = fri_proof + @offsetOf(fri.Proof, "running_queries");

    const shared = comptime std.mem.alignForward(usize, @sizeOf(verifier.VerifyInput), 8);
    const target = shared + 16;
    var image: [target + 64]u8 align(8) = @splat(0);
    const mapped_base = @intFromPtr(&image);

    putSlice(&image, input_queries_header, encoded_base + shared, 1);
    putSlice(&image, running_queries_header, encoded_base + shared, 1);
    // The shared inner header: an empty slice pointing into the image.
    putSlice(&image, shared, encoded_base + target, 0);

    verifier_ray.image_relocation.rebase(&image, image.len, encoded_base, mapped_base);

    try expectPointer(&image, input_queries_header, mapped_base + shared);
    try expectPointer(&image, running_queries_header, mapped_base + shared);
    try expectPointer(&image, shared, mapped_base + target);
}

extern fn mmap(address: ?*anyopaque, length: usize, protection: c_int, flags: c_int, fd: c_int, offset: i64) *anyopaque;
extern fn mprotect(address: *anyopaque, length: usize, protection: c_int) c_int;
extern fn munmap(address: *anyopaque, length: usize) c_int;

/// A zeroed `len`-byte image that ends exactly at an inaccessible guard
/// region, so any read or write past it faults instead of passing silently.
const GuardedImage = struct {
    // 64 KiB is a multiple of every supported page size.
    const region = 64 * 1024;

    raw: *anyopaque,
    image: [*]u8,

    fn init(len: usize) !GuardedImage {
        const prot_none: c_int = 0;
        const prot_read_write: c_int = 3;
        const map_private: c_int = 2;
        const map_anonymous: c_int = if (@import("builtin").os.tag == .macos) 0x1000 else 0x20;
        const raw = mmap(null, 2 * region, prot_read_write, map_private | map_anonymous, -1, 0);
        if (@intFromPtr(raw) == ~@as(usize, 0)) return error.MmapFailed;
        const base: [*]u8 = @ptrCast(raw);
        if (mprotect(@ptrCast(base + region), region, prot_none) != 0) {
            _ = munmap(raw, 2 * region);
            return error.MprotectFailed;
        }
        return .{ .raw = raw, .image = base + region - len };
    }

    fn deinit(g: GuardedImage) void {
        _ = munmap(g.raw, 2 * region);
    }
};

test "relocation stays inside the image on arbitrary bytes" {
    // Alternating words: a pointer to the image start, then a count far larger
    // than the image. Every header with the pointer parity claims an oversized
    // payload aliasing the root; the walk must stay inside the image.
    const encoded_base: usize = 0x08800000;
    const len = 4096;
    const guarded = try GuardedImage.init(len);
    defer guarded.deinit();
    const image = guarded.image;

    // Pointer words share the root `public_inputs` header's parity, so that
    // header is one the walk relocates.
    const pointer_parity = (@offsetOf(verifier.VerifyInput, "public_inputs") / 8) % 2;
    for (0..len / 8) |i| {
        const word: u64 = if (i % 2 == pointer_parity) encoded_base else 1024;
        std.mem.writeInt(u64, image[i * 8 ..][0..8], word, .little);
    }
    const mapped_base = @intFromPtr(image);

    verifier_ray.image_relocation.rebase(image, len, encoded_base, mapped_base);

    // A header the walk reaches was relocated to the image start.
    try expectPointer(image[0..len], @offsetOf(verifier.VerifyInput, "public_inputs"), mapped_base);
}

test "relocation clamps a slice count to the elements that fit" {
    // input_queries -> one inner slice -> one opening whose `leaves` start one
    // element before the image end but claim 1000 elements. The walk reads
    // each leaf's presence flag, so an unclamped count reads into the guard.
    const encoded_base: usize = 0x08800000;
    const root = comptime std.mem.alignForward(usize, @sizeOf(verifier.VerifyInput), 8);
    const queries = root;
    const opening = queries + 16;
    const leaves = opening + comptime std.mem.alignForward(usize, @sizeOf(merkle.InputTreeOpening), 8);
    const len = comptime leaves + @sizeOf(?merkle.RowPair);
    const guarded = try GuardedImage.init(len);
    defer guarded.deinit();
    const image = guarded.image[0..len];
    @memset(image, 0);
    const mapped_base = @intFromPtr(image.ptr);

    const pcs_opening = @offsetOf(verifier.VerifyInput, "proof") + @offsetOf(verifier.Proof, "pcs_opening") +
        @offsetOf(verifier.PcsOpening, "proof");
    putSlice(image, pcs_opening + @offsetOf(pcs.OpeningProof, "input_queries"), encoded_base + queries, 1);
    putSlice(image, queries, encoded_base + opening, 1);
    putSlice(image, opening + @offsetOf(merkle.InputTreeOpening, "leaves"), encoded_base + leaves, 1000);

    verifier_ray.image_relocation.rebase(image.ptr, len, encoded_base, mapped_base);

    try expectPointer(image, opening + @offsetOf(merkle.InputTreeOpening, "leaves"), mapped_base + leaves);
}

test "relocation leaves out-of-image pointers alone and relocates the image end" {
    const encoded_base: usize = 0x08800000;
    const len = comptime std.mem.alignForward(usize, @sizeOf(verifier.VerifyInput), 8) + 64;
    var image: [len]u8 align(8) = @splat(0);
    const mapped_base = @intFromPtr(&image);

    const public_inputs = @offsetOf(verifier.VerifyInput, "public_inputs");
    const rounds = @offsetOf(verifier.VerifyInput, "proof") + @offsetOf(verifier.Proof, "rounds");
    const module_sizes = @offsetOf(verifier.VerifyInput, "proof") + @offsetOf(verifier.Proof, "module_sizes");
    putSlice(&image, public_inputs, encoded_base - 8, 1); // before the image
    putSlice(&image, rounds, encoded_base + len + 8, 1); // past the image
    putSlice(&image, module_sizes, encoded_base + len, 0); // empty slice at the end

    verifier_ray.image_relocation.rebase(&image, image.len, encoded_base, mapped_base);

    try expectPointer(&image, public_inputs, encoded_base - 8);
    try expectPointer(&image, rounds, encoded_base + len + 8);
    try expectPointer(&image, module_sizes, mapped_base + len);
}
