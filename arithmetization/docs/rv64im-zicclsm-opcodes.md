# RV64IM + Zicclsm Opcode Reference

## Extensions covered


| Extension | Description                                                                                       |
| --------- | ------------------------------------------------------------------------------------------------- |
| RV64I     | Base 64-bit integer instruction set (includes all RV32I instructions plus *W word-width variants) |
| M         | Integer multiplication and division                                                               |
| Zicsr     | Control and Status Register (CSR) instructions (part of the standard unprivileged ISA)            |
| Zifencei  | Instruction-fetch fence (part of the standard unprivileged ISA)                                   |
| Zicclsm   | Misaligned loads and stores are supported — **no new opcodes**, behavioural guarantee only        |


---

## Instruction format encoding

```
R-type:  [ funct7 | rs2 | rs1 | funct3 | rd  | opcode ]
          31    25  24 20  19 15  14  12  11 7   6    0

I-type:  [      imm[11:0]      | rs1 | funct3 | rd  | opcode ]
          31             20    19 15   14  12  11 7   6    0

S-type:  [ imm[11:5] | rs2 | rs1 | funct3 | imm[4:0] | opcode ]
          31       25  24 20  19 15  14  12  11      7   6    0

B-type:  [ imm[12|10:5] | rs2 | rs1 | funct3 | imm[4:1|11] | opcode ]
          31          25  24 20  19 15  14  12  11          7   6    0

U-type:  [          imm[31:12]          | rd  | opcode ]
          31                          12  11 7   6    0

J-type:  [ imm[20|10:1|11|19:12] | rd  | opcode ]
          31                   12  11 7   6    0
```

---

## Opcode groups (7-bit `opcode` field)


| Opcode group | Binary  | Hex  | Type | Instructions                                                                                     |
| ------------ | ------- | ---- | ---- | ------------------------------------------------------------------------------------------------ |
| LOAD         | 0000011 | 0x03 | I    | LB, LH, LW, LD, LBU, LHU, LWU                                                                    |
| MISC-MEM     | 0001111 | 0x0F | I    | FENCE, FENCE.I                                                                                   |
| OP-IMM       | 0010011 | 0x13 | I    | ADDI, SLTI, SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI                                             |
| AUIPC        | 0010111 | 0x17 | U    | AUIPC                                                                                            |
| OP-IMM-32    | 0011011 | 0x1B | I    | ADDIW, SLLIW, SRLIW, SRAIW                                                                       |
| STORE        | 0100011 | 0x23 | S    | SB, SH, SW, SD                                                                                   |
| OP           | 0110011 | 0x33 | R    | ADD, SUB, SLL, SLT, SLTU, XOR, SRL, SRA, OR, AND, MUL, MULH, MULHSU, MULHU, DIV, DIVU, REM, REMU |
| LUI          | 0110111 | 0x37 | U    | LUI                                                                                              |
| OP-32        | 0111011 | 0x3B | R    | ADDW, SUBW, SLLW, SRLW, SRAW, MULW, DIVW, DIVUW, REMW, REMUW                                     |
| BRANCH       | 1100011 | 0x63 | B    | BEQ, BNE, BLT, BGE, BLTU, BGEU                                                                   |
| JALR         | 1100111 | 0x67 | I    | JALR                                                                                             |
| JAL          | 1101111 | 0x6F | J    | JAL                                                                                              |
| SYSTEM       | 1110011 | 0x73 | I    | ECALL, EBREAK, CSRRW, CSRRS, CSRRC, CSRRWI, CSRRSI, CSRRCI                                       |


---

## Full opcode table

Columns:

- **funct7**: 7-bit qualifier in bits [31:25]; `—` when unused.
- **funct3**: 3-bit qualifier in bits [14:12]; `—` when unused.
- For immediate-shift instructions (SLLI, SRLI, SRAI, etc.) the top bits of the immediate field act as a qualifier; this is shown in the funct7 column as `imm[11:5]` or `imm[11:6]`.

### RV64I — Base integer instructions

#### OP (0110011) — R-type register-register arithmetic --- @Olivier


| Mnemonic | Type | Opcode  | funct3 | funct7  | Operation                         | Extension | Status |
| -------- | ---- | ------- | ------ | ------- | --------------------------------- | --------- | ------ |
| ADD      | R    | 0110011 | 000    | 0000000 | rd = rs1 + rs2                    | RV32I     | ✓      |
| SUB      | R    | 0110011 | 000    | 0100000 | rd = rs1 − rs2                    | RV32I     | ✓      |
| SLL      | R    | 0110011 | 001    | 0000000 | rd = rs1 << rs2[5:0]              | RV32I     | ✓      |
| SLT      | R    | 0110011 | 010    | 0000000 | rd = (rs1 < rs2) ? 1:0 (signed)   | RV32I     | ✓      |
| SLTU     | R    | 0110011 | 011    | 0000000 | rd = (rs1 < rs2) ? 1:0 (unsigned) | RV32I     | ✓      |
| XOR      | R    | 0110011 | 100    | 0000000 | rd = rs1 ^ rs2                    | RV32I     | ✓      |
| SRL      | R    | 0110011 | 101    | 0000000 | rd = rs1 >> rs2[5:0] (logical)    | RV32I     | ✓      |
| SRA      | R    | 0110011 | 101    | 0100000 | rd = rs1 >> rs2[5:0] (arithmetic) | RV32I     | ✓      |
| OR       | R    | 0110011 | 110    | 0000000 | rd = rs1 | rs2                    | RV32I     | ✓      |
| AND      | R    | 0110011 | 111    | 0000000 | rd = rs1 & rs2                    | RV32I     | ✓      |


#### OP-32 (0111011) — R-type 32-bit register-register arithmetic (RV64I) --- @Olivier


| Mnemonic | Type | Opcode  | funct3 | funct7  | Operation                                     | Extension |
| -------- | ---- | ------- | ------ | ------- | --------------------------------------------- | --------- |
| ADDW     | R    | 0111011 | 000    | 0000000 | rd = sext(rs1[31:0] + rs2[31:0])              | RV64I     |
| SUBW     | R    | 0111011 | 000    | 0100000 | rd = sext(rs1[31:0] − rs2[31:0])              | RV64I     |
| SLLW     | R    | 0111011 | 001    | 0000000 | rd = sext(rs1[31:0] << rs2[4:0])              | RV64I     |
| SRLW     | R    | 0111011 | 101    | 0000000 | rd = sext(rs1[31:0] >> rs2[4:0]) (logical)    | RV64I     |
| SRAW     | R    | 0111011 | 101    | 0100000 | rd = sext(rs1[31:0] >> rs2[4:0]) (arithmetic) | RV64I     |


#### OP-IMM (0010011) — I-type immediate arithmetic @amkcha


| Mnemonic | Type | Opcode  | funct3 | funct7 / imm qualifier | Operation                                 | Extension | Status |
| -------- | ---- | ------- | ------ | ---------------------- | ----------------------------------------- | --------- | ------ |
| ADDI     | I    | 0010011 | 000    | —                      | rd = rs1 + sext(imm12)                    | RV32I     | ✓      |
| SLTI     | I    | 0010011 | 010    | —                      | rd = (rs1 < sext(imm12)) ? 1:0 (signed)   | RV32I     | ✓      |
| SLTIU    | I    | 0010011 | 011    | —                      | rd = (rs1 < sext(imm12)) ? 1:0 (unsigned) | RV32I     | ✓      |
| XORI     | I    | 0010011 | 100    | —                      | rd = rs1 ^ sext(imm12)                    | RV32I     | ✓      |
| ORI      | I    | 0010011 | 110    | —                      | rd = rs1 | sext(imm12)                    | RV32I     | ✓      |
| ANDI     | I    | 0010011 | 111    | —                      | rd = rs1 & sext(imm12)                    | RV32I     | ✓      |
| SLLI     | I    | 0010011 | 001    | imm[11:6]=000000       | rd = rs1 << imm[5:0]                      | RV64I     | ✓      |
| SRLI     | I    | 0010011 | 101    | imm[11:6]=000000       | rd = rs1 >> imm[5:0] (logical)            | RV64I     | ✓      |
| SRAI     | I    | 0010011 | 101    | imm[11:6]=010000       | rd = rs1 >> imm[5:0] (arithmetic)         | RV64I     | ✓      |


> Note: In RV32I SLLI/SRLI/SRAI use only imm[4:0] as the shift amount and imm[11:5] as the qualifier.
> In RV64I the shift amount is imm[5:0] and imm[11:6] is the qualifier.

#### OP-IMM-32 (0011011) — I-type 32-bit immediate arithmetic (RV64I) @amkcha


| Mnemonic | Type | Opcode  | funct3 | funct7 / imm qualifier | Operation                                     | Extension | Status |
| -------- | ---- | ------- | ------ | ---------------------- | --------------------------------------------- | --------- | ------ |
| ADDIW    | I    | 0011011 | 000    | —                      | rd = sext(rs1[31:0] + sext(imm12))            | RV64I     | ✓      |
| SLLIW    | I    | 0011011 | 001    | imm[11:5]=0000000      | rd = sext(rs1[31:0] << imm[4:0])              | RV64I     | ✓      |
| SRLIW    | I    | 0011011 | 101    | imm[11:5]=0000000      | rd = sext(rs1[31:0] >> imm[4:0]) (logical)    | RV64I     | ✓      |
| SRAIW    | I    | 0011011 | 101    | imm[11:5]=0100000      | rd = sext(rs1[31:0] >> imm[4:0]) (arithmetic) | RV64I     | ✓      |


#### LOAD (0000011) — I-type loads @amkcha


| Mnemonic | Type | Opcode  | funct3 | funct7 | Operation                           | Extension | Status |
| -------- | ---- | ------- | ------ | ------ | ----------------------------------- | --------- | ------ |
| LB       | I    | 0000011 | 000    | —      | rd = sext(M[rs1+sext(imm12)][7:0])  | RV32I     | ✓      |
| LH       | I    | 0000011 | 001    | —      | rd = sext(M[rs1+sext(imm12)][15:0]) | RV32I     | ✓      |
| LW       | I    | 0000011 | 010    | —      | rd = sext(M[rs1+sext(imm12)][31:0]) | RV32I     | ✓      |
| LD       | I    | 0000011 | 011    | —      | rd = M[rs1+sext(imm12)][63:0]       | RV64I     | ✓      |
| LBU      | I    | 0000011 | 100    | —      | rd = zext(M[rs1+sext(imm12)][7:0])  | RV32I     | ✓      |
| LHU      | I    | 0000011 | 101    | —      | rd = zext(M[rs1+sext(imm12)][15:0]) | RV32I     | ✓      |
| LWU      | I    | 0000011 | 110    | —      | rd = zext(M[rs1+sext(imm12)][31:0]) | RV64I     | ✓      |


#### STORE (0100011) — S-type stores --- implementer @Lorenzo


| Mnemonic | Type | Opcode  | funct3 | funct7 | Operation                            | Extension | Status |
| -------- | ---- | ------- | ------ | ------ | ------------------------------------ | --------- | ------ |
| SB       | S    | 0100011 | 000    | —      | M[rs1+sext(imm12)][7:0] = rs2[7:0]   | RV32I     | ✓      |
| SH       | S    | 0100011 | 001    | —      | M[rs1+sext(imm12)][15:0] = rs2[15:0] | RV32I     | ✓      |
| SW       | S    | 0100011 | 010    | —      | M[rs1+sext(imm12)][31:0] = rs2[31:0] | RV32I     | ✓      |
| SD       | S    | 0100011 | 011    | —      | M[rs1+sext(imm12)][63:0] = rs2[63:0] | RV64I     | ✓      |


#### BRANCH (1100011) — B-type conditional branches @Olivier


| Mnemonic | Type | Opcode  | funct3 | funct7 | Operation                                   | Extension | Status |
| -------- | ---- | ------- | ------ | ------ | ------------------------------------------- | --------- | ------ |
| BEQ      | B    | 1100011 | 000    | —      | if rs1 == rs2: PC += sext(imm13)            | RV32I     | ✓      |
| BNE      | B    | 1100011 | 001    | —      | if rs1 != rs2: PC += sext(imm13)            | RV32I     | ✓      |
| BLT      | B    | 1100011 | 100    | —      | if rs1 < rs2 (signed): PC += sext(imm13)    | RV32I     | ✓      |
| BGE      | B    | 1100011 | 101    | —      | if rs1 >= rs2 (signed): PC += sext(imm13)   | RV32I     | ✓      |
| BLTU     | B    | 1100011 | 110    | —      | if rs1 < rs2 (unsigned): PC += sext(imm13)  | RV32I     | ✓      |
| BGEU     | B    | 1100011 | 111    | —      | if rs1 >= rs2 (unsigned): PC += sext(imm13) | RV32I     | ✓      |


#### Upper-immediate and jump ---- implementer @letypequividelespoubelles


| Mnemonic | Type | Opcode  | funct3 | funct7 | Operation                              | Extension | Status |
| -------- | ---- | ------- | ------ | ------ | -------------------------------------- | --------- | ------ |
| LUI      | U    | 0110111 | —      | —      | rd = imm20 << 12 (zero lower 12 bits)  | RV32I     | ✓      |
| AUIPC    | U    | 0010111 | —      | —      | rd = PC + (imm20 << 12)                | RV32I     | ✓      |
| JAL      | J    | 1101111 | —      | —      | rd = PC+4; PC += sext(imm21)           | RV32I     | ✓      |
| JALR     | I    | 1100111 | 000    | —      | rd = PC+4; PC = (rs1+sext(imm12)) & ~1 | RV32I     | ✓      |


#### SYSTEM (1110011)


| Mnemonic   | Type   | Opcode    | funct3   | funct7 / imm qualifier   | Operation                                     | Extension   | Status   |
| ---------- | ------ | --------- | -------- | ------------------------ | --------------------------------------------- | ----------- | -------- |
| ECALL      | I      | 1110011   | 000      | imm12=000000000000       | Environment call (trap to higher privilege)   | RV32I       | ¬        |
| EBREAK     | I      | 1110011   | 000      | imm12=000000000001       | Environment break (debugger trap)             | RV32I       | ¬        |
| ---------- | ------ | --------- | -------- | ------------------------ | --------------------------------------------- | ----------- | -------- |


---

### M extension — Integer multiplication and division

All M-extension instructions are R-type with **funct7 = 0000001** and share the `OP` or `OP-32` opcode.

#### OP (0110011) + funct7=0000001 --- implementer @Lorenzo


| Mnemonic | Type | Opcode  | funct3 | funct7  | Operation                                             | Extension | Status |
| -------- | ---- | ------- | ------ | ------- | ----------------------------------------------------- | --------- | ------ |
| MUL      | R    | 0110011 | 000    | 0000001 | rd = (rs1 × rs2)[63:0] (lower 64 bits)                | M         | ✓      |
| MULH     | R    | 0110011 | 001    | 0000001 | rd = (rs1 × rs2)[127:64] (signed × signed, upper)     | M         | ✓      |
| MULHSU   | R    | 0110011 | 010    | 0000001 | rd = (rs1 × rs2)[127:64] (signed × unsigned, upper)   | M         | ✓      |
| MULHU    | R    | 0110011 | 011    | 0000001 | rd = (rs1 × rs2)[127:64] (unsigned × unsigned, upper) | M         | ✓      |
| DIV      | R    | 0110011 | 100    | 0000001 | rd = rs1 / rs2 (signed, truncate toward zero)         | M         | ✓      |
| DIVU     | R    | 0110011 | 101    | 0000001 | rd = rs1 / rs2 (unsigned)                             | M         | ✓      |
| REM      | R    | 0110011 | 110    | 0000001 | rd = rs1 % rs2 (signed remainder)                     | M         | ✓      |
| REMU     | R    | 0110011 | 111    | 0000001 | rd = rs1 % rs2 (unsigned remainder)                   | M         | ✓      |


#### OP-32 (0111011) + funct7=0000001 --- @Olivier


| Mnemonic | Type | Opcode  | funct3 | funct7  | Operation                                             | Extension | Status |
| -------- | ---- | ------- | ------ | ------- | ----------------------------------------------------- | --------- | ------ |
| MULW     | R    | 0111011 | 000    | 0000001 | rd = sext((rs1[31:0] × rs2[31:0])[31:0])              | M (RV64)  | ✓      |
| DIVW     | R    | 0111011 | 100    | 0000001 | rd = sext(rs1[31:0] / rs2[31:0]) (signed)             | M (RV64)  | ✓      |
| DIVUW    | R    | 0111011 | 101    | 0000001 | rd = sext(rs1[31:0] / rs2[31:0]) (unsigned)           | M (RV64)  | ✓      |
| REMW     | R    | 0111011 | 110    | 0000001 | rd = sext(rs1[31:0] % rs2[31:0]) (signed remainder)   | M (RV64)  | ✓      |
| REMUW    | R    | 0111011 | 111    | 0000001 | rd = sext(rs1[31:0] % rs2[31:0]) (unsigned remainder) | M (RV64)  | ✓      |


---

### Zicclsm — Misaligned loads and stores

No new opcodes. This extension is a profile feature indicating that the hart supports **naturally-misaligned** memory accesses for all LOAD and STORE instructions listed above (LB through LD, SB through SD). Accesses that cross a naturally-aligned power-of-2 boundary complete atomically or in multiple transactions transparently to software, without raising a misaligned-address exception.

---

## Summary counts


| Extension   | Instruction count                                                              |
| ----------- | ------------------------------------------------------------------------------ |
| RV64I       | 47 (includes all RV32I instructions + *W word variants + wider loads/stores)   |
| M           | 13 (8 × 64-bit + 5 × 32-bit *W variants)                                       |
| Zicclsm     | 0 (behavioural)                                                                |
| ----------- | ------------------------------------------------------------------------------ |
| Zicsr       | 6                                                                              |
| Zifencei    | 1                                                                              |
| ----------- | ------------------------------------------------------------------------------ |
| **Total**   | **67**                                                                         |


