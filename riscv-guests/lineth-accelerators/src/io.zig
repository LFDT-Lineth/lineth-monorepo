// zkVM standard io-interface: void write_output(const uint8_t* output, size_t size)
// https://github.com/eth-act/zkvm-standards/tree/main/standards/io-interface
//
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

