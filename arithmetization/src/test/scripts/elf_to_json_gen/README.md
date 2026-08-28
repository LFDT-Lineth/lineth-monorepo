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
| `blobs_executable`            | `executable:u1`                                      | Optional. `1` if blob `i` holds executable (`SHF_EXECINSTR`) bytes, else `0`. Emitted only when `ELF2JSON_PREDECODING_PROOF=true` (offline predecoding justification in `predecoding/*.zkc`). |
| `blobs_data`                  | `byte:u8`                                             | concatenated bytes of all blobs, in blob order                    |
| `instruction_base`            | `base:Address`                                       | lowest executable address (4-aligned); maps `pc` → table index    |
| `decoded`                     | `compute_op, imm, rs1, rs2, rd`                      | unified pre-decoded instruction table (one record per 4-byte word)|

The semantic operation, its writeback (`*_WB`) variant,
and rd=`x0` no-op folding are all encoded directly in `compute_op`.

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

## I-type semantic micro-ops (writeback folded)

For an I-type instruction, the `decoded` record does not replay raw `funct3` /
opcode bits. Instead, `decodeITypeSemantic` in `main.go` maps each I-type
encoding to:

- **`compute_op`** — what to execute. When `rd != x0`, writeback-capable ops
  use the `*_WB` variant (e.g. `READ8_SGN_WB`, `OP_ADDI_WB`). When `rd == x0`,
  architecturally inert instructions emit `NO_OP` (0). Control/system
  ops (`ITYPE_JALR`, `ITYPE_ECALL`, `ITYPE_EBREAK`) keep their semantic op even
  when `rd == x0`.
- **`imm`, `rs1`, `rd`** — operands (shift amounts are normalized at decode time;
  `imm` is the sign-extended 12-bit immediate)

When `rd != x0`, `itypeOpForRd` selects the matching `*_WB` variant via
`finalizeComputeOp`. When `rd == x0`, inert paths map to `NO_OP`.

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

## U-type semantic micro-ops (writeback folded)

`decodeUTypeSemantic` maps LUI/AUIPC to local op indices; `finalizeComputeOp`
selects `UTYPE_*_WB` when `rd != x0` or `NO_OP` when `rd == x0`.
The upper immediate is sign-extended into `imm` at ELF time. At runtime, the
interpreter's flat `switch compute_op` handles `UTYPE_*_WB` cases. Invalid
opcodes (including `UTYPE_INVALID`) are handled by the `default` arm.

## R-type semantic micro-ops (writeback folded)

For an R-type instruction, the `decoded` record does not replay raw `funct3` /
`funct7` / opcode bits. Instead, `decodeRTypeSemantic` maps each R-type encoding
to a local op index; `finalizeComputeOp` selects `RTYPE_*_WB` when `rd != x0` or
`NO_OP` when `rd == x0` (except Custom-1 precompiles, which always
keep their semantic op). Custom-1 precompiles (`RTYPE_KECCAK`, `RTYPE_POSEIDON2`,
`RTYPE_WRITE_OUTPUT`) have no `_WB` variant and return early after the
precompile side effects.

### Custom-1 precompiles (`opcode` = `0b0101011`)

Both use `funct7 = 0b0000000`. `decodeRTypeSemantic` discriminates on `funct3`:

| `funct3` | Local op (`main.go`) | Unified `compute_op` (`constants.zkc`) | Runtime handler |
| -------- | -------------------- | --------------------------------------- | --------------- |
| `0b000`  | `rtypeOpKeccak` (28) | `RTYPE_KECCAK` (53)                     | `keccak(...)` in `interpreter.zkc` |
| `0b001`  | `rtypeOpPoseidon2` (29) | `RTYPE_POSEIDON2` (54)              | `poseidon2(...)` in `interpreter.zkc` |

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

## rd=x0 → NO_OP

When `rd == x0`, architecturally inert I/R/U instructions emit
`NO_OP` (0) as the record's `compute_op`. At runtime the interpreter
only advances `pc` by 4 — the same path used for `FENCE` / `FENCE.I`.

Examples mapped to `NO_OP`:

- `addi x0, t0, 1`, `add x0, t0, t1` (result discarded into x0)
- `ld x0, …` (load to x0 — no register writeback, no memory read at runtime)
- `lui x0, imm`, `auipc x0, imm`
- `addi x0, x0, 0`, `add x0, x0, x0` (strict no-ops)

Not mapped to `NO_OP` (keep semantic `compute_op`):

- `jal` / `jalr x0, …` — control flow
- branches and stores — side effects
- `ecall` / `ebreak` — syscalls
- **any Custom-1 precompile** (`KECCAK`, `POSEIDON2`, `WRITE_OUTPUT`) — memory side effects

The predicate lives in `shouldUseNoOp` / `finalizeComputeOp` in `main.go`
(see `main_test.go`).

## How the pre-decoding is done

All of this happens in `buildDecodedProgram`:

1. **Find the code region.** Scan **executable blobs** emitted by
   `extractProgramBlobs` (allocated `SHF_ALLOC` sections inside `PT_LOAD`, with
   `SHF_EXECINSTR`), and compute the span `[base, end)` with `base` aligned down
   and `end` up to a 4-byte boundary. Non-loadable exec sections are excluded so
   pre-decoding matches `blobs_data` / `read_instruction_from_blobs`.
2. **OOM guard.** The tables are *dense* (one record per 4-byte slot in the
   span), so an implausibly large / non-contiguous span is rejected. The cap is
   `defaultMaxDecodedRecords` (2,000,000), overridable via the
   `ELF2JSON_MAX_DECODED_RECORDS` environment variable.
3. **Flatten to a byte image.** Copy each executable blob into a zero-filled
   buffer so it can be indexed contiguously (gaps read as zero).
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


## Environment variables

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `ELF2JSON_PREDECODING_PROOF` | `false` | When `true`, also emit `blobs_executable` for the offline predecoding justification pass. Decode tables (`instruction_base`, `decoded`) are always emitted. Makefile passthrough: `PREDECODING_PROOF=true`. |
| `ELF2JSON_WRITE_SECTIONS` | `false` | When `true`, write a `.sections` debug file next to the ELF. |
| `ELF2JSON_MAX_DECODED_RECORDS` | `2000000` | Cap on pre-decoded instruction records (OOM guard). |


## Related files

- `arithmetization/src/main/riscv/memory.zkc` — the `pub input` declarations.
- `arithmetization/src/main/common/constants.zkc` — `compute_op`, opcode, and
  funct3/funct7 constants (must match `main.go`).
- `arithmetization/src/main/common/type.zkc` — field types and bit widths.
- `arithmetization/src/main/riscv/interpreter.zkc` — reads the unified `decoded`
  table by index and dispatches on a single flat `switch compute_op`.
- `arithmetization/src/main/predecoding/` — one-time proof that this tool's
  tables match the raw program image.
