const custom_std = @import("custom_std.zig");

export fn main() noreturn {
    const data: [*c]const u8 = @ptrFromInt(0x1000);
    const len: usize = 64;
    const output: [*c]zkvm_keccak256_hash = @ptrFromInt(0x2000);

    _ = zkvm_keccak256(data, len, output);

    custom_std.exit(0);
}

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L42
pub const zkvm_status = enum(c_int) {
    ZKVM_EOK = 0, // Success
    ZKVM_EFAIL = -1, // Failure
};

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L72
pub const zkvm_keccak256_hash = extern struct {
    data: [32]u8 align(8),
};

// https://github.com/eth-act/zkvm-standards/blob/282cd356c3a0498416bb0619f9c8a347ce9933fb/standards/c-interface-accelerators/zkvm_accelerators.h#L166
export fn zkvm_keccak256(data: [*c]const u8, len: usize, output: [*c]zkvm_keccak256_hash) zkvm_status {
    if (data == null or output == null) {
        custom_std.panic();
    }

    // invoke custom opcode for keccak
    // opcode format: opcode(0x0c = custom-1) | funct3(0b000) | funct7(0b0000000) | rd(output_offset) | rs1(input_offset) | rs2(input_size)
    asm volatile (
        \\.insn r 0x0c, 0b000, 0b0000000, %[out], %[in], %[size]
        :
        : [out] "r" (@intFromPtr(output)),
          [in] "r" (@intFromPtr(data)),
          [size] "r" (len),
    );
    return .ZKVM_EOK;
}
