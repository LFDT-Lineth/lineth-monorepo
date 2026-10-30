//! Freestanding rv64im smoke guest: decompresses the input frame against the embedded dictionary
//! and publishes the decompressed bytes. Exit codes: 0 accepted; 2.. the `zstd.Error` category.

const std = @import("std");
const builtin = @import("builtin");

const lineth_da = @import("lineth_da");
const accelerators = @import("lineth_accelerators");

const zstd = lineth_da.zstd;

extern var _heap_start: u8;
extern var _heap_end: u8;

const max_output_len = 1 << 20;

fn smokeMain() callconv(.c) noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();

    var input_ptr: [*]const u8 = undefined;
    var input_len: usize = undefined;
    accelerators.read_input(&input_ptr, &input_len);

    const out = allocator.alloc(u8, max_output_len) catch accelerators.zkvm_exit(exitCode(error.OutOfMemory));
    const decoder = zstd.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21) catch |err| accelerators.zkvm_exit(exitCode(err));
    const decompressed = decoder.decompress(input_ptr[0..input_len], out) catch |err| accelerators.zkvm_exit(exitCode(err));

    accelerators.write_output(decompressed.ptr, decompressed.len);
    accelerators.zkvm_exit(0);
}

fn exitCode(err: zstd.Error) u32 {
    return switch (err) {
        error.OutOfMemory => 2,
        error.InvalidDictionary => 3,
        error.DictionaryMismatch => 4,
        error.TruncatedFrame => 5,
        error.TrailingBytes => 6,
        error.MalformedFrame => 7,
        error.OutputTooLarge => 8,
    };
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&smokeMain, .{ .name = "main" });
    }
}
