//! C allocator API adapter the Constantine archive needs on a freestanding guest (ARC's useMalloc).
//! Every entry point delegates to the guest's own allocator: `guest_allocator` is the
//! FixedBufferAllocator over `_heap_start`, exported by the guest root (evm_execution_guest.zig).
//! A 16-byte header before each returned pointer records the size, which `free`/`realloc` need
//! to call back into rawFree/rawRemap.

const std = @import("std");

// extern vars can't have slice-containing types, so this mirrors std.mem.Allocator's
// two-word (ptr, vtable) layout and pointer-casts at use.
const AllocatorWords = extern struct { ptr: *anyopaque, vtable: *const anyopaque };
extern var guest_allocator: AllocatorWords;

const MIN_ALIGN: std.mem.Alignment = .@"16";
const HDR: usize = 16; // [8]=payload size, [8]=reserved

fn allocator() *std.mem.Allocator {
    return @ptrCast(&guest_allocator);
}

// rawAlloc over-allocates by the alignment and the payload sits HDR bytes past an aligned
// boundary; the header records the payload size. rawFree/rawRemap are given the payload slice
// at MIN_ALIGN: exact for minimum-aligned payloads, and for over-aligned ones the FBA's
// free is a no-op unless the block is the most recent allocation, so the slack is harmless.
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

// POSIX spelling Constantine's posixMemalign wrapper imports; same contract as aligned_alloc
// plus the C89 constraint (alignment is a power-of-two multiple of sizeof(void*)).
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
    const p = ptr orelse return;
    allocator().rawFree(payloadOf(p), MIN_ALIGN, 0);
}

export fn realloc(ptr: ?*anyopaque, size: usize) ?*anyopaque {
    const p = ptr orelse return malloc(size);
    const old = payloadOf(p);
    const np = allocImpl(size, MIN_ALIGN) orelse return null;
    const keep = @min(size, old.len - HDR);
    @memcpy(@as([*]u8, @ptrCast(np))[0..keep], @as([*]const u8, @ptrCast(p))[0..keep]);
    allocator().rawFree(old, MIN_ALIGN, 0);
    return np;
}
