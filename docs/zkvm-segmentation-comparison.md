# Segmentation & Cross-Table Lookups: RISC Zero, SP1, and Linea Limitless

A comparison of how three ZK proving systems handle large computations by splitting execution into segments and managing global constraints (lookups, memory, permutations) across those segments.

## 1. The Core Problem

A single monolithic execution trace is too large to prove in one shot — it exhausts memory, time, or both. All three systems solve this with the same high-level pattern:

1. **Segment** the execution into smaller, independently provable pieces
2. **Prove** each segment (ideally in parallel)
3. **Aggregate** the segment proofs into a single final proof

The differences lie in *how* they segment, *what constraints span segments*, and *how they handle those global constraints*.

---

## 2. Architecture Overview

| | RISC Zero | SP1 | Linea Limitless |
|---|---|---|---|
| **VM** | RISC-V (rv32im) | RISC-V (rv32im) | EVM (custom arithmetization) |
| **Proof system** | STARK (FRI-based) | STARK → Hypercube/multilinear (Plonky3) | Wizard-IOP → gnark (Plonk + Groth16) |
| **Segmentation term** | Continuations / Segments | Shards | Limitless / Segments |
| **Segment granularity** | Temporal (time slices of execution) | Temporal (~2M cycle chunks) | Structural (by module + row bands) |
| **Precompile architecture** | Extended instructions in single CPU circuit | Separate chips/tables per precompile | Separate modules per precompile |
| **Cross-segment constraint type** | State continuity only (Merkle roots) | Cross-table lookups (logUp → LogUp-GKR) + EC multiset hash (memory) | Cross-module lookups (log-derivative sums) |

---

## 3. Precompile / Accelerator Design

### RISC Zero: Accelerators as Extended Instructions

Precompiles are embedded as **ECALL sub-circuits within the single RISC-V circuit**. When the guest calls `sys_bigint(...)`, the circuit has specialized constraints that verify the result in fewer cycles than software emulation. The key mechanism for BigInt: the host provides the product `c` as advice, the verifier gives randomness `r`, and the circuit checks `a(r) * b(r) == c(r)` (polynomial identity testing in the auxiliary trace).

Supported: SHA-256, Keccak (patched `tiny-keccak`), secp256k1 (`k256`), P-256, Ed25519/Curve25519, RSA, BLS12-381, BigInt modular arithmetic, KZG.

**Consequence**: Everything stays in one trace. No cross-table lookups needed.

### SP1: Precompiles as Separate Chips

Each precompile is a **dedicated STARK table (chip)** with its own AIR constraints. The CPU chip dispatches to precompile chips via the `ecall` instruction; each chip records events and generates its own trace. Chips communicate via **cross-table lookups** (send/receive interactions compiled into logUp).

Supported: SHA-256, Keccak-256, secp256k1, secp256r1, Ed25519, BLS12-381, RSA, BigInt, uint256.

**Consequence**: Cross-table lookups are needed between CPU ↔ precompile chips, creating global constraints that must be managed across shards.

### Linea: Precompiles as Dedicated Modules

Each EVM precompile is a **completely separate arithmetic module** with its own constraint system, optimized for the specific operation. The Hub module dispatches to precompile modules via **cross-module lookups** (log-derivative arguments).

Supported: Keccak, ECDSA/ecrecover, ModExp, EC Add/Mul/Pair, SHA-256, BLS (G1/G2 Add, MSM, Map, Pairing), EIP-4844 Point Evaluation, P256 signature verification.

**Consequence**: Cross-module lookups are intrinsic to the architecture. When the trace is segmented, these lookups span segments and require a global consistency mechanism.

---

## 4. How Each System Handles Lookups Across Segments

### RISC Zero: Lookups Are Local (No Cross-Segment Lookups)

RISC Zero sidesteps the problem entirely. Their architecture makes every lookup self-contained within a single segment:

- **Memory verification**: Within each segment, memory operations are committed in execution order and address-sorted order. The PLONK grand product accumulator runs entirely within the segment (starts at 1, ends at 1).
- **Cross-segment memory consistency**: Handled via **memory image Merkle roots** — a Merkle tree of the full memory state is committed at each segment boundary. During recursive composition, the verifier checks `segment[i].ending_root == segment[i+1].starting_root`.
- **Range checks (PLOOKUP)**: The bytes table `{0..255}` is a fixed, constant table replicated in every segment. Each segment's PLOOKUP argument is self-contained.

**Cost**: Merkle proof overhead inside the circuit (~20 hash evaluations per cross-segment memory access); segment boundary boot/shutdown overhead; intermediate state must be serialized to memory at segment boundaries.

**Why it works**: RISC-V has a flat memory model — the entire state can be compactly committed as a Merkle root.

### SP1: Cross-Table Lookups via Cumulative Sums (Evolved Across Versions)

SP1 has separate chips that communicate via cross-table lookups, and these lookups span shards. Their approach has evolved significantly:

**SP1 V1–V3 (logUp-based)**:
- Each chip interaction generates a partial logUp sum: `sum_i (multiplicity_i / (r + rlc(query_i; gamma)))`
- Each shard exposes its partial `cumulative_sum` as a public value
- During recursive aggregation, cumulative sums from all shards are checked to sum to zero globally
- **Problem**: logUp requires verifier randomness (Fiat-Shamir), creating a synchronization barrier — must complete all commitments before computing the randomness

**SP1 Turbo V4 (EC multiset hash for memory)**:
- Replaced logUp for **memory consistency only** with elliptic curve-based multiset hashing
- EC multiset hash doesn't need verifier randomness — runs on-the-fly during execution
- Eliminated the Fiat-Shamir synchronization barrier for memory
- **Non-memory cross-table lookups** (CPU ↔ precompile chips) still used logUp with cumulative sums

**SP1 Hypercube V6 (LogUp-GKR)**:
- Switched to a **multilinear polynomial** proof system
- Cross-table interactions now use **LogUp-GKR protocol**: constructs virtual multilinear polynomials for the interaction and uses GKR (sumcheck-based) to verify `sum(m(x)/(rho - t(x))) == sum(1/(rho - a(x)))` over the boolean hypercube
- Uses **Jagged PCS** for variable-sized tables ("pay only for what you use")
- The GKR-based approach is more efficient for the multilinear setting and handles cross-table interactions natively

### Linea Limitless: Shared Randomness + Partial Log-Derivative Sums

Linea's approach is designed for the EVM's multi-module architecture where cross-module lookups are fundamental:

1. **Bootstrap**: Run the full protocol once on the trace to get the witness
2. **Segment**: Split by module (Hub, Keccak, etc.) into GL and LPP segments, and by row bands within each module
3. **Prove GL segments** in parallel — each produces a segment proof
4. **Derive shared randomness** from GL proofs via MiMC multiset hash (`GetSharedRandomnessFromSegmentProofs`)
5. **Prove LPP segments** in parallel using the shared randomness (gamma/alpha for the log-derivative argument)
6. Each segment computes its **local log-derivative sum** (a partial contribution) and exposes it as a public input
7. **Hierarchical conglomeration** pairs segment proofs 2-by-2, adding log-derivative sums (`accLogDeriv += child.LogDerivativeSum`), multiplying grand products (`accGrandProduct *= child.GrandProduct`), adding Horner sums
8. **Outer circuit** (`execution-limitless`) checks at the root: `LogDerivativeSum == 0`, `GrandProduct == 1`, `HornerSum == 0`, plus VK Merkle root, segment counts, multiset hash consistency

**Key detail**: When a `ColumnSegmenter` is provided (limitless mode), the "sum must be zero" check is skipped at the segment level — each segment's sum is only a partial contribution.

---

## 5. Parallelism

| | RISC Zero | SP1 (Turbo+) | Linea Limitless |
|---|---|---|---|
| **Segment proving** | All segments in parallel — no inter-segment dependency | All shards in parallel — EC multiset hash runs on-the-fly (no barrier for memory); LogUp-GKR for chip interactions | GL segments in parallel, then LPP segments in parallel (two phases) |
| **Barrier** | None between segment proofs | Depends on version; Turbo eliminated barrier for memory | GL → shared randomness → LPP |
| **Why barrier exists/doesn't** | All constraints are local; randomness is local per segment | Memory: EC hash needs no randomness; Cross-table: LogUp-GKR uses Fiat-Shamir but within shard scope | Log-derivative argument needs global randomness derived from GL commitments |
| **Aggregation** | Recursive STARK composition (tree-shaped) | Recursive shard proof aggregation | Hierarchical 2-by-2 conglomeration |

---

## 6. Tradeoffs

### Generality vs Specialization

| | RISC Zero | SP1 | Linea |
|---|---|---|---|
| **Constraint efficiency** | Generalized (RISC-V + sub-circuits) — good but not maximally tight | Specialized per chip, within RISC-V framework | Maximally specialized per EVM operation |
| **Segmentation complexity** | Simple (time slices, local lookups, Merkle chaining) | Moderate (shards with cross-table cumulative sums) | Complex (multi-module segments, shared randomness, hierarchical conglomeration) |
| **Parallelism** | Fully parallel (one phase) | Mostly parallel (memory barrier eliminated in Turbo) | Two-phase (GL then LPP) |

### Why Each Approach Works for Its VM

- **RISC Zero**: RISC-V has a flat memory model → state is compactly committable via Merkle root → lookups stay local → simple segmentation
- **SP1**: RISC-V with separate precompile chips → cross-table lookups needed → evolved from logUp to EC multiset hash (memory) + LogUp-GKR (interactions) to minimize barriers
- **Linea**: EVM with specialized arithmetic modules → cross-module lookups are intrinsic → log-derivative partial sums with shared randomness is the right tool for global set-membership over structurally segmented traces

### The Fundamental Distinction

- **RISC Zero**: Cross-segment relationships are **state continuity** (memory flows forward in time) → solved by Merkle root chaining
- **SP1**: Cross-shard relationships are both **state continuity** (memory) and **set membership** (chip interactions) → solved by EC multiset hash + LogUp-GKR
- **Linea**: Cross-segment relationships are **set membership** (rows in module A must appear in module B) → solved by shared randomness + partial log-derivative sums + hierarchical aggregation

---

## 7. Summary Table

| Aspect | RISC Zero | SP1 | Linea Limitless |
|---|---|---|---|
| Segmentation | Temporal (continuations) | Temporal (shards, ~2M cycles) | Structural (by module + row bands) |
| Precompiles | Embedded in CPU circuit | Separate STARK chips | Separate arithmetic modules |
| Cross-segment lookup | None (local only) | Cumulative sums (logUp → LogUp-GKR) | Partial log-derivative sums |
| Memory consistency | Merkle root chaining | EC multiset hash (Turbo+) | Permutation within modules |
| Parallelism barriers | None | Minimal (Turbo eliminated memory barrier) | One (GL → randomness → LPP) |
| Aggregation | Recursive STARK | Recursive shard aggregation | Hierarchical 2-by-2 conglomeration |
| Final proof | Receipt (optionally Groth16) | Compressed STARK (optionally Groth16) | gnark proof (Groth16) |
| Lookup table replication | Yes (256-byte table per segment) | No (cross-table via bus) | No (cross-module via lookups) |
| Constraint specialization | Low (generalized CPU circuit) | Medium (per-chip AIR) | High (per-module arithmetization) |
