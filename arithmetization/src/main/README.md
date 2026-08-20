# RISC-V interpreter

ZkC arithmetization for a RISC-V guest VM. 

## Folder structure

| Path           | Entry point  | Role |
| -------------- | ------------ | ---- |
| `riscv/`       | `main.zkc`   | Full VM — predecoding justification, ELF blob load into RAM, PC-driven execution via `interpreter.zkc` |
| `predecoding/` | `main.zkc`   | Standalone justification — same linear scan as phase 1 of `riscv/main.zkc`, without blob load or execution |
| `common/`      | —            | Shared types (`type.zkc`) and opcode / `compute_op` constants (`constants.zkc`) |
| `lib/`         | —            | Guest accelerators (Keccak, Poseidon2); see [`lib/README.md`](lib/README.md) |

Offline JSON inputs are produced by
[`../test/scripts/elf_to_json_gen/`](../test/scripts/elf_to_json_gen/README.md).

## Predecoding

RISC-V instructions arrive as raw 32-bit encoded words (`Instruction = u32`).
Without predecoding, the interpreter would decode every instruction **at runtime,
every step**: fetch the word from RAM, peel off `opcode`, derive the instruction
type, then split bitfields (`rd`, `rs1`, `imm`, …). That work repeats for each
executed instruction and adds to trace cost.

Because program text is static, we **pre-decode each word once, offline**, and
pass the result as a public input table. At runtime the interpreter performs a
table lookup by instruction index instead of decoding on every step. Each row is
a structured record: `compute_op`, `rd`, `rs1`, `rs2`, sign-extended immediates,
and normalized shift amounts.

### Decode input table

The table is indexed by **instruction address**, not execution step:

```
index = (pc - instruction_base) / 4
```

One record exists per 4-byte word in the executable span, so table size tracks
program size — not step count. A loop that runs one instruction a million times
still reads the same slot.

The layout is **dense** over `[instruction_base, executable_region_end)`:
`(pc - instruction_base) / 4` subscripts directly into `decoded[]`, matching
PC-driven execution. Each step holds `pc`; branches and jumps update it; `index`
is derived in constant time. A dense layout avoids per-step blob walks or raw
word decoding from `blobs_data`. Gaps between executable ELF sections (e.g.
exec → data → exec in the address map) still occupy slots as `COMPUTE_INVALID`,
keeping the PC → index mapping uniform across the whole span.

### Compute operation and bitfields

At ELF time, `elf_to_json_gen` maps each raw word to a semantic dispatch tag
`compute_op` plus normalized operands in a unified `decoded[]` record:

```
compute_op, imm, rs1, rs2, rd = decoded[index]
```

The interpreter does not branch on `(opcode, funct3, funct7)` at runtime — it
`switch`es on `compute_op` only (`interpreter.zkc`).

| Kind                      | Role |
| ------------------------- | ---- |
| `OP_*_WB` / `RTYPE_*_WB` / `UTYPE_*_WB` | Compute plus writeback to `registers[rd]` when `rd != x0` |
| `COMPUTE_MISC_MEM` (0)    | `FENCE` and all architecturally inert `rd=x0` paths — advance `pc` only |
| `JTYPE_JAL`, `ITYPE_JALR`, branches, stores, syscalls, precompiles | Semantic ops preserved even when `rd=x0` |
| `COMPUTE_INVALID` (255)   | Words the interpreter does not model (gaps, padding, unsupported opcodes). Execution **fails only if `pc` reaches that slot** (jump or fall-through into a gap). |

Immediates are sign-extended and shift amounts normalized in Go; the interpreter
reads a ready-to-use `imm:DoubleWord`. Unused operand fields (e.g. `rs2` on
I-type) are stored as zero.

### Justification

We must prove that the raw RISC-V program matches the predecoded table: for each
address in the executable span, the raw blob word and `decoded[index]` describe
the same instruction. A ZkC program re-reads each raw word and checks that
`compute_op` and operands are consistent with `(opcode, funct3, funct7, …)`.
`COMPUTE_MISC_MEM` and `COMPUTE_INVALID` need no operand checks once the
type / `compute_op` match.

The justification pass runs in `predecoding/` (or as phase 1 of `riscv/main.zkc`).
After it succeeds, the interpreter trusts `decoded[]` for every executed step.

| File                              | Role |
| --------------------------------- | ---- |
| `predecoding/main.zkc`            | Entry: linear scan over `[instruction_base, executable_region_end)` |
| `predecoding/predecoding.zkc`     | Dispatch by instruction type + per-type operand checkers |
| `predecoding/executable_region.zkc` | End of executable region from `blobs_executable` |
| `predecoding/read_instruction.zkc`| Fetch raw 32-bit word at `pc` from executable blobs |
| `predecoding/check/check_*_type.zkc` | Per-format operand verification (B/I/R/J/U/S) |
| `common/constants.zkc`            | Canonical `OPCODE_*`, `FUNCT3_*`, `FUNCT7_*`, `RTYPE_*`, … |

### Offline predecoded program generation (`elf_to_json_gen`)

Predecoding happens **offline**, before ZkC execution. The Go tool
[`elf_to_json_gen`](../test/scripts/elf_to_json_gen/) reads a RISC-V ELF and
emits JSON public inputs for `riscv/memory.zkc`:

| Output group | Keys | Purpose |
| ------------ | ---- | ------- |
| RAM image    | `entry_point_and_blobs_count`, `blobs_offset_and_size`, `blobs_executable`, `blobs_data` | Sparse loadable sections (+ optional `IN_BYTES` at a fixed offset) |
| Decode table | `instruction_base`, `decoded[]` | Lowest executable address and one dense row per 4-byte word; inter-section gaps → `COMPUTE_INVALID` |

[`Predecoded program generation README`](../test/scripts/elf_to_json_gen/README.md)

```shell
# direct invocation
cd arithmetization/src/test/scripts/elf_to_json_gen
go run main.go <elfFile> <inBytes|@hexFile> <inBytesOffset> > input.json
```
