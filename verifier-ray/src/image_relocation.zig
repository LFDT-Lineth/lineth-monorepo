const std = @import("std");

comptime {
    // encoded_pointer/encoded_base/mapped_base are all treated as one 64-bit
    // address space below; every supported target (x86_64, aarch64, R5/RV64)
    // has a 64-bit usize, so no width-narrowing cast is needed or checked for.
    if (@bitSizeOf(usize) != 64) @compileError("image_relocation assumes a 64-bit usize");
}

pub const Error = error{
    PointerBeforeImage,
    AddressOverflow,
};

/// Returns an encoded absolute pointer's byte offset within its image.
pub fn imageOffset(encoded_pointer: u64, encoded_base: usize) Error!usize {
    if (encoded_pointer < encoded_base) return error.PointerBeforeImage;
    return encoded_pointer - encoded_base;
}

/// Relocates an absolute in-image pointer from encoded_base to mapped_base.
pub fn relocatePointer(encoded_pointer: u64, encoded_base: usize, mapped_base: usize) Error!usize {
    const offset = try imageOffset(encoded_pointer, encoded_base);
    return std.math.add(usize, mapped_base, offset) catch error.AddressOverflow;
}
