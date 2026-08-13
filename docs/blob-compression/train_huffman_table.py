#!/usr/bin/env python3
"""Train the hardcoded Huffman table for the lzss+huffman arm.

Produces results/huffman-table.hex, which candidate_comparison.py consumes.

The alphabet is 512 symbols: 0-255 literal bytes, 256-511 backref lengths
1..256. Every code is at least 8 bits, so fewer than 8 bits of trailing padding
can never complete a codeword and the stream self-terminates without an EOF
symbol -- the property the deployed byte-aligned format has for free.

The table is stored as TEXT. A raw 512-byte table would be silently corrupted by
this repository's `* text eol=lf` attribute, which strips CR before LF: code
lengths 13 and 10 are both legal values, so an adjacent pair is a live hazard.
The current table happens to contain none, which is luck, not safety.

The Huffman work lives on an unmerged branch of consensys/compress, so there is
no module version to pin; the commit is asserted at run time instead.
"""
import argparse
import pathlib
import subprocess
import sys
import tempfile

CHUNK = 780_000                     # same unit as candidate_comparison.py
COMPRESS_COMMIT = "c509f05"
COMPRESS_BRANCH = "feat/huffman-on-lengths"


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--compress-repo", type=pathlib.Path, required=True,
                    help=f"checkout of consensys/compress at {COMPRESS_COMMIT} "
                         f"({COMPRESS_BRANCH})")
    ap.add_argument("--payloads", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "payloads")
    ap.add_argument("--dict", type=pathlib.Path, required=True)
    ap.add_argument("--out", type=pathlib.Path,
                    default=pathlib.Path(__file__).parent / "results" / "huffman-table.hex")
    args = ap.parse_args()
    repo, dict_path = args.compress_repo.resolve(), args.dict.resolve()
    args.out = args.out.resolve()

    head = subprocess.run(["git", "-C", str(repo), "rev-parse", "--short", "HEAD"],
                          capture_output=True, text=True, check=True).stdout.strip()
    if not (head.startswith(COMPRESS_COMMIT) or COMPRESS_COMMIT.startswith(head)):
        sys.exit(f"{repo} is at {head}, expected {COMPRESS_COMMIT} ({COMPRESS_BRANCH})")

    with tempfile.TemporaryDirectory() as tmp:
        tmp = pathlib.Path(tmp)
        hufftable = tmp / "hufftable"
        subprocess.run(["go", "build", "-o", str(hufftable), "./cmd/hufftable"],
                       cwd=repo, check=True)

        # Train on the same blob-sized chunks the ratios are measured on, so the
        # table is not tuned to a unit nobody uses.
        chunks = tmp / "chunks"
        chunks.mkdir()
        n = 0
        for payload in sorted(args.payloads.glob("*.payload.bin")):
            data = payload.read_bytes()
            window = payload.name.removesuffix(".payload.bin")
            for i in range(len(data) // CHUNK):
                (chunks / f"{window}_{i:02d}.bin").write_bytes(
                    data[i * CHUNK:(i + 1) * CHUNK])
                n += 1
        print(f"training on {n} chunks of {CHUNK:,} B", flush=True)

        raw = tmp / "table.bin"
        out = subprocess.run([str(hufftable), "-dict", str(dict_path),
                              "-files", str(chunks / "*.bin"), "-o", str(raw)],
                             capture_output=True, text=True, check=True)
        print(out.stdout.strip())

        table = raw.read_bytes()

    if len(table) != 512:
        sys.exit(f"expected a 512-byte table, got {len(table)}")
    crlf = [i for i in range(len(table) - 1) if table[i] == 13 and table[i + 1] == 10]

    header = [
        "# Canonical Huffman code lengths for consensys/compress lzss.",
        f"# Source: {COMPRESS_BRANCH} @ {head}",
        "# 512 symbols: 0-255 literals, 256-511 backref lengths 1..256.",
        "# One length per symbol, hex, 32 per line.",
        "#",
        "# Stored as TEXT deliberately: this repo's `* text eol=lf` strips CR before",
        "# LF, and code lengths 13 and 10 are both legal, so a raw table can be",
        f"# silently corrupted. This one has {len(crlf)} such adjacent pairs.",
        f"# Trained on {n} x {CHUNK:,} B chunks across all corpus windows.",
    ]
    body = [" ".join(f"{b:02x}" for b in table[i:i + 32]) for i in range(0, 512, 32)]
    args.out.write_text("\n".join(header + body) + "\n")

    print(f"\nwrote {args.out}")
    print(f"  code lengths {min(table)}..{max(table)}, "
          f"Kraft {sum(2.0 ** -b for b in table):.9f}")


if __name__ == "__main__":
    sys.exit(main())
