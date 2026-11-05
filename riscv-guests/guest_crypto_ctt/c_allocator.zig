//! C allocator adapter for the Constantine archive on a freestanding guest.
//! A 16-byte header before each allocation stores the payload size.

const std = @import("std");

// `extern` vars cannot hold slices, so this mirrors `std.mem.Allocator`'s two-word layout.
const AllocatorWords = extern struct { ptr: *anyopaque, vtable: *const anyopaque };
extern var guest_allocator: AllocatorWords;

const MIN_ALIGN: std.mem.Alignment = .@"16";
const HDR: usize = 16; // [8]=payload size, [8]=reserved

fn allocator() *std.mem.Allocator {
    return @ptrCast(&guest_allocator);
}

// Over-allocates for alignment and leaves the payload behind the header.
fn allocImpl(size: usize, align_req: std.mem.Alignment) ?*anyopaque {
    const alignment: std.mem.Alignment = if (@intFromEnum(align_req) > @intFromEnum(MIN_ALIGN)) align_req else MIN_ALIGN;
    const total = alignment.toByteUnits() + HDR + size;
    const raw = allocator().rawAlloc(total, MIN_ALIGN, 0) orelse return null;
    const addr = std.mem.alignForward(usize, @intFromPtr(raw) + HDR, alignment.toByteUnits());
    const hdr: [*]usize = @ptrFromInt(addr - HDR);
    hdr[0] = size;
    hdr[1] = 0;
    return @ptrFromInt(addr);
}

fn payloadOf(p: *anyopaque) []u8 {
    const raw: [*]u8 = @ptrFromInt(@intFromPtr(p) - HDR);
    const hdr: [*]const usize = @ptrCast(@alignCast(raw));
    return raw[0 .. HDR + hdr[0]];
}

export fn malloc(size: usize) ?*anyopaque {
    return allocImpl(size, MIN_ALIGN);
}

export fn aligned_alloc(alignment: usize, size: usize) ?*anyopaque {
    if (!std.math.isPowerOfTwo(alignment)) return null;
    return allocImpl(size, @enumFromInt(@ctz(alignment)));
}

// POSIX `posix_memalign` requires a power-of-two multiple of `sizeof(void*)`.
export fn posix_memalign(out: *?*anyopaque, alignment: usize, size: usize) c_int {
    if (!std.math.isPowerOfTwo(alignment) or alignment % @sizeOf(*anyopaque) != 0) return 22; // EINVAL
    const p = allocImpl(size, @enumFromInt(@ctz(alignment))) orelse return 12; // ENOMEM
    out.* = p;
    return 0;
}

export fn calloc(n: usize, size: usize) ?*anyopaque {
    const total = n * size;
    const p = allocImpl(total, MIN_ALIGN) orelse return null;
    @memset(@as([*]u8, @ptrCast(p))[0..total], 0);
    return p;
}

export fn free(ptr: ?*anyopaque) void {
    _ = ptr;
}

export fn realloc(ptr: ?*anyopaque, size: usize) ?*anyopaque {
    const p = ptr orelse return malloc(size);
    const old = payloadOf(p);
    const np = allocImpl(size, MIN_ALIGN) orelse return null;
    const keep = @min(size, old.len - HDR);
    @memcpy(@as([*]u8, @ptrCast(np))[0..keep], @as([*]const u8, @ptrCast(p))[0..keep]);
    return np;
}
