//! Freestanding rv64im smoke guest: truncates the block in the input, round-trips the encoded
//! payload through the 254-bit packing of a blob and publishes the block hash, the parent hash and
//! the payload. The input is the chain ID as 8 big-endian bytes followed by the block's RLP. Exit
//! codes: 0 truncated; 2 input too short; 3 malformed block; 4 sender not recoverable; 5 out of
//! memory; 6 payload not preserved by the packing.

const std = @import("std");
const builtin = @import("builtin");

const lineth_da = @import("lineth_da");
const accelerators = @import("lineth_accelerators");
const zkvm_provide = @import("zkvm_provide");

const blob_payload = lineth_da.encoding.blob_payload;
const truncation = lineth_da.encoding.truncation;

extern var _heap_start: u8;
extern var _heap_end: u8;

fn smokeMain() callconv(.c) noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();
    zkvm_provide.setAllocator(allocator);

    var input_ptr: [*]const u8 = undefined;
    var input_len: usize = undefined;
    accelerators.read_input(&input_ptr, &input_len);
    if (input_len < 8) accelerators.zkvm_exit(2);
    const chain_id = std.mem.readInt(u64, input_ptr[0..8], .big);
    const block_rlp = input_ptr[8..input_len];

    const block = truncation.truncateBlockRlp(allocator, allocator, block_rlp, chain_id) catch |err| accelerators.zkvm_exit(switch (err) {
        error.MalformedBlock => 3,
        error.SenderRecoveryFailed => 4,
        error.OutOfMemory => 5,
    });
    const payload = truncation.encodeBlocks(allocator, allocator, &.{block}) catch accelerators.zkvm_exit(5);

    var blob: [blob_payload.blob_len]u8 = undefined;
    var stream: [blob_payload.stream_len]u8 = undefined;
    if (payload.len <= blob_payload.payload_capacity) {
        blob_payload.pack(payload, &blob) catch accelerators.zkvm_exit(6);
        const unpacked = blob_payload.unpack(&blob, &stream) catch accelerators.zkvm_exit(6);
        if (!std.mem.eql(u8, unpacked, payload)) accelerators.zkvm_exit(6);
    }

    const out = std.mem.concat(allocator, u8, &.{ &block.block_hash, &block.parent_hash, payload }) catch accelerators.zkvm_exit(5);
    accelerators.write_output(out.ptr, out.len);
    accelerators.zkvm_exit(0);
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&smokeMain, .{ .name = "main" });
        _ = zkvm_provide;
    }
}
