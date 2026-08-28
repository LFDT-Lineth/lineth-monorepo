# R5 zkVM Specification

This document is the entry point for the R5 zkVM specification — a hash-based,
post-quantum-friendly proof system for RISC-V execution targeting blockchain
and rollup workloads. The specification is organized into sections, each in
its own file. This index lists them, summarizes their content, and tracks
which are drafted and which are not yet started.

---

## Sections

### §1. Goals and Objectives — *Drafted*

**File:** [`section1_goals_and_objective.md`](section1_goals_and_objective.md)

### §2. Arithmetization — *Drafted*

**File:** [`section2_arithmetization.md`](section2_arithmetization.md)

### §3. Cryptographic Compilation — *Drafted*

**File:** [`section3_cryptographic_compilation.md`](section3_cryptographic_compilation.md)

### §4. Proof Composition — *Drafted*

**File:** [`section4_proof_composition.md`](section4_proof_composition.md)


### §5. Precompiles — **Not started**

*File: to be created.*

Will specify the dedicated arithmetizations used to accelerate operations
that would be prohibitively expensive through generic RISC-V instructions —
Keccak, elliptic-curve arithmetic, signature verification, and others. Both
application-layer precompiles (used by guest programs, §1.4 / §2.7) and
recursion-layer precompiles (used by Verifier Ray guests, §4.3.2) belong
here. Drafting is paused pending the team's decisions document on which
precompiles will be implemented and how they integrate with the ZkC source
and the rest of the toolchain.

---

## Forward references awaiting §5

Three drafted sections forward-reference §5 and will resolve cleanly once it
lands:

- **§1.4** Cryptographic Acceleration — names precompiles as the mechanism;
  §5 supplies the catalog and integration.
- **§2.7** Precompile Arithmetization — describes the detect → route →
  compile → inject path at the boundary level; §5 supplies the per-primitive
  detail.
- **§4.3.2** Recursion in Practice — lists Poseidon2 as a decided recursion
  precompile and folded-evaluation-quotient and global-quotient as
  candidates; §5 supplies the full recursion-side catalog.

---

## Conventions

The drafted sections share the following conventions; new sections should
follow them.

- **Math notation.** Inline math is delimited by `$...$`; display math by
  `$$...$$`. Multi-letter named operators use `\mathrm{}` (e.g.
  `\mathrm{Num}`, `\mathrm{Den}`, `\mathrm{root}`). Field notation uses
  `\mathbb{F}`.
- **Code identifiers.** Backticks for code-level identifiers like
  `wiop.System`, `interpreter()`, `air.Schema`, `Oracle`. Framework names
  (Wizard-IOP, Arcane, Vortex, Poseidon2, FRI) are plain or bold prose,
  not code.
- **Cross-references.** §N.M for subsection references; §N for whole
  sections. Cross-references are not hyperlinks — they're textual anchors
  the reader uses with the table of contents.
- **TODO callouts.** Inline TODOs are written as `> *[TODO: ...]*`
  blockquotes. Each section ends with an "Open items" list summarizing
  what is genuinely unsettled.
- **Diagrams.** New diagrams use Mermaid (`flowchart TD` / `flowchart BT` /
  `flowchart LR` blocks). The pre-existing architecture diagram in §1.6 is
  Mermaid; §2.6 has dropped its earlier diagram in favor of prose per
  go-corset's request that the internal IR structure not be pinned down
  yet.
- **Provisional choices.** Where a choice is provisional (a hash function,
  a parameter, an option among design alternatives), it is named once with
  a clearly-flagged provisional caveat so that a future global rename is
  mechanical.

---

## Open items overview

Each section ends with its own open-items list. As of this draft, the most
notable still-open design questions, grouped by section:

**§3 (Cryptographic Compilation)**

- Proximity-gap bound choice — 2020/2023 vs 2025 at the Johnson radius
  (§3.4.3).
- Concrete parameters $\rho$ (Reed–Solomon rate) and $Q$ (query count)
  against the target security level of §1.2.
- Hash function — Poseidon2 is provisional pending the Ethereum Foundation
  hash competition picture (§3.4.4 / §3.5).
- Grinding budget — concrete number of grinded bits $b$ (§3.5.3).
- Formal round-by-round Fiat–Shamir soundness statement over
  $\mathbb{F}_{p^6}$.

**§4 (Proof Composition)**

- Bus / lookup annotation in ZkC source — confirmation with go-corset team
  pending (§4.1.2).
- Deterministic shard allocation — confirmation as a go-corset design
  commitment pending (§4.1.6).
- Full shard-allocation policy — pending implementation (§4.1.6).
- Shard-index modeling — to be written back to go-corset team (§4.1.7).
- Multiset hash construction — concrete choice depends on Verifier Ray
  verification cost (§4.2.2).
- Shard count binding cycle — under design (§4.2.6).
- Recursion-side precompile catalog — to be filled in once §5 lands
  (§4.3.2).

**§5 (Precompiles)** — entire section pending.

---

## Document status

| Section | Status |
| --- | --- |
| §1. Goals and Objectives | Drafted |
| §2. Arithmetization | Drafted |
| §3. Cryptographic Compilation | Drafted |
| §4. Proof Composition | Drafted |
| §5. Precompiles | **Not started** |