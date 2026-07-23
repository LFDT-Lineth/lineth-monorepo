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

## The JSON output

`printJson` emits the following keys. Each value is a single `0x…` hex string,
and the field set and order of every record **must** match the corresponding
`pub input` declaration in `arithmetization/src/main/riscv/memory.zkc`.

| Key                           | Record fields                                        | Purpose                                                             |
| ----------------------------- | ---------------------------------------------------- | ------------------------------------------------------------------ |
| `entry_point_and_blobs_count` | `entry_point:Address, blobs_count:u64`               | ELF entry point + number of sparse RAM blobs                       |
| `blobs_offset_and_size`       | `blob_offset:Address, blob_size:Length`              | RAM offset and byte length of each blob                            |
| `blobs_executable`            | `executable:u1`                                      | `1` if blob `i` holds executable (`SHF_EXECINSTR`) bytes, else `0` |
| `blobs_data`                  | `byte:u8`                                             | concatenated bytes of all blobs, in blob order                    |
| `instruction_base`            | `base:Address`                                       | lowest executable address (4-aligned); maps `pc` → table index    |
| `decoded`                     | `compute_op, imm, rs1, rs2, rd`                      | unified pre-decoded instruction table (one record per 4-byte word)|

The semantic operation, its writeback (`*_WB`) variant,
and rd=`x0` no-op folding are all encoded directly in `compute_op` (there is no
longer a separate `instruction_type` or `writeback` field). At runtime the
interpreter dispatches on a single flat `switch compute_op` (see
`interpreter.zkc`).

For I-type the 12-bit immediate (normalized for shifts at ELF time) is
sign-extended to 64 bits at decode time into `imm`.

For S-type the 12-bit store immediate is reassembled
(`imm[11] :: imm[10:5] :: imm[4:0]`) and sign-extended to 64 bits at decode
time, so the interpreter uses `imm` directly.

For B-type the 13-bit branch offset is reassembled
(`imm[12] :: imm[11] :: imm[10:5] :: imm[4:1] :: 0`) and sign-extended to 64
bits at decode time, so the interpreter uses `imm` directly.

For J-type the 21-bit jump offset is reassembled
(`imm[20] :: imm[19:12] :: imm[11] :: imm[10:1] :: 0`) and sign-extended to 64
bits at decode time, so the interpreter uses `imm` directly.

For U-type the upper immediate (`imm[31:12]`, lower 12 bits zeroed) is
sign-extended to 64 bits at decode time into `imm`.

## I-type semantic micro-ops (compute + writeback folded)

For an I-type instruction, the `decoded` record does not replay raw `funct3` /
opcode bits. Instead, `decodeITypeSemantic` in `main.go` maps each I-type
encoding to:

- **`compute_op`** — what to execute. Writeback-capable ops come in a pair:
  `READ8_SGN` (compute only) and `READ8_SGN_WB` (compute + `registers[rd] = result`).
  The even/odd pairing applies to the **local** op indices in `main.go`
  (base even, `*_WB` = base + 1); after adding `computeITypeBase` (1) to form the
  unified `ComputeOp` stored in the JSON, the parity flips — e.g.
  `READ8_SGN = 1` and `READ8_SGN_WB = 2` in `constants.zkc`. Control/system ops
  (`ITYPE_ECALL`, `ITYPE_EBREAK`, `ITYPE_INVALID`) have no `_WB` variant.
- **`imm`, `rs1`, `rd`** — operands (shift amounts are normalized at decode time;
  `imm` is the sign-extended 12-bit immediate)

When `rd` is `x0`, `itypeOpForRd` in `main.go` keeps the base opcode; otherwise it
selects the matching `*_WB` variant. This replaces the former separate `writeback`
field and second runtime switch.

At runtime, the interpreter's flat `switch compute_op` handles these cases
directly. Paired cases share compute logic; `*_WB` arms additionally write
`registers[rd]`.

Constants for `compute_op` live in `arithmetization/src/main/common/constants.zkc`
and are mirrored in `main.go` (`itypeOpAddi`, `itypeOpAddiWB`, …). See
`main_test.go` for `decodeITypeSemantic` and `itypeOpForRd` coverage.

## S-type semantic micro-ops (store width folded)

`decodeSTypeSemantic` maps each S-type `funct3` directly to a store-width opcode
(`STYPE_STORE8`, `STYPE_STORE16`, `STYPE_STORE32`, `STYPE_STORE64`). The store
offset is sign-extended into `imm` at ELF time. At runtime, the interpreter's
flat `switch compute_op` performs the RAM write. Invalid opcodes (including
`STYPE_INVALID`) are handled by the `default` arm.

## B-type semantic micro-ops

`decodeBTypeSemantic` validates the branch `funct3` and stores it directly as
the record's `compute_op` (`BRANCH_BEQ`, `BRANCH_BNE`, …). The 13-bit branch
offset is reassembled and sign-extended into `imm` at ELF time. At runtime, the
interpreter's flat `switch compute_op` handles the branch. Invalid opcodes
(including `BTYPE_INVALID`) are handled by the `default` arm.

## J-type semantic micro-ops (compute + writeback folded)

`decodeJTypeSemantic` maps JAL to base `JTYPE_JAL`; `jtypeOpForRd` selects
`JTYPE_JAL_WB` when `rd != x0`. The 21-bit jump offset is reassembled and
sign-extended into `imm` at ELF time. At runtime, the interpreter's flat
`switch compute_op` has separate `JTYPE_JAL` and `JTYPE_JAL_WB` cases; the `_WB`
case additionally writes the link address to `registers[rd]`. Invalid opcodes
(including `JTYPE_INVALID`) are handled by the `default` arm.

## U-type semantic micro-ops (compute + writeback folded)

`decodeUTypeSemantic` maps LUI/AUIPC to base opcodes; `utypeOpForRd` selects the
matching `*_WB` variant when `rd != x0`. The upper immediate is sign-extended
into `imm` at ELF time. At runtime, the interpreter's flat `switch compute_op`
has separate base and `_WB` cases. Invalid opcodes (including `UTYPE_INVALID`)
are handled by the `default` arm.

## R-type semantic micro-ops (compute + writeback folded)

For an R-type instruction, the `decoded` record does not replay raw `funct3` /
`funct7` / opcode bits. Instead, `decodeRTypeSemantic` maps each R-type encoding
to a base `compute_op`; `rtypeOpForRd` selects the matching `*_WB` variant when
`rd != x0`. Custom-1 precompiles (`RTYPE_KECCAK`, `RTYPE_POSEIDON2`) have no
`_WB` variant and return early after the precompile side effects.

### Custom-1 precompiles (`opcode` = `0b0101011`)

Both use `funct7 = 0b0000000`. `decodeRTypeSemantic` discriminates on `funct3`:

| `funct3` | Local op (`main.go`) | Unified `compute_op` (`constants.zkc`) | Runtime handler |
| -------- | -------------------- | --------------------------------------- | --------------- |
| `0b000`  | `rtypeOpKeccak` (56) | `RTYPE_KECCAK` (121)                    | `keccak(...)` in `interpreter.zkc` |
| `0b001`  | `rtypeOpPoseidon2` (57) | `RTYPE_POSEIDON2` (122)              | `poseidon2(...)` in `interpreter.zkc` |

Any other `(funct3, funct7)` pair on Custom-1 maps to `rtypeInvalid` → `COMPUTE_INVALID`
(255) in the `decoded` table.

At runtime, the interpreter's flat `switch compute_op` handles the base `RTYPE_*`
cases and the `RTYPE_*_WB` cases in `interpreter.zkc`; the `_WB` arms additionally
write `registers[rd]`. When `rd != x0`, `buildDecodedProgram` emits the `*_WB`
`compute_op`; Custom-1 precompiles have no `_WB` variant.

Constants for R-type `compute_op` and Custom-1 `funct3`/`funct7` live in
`arithmetization/src/main/common/constants.zkc` and are mirrored in `main.go`
(`rtypeOpAdd`, `rtypeOpAddWB`, `rtypeOpKeccak`, `rtypeOpPoseidon2`, …). Named
`FUNCT3_*` / `FUNCT7_*` constants are used by the pre-decoding verifier (see
below); redundant per-instruction `FUNCT7_*` aliases that duplicated
`FUNCT7_ADD` or `FUNCT7_MUL` have been removed from `constants.zkc`. See
`main_test.go` for `decodeRTypeSemantic` and `rtypeOpForRd` coverage.

## Static rd=x0 no-op folding

Some encodings write their result to `x0`, which RISC-V discards. When an
instruction has **no other visible effects** (no memory access, no control-flow
change, no non-`x0` register reads), `buildDecodedProgram` emits
`COMPUTE_MISC_MEM` (0) as the record's `compute_op`.

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
- **any Custom-1 precompile** (`KECCAK`, `POSEIDON2`) — memory side effects

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
   fields with shifts/masks. `classifyInstruction` in `main.go` derives the
   instruction type (`instructionTypeFromOpcode`), applies the semantic
   `decode*Semantic` map, folds writeback (`*OpForRd`) and rd=`x0` no-ops into a
   single unified `compute_op`, and `unifiedOperands` packs the operands.
5. **Bit-pack each record** into the `decoded` stream (see below).
6. **Hex-encode** the bit buffer into the hex string for the `decoded` JSON key.

## Bit-packing (the subtle part)

ZkC deserializes `pub input` records by packing each field at its **exact bit
width**, *not* rounded up to bytes. So the generator cannot just print
byte-aligned hex; it must emit a tightly packed bit stream.

`bitWriter.writeBits(val, width)` appends the low `width` bits of `val`,
most-significant bit first, into a continuous big-endian bit stream. Records are
concatenated with **no per-record alignment**, and the final byte is zero-padded
in its low bits. This mirrors ZkC's `EncodeBytes` / `DecodeUnsignedInt`.

The field widths come from the semantic types in
`arithmetization/src/main/common/type.zkc`, so each `decoded` record is the sum
of its field widths:

| Table     | Field widths (bits)                                    | Record size |
| --------- | ----------------------------------------------------- | ----------- |
| `decoded` | compute_op 8, imm 64, rs1 5, rs2 5, rd 5              | 87 bits     |

Operand fields not used by a given instruction format are written as zero (e.g.
`rs2 = 0` for I-type, `rd = 0` for S/B-type); see `unifiedOperands` in `main.go`.

> Important: if you change a field's type/width in `memory.zkc`, update the
> matching `writeBits` calls here (and vice versa). A width or order mismatch
> silently misaligns the whole stream and the interpreter reads garbage.


## Related files

- `arithmetization/src/main/riscv/memory.zkc` — the `pub input` declarations.
- `arithmetization/src/main/common/constants.zkc` — `compute_op`, opcode, and
  funct3/funct7 constants (must match `main.go`).
- `arithmetization/src/main/common/type.zkc` — field types and bit widths.
- `arithmetization/src/main/riscv/interpreter.zkc` — reads the unified `decoded`
  table by index and dispatches on a single flat `switch compute_op`.
- `arithmetization/src/main/predecoding/` — one-time proof that this tool's
  tables match the raw program image.
