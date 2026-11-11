// Field accelerator: one ECALL for multiplication, division and square root in a known 256-bit
// prime field, proven by the arithmetization (arithmetization/src/main/lib/field/field.zkc).
//
// See include/lineth_accelerators.h for the contract.

/// ECALL number (a7). Kept in sync with FIELD_OP_SYSCALL in arithmetization/src/main/common/constants.zkc.
const syscall_number: u64 = 0xF0;

/// out = a*b, a/b (0 if b = 0) or a square root of a (or of -a if a is not a square) in `field`.
/// Operands are four little-endian 64-bit limbs. Returns 1 when there is no result (b = 0, or a
/// is not a square), 0 otherwise.
pub fn lineth_zkvm_field_op(op: u32, field: u32, out: *[4]u64, a: *const [4]u64, b: *const [4]u64) callconv(.c) u32 {
    // a0..a4 already hold the arguments under the C ABI, so this compiles to `li a7; ecall; ret`.
    return asm volatile ("ecall"
        : [status] "={x10}" (-> u32),
        : [op] "{x10}" (op),
          [field] "{x11}" (field),
          [out] "{x12}" (@intFromPtr(out)),
          [a] "{x13}" (@intFromPtr(a)),
          [b] "{x14}" (@intFromPtr(b)),
          [n] "{x17}" (syscall_number),
          // The call writes 32 bytes to *out and reads *a and *b.
        : .{ .memory = true });
}
