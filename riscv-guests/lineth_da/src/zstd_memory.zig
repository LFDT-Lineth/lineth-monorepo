//! libzstd's memory hooks (`ZSTD_customMem`) over a Zig allocator, shared by the decoder and the
//! encoder.

const std = @import("std");

const c = @import("zstd_c");

const Allocator = std.mem.Allocator;

pub const Memory = struct {
    allocator: Allocator,
    /// Set when the allocator refused a request from libzstd.
    exhausted: bool = false,

    /// The hooks that serve libzstd from `allocator`. `self` has to stay at its address until
    /// every object created with them is freed.
    pub fn hooks(self: *Memory) c.ZSTD_customMem {
        return .{ .customAlloc = allocate, .customFree = release, .@"opaque" = self };
    }
};

// libzstd allocations carry a header holding the block size, so `release` can return the exact
// block to the allocator. The header keeps the payload 16-byte aligned.
const block_alignment: std.mem.Alignment = .@"16";
const header_len = block_alignment.toByteUnits();

fn allocate(opaque_memory: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque {
    const memory: *Memory = @ptrCast(@alignCast(opaque_memory));
    const total = std.math.add(usize, size, header_len) catch return refuse(memory);
    const block = memory.allocator.alignedAlloc(u8, block_alignment, total) catch return refuse(memory);
    const header: *usize = @ptrCast(block.ptr);
    header.* = total;
    return block.ptr + header_len;
}

fn release(opaque_memory: ?*anyopaque, address: ?*anyopaque) callconv(.c) void {
    const memory: *Memory = @ptrCast(@alignCast(opaque_memory));
    const payload: [*]u8 = @ptrCast(address orelse return);
    const base: [*]align(block_alignment.toByteUnits()) u8 = @alignCast(payload - header_len);
    const header: *const usize = @ptrCast(base);
    memory.allocator.free(base[0..header.*]);
}

fn refuse(memory: *Memory) ?*anyopaque {
    memory.exhausted = true;
    return null;
}
