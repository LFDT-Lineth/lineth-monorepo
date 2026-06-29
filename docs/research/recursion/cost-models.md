# Cost models for zkc

Three separate models measure zkc performance. They chain by multiplication, and only the product is real proof cost.

| Model | Unit | Measures |
| --- | --- | --- |
| **A — cycles** | R5 cycles | instruction count: 1 instruction = 1 cycle, flat (`add` = `remuw` = 1) |
| **B — rows/handler** | AIR rows | rows each instruction's handler emits (`add` → a few; `remuw` → tens) |
| **C — constraints** | degree × width | cost of each row's constraints (degree-2 ≫ degree-1; lookups cost more still) |

```
proof cost = Σ over executed instructions ( Σ over that instruction's constraints ( degree × width ) )
             └─ model A: # terms ─┘          └─ model B: # terms ─┘   └─ model C: value ─┘
```

A benchmark using only A sees `remuw = add = 1`. But model A counts only *instructions*, not the rows or constraints each emits — so a single `remuw` cycle hides a division gadget (quotient/remainder witnesses, a degree-2 product, range-check lookups) that ADD does not have. We have not measured the per-instruction row counts (models B/C), so we cannot state the ratio; structurally, though, **flat cycle counts under-represent divide, remainder, and multiply relative to their proof cost.**

**Cycles ≠ trace length.** Cycle count is the number of *instructions*. The trace is a rows × columns table; its **length (height) is the number of rows = Σ over executed instructions of that instruction's rows (A × B)**. Cycles equal trace length only if every handler emits exactly one row, which is not the case. And neither equals proof cost — that is the full A × B × C product (rows further weighted by each constraint's degree × width).

---

# Which metric should I optimize?

`verifier-ray` is a verifier compiled to R5 to be **proven**, so the metric that matters is **proof cost**. Cycles and interpreter steps are easy to measure but mislead — both count *execution* work, blind to gadget expansion.

- **Primary — proof cost (B × C).** Approximate without a prover run by **instruction mix weighted by handler cost**, ranking by gadget structure (exact magnitudes unmeasured): `mul`/`div`/`rem`/`remuw` (and `% modulus`, which emits REMUW), bitwise, and shifts carry division/product/lookup gadgets and are **heavier**; `add`/`sub`/compares/branches are degree-1 and **lighter**. When implementations differ in instruction *mix*, this — not cycles — decides.
- **Cycles (A) — secondary proxy.** A count of instructions, not trace rows (those are A × B). Useful as a rough proxy for trace length (which bounds FFT/commitment size) or to compare implementations with the **same** instruction mix. Never as the sole metric when the mix changes.
- **Interpreter steps — emulator speed only** (how fast `zkc exec` runs). Never for choosing a field-op implementation.

---

# The one-line takeaway

A flat cycle count is an instruction count — a rough proxy for trace length (rows = A × B), and neither of those is proof cost. The single-cycle `mul`, `div`, `rem`, and their `W` variants carry, by construction, division/limb-product gadgets with degree-2 constraints and range-check lookups that `add`/`sub` do not — so equal cycle counts do not mean equal proof cost. (The row/constraint magnitudes have not been measured.)

---

# Appendix A — measured interpreter steps per opcode

Measured by the arithmetization team. A "step" is one interpreter iteration (`Interpreter.Execute` ticks once per executed bytecode), and each R5 opcode's handler expands into several bytecodes — so this is **interpreter bytecodes executed per R5 instruction**. `_taken` rows are the branch-taken variant.

This is **none of the three models above**. Not model A (which counts whole instructions, flat); not model B (AIR rows). It counts *execution* work, not *proof* work: DIV = 32 ≈ ADD = 31 here, because the number of bytecodes to run a division is close to an add — whereas DIV's *proof* cost (its division gadget) is not something these step counts capture. Use it for **interpreter execution time only**.

| Opcode | Steps | Opcode | Steps | Opcode | Steps |
| --- | --- | --- | --- | --- | --- |
| ADD | 31 | JAL | 44 | SLLI | 45 |
| ADDI | 41 | JALR | 42 | SLLIW | 56 |
| ADDIW | 52 | LB | 60 | SLLW | 63 |
| ADDW | 62 | LBU | 52 | SLT | 44 |
| AND | 29 | LD | 52 | SLTI | 59 |
| ANDI | 41 | LH | 61 | SLTIU | 44 |
| AUIPC | 41 | LHU | 53 | SLTU | 31 |
| BEQ | 35 | LUI | 33 | SRA | 39 |
| BEQ_taken | 35 | LW | 57 | SRAI | 42 |
| BGE | 51 | LWU | 49 | SRAIW | 41 |
| BGE_taken | 51 | MUL | 32 | SRAW | 68 |
| BGEU | 35 | MULH | 64 | SRL | 31 |
| BGEU_taken | 35 | MULHSU | 48 | SRLI | 45 |
| BLT | 44 | MULHU | 31 | SRLIW | 55 |
| BLT_taken | 44 | MULW | 60 | SRLW | 63 |
| BLTU | 28 | OR | 30 | SUB | 33 |
| BLTU_taken | 28 | ORI | 42 | SUBW | 64 |
| BNE | 28 | REM | 32 | SW | 53 |
| BNE_taken | 28 | REMU | 31 | XOR | 30 |
| DIV | 32 | REMUW | 63 | XORI | 42 |
| DIVU | 32 | REMW | 64 | | |
| DIVUW | 56 | SB | 55 | | |
| DIVW | 57 | SD | 54 | | |
| ECALL_write | 74 | SH | 55 | | |
| FENCE | 15 | SLL | 31 | | |
| FENCE.I | 15 | | | | |

- The `W` (32-bit, sign-extended) variants are consistently priciest: ADDW 62 vs ADD 31, SUBW 64 vs SUB 33, REMUW 63 vs REMU 31.
- DIV/REM/MUL look cheap here (~32) — the blind spot: their proof cost lives in the division/limb-product gadgets, not steps.

---

# Appendix B — model B worked source (rows per handler)

**ADD** (`r_type.zkc`): `(v1 as u65) + (v2 as u65)` → store `rd`. Two operations after optimization. Very cheap.

**ADDI** (`i_type.zkc`): sign-extend imm12, add, store — a few more operations than ADD for the sign extension.

**REMUW**: division decomposes into bit ranges, quotient/remainder witnesses, and range checks — structurally more handler work than ADD, though the exact row count has not been measured.

---

# Appendix C — model C worked example (MUL)

Proof cost for one constraint ≈ degree × width × trace_length.

**ADD:** 1 constraint, degree 1, width 3 (rs1, rs2, rd). No auxiliary columns.

**MUL:**
- *Limb decomposition* — `rs1 = a_hi·2^16 + a_lo`, same for `rs2`: 2 degree-1 constraints, 4 aux columns.
- *Cross products* — `a_lo·b_lo`, `a_lo·b_hi`, `a_hi·b_lo`, `a_hi·b_hi`: 4 **degree-2** constraints, 4 aux columns. This is where MUL stops being linear.
- *Carry propagation* — 2 degree-1 constraints.
- *Range checks (the expensive part)* — each 16-bit limb must be proven `< 2^16`. Not expressible as one low-degree constraint, so a **lookup argument** against a 2^16-entry table is used: low-degree but commitment-heavy (per-check accumulator/multiplicity columns + cross-row recurrence). The four range-check lookups — not the multiplication — are what make MUL and REMU costly.

---

# Appendix D — per-instruction proof obligations

Each instruction computes a value and (where relevant) must *prove* it. The `W` suffix denotes a 32-bit operation sign-extended to 64 bits.

**R-type arithmetic**
- **ADD / SUB** — `rd = rs1 ± rs2`. One degree-1 equality. Trivial; overflow wraps.
- **ADDW / SUBW** — 32-bit ±, plus a sign-extension constraint tying bit 31 into the upper bits.
- **SLL / SRL / SRA** — shift by `rs2 mod 64`. A shift is mul/div by a power of two: decompose into bits/limbs and reassemble, with range checks. SRA also preserves the sign bit. SLLW/SRLW/SRAW are the 32-bit variants.
- **SLT / SLTU** — `rd = (rs1 < rs2)`. Comparison via subtraction + borrow/sign-bit inspection; result constrained to {0,1}.
- **AND / OR / XOR** — not native to field arithmetic: bit-decompose both operands, constrain the per-bit truth table, reassemble. Bit-decomposition + range checks, costlier than ADD.

**I-type arithmetic**
- **ADDI / ADDIW** — sign-extend imm12, then a degree-1 add.
- **SLTI / SLTIU** — set-less-than against an immediate.
- **ANDI / ORI / XORI** — same bit-decomposition cost as the register forms.
- **SLLI / SRLI / SRAI** — shift by a constant; same shift gadget.

**M-extension (the expensive family)**
- **MUL** — low 64 bits. Limb decomposition, four degree-2 cross products, carry propagation, per-limb range checks (Appendix C). Moderate-to-expensive.
- **MULH / MULHU / MULHSU** — high 64 bits; same machinery plus a constrained carry-out, at least as expensive. **MULW** — low 32 bits, sign-extended.
- **DIV / DIVU** — prove `a = q·b + r, 0 ≤ r < b` with `q`,`r` as witnesses. The `q·b` is a degree-2 multiplication; `0 ≤ r < b` is a range/comparison gadget; div-by-zero/overflow need extra constraints. The most expensive arithmetic op.
- **REM / REMU** — identical gadget to DIV (must produce both `q` and `r`), output `r`. Essentially the same cost as DIVU.
- **DIVW / DIVUW / REMW / REMUW** — 32-bit variants, sign-extended. **REMUW is what `% modulus` emits in field add/sub/double** — the full division gadget, narrowed to 32 bits.

**Loads / stores**
- **LB/LH/LW/LD** (sign-extended), **LBU/LHU/LWU** (zero-extended) — prove the loaded value matches memory at the address (memory-consistency argument), plus correct extension and a width range check.
- **SB/SH/SW/SD** — prove the value is written consistently; narrow stores constrain that only the intended bytes change.

**Control flow**
- **BEQ/BNE/BLT/BGE/BLTU/BGEU** — comparison (subtraction + flag) and that the PC took the correct next value.
- **JAL / JALR** — `rd = PC + 4`; the link value is old-PC + width, new PC is the computed target.

**Upper-immediate / system**
- **LUI** — `rd = imm << 12`. One placement constraint. Cheap.
- **AUIPC** — `rd = PC + (imm << 12)`. One addition against the PC. Cheap.
- **ECALL / EBREAK** — system traps; mark I/O or halt boundaries, constrained by surrounding execution-control logic.
- **FENCE** — no-op for a single-threaded trace; advances the PC without changing state.
