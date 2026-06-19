# 2. Arithmetization

## 2.1 Overview

Arithmetization is the process of converting VM execution semantics into algebraic
constraints that a proving system can consume. It is the bridge between an
execution trace and a cryptographic proof: it defines *what it means* for a trace
to represent a valid RISC-V execution, expressed as a system of algebraic
constraints over a finite field.

In this system the arithmetization is not a hand-written circuit. It is a
**RISC-V interpreter written in ZkC** (§2.5), compiled by the `zkc` toolchain into
an algebraic constraint system. The interpreter defines the machine; the compiler
turns its execution into constraints. The arithmetization therefore lives as
ZkC source code — which is then regenerated into constraints on each build —
rather than as a hand-maintained constraint listing.

## 2.2 Design Philosophy: a no-CPU architecture

The arithmetization follows a **no-CPU** style. The system *does* have a
fetch–decode–execute datapath and a top-level dispatcher — any RISC-V
arithmetization must, and ours is a function currently named `interpreter()` in
the ZkC source — and the no-CPU label is **not** a denial of either. What it
names is the choice to avoid materializing the program's trace as **one
monolithic chip / module whose trace size is $O(n)$ in the number of executed
R5 instructions**. Instead the work is distributed across a collection of
specialized **tables**, each with its own much smaller trace. This design
originates with **OpenVM** (initiated by Axiom, Scroll, and collaborators); this
system adopts that structure.

Two distinct constraint mechanisms link the tables together; they are not the
same thing and the spec is careful to distinguish them:

- **The message bus**, for inter-instruction control transfer. Advancing from
  one instruction to the next is a message on the bus: a step "emits" its
  successor state onto the bus and the next instruction's handler "receives"
  it. The dispatcher and the bus coexist — the dispatcher selects which
  handler runs at each step, and the bus is the medium by which the resulting
  state is passed onward to the next step.
- **Lookup (inclusion) constraints**, for local function calls. When an
  instruction handler delegates a sub-computation — sign extension, signed
  comparison, multiplication/division, and similar — to a helper function, the
  call site is realized as a **lookup constraint** into the helper's table, not
  as a bus connection. The bus is *not* the universal interconnect; it is
  reserved for inter-instruction transfer, while intra-instruction delegation
  uses lookups.

Both mechanisms are reduced to log-derivative-style arguments by the
cryptographic compilation in §3 (the inclusion pass of §3.3.2 and the
log-derivative-sum reduction of §3.3.3), but they remain architecturally
distinct primitives serving different purposes.

This decomposition is what makes the no-CPU design pay off:

- **Smaller traces.** No single table carries an $O(n)$-row trace; each
  table's trace size is governed by how often its piece of work is exercised.
- **Reuse.** A sub-computation realized as its own helper table is shared by
  every handler that calls it (via the lookup), so the cost of arithmetizing an
  operation is paid once. (For related reasons, recursion is frequently more
  constraint-efficient than iteration in ZkC; see §2.5.)
- **Modularity.** Each instruction handler and each helper table (sign
  extension, signed comparison, absolute value, multiplication/division, …) is
  an isolated unit that can be specified, tested, and optimized independently.
- **Bounded local complexity.** No single table has to encode the whole ISA;
  complexity stays local to each handler.


## 2.3 Execution Model

The arithmetized VM models the architectural state and its transitions:

- the general-purpose **register file**;
- the **program counter** (`pc`);
- **memory** (a read/write RAM, plus the input/output and static memories of the
  ZkC model — §2.5);
- **instruction decoding** state (the fields extracted from each instruction
  word);
- the **execution transition** from one step to the next, advanced by a clock
  cycle.

Execution is a fetch–decode–dispatch loop. At each step the interpreter reads the
32-bit instruction word at `pc`, increments the clock cycle, decodes the
instruction, dispatches on its opcode group, applies the instruction's semantics
(updating registers and/or memory), and produces the next `pc`. A distinguished
`pc` sentinel value signals termination (set by the environment-call path).

Each execution step must satisfy constraints proving that:

1. the instruction was decoded correctly from the fetched word;
2. the operands were selected correctly (register reads, immediate
   reconstruction);
3. the instruction semantics were applied correctly;
4. the resulting machine-state transition (registers, memory, `pc`) is valid.

## 2.4 Opcode-Specific Arithmetization

> *This subsection records the opcode-family structure at a level useful for
> orientation; the per-opcode constraint definitions are owned by the team
> member responsible for instruction decoding and the detailed content is
> pending their review.*

Instruction behaviour is organized by **instruction format**, matching the RISC-V
encoding. After the 7-bit opcode is stripped, each format is dispatched to a
dedicated processor function (`process_R_type_instruction`, and the I/S/B/U/J
analogues), with `funct3`/`funct7` selecting the operation within a family.

The opcode groups covered are:

| Group     | Type | Representative instructions |
| --------- | ---- | --------------------------- |
| LOAD      | I    | LB, LH, LW, LD, LBU, LHU, LWU |
| MISC-MEM  | I    | FENCE, FENCE.I |
| OP-IMM    | I    | ADDI, SLTI, SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI |
| AUIPC     | U    | AUIPC |
| OP-IMM-32 | I    | ADDIW, SLLIW, SRLIW, SRAIW |
| STORE     | S    | SB, SH, SW, SD |
| OP        | R    | ADD, SUB, SLL, SLT, SLTU, XOR, SRL, SRA, OR, AND, MUL, MULH, MULHSU, MULHU, DIV, DIVU, REM, REMU |
| LUI       | U    | LUI |
| OP-32     | R    | ADDW, SUBW, SLLW, SRLW, SRAW, MULW, DIVW, DIVUW, REMW, REMUW |
| BRANCH    | B    | BEQ, BNE, BLT, BGE, BLTU, BGEU |
| JALR      | I    | JALR |
| JAL       | J    | JAL |
| SYSTEM    | I    | ECALL, EBREAK, CSRRW, CSRRS, CSRRC, CSRRWI, CSRRSI, CSRRCI |

Signed operations (SLT, SRA, MULH, MULHSU, DIV, REM, and their `*W` variants)
treat register contents as two's-complement; division and remainder follow
RISC-V's defined behaviour for division by zero and signed overflow. These
semantics are realized through shared helper functions (sign bit, sign
extension, absolute value, negation, signed comparison) that the family
processors call — and which, per §2.2, are realized as their own tables that
handlers reach via lookups.

This specification deliberately describes opcode handling at the level of *which
families exist and how they are dispatched and constrained*, and relies on the
arithmetization source for the exact per-opcode constraint definitions (§2.9).

## 2.5 Relationship to ZkC and the Proving Boundary

The arithmetization is written in **ZkC**, a small imperative language for
programs whose executions can be proved, maintained by the Linea arithmetization
team. ZkC's relevant characteristics:

- **Fixed-width unsigned integers of arbitrary bitwidth** (`u1`, `u25`, `u160`,
  …), which lets the interpreter model exact bit-level RISC-V encodings and
  field-element widths directly.
- **Explicit memory** declarations (`input` / `output` / `memory` / `static`),
  with **module-level** `pub` annotation. The `pub` keyword marks an entire
  module — not an individual column — as public; modules so marked become the
  public-input modules of the proof, while unmarked modules are private. A
  public-input module carries one or more columns of the same size; it is not
  required to be statically sized. How a set of pub-modules is materialized
  into the concrete public input of the produced proof is a design discussion
  in progress (see the open-items note below).
- **`fail`**, which terminates execution and renders the generated constraints
  unsatisfiable for any trace reaching it — used to enforce preconditions.
- **Local function calls realized as lookup constraints.** Sub-computations
  delegated to helper functions (sign extension, signed comparison, and so on)
  compile to **lookup (inclusion) constraints** into the helper's table. This
  is the mechanism underlying the no-CPU decomposition for intra-instruction
  delegation (§2.2). Note this is distinct from the **message bus**, which is
  reserved for inter-instruction control transfer; the two are separate
  constraint mechanisms that the spec is careful not to conflate.
- **`printf`**, a debugging aid with no effect on the generated constraints.

**Public-input materialization.** The public inputs of a guest program are technically its output.
They are stored inside a ZkC module tagged as `pub`.
The prover reads the public inputs from this module and attaches them to the proof.
The prover may add its own public inputs on top and send
the **Proof**, **PublicInputs**, and **VerificationKey** to the verifier.
   
**The compilation boundary.** The arithmetization is compiled by the `zkc`
toolchain (part of go-corset) into a go-corset **`air.Schema`** — an Algebraic
Intermediate Representation over the KoalaBear field — together with an execution
**trace** (the assignment). This `(AIR, assignment)` pair is the entire interface
between arithmetization and proving:

```
RISC-V program + inputs
        │
        ▼  (executed by the ZkC RISC-V interpreter)
 execution trace
        │
ZkC interpreter source ──zkc compile──▶ air.Schema (AIR)
        │                                    │
        └──────────────┬─────────────────────┘
                       ▼  zkcdriver
              wiop.System  +  Runtime
                       │
                       ▼
              Arcane compiler  (§3)
```

Everything to the left of `zkcdriver` is ZkC / go-corset; everything from the
`wiop.System` rightward is the cryptographic compilation of §3. The
**`zkcdriver`** integration layer is the concrete bridge: it scans the
`air.Schema`'s modules, columns, and constraints into a `wiop.System` (preserving
a corset-name → column map), and populates the `Runtime` with the trace. The
proving system is therefore independent of arithmetization details: it sees only
columns and constraints.

## 2.6 The go-corset Compilation Pipeline

The single arrow "ZkC → AIR" used above is, in go-corset, a lowering through
several distinct internal intermediate representations rather than a single
step; the toolchain can dump any of these levels for inspection. The set of
levels and their names are an implementation detail of go-corset and are
expected to change over time, so this specification deliberately does **not**
take a dependency on them.

The single object this specification *does* depend on is **AIR** — the
arithmetic intermediate representation. AIR is the final form go-corset emits:
the schema of bounded-degree vanishing polynomial constraints, range
constraints, and lookup / bus arguments that becomes the Wizard AIR
(`wiop.System`) consumed by §3. The pre-compiled binary (`.bin`) that
`zkcdriver` reads (§2.5) is a serialized AIR schema, so the AIR boundary is
also the boundary at which the arithmetization can be cached and reloaded
without recompiling from source.

## 2.7 Precompile Arithmetization

Cryptographic precompiles are treated differently from ordinary instructions
(see also §1.4 and §5). Rather than being unrolled into generic VM steps, a
precompile invocation is:

1. **detected** during execution;
2. **routed** to a specialized constraint definition for that primitive;
3. **compiled** through the same ZkC → AIR path into dedicated constraints;
4. **injected** into the overall proving workflow.

This preserves soundness while avoiding the prohibitive cost of expressing large
cryptographic computations (e.g. Keccak-f) through generic instructions. The
detailed detection-and-dispatch mechanism is the subject of §5; this section only
records that precompile constraints enter the pipeline through the same
arithmetization/proving boundary as ordinary instructions.

## 2.8 Trace and Prover Interaction

The arithmetization consumes execution traces produced by running the ZkC
interpreter. Traces should be made available to the proving pipeline with minimal
latency: in production deployments, trace generation and proving are expected to
run in close proximity, to avoid data-transfer overhead and to meet the
low-latency target of §1.2. The trace is delivered to the prover as the
assignment side of the `(AIR, assignment)` interface (§2.5), i.e. as the
column data the `Runtime` is populated with.

## 2.9 External References

This specification describes the arithmetization at the architectural and
interface level and relies on the implementation repositories for opcode-level
constraint definitions:

- **ZkC language reference** — `ZKC_LANGUAGE.md` in the go-corset repository
  (`github.com/Consensys/go-corset`), and the `zkc` toolchain (`cmd/zkc`,
  `zkc compile`).
- **ZkC compiler architecture** — `ZKC_ARCHITECTURE.md` and
  `docs/ZKC_ARCHITECTURE_{AST,IR,AIR}.md` in go-corset, which describe the
  front-end → IR → arithmetization pipeline and the **bus** primitive (a call
  site is a bus, discharged at the AIR layer as a lookup argument).
- **RISC-V arithmetization source** — the ZkC interpreter under
  `arithmetization/src/main/riscv/` (`main.zkc`, `interpreter.zkc`, the
  `instruction_processing/*_type.zkc` family processors, `memory.zkc`,
  `ram/`, and `utils/`).
- **Opcode reference** — `arithmetization/docs/rv64im-zicclsm-opcodes.md` for the
  per-format encodings and the full opcode/extension coverage.
- **Integration layer** — `prover-ray/zkcdriver`, which ports a go-corset
  `air.Schema` and its trace into a `wiop.System` and `Runtime`.
- **No-CPU architecture** — OpenVM (`openvm.dev`), which introduced the no-CPU
  zkVM design this arithmetization adopts; see the OpenVM whitepaper for the
  originating description.