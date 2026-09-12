//! Implements `read_input` over the linker-defined IN region: an eight-byte little-endian length
//! followed by the payload.

const std = @import("std");

extern var _in_start: u8;

/// Linker-defined 1 GiB input region.
const IN_REGION_SIZE: usize = 0x40000000;

pub fn read_input(buf_ptr: *[*]const u8, buf_size: *usize) void {
    const base: [*]const u8 = @ptrCast(&_in_start);
    const payload_len = std.mem.readInt(u64, base[0..8], .little);
    if (payload_len > IN_REGION_SIZE - 8) @panic("input payload_len exceeds IN region");
    buf_ptr.* = base + 8;
    buf_size.* = @intCast(payload_len);
}
