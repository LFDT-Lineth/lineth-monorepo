const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;
pub const zkvm_bytes_32 = types.zkvm_bytes_32;
pub const zkvm_bytes_64 = types.zkvm_bytes_64;

// EIP-196 encodings: a G1 point is x‖y (two 32-byte big-endian Fp elements) and
// the ECMUL scalar is a 32-byte big-endian integer.
pub const zkvm_bn254_g1_point = zkvm_bytes_64;
pub const zkvm_bn254_scalar = zkvm_bytes_32;

// BN254 (alt_bn128) G1 point addition — EVM ECADD precompile (0x06, EIP-196).
//
// Custom opcode (kept in sync with arithmetization/src/main/riscv/utils/constants.zkc):
// opcode(0x0b = custom-0) | funct3(0b000) | funct7(0b0000110)
//   rs1 = input ptr  (x1‖y1‖x2‖y2, 128 bytes big-endian)
//   rs2 = output ptr (x3‖y3, 64 bytes big-endian)
//   rd  <- status (1 = success, 0 = invalid input)
//
// The R-type custom instruction carries only three register operands, and the
// accelerator reads the two input points as one contiguous 128-byte block, so p1
// and p2 are packed into a single buffer here (the C ABI does not require them to
// be adjacent).
//
// Unlike ecrecover/sha2 (which always succeed and signal failure via a zeroed
// output), ECADD must report validity: an invalid point (a coordinate >= p or a
// finite point off the curve) is an EVM precompile FAILURE that reverts the
// calling frame. So the accelerator returns a status in rd, which is mapped to
// ZKVM_EOK / ZKVM_EFAIL for the zesu caller.
pub fn zkvm_bn254_g1_add(
    p1: [*c]const zkvm_bn254_g1_point,
    p2: [*c]const zkvm_bn254_g1_point,
    result: [*c]zkvm_bn254_g1_point,
) callconv(.c) zkvm_status {
    if (p1 == null or p2 == null or result == null) {
        lineth_std.panic();
    }

    var input: [128]u8 align(8) = undefined;
    const p1_bytes: [*]const u8 = @ptrCast(p1);
    const p2_bytes: [*]const u8 = @ptrCast(p2);
    var i: usize = 0;
    while (i < 64) : (i += 1) {
        input[i] = p1_bytes[i];
        input[64 + i] = p2_bytes[i];
    }

    var status: usize = undefined;
    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0000110, %[status], %[in], %[out]
        : [status] "=r" (status),
        : [in] "r" (@intFromPtr(&input)),
          [out] "r" (@intFromPtr(result)),
          // rd returns the status; the memory clobber keeps the input reads and
          // the output store ordered around the custom instruction.
        : .{ .memory = true });
    return if (status != 0) .ZKVM_EOK else .ZKVM_EFAIL;
}

// BN254 (alt_bn128) G1 scalar multiplication — EVM ECMUL precompile (0x07, EIP-196).
//
// Custom opcode (kept in sync with arithmetization/src/main/riscv/utils/constants.zkc):
// opcode(0x0b = custom-0) | funct3(0b000) | funct7(0b0000111)
//   rs1 = input ptr  (x‖y‖scalar, 96 bytes big-endian)
//   rs2 = output ptr (x3‖y3, 64 bytes big-endian)
//   rd  <- status (1 = success, 0 = invalid input)
//
// The point and scalar are packed into one contiguous 96-byte block for the same
// reason as ECADD. The scalar is the full 256-bit value (not reduced mod the
// group order). See zkvm_bn254_g1_add for the status/failure rationale.
pub fn zkvm_bn254_g1_mul(
    point: [*c]const zkvm_bn254_g1_point,
    scalar: [*c]const zkvm_bn254_scalar,
    result: [*c]zkvm_bn254_g1_point,
) callconv(.c) zkvm_status {
    if (point == null or scalar == null or result == null) {
        lineth_std.panic();
    }

    var input: [96]u8 align(8) = undefined;
    const point_bytes: [*]const u8 = @ptrCast(point);
    const scalar_bytes: [*]const u8 = @ptrCast(scalar);
    var i: usize = 0;
    while (i < 64) : (i += 1) {
        input[i] = point_bytes[i];
    }
    i = 0;
    while (i < 32) : (i += 1) {
        input[64 + i] = scalar_bytes[i];
    }

    var status: usize = undefined;
    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0000111, %[status], %[in], %[out]
        : [status] "=r" (status),
        : [in] "r" (@intFromPtr(&input)),
          [out] "r" (@intFromPtr(result)),
        : .{ .memory = true });
    return if (status != 0) .ZKVM_EOK else .ZKVM_EFAIL;
}
