#!/usr/bin/env python3
"""Compare candidate compressors on blob-sized units.

Production compresses ONE BLOB AT A TIME, so the matcher only ever sees
MaxUncompressedBytes (780 kB) of context. Measuring on whole multi-megabyte
windows lets LZ find matches across history it will never have and inflates
every ratio by roughly 10%, so everything here runs on 780 kB chunks.

Each scheme is additionally run against an all-zero dictionary of the same size,
which isolates how much the real dictionary actually contributes. (Answer, so
far: 0.6-2%. A 64 kB dictionary against 780 kB of payload is marginal.)
"""
import argparse
import csv
import pathlib
import re
import subprocess
import sys
import tempfile

PROVER_DIR = pathlib.Path(__file__).resolve().parents[2] / "prover"
TOTAL_RE = re.compile(r"^TOTAL\s+(\d+)\s+(\d+)\s+([\d.]+)x", re.M)


def lzss_chunked(payload, dict_path, chunk_dir):
    """Run the deployed LZSS over blob-sized chunks; returns (raw, compressed).

    blob-chunks also writes the chunks to chunk_dir, which the other schemes
    then reuse so every scheme is measured on byte-identical units.
    """
    out = subprocess.run(
        ["go", "run", "./cmd/dev-tools/blob-chunks", str(payload),
         str(dict_path), str(chunk_dir)],
        cwd=PROVER_DIR, capture_output=True, text=True, check=True).stdout
    m = TOTAL_RE.search(out)
    if not m:
        raise RuntimeError(f"no TOTAL line in blob-chunks output:\n{out[-500:]}")
    return int(m.group(1)), int(m.group(2))


def compressed_size(cmd, chunk, dict_path):
    argv = list(cmd) + (["-D", str(dict_path)] if dict_path else []) + ["-c", str(chunk)]
    return len(subprocess.run(argv, capture_output=True, check=True).stdout)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--payloads", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "payloads")
    ap.add_argument("--dict", type=pathlib.Path, required=True)
    ap.add_argument("--out", type=pathlib.Path,
                    default=pathlib.Path(__file__).parent / "results" / "candidate-comparison.csv")
    args = ap.parse_args()

    # blob-chunks runs with cwd=prover/, so every path handed to a subprocess
    # must be absolute or it resolves against the wrong directory.
    args.dict = args.dict.resolve()
    args.payloads = args.payloads.resolve()
    args.out = args.out.resolve()

    args.out.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmp:
        zero_dict = pathlib.Path(tmp) / "zero_dict.bin"
        zero_dict.write_bytes(bytes(args.dict.stat().st_size))

        rows = []
        for payload in sorted(args.payloads.glob("*.payload.bin")):
            window = payload.name.removesuffix(".payload.bin")
            print(f"=== {window} ===", flush=True)
            chunk_dir = pathlib.Path(tmp) / window
            chunk_dir.mkdir()

            raw, lzss_real = lzss_chunked(payload, args.dict, chunk_dir)
            _, lzss_zero = lzss_chunked(payload, zero_dict, pathlib.Path(tmp) / (window + "_z"))
            chunks = sorted(chunk_dir.glob("chunk_*.bin"))

            rows.append((window, "lzss(deployed)", args.dict.name, len(chunks), raw, lzss_real))
            rows.append((window, "lzss(deployed)", "all-zero", len(chunks), raw, lzss_zero))

            for scheme, cmd in (("zstd-19", ["zstd", "-19", "-q"]),
                                ("lz4-9", ["lz4", "-9", "-q"])):
                for label, d in ((args.dict.name, args.dict), ("none", None)):
                    total = sum(compressed_size(cmd, c, d) for c in chunks)
                    rows.append((window, scheme, label, len(chunks), raw, total))

        with args.out.open("w", newline="") as fh:
            w = csv.writer(fh)
            w.writerow(["window", "scheme", "dictionary", "chunks",
                        "raw_bytes", "compressed_bytes", "ratio"])
            for window, scheme, d, n, raw, comp in rows:
                w.writerow([window, scheme, d, n, raw, comp, f"{raw / comp:.4f}"])
    print(f"wrote {args.out}")


if __name__ == "__main__":
    sys.exit(main())
