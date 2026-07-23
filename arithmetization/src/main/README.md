# RISC-V interpreter

## predecoding

RISC-V instructions arrive as raw 32-bit encoded words (type Instruction = u32). 
Without pre-decoding, the ZkC interpreter decodes each of these instructions *at runtime,
every step*: fetch the 32-bit word from RAM, peel off the `opcode`, derive the
instruction `type`, then split out the bitfields (`rd`, `rs1`, `imm`, …).
That work repeats for every executed instruction and adds to the machine's cost.

Because the program text is static, we can pre-decode **every instruction word
once, ahead of time**, and ship the results as an extra input table in the JSON.
At runtime the interpreter just does a table lookup by instruction index instead
of decoding on every step. 

### Decode input table

The decode input table is indexed by instruction address, not execution step:

```
index = (pc - instruction_base) / 4
```

This keeps the table proportional to program size (one record per 4-byte word
in the executable span). 
This table is dense ADD HERE

### Compute operation

The predecoding phase dispatches the original instructions into compute operations `compute_op`. 

### Justification

We must prove that the raw RISC-V program corresponds to the predecoded one, justifying that each structured instruction the interpreter executes with the predecoding corresponds to the raw word sitting in memory. A ZkC program re-reads each raw instruction word from the blob image and checks
that `decoded[index]` is consistent with the raw `(opcode, funct3, funct7, …)`
fields and operands.

| File | Role |
| ---- | ---- |
| `arithmetization/src/main/predecoding/main.zkc` | Entry point: linear scan over `[instruction_base, executable_region_end())` |
| `arithmetization/src/main/predecoding/predecoding.zkc` | Dispatches by instruction type + per-type operand checkers |
| `arithmetization/src/main/predecoding/executable_region.zkc` | Computes the end of the executable region |
| `arithmetization/src/main/predecoding/check/check_{b,i,r,j,u,s}_type.zkc` | Per instruction-format operand verification |
| `arithmetization/src/main/predecoding/read_instruction.zkc` | Fetches the raw 32-bit word at `pc` from blobs |
| `arithmetization/src/main/common/constants.zkc` | Canonical `OPCODE_*`, `FUNCT3_*`, `FUNCT7_*`, `RTYPE_*`, … constants |

`predecoding` derives the instruction type from the raw opcode and, per type,
verifies that `decoded[index].compute_op` and operands are consistent with the
raw `(opcode, funct3, funct7, …)` fields. `COMPUTE_MISC_MEM` and `COMPUTE_INVALID` need no operand checks once
the type/compute_op match.

