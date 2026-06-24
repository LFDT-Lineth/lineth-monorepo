# elf_to_json_gen

Generates the ZkC JSON input file for a given RISC-V ELF binary. It does two
jobs:

1. Converts the ELF's loadable sections (and an optional `IN_BYTES` input) into
   the RAM blobs the interpreter loads before execution.
2. **Statically pre-decodes** every instruction in the executable region into
   lookup tables, so the interpreter can skip per-step instruction decoding.

```
go run main.go <elfFile> <inBytes|@hexFile> <inBytesOffset> > input.json
```

## What pre-decoding is and why

Without pre-decoding, the ZkC interpreter decodes each instruction *at runtime,
every step*: fetch the 32-bit word from RAM, peel off the `opcode`, derive the
instruction `type`, then split out the per-format fields (`rd`, `rs1`, `imm`, …).
That work repeats for every executed instruction and adds to the machine's cost.

Because the program text is static, this tool decodes **every instruction word
once, ahead of time**, and ships the results as extra input tables in the JSON.
At runtime the interpreter just does a table lookup by instruction index instead
of decoding.

The tables are indexed by instruction address, not execution step:

```
index = (pc - instruction_base) >> 2
```

This keeps the tables proportional to program size (one record per 4-byte word
in the executable span), not to the number of executed steps.

## How the JSON output changed

In addition to the original keys (`entry_point_and_blobs_count`,
`blobs_offset_and_size`, `blobs_data`), `printJson` now emits:

| Key                | Record fields                                                                          | Role                                          |
| ------------------ | ------------------------------------------------------------------------------------- | --------------------------------------------- |
| `instruction_base` | `base:Address`                                                                        | base address used to map `pc` → table index   |
| `decoded`          | `operation, a_src, b_src, read_kind, write_kind, pc_kind, rs1, rs2, rd, imm`          | the single unified pre-decoded instruction    |

Each value is a single `0x…` hex string. The field set and order of the record
**must** match the `decoded` `pub input` declaration in
`arithmetization/src/main/riscv/memory.zkc`.

## The unified instruction model (zisk-style pipeline)

Every RISC-V instruction is reduced at decode time to ONE uniform record driving
the interpreter's four-phase pipeline (read → compute → write → advance PC), so
the interpreter no longer switches on the instruction type. `decodeUnified` in
`main.go` maps each encoding to:

- **`operation`** — the compute op applied to operands `a` and `b`
  (`OPR_ADD`, `OPR_SLL`, `OPR_MUL`, `OPR_MOVE_LOADED`, `OPR_LINK`, `OPR_CMP_EQ`,
  `OPR_KECCAK`, `OPR_NOP`, …). Immediate and register ALU forms share one op
  (e.g. ADD/ADDI, SLL/SLLI), with operand `b` selected by `b_src`.
- **`a_src` / `b_src`** — operand sources: `a` is `registers[rs1]` or `pc`
  (`A_RS1` / `A_PC`); `b` is `registers[rs2]` or the immediate (`B_RS2` / `B_IMM`).
- **`read_kind`** — memory load width/extension for loads (address = `a + b`):
  `RK_NONE`, `RK_8S`, …, `RK_32U`.
- **`write_kind`** — writeback target: `WK_REG` (register `rd`), `WK_MEM8..64`
  (store `registers[rs2]`), or `WK_NONE`.
- **`pc_kind`** — pc update: `PK_NEXT`, `PK_BRANCH`, `PK_JUMP_REL` (JAL),
  `PK_JUMP_ABS` (JALR), `PK_SYSCALL` (ECALL), `PK_HALT` (EBREAK).
- **`rs1`, `rs2`, `rd`** — register indices (`rs2` also carries store data).
- **`imm`** — a single 64-bit immediate, fully sign-extended at decode time for
  I/S/B/J/U; shift forms carry the raw shift amount (no sign extension); U-type
  carries `sext32(imm20 << 12)`.

`decodeUnified` reuses the per-type semantic decoders `decodeITypeSemantic` and
`decodeRTypeSemantic` (and the I/R-type `compute_op` constants in `constants.zkc`,
mirrored in `main.go`) to derive `operation` and the selectors. See `main_test.go`
for `decodeITypeSemantic` / `decodeRTypeSemantic` coverage.

## Static rd=x0 no-op folding

Some encodings write their result to `x0`, which RISC-V discards. When an
instruction has **no other visible effects** (no memory access, no control-flow
change, no non-`x0` register reads), `buildDecodedProgram` rewrites its type to
`miscMemType`, which `decodeUnified` emits as `OPR_NOP` (no compute, no writeback,
`PK_NEXT`).

At runtime the interpreter then only advances `pc` by 4 — the same path used for
`FENCE` / `FENCE.I`.

Examples folded:

- `addi x0, x0, imm` (NOP / hint)
- `lui x0, imm`
- `add/xor/… x0, x0, x0` (R-type with `rs1 = rs2 = x0`)

Not folded (still have side effects):

- `ld x0, …` — memory read
- `jal/jalr x0, …` — control flow
- `addi x0, t0, imm` — reads `t0`
- `auipc x0, imm` — uses PC in the computation

The predicate lives in `isRdZeroNoop` in `main.go` (see `main_test.go`).

## How the pre-decoding is done

All of this happens in `buildDecodedProgram`:

1. **Find the code region.** Scan ELF sections, keep the executable, file-backed
   ones (`SHF_EXECINSTR`), and compute the span `[base, end)` with `base` aligned
   down and `end` up to a 4-byte boundary.
2. **OOM guard.** The tables are *dense* (one record per 4-byte slot in the
   span), so an implausibly large / non-contiguous span is rejected. The cap is
   `defaultMaxDecodedRecords` (2,000,000), overridable via the
   `ELF2JSON_MAX_DECODED_RECORDS` environment variable.
3. **Flatten to a byte image.** Copy each section into a zero-filled buffer so it
   can be indexed contiguously (gaps read as zero).
4. **Decode each word.** Read the little-endian 32-bit instruction and extract
   fields with shifts/masks. `instructionTypeFromOpcode` reproduces the ZkC
   `instruction_type_from_opcode` mapping (and the constants mirror
   `constants.zkc`). `isRdZeroNoop` may rewrite the type to `MISC_MEM_TYPE`.
5. **Decode to the unified record** (`decodeUnified`) and **bit-pack** it (see below).
6. **Hex-encode** the bit buffer into the hex string for the `decoded` JSON key.

## Bit-packing (the subtle part)

ZkC deserializes `pub input` records by packing each field at its **exact bit
width**, *not* rounded up to bytes. So the generator cannot just print
byte-aligned hex; it must emit a tightly packed bit stream.

`bitWriter.writeBits(val, width)` appends the low `width` bits of `val`,
most-significant bit first, into a continuous big-endian bit stream. Records are
concatenated with **no per-record alignment**, and the final byte is zero-padded
in its low bits. This mirrors ZkC's `EncodeBytes` / `DecodeUnsignedInt`.

The field widths come from the semantic types in `utils/type.zkc`, so the record
size is the sum of its field widths:

| Table     | Field widths (bits)                                                                           | Record size |
| --------- | -------------------------------------------------------------------------------------------- | ----------- |
| `decoded` | operation 6, a_src 1, b_src 1, read_kind 3, write_kind 3, pc_kind 3, rs1 5, rs2 5, rd 5, imm 64 | 96 bits     |

> Important: if you change a field's type/width in `memory.zkc`, update the
> matching `writeBits` calls here (and vice versa). A width or order mismatch
> silently misaligns the whole stream and the interpreter reads garbage.

## Related files

- `arithmetization/src/main/riscv/memory.zkc` — the `decoded` `pub input` declaration.
- `arithmetization/src/main/riscv/interpreter.zkc` — reads `decoded` and runs the
  four-phase pipeline (no instruction-type dispatch).
- `arithmetization/src/main/riscv/pipeline/{read,effect,write,pc}.zkc` — the read,
  compute, write, and advance-PC phases.
