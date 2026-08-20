# proofserialization — the proof image format

The format this package implements, and the reasoning behind it.

- `Project` maps a `wiop.Proof` onto the verifier's round-major shape.
- `Encode` lays that out as the byte image verifier-ray casts straight out of its
  input region, relocated for a given base address.
- `Decode` / `Validate` read an image back, host-side.
- `Measure` reports what an image would cost without building one.

The layout is not a free choice: it mirrors the Zig ABI of verifier-ray's
`verifier.Proof`, measured from the compiler and pinned by
`verifier-ray/src/proof_abi.zig`. Sections 5–7 are the load-bearing ones if you
are changing either side.

## 1. Goal

Produce, from a `wiop.Proof`, a single contiguous byte image that the Zig
verifier consumes with **zero decode work**: one pointer cast, no parsing, no
allocation, no fix-up pass.

Encoding cost is explicitly not a design constraint. Decoding cost is the only
thing being optimised, and the target is literally zero.

The image doubles as the guest's RAM witness: it is written into the ZkC input
region and the guest reads its proof straight out of that memory.

The image is fed **directly to verifier-ray**. It is not a coordinator wire
format — see §12.

## 2. The serializer is faithful, not clever

**The serializer takes `verifier.Proof` exactly as it is and reproduces its
in-memory representation byte-for-byte.** It makes no representation decisions:
whatever fields that struct has, whatever unions, whatever padding, the image
mirrors them.

This is the design rule that keeps the artifact honest. If the image ever
diverged from the in-memory layout — packing something, dropping something,
canonicalising something — the cast would stop being valid and we would be back
to decoding.

Corollary: **questions about what `verifier.Proof` *should* contain are out of
scope here.** Some of its content is redundant with the compiled system and
arguably belongs there instead (§11), but that is verifier-ray's design call, to
be made on their side and on their schedule. The serializer follows whatever they
land on; it must not try to lead it.

## 3. This is a projection of `wiop.Proof`, not a dump of it

The one place the encoder does more than copy bytes: `wiop.Proof` is not the
thing being serialized. The consumer's type is `verifier.Proof`, and the two are
structurally different:

| | prover-ray (`wiop.Proof`) | verifier-ray (`verifier.Proof`) |
|---|---|---|
| cells | `map[ObjectID]field.Gen`, keyed globally | `rounds[r].cells []Scalar`, round-major, dense |
| columns | not carried | `rounds[r].commitment ?Commitment` (Merkle root only) |
| module sizes | `map[int]int` | `module_sizes []usize`, canonical dynamic-module order |
| PCS claims | inside `Cells` | not carried separately — the verifier reconstructs them from `rounds[*].cells` at verify time, via the compiled system's per-column claim-cell table |
| FRI proof | `*fri.OpeningProof` | `pcs_opening.proof` |

So the pipeline is: **project** `wiop.Proof` onto `verifier.Proof`'s shape, then
dump that shape faithfully per §2. The Go maps disappear in the projection step,
which is why they were never an obstacle to the dump.

That projection already exists and is already tested: it is what
`verifier-ray/testdata/generate` does when it emits `verify.zig` golden vectors
(`writeVerifyProof`, `pcsOpeningZigLiteral`). It renders the projected proof as
**Zig source text** today; this work makes it render **bytes**. Reuse that code
path rather than writing a second projection — two projections that must agree is
a bug factory.

Note what the projection *drops*, because the verifier's types have no field for
it:

- `fri.Branch.AuxSiblings` — the Zig `merkle.Branch` has no such field.
- `fri.QueryLayer` is `[]Branch` in Go but only `layer[0]` is used; the Zig side
  is one `Branch` per fold round.

## 4. Why the format is already decided by verifier-ray

`verifier-ray/src/main.zig` already casts a raw address to the proof type, on
both paths:

```zig
fn loadR5Input() *const verifier.Proof {
    // TODO: we have kept the compatibility with the old way of loading input,
    // but we don't have serialization so it will fail if the input is not embedded.
    return @ptrCast(@alignCast(&_in_start));
}
```

The zero-decode design is not a proposal — it is the existing contract with the
producer side missing. This branch fills exactly that hole. Everything in §6 is
measured from the compiler rather than designed.

## 5. Target ABI

Guest target is `riscv64-freestanding-none`, `generic_rv64+m`
(`riscv-guests/build_common/build.zig: standardGuestTarget`) — **rv64, not
rv32**. So `usize` is 8 bytes on the guest, same as x86_64/aarch64 hosts.

Measured with Zig 0.16.0: **every size, alignment and field offset in §6 is
byte-identical between `aarch64` and `riscv64`.** One image serves the R5 guest
and the native mmap smoke-test path.

- Endianness: little, all targets.
- Field elements are `koalabear.Element` = one `u32` holding the **canonical
  (non-Montgomery) representative**. The prover's internal `field.Element` is
  Montgomery-encoded (gnark-crypto's koalabear package), while Zig's
  `field.Element` stores and computes with the canonical representative
  throughout (plain modular arithmetic, no REDC) — so every value MUST be
  reduced via `field.Element.Bits()` (or `.Uint64()`) before it is written into
  this image. Writing the internal Montgomery representation verbatim is a bug:
  addition/subtraction happen to be representation-transparent so a Montgomery
  mismatch is invisible there, but multiplication is not, so it silently
  corrupts the very first Poseidon2 absorption of a nontrivial wire value (see
  `elementFrom` in `types.go`).
- Maximum alignment anywhere in the graph is 8.

### 5.1 Base address

`riscv-guests/build_common/linker_script.ld`:

```
IN (r) : ORIGIN = 0x08800000, LENGTH = 0x40000000
_in_start = ORIGIN(IN);
```

The guest base is the fixed constant `0x08800000`, 2^23-aligned (satisfies
alignment 8 with room to spare), region 1 GiB. The encoder takes the base as a
parameter, defaults to this value, and must reject images exceeding the region.

### 5.2 Relocation: absolute pointers, baked in at encode time

Pointers in the image are **absolute guest addresses**, computed as
`base + offset_in_image` at encode time. The guest dereferences native pointers
with no arithmetic and no indirection — this is what makes decode free rather
than merely cheap.

The cost is that an image is valid only at the base it was relocated for.

**This breaks the native mmap path as currently written.** `loadNativeInput` does
`mmap(null, ...)`, so the kernel picks the address and the baked-in pointers are
wrong. Options:

1. Native path maps at a fixed address:
   `mmap(0x08800000, len, PROT_READ, MAP_FIXED|MAP_PRIVATE, fd, 0)`. One image,
   one format, both paths. **Recommended.**
2. Emit a second image relocated for the host's chosen base. Encoding is cheap so
   this is nearly free, but "the proof file" stops being a single artifact.
3. Base-relative offsets resolved on every dereference. Portable, but pays a
   register add per pointer in the guest, i.e. gives up the whole point.

Recommend (1), keeping the `base` parameter so (2) stays available for tests.

## 6. Measured layout

| Type | Size | Align | Notes |
|---|---|---|---|
| `[]const T` (slice) | 16 | 8 | `ptr` @0 (8B), `len` @8 (element count) — **no capacity field** |
| `base.Element` | 4 | 4 | one canonical (non-Montgomery) `u32` |
| `ext.Ext` (E6) | 24 | 4 | `B0` @0, `B1` @8, `B2` @16; each E2 = `a0,a1` |
| `poseidon2.Digest` / `Commitment` | 32 | 4 | `[8]Element` |
| `protocol.RoundMessage` | 56 | 8 | `cells` @0, `commitment` @16 |
| `merkle.RowOpening` | 32 | 8 | `base` @0, `ext` @16 |
| `merkle.RowPair` = `[2]RowOpening` | 64 | 8 | `[0]` @0, `[1]` @32 |
| `merkle.Branch` | 48 | 8 | **`siblings` @0, `leaf` @16** — see §7 |
| `merkle.InputTreeOpening` | 32 | 8 | `siblings` @0, `leaves` @16 |
| `fri.Proof` | 48 | 8 | `round_roots` @0, `final_poly` @16, `running_queries` @32 |
| `pcs.OpeningProof` | 64 | 8 | `input_queries` @0, `fri_proof` @16 |
| `verifier.PcsOpening` | 64 | 8 | `proof` @0 (its only field — no `entry_claims`; the verifier reconstructs those from `rounds[*].cells` instead) |
| `verifier.Proof` | 96 | 8 | `rounds` @0, `module_sizes` @16, `pcs_opening` @32 |

Unions and optionals. Zig gives **no ABI guarantee** for these — the offsets are
measured facts about one compiler version, not language guarantees. Per §2 we
serialize them as they are; §7 covers keeping the numbers honest.

| Type | Size | Align | Payload | Tag | Tag values |
|---|---|---|---|---|---|
| `value.Scalar` | 28 | 4 | @0, 24B | `u8` @24, 3B pad | `0` = base, `1` = ext |
| `?protocol.Commitment` | 36 | 4 | @0, 32B | `u8` @32, 3B pad | `0` = absent, `1` = present |
| `?merkle.RowPair` | 72 | 8 | @0, 64B | `u8` @64, 7B pad | `0` = null, `1` = present |

The root `verifier.Proof` occupies image offsets `[0, 96)`, because the loader
casts the base address itself.

## 7. Field ordering is pinned and machine-checked

Assuming declaration order was not safe: **`merkle.Branch` does not follow it.**
It declares `leaf` then `siblings`, but Zig's `auto` layout puts `siblings` at
offset 0 and `leaf` at 16. An encoder written from the declarations alone
produces a valid-looking, completely wrong image for that type — and because a
wrong image still casts cleanly, it would surface as an unrelated verification
failure rather than as a layout bug. Hence pinning up front, not at integration.

`extern struct` cannot do the pinning. Zig rejects slices in extern structs:

```
error: extern structs cannot contain fields of type '[]const u32'
note: slices have no guaranteed in-memory representation
```

That note is worth sitting with: the language explicitly declines to guarantee
the very representation this format is built on. So the strongest thing
available for the types as they stand is an *asserted* layout, not a guaranteed
one.

**Landed** (verifier-ray, additive, no use-site changes):

- `src/proof_abi.zig` — comptime assertions on every proof type's size,
  alignment, field offsets, and each union's discriminant *values*. Failures name
  the actual versus pinned number. Exported from `lib.zig` so the checks are
  analyzed on every build that uses the library, including the guest.
- `test/proof_abi_test.zig` — the parts `@offsetOf` cannot express: each
  discriminant's byte offset, and that an empty slice's pointer is non-null.
  Registered in `test/all.zig`.

They **hold on the rv64 guest target** (`zig build -Dr5=true` passes), so §6's
numbers are machine-checked on the target that matters rather than just observed
by a probe.

The failure messages are written for whoever trips them, who will not have this
format in mind and did not write the assertion. Each one states what moved, why
that happens (Zig's align-descending sort), what it would break downstream (the
cast still succeeds — the verifier silently reads misplaced bytes and fails
somewhere unrelated), and what to do, including the type's current field layout
with alignments and offsets. Every message lists the other places that must change
in the same commit: the pin, prover-ray's encoder, and §6's table.

All four failure paths were exercised rather than assumed:

| Simulated change | What fires | What it tells you |
|---|---|---|
| field added to `Branch` | `@sizeOf` is 56, pinned 48 | "a field was added"; prints the resulting layout |
| `RowOpening`'s two fields swapped | `@offsetOf("base")` is 16, pinned 0 | equal alignments ⇒ memory order *is* declaration order, so put them back |
| union variant renumbered | discriminant pin | append new variants rather than inserting |
| discriminant byte moves | runtime test | dumps the raw bytes so the real offset is visible |

One subtlety that needed fixing: for a struct whose fields share an alignment, a
re-derived "recommended" order can only echo what is declared now — which, when
that declaration is the thing that broke, is advice to cement the bug. The
messages therefore report the **current** layout, labelled as current, and branch
their advice on whether alignments are mixed (declare align-descending) or equal
(a plain reorder to undo).

### 7.1 How stable is the ordering, really?

Zig's rule, measured on 0.16 and identical on aarch64 and riscv64: **fields are
stable-sorted by alignment, descending.** Equal alignments keep declaration
order; align-8 fields precede align-4, which precede align-1.

That is a deterministic and unsurprising rule (minimise padding), not arbitrary
compiler whim, and it explains everything in §6:

- Eight of the nine proof structs are made entirely of slices — uniformly align
  8 — so declaration order already *is* memory order for them.
- `merkle.Branch` was the sole exception, because it mixes an align-8 slice with
  an align-4 `[8]Element`.

**So the ordering can be made structurally stable rather than merely probable:
declare fields in descending alignment order and there is nothing left for the
compiler to reorder.** Verified directly — a struct declared `{slice, [8]u32}`
and one declared `{[8]u32, slice}` produce byte-identical layouts, so the
align-descending declaration is the one that tells the truth.

`Branch` has been reordered accordingly (`siblings` then `leaf`) and the
convention is documented in `proof_abi.zig`. No ABI change — the layout was
already that; only the source now agrees with it. Declaration order now equals
memory order across the entire proof graph.

Two caveats worth keeping in view:

- **No version guarantee.** Zig documents `auto` layout as unspecified and
  reserves the right to change it. The current heuristic is the natural one and a
  change would be a notable compiler event, but it is not promised, and this is
  not something we can verify ahead of time.
- **The likelier risk is us, not Zig.** Adding or reordering a field in
  verifier-ray is an ordinary code change and shifts offsets immediately —
  considerably more probable than a Zig codegen change. §7's assertions catch both
  cases identically, which is the real argument for having them regardless of how
  stable the layout rule turns out to be.

### 7.2 Remaining follow-up

- **Explicit extern fat pointer**, if a language-level guarantee is wanted:
  `Slice(T) = extern struct { ptr: [*]const T, len: usize }` measures 16 B /
  align 8 — byte-identical to a native slice — and extern layout follows
  declaration order, so every proof type could become `extern struct`. Measured
  cost: ~65 field references in `src/`, ~114 in hand-written tests, and 2789 in
  `testdata/generated/` that are free because the Go emitter regenerates them.
  The tagged unions and `?RowPair` would additionally need explicit
  tag-plus-union representations. Worth folding in if these types are touched
  anyway.

## 8. Layout: inline payloads, depth-first

A Zig slice is **two** words — `{ptr, len}`, no capacity field, unlike Go's
`{ptr, len, cap}` triplet. So a slice header is 16 bytes and its payload can be
laid out immediately after it.

The encoder walks the object graph depth-first and appends each slice's payload
directly behind the structure that references it. For a **leaf slice** — one
whose elements are scalars (`Element`, `Ext`, `Digest`, `usize`) — this is
exactly the local rule: `ptr = base + self_offset + 16`, payload follows in
place, no bookkeeping at all. That covers nearly every byte in the image.

The rule needs the bump pointer rather than `self + 16` in two cases:

- A struct with more than one field. The root's `rounds` header sits at `[0,16)`,
  but offset 16 is `module_sizes`, not the rounds payload — so the payload goes
  after the whole 112-byte root.
- A slice whose elements themselves contain slices (`[]RoundMessage`,
  `[]InputTreeOpening`, `[]const []const Branch`). The element array must be
  contiguous for the guest to index it by stride, so the elements' own payloads
  land after the array, not interleaved with it.

The general rule that covers both: **a payload goes at the current bump pointer,
and payloads are appended depth-first after their containing structure is
written.** Pointer values are then patched once per header, or avoided entirely
by emitting children before parents. Either is trivial since encode cost is free;
depth-first-with-patching is preferable because it gives the guest better
locality — a structure and the data it points at end up adjacent.

The root is the one fixed constraint: it must occupy `[0, 112)`, so reserve it up
front and fill it last.

## 9. Determinism: match Zig's own representation

The image must be a pure function of the proof and the base — byte-identical
across runs, machines and Go versions. Where Zig's representation is a free
choice, **we copy what Zig does** rather than inventing a convention, since the
whole point is that the bytes are indistinguishable from an in-memory value.

- **Zero every padding byte.** Zig leaves padding undefined and our probe saw
  real stack garbage in those positions. Zeroing is what makes the image
  hashable and diffable, which the tests depend on.
- **Empty slices carry a non-null pointer.** Zig's `[]const T` holds a
  non-optional `[*]const T`, so null is UB even at length 0. Empirically Zig
  emits a small aligned dummy (`0x4`) for empty slice literals. Match that
  behaviour — the encoder must never emit `ptr = 0`. Exact value to be confirmed
  against whatever Zig does for the types in question rather than assumed
  uniform.
- Go map iteration order must never reach the image. The projection (§3) already
  imposes round-major / canonical order, which is what makes this hold.

## 10. Validation and the trust boundary

The image is **untrusted input**. A zero-decode format has no parse step, so it
also has no natural place to reject a malformed proof — every structural
guarantee that parsing normally provides has to be re-established explicitly.
This is the main risk the format introduces.

- Whoever writes the image into the guest's IN region (host, not guest) must
  bound-check it: total length ≤ `LENGTH(IN)`, every pointer inside
  `[base, base+len)`, every `ptr + count*sizeof(elem)` inside the image, every
  `len` sane, alignment respected, no pointer into the root header.
- The guest cannot cheaply re-validate pointers, so the honest framing is: the
  guest trusts the *shape* of its input region because the host validated it, and
  trusts none of the *values*. Value-level checks stay where they are
  (`fri.checkOpeningProofShape`, the verifier's own count checks).
- Consequence for review: this format moves shape-validation from the decoder to
  the writer. If a proof ever arrives from an untrusted peer as a raw image
  rather than being re-encoded locally, that validator is the only thing between
  the guest and arbitrary out-of-bounds reads. Prefer a design where the host
  always re-encodes from a `wiop.Proof` it produced or verified, and the raw
  image is never an ingress format.

A `Validate(image, base) error` pass belongs alongside the encoder and should be
mandatory on any path that did not just produce the image itself.

## 11. Measured size (Phase 0)

Size is descriptive, not a lever: per §2 the encoder has no packing decisions to
make. The point of measuring was to know the artifact's size, and to settle
whether the overhead the format trades away is acceptable.

`proofserialization.Measure(sys, proof, pub)` computes an image's size and
composition without encoding anything. It works on any `wiop.Proof`, so nothing
here depends on ZKC or the arithmetization: `go test ./wiop/proofserialization/`
measures PCS-compiled proofs built from wiop primitives alone.

The table below came from applying it to four ZKC testdata programs, as a one-off
run to get realistic magnitudes. Each proof went through the full pipeline
including `pcs.Compile` and was verified before being measured.

| program | image | payload | overhead | cells | row data |
|---|---|---|---|---|---|
| `modexp_u64` | 9.13 MiB | 7.86 MiB | 13.9% (1.16×) | 2185 (0.6%) | 70.3% |
| `modexp_u256` | 42.51 MiB | 41.19 MiB | **3.1% (1.03×)** | 16044 (1.0%) | 92.8% |
| `secp256k1_add_u256` | 39.43 MiB | 38.12 MiB | **3.3% (1.03×)** | 12410 (0.8%) | 92.4% |
| `secp256k1_scalarmul_u256` | 34.99 MiB | 33.69 MiB | **3.7% (1.04×)** | 10191 (0.8%) | 91.6% |

"Payload" is the irreducible content: field elements and Merkle digests.
"Overhead" is everything the format adds — slice headers, union tags, presence
flags, padding, struct headers.

### 11.0 Correction: overhead is circuit-dependent

An earlier draft claimed the structural overhead was program-independent, on the
strength of it being 1.33–1.39 MB across all four programs above. **That was
wrong, and it was overfitting to four similar circuits.**

Overhead is driven by the number of `?RowPair` slots, which is
`queries × input_trees × tree_depth`. The FRI query count is fixed at 229, but
trees and depth are properties of the committed layout, so they move with the
circuit. All four programs above happened to have 4 trees and depth 17, which is
what made it look constant.

Measured across the 29 `wioptest` circuits (`TestImageShapeIsCircuitDependent`),
leaf slots per query ranges **6 to 12** — a 2× spread — and overhead runs **54%
to 72%** of the image on those small circuits, against 3–4% on the larger
programs above. The ratio improves as row data grows, but nothing about it is
constant.

The size model in §11.5 is the thing to trust; the "it's constant" shortcut is
withdrawn.

### 11.1 What the numbers settle

**The space tradeoff is a non-issue at production circuit sizes.** For the larger
programs the image costs **3–4% over a packed encoding**, so zero-cost decoding
is bought for a rounding error. On small circuits the ratio is much worse (54–72%)
because the fixed FRI structure dominates a tiny payload — but a proof that small
is not worth optimising.

**The model is validated against the encoder, not just asserted.**
`TestMeasureAgreesWithEncode` compares `Measure`'s arithmetic against
`len(Encode(...))` on real proofs; they now agree **byte for byte, with zero
padding**. That comparison immediately found a bug in the model: it counted a
192-byte "pcs_opening header" for `PcsOpening` + `OpeningProof` + `fri.Proof`,
all three of which are stored *inline* in their parent and were therefore already
inside the root's 112 bytes. Every figure this document quoted was inflated by
that fixed 192 bytes — negligible against 43 MB, but the model was unchecked
until the encoder existed to check it against.

**Opened row data is 91–93% of the image** and is pure field elements. Nothing
about the format touches it.

**Cells are 0.6–1.0%, and every cell is `ext` — zero base cells in all four
programs.** So §4's base-vs-ext cell packing question, which earlier drafts spent
the most words on, was worth under 1% of the image and has no instances anyway.
Removing the `Scalar` tag would save `28 → 24`, i.e. 0.1% of the image. Not worth
a transcript change on space grounds; the §11.3 soundness argument stands on its
own.

**`?RowPair`'s presence flag is the one overhead worth naming**: 1.12 MB fixed,
of which the flag-plus-padding is 125 KB and null slots are 148–577 KB depending
on the program. §4.4's question — can the verifier derive presence from the
reconstructed layout? — is worth roughly 1% of the image. Worth asking, not worth
blocking on.

**Fits the guest comfortably.** At 43 MiB the largest measured image uses 4% of
the 1 GiB `IN` region (§5.1).

**Reinforces that the coordinator must not receive this** (§12). A 10–43 MB
memory image is fine as a RAM witness and wrong as a network payload.

### 11.2 Structural constants

Identical across all four programs, since they follow from the FRI parameters
rather than the circuit: 229 queries, 4 input trees per query, opening depth 17,
16 FRI rounds (15 round roots), 15 layers per running query, 3435 branches,
30,915 branch sibling digests, 1 final-poly coefficient. Only `row data` and
`cells` scale with the program.

Also confirmed: **all 30,915 `AuxSiblings` slots are nil**, so the Zig
`merkle.Branch` having no such field (§3) drops nothing. Had any been non-nil it
would have meant the two verifiers reconstruct different roots — checked
explicitly rather than assumed, since the projection silently discards the field.

### 11.3 Still not counted

Public columns are absent from `wiop.Proof`, so their size is a Phase 1 number.
It is small relative to row data, but the total above is a lower bound until it
is added.

(`entry_claims` used to be listed here too, when it was still a proposed
separate field the projection would derive. It never shipped as a serialized
field: the claimed evaluations are ordinary `LagrangeEval.EvaluationClaims`
cells, already counted once as part of `cells` above, and the verifier
reconstructs its canonical-entry-order view of them from those same round
cells at verify time instead of trusting a second, separately-serialized
copy.)

### 11.3a Why 43 MiB, and why that is a prover question

43 MiB is a startling number for a FRI proof — a recursion proof would normally be
around 1 MB — so it is worth showing where it comes from. It is not the format:
**the image is 97% payload**, so a perfectly packed encoding of the same proof
would also be ~41 MB. The proof itself is that size.

Decomposing `modexp_u256`, whose 41.3 MB of row data is 93% of the image:

```
2,960,512 base elements x 4 B  = 11.8 MB
1,229,272 ext  elements x 24 B = 29.5 MB
```

which is `229 queries x 4 input trees x 17 levels x 2 conjugate rows x ~140
values per row`. Three multipliers stack up:

- **229 queries.** `FRILogInverseRate = 1`, i.e. a blow-up factor of 2. At rate
  1/2 you need ~229 queries for 128 bits (per the soundcalc reference in
  `pcs.go`). Systems quoting ~1 MB proofs typically use blow-up 4–8 and 30–80
  queries — a 3–7× difference on its own.
- **Multi-size openings.** Each query opens one row *per committed size* per tree,
  so the per-query cost is the total committed column count across all sizes, not
  one row.
- **Row width.** ~140 values per opened row, and extension values cost 24 B each.

So proof size ≈ `queries × 2 × (total committed columns) × element size`. All
three factors are prover-side parameters or circuit properties; none is
serialization. **If ~1 MB is the target, the lever is the FRI rate and query
count, and how much is committed — not this format.** Worth a separate
conversation; flagged here only because measuring the image is what surfaced it.

One caveat on the 43 MiB itself: it is `secp256k1`/`modexp` rather than the real
zkEVM circuit. Treat it as the right order of magnitude for these circuits, not
as a zkEVM proof size. (An earlier draft of this caveat also excluded
`entry_claims` from the count; that field never shipped, so there is nothing
excluded on that account any more — see §11.3.)

### 11.4 Why no R5 number, and why it does not matter

The R5 arithmetization's absolute proof size is unmeasured, and the format does
not depend on knowing it.

What the format needed from Phase 0 was the *overhead*, and overhead turns out to
be program-independent: §11.2's structure is fixed by the FRI parameters, so the
1.33–1.39 MB is the same for any circuit. A bigger circuit adds row data, which
is pure payload, so the overhead fraction only falls. R5 being much larger than
these programs means its overhead is *below* 3%, and no measurement is needed to
conclude that.

If someone does want R5's size later, `Measure` takes any `wiop.Proof` — call it
wherever an R5 proof already exists. (Note that `arithmetization/.../main.zkc`
does not currently compile against the pinned `zkc` version; every R5 benchmark
in `zkcdriver/r5_benchmark_test.go` fails identically, so that is a pre-existing
arithmetization/`zkc` sync issue and unrelated to serialization.)

## 11.5 Observations that are not ours to act on

Recorded because someone should know, then deliberately **left alone** — both are
verifier-ray/wiop design questions, not serialization ones:

- **Some proof content is redundant with the compiled system.** A cell's
  base/ext kind, a column's oracle-vs-public kind, and possibly `?RowPair`
  presence are all properties the system already knows, yet they ride along as
  union discriminants in the proof. Moving them into the system description would
  shrink the image and delete every tagged union from it — including the
  unspecified-layout concern in §6/§7. Entirely verifier-ray's call.
- **A cell's declared field kind is not checked against its assigned value.**
  `Cell.IsExtension()` is fixed at construction from the expression's static type
  (`query_lagrange_eval.go` builds claim cells with `pv.IsExtension()`), so the
  kind is a compile-time property of the system. But `Runtime.AssignCell` stores
  the supplied `field.Gen` verbatim without comparing its tag against
  `cell.IsExtension()`, and `AdvanceRound` then absorbs on that tag
  (`fs.UpdateGeneric`: 1 element for base, 6 for ext).

  So this is a **missing validation of a statically-known property**, not a hole
  in the design — the protocol knows the answer and simply does not assert it.
  Adding the check is the fix. Tracked as its own issue; out of scope here, and
  the image faithfully carries whatever tag it is given either way.

## 12. Answers settled in review

- **Package location** — `wiop/proofserialization`, tentative. Expected to
  relocate into verifier-ray later; keeping it in wiop for now avoids blocking on
  that.
- **Public inputs** — verifier-ray has no public-input support yet
  (`verifier.Proof` has no such field), so the image carries none. When they add
  it we match their representation. Not this branch.
- **Versioning** — no version header. A zero-decode image implies exactly one
  layout per verifier build, so there is nothing to branch on at runtime: a
  second version would mean a second proof struct. Assume the image matches the
  verifier it is fed to, and re-encode if it ever does not.
- **Coordinator** — the coordinator does **not** receive this image. It goes
  straight into verifier-ray. Whatever `backend.SerializeProof` eventually sends
  the coordinator is a separate format and a separate piece of work; this branch
  should not wire it.
- **Field-order pinning** — done now rather than deferred, because without it the
  encoder is untestable in any meaningful sense: see §7 for what landed and why
  `extern struct` was not available.
- **Union/tag design** — still for verifier-ray to decide; Alexandre is asking
  them about their plans. Nothing here blocks on the answer: §2 means the
  serializer follows whatever they have at the time, and §7's assertions make any
  change to it a loud build failure rather than a silent wire break.

## 13. Implementation plan

- **Done — pin the ABI.** §7. Prerequisite for everything below: without it the
  encoder's target is unverifiable.
- **Done — Phase 0, measure.** §11, via `proofserialization.Measure`. Image is
  9–43 MiB at 3–4% structural overhead, 91–93% opened row data, cells under 1%
  and all `ext`. Nothing in the numbers argues against the format.
- **Done — the serializer.** `wiop/proofserialization`:
  - `layout.go` — the ABI mirror: sizes, field offsets, discriminant values.
  - `types.go` — Go mirrors of the verifier's proof types, plus the
    `field.Ext`/`field.Gen` conversions the projection will use.
  - `encode.go` — `Encode(proof, base)`, depth-first inline layout (§8) with
    absolute pointers baked in.
  - `decode.go` — `Decode`/`Validate` (§10), a strict host-side reader.

  Tested: value and image round-trip, determinism, root at offset 0, relocation
  across bases, tag polarity, zeroed padding, and rejection of null, below-base,
  past-end, over-length and misaligned pointers plus out-of-range discriminants.

  Two things the implementation settled that this spec had wrong:

  - **`base` must be non-zero.** §8.2's rule of pointing empty slices at `base`
    makes them null when `base == 0`, breaking the very invariant it exists to
    uphold. `Encode` and `Decode` now reject base 0: an in-image pointer would be
    indistinguishable from null, so base 0 is incompatible with the format rather
    than merely awkward.
  - **nil and empty slices are indistinguishable**, deliberately. Go separates
    them; Zig's `[]const T` is just `{ptr, len}`. Both encode identically and
    decode as nil, so the round trip is a Go-value identity only up to that, and
    an exact identity on the *image* — which is what the guest reads.

  **Mutation-tested, because passing tests are not evidence of powerful ones.**
  52 deliberate defects were injected — wrong offsets, inverted tags, dropped
  coordinates, removed bounds checks, perturbed layout constants — and 51 were
  caught. The single survivor is an unreachable invariant assertion (`alloc` on
  an empty buffer provably returns 0), which no test can exercise by
  construction.

  Two genuine blind spots only showed up this way, both since closed:

  - **The ABI cross-check was vacuous for discriminants.** It compared Zig's
    pinned values against *hardcoded copies of those values* rather than the Go
    constants, so changing `TagColumnPublic` to 2 left every test green. It now
    references the constants, and byte-level assertions on the discriminants back
    it up.
  - **`GuestBase`, `MaxImageSize` and the `field` conversions were untested.**
    Every other test relocates relative to whatever `GuestBase` happens to be, so
    changing it kept them all green while making every image point at the wrong
    region; `ExtFrom` could drop a coordinate unnoticed. `GuestBase` and
    `MaxImageSize` are now checked against `ORIGIN(IN)`/`LENGTH(IN)` in the guest
    linker script, and the conversions against distinct per-coordinate values.

  Worth noting what mutation testing exposed about round-trip tests generally:
  they cannot see a *symmetric* error, because encode and decode share the
  constants. Changing an offset in `layout.go` keeps them in perfect agreement
  while making the image disagree with the verifier. Only the ABI cross-check and
  the literal byte assertions have power there — which is the argument for
  keeping both, and for the cross-language golden test still outstanding.

  `abi_agreement_test.go` closes the drift direction nothing else covered:
  `proof_abi.zig` catches Zig's layout moving, and the encoder's own tests catch
  Go bugs against Go's constants, but neither notices the two sides' *numbers*
  diverging. It parses `proof_abi.zig` and compares every pinned size, offset and
  discriminant against `layout.go`, reporting which constant disagrees and what it
  would corrupt. Verified to fire by perturbing a pin.

- **Done — the projection.** `project.go`: `Project(sys, proof, pub)`
  turns a `wiop.Proof` into the verifier's shape. The Go maps disappear here,
  which is why they were never an obstacle to the dump. Cells go out round-major
  in declaration order *including public inputs*, since the verifier absorbs them
  in that order and omitting them would desynchronise the transcript replay.

  `TestProjectEncodeDecode_EndToEnd` closes the loop on real proofs:
  prove → project → encode → decode → re-encode, over all 29 scenarios.

  **`entry_claims` is gone, not merely derived.** An earlier draft of this
  section described it as "still a parameter, not derived" and called closing
  that seam the next piece of work. It turned out the right resolution was not
  to derive and serialize a second copy of those claims at all: they are
  ordinary `LagrangeEval.EvaluationClaims` cells, already carried once in the
  projected round messages, and verifier-ray's verifier reconstructs its own
  canonical-entry-order view of them directly from those round cells at verify
  time (via a codegen-emitted per-column claim-cell table). So `PcsOpening` no
  longer has an `EntryClaims` field, `Project` takes no `entryClaims` parameter,
  and there is no second ordering left to keep in agreement with
  verifier-ray's PCS codegen.
- **Done — the cross-language check.** Earlier drafts called this blocked on
  §5.2's `MAP_FIXED` decision. That was wrong: a *test* only needs an address the
  test process can map, not the production one. Measured on arm64 macOS,
  `MAP_FIXED` fails at `0x08800000`, `0x30000000` and `0x100000000` but succeeds
  at `0x400000000`, so the fixture image is relocated there.

  `wiop/proofserialization/abi_agreement_test.go` writes
  `verifier-ray/testdata/proof_image.bin` (856 B) and fails if it goes stale;
  `verifier-ray/test/proof_image_test.zig` maps it and casts it to a real
  `verifier.Proof` — mmap, cast, read, with no Zig-side parsing — then asserts
  every variant: both `Scalar` discriminants, both `Vector` discriminants, both
  a present and an absent round commitment, a null and a present `?RowPair`, an
  empty round, and `merkle.Branch`'s reordered fields. (It used to also assert a
  jagged `entry_claims` with an empty inner slice; that field is gone, and the
  values it exercised are exercised instead by the ordinary round-cell
  assertions the same test already makes.)

  This is the only place a byte written by Go is interpreted by the actual Zig
  type. Everything else is indirect: the pins assert Zig's layout, prover-ray
  asserts its encoder against its own copy of the pins, and prover-ray's round
  trip cannot see a symmetric error because its encoder and decoder share the
  constants. Verified to have power by corrupting a byte of the image and
  confirming the Zig test fails. It skips, rather than passing, when `MAP_FIXED`
  is unavailable.

- **Next — wire it up.** Guest input-region writer; remove the
  `loadR5Input`/`loadNativeInput` TODOs so the non-embedded path works. Still
  needs §5.2's decision on the native path base, but only for production now.