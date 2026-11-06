// secp256k1 base-field multiplication / squaring accelerator.
//
// Each operand is a 32-byte, 8-byte-aligned, little-endian array of four u64
// (the limb layout of Constantine's Fp[secp256k1]). Inputs may be any 256-bit
// value; the result is canonical (< p). The destination may alias a source.
//
// Only provable under arithmetization's `riscv/main_fieldaccel.zkc`: the default
// `riscv/main.zkc` fails on this instruction.

pub const Fp = extern struct {
    limbs: [4]u64 align(8),
};

/// out = a * b mod p
// opcode format: opcode(0x2b = custom-1) | funct3(0b011) | funct7(0b0000000) | rd(out ptr) | rs1(a ptr) | rs2(b ptr)
pub inline fn fpMul(out: *Fp, a: *const Fp, b: *const Fp) void {
    asm volatile (
        \\.insn r 0x2b, 0b011, 0b0000000, %[out], %[a], %[b]
        :
        : [out] "r" (@intFromPtr(out)),
          [a] "r" (@intFromPtr(a)),
          [b] "r" (@intFromPtr(b)),
        : .{ .memory = true });
}

/// out = a * a mod p
// opcode format: opcode(0x2b = custom-1) | funct3(0b011) | funct7(0b0000001) | rd(out ptr) | rs1(a ptr) | rs2(ignored)
pub inline fn fpSq(out: *Fp, a: *const Fp) void {
    asm volatile (
        \\.insn r 0x2b, 0b011, 0b0000001, %[out], %[a], x0
        :
        : [out] "r" (@intFromPtr(out)),
          [a] "r" (@intFromPtr(a)),
        : .{ .memory = true });
}

/// out = a^-1 mod p (0 if a = 0 mod p)
// opcode format: opcode(0x2b = custom-1) | funct3(0b011) | funct7(0b0000010) | rd(out ptr) | rs1(a ptr) | rs2(ignored)
pub inline fn fpInv(out: *Fp, a: *const Fp) void {
    asm volatile (
        \\.insn r 0x2b, 0b011, 0b0000010, %[out], %[a], x0
        :
        : [out] "r" (@intFromPtr(out)),
          [a] "r" (@intFromPtr(a)),
        : .{ .memory = true });
}

/// Square-root result: `root` is a square root of `a` when `is_square` is 1 (r = a^((p+1)/4), which is
/// either root), and otherwise a square root of -a.
pub const FpSqrt = extern struct {
    root: Fp align(8),
    is_square: u64,
};

// opcode format: opcode(0x2b = custom-1) | funct3(0b011) | funct7(0b0000011) | rd(out ptr, 40 bytes) | rs1(a ptr) | rs2(ignored)
pub inline fn fpSqrt(out: *FpSqrt, a: *const Fp) void {
    asm volatile (
        \\.insn r 0x2b, 0b011, 0b0000011, %[out], %[a], x0
        :
        : [out] "r" (@intFromPtr(out)),
          [a] "r" (@intFromPtr(a)),
        : .{ .memory = true });
}
