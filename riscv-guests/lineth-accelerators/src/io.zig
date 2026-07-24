const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;

pub fn zkvm_write_output(output: [*c]const u8, size: usize) callconv(.c) zkvm_status {
    if (output == null) {
        lineth_std.panic();
    }

    size = output.len();

    // invoke custom opcode for keccak
    // opcode format: opcode(0x2b = custom-1) | funct3(0b000) | funct7(0b0000000) | rd(output_offset) | rs1(input_offset) | rs2(input_size)
    asm volatile (
        \\.insn r 0x2b, 0b010, 0b0000000, 0, %[in], %[size]
        :
        : [in] "r" (@intFromPtr(output)),
          [size] "r" (size),
          // The opcode writes 32 bytes to *output through rd. output is passed as an integer
          // (@intFromPtr), so without this memory clobber the optimizer assumes the asm touches no
          // memory and may drop/reorder/stale-read the output buffer in the emitted ELF.
        : .{ .memory = true });
    return .ZKVM_EOK;
}
