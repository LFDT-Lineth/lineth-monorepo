#!/usr/bin/env python3
"""Consolidate every decoder benchmark into one table.

Each bench writes its own CSV in its own schema, which is fine for running a
single bench and useless for comparing them. This reads them all, joins the
compression ratios measured over the whole corpus, and derives the quantities
the cost model actually uses.

    S(rho, kappa) = Lambda / (rho * B) + kappa        cycles per payload byte

with B = 4096 * 254 / 8 = 130048 usable bytes per blob and Lambda the single
free parameter (cycles a blob's L1 cost plus per-blob proving overhead is
worth). Because S is linear in 1/rho and not in rho, 1/rho -- compressed bytes
per payload byte -- is the column to compare, and the crossover between any two
arms is

    Lambda* = B * (kappa_A - kappa_B) / (1/rho_B - 1/rho_A)

Ratios come from the four-window corpus aggregate (total compressed / total
raw), not the per-bench fixture: the fixture is one 780,000- or 262,144-byte
chunk of one window, while the aggregate weights each regime by the bytes it
actually produces. Both are shown; they differ by a few percent.

Usage:  python3 summarize.py
"""

import csv
import os
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
CORPUS = os.path.join(
    HERE, "..", "..", "docs", "blob-compression", "results", "candidate-comparison.csv"
)

B = 4096 * 254 // 8  # usable bytes per EIP-4844 blob
KAPPA_FIX = 9.50  # Poseidon2 over the payload, bench_hash; common to all arms

# Per-bench CSV -> (display name, dictionary, corpus scheme key or None).
# The corpus key joins to candidate-comparison.csv (scheme, dictionary).
ADAPTERS = {
    "bench_lzss/bench/bench-lzss.csv": {
        "lzss v0.3.0 (A0)": ("LZSS v0.3.0 (A0)", "yes", ("lzss(deployed)", "25-04-21.bin")),
        "lzss + huffman-on-lengths": ("LZSS + Huffman", "yes", ("lzss+huffman", "25-04-21.bin")),
        "lzss v3 huffman": ("LZSS v3 (Huffman, LSB)", "yes", ("lzss+huffman", "25-04-21.bin")),
    },
    "bench_zstd/bench/bench-zstd.csv": {
        "zstd -19 (no dictionary)": ("zstd-19 zig-std +wide memcpy", "no", ("zstd-19", "none")),
        "zstd -19 (no dictionary, compiler-rt memcpy)": (
            "zstd-19 zig-std (compiler-rt)", "no", ("zstd-19", "none")),
    },
    "bench_zstd_c/bench/bench-zstd-c.csv": {
        "zstd -19 (C reference v1.5.7)": ("zstd-19 C reference", "no", ("zstd-19", "none")),
    },
    "bench_lz4/bench/bench-lz4.csv": {
        "lz4 HC-9": ("LZ4 (HC-9)", "yes", ("lz4-9", "25-04-21.bin")),
    },
}

# Benches with no runner yet; their numbers exist only in session notes, so they
# are named here rather than silently omitted from the table.
MISSING = {
    "bench_hash": "Poseidon2 measured at 9.50 c/B ad-hoc; no run.go",
    "bench_memory": "no run.go (ROM/RAM microbenchmark, no per-byte figure)",
    "bench_bitread": "run.go exists but writes no CSV",
}


def corpus_ratios():
    """Aggregate ratio per (scheme, dictionary): total raw / total compressed."""
    if not os.path.exists(CORPUS):
        return {}
    totals = defaultdict(lambda: [0, 0])
    for r in csv.DictReader(open(CORPUS)):
        k = (r["scheme"], r["dictionary"])
        totals[k][0] += int(r["raw_bytes"])
        totals[k][1] += int(r["compressed_bytes"])
    return {k: raw / comp for k, (raw, comp) in totals.items()}


def load():
    agg = corpus_ratios()
    rows = []
    for rel, mapping in ADAPTERS.items():
        path = os.path.join(HERE, rel)
        if not os.path.exists(path):
            continue
        for r in csv.DictReader(open(path)):
            m = mapping.get(r["variant"])
            if m is None:
                continue
            name, has_dict, key = m
            fixture_ratio = int(r["decompressed_bytes"]) / int(r["compressed_bytes"])
            rows.append({
                "name": name,
                "dict": has_dict,
                "kappa": float(r["cycles_per_byte"]),
                "fixture_ratio": fixture_ratio,
                "corpus_ratio": agg.get(key),
                "bytes": int(r["decompressed_bytes"]),
            })
    return rows


def main():
    rows = load()
    if not rows:
        print("no bench CSVs found; run the per-bench run.go first")
        return
    # Rank by the quantity the model uses: compressed bytes per payload byte.
    rows.sort(key=lambda r: r["kappa"])

    print(f"B = {B:,} usable bytes/blob   kappa_fix = {KAPPA_FIX} c/B (hashing, common to all)\n")
    hdr = (f"{'scheme':<30}{'dict':>5}{'ratio(fix)':>11}{'ratio(corp)':>12}"
           f"{'1/rho':>9}{'k_dec':>9}{'k_total':>9}")
    print(hdr)
    print("-" * len(hdr))
    for r in rows:
        rho = r["corpus_ratio"] or r["fixture_ratio"]
        corp = f"{r['corpus_ratio']:.3f}" if r["corpus_ratio"] else "-"
        print(f"{r['name']:<30}{r['dict']:>5}{r['fixture_ratio']:>11.3f}{corp:>12}"
              f"{1/rho:>9.5f}{r['kappa']:>9.2f}{r['kappa'] + KAPPA_FIX:>9.2f}")

    # The crossover is one Lambda per unordered pair, so list pairs rather than
    # a symmetric matrix, and say which arm wins on each side of it.
    print("\npairwise crossovers (Lambda = cycles per blob that L1 cost is worth):")
    for i, a in enumerate(rows):
        for b in rows[i + 1:]:
            ra = 1 / (a["corpus_ratio"] or a["fixture_ratio"])
            rb = 1 / (b["corpus_ratio"] or b["fixture_ratio"])
            if abs(rb - ra) < 1e-12:
                continue
            lam = B * (a["kappa"] - b["kappa"]) / (rb - ra)
            cheap, dense = (a, b) if a["kappa"] < b["kappa"] else (b, a)
            if lam < 0:
                # One arm decodes cheaper AND compresses better: no Lambda flips it.
                print(f"  {dense['name']} dominated by {cheap['name']} at every Lambda")
            else:
                print(f"  Lambda < {lam/1e6:8.1f}M : {cheap['name']:<30}"
                      f" | above: {dense['name']}")

    # An arm is only ever optimal if it is on the lower convex hull of
    # (1/rho, kappa); one strictly between two others in both coordinates can
    # still be beaten at every Lambda.
    print("\noptimal at some Lambda:")
    for r in rows:
        x, k = 1 / (r["corpus_ratio"] or r["fixture_ratio"]), r["kappa"]
        beaten = False
        for o in rows:
            if o is r:
                continue
            xo, ko = 1 / (o["corpus_ratio"] or o["fixture_ratio"]), o["kappa"]
            if xo <= x and ko <= k:
                beaten = True
                break
        if not beaten:
            # Check it is not above the chord joining a cheaper and a denser arm.
            for lo in rows:
                for hi in rows:
                    xl, kl = 1 / (lo["corpus_ratio"] or lo["fixture_ratio"]), lo["kappa"]
                    xh, kh = 1 / (hi["corpus_ratio"] or hi["fixture_ratio"]), hi["kappa"]
                    if xh < x < xl and kl < k < kh:
                        t = (x - xl) / (xh - xl)
                        if kl + t * (kh - kl) < k:
                            beaten = True
        print(f"  {'yes' if not beaten else 'NO (off the hull)':<18} {r['name']}")

    if MISSING:
        print("\nnot in the table (no CSV):")
        for k, v in sorted(MISSING.items()):
            print(f"  {k:<16} {v}")


if __name__ == "__main__":
    main()
