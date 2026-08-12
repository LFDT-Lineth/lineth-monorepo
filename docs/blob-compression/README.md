# Blob compression evaluation — measurements

Reproducible results backing [`../blob-compression-evaluation-plan.md`](../blob-compression-evaluation-plan.md).
Two tables live in [`results/`](results/), together with the code that regenerates them.

## Reproducing

```bash
# 1. Fetch and verify the corpus (~10 min, writes outside the repo)
python3 fetch_corpus.py

# 2. Payload structure
python3 payload_structure.py ~/linea-blob-corpus/streams > results/payload-structure.csv

# 3. Candidate comparison
python3 candidate_comparison.py --dict ../../prover/lib/compressor/dict/25-04-21.bin
```

Requires `go`, `zstd` and `lz4` on `PATH`. The Go tools live in
[`../../prover/cmd/dev-tools/`](../../prover/cmd/dev-tools/): `blob-anatomy`
(fetch), `blob-roundtrip` (verify), `blob-streams` (split into field classes),
`blob-chunks` (blob-sized chunking + LZSS), `blob-calldata-probe`.

## Why the corpus is not in this repository

It is ~100 MB of binary, and `.gitattributes` sets `* text eol=lf` with **no
`*.bin` exception**. Committing therefore strips every CR that precedes an LF,
which silently removed 61–111 bytes per corpus file — enough to make the outer
RLP undecodable. `git check-attr text` reports `set` for our corpus and also for
`prover/lib/compressor/dict/25-04-21.bin`, `blocks_rlp.bin` and the sample
blobs; all four contain zero CRLF pairs where chance predicts ~2.8, which is
consistent with the same normalisation having already touched them. **That is a
pre-existing repository bug worth fixing separately.**

Everything here is reproducible from the fixed block numbers in
`fetch_corpus.py`, so the corpus lives in `~/linea-blob-corpus/` instead.
Provenance for the published tables is in [`results/manifests/`](results/manifests/).

## The corpus

Four contiguous windows, each sized to **15.6 MB of payload** — 20 blob-payloads
at `MaxUncompressedBytes` (780 kB). Sizing by bytes rather than block count
matters: traffic density varies ~10× across regimes, so a fixed block count
would sample them wildly unequally.

| window | start block | blocks | txs |
|---|---|---|---|
| `2025-09-25_busy` | 23,769,747 | 3,945 | 21,621 |
| `2025-01-06_median` | 14,261,848 | 10,360 | 31,185 |
| `2026-04-28_quiet` | 30,425,277 | 12,745 | 23,571 |
| `2026-07-28_recent` | 31,548,567 | 13,644 | 20,955 |

No window is anchored to the chain tip; a tip-anchored window yields different
data on every run. (`2026-08-11_current` was such a window and is superseded by
`recent`; its manifest is retained for the record.)

Every block is verified: `DeriveSha(txs) == header.TxHash`, boundary recovery via
`ScanBlockByteLen`, full decode, per-field equality, sender recovery, and the
locally recomputed block hash against the node's.

## `results/payload-structure.csv`

Per field class, per window: raw bytes, zero bytes and zero runs, the zero-free
residue, and compressed size.

Zero bytes are near-free to an LZ stage — a run of *N* zeros costs one match
reference regardless of *N* — so a field's share of the **raw** payload
overstates the work it gives the compressor. The `share_of_nonzero_pct` column
rescales onto the zero-free residue that LZ and the entropy coder must actually
model.

Two things it shows:

- Calldata is the only field whose zeros form long runs (mean ≈ 26 bytes; every
  other field averages ≈ 1.0, i.e. isolated zeros). Rescaling therefore drops
  calldata from ~88% of raw to ~73% zero-free and lifts everything else
  proportionally, without changing the ordering.
- **`hashes` is the anomaly on every basis.** ~2.8% of raw and ~6.9% zero-free,
  but **12.6% of compressed output**, because 32 random bytes per block are
  incompressible (own ratio 1.00×; zstd emits them slightly *larger*). Its share
  rises as everything else compresses better, so it is worst in exactly the
  regimes where the total is smallest.

**Method caveat.** Each class is compressed *in isolation*, so shares are against
the sum of independently-compressed streams — 1.3% larger than the real
interleaved payload. Splitting also destroys cross-class matches, so any class
whose bytes recur elsewhere is **overstated**; `from` addresses plausibly appear
inside calldata as ABI arguments, making that column an upper bound. `hashes` is
unaffected by this, having no cross-class redundancy by construction. A true
marginal cost needs ablation (compress with and without a class, take the
difference).

## `results/candidate-comparison.csv`

Each scheme on **780 kB chunks**, with the deployed dictionary and with an
all-zero dictionary of equal size.

Chunking is not incidental. Production compresses one blob at a time, so the
matcher only ever sees 780 kB of context; measuring whole 15 MB windows lets LZ
find matches across history it will never have, inflating every ratio by ~10%.

Ratios with the deployed dictionary:

| window | LZSS (deployed) | zstd-19 | lz4-9 |
|---|---|---|---|
| `2025-09-25_busy` | 6.01× | 6.91× | 5.02× |
| `2025-01-06_median` | 4.34× | 4.83× | 3.69× |
| `2026-04-28_quiet` | 3.86× | 4.29× | 3.43× |
| `2026-07-28_recent` | 3.79× | 4.14× | 3.45× |

- **zstd beats the deployed scheme by 9–15%**, not the large margin whole-window
  measurements suggested.
- **lz4 is consistently *worse* than the deployed LZSS.** The cause is window
  size, not cleverness: `consensys/compress` LZSS uses 21 address bits (2 MiB
  window), while lz4's block format has 16-bit offsets and is hard-capped at
  64 KiB — 8% of a chunk. This is a format limit no tuning fixes.
- **The dictionary is marginal for everyone**: +1.95% for LZSS, +1.4% for zstd,
  +0.6% for lz4. 64 kB of dictionary against 780 kB of payload barely registers,
  which is what the all-zero control isolates.

## What this implies for the decision

Δ(1/ρ) between LZSS and zstd on `recent` is 0.0225, so zstd's extra decoder
cycles must fit inside roughly 1.6 cycles/byte at 1 gwei blob fees, or 48 at
30 gwei (§4 of the plan). That is the question the zkVM work has to answer.

Separately, dropping `blockHash` is worth ~12.6% of compressed payload — larger
than the entire zstd-over-LZSS gap — at the cost of forcing re-execution for
state recovery. It is a protocol decision rather than a compression one, and the
figure deserves confirming by ablation before it is argued anywhere.
