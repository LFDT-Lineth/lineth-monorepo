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
| columns | not carried | `rounds[r].columns []ColumnMessage` |
| module sizes | `map[int]int` | `module_sizes []usize`, canonical dynamic-module order |
| PCS claims | inside `Cells` | `pcs_opening.entry_claims [][]Ext`, canonical entry order |
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
- Field elements are `koalabear.Element` = one `u32` in **Montgomery form**. The
  image stores the internal representation verbatim; both sides already compute
  in Montgomery form, so no conversion happens anywhere. Worth stating because it
  means the image is not a canonical-integer encoding and is not meant to be read
  by anything that isn't koalabear-aware.
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
| `base.Element` | 4 | 4 | one Montgomery `u32` |
| `ext.Ext` (E6) | 24 | 4 | `B0` @0, `B1` @8, `B2` @16; each E2 = `a0,a1` |
| `poseidon2.Digest` / `Commitment` | 32 | 4 | `[8]Element` |
| `protocol.RoundMessage` | 32 | 8 | `columns` @0, `cells` @16 |
| `merkle.RowOpening` | 32 | 8 | `base` @0, `ext` @16 |
| `merkle.RowPair` = `[2]RowOpening` | 64 | 8 | `[0]` @0, `[1]` @32 |
| `merkle.Branch` | 48 | 8 | **`siblings` @0, `leaf` @16** — see §7 |
| `merkle.InputTreeOpening` | 32 | 8 | `siblings` @0, `leaves` @16 |
| `fri.Proof` | 48 | 8 | `round_roots` @0, `final_poly` @16, `running_queries` @32 |
| `pcs.OpeningProof` | 64 | 8 | `input_queries` @0, `fri_proof` @16 |
| `verifier.PcsOpening` | 80 | 8 | `entry_claims` @0, `proof` @16 |
| `verifier.Proof` | 112 | 8 | `rounds` @0, `module_sizes` @16, `pcs_opening` @32 |

Unions and optionals. Zig gives **no ABI guarantee** for these — the offsets are
measured facts about one compiler version, not language guarantees. Per §2 we
serialize them as they are; §7 covers keeping the numbers honest.

| Type | Size | Align | Payload | Tag | Tag values |
|---|---|---|---|---|---|
| `value.Scalar` | 28 | 4 | @0, 24B | `u8` @24, 3B pad | `0` = base, `1` = ext |
| `value.Vector` | 24 | 8 | @0, 16B | `u8` @16, 7B pad | `0` = base, `1` = ext |
| `protocol.ColumnMessage` | 40 | 8 | @0, 32B | `u8` @32, 7B pad | `0` = oracle, `1` = public |
| `?merkle.RowPair` | 72 | 8 | @0, 64B | `u8` @64, 7B pad | `0` = null, `1` = present |

The root `verifier.Proof` occupies image offsets `[0, 112)`, because the loader
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

Verified two ways: the assertions **fire** (deliberately breaking a pinned offset
fails the build with `proof ABI drift: @offsetOf(...) is 0, pinned at 8`), and
they **hold on the rv64 guest target** (`zig build -Dr5=true` passes), so §6's
numbers are now machine-checked on the target that matters rather than just
observed by a probe.

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

## 11. Size model, and observations that are not ours to act on

Size is descriptive here, not a lever: per §2 the encoder has no packing
decisions to make. The model exists so we know how big the artifact is.

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

Phase 0 fills these in from the real R5 fixture. There is nothing to decide from
the numbers, but it is worth knowing the artifact's size before shipping it.

Two things noticed while measuring, recorded here because someone should know and
then deliberately **left alone** — both are verifier-ray/wiop design questions,
not serialization ones:

- **Some proof content is redundant with the compiled system.** A cell's
  base/ext kind, a column's oracle-vs-public kind, and possibly `?RowPair`
  presence are all properties the system already knows, yet they ride along as
  union discriminants in the proof. Moving them into the system description would
  shrink the image and delete every tagged union from it — including the
  unspecified-layout concern in §6/§7. Entirely verifier-ray's call.
- **`Gen.isBase` is prover-supplied and transcript-affecting.**
  `Runtime.AdvanceRound` absorbs cells via `fs.UpdateGeneric`, which branches on
  the tag (`crypto/koalabear/fiatshamir/poseidon2.go:52-58`): base absorbs 1
  field element, ext absorbs 6. Since `Proof.Cells` comes from the prover and the
  verifier replays absorption from those values, flipping a base cell's tag
  changes the coins without changing any claimed value — free coin grinding at no
  witness cost. This is a pre-existing wiop soundness gap, unrelated to
  serialization except that the image will faithfully carry the tag. Tracked
  separately; it needs its own branch and a transcript change.

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
- **Phase 0 — measure.** Build the projected proof for the R5 fixture, fill in
  §11's counts, and record the artifact size.
- **Phase 1 — projection.** Lift the `wiop.Proof` → `verifier.Proof` projection
  out of `verifier-ray/testdata/generate` into a shared, tested package. No
  behaviour change: the existing Zig-literal generator becomes its first
  consumer, keeping the golden vectors as a regression net.
- **Phase 2 — encoder.** Depth-first inline writer (§8) + `Validate` (§10) in
  `wiop/proofserialization`. Round-trip test via a Go reader over the image, plus
  a cross-language golden test asserting the Go-produced image is byte-identical
  to the Zig-compiled `verifier.Proof` for the same fixture — that validates the
  format end to end rather than validating our own assumptions about it.
- **Phase 3 — wire it up.** Guest input-region writer; remove the
  `loadR5Input`/`loadNativeInput` TODOs so the non-embedded path works. Needs
  §5.2's decision on the native path base.