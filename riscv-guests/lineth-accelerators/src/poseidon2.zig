const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;
pub const zkvm_bytes_64 = types.zkvm_bytes_64;

// The Poseidon2 state is 16 KoalaBear field elements. Each element is a
// canonical value in [0, 2^31 - 2^24 + 1) and is laid out as a native
// little-endian 32-bit word, matching the word-addressed RAM marshalling on the
// zkc side (read_32 / write_32). The 16 * 4 = 64 bytes are passed as a
// zkvm_bytes_64.

// Apply the Poseidon2 permutation to the state at `input`, writing the permuted
// state to `output`. `input` and `output` may alias.
pub fn lineth_zkvm_poseidon2_permutation(input: [*c]const zkvm_bytes_64, output: [*c]zkvm_bytes_64) callconv(.c) zkvm_status {
    if (input == null or output == null) {
        lineth_std.panic();
    }

    // invoke custom opcode for the Poseidon2 permutation. Kept in sync with arithmetization/src/main/riscv/utils/constants.zkc
    // opcode format: opcode(0x2b = custom-1) | funct3(0b001) | funct7(0b0000000) | rd(output_offset) | rs1(input_offset) | rs2(unused)
    asm volatile (
        \\.insn r 0x2b, 0b001, 0b0000000, %[out], %[in], %[unused]
        :
        : [out] "r" (@intFromPtr(output)),
          [in] "r" (@intFromPtr(input)),
          [unused] "r" (0),
          // The opcode writes 16 words to *output through rd. output is passed as an
          // integer (@intFromPtr), so without this memory clobber the optimizer assumes the asm
          // touches no memory and may drop/reorder/stale-read the output buffer in the emitted ELF.
        : .{ .memory = true });
    return .ZKVM_EOK;
}
