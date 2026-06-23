const custom_std = @import("custom_std.zig");

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L42
pub const zkvm_status = enum(c_int) {
    ZKVM_EOK = 0, // Success
    ZKVM_EFAIL = -1, // Failure
};

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L72
pub const zkvm_bytes_32 = extern struct {
    data: [32]u8 align(8),
};

pub const zkvm_keccak256_hash = zkvm_bytes_32;

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L166
pub fn zkvm_keccak256(data: [*c]const u8, len: usize, output: [*c]zkvm_keccak256_hash) callconv(.c) zkvm_status {
    if (data == null or output == null) {
        custom_std.panic();
    }

    // invoke custom opcode for keccak
    // opcode format: opcode(0x2b = custom-1) | funct3(0b000) | funct7(0b0000000) | rd(output_offset) | rs1(input_offset) | rs2(input_size)
    asm volatile (
        \\.insn r 0x2b, 0b000, 0b0000000, %[out], %[in], %[size]
        :
        : [out] "r" (@intFromPtr(output)),
          [in] "r" (@intFromPtr(data)),
          [size] "r" (len),
          // The opcode writes 32 bytes to *output through rd. output is passed as an integer
          // (@intFromPtr), so without this memory clobber the optimizer assumes the asm touches no
          // memory and may drop/reorder/stale-read the output buffer in the emitted ELF.
        : .{ .memory = true });
    return .ZKVM_EOK;
}
