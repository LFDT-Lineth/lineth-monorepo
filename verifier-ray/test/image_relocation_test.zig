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
