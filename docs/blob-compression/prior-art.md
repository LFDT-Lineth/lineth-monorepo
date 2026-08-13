# What other rollups use for DA compression

Survey with receipts, gathered 2026-08-12. Claims marked **[source]** were
verified by reading the repository directly; the rest come from published
specifications or documentation and are linked.

## Summary

| project | scheme | must prove decompression? | how verified |
|---|---|---|---|
| **Scroll** | **zstd, inside a zkVM** | yes (validity) | **[source]** |
| Arbitrum | brotli | fraud proof only | docs |
| OP Stack / Base | brotli since Fjord; zlib before | fault proof only | spec |
| Taiko | zlib | yes (SGX / SP1 / RISC0) | **[source]** |
| zkSync Era | bespoke state-diff compression | yes (validity) | **[source]** |
| Linea | LZSS, hand-arithmetized | yes (validity) | this repo |

## The distinction that matters

Optimistic and validity rollups face different constraints and are not
interchangeable as precedent. Arbitrum and the OP Stack only decompress inside a
*dispute*, so per-byte proving cost barely constrains them and they can afford
brotli's context modelling. Citing "Base uses brotli" as precedent for Linea
would be a category error.

The genuine peers are Scroll, Taiko and zkSync Era, all of which must prove
decompression on the happy path.

## Scroll — the closest precedent

Scroll is a validity rollup that must prove decompression, chose zstd, and runs
the decoder in a zkVM. That is precisely the path this evaluation is
considering.

- [`scroll-tech/rust-zstd-decompressor`](https://github.com/scroll-tech/rust-zstd-decompressor)
  — description: "A light-weight decompressor for zstd compressed data used by
  scroll's DA blob". README opens "# zstd decompressor for zkvm" and the build
  instructions are `cargo openvm build`, i.e. the
  [OpenVM](https://github.com/openvm-org/openvm) zkVM. Last pushed 2025-08-20.
- The package is named `vm-zstd`. Its dependencies are `bitstream-io`,
  `itertools`, `strum`, `serde` — **a pure-Rust reimplementation, not bindings
  to the C library.** `zstd-encoder` appears only as an optional feature and a
  dev-dependency, pulled from `scroll-tech/da-codec`.
- The encoder lives separately in
  [`scroll-tech/da-codec`](https://github.com/scroll-tech/da-codec) (Go), with
  `libzstd/encoder-standard` and `libzstd/encoder-legacy`.

Two things worth taking from this. They split encoder and decoder across
repositories, mirroring the profile-pinning / decoder-implementation split in
our own plan. And they judged a **reimplementation** cheaper than porting the C
reference — a live question for us, since compiling zstd's decode-only sources
with `zig cc` is the alternative.

## Arbitrum — brotli

- [Data availability](https://docs.arbitrum.io/how-arbitrum-works/data-availability),
  [sequencer deep dive](https://docs.arbitrum.io/how-arbitrum-works/deep-dives/sequencer):
  batches are brotli-compressed, with the compression level tuned dynamically
  against the backlog of batches waiting to post — speed is prioritised when the
  backlog is deep.
- [Nitro v3.10.0](https://github.com/OffchainLabs/nitro/releases/tag/v3.10.0)
  added `--node.batch-poster.compression-levels`, a JSON array keyed to backlog
  thresholds, with validation that levels do not increase at higher thresholds.

## OP Stack and Base — brotli since Fjord

- [Fjord derivation spec](https://specs.optimism.io/protocol/fjord/derivation.html):
  a versioned channel encoding, `channel_version_byte ++ compress(rlp_batches)`.
  The only valid version byte is `1`, meaning brotli (RFC-7932) **with no custom
  dictionary**. The version byte must never have its low four bits set to 8 or
  15, so a decoder can distinguish versioned channels from legacy zlib ones.
- Reported ~35% reduction in DA usage; Base reports the input-to-output ratio
  moving from 0.4 to 0.25.
- Span batches themselves predate Fjord, arriving in
  [Delta](https://specs.optimism.io/protocol/delta/span-batches.html).

**Worth noting for our §8 soundness section:**
[optimism#19333](https://github.com/ethereum-optimism/optimism/issues/19333) —
Kona and op-node can disagree on brotli channels, with Kona's
`decompress_brotli` returning `Ok` where decompression did not reach
`ResultSuccess`. In a fault-proof context that divergence could let one client
accept output the other rejects. This is a production instance of the
malleability hazard, on the algorithm class we are evaluating.

## Taiko — zlib

**[source]** `zlib.NewWriter` in
`packages/taiko-client/pkg/utils/compress.go` of `taikoxyz/taiko-mono`:

```go
// Compress compresses the given txList bytes using zlib.
func Compress(txList []byte) ([]byte, error) {
```

The most conservative choice among the peers, and the one our measurements rank
last (see below).

## zkSync Era — a different problem

**[source]** `compress_state_diffs` in
`core/lib/types/src/storage/writes/mod.rs`, plus
`core/lib/multivm/src/pubdata_builders/`. A repository-wide search finds no
brotli or zstd.

Era posts **state diffs** rather than transaction data, so its compression
problem is a bespoke encoding of storage-slot changes, not a general-purpose
codec applied to a transaction stream. Not directly comparable — but a useful
reminder that changing *what* is posted can dominate changing *how* it is
compressed. That is the same shape as our `blockHash` finding, where a single
incompressible 32-byte field costs 6.7–12.6% of compressed payload.

## Cross-referencing against our own measurements

Every scheme chosen by a peer has now been measured on Linea payloads
(`results/candidate-comparison.csv`, 780 kB chunks, `recent` window):

| scheme | chosen by | ratio on Linea data |
|---|---|---|
| brotli-11 | Arbitrum, OP Stack, Base | **4.20** |
| zstd-19 | Scroll | 4.14 |
| LZSS | Linea (current) | 3.79 |
| zlib / DEFLATE | Taiko | 3.33 |

So the two schemes picked by projects that had to prove decompression in a zkVM
or a fault proof — zstd and brotli — are also the two that perform best on our
data, and Taiko's zlib is the weakest option measured. That is weak evidence,
since none of these projects optimised for Linea's payload, but it is at least
consistent.
