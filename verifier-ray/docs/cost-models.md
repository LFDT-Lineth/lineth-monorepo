# The three cost models

They are three separate models that measuring zkc performance.

### A — the cycle model: *how many instructions*
- **Unit:** R5 cycles.
- **Rule:** 1 instruction = 1 cycle. Flat. `add`, `addi`, and `remuw` all cost 1.
- This is execution-trace length measured at the instruction level. It is what a naive benchmark reports as "cycles/op."

### B — rows per handler: *how many trace rows each instruction produces*
- **Unit:** trace rows (zkc AIR rows).
- Each instruction's `.zkc` handler is compiled through a multi-stage pipeline — ZKC source → IR → AIR (see [ZKC_ARCHITECTURE.md](../../zkc/docs/ZKC_ARCHITECTURE.md)) — and expands into some number of AIR rows. ADD compiles to a handful of rows; REMUW's division gadget compiles to tens of rows.
- **Instructions are not equal in rows even though they are equal in cycles.** This is the layer A is blind to.

### C — the constraint model: *how much each constraint costs*
- **Unit:** degree × width.
- Each constraint has two cost properties: **degree** (highest polynomial degree) and **width** (how many columns it touches). A degree-2 constraint costs quadratically more than degree-1; a range-check lookup costs more still.

### They chain by multiplication

```
proof cost = Σ over executed instructions ( Σ over that instruction's constraints ( degree × width ) )
```

- **A** sets how many terms are in the outer sum.
- **B** sets how many terms are in the inner sum (per instruction).
- **C** sets the value of each inner term.

A benchmark using only A sees `remuw = add = 1`. The full A×B×C cost sees `remuw ≫≫ add`, because its single cycle hides tens of rows, several carrying range-check lookups. The flat cycle count systematically under-prices exactly the instructions — divide, remainder, multiply — that dominate real proof cost.

---

# Cost unit of model B (rows per handler), from the source

**ADD** (`r_type.zkc`, lines 88–92):

```
overflow::double_word_working_copy = (v1 as u65) + (v2 as u65)
registers[rd] = double_word_working_copy
```

Two operations after optimization. Very cheap.

**ADDI** (`i_type.zkc`, lines 45–47):

```
var signext_imm12_64 = sgn_extension_u12_u64(imm12)         // sign extend the immediate
var rs1_plus_signext_imm12 = signed_sum_of_double_word(...) // add
registers[rd] = rs1_plus_signext_imm12                      // store
```

A few more operations because it must sign-extend the immediate first.

**REMUW** (unsigned 32-bit remainder): division requires a multi-step decomposition into bit ranges, quotient/remainder witnesses, and range checks — likely tens of trace rows even after optimization. Far more expensive in rows than ADD or ADDI, even though a cycle benchmark counts them all as 1.

---

# Cost unit of model C (constraints), worked examples

**Proof cost for a single constraint ≈ degree × width × trace_length.**

### ADD

```
rd = rs1 + rs2
```
- 1 constraint, degree 1 (linear, no products), width 3 (rs1, rs2, rd).
- No auxiliary columns.
- **Total: 1 degree-1 constraint, 3 columns.**

### MUL — properly

**Decomposition into 16-bit limbs**
```
rs1 = a_hi × 2^16 + a_lo
rs2 = b_hi × 2^16 + b_lo
```
2 constraints, degree 1 each. Introduces 4 auxiliary columns (a_hi, a_lo, b_hi, b_lo). Cheap on their own — they exist to enable the range checks below.

**Cross products**
```
p0 = a_lo × b_lo        (bits  0–31)
p1 = a_lo × b_hi        (bits 16–47)
p2 = a_hi × b_lo        (bits 16–47)
p3 = a_hi × b_hi        (bits 32–63)
```
4 constraints, **degree 2 each** (product of two columns). 4 auxiliary columns. This is where MUL stops being linear: degree-2 constraints cost quadratically more than degree-1.

**Carry propagation**
```
result_lo = p0 + (p1 + p2) × 2^16   (mod 2^32)
carry     = p3 + ((p1 + p2) >> 16)
```
2 constraints, degree 1 (linear over the partial products). 2 columns.

**Range checks — the most expensive part**

Each limb must be proven to fit in 16 bits: `0 ≤ a_lo < 2^16`, and similarly for the others. A range check cannot be written as a single low-degree constraint (a vanishing polynomial pinning a value to 65,536 valid values would be degree 65,536). The standard approach is a **lookup argument**: prove every queried limb value appears in a fixed, precomputed 16-bit table.

The lookup is low-degree but commitment-heavy. It imports a 2^16-entry table (a one-time fixed cost, shared across all range checks in the circuit) and adds per-check accumulator/multiplicity columns plus a cross-row recurrence. This is what makes MUL and REMU costly: not the multiplication arithmetic, but the four range-check lookups dragging in table commitments and accumulators.

---

# What each R5 instruction is doing

Below, each instruction states what it computes and — where relevant — what it must *prove* in the constraint system, since the proof obligation is the real cost driver. Instructions are grouped by family. (R5 here is the RISC-V-style instruction set the zkc emulator targets; the `W` suffix denotes a 32-bit "word" operation whose result is sign-extended to 64 bits.)

## Integer register-register arithmetic (R-type)

**ADD** — `rd = rs1 + rs2`.
*Proves:* one degree-1 equality. Trivial. Overflow beyond the word is dropped (wrapping).

**SUB** — `rd = rs1 − rs2`.
*Proves:* one degree-1 equality, same shape as ADD.

**ADDW / SUBW** — 32-bit add/subtract, result sign-extended to 64 bits.
*Proves:* the low-32-bit sum/difference, plus a sign-extension constraint tying bit 31 into the upper bits.

**SLL / SRL / SRA** — shift left logical / shift right logical / shift right arithmetic, by `rs2 mod 64`.
*Proves:* a shift is a multiplication or division by a power of two; the gadget decomposes the value into bits or limbs and reassembles, with range checks on the pieces. SRA additionally must preserve the sign bit.

**SLLW / SRLW / SRAW** — 32-bit shift variants, sign-extended.

**SLT / SLTU** — set-less-than (signed / unsigned): `rd = (rs1 < rs2) ? 1 : 0`.
*Proves:* the comparison via a subtraction and an inspection of the borrow/sign bit; the boolean result must be constrained to {0,1}.

**AND / OR / XOR** — bitwise logical operations.
*Proves:* these are not native to field arithmetic. The gadget decomposes both operands into bits and constrains the per-bit truth table, then reassembles — so each is a bit-decomposition plus range checks, more expensive than an ADD.

## Integer register-immediate arithmetic (I-type)

**ADDI** — `rd = rs1 + signext(imm12)`.
*Proves:* sign-extend the 12-bit immediate, then a degree-1 add. Slightly more handler work than ADD because of the sign extension.

**ADDIW** — 32-bit `rd = rs1 + signext(imm12)`, result sign-extended.

**SLTI / SLTIU** — set-less-than against an immediate (signed / unsigned).

**ANDI / ORI / XORI** — bitwise ops against an immediate; same bit-decomposition cost as their register forms.

**SLLI / SRLI / SRAI** — shift by an immediate amount; same shift-gadget cost as the register forms but with a constant shift.

## Multiply / divide (M-extension) — the expensive family

**MUL** — `rd = (rs1 × rs2)` low 64 bits.
*Proves:* limb decomposition, four degree-2 cross products, carry propagation, and range checks on every limb (see the worked example above). Moderate-to-expensive.

**MULH / MULHU / MULHSU** — high 64 bits of the 128-bit product (signed×signed / unsigned×unsigned / signed×unsigned).
*Proves:* the same partial-product machinery as MUL, but the carry-out into the upper word must also be constrained, so it is at least as expensive.

**MULW** — low 32 bits of a 32-bit product, sign-extended.

**DIV / DIVU** — signed / unsigned division: `rd = rs1 / rs2`.
*Proves:* the defining identity of division. Introduce quotient `q` and remainder `r` as witness columns and prove
```
a = q × b + r        and        0 ≤ r < b
```
The `q × b` term is a multiplication (degree 2, with its own limb/range machinery), and `0 ≤ r < b` is a range/comparison gadget. Division-by-zero and signed-overflow edge cases need extra constraints. This is the full division gadget — the most expensive arithmetic op.

**REM / REMU** — signed / unsigned remainder: `rd = rs1 mod rs2`.
*Proves:* identical gadget to DIV — you must produce both `q` and `r` to prove `a = q·b + r, 0 ≤ r < b` — and then output `r` instead of `q`. So REMU costs essentially the same as DIVU even though it "only" returns the remainder.

**DIVW / DIVUW / REMW / REMUW** — 32-bit division/remainder variants, sign-extended to 64 bits. REMUW (unsigned 32-bit remainder) is the instruction emitted by `% modulus` in field add/sub/double: same division gadget, narrowed to 32 bits, with the result sign-extended.

## Loads and stores

**LB / LH / LW / LD** — load byte / halfword / word / doubleword from memory, sign-extended.
**LBU / LHU / LWU** — load byte / halfword / word, zero-extended.
*Proves:* the loaded value matches memory at the computed address (a memory-consistency argument across the trace), plus the correct sign/zero extension and a range check that the value fits the loaded width.

**SB / SH / SW / SD** — store byte / halfword / word / doubleword to memory.
*Proves:* the stored value is written consistently, again via the memory argument; the narrow stores must constrain that only the intended bytes change.

## Control flow

**BEQ / BNE / BLT / BGE / BLTU / BGEU** — conditional branches (equal, not-equal, less-than, greater-or-equal, in signed and unsigned forms).
*Proves:* the comparison (a subtraction + flag inspection), and that the program counter took the correct next value depending on the boolean outcome.

**JAL / JALR** — jump-and-link (relative / register-indirect): set `rd = PC + 4` and jump.
*Proves:* the link value is the old PC plus the instruction width, and the new PC is the computed target.

## Upper-immediate and system

**LUI** — load upper immediate: `rd = imm << 12`.
*Proves:* a single shift/placement constraint. Cheap.

**AUIPC** — add upper immediate to PC: `rd = PC + (imm << 12)`.
*Proves:* one addition against the program counter. Cheap.

**ECALL / EBREAK** — environment call / breakpoint (system traps).
*Proves:* a transition into the handling convention; in a zkVM these usually mark I/O or halt boundaries and are constrained by the surrounding execution-control logic.

**FENCE** — memory ordering fence.
*Proves:* effectively a no-op for a single-threaded trace; constrained to advance the PC without changing state.

---

## The one-line takeaway

A flat cycle count (model A) tells you trace length, not proof cost. To get proof cost you multiply by how many constraints each instruction emits (model B) and how much each constraint costs (model C). Under that full accounting, the cheap-looking single-cycle instructions — `mul`, `div`, `rem`, and their `W` variants — are the expensive ones, because their one cycle expands into many trace rows carrying degree-2 products and range-check lookups.
