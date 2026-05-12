#!/usr/bin/env python3
"""Generate atomic single-opcode RV64IM tests for the zkc test harness.

Each test follows the same pattern:
    1. set up two operands in a0 / a1
    2. run the instruction under test, putting result in a2
    3. exit with code (a2 - expected) — exit-0 means PASS

Output: ../src/op_<opcode>.s (one file per opcode).

Re-running this script overwrites existing op_*.s files.
"""

import os
import textwrap

# Resolve ../src/ relative to this script — no absolute paths.
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(SCRIPT_DIR, "..", "src/op"))


def emit(name, body):
    path = os.path.join(OUT, f"op_{name}.s")
    body = textwrap.dedent(body).lstrip()
    if not body.endswith("\n"):
        body += "\n"
    with open(path, "w") as f:
        f.write(body)


PROLOGUE = """\
.section .text
.global _start
_start:
    la sp, _stack_start
"""

EPILOGUE = """\
    li a7, 93
    ecall
"""


def rr_test(op, val1, val2, expected):
    """Register-register ALU: op a2, a0, a1."""
    return PROLOGUE + f"""\
    li a0, {hex(val1)}
    li a1, {hex(val2)}
    {op} a2, a0, a1
    li a3, {hex(expected)}
    sub a0, a2, a3
""" + EPILOGUE


def imm_test(op, val, imm, expected):
    """Register-immediate ALU: op a2, a0, imm. Imm fits in 12-bit signed."""
    return PROLOGUE + f"""\
    li a0, {hex(val)}
    {op} a2, a0, {imm}
    li a3, {hex(expected)}
    sub a0, a2, a3
""" + EPILOGUE


def shamt_test(op, val, shamt, expected):
    """Shift-by-immediate (slli/srli/srai/...): op a2, a0, shamt."""
    return PROLOGUE + f"""\
    li a0, {hex(val)}
    {op} a2, a0, {shamt}
    li a3, {hex(expected)}
    sub a0, a2, a3
""" + EPILOGUE


U64_NEG = lambda v: v & 0xFFFFFFFFFFFFFFFF
U32_NEG = lambda v: v & 0xFFFFFFFF


def branch_taken_test(op, val1, val2):
    return PROLOGUE + f"""\
    li a0, {hex(val1)}
    li a1, {hex(val2)}
    {op} a0, a1, taken
    li a0, 1
    j done
taken:
    li a0, 0
done:
""" + EPILOGUE


def branch_nottaken_test(op, val1, val2):
    return PROLOGUE + f"""\
    li a0, {hex(val1)}
    li a1, {hex(val2)}
    {op} a0, a1, taken
    li a0, 0
    j done
taken:
    li a0, 1
done:
""" + EPILOGUE


# ─────────────────────────────────────────────────────────────────────────────
# RV64I R-type
# ─────────────────────────────────────────────────────────────────────────────

emit("add",     rr_test("add",   5, 7, 12))
emit("sub",     rr_test("sub",   12, 7, 5))
emit("sll",     rr_test("sll",   1, 4, 16))
emit("slt_neg", rr_test("slt", U64_NEG(-1), 0, 1))
emit("slt_pos", rr_test("slt", 0, 1, 1))
emit("sltu",    rr_test("sltu", U64_NEG(-1), 0, 0))
emit("xor",     rr_test("xor", 0xff, 0x0f, 0xf0))
emit("srl",     rr_test("srl", 16, 4, 1))
emit("sra",     rr_test("sra", U64_NEG(-16), 1, U64_NEG(-8)))
emit("or",      rr_test("or",  0xf0, 0x0f, 0xff))
emit("and",     rr_test("and", 0xff, 0x0f, 0x0f))

# ─────────────────────────────────────────────────────────────────────────────
# RV64I R-type W
# ─────────────────────────────────────────────────────────────────────────────

emit("addw",    rr_test("addw", 5, 7, 12))
emit("subw",    rr_test("subw", 12, 7, 5))
emit("sllw",    rr_test("sllw", 1, 4, 16))
emit("srlw",    rr_test("srlw", 16, 4, 1))
emit("sraw",    rr_test("sraw", U32_NEG(-16), 1, U64_NEG(-8)))

# ─────────────────────────────────────────────────────────────────────────────
# RV64I I-type
# ─────────────────────────────────────────────────────────────────────────────

emit("addi",    imm_test("addi",  5, 7, 12))
emit("slti",    imm_test("slti",  U64_NEG(-1), 0, 1))
emit("sltiu",   imm_test("sltiu", U64_NEG(-1), 0, 0))
emit("xori",    imm_test("xori",  0xff, 0x0f, 0xf0))
emit("ori",     imm_test("ori",   0xf0, 0x0f, 0xff))
emit("andi",    imm_test("andi",  0xff, 0x0f, 0x0f))
emit("slli",    shamt_test("slli", 1, 4, 16))
emit("srli",    shamt_test("srli", 16, 4, 1))
emit("srai",    shamt_test("srai", U64_NEG(-16), 1, U64_NEG(-8)))

# ─────────────────────────────────────────────────────────────────────────────
# RV64I I-type W
# ─────────────────────────────────────────────────────────────────────────────

emit("addiw",   imm_test("addiw", 5, 7, 12))
emit("slliw",   shamt_test("slliw", 1, 4, 16))
emit("srliw",   shamt_test("srliw", 16, 4, 1))
emit("sraiw",   shamt_test("sraiw", U32_NEG(-16), 1, U64_NEG(-8)))

# ─────────────────────────────────────────────────────────────────────────────
# RV64M
# ─────────────────────────────────────────────────────────────────────────────

emit("mul",     rr_test("mul",    7, 6, 42))
emit("mulh",    rr_test("mulh",   7, 6, 0))
emit("mulhsu",  rr_test("mulhsu", 7, 6, 0))
emit("mulhu",   rr_test("mulhu",  7, 6, 0))
emit("div",     rr_test("div",    42, 6, 7))
emit("divu",    rr_test("divu",   42, 6, 7))
emit("rem",     rr_test("rem",    43, 6, 1))
emit("remu",    rr_test("remu",   43, 6, 1))

emit("mulw",    rr_test("mulw",  7, 6, 42))
emit("divw",    rr_test("divw",  42, 6, 7))
emit("divuw",   rr_test("divuw", 42, 6, 7))
emit("remw",    rr_test("remw",  43, 6, 1))
emit("remuw",   rr_test("remuw", 43, 6, 1))

# ─────────────────────────────────────────────────────────────────────────────
# RV64I B-type — taken / not-taken pairs
# ─────────────────────────────────────────────────────────────────────────────

emit("beq_taken",     branch_taken_test   ("beq",  5, 5))
emit("beq_nottaken",  branch_nottaken_test("beq",  5, 7))
emit("bne_taken",     branch_taken_test   ("bne",  5, 7))
emit("bne_nottaken",  branch_nottaken_test("bne",  5, 5))
emit("blt_taken",     branch_taken_test   ("blt",  U64_NEG(-1), 0))
emit("blt_nottaken",  branch_nottaken_test("blt",  0, U64_NEG(-1)))
emit("bge_taken",     branch_taken_test   ("bge",  0, U64_NEG(-1)))
emit("bge_nottaken",  branch_nottaken_test("bge",  U64_NEG(-1), 0))
emit("bltu_taken",    branch_taken_test   ("bltu", 0, 1))
emit("bltu_nottaken", branch_nottaken_test("bltu", U64_NEG(-1), 0))
emit("bgeu_taken",    branch_taken_test   ("bgeu", U64_NEG(-1), 0))
emit("bgeu_nottaken", branch_nottaken_test("bgeu", 0, 1))

# ─────────────────────────────────────────────────────────────────────────────
# RV64I U-type
# ─────────────────────────────────────────────────────────────────────────────

emit("lui", PROLOGUE + """\
    lui a2, 1
    li a3, 0x1000
    sub a0, a2, a3
""" + EPILOGUE)

emit("auipc", PROLOGUE + """\
    auipc a0, 0
    auipc a1, 0
    sub a2, a1, a0
    addi a0, a2, -4
""" + EPILOGUE)

# ─────────────────────────────────────────────────────────────────────────────
# RV64I J-type
# ─────────────────────────────────────────────────────────────────────────────

emit("jal", PROLOGUE + """\
    jal x0, target
    li a0, 1
    j done
target:
    li a0, 0
done:
""" + EPILOGUE)

emit("jalr", PROLOGUE + """\
    la t0, target
    jalr x0, 0(t0)
    li a0, 1
    j done
target:
    li a0, 0
done:
""" + EPILOGUE)

# ─────────────────────────────────────────────────────────────────────────────
# RV64I Loads — read known bytes from .rodata
# ─────────────────────────────────────────────────────────────────────────────

LOAD_PROLOGUE = """\
.section .rodata
.balign 8
data:
    .byte 0x42
    .byte 0x55
    .byte 0x66
    .byte 0x77
    .byte 0x88
    .byte 0x99
    .byte 0xaa
    .byte 0xbb

.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, data
"""

emit("lb",  LOAD_PROLOGUE + """\
    lb a2, 0(t0)
    li a3, 0x42
    sub a0, a2, a3
""" + EPILOGUE)

emit("lbu", LOAD_PROLOGUE + """\
    lbu a2, 4(t0)
    li a3, 0x88
    sub a0, a2, a3
""" + EPILOGUE)

emit("lh",  LOAD_PROLOGUE + """\
    lh a2, 0(t0)
    li a3, 0x5542
    sub a0, a2, a3
""" + EPILOGUE)

emit("lhu", LOAD_PROLOGUE + """\
    lhu a2, 4(t0)
    li a3, 0x9988
    sub a0, a2, a3
""" + EPILOGUE)

emit("lw",  LOAD_PROLOGUE + """\
    lw a2, 0(t0)
    li a3, 0x77665542
    sub a0, a2, a3
""" + EPILOGUE)

emit("lwu", LOAD_PROLOGUE + """\
    lwu a2, 0(t0)
    li a3, 0x77665542
    sub a0, a2, a3
""" + EPILOGUE)

emit("ld",  LOAD_PROLOGUE + """\
    ld a2, 0(t0)
    li a3, 0xbbaa998877665542
    sub a0, a2, a3
""" + EPILOGUE)

# ─────────────────────────────────────────────────────────────────────────────
# RV64I Stores — store, then load back to verify
# ─────────────────────────────────────────────────────────────────────────────

STORE_PROLOGUE = """\
.section .data
.balign 8
buf:
    .dword 0

.section .text
.global _start
_start:
    la sp, _stack_start
    la t0, buf
"""

emit("sb", STORE_PROLOGUE + """\
    li t1, 0xab
    sb t1, 0(t0)
    lbu a2, 0(t0)
    li a3, 0xab
    sub a0, a2, a3
""" + EPILOGUE)

emit("sh", STORE_PROLOGUE + """\
    li t1, 0xabcd
    sh t1, 0(t0)
    lhu a2, 0(t0)
    li a3, 0xabcd
    sub a0, a2, a3
""" + EPILOGUE)

emit("sw", STORE_PROLOGUE + """\
    li t1, 0x12345678
    sw t1, 0(t0)
    lwu a2, 0(t0)
    li a3, 0x12345678
    sub a0, a2, a3
""" + EPILOGUE)

emit("sd", STORE_PROLOGUE + """\
    li t1, 0x1122334455667788
    sd t1, 0(t0)
    ld a2, 0(t0)
    li a3, 0x1122334455667788
    sub a0, a2, a3
""" + EPILOGUE)

# ─────────────────────────────────────────────────────────────────────────────

files = sorted(f for f in os.listdir(OUT) if f.startswith("op_") and f.endswith(".s"))
print(f"Generated {len(files)} test files in {OUT}")
