# Fixed-width compiled system (design)

Status: design only. Nothing in this document is implemented yet.

**Depends on two changes not yet on `main`:** the runtime decoder (#3980) and,
critically, the replacement of `ExprOp.operands: []const usize` with fixed
`lhs`/`rhs` fields (`021288b52`, part of #3980). On `main` as it stands,
`ExprOp` still holds a slice, and narrowing the integer fields there changes
nothing at all — see "Why this needs the slice removal first" below. This
document is written against the post-#3980 shape.

## Why revisit the encoding

`perf(verifier-ray): decode compiled system at runtime` replaced a ~20 MB
comptime Zig value in `.rodata` with a varint blob decoded once at startup into
a `.bss` arena. Against 20 MB of `usize`-padded slices that was the right
trade: the blob is 1.33 bytes per encoded value against 8 bytes per `usize`.

Two later changes moved the ground under that decision:

- replacing `ExprOp.operands: []const usize` with fixed `lhs`/`rhs` fields
  removed the fat pointer that dominated `ExprNode`'s size
- interning the expression DAG cut 482,341 nodes to 107,173

The blob is now 815,429 B. Decoding it still costs ~32% of total verifier
cycles, and that cost is now large relative to what the encoding saves.

## The trade, measured

The blob loader in `main.zkc` copies every byte of guest RAM through `write_8`,
a read-modify-write on word-addressed RAM. Its cost was measured by running the
`-Ddecode-only` guest twice against proof images differing only by 2 MB of
trailing padding:

| blob | bytecode steps |
|---|---:|
| 20,199,348 B | 2,507,184,308 |
| 22,296,500 B | 2,548,603,060 |
| +2,097,152 B | +41,418,752 |

**19.75 bytecode steps per byte**, about 0.58 RISC-V cycles per byte.

Encoding density across the whole blob (613,360 values):

| | bytes/value | blob |
|---|---:|---:|
| varint | 1.33 | 815,429 B |
| fixed u16 | 2.00 | ~1,226,720 B |

So moving to fixed width costs ~411 KB of extra blob, and the loader charges
~0.58 cycles/byte for it:

| | cycles |
|---|---:|
| extra blob loading | ~239,000 |
| decode phase removed | ~185,000,000 |

The encoding is optimizing the cheaper axis by roughly two orders of magnitude.

## What fixed width buys

A fixed-width, alignment-stable record can be `@ptrCast` straight out of
`.rodata`. There is no decode pass and no arena:

- `system_decode` disappears — currently the largest single phase
- the `.bss` arena disappears (8.15 MB today, generated as 10x the blob)
- startup cost becomes the blob copy alone

Node width falls out of the union's widest arm. With `ExprOp` already reduced
to two fixed fields, that arm is `ScalarRef`:

| | usize | u16 |
|---|---:|---:|
| `ExprNode` | 32 B | 8 B |
| `ExprOp` | 24 B | 6 B |
| `ScalarRef` | 16 B | 4 B |

Measured with `@sizeOf` on both shapes. The union's alignment drops from 8 to
4, which is what collapses 32 B to 8 B.

## Why this needs the slice removal first

This only works after `ExprOp`'s operand slice is gone. Building both shapes at
`3e8a1da37` — the commit before the runtime decoder, and the shape `main` still
has — gives:

| | usize | u16 |
|---|---:|---:|
| `ExprNode` | 32 B | **32 B** |
| `ExprOp` | 24 B | 24 B |
| `ScalarRef` | 16 B | 4 B |

`ScalarRef` shrinks, but `[]const u16` is still a 16-byte fat pointer, so
`ExprOp` is unchanged and remains the union's widest arm. The node stays 32 B
and nothing is saved. An earlier attempt to narrow these types found exactly
this and was abandoned.

## Value ranges

Measured on the current interned system:

| field | count | max | fits |
|---|---:|---:|---|
| `lhs` / `rhs` | 177,900 | 11,109 | u16 |
| `column_claim` | 14,462 | 1,608 | u16 |
| `coin_value` | 430 | 110 | u16 |
| `constant` | 827 | 2,130,706,432 | u32 (stays `field.Element`) |

u8 is not viable: 68.4% of node indices exceed 255.

u16 caps a module at 65,535 nodes. The largest module is 11,109 nodes after
interning (58,797 before), so there is 5.9x headroom, but codegen must assert
the bound rather than truncate silently.

## Work required

This is not a type change. `@ptrCast`ing a record out of `.rodata` requires:

1. **Layout guarantees.** Zig's `union(enum)` has no defined layout. The node
   needs `extern union` with an explicit tag, or a `packed struct`, and the
   decision affects how the evaluator branches.

2. **Slices become offset + length.** `.rodata` addresses are not known at
   codegen time, so every slice in the bundle — `Bucket`, `Vanishing`,
   `ColumnDesc.shifts`, `claim_cells` — must become a pair of indices into a
   flat array rather than a pointer. This is the bulk of the work.

3. **Codegen emits fixed records.** `WriteCompiledSystemBinary` currently emits
   varints; it would emit fixed-width little-endian records plus the flat
   arrays, and assert the u16 bound.

4. **Endianness and alignment.** The guest is RV64 little-endian, matching the
   host that generates the blob, but the cast needs an explicit alignment
   guarantee at the embed site.

## Risks

- **Sub-word loads.** Reading a `u16` on RV64 is `lhu` plus masking where a
  `usize` is a single `ld`. `evalExpr` is the hottest loop in the verifier, so
  some of the decode saving returns as evaluation cost. Unquantified.
- **`@ptrCast` may not be safe** for the chosen representation without
  `extern`/`packed`, and those constrain what the union can hold.
- The ~185M decode figure is from a q=1 `-Ddecode-only` run at 482,341 nodes.
  Post-interning it is lower; the ratio remains lopsided but the absolute
  saving is smaller than that number suggests.
