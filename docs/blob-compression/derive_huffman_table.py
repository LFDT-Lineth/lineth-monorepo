#!/usr/bin/env python3
"""Derive hardcoded canonical Huffman code lengths for the LZSS symbol alphabet.

Design being tabulated (see README):

    alphabet = 256 literals + 256 backref lengths + 1 EOF = 513 symbols
    after a length symbol: 1 flag bit (near/far), then a
    raw 14- or 21-bit address

The alternative -- folding near/far into the alphabet as 768 symbols -- codes
0.11% better but pushes the maximum code length from 11 to 19 bits, which is
~70% more iterations in a table-free decoder, paid on every literal. Not worth
it. Near/far is 48.4/51.6 across the corpus, so the flag bit carries 0.9992
bits and wastes essentially nothing.

Why an EOF symbol. The deployed scheme needs none: every symbol is exactly 8
bits and byte-aligned, so exhausting the input terminates the stream
unambiguously. Huffman breaks that -- up to 7 bits of padding remain in the
final byte and could decode as a spurious symbol. The obvious fix, "no codeword
shorter than 8 bits", is impossible: 513 symbols with every length >= 8 gives a
Kraft sum of at least 513/256 > 1. A minimum length of 8 admits at most 256
codewords, so on a byte alphabet the only code satisfying it is the flat 8-bit
one, which compresses nothing. An explicit end-of-stream symbol is what DEFLATE
uses (its symbol 256) and costs one occurrence per blob.

The table is HARDCODED rather than transmitted per block. Measured cost of that
is 0.51% (cross-entropy against tables trained on other windows only), and it
buys three things: the decoder never builds a table, so under zkC the table is a
precomputed column rather than a per-proof committed one; there is no untrusted
table to validate, removing a class of malleability bug that DEFLATE and zstd
decoders all carry; and the encoder loses its frequency-counting pass.

Only the code LENGTHS are emitted. Canonical Huffman derives the codes from the
lengths deterministically (same rule as DEFLATE 3.2.2), so lengths are the whole
table.

Two properties matter for correctness and are asserted below:

  * Every symbol gets a non-zero length. A hardcoded table must be able to
    encode input it has never seen -- a literal byte absent from the corpus
    still has to be representable -- so frequencies are Laplace-smoothed.
  * Code lengths are limited to --max-bits via package-merge, not by plain
    Huffman plus a fixup. The decode loop iterates once per code-length level,
    so the limit is a direct decoder cost.
"""
import argparse
import collections
import math
import pathlib
import subprocess
import sys

PROVER_DIR = pathlib.Path(__file__).resolve().parents[2] / "prover"
CHUNK = 780_000
N_LITERALS = 256
N_LENGTHS = 256                     # LZSS backref lengths are 1..256
EOF_SYMBOL = N_LITERALS + N_LENGTHS
ALPHABET = EOF_SYMBOL + 1


class BitReader:
    """MSB-first, matching icza/bitio as used by consensys/compress."""

    def __init__(self, buf):
        self.buf, self.pos = buf, 0

    def bits(self, n):
        v = 0
        for _ in range(n):
            v = (v << 1) | ((self.buf[self.pos >> 3] >> (7 - (self.pos & 7))) & 1)
            self.pos += 1
        return v

    def left(self):
        return len(self.buf) * 8 - self.pos


def parse_symbols(stream):
    """Yield alphabet indices from an LZSS bitstream.

    Wire format (backref.go): a literal is a raw byte; 0xFE and 0xFF are
    delimiters introducing a backref of 8-bit length-1 then a 14- or 21-bit
    address. The delimiter and the length collapse into one alphabet symbol
    here, which is where most of the coding gain comes from.
    """
    r = BitReader(stream)
    while r.left() >= 8:
        sym = r.bits(8)
        if sym in (0xFE, 0xFF):
            width = 14 if sym == 0xFE else 21
            if r.left() < 8 + width:
                break
            length = r.bits(8) + 1
            r.bits(width)                     # address: raw bits, not coded
            yield N_LITERALS + (length - 1)
        else:
            yield sym


def package_merge(weights, limit):
    """Length-limited optimal prefix code lengths (Larmore-Hirschberg).

    Plain Huffman would be simpler, and on this corpus its natural maximum is
    11 bits so the limit rarely binds -- but "rarely" is not "never" once
    smoothing adds near-zero-frequency symbols, and a fixup heuristic can emit
    a code that violates Kraft. Package-merge is exact.
    """
    n = len(weights)
    if n < 2:
        return {i: 1 for i, _ in weights}
    if 2 ** limit < n:
        raise ValueError(f"{n} symbols cannot fit in {limit} bits")

    items = sorted(((w, [i]) for i, w in weights), key=lambda t: t[0])
    current = list(items)
    for _ in range(limit - 1):
        packaged = [(current[j][0] + current[j + 1][0], current[j][1] + current[j + 1][1])
                    for j in range(0, len(current) - 1, 2)]
        current = sorted(items + packaged, key=lambda t: t[0])

    lengths = collections.Counter()
    for _, members in current[:2 * n - 2]:
        lengths.update(members)
    return lengths


def canonical_codes(lengths):
    """Assign canonical codes, DEFLATE 3.2.2 order: by length, then symbol."""
    by_len = collections.Counter(lengths.values())
    code, next_code = 0, {}
    for bits in range(1, max(by_len) + 1):
        code = (code + by_len.get(bits - 1, 0)) << 1
        next_code[bits] = code
    codes = {}
    for sym in sorted(lengths, key=lambda s: (lengths[s], s)):
        codes[sym] = next_code[lengths[sym]]
        next_code[lengths[sym]] += 1
    return codes


def collect_counts(payload_dir, dict_path, chunks_per_window):
    lzss_bin = PROVER_DIR / "bin" / "lzss-size"
    if not lzss_bin.exists():
        subprocess.run(["go", "build", "-o", "bin/", "./cmd/dev-tools/lzss-size"],
                       cwd=PROVER_DIR, check=True)
    counts = collections.Counter()
    tmp_in = pathlib.Path("/tmp/_huff_in.bin")
    tmp_out = pathlib.Path("/tmp/_huff_out.lzss")
    for payload in sorted(payload_dir.glob("*.payload.bin")):
        data = payload.read_bytes()
        for i in range(chunks_per_window):
            chunk = data[i * CHUNK:(i + 1) * CHUNK]
            if len(chunk) < CHUNK:
                break
            tmp_in.write_bytes(chunk)
            subprocess.run([str(lzss_bin), str(tmp_in), str(dict_path), str(tmp_out)],
                           capture_output=True, check=True)
            counts.update(parse_symbols(tmp_out.read_bytes()))
        print(f"  {payload.name}: running total {sum(counts.values()):,} symbols",
              flush=True)
    return counts


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--payloads", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "payloads")
    ap.add_argument("--dict", type=pathlib.Path, required=True)
    ap.add_argument("--chunks-per-window", type=int, default=4)
    ap.add_argument("--max-bits", type=int, default=15,
                    help="code-length limit; the decode loop iterates once per level")
    ap.add_argument("--min-bits", type=int, default=1,
                    help="minimum code length; >1 is checked against Kraft and will "
                         "fail for any alphabet larger than 2^min-bits")
    ap.add_argument("--emit-go", type=pathlib.Path,
                    help="write the code lengths as a Go source file")
    args = ap.parse_args()
    args.dict = args.dict.resolve()

    print("collecting symbol counts", flush=True)
    counts = collect_counts(args.payloads, args.dict, args.chunks_per_window)
    # One end-of-stream symbol per blob. Rare by construction, so it lands at the
    # maximum code length -- which is exactly right: it is read once.
    counts[EOF_SYMBOL] = counts.get(EOF_SYMBOL, 0) + args.chunks_per_window * 4
    total = sum(counts.values())
    unseen = ALPHABET - len(counts)

    # Laplace smoothing: a hardcoded table must encode symbols the corpus never
    # contained. Cost is bounded and reported below.
    weights = [(s, counts.get(s, 0) + 1) for s in range(ALPHABET)]

    if args.min_bits > 1:
        capacity = 2.0 ** -args.min_bits * ALPHABET
        if capacity > 1.0:
            sys.exit(f"infeasible: {ALPHABET} symbols with every code >= {args.min_bits} "
                     f"bits gives a Kraft sum of {capacity:.3f} > 1. A minimum length "
                     f"of L admits at most 2^L codewords.")

    lengths = package_merge(weights, args.max_bits)
    codes = canonical_codes(lengths)

    kraft = sum(2.0 ** -lengths[s] for s in range(ALPHABET))
    assert abs(kraft - 1.0) < 1e-9, f"Kraft sum {kraft}, code is not complete/prefix-free"
    assert all(1 <= lengths[s] <= args.max_bits for s in range(ALPHABET))
    assert len(set(codes.values())) == ALPHABET

    ideal = -sum((c / total) * math.log2(c / total) for c in counts.values())
    actual = sum(counts.get(s, 0) * lengths[s] for s in range(ALPHABET)) / total
    print(f"\n{total:,} symbols over {len(counts)} distinct "
          f"({unseen} of {ALPHABET} unseen, smoothed in)")
    print(f"  ideal entropy      {ideal:7.4f} bits/symbol")
    print(f"  this fixed table   {actual:7.4f} bits/symbol   "
          f"(+{100 * (actual / ideal - 1):.2f}% over adaptive-ideal)")
    print(f"  max code length    {max(lengths.values())} bits (limit {args.max_bits})")
    print(f"  Kraft sum          {kraft:.12f}")

    hist = collections.Counter(lengths.values())
    print("\n  code length histogram:")
    for b in sorted(hist):
        print(f"    {b:>2} bits: {hist[b]:>4} symbols")

    if args.emit_go:
        lines = [
            "// Code generated by docs/blob-compression/derive_huffman_table.py. DO NOT EDIT.",
            "",
            "package lzss",
            "",
            "// SymbolCodeLengths holds canonical Huffman code lengths for the combined",
            "// literal/length alphabet: indices 0..255 are literal bytes, 256..511 are",
            "// backref lengths 1..256, and 512 is end-of-stream. Codes follow the canonical",
            "// construction (DEFLATE 3.2.2): order by length, then by symbol.",
            "//",
            "// A backref symbol is followed by one flag bit (0 = near, 14-bit address;",
            "// 1 = far, 21-bit address) and then the raw address bits. Symbol 512 is",
            "// end-of-stream: Huffman codes are not byte-aligned, so trailing padding",
            "// in the final byte would otherwise decode as a spurious symbol.",
            "//",
            f"// Derived from {total:,} symbols; {actual:.4f} bits/symbol against an ideal",
            f"// {ideal:.4f}. Frequencies are Laplace-smoothed so every symbol is codeable.",
            f"var SymbolCodeLengths = [{ALPHABET}]uint8{{",
        ]
        for row in range(0, ALPHABET, 16):
            end = min(row + 16, ALPHABET)      # the last row is short; do not pad
            lines.append("\t" + " ".join(f"{lengths[s]}," for s in range(row, end)))
        lines += ["}", ""]
        args.emit_go.write_text("\n".join(lines))
        print(f"\nwrote {args.emit_go}")


if __name__ == "__main__":
    sys.exit(main())
