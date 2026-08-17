const std = @import("std");
const builtin = @import("builtin");
const lineth_accel = @import("lineth_zkvm_accel");

// Minimal guest for main.zkc codegen bootstrap fixtures (verifier-ray/codegen).
//
// main.zkc's dynamic modules (one per RISC-V opcode family it supports: keccak, poseidon2,
// div/mul/rem, RAM/register bookkeeping, syscall dispatch, ...) are checked by AIR constraints
// that reference neighboring rows (shifts like {0, +1, -1}), fixed at compile time regardless of
// witness — see prover-ray/zkcdriver/definition.go's compColumnByCorsetColumnAccess. A cyclic
// domain of size N aliases shift -1 onto row N-1, so a {0,+1,-1} schedule needs N >= 4 real rows
// to stay collision-free (verifier-ray/codegen/pcs.go's pcsDynamicMinSizeLog2). A guest that only
// exercises a narrow slice of opcodes (e.g. verifier-ray's own R5 smoke-test guest, poseidon2
// only; or an earlier version of this guest, keccak/write_output only) leaves every OTHER dynamic
// module at the padding floor (size 1-2), which BuildPcsSystem's dynamic-minimum-size guard
// correctly rejects as unsound.
//
// This guest exists purely to produce a witness that clears that floor on every module a small
// program can plausibly reach, without approaching main.zkc's current static column caps (a full
// EVM block overflows those — see riscv-guests/l2-execution). It deliberately exercises a WIDE
// instruction mix: both hash accelerators (keccak, poseidon2), all four M-extension operation
// families at both register widths and both signednesses (mul/div/rem, 32-bit W-suffixed and
// 64-bit), sign-extension, real RAM traffic (array reads/writes wider than a register), and
// multiple function calls / syscalls (JAL, ecall) — so interpreter bookkeeping modules
// (ram/registers/read_16/write_16/process_syscall/process_J_type_instruction) also clear their
// floor, not just the two hash accelerators.
//
// volatile globals prevent the optimizer from constant-folding every operation away (this is
// ReleaseSmall freestanding code with no OS to observe side effects through).
var sink: u64 = 0;

// The M-extension opcodes below are emitted via inline asm, not Zig's `/`/`%`/`*%` operators:
// LLVM's RV64 backend legally lowers those to whichever instruction produces the same VALUE
// (e.g. a 32-bit truncating multiply doesn't need MULW's specific sign-extending behavior unless
// that behavior is actually observed, so it emits plain MUL + a truncate instead) — verified by
// disassembling an earlier version of this file with riscv64-linux-gnu-objdump: MULW, REM, REMU,
// REMW, REMUW, LH, LHU, SH, SW, and direct JAL never appeared despite equivalent Zig source, while
// MUL/MULH/MULHU/MULHSU/DIV/DIVU/DIVW/DIVUW/SRA/SRAW did. Hand-written asm makes the opcode a hard
// requirement instead of a value the optimizer is free to reach another way.
fn touchArithmetic(a: u64, b: u64) void {
    const ai: i64 = @bitCast(a);
    const bi: i64 = @bitCast(b);

    // 64-bit M-extension ops: MUL, DIV/DIVU (verified emitted without asm), REM/REMU (asm).
    sink +%= a *% b;
    sink +%= a / b;
    sink +%= @as(u64, @bitCast(@divTrunc(ai, bi)));
    sink +%= asmRem(a, b);
    sink +%= @bitCast(asmRemSigned(ai, bi));

    // MULH/MULHU/MULHSU (mul64_ss/mul64_uu/mul64_su): the HIGH 64 bits of a
    // 128-bit product — a widening multiply, not the truncating `*%` above,
    // which only ever produces the low word and so never lowers to these
    // opcodes regardless of operand signedness.
    const uu: u128 = @as(u128, a) * @as(u128, b);
    sink +%= @truncate(uu >> 64);
    const ss: i128 = @as(i128, ai) * @as(i128, bi);
    sink +%= @bitCast(@as(i64, @truncate(ss >> 64)));
    const su: i128 = @as(i128, ai) * @as(i128, @as(i128, b));
    sink +%= @bitCast(@as(i64, @truncate(su >> 64)));

    // 32-bit W-suffixed M-extension ops (MULW, DIVW, DIVUW verified emitted without asm, REMW,
    // REMUW): operate on the low 32 bits, sign-extending the result back to 64 — a separate
    // opcode family/module from their 64-bit counterparts above.
    const a32: u32 = @truncate(a);
    const b32: u32 = @truncate(b | 1);
    const a32i: i32 = @bitCast(a32);
    const b32i: i32 = @bitCast(b32);
    sink +%= asmMulw(a32, b32);
    sink +%= a32 / b32;
    sink +%= @as(u32, @bitCast(@divTrunc(a32i, b32i)));
    sink +%= asmRemw(a32, b32);
    sink +%= @as(u32, @bitCast(asmRemwSigned(a32i, b32i)));

    // signed_LT / comparisons.
    if (ai < bi) sink +%= 1;
}

fn asmMulw(a: u32, b: u32) u32 {
    return asm volatile ("mulw %[ret], %[a], %[b]"
        : [ret] "=r" (-> u32),
        : [a] "r" (a),
          [b] "r" (b),
    );
}

fn asmRem(a: u64, b: u64) u64 {
    return asm volatile ("remu %[ret], %[a], %[b]"
        : [ret] "=r" (-> u64),
        : [a] "r" (a),
          [b] "r" (b),
    );
}

fn asmRemSigned(a: i64, b: i64) i64 {
    return asm volatile ("rem %[ret], %[a], %[b]"
        : [ret] "=r" (-> i64),
        : [a] "r" (a),
          [b] "r" (b),
    );
}

fn asmRemw(a: u32, b: u32) u32 {
    return asm volatile ("remuw %[ret], %[a], %[b]"
        : [ret] "=r" (-> u32),
        : [a] "r" (a),
          [b] "r" (b),
    );
}

fn asmRemwSigned(a: i32, b: i32) i32 {
    return asm volatile ("remw %[ret], %[a], %[b]"
        : [ret] "=r" (-> i32),
        : [a] "r" (a),
          [b] "r" (b),
    );
}

fn touchMemory(buf: []u8) void {
    // Real RAM traffic wider than a single register: write then read back a
    // pattern across several words, exercising read_16/write_16/write_32/ram.
    // SH/LH/LHU emitted via asm (see touchArithmetic's comment on why: LLVM
    // otherwise reaches the same value through SB/SD instead).
    for (buf, 0..) |*byte, i| {
        byte.* = @truncate(i *% 2654435761 +% 1);
    }
    var acc: u64 = 0;
    for (buf) |byte| acc +%= byte;

    // Signed/unsigned byte/halfword loads (LB/LH/LHU -> sgn_extension_u8_u64/u16_u64)
    // and arithmetic shift right at both widths (SRA/SRAW -> _ashr_u64/u32).
    var i: usize = 0;
    while (i + 4 <= buf.len) : (i += 5) {
        asmStoreHalf(&buf[i], @truncate(i *% 40503 +% 7));
        const byte_ptr: *const i8 = @ptrCast(&buf[i]);
        acc +%= @bitCast(@as(i64, byte_ptr.*));
        acc +%= @bitCast(@as(i64, asmLoadHalfSigned(&buf[i])));
        acc +%= asmLoadHalfUnsigned(&buf[i]);
    }
    // SRA/SRAW/SLLI at both widths, via asm and looped with a runtime-varying
    // shift amount: like touchArithmetic's M-extension ops, a couple of these
    // (32-bit arithmetic shift right, the compiler-synthesized small-constant
    // left-shift helpers) got folded to a single execution by the optimizer
    // even across a runtime-seeded multi-iteration loop, so every op here is
    // asm to make the opcode a hard requirement rather than a reachable value.
    var shiftAmount: u6 = @truncate(acc | 1);
    var wide: i64 = @bitCast(acc | 1);
    var narrow: i32 = @truncate(wide);
    for (0..3) |_| {
        wide = asmSra(wide, shiftAmount);
        narrow = asmSraw(narrow, shiftAmount);
        acc +%= @bitCast(wide);
        acc +%= @as(u32, @bitCast(narrow));
        acc +%= asmSll5(@truncate(acc));
        acc +%= asmSll21(@truncate(acc));
        shiftAmount = (shiftAmount % 30) +% 1;
    }
    sink +%= acc;
}

fn asmSra(value: i64, amount: u6) i64 {
    return asm volatile ("sra %[ret], %[value], %[amount]"
        : [ret] "=r" (-> i64),
        : [value] "r" (value),
          [amount] "r" (@as(u64, amount)),
    );
}

fn asmSraw(value: i32, amount: u6) i32 {
    return asm volatile ("sraw %[ret], %[value], %[amount]"
        : [ret] "=r" (-> i32),
        : [value] "r" (value),
          [amount] "r" (@as(u64, amount)),
    );
}

// $bit_shl_u5/$bit_shl_u21: zkc's compiler-synthesized helpers for small-
// immediate left shifts (SLLI with a 5-bit vs. wider immediate window,
// matching main.zkc's own internal shift-decoding constants). Fixed
// immediates so these are genuine SLLI, not the register-shift SLL.
fn asmSll5(value: u32) u32 {
    return asm volatile ("slli %[ret], %[value], 5"
        : [ret] "=r" (-> u32),
        : [value] "r" (value),
    );
}

fn asmSll21(value: u32) u32 {
    return asm volatile ("slli %[ret], %[value], 21"
        : [ret] "=r" (-> u32),
        : [value] "r" (value),
    );
}

fn asmStoreHalf(ptr: *u8, value: u16) void {
    asm volatile ("sh %[value], 0(%[ptr])"
        :
        : [ptr] "r" (ptr),
          [value] "r" (value),
        : .{ .memory = true });
}

fn asmLoadHalfSigned(ptr: *const u8) i16 {
    return asm volatile ("lh %[ret], 0(%[ptr])"
        : [ret] "=r" (-> i16),
        : [ptr] "r" (ptr),
        : .{ .memory = true });
}

fn asmLoadHalfUnsigned(ptr: *const u8) u16 {
    return asm volatile ("lhu %[ret], 0(%[ptr])"
        : [ret] "=r" (-> u16),
        : [ptr] "r" (ptr),
        : .{ .memory = true });
}

// Linux write syscall (a7=64, fd=1/stdout) — the RISC-V syscall convention
// main.zkc's process_syscall module recognizes as WRITE_SYSCALL, distinct
// from EXIT_SYSCALL (a7=93, issued by zkvm_exit below). Mirrors
// zesu-zkvm's linea/src/zkvm_io.zig writeEcall.
fn writeSyscall(s: []const u8) void {
    _ = asm volatile ("ecall"
        : [ret] "={a0}" (-> usize),
        : [fd] "{a0}" (@as(usize, 1)),
          [buf] "{a1}" (@intFromPtr(s.ptr)),
          [count] "{a2}" (s.len),
          [syscall] "{a7}" (@as(usize, 64)),
        : .{ .memory = true });
}

// A direct jal (PC-relative, not through a register): jumps 4 bytes forward,
// over a single nop, to the very next instruction — a no-op control-flow
// edge whose only purpose is to be a genuine JAL distinct from the entry
// stub's own `call main`.
fn asmDirectJump() void {
    asm volatile (
        \\jal x1, 1f
        \\nop
        \\1:
    );
}

fn guestMain() callconv(.c) noreturn {
    // Two differently-sized keccak calls: pushes the absorbing phase across more
    // than one 136-byte Keccak-f[1600] rate block, so kec_absorbing_phase/keccak_f
    // get more than a single call's worth of real rows.
    var digest1: lineth_accel.zkvm_keccak256_hash = undefined;
    const message1 = "verifier-ray main.zkc bootstrap witness";
    _ = lineth_accel.zkvm_keccak256(message1.ptr, message1.len, &digest1);

    var digest2: lineth_accel.zkvm_keccak256_hash = undefined;
    const message2 = "verifier-ray main.zkc bootstrap witness " ** 4;
    _ = lineth_accel.zkvm_keccak256(message2.ptr, message2.len, &digest2);

    // Two poseidon2 permutations (state depends on the keccak digests above, so
    // the optimizer can't precompute this at comptime): exercises
    // full_round/partial_round/sbox/mat_mul_external/mat_mul_internal/mat_mul_m4/
    // add_round_key_full/add_round_key_partial/permutation/poseidon2_state.
    var pstate: [64]u8 align(8) = undefined;
    @memcpy(pstate[0..32], &digest1.data);
    @memcpy(pstate[32..64], &digest2.data);
    var pout1: lineth_accel.zkvm_bytes_64 align(8) = undefined;
    _ = lineth_accel.lineth_zkvm_poseidon2_permutation(@ptrCast(&pstate), &pout1);
    var pout2: lineth_accel.zkvm_bytes_64 align(8) = undefined;
    _ = lineth_accel.lineth_zkvm_poseidon2_permutation(&pout1, &pout2);

    // Function calls (JAL) and a spread of M-extension arithmetic at both
    // widths/signednesses, looped so each opcode's module gets several rows
    // rather than exactly one. Seeded from the poseidon2 output (genuine
    // runtime data the compiler cannot precompute) and called through
    // @call(.never_inline, ...) so ReleaseSmall cannot constant-fold the
    // whole loop into a single already-known result — ordinary comptime-
    // provable inputs get folded away entirely, collapsing every M-extension
    // opcode down to a single executed instance regardless of loop count.
    var a: u64 = std.mem.readInt(u64, pout2.data[0..8], .little);
    var b: u64 = std.mem.readInt(u64, pout2.data[8..16], .little) | 1; // odd, non-zero divisor at every width
    for (0..4) |_| {
        @call(.never_inline, touchArithmetic, .{ a, b });
        a = a ^ (a << 13) ^ (a >> 7);
        b +%= 2;
    }

    // Real RAM traffic: a buffer bigger than a register, exercised via a
    // function call (not inlined into guestMain, so it's a real JAL/RAM
    // sequence, matching what process_J_type_instruction/ram/registers see
    // from ordinary function calls in a larger program).
    var buf: [64]u8 = undefined;
    @call(.never_inline, touchMemory, .{&buf});

    // Extra, non-looped call sites for the two ops that stayed at a single
    // real row even inside touchMemory's 3-iteration loop (_ashr_u32,
    // $bit_shl_u5) — main.zkc may count call-sites/selectors rather than
    // raw dynamic execution count for these, so a second textually distinct
    // call, not just a second dynamic execution of the same one, is tried
    // here.
    _ = asmSraw(@bitCast(@as(u32, @truncate(sink >> 3 | 1))), @truncate(sink | 1));
    _ = asmSll5(@truncate(sink));

    // Two DIRECT jal instructions (Zig's ordinary function-call codegen uses
    // jalr — an indirect call through a computed register — even for
    // statically known targets, so sgn_extension_u21_u64/
    // process_J_type_instruction never see more than the entry stub's own
    // single `call main` jal without this; that one apparently doesn't
    // register as a second real row either, so two explicit ones here).
    asmDirectJump();
    asmDirectJump();

    var output: [64 + 8]u8 = undefined;
    @memcpy(output[0..32], &digest1.data);
    @memcpy(output[32..64], &digest2.data);
    @memcpy(output[64..72], @as([*]const u8, @ptrCast(&sink))[0..8]);
    lineth_accel.write_output(&output, output.len);

    // A second, distinct syscall (Linux write, a7=64) before the exit ecall
    // (a7=93): main.zkc's process_syscall module switches on the syscall
    // number, so a guest that only ever exits gives that module just one
    // real row. This one is debug-only (main.zkc's WRITE_SYSCALL handler
    // just echoes bytes to its own printf trace) and has no effect on the
    // guest's actual output (write_output above already carries that).
    writeSyscall("bootstrap witness\n");

    lineth_accel.zkvm_exit(0);
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&guestMain, .{ .name = "main" });
    }
}
