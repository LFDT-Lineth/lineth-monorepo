// zkVM standard io-interface: read_input / write_output
// https://github.com/eth-act/zkvm-standards/tree/main/standards/io-interface

const std = @import("std");

// The linker-defined start of the IN memory region the proving system maps the private input
// into. The region begins with an 8-byte little-endian u64 payload length, followed by that many
// payload bytes.
extern var _input_start: u8;

/// Size of the linker-defined IN memory region (1 GiB).
const IN_REGION_SIZE: usize = 0x40000000;

/// Read the private input from the memory-mapped IN region: hands back a zero-copy slice into
/// that region via `buf_ptr`/`buf_size`. The input slot's address is this module's own detail —
/// a guest entry point only ever sees the slice.
pub fn read_input(buf_ptr: *[*]const u8, buf_size: *usize) void {
    const base = @intFromPtr(&_input_start);
    const size_ptr: *const u64 = @ptrFromInt(base);
    const payload_len = std.mem.littleToNative(u64, size_ptr.*);
    if (payload_len > IN_REGION_SIZE - 8) @panic("input payload_len exceeds IN region");

    buf_ptr.* = @ptrFromInt(base + 8);
    buf_size.* = @intCast(payload_len);
}

// Append `size` bytes read from the buffer at `output` to the guest's public
// output. Implemented as the Lineth custom opcode (custom-1); the prover's
// RISC-V interpreter turns this into a write_output circuit call.
//
// Signature matches zesu's authoritative extern (zesu-zkvm zkvm/extern_io.zig):
//   @extern(*const fn ([*]const u8, usize) callconv(.c) void, .{ .name = "write_output" })
pub fn write_output(output: [*]const u8, size: usize) callconv(.c) void {
    // opcode format: opcode(0x2b = custom-1) | funct3(0b010) | funct7(0b0000000) | rd(unused) | rs1(input_offset) | rs2(input_size)
    asm volatile (
        \\.insn r 0x2b, 0b010, 0b0000000, x0, %[in], %[size]
        :
        : [in] "r" (@intFromPtr(output)),
          [size] "r" (size),
          // The opcode READS `size` bytes from *output (rs1) and appends them to the
          // guest's public output; it writes nothing back to guest memory. output is
          // passed as an integer (@intFromPtr), so without this memory clobber the
          // optimizer may drop/reorder the buffer's stores before the opcode reads them.
        : .{ .memory = true });
}

