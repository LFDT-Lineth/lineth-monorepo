const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;
pub const zkvm_bytes_32 = types.zkvm_bytes_32;

pub const zkvm_sha256_hash = zkvm_bytes_32;

// https://github.com/eth-act/zkvm-standards/blob/main/standards/c-interface-accelerators/zkvm_accelerators.h#L222
pub fn zkvm_sha256(data: [*c]const u8, len: usize, output: [*c]zkvm_sha256_hash) callconv(.c) zkvm_status {
    if (data == null or output == null) {
        lineth_std.panic();
    }

    // Invoke the SHA-256 custom opcode.
    // Format: opcode(0x0b = custom-0) | funct3(0b000) | funct7(0b0000010) | rd(output) | rs1(input) | rs2(size)
    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0000010, %[out], %[in], %[size]
        :
        : [out] "r" (@intFromPtr(output)),
          [in] "r" (@intFromPtr(data)),
          [size] "r" (len),
          // The opcode writes 32 bytes to *output through rd. Treat rd as an input because it carries
          // the destination address; the memory clobber prevents output-buffer accesses from being
          // dropped, reordered, or satisfied with stale data around the custom instruction.
        : .{ .memory = true });
    return .ZKVM_EOK;
}
