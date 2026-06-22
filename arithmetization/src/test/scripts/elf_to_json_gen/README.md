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

| Key                | Record fields                                          | Replaces (in the interpreter)                              |
| ------------------ | ------------------------------------------------------ | ---------------------------------------------------------- |
| `instruction_base` | `base:Address`                                         | base address used to map `pc` → table index               |
| `decoded_core`     | `opcode, instruction_type, instruction_parameters`     | `instruction_parameters::opcode = instruction` + type map  |
| `decoded_itype`    | `compute_op, writeback, imm12, rs1, rd`                | flat semantic micro-op dispatch in `i_type.zkc`              |
| `decoded_rtype`    | `compute_op, writeback, rs1, rs2, rd`                  | flat semantic micro-op dispatch in `r_type.zkc`              |
| `decoded_stype`    | `imm12, rs2, rs1, funct3`                              | `imm_sign::uimm6::rs2::rs1::funct3::uimm5 = instruction_parameters` |
| `decoded_btype`    | `imm_sign, imm_10_5, rs2, rs1, funct3, imm_4_1, imm_11`| `imm_sign::imm_10_5::rs2::rs1::funct3::imm_4_1::imm_11 = instruction_parameters` |
| `decoded_jtype`    | `imm, rd`                                              | `imm20::imm10_1::imm11::imm19_12::rd = instruction_parameters` + sign-aware reassembly |
| `decoded_utype`    | `imm20, rd`                                            | `imm20::rd = instruction_parameters`                       |

Each value is a single `0x…` hex string. The field set and order of every table
**must** match the corresponding `pub input` declaration in
`arithmetization/src/main/riscv/memory.zkc`.

For S-type the 12-bit store immediate is reassembled
(`imm[11] :: imm[10:5] :: imm[4:0]`) into a single `imm12` field, so the
interpreter no longer has to recombine the split immediate.

For J-type the 21-bit signed jump offset is sign-extended into a single 64-bit
`imm` at decode time, so `j_type.zkc` does no shift or sign extension at runtime.

## I-type semantic micro-ops (split compute + writeback)

`decoded_itype` no longer replays raw `funct3` / opcode bits. Instead,
`decodeITypeSemantic` in `main.go` maps each I-type encoding to:

- **`compute_op`** — what to execute (`READ8_SGN`, `OP_ADDI`, `OP_SLLI`, `JALR`, …)
- **`writeback`** — whether to store the computed result into `rd` (`WB_STORE_REG` or `WB_NONE`)
- **`imm12`, `rs1`, `rd`** — operands (shift amounts are normalized into `imm12` at decode time)

At runtime, `process_I_type_instruction` runs a flat `switch compute_op`, then a
separate `switch writeback` with the usual `if (rd != 0)` guard. This is a
Zisk-like split (compute, then store-to-register) scoped to I-type for now;
`STORE_MEM` semantic ops for S-type are deferred.

Constants for `compute_op` / `writeback` live in `constants.zkc` and are mirrored
in `main.go` (`itypeOpAddi`, `wbStoreReg`, …). See `main_test.go` for
`decodeITypeSemantic` coverage.

## R-type semantic micro-ops (split compute + writeback)

`decoded_rtype` no longer replays raw `funct3` / `funct7` / opcode bits. Instead,
`decodeRTypeSemantic` in `main.go` maps each R-type encoding to:

- **`compute_op`** — what to execute (`RTYPE_ADD`, `RTYPE_MUL`, `RTYPE_KECCAK`, …)
- **`writeback`** — whether to store the computed result into `rd` (`WB_STORE_REG` or `WB_NONE`)
- **`rs1`, `rs2`, `rd`** — operands

At runtime, `process_R_type_instruction` runs a flat `switch compute_op`, then a
separate `switch writeback` with the usual `if (rd != 0)` guard. `RTYPE_KECCAK`
uses `WB_NONE` and returns early after the precompile side effects.

Constants for R-type `compute_op` live in `constants.zkc` and are mirrored in
`main.go` (`rtypeOpAdd`, …). See `main_test.go` for `decodeRTypeSemantic` coverage.

## Static rd=x0 no-op folding

Some encodings write their result to `x0`, which RISC-V discards. When an
instruction has **no other visible effects** (no memory access, no control-flow
change, no non-`x0` register reads), `buildDecodedProgram` rewrites
`decoded_core[i].instruction_type` to `MISC_MEM_TYPE` (7).

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
5. **Bit-pack each table** (see below).
6. **Hex-encode** each bit buffer into the hex string for its JSON key.

## Bit-packing (the subtle part)

ZkC deserializes `pub input` records by packing each field at its **exact bit
width**, *not* rounded up to bytes. So the generator cannot just print
byte-aligned hex; it must emit a tightly packed bit stream.

`bitWriter.writeBits(val, width)` appends the low `width` bits of `val`,
most-significant bit first, into a continuous big-endian bit stream. Records are
concatenated with **no per-record alignment**, and the final byte is zero-padded
in its low bits. This mirrors ZkC's `EncodeBytes` / `DecodeUnsignedInt`.

The field widths come from the semantic types in `utils/type.zkc`, so each record
size is the sum of its field widths:

| Table           | Field widths (bits)            | Record size |
| --------------- | ------------------------------ | ----------- |
| `decoded_core`  | opcode 7, type 3, params 25    | 35 bits     |
| `decoded_itype` | compute_op 6, writeback 2, imm12 12, rs1 5, rd 5 | 30 bits     |
| `decoded_rtype` | compute_op 6, writeback 2, rs1 5, rs2 5, rd 5 | 23 bits |
| `decoded_stype` | imm12 12, rs2 5, rs1 5, funct3 3 | 25 bits   |
| `decoded_btype` | imm_sign 1, imm_10_5 6, rs2 5, rs1 5, funct3 3, imm_4_1 4, imm_11 1 | 25 bits |
| `decoded_jtype` | imm 64, rd 5 | 69 bits |
| `decoded_utype` | imm20 20, rd 5 | 25 bits |

> Important: if you change a field's type/width in `memory.zkc`, update the
> matching `writeBits` calls here (and vice versa). A width or order mismatch
> silently misaligns the whole stream and the interpreter reads garbage.

## Related files

- `arithmetization/src/main/riscv/memory.zkc` — the `pub input` declarations.
- `arithmetization/src/main/riscv/interpreter.zkc` — reads `decoded_core` and
  dispatches by `instruction_type`.
- `arithmetization/src/main/riscv/instruction_processing/{i,r,s,b,j,u}_type.zkc` — read
  their respective decoded tables by index.
