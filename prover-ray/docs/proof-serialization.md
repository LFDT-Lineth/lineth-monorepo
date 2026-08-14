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
| cells | `map[ObjectID]field.Gen`, keyed globally | `rounds[r].cells`, round-major, dense |
| columns | not carried | `rounds[r].columns` |
| module sizes | `map[int]int` | `module_sizes []usize`, canonical dynamic-module order |
| PCS claims | inside `Cells` | `pcs_opening.entry_claims [][]Ext`, canonical entry order |
| FRI proof | `*fri.OpeningProof` | `pcs_opening.proof` |

So the encoder's job is: **project** `wiop.Proof` onto the verifier's
round-major dense shape, then emit that shape's exact memory image. The Go maps
disappear during projection, which is why they were never an obstacle.

That projection already exists and is already tested: it is what
`verifier-ray/testdata/generate` does today when it emits `verify.zig` golden
vectors (`writeVerifyProof`, `pcsOpeningZigLiteral`). It currently renders the
projected proof as **Zig source text**; this work makes it render **bytes**
instead. Reuse that code path rather than writing a second projection — two
projections that must agree is a bug factory.

Note what the projection *drops*, because the verifier does not consume it:

- `fri.Branch.AuxSiblings` — the Zig `merkle.Branch` has no such field.
- `fri.QueryLayer` is `[]Branch` in Go but only `layer[0]` is used; the Zig side
  is one `Branch` per fold round.

The image must not carry these. Anything in the image that the verifier does not
read is a soundness liability, not just wasted space.

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

Consequence: the format is largely not ours to choose. It is the Zig ABI of
`verifier.Proof` for the guest target, and §5 is measured from the compiler
rather than designed. §4 is the one part we do get to choose.

## 4. Governing principle: comptime knowledge never goes on the wire

Anything the compiled system already knows must not be carried by the proof.
This is the one design rule that is ours, and it decides the format's shape.

Applying it removes **every tagged union** from the image, which is worth doing
for three independent reasons: it is smaller, it is faster (no branch per cell),
and it deletes the entire "Zig union layout is unspecified" hazard class (§5.2)
along with a soundness gap (§4.2).

### 4.1 Cells carry no base/ext tag

Today a cell is `field.Gen` in Go and `value.Scalar` in Zig — both a 24-byte
extension value plus a 1-byte discriminant. That discriminant is a property of
the *column the cell opens*, i.e. a compile-time property of the system, not a
per-proof value. It belongs in the system description.

**Decision: cells are bare `ext.Ext` on the wire (24 B, no tag).** Whether a
cell is treated as a base or extension value — which is only observable in the
Fiat-Shamir absorption width — comes from the comptime system description on
the verifier side.

This requires a prerequisite change in wiop: `System` must declare each cell's
field kind, and `Runtime.AdvanceRound` must take the absorption width from that
declaration instead of from `Gen.isBase`. That is a transcript change and needs
to land before or with the encoder.

### 4.2 Why that prerequisite is not optional

The current arrangement is not merely redundant, it is a soundness gap, and the
serialization work is what forces the issue.

`AdvanceRound` absorbs every cell with `fs.UpdateGeneric`, which branches on
`Gen.IsBase()` (`crypto/koalabear/fiatshamir/poseidon2.go:52-58`): a base cell
absorbs **1** field element, an extension cell absorbs **6**. `Proof.Cells` is
prover-supplied, and the verifier replays absorption from exactly those values.

So for a base-valued cell the prover can flip the tag to `ext` without changing
the numeric value — `Gen` stores a lifted `Ext` either way — and obtain a
*different* transcript, hence different coins, for a semantically identical
proof. That is roughly `2^(#base cells)` free coin-grinding attempts at no
witness cost. No verifier check constrains a cell's tag (`RangeCheck.Check` does
check `IsBase`, but on columns, and it is compiled away).

Moving the tag into the comptime system closes this: the width becomes a
property of the circuit, not of the proof. Had we kept the tag on the wire, the
format would have been obliged to preserve it bit-exactly and the gap would have
been locked in.

### 4.3 The same argument applies to the other two unions

- `protocol.ColumnMessage` (`oracle_commitment` | `public_column`): whether a
  column is committed or public is a declaration-time property. The round
  message should carry two separate dense arrays — commitments and public
  columns — with the comptime spec giving the order.
- `value.Vector` (`base` | `ext`) for public columns: same reasoning as cells.
  Tag-free needs two arrays (`[]const []const Element` and `[]const []const Ext`)
  because the element width differs, which is slightly awkward. Public columns
  are few and small, so this is the low-value case; if it complicates
  verifier-ray more than it is worth, keeping this one tag is defensible.

Priority: cells first (they are the volume, and the soundness gap is there),
`ColumnMessage` second (free), `Vector` last (optional).

### 4.4 The one possibly-remaining tag

`?merkle.RowPair` in `InputTreeOpening.leaves` marks which tree levels have a
materialised row pair. That pattern is derived from the reconstructed layout
(`columns` + `module_sizes`), so the verifier can compute it rather than be told
it — in which case the field becomes a dense array of present pairs and the
image is entirely tag-free. **Needs confirmation from verifier-ray** that
presence is fully determined by the reconstruction and never prover-chosen. If
it is prover-chosen, that is itself worth a look, for the same reason as §4.2.

## 5. Target ABI

Guest target is `riscv64-freestanding-none`, `generic_rv64+m`
(`riscv-guests/build_common/build.zig: standardGuestTarget`) — **rv64, not
rv32**. So `usize` is 8 bytes on the guest, same as x86_64/aarch64 hosts.

Measured with Zig 0.16.0: **every size, alignment and field offset below is
byte-identical between `aarch64` and `riscv64`.** One image serves the R5 guest
and the native mmap smoke-test path.

- Endianness: little, all targets.
- Field elements are `koalabear.Element` = one `u32` in **Montgomery form**. The
  image stores the internal representation verbatim; both sides already compute
  in Montgomery form, so no conversion happens at any point. Worth stating
  because it means the image is *not* a canonical-integer encoding and is not
  meant to be read by anything that isn't koalabear-aware.
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

**This breaks the native mmap path as currently written.** `loadNativeInput`
does `mmap(null, ...)`, so the kernel picks the address and the baked-in
pointers are wrong. Options:

1. Native path maps at a fixed address:
   `mmap(0x08800000, len, PROT_READ, MAP_FIXED|MAP_PRIVATE, fd, 0)`. One image,
   one format, both paths. **Recommended.**
2. Emit a second image relocated for the host's chosen base. Encoding is cheap
   so this is nearly free, but "the proof file" stops being a single artifact,
   which is awkward for fixtures and for anything that stores proofs.
3. Base-relative offsets resolved on every dereference. Portable, but pays a
   register add per pointer in the guest, i.e. gives up the whole point.

Recommend (1), keeping the `base` parameter so (2) stays available for tests.

### 5.3 Measured layout

Sizes below are as measured **today**, i.e. with the §4 unions still present.
Adopting §4 removes the union rows and shrinks `RoundMessage`'s element types.

| Type | Size | Align | Notes |
|---|---|---|---|
| `[]const T` (slice) | 16 | 8 | `ptr` @0 (8B), `len` @8 (element count) |
| `base.Element` | 4 | 4 | one Montgomery `u32` |
| `ext.Ext` (E6) | 24 | 4 | `B0` @0, `B1` @8, `B2` @16; each E2 = `a0,a1` |
| `poseidon2.Digest` / `Commitment` | 32 | 4 | `[8]Element` |
| `protocol.RoundMessage` | 32 | 8 | `columns` @0, `cells` @16 |
| `merkle.RowOpening` | 32 | 8 | `base` @0, `ext` @16 |
| `merkle.RowPair` = `[2]RowOpening` | 64 | 8 | `[0]` @0, `[1]` @32 |
| `merkle.Branch` | 48 | 8 | **`siblings` @0, `leaf` @16** — see §6 |
| `merkle.InputTreeOpening` | 32 | 8 | `siblings` @0, `leaves` @16 |
| `fri.Proof` | 48 | 8 | `round_roots` @0, `final_poly` @16, `running_queries` @32 |
| `pcs.OpeningProof` | 64 | 8 | `input_queries` @0, `fri_proof` @16 |
| `verifier.PcsOpening` | 80 | 8 | `entry_claims` @0, `proof` @16 |
| `verifier.Proof` | 112 | 8 | `rounds` @0, `module_sizes` @16, `pcs_opening` @32 |

Unions and optionals, for reference until §4 removes them. Zig gives **no ABI
guarantee** for these; the offsets are measured facts about one compiler
version, not language guarantees — a further argument for §4.

| Type | Size | Align | Payload | Tag | Tag values |
|---|---|---|---|---|---|
| `value.Scalar` | 28 | 4 | @0, 24B | `u8` @24, 3B pad | `0` = base, `1` = ext |
| `value.Vector` | 24 | 8 | @0, 16B | `u8` @16, 7B pad | `0` = base, `1` = ext |
| `protocol.ColumnMessage` | 40 | 8 | @0, 32B | `u8` @32, 7B pad | `0` = oracle, `1` = public |
| `?merkle.RowPair` | 72 | 8 | @0, 64B | `u8` @64, 7B pad | `0` = null, `1` = present |

The root `verifier.Proof` occupies image offsets `[0, 112)`, because the loader
casts the base address itself.

## 6. Field ordering: working assumption and one live exception

**Working assumption: struct fields are laid out in declaration order.** We take
verifier-ray's declarations at face value for now and do not build machinery to
police it. When the two sides are actually wired together, verifier-ray pins the
ordering — ideally by making the proof-facing types `extern struct`, which turns
the assumption into an ABI guarantee and makes hand-written Go offsets safe.

One exception to be aware of while that is pending: **`merkle.Branch` does not
currently follow declaration order.** It declares `leaf` then `siblings`, but
Zig's `auto` layout puts `siblings` at offset 0 and `leaf` at 16. So an encoder
written purely on the declaration-order assumption produces a valid-looking,
completely wrong image for this one type today.

That needs no machinery to handle — just take the offsets in §5.3 as the source
of truth for the encoder, keep them in one place, and fold them into the pinning
conversation at integration. `extern struct` would make `Branch` match its
declaration and let the special case disappear.

## 7. Maps and the comptime boundary

Checked, because a hash map genuinely cannot carry baked-in absolute pointers
and would break the whole approach: **there are no maps in the Zig verifier.**
No `HashMap`, `AutoHashMap` or `StringHashMap` anywhere in `src/`.

The things named "map" are `pcs.System.witness_map` and `quotient_map`, both
`[]const pcs.ClaimRef` where:

```zig
pub const ClaimRef = struct { col_decl_idx: usize, shift: usize };
```

Flat arrays of index pairs, 16 bytes each, resolved by direct indexing. Nothing
associative, no key hashing, no pointer chasing — they relocate exactly like
every other slice in the image, so had they been in the proof they would have
been no problem at all.

They are not in the proof, though. They live in `pcs.System`, part of
`verifier.Systems`, which is **comptime**:

```zig
const verifier_case = comptime embedded_data.get(embedded_data_conf.spec_index);
```

`spec`/`systems` are baked into `.rodata` and the linker resolves their pointers;
only the proof is a runtime value. So the compiled system is outside this
format's scope, and no serialization of it is needed for the proof image to work.

Worth knowing for the future: the system is **structurally** comptime, not just
conveniently so. `vanishing.verify` takes `comptime system: System` and does
`inline for (system.modules)` with comptime-specialised `static_n`
(`src/query/vanishing.zig:86-99`), and `powModuleSize` folds size-derived terms
at comptime. Turning the system into runtime data would therefore not be a
serialization exercise — it would mean giving up that specialisation and paying
for it in guest cycles.

That trade only comes up if we want one guest binary to verify many different
circuits, which is a separate architectural decision. It is not on the path for
this branch.

## 8. Encoder

### 8.1 Algorithm

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

### 8.2 Determinism rules

The image must be a pure function of the proof and the base — byte-identical
across runs, machines and Go versions. So:

- **Zero every padding byte.** Zig leaves padding undefined; our probe saw real
  stack garbage in those positions. Zeroing makes the image hashable and
  diffable, which the tests depend on. (§4 removes most padding by removing the
  unions.)
- **Empty slices need a non-null pointer.** Zig's `[]const T` holds a
  non-optional `[*]const T`; a null pointer is UB even at length 0. The probe
  showed Zig itself uses a small aligned dummy (`0x4`) for empty slice literals.
  Rule: emit `ptr = base` (valid, aligned, never dereferenced) and `len = 0`.
  Do not emit `ptr = 0`.
- Go map iteration order must never reach the image — the projection already
  imposes round-major / canonical order, which is what makes this hold.

## 9. Validation and the trust boundary

The image is **untrusted input**. A zero-decode format has no parse step, so it
also has no natural place to reject a malformed proof — every structural
guarantee that parsing normally provides has to be re-established explicitly.
This is the main risk the format introduces.

- Whoever writes the image into the guest's IN region (host, not guest) must
  bound-check it: total length ≤ `LENGTH(IN)`, every pointer inside
  `[base, base+len)`, every `ptr + count*sizeof(elem)` inside the image, every
  `len` sane, alignment respected, no pointer into the root header.
- The guest cannot cheaply re-validate pointers, so the honest framing is: the
  guest trusts the *shape* of its input region because the host validated it,
  and trusts none of the *values*. Value-level checks stay where they are
  (`fri.checkOpeningProofShape`, the verifier's own count checks).
- Consequence for review: this format moves shape-validation from the decoder to
  the writer. If a proof ever arrives from an untrusted peer as a raw image
  rather than being re-encoded locally, that validator is the only thing between
  the guest and arbitrary out-of-bounds reads. Prefer a design where the host
  always re-encodes from a `wiop.Proof` it produced or verified, and the raw
  image is never an ingress format.

A `Validate(image, base) error` pass belongs in the same package as the encoder
and should be mandatory on any path that did not just produce the image itself.

## 10. Size model

Per-node overhead versus a packed encoding, after adopting §4:

| Node | Image | Packed | Overhead |
|---|---|---|---|
| cell (as `Ext`) | 24 | 24 | **1×** |
| cell, if declared base | 24 | 4 | 6× — see below |
| any slice | +16 header | +≈4 length | 4× on headers |
| `RowPair` (dense, §4.4) | 64 | 64 | 1× |
| oracle commitment | 32 | 32 | 1× |

Adopting §4 is what makes this table boring, which is the point: with tags gone
the image is within a slice-header constant of a packed encoding. Before §4,
cells cost 28 B against a packed 4 B — 7× — and were the dominant term.

The one remaining question is cells declared base. §4.1 stores every cell as a
24-byte `Ext`; a base-declared cell only needs 4. Since the kind is comptime
known, a tag-free split (`base_cells: []const Element`, `ext_cells: []const Ext`)
would recover that with no discriminant, at the cost of two arrays per round.
Whether that is worth it depends on the base/ext split in the real fixture,
which Phase 0 measures.

Total image size:

```
112                                    root
+ 32·R                                 rounds
+ Σ_r (32·O_r + 24·L_r)                per-round commitments and cells
+ Σ public columns (4 or 24)·size      public column data
+ 8·M                                  module_sizes
+ 16·E + 24·Σ_e shifts_e               entry_claims
+ 16·Q + 32·Σ_q T_q                    input_queries
+ Σ openings (32·D + 64·D + row data)  input-tree openings
+ 32·(NR-1) + 24·2^logFinalPolySize    round_roots, final_poly
+ 16·Q + 48·Q·(NR-1) + 32·Σ siblings   running_queries
```

(R rounds, O_r/L_r oracle columns/cells per round, M dynamic modules, E claim
entries, Q FRI queries, T_q input trees per query, D tree depth, NR FRI rounds.)

## 11. Open questions for review

1. **wiop cell field-kind declaration** (§4.1) — the prerequisite. Where does
   the kind live on `System`, and who owns the `AdvanceRound` transcript change?
2. **verifier-ray changes** (§4.3, §4.4, §5.2, §6) — dropping the unions,
   confirming `?RowPair` presence is derivable, `MAP_FIXED` on the native path,
   and pinning field order via `extern struct`. All need their agreement; worth
   one conversation rather than four.
3. **Package location** — encoder in prover-ray (`wiop/proofimage`?) or in
   verifier-ray's `codegen` module next to the projection it reuses? The
   projection lives in verifier-ray today; `backend.SerializeProof` lives in
   prover-ray and needs to call it. Leaning: format + encoder in prover-ray,
   with the projection lifted out of `testdata/generate` into a shared spot.
4. **Public inputs** — `wiop.PublicInput` (`[]field.Gen`) has no counterpart in
   `verifier.Proof`; today its values reach the verifier as cells in the round
   messages. Confirm the image carries no separate public-input section and that
   `backend.Result.PublicInputs` stays the separate coordinator-facing struct it
   is now.
5. **Versioning** — the root must be at offset 0 for the cast, so there is no
   room for a magic/version header in the image. Carry the format version out of
   band (alongside the image in `Result`, or as a build-time constant asserted on
   both sides)? Without it, a layout change is undetectable at runtime.
6. **Does the coordinator get this image, or a compact encoding?**
   `backend.SerializeProof` feeds the coordinator's `proof` field. Sending a
   guest-relocated memory image over the wire is odd. §4 shrinks the overhead
   enough that one format may now be fine — decide explicitly rather than by
   default.

## 12. Implementation plan

- **Phase 0 — measure.** Build the projected proof for the R5 fixture and count
  R, O_r, L_r, the base/ext cell split, Q, D, NR. Settles §10's one open
  question (base-cell split) before any format is committed to.
- **Phase 1 — prerequisite.** Cell field kind onto `wiop.System`; `AdvanceRound`
  takes absorption width from the declaration rather than `Gen.isBase` (§4.1),
  closing §4.2. Transcript change, so it lands with its own tests and a
  verifier-ray counterpart.
- **Phase 2 — projection.** Lift the `wiop.Proof` → verifier-shape projection
  out of `verifier-ray/testdata/generate` into a shared, tested package. No
  behaviour change: the existing Zig-literal generator becomes its first
  consumer, keeping the golden vectors as a regression net.
- **Phase 3 — encoder.** Post-order arena writer (§8) + `Validate` (§9).
  Round-trip test via a Go reader over the image, plus a cross-language golden
  test asserting the Go-produced image is byte-identical to the Zig-compiled
  `verifier.Proof` for the same fixture — that validates the format end to end
  rather than validating our own assumptions about it.
- **Phase 4 — wire it up.** `backend.SerializeProof`, the guest input-region
  writer, and remove the two `loadR5Input`/`loadNativeInput` TODOs so the
  non-embedded path works.
