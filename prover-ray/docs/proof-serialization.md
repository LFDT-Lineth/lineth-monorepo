# Proof serialization — the proof image format

Status: **draft for review**. No code yet; this pins the format so the encoder
can be written against a fixed target.

Branch: `feat/prover-ray-proof-serde`.

## 1. Goal

Produce, from a `wiop.Proof`, a single contiguous byte image that the Zig
verifier consumes with **zero decode work**: one pointer cast, no parsing, no
allocation, no fix-up pass.

Encoding cost is explicitly not a design constraint. Decoding cost is the only
thing being optimised, and the target is literally zero.

The image doubles as the guest's RAM witness: it is written into the ZkC input
region and the guest reads its proof straight out of that memory.

Space efficiency is deliberately traded away (see §9 for how much).

## 2. The key insight: this is a projection, not a dump

`wiop.Proof` cannot be the thing we dump, and the consumer's proof type is not
`wiop.Proof`. They are structurally different types:

| | prover-ray (`wiop.Proof`) | verifier-ray (`verifier.Proof`) |
|---|---|---|
| cells | `map[ObjectID]field.Gen`, keyed globally | `rounds[r].cells []Scalar`, round-major, dense |
| columns | not carried | `rounds[r].columns []ColumnMessage` |
| module sizes | `map[int]int` | `module_sizes []usize`, canonical dynamic-module order |
| PCS claims | inside `Cells` | `pcs_opening.entry_claims [][]Ext`, canonical entry order |
| FRI proof | `*fri.OpeningProof` | `pcs_opening.proof` |

So the encoder's job is: **project** `wiop.Proof` onto the verifier's
round-major dense shape, then emit that shape's exact memory image. The maps
disappear during projection, which is why they were never an obstacle.

That projection already exists and is already tested: it is what
`verifier-ray/testdata/generate` does today when it emits `verify.zig` golden
vectors (`writeVerifyProof`, `pcsOpeningZigLiteral`). It currently renders the
projected proof as **Zig source text**; this work makes it render **bytes**
instead. Reuse that code path rather than writing a second projection —
two projections that must agree is a bug factory.

Note what the projection *drops*, because the verifier does not consume it:

- `fri.Branch.AuxSiblings` — the Zig `merkle.Branch` has no such field.
- `fri.QueryLayer` is `[]Branch` in Go but only `layer[0]` is used; the Zig
  side is one `Branch` per fold round.

The image must not carry these. Anything in the image that the verifier does
not read is a soundness liability, not just wasted space.

## 3. Why the format is "already decided" by verifier-ray

`verifier-ray/src/main.zig` already casts a raw address to the proof type, on
both paths:

```zig
fn loadR5Input() *const verifier.Proof {
    // TODO: we have kept the compatibility with the old way of loading input,
    // but we don't have serialization so it will fail if the input is not embedded.
    return @ptrCast(@alignCast(&_in_start));
}
```

The zero-decode design is therefore not a proposal — it is the existing
contract, with the producer side missing. This branch fills exactly that hole.

Consequence: **the format is not ours to choose freely. It is the Zig ABI of
`verifier.Proof` for the guest target.** Everything in §5 is measured from the
compiler, not designed.

## 4. Target ABI

Guest target is `riscv64-freestanding-none`, `generic_rv64+m`
(`riscv-guests/build_common/build.zig: standardGuestTarget`) — **rv64, not
rv32**. So `usize` is 8 bytes on the guest, same as x86_64/aarch64 hosts.

Measured with Zig 0.16.0: **every size, alignment and field offset below is
byte-identical between `aarch64` and `riscv64`.** One image serves the R5 guest
and the native mmap smoke-test path.

- Endianness: little, all targets.
- Field elements are `koalabear.Element` = one `u32` in **Montgomery form**.
  The image stores the internal representation verbatim; both sides already
  compute in Montgomery form, so no conversion happens at any point. Worth
  stating explicitly because it means the image is *not* a canonical-integer
  encoding and is not meant to be read by anything that isn't koalabear-aware.
- Maximum alignment anywhere in the graph is 8.

### 4.1 Base address

`riscv-guests/build_common/linker_script.ld`:

```
IN (r) : ORIGIN = 0x08800000, LENGTH = 0x40000000
_in_start = ORIGIN(IN);
```

So the guest base is the fixed constant `0x08800000`, which is 2^23-aligned
(satisfies the alignment-8 requirement with room to spare), and the region is
1 GiB. The encoder takes the base as a parameter and defaults to this value;
it must reject images exceeding the region length.

### 4.2 Relocation: absolute pointers, baked in at encode time

Pointers in the image are **absolute guest addresses**, computed as
`base + offset_in_image` at encode time. The guest then dereferences native
pointers with no arithmetic and no indirection — this is what makes decode
free, rather than merely cheap.

The cost is that an image is valid only at the base it was relocated for.

**This breaks the native mmap path as currently written.** `loadNativeInput`
does `mmap(null, ...)`, so the kernel picks the address and the baked-in
pointers are wrong. Options:

1. Native path maps at a fixed address:
   `mmap(0x08800000, len, PROT_READ, MAP_FIXED|MAP_PRIVATE, fd, 0)`. One image,
   one format, both paths. **Recommended.**
2. Emit a second image relocated for the host's chosen base. Encoding is cheap,
   so this is nearly free — but it means "the proof file" is not a single
   artifact, which is awkward for fixtures and for anything that stores proofs.
3. Base-relative offsets resolved on every dereference. Portable, but pays a
   register add per pointer in the guest, i.e. gives up the whole point.

Recommend (1), keeping the `base` parameter so (2) remains available for tests.

## 5. Measured layout

All values from Zig 0.16.0, confirmed identical on `aarch64` and `riscv64`.

### 5.1 Primitives

| Type | Size | Align | Notes |
|---|---|---|---|
| `[]const T` (slice) | 16 | 8 | `ptr` @0 (8B), `len` @8 (8B, element count) |
| `base.Element` | 4 | 4 | one Montgomery `u32` |
| `ext.Ext` (E6) | 24 | 4 | `B0` @0, `B1` @8, `B2` @16; each E2 = `a0,a1` |
| `poseidon2.Digest` / `Commitment` | 32 | 4 | `[8]Element` |

### 5.2 Tagged unions and optionals — the unspecified part

Zig's `auto` layout gives **no ABI guarantee** for these. The offsets below are
measured facts about one compiler version, not language guarantees. §7 covers
how we stop this from silently drifting.

| Type | Size | Align | Payload | Tag | Tag values |
|---|---|---|---|---|---|
| `value.Scalar` | 28 | 4 | @0, 24B (`Ext`) | `u8` @24, 3B pad | `0` = base, `1` = ext |
| `value.Vector` | 24 | 8 | @0, 16B (slice) | `u8` @16, 7B pad | `0` = base, `1` = ext |
| `protocol.ColumnMessage` | 40 | 8 | @0, 32B | `u8` @32, 7B pad | `0` = oracle_commitment, `1` = public_column |
| `?merkle.RowPair` | 72 | 8 | @0, 64B | `u8` @64, 7B pad | `0` = null, `1` = present |

### 5.3 Structs

Zig's `auto` layout **reorders fields**. `merkle.Branch` declares `leaf` before
`siblings`, but in memory `siblings` is at offset 0 and `leaf` at 16. An encoder
that assumes declaration order produces a valid-looking, completely wrong image.
Always emit against measured offsets.

| Type | Size | Align | Fields (offset) |
|---|---|---|---|
| `protocol.RoundMessage` | 32 | 8 | `columns` @0, `cells` @16 |
| `merkle.RowOpening` | 32 | 8 | `base` @0, `ext` @16 |
| `merkle.RowPair` = `[2]RowOpening` | 64 | 8 | `[0]` @0, `[1]` @32 |
| `merkle.Branch` | 48 | 8 | **`siblings` @0, `leaf` @16** (reordered) |
| `merkle.InputTreeOpening` | 32 | 8 | `siblings` @0, `leaves` @16 |
| `fri.Proof` | 48 | 8 | `round_roots` @0, `final_poly` @16, `running_queries` @32 |
| `pcs.OpeningProof` | 64 | 8 | `input_queries` @0, `fri_proof` @16 |
| `verifier.PcsOpening` | 80 | 8 | `entry_claims` @0, `proof` @16 |
| `verifier.Proof` | 112 | 8 | `rounds` @0, `module_sizes` @16, `pcs_opening` @32 |

The root `verifier.Proof` occupies image offsets `[0, 112)`, because the loader
casts the base address itself.

## 6. Encoder

### 6.1 Algorithm

Post-order arena emission — children first, so a parent is written with its
children's addresses already known and no back-patching is needed:

```
emit(node) -> offset:
    for each child: childOff = emit(child)
    off = arena.allocAligned(sizeof(node), alignof(node))
    write node's scalar fields at their measured offsets
    write each slice header as { ptr: base + childOff, len: elemCount }
    return off
```

The single exception is the root, which must land at offset 0: reserve
`[0, 112)` up front, emit the rest, then fill the reserved header last.

### 6.2 Determinism rules

The image must be a pure function of the proof and the base — byte-identical
across runs, machines, and Go versions. So:

- **Zero every padding byte and every unused payload byte.** Zig leaves union
  padding and inactive-variant bytes undefined; our probe saw real stack garbage
  in those positions. The verifier must never read them, and the encoder must
  never emit non-zero there. Zeroing makes the image hashable and diffable,
  which the tests depend on.
- **Empty slices need a non-null pointer.** Zig's `[]const T` holds a
  non-optional `[*]const T`; a null pointer is UB even at length 0. The probe
  showed Zig itself uses a small aligned dummy (`0x4`) for empty slice literals.
  Rule: emit `ptr = base` (valid, aligned, never dereferenced) and `len = 0`.
  Do not emit `ptr = 0`.
- Go map iteration order must never reach the image — the projection already
  imposes round-major / canonical order, which is what makes this hold.

### 6.3 The `Scalar` tag: two hazards

`field.Gen` and `value.Scalar` are the same 28 bytes with the tag at offset 24,
which invites a `memcpy`. Do not do that:

**Tag polarity is inverted.** Go's `Gen.isBase` is `true` (byte `1`) for a base
element. Zig's `Scalar` tag is `0` for `.base`. A raw copy silently inverts
every cell's variant. The tag must be written as `if g.IsBase() { 0 } else { 1 }`.

**`Gen.isBase` is unexported**, so the encoder must go through `IsBase()`. Fine,
but it means the encoder cannot live on an unsafe fast path even if we wanted one.

For a base-valued `Gen`, Go's storage is already `Lift(b)` = `{b, 0, 0, 0, 0, 0}`,
and Zig's `.base` variant reads only the first 4 bytes. Copying all 24 payload
bytes is therefore both correct and canonical.

### 6.4 Soundness note: the `Scalar` tag is transcript-affecting

Flagging this because the format has to encode the bit, but the underlying issue
is in wiop and is not created by this work.

`Runtime.AdvanceRound` absorbs every cell with `fs.UpdateGeneric`, which branches
on the tag (`crypto/koalabear/fiatshamir/poseidon2.go:52-58`): a base cell
absorbs **1** field element, an extension cell absorbs **6**. `Proof.Cells` is
prover-supplied, and the verifier replays absorption from exactly those values.

So for a base-valued cell the prover can flip the tag to `ext` without changing
the numeric value — `Gen`'s `Ext` storage is identical either way — and get a
*different* Fiat-Shamir transcript, hence different coins, for a semantically
identical proof. That is roughly `2^(#base cells)` free coin-grinding attempts
at no witness cost. No verifier check constrains a cell's tag
(`RangeCheck.Check` does check `IsBase`, but on columns, and it is compiled away).

Implications for this branch:

- The tag **must** be carried on the wire bit-exactly. The image cannot
  canonicalise it, because canonicalising changes the transcript and would break
  replay against today's prover.
- The real fix belongs in wiop: either derive the tag from the cell's declared
  type in `System` rather than taking it from the proof, or always absorb 6
  elements. Either is a transcript change and needs its own branch.
- Until then the round-trip tests must assert tag preservation, so the format
  does not quietly paper over the gap.

Tracked separately from the format review.

## 7. Layout-drift guard

The whole format rests on `auto`-layout offsets that Zig does not promise to
keep stable across compiler versions, and that a verifier-ray refactor can
change by adding a field or reordering one. `Branch`'s reordering shows this is
not hypothetical.

Manual constants in the Go encoder would rot silently and produce a wrong image
that still casts cleanly — the worst possible failure mode.

Two ways to prevent it, in preference order:

1. **Generate the layout from the compiler.** A tiny Zig program in verifier-ray
   emits a JSON manifest of `@sizeOf` / `@alignOf` / `@offsetOf` for every proof
   type plus each union's tag offset and variant values, for the guest target.
   The Go encoder loads it (or has constants generated from it) and CI fails on
   any diff. Fully removes the drift class.
2. **Make the layout explicit in Zig.** Convert the proof-facing types to
   `extern struct` with explicit tag fields and sentinel-based optionals, so the
   layout becomes ABI-guaranteed and hand-written Go constants are safe. Cleaner
   contract, but it is a real change to verifier-ray's types and to code that
   pattern-matches on those unions.

Recommend (1) now — it is additive and does not touch the verifier — and treat
(2) as a possible follow-up if the manifest proves awkward.

Regardless: a cross-language golden test. `verifier-ray` already has the proof
as generated Zig literals, so we can assert that the Go-produced image and the
Zig-compiled `verifier.Proof` for the same fixture are byte-identical, which
validates the format end to end rather than validating our own assumptions.

## 8. Validation and the trust boundary

The image is **untrusted input**. A zero-decode format has no parse step, so it
also has no natural place to reject a malformed proof — every structural
guarantee that parsing would normally provide has to be re-established
explicitly. This is the main risk the format introduces.

- Whoever writes the image into the guest's IN region (host, not guest) must
  bound-check it: total length ≤ `LENGTH(IN)`, every pointer inside
  `[base, base + len)`, every `ptr + count*sizeof(elem)` inside the image, every
  `len` sane, alignment respected, no pointer into the root header.
- The guest cannot cheaply re-validate pointers, so the honest framing is: the
  guest trusts the *shape* of its input region because the host validated it,
  and trusts none of the *values*. Value-level checks stay where they are today
  (`fri.checkOpeningProofShape`, the verifier's own count checks).
- Consequence to make explicit in review: this format moves shape-validation
  from the decoder to the writer. If a proof ever arrives from an untrusted
  peer as a raw image rather than being re-encoded locally, that validator is
  the only thing standing between the guest and arbitrary out-of-bounds reads.
  Prefer a design where the host always re-encodes from a `wiop.Proof` it
  produced or verified, and the raw image is never an ingress format.

A `Validate(image, base) error` pass belongs in the same package as the encoder,
and should be mandatory on any path that did not just produce the image itself.

## 9. Size model

Per-node overhead versus a packed encoding, using the measured sizes:

| Node | Image | Packed | Overhead |
|---|---|---|---|
| base cell | 28 (`Scalar`) | 4 | **7×** |
| ext cell | 28 | 24 | 1.17× |
| any slice | +16 header | +≈4 length | 4× on headers |
| `?RowPair` present | 72 | 64 | 1.13× |
| `?RowPair` null | 72 | ≈0 | 72 B per absent level |
| oracle column | 40 (`ColumnMessage`) | 32 | 1.25× |

Total image size:

```
112                                    root
+ 32·R                                 rounds
+ Σ_r (40·C_r + 28·L_r)                per-round columns and cells
+ Σ public columns (4 or 24)·size      public column data
+ 8·M                                  module_sizes
+ 16·E + 24·Σ_e shifts_e               entry_claims
+ 16·Q + 32·Σ_q T_q                    input_queries
+ Σ openings (32·D + 72·D + row data)  input-tree openings
+ 32·(NR-1) + 24·2^logFinalPolySize    round_roots, final_poly
+ 16·Q + 48·Q·(NR-1) + 32·Σ siblings   running_queries
```

(R rounds, C_r/L_r columns/cells per round, M dynamic modules, E claim entries,
Q FRI queries, T_q input trees per query, D tree depth, NR FRI rounds.)

The dominant term is likely `28·ΣL_r` — cells, most of which are base-valued
and pay 7×. **First implementation task should be to measure this against the
real R5 fixture**, because if cells dominate the image, that is the one number
worth knowing before committing to the format. Two mitigations exist if it is
bad (split base/ext cell arrays; or the §6.4 fix, which would make the tag
unnecessary), and both are easier to adopt before the format ships than after.

## 10. Open questions for review

1. **Native path base** — adopt `MAP_FIXED` at `0x08800000` (§4.2 option 1)?
   This is a verifier-ray change and needs their agreement.
2. **Drift guard** — Zig-emitted manifest (§7 option 1) or `extern struct`
   (option 2)?
3. **Package location** — encoder in prover-ray (`wiop/proofimage`?) or in
   verifier-ray's `codegen` module next to the projection it reuses? The
   projection lives in verifier-ray today, and `backend.SerializeProof` lives in
   prover-ray and needs to call it. Leaning: format + encoder in prover-ray,
   with the projection lifted out of `testdata/generate` into a shared spot.
4. **Public inputs** — `wiop.PublicInput` (`[]field.Gen`) has no counterpart in
   `verifier.Proof`; today its values reach the verifier as cells in the round
   messages. Confirm that the image carries no separate public-input section and
   that `backend.Result.PublicInputs` stays the separate coordinator-facing
   struct it is now.
5. **Versioning** — the root must be at offset 0 for the cast, so there is no
   room for a magic/version header in the image itself. Carry the format version
   out of band (alongside the image in `Result`, or as a build-time constant
   asserted on both sides)? Without it, a layout change is undetectable at
   runtime.
6. **Does the coordinator get this image, or a compact encoding?**
   `backend.SerializeProof` feeds the coordinator's `proof` field. Sending a
   guest-relocated memory image over the wire is odd, and §9's overhead is paid
   on the network. A compact wire format for the coordinator plus the image for
   the guest may be the right split — but that is two formats, and worth
   deciding now rather than discovering later.

## 11. Implementation plan

Phase 0 is deliberately first: it can invalidate §9 and therefore the format.

- **Phase 0 — measure.** Build the projected proof for the R5 fixture, count
  R, C_r, L_r, base-vs-ext cell split, Q, D, and compute the image size from
  §9. Decide whether the cell overhead is acceptable.
- **Phase 1 — drift guard.** Zig layout-manifest emitter in verifier-ray plus a
  CI check, so Phase 2 is written against generated constants from day one.
- **Phase 2 — projection.** Lift the `wiop.Proof` → verifier-shape projection
  out of `verifier-ray/testdata/generate` into a shared, tested package. No
  behaviour change: the existing Zig-literal generator becomes its first
  consumer, which keeps the golden vectors as a regression net.
- **Phase 3 — encoder.** Post-order arena writer (§6) + `Validate` (§8).
  Round-trip test via a Go reader over the image; cross-language golden test
  against the Zig-compiled fixtures.
- **Phase 4 — wire it up.** `backend.SerializeProof`, the guest input region
  writer, and remove the two `loadR5Input`/`loadNativeInput` TODOs so the
  non-embedded path works.
