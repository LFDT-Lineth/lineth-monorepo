const lineth_std = @import("std.zig");
const types = @import("zkvm_types.zig");

pub const zkvm_status = types.zkvm_status;

// EIP-2537 encodings, with the 16-byte-per-limb calldata padding already
// stripped by the caller: a G1 point is x‖y (two 48-byte big-endian Fp
// elements) and a G2 point is x.c0‖x.c1‖y.c0‖y.c1 (four of them).
//
// NOTE THE Fp2 COEFFICIENT ORDER: c0 BEFORE c1. EIP-2537 does not use the
// ZCash/blst "c1 first" serialisation that most BLS12-381 tooling defaults to,
// and the arithmetization decodes these bytes in exactly this order (see
// arithmetization/src/main/lib/bls12_381/impl.zkc).
pub const zkvm_bls12_381_g1_point = [96]u8;
pub const zkvm_bls12_381_g2_point = [192]u8;

// Deliberately raw byte arrays rather than the header's zkvm_bytes_96 /
// zkvm_bytes_192, which declare `align(8)`. The buffer zesu actually passes is
// an allocation of `extern struct { g1: [96]u8, g2: [192]u8 }` — alignment 1 —
// so claiming 8 here would be promising the compiler something the caller does
// not provide. Nothing below dereferences the pointer, but the declaration
// should still be true. Layout is identical either way: 288 bytes, no padding.
pub const zkvm_bls12_381_pairing_pair = extern struct {
    g1: zkvm_bls12_381_g1_point,
    g2: zkvm_bls12_381_g2_point,
};

// Status values returned in rd by the accelerator. Kept in sync with
// bls12_pairing_check in arithmetization/src/main/lib/bls12_381/impl.zkc.
const STATUS_INVALID: usize = 0;
const STATUS_VALID_NOT_ONE: usize = 1;
const STATUS_VALID_ONE: usize = 2;

// BLS12-381 pairing check — EVM BLS12_PAIRING_CHECK precompile (0x0f, EIP-2537).
//
// Custom opcode (kept in sync with arithmetization/src/main/common/constants.zkc):
// opcode(0x0b = custom-0) | funct3(0b000) | funct7(0b0001111)
//   rs1 = pair array ptr (num_pairs * 288 bytes, unpadded, big-endian)
//   rs2 = num_pairs
//   rd  <- status (0 = invalid input, 1 = valid and != 1, 2 = valid and == 1)
//
// Unlike the ECADD/ECMUL wrappers, nothing is packed or copied here: the whole
// result is two bits, so it fits in rd and all three operands fit in the
// R-type's three registers. The pair array is passed by pointer exactly as the
// caller laid it out.
//
// THE TWO SIGNALS ARE INDEPENDENT, and the C ABI needs both:
//   - a non-zero return means the INPUT was rejected (a coordinate >= p, a
//     point off the curve, or a point outside the prime-order subgroup). That
//     is an EVM precompile failure and the calling frame reverts.
//   - a zero return with verified = false means the input was fine and the
//     product of pairings simply is not 1. The precompile SUCCEEDS and returns
//     bytes32(0).
// Collapsing them would turn every "pairing != 1" answer into a frame revert.
//
// num_pairs is passed through unbounded. The arithmetization's staging area
// holds 64 pairs and it processes longer calls in successive chunks, so there is
// no count this wrapper has to screen for — only proving cost grows, linearly.
pub fn zkvm_bls12_pairing(
    pairs: [*c]const zkvm_bls12_381_pairing_pair,
    num_pairs: usize,
    verified: [*c]bool,
) callconv(.c) zkvm_status {
    if (pairs == null or verified == null) {
        lineth_std.panic();
    }

    var status: usize = undefined;
    asm volatile (
        \\.insn r 0x0b, 0b000, 0b0001111, %[status], %[pairs], %[count]
        : [status] "=r" (status),
        : [pairs] "r" (@intFromPtr(pairs)),
          [count] "r" (num_pairs),
          // rd returns the status; the memory clobber keeps the reads of the
          // pair array ordered around the custom instruction.
        : .{ .memory = true });

    if (status == STATUS_INVALID) {
        verified.* = false;
        return .ZKVM_EFAIL;
    }
    verified.* = (status == STATUS_VALID_ONE);
    return .ZKVM_EOK;
}
