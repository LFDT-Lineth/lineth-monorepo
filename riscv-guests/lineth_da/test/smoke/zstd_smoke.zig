//! Freestanding rv64im smoke guest: decompresses a frame against the embedded dictionary and
//! publishes the decompressed bytes.
//!
//! Input: the length of the decompressed bytes (4 bytes, big endian), then the frame.
//!
//! Exit codes: 0 accepted; 2 input malformed; 3.. the `zstd_decoder.Error` category.

const std = @import("std");
const builtin = @import("builtin");

const lineth_da = @import("lineth_da");
const accelerators = @import("lineth_accelerators");

const zstd_decoder = lineth_da.zstd_decoder;

extern var _heap_start: u8;
extern var _heap_end: u8;

fn smokeMain() callconv(.c) noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();

    var input_ptr: [*]const u8 = undefined;
    var input_len: usize = undefined;
    accelerators.read_input(&input_ptr, &input_len);

    if (input_len < 4) accelerators.zkvm_exit(2);
    const input = input_ptr[0..input_len];
    const out = allocator.alloc(u8, std.mem.readInt(u32, input[0..4], .big)) catch accelerators.zkvm_exit(exitCode(error.OutOfMemory));
    const decoder = zstd_decoder.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21) catch |err| accelerators.zkvm_exit(exitCode(err));
    decoder.decompress(input[4..], out) catch |err| accelerators.zkvm_exit(exitCode(err));

    accelerators.write_output(out.ptr, out.len);
    accelerators.zkvm_exit(0);
}

fn exitCode(err: zstd_decoder.Error) u32 {
    return switch (err) {
        error.OutOfMemory => 3,
        error.InvalidDictionary => 4,
        error.TruncatedFrame => 5,
        error.TrailingBytes => 6,
        error.MalformedFrame => 7,
        error.MissingContentSize => 8,
        error.ContentSizeMismatch => 9,
        error.MissingDictionaryId => 10,
        error.DictionaryMismatch => 11,
        error.ChecksumPresent => 12,
        error.WindowTooLarge => 13,
    };
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&smokeMain, .{ .name = "main" });
    }
}
