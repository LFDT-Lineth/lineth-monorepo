// Field accelerator: one ECALL for multiplication, division and square root in a known 256-bit
// or 384-bit prime field, proven by the arithmetization (arithmetization/src/main/lib/field/field.zkc).
//
// See include/lineth_accelerators.h for the contract.

/// ECALL number (a7). Kept in sync with FIELD_OP_SYSCALL in arithmetization/src/main/common/constants.zkc.
const syscall_number: u64 = 0xF0;

inline fn fieldOpSyscall(op: u32, field: u32, out_addr: usize, a_addr: usize, b_addr: usize) u32 {
    // a0..a4 already hold the arguments under the C ABI, so each wrapper compiles to `li a7; ecall; ret`.
    return asm volatile ("ecall"
        : [status] "={x10}" (-> u32),
        : [op] "{x10}" (op),
          [field] "{x11}" (field),
          [out] "{x12}" (out_addr),
          [a] "{x13}" (a_addr),
          [b] "{x14}" (b_addr),
          [n] "{x17}" (syscall_number),
        : .{ .memory = true });
}

/// 256-bit (4-limb, 32-byte) field operation for LINETH_FIELD_* ids 1..7.
pub fn lineth_zkvm_field_op_256(op: u32, field: u32, out: *[4]u64, a: *const [4]u64, b: *const [4]u64) callconv(.c) u32 {
    return fieldOpSyscall(op, field, @intFromPtr(out), @intFromPtr(a), @intFromPtr(b));
}

/// 384-bit (6-limb, 48-byte) field operation for LINETH_FIELD_BLS12_381_FP (8).
pub fn lineth_zkvm_field_op_384(op: u32, field: u32, out: *[6]u64, a: *const [6]u64, b: *const [6]u64) callconv(.c) u32 {
    return fieldOpSyscall(op, field, @intFromPtr(out), @intFromPtr(a), @intFromPtr(b));
}
