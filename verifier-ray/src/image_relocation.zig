const std = @import("std");

pub const Error = error{
    PointerBeforeImage,
    AddressOverflow,
};

/// Returns an encoded absolute pointer's byte offset within its image.
pub fn imageOffset(encoded_pointer: u64, encoded_base: usize) Error!usize {
    const base = std.math.cast(u64, encoded_base) orelse return error.AddressOverflow;
    if (encoded_pointer < base) return error.PointerBeforeImage;
    return std.math.cast(usize, encoded_pointer - base) orelse error.AddressOverflow;
}

/// Relocates an absolute in-image pointer from encoded_base to mapped_base.
pub fn relocatePointer(encoded_pointer: u64, encoded_base: usize, mapped_base: usize) Error!usize {
    const offset = try imageOffset(encoded_pointer, encoded_base);
    return std.math.add(usize, mapped_base, offset) catch error.AddressOverflow;
}
