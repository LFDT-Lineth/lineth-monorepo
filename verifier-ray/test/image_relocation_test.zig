const std = @import("std");
const relocation = @import("image_relocation");

test "pair members relocate relative to their own encoded and mapped bases" {
    const guest_base: usize = 0x08800000;
    const mapped_base: usize = 0x50000000;
    const member_offsets = [_]usize{ 16, 0x1238 };
    const payload_offsets = [_]usize{ 112, 0x4560 };

    for (member_offsets, payload_offsets) |member_offset, payload_offset| {
        const encoded_member_base = guest_base + member_offset;
        const mapped_member_base = mapped_base + member_offset;
        const encoded_pointer = encoded_member_base + payload_offset;

        try std.testing.expectEqual(
            payload_offset,
            try relocation.imageOffset(encoded_pointer, encoded_member_base),
        );
        try std.testing.expectEqual(
            mapped_member_base + payload_offset,
            try relocation.relocatePointer(encoded_pointer, encoded_member_base, mapped_member_base),
        );
    }
}

test "single image relocation preserves its existing base-relative behavior" {
    const guest_base: usize = 0x08800000;
    const mapped_base: usize = 0x70000000;
    const payload_offset: usize = 0x3210;

    try std.testing.expectEqual(
        mapped_base + payload_offset,
        try relocation.relocatePointer(guest_base + payload_offset, guest_base, mapped_base),
    );
}

test "relocation rejects pointers outside the encoded image" {
    try std.testing.expectError(
        error.PointerBeforeImage,
        relocation.relocatePointer(0x1000, 0x2000, 0x3000),
    );
    try std.testing.expectError(
        error.AddressOverflow,
        relocation.relocatePointer(0x2001, 0x2000, std.math.maxInt(usize)),
    );
}
