# Blob compression evaluation — measurements

Reproducible results backing [`../blob-compression-evaluation-plan.md`](../blob-compression-evaluation-plan.md).
Tables live in [`results/`](results/), together with the code that regenerates
them. [`prior-art.md`](prior-art.md) surveys what other rollups use, with
receipts.

## Reproducing

```bash
# 1. Fetch and verify the corpus (~10 min, writes outside the repo)
#    Verification dominates: it re-checks every block hash against the node.
#    Pass --skip-verify to halve it once the corpus is known good.
python3 fetch_corpus.py

# 2. Payload structure
python3 payload_structure.py ~/linea-blob-corpus/streams > results/payload-structure.csv

# 3. Huffman table for the lzss+huffman arm (needs a consensys/compress checkout)
python3 train_huffman_table.py --compress-repo ~/Projects/compress \
    --dict ../../prover/lib/compressor/dict/25-04-21.bin

# 4. Candidate comparison (--compress-repo enables the lzss+huffman arm)
python3 candidate_comparison.py --dict ../../prover/lib/compressor/dict/25-04-21.bin \
    --compress-repo ~/Projects/compress
```

The `lzss+huffman` arm needs a checkout of `consensys/compress` at commit
**`c509f05`** on branch **`feat/huffman-on-lengths`**. The Huffman work is
unmerged, so there is no module version to pin; both scripts assert the commit
and refuse to run against an unknown revision rather than silently publishing
numbers from one.

Requires `go`, `zstd`, `lz4`, `brotli` and `bzip2` on `PATH` (DEFLATE uses
Python's `zlib`, since the `gzip` CLI cannot accept a dictionary and that would
leave it as the only arm measured without one). The Go tools live in
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

Seven schemes on **780 kB chunks**, in two groups: each scheme at its **best
available configuration**, then the **dictionary ablation**. Within a group rows
are ordered by measured size, best first, so adjacent rows are directly
comparable.

bzip2 appears only in the first group, with dictionary `n/a`. Block sorting has
no external-dictionary mechanism, so dictionary-less *is* its best configuration
— pairing it with everyone else's ablated runs would understate it — and it has
no dictionary contribution to ablate in the second. Ordering by measurement
rather than a fixed scheme list also matters because the two groups disagree.

Chunking is not incidental. Production compresses one blob at a time, so the
matcher only ever sees 780 kB of context; measuring whole 15 MB windows lets LZ
find matches across history it will never have, inflating every ratio by ~10%.
An earlier claim here that DEFLATE underperforms lz4 was made on whole-window
numbers; it survives re-measurement on chunks, but the original reasoning did
not.

**Best available configuration** (deployed dictionary where the format supports one):

| scheme | median | busy | quiet | recent |
|---|---|---|---|---|
| brotli-11 | 5.09 | 7.41 | 4.38 | **4.20** |
| zstd-19 | 4.83 | 6.91 | 4.29 | 4.14 |
| **lzss+huffman** | **4.53** | **6.32** | **4.00** | **3.90** |
| lzss (deployed) | 4.34 | 6.01 | 3.86 | 3.79 |
| bzip2-9 *(dictionary n/a)* | 4.19 | 6.09 | 3.79 | 3.72 |
| lz4-9 | 3.69 | 5.02 | 3.43 | 3.45 |
| deflate-9 | 3.52 | 4.81 | 3.31 | 3.33 |

`lzss+huffman` is the deployed scheme with a canonical Huffman code over a
combined 512-symbol alphabet (256 literals + 256 backref lengths), from
`consensys/compress` at `c509f05`. It appears only in the first group: the
Huffman table is hardcoded, so there is no per-run table to ablate, and the LZ
dictionary is the same one the deployed arm uses.

It gains **+3.88%** over the deployed scheme overall (+5.16% on `busy`, +2.95%
on `recent`), which closes about a third of the distance to zstd. Two caveats:
the table is trained on the same 80 chunks it is measured on, and the earlier
cross-window test put the honest out-of-sample penalty at ~0.51%, so call it
+3.4%. And this is below the +4.9% an idealised order-0 entropy calculation
predicted — the gap being integer code lengths, the minimum-8-bit constraint
(Kraft slack 1.6%), and a parse that is not re-optimised against the final code
lengths.

Round-trip was verified on all 80 chunks with zero failures.

**Dictionary ablation** (same schemes, dictionary removed):

| scheme | median | busy | quiet | recent |
|---|---|---|---|---|
| brotli-11 | 4.82 | 7.06 | 4.27 | 4.11 |
| zstd-19 | 4.63 | 6.69 | 4.22 | 4.08 |
| lzss (deployed) | 4.14 | 5.78 | 3.77 | 3.72 |
| lz4-9 | 3.65 | 4.99 | 3.42 | 3.44 |
| deflate-9 | 3.51 | 4.80 | 3.30 | 3.32 |

For LZSS the dictionary is not optional at the API level, so "no dictionary"
means an **empty** one — `AugmentDict` reduces it to the two reserved symbols.
An all-zero 64 kB filler was tried as an alternative control, on the theory that
it might supply free zero-run matches on data that is 66–70% zeros. It lands
within 12 bytes of empty, and the reason is that LZSS already encodes runs
optimally without help: it emits one literal zero and then back-references at
displacement 1, the classic LZ77 run encoding. An all-zero dictionary can save
only that single leading literal, once per chunk — 20 bytes across a window,
which is the magnitude observed.

- **brotli wins every window**, by 1.4–7.2% over zstd and 10–23% over the
  deployed LZSS. It also gains most from the dictionary (+5.6% on `median`),
  which is consistent with its own built-in 122 kB dictionary being
  text-oriented and largely useless on binary blob data.
- **The ranking is essentially identical with and without a dictionary**, and
  the gaps barely move. Scheme choice can therefore be made on the
  dictionary-less numbers, treating the dictionary as an orthogonal few-percent
  bonus: +0.3% (deflate), +0.6% (lz4), +1.4–4.3% (zstd), +2.0% (lzss),
  +2.2–5.6% (brotli).
- **lz4 and DEFLATE are both worse than the deployed LZSS**, for the same
  structural reason: window size. `consensys/compress` LZSS uses 21 address bits
  (2 MiB window); lz4's block format has 16-bit offsets (64 KiB) and DEFLATE has
  32 KiB. DEFLATE's window also caps how much of the dictionary it can reach —
  at most the last 32,768 bytes — which is why it gains almost nothing from one.
  Adding Huffman to LZ77 does not compensate for a window a sixtieth the size.
- **bzip2 beats the deployed scheme on `busy` (6.09 vs 6.01) and matches it
  elsewhere — with no dictionary at all**, against an LZSS that has one. BWT genuinely extracts more from this data than LZSS
  does. It stays a reference point rather than a candidate: inverting a BWT means
  executing a full block sort, which is cheap as a Plonk permutation argument and
  expensive in a RISC-V zkVM. (Note this cuts against the intuition that moving
  to a zkVM makes everything easier — for BWT specifically it makes it harder.)
  Brotli, despite the name-adjacency, contains no BWT: it is LZ77 + Huffman +
  context modelling + a static dictionary.

## What this implies for the decision

Against the deployed LZSS on `recent`, Δ(1/ρ) is 0.0225 for zstd and 0.0257 for
brotli. Using §4 of the plan, that buys roughly 1.6 and 1.8 cycles/byte
respectively at 1 gwei blob fees, or 48 and 55 at 30 gwei. Both decoders have to
fit inside that budget, and brotli buys marginally more room while carrying
context modelling that zstd does not — so both are worth pricing in the zkVM
step, not just zstd.

Separately, dropping `blockHash` is worth ~12.6% of compressed payload — larger
than the entire zstd-over-LZSS gap — at the cost of forcing re-execution for
state recovery. It is a protocol decision rather than a compression one, and the
figure deserves confirming by ablation before it is argued anywhere.
