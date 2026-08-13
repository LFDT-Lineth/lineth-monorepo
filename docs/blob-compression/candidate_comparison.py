#!/usr/bin/env python3
"""Compare candidate compressors on blob-sized units.

Production compresses ONE BLOB AT A TIME, so the matcher only ever sees
MaxUncompressedBytes (780 kB) of context. Measuring on whole multi-megabyte
windows lets LZ find matches across history it will never have and inflates
every ratio by roughly 10%, so everything here runs on 780 kB chunks.

Each scheme is additionally run without a dictionary, which isolates how much
the real one contributes. (Answer, so far: 0.3-5.6%. A 64 kB dictionary against
780 kB of payload is marginal for every scheme.) For LZSS the dictionary is not
optional at the API level, so "no dictionary" means an EMPTY one: AugmentDict
reduces it to the two reserved symbols. An all-zero filler dictionary was tried
as an alternative control and lands within 12 bytes of empty -- LZSS caps
backrefs at 256 bytes and encodes near matches more cheaply, so zeros already in
the chunk always beat zeros in the dictionary -- but empty needs no such
argument.

lzss+huffman is the deployed scheme with a canonical Huffman code over a
combined 512-symbol alphabet (256 literals + 256 backref lengths), from
consensys/compress branch feat/huffman-on-lengths. Every code is >= 8 bits, so
sub-byte padding cannot complete a codeword and the stream still
self-terminates without an EOF symbol. The table is hardcoded, not transmitted;
results/huffman-table.hex holds it as text, since a raw 512-byte table would be
corrupted by this repo's `* text eol=lf` attribute.

bzip2 is included as the Burrows-Wheeler reference point rather than as a
serious candidate. BWT has no external-dictionary mechanism, and its inverse is
a full block sort: cheap in a hand-arithmetized circuit, where a permutation is
just a permutation argument, but expensive in a zkVM where the sort must
actually be executed instruction by instruction. It is here to bound what more
aggressive compression buys, not to be ported.
"""
import argparse
import csv
import pathlib
import re
import subprocess
import sys
import tempfile
import zlib

PROVER_DIR = pathlib.Path(__file__).resolve().parents[2] / "prover"

# The lzss+huffman arm needs consensys/compress at an unmerged branch, so it
# cannot be pinned by module version. The commit is asserted at run time instead;
# numbers in results/ came from exactly this one.
COMPRESS_COMMIT = "c509f05"
COMPRESS_BRANCH = "feat/huffman-on-lengths"
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


def deflate_size(chunk, dict_path):
    """Raw DEFLATE via zlib, in-process.

    The gzip CLI cannot take a dictionary, which would leave DEFLATE as the only
    arm measured without one; zlib's API can. wbits=-15 selects raw DEFLATE so we
    measure the codec rather than a wrapper. Note zlib's window is 32 KiB, so at
    most the last 32768 bytes of the dictionary are reachable however large it is
    -- itself a reason DEFLATE underperforms here.
    """
    kw = dict(level=9, method=zlib.DEFLATED, wbits=-15, memLevel=9,
              strategy=zlib.Z_DEFAULT_STRATEGY)
    co = (zlib.compressobj(**kw, zdict=dict_path.read_bytes())
          if dict_path else zlib.compressobj(**kw))
    return len(co.compress(chunk.read_bytes()) + co.flush())


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--payloads", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "payloads")
    ap.add_argument("--dict", type=pathlib.Path, required=True)
    ap.add_argument("--compress-repo", type=pathlib.Path,
                    help="checkout of consensys/compress; enables the lzss+huffman "
                         f"arm. Must be at commit {COMPRESS_COMMIT} "
                         f"(branch {COMPRESS_BRANCH}) -- the Huffman work is not "
                         "merged, so there is no released version to pin.")
    ap.add_argument("--huffman-table", type=pathlib.Path,
                    default=pathlib.Path(__file__).parent / "results" / "huffman-table.hex")
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
        # AugmentDict turns this into just the two reserved symbols, which is
        # the closest LZSS has to "no dictionary".
        empty_dict = pathlib.Path(tmp) / "empty_dict.bin"
        empty_dict.write_bytes(b"")

        # The hex table is unpacked to a raw file only inside this temp dir; it is
        # never stored raw in the repository.
        huff_tbl = linzip = None
        if args.compress_repo:
            repo = args.compress_repo.resolve()
            head = subprocess.run(["git", "-C", str(repo), "rev-parse", "--short", "HEAD"],
                                  capture_output=True, text=True, check=True).stdout.strip()
            if not head.startswith(COMPRESS_COMMIT) and not COMPRESS_COMMIT.startswith(head):
                sys.exit(f"{repo} is at {head}, expected {COMPRESS_COMMIT} "
                         f"({COMPRESS_BRANCH}). Refusing to publish numbers from an "
                         f"unknown revision.")
            linzip = pathlib.Path(tmp) / "linzip"
            subprocess.run(["go", "build", "-o", str(linzip), "."], cwd=repo, check=True)
            print(f"built linzip from {repo} @ {head}", flush=True)
            hexed = "".join(l for l in args.huffman_table.read_text().splitlines()
                            if not l.startswith("#"))
            huff_tbl = pathlib.Path(tmp) / "huffman-table.bin"
            huff_tbl.write_bytes(bytes.fromhex(hexed.replace(" ", "")))

        def linzip_size(chunk, _dict_unused):
            out = pathlib.Path(tmp) / "linzip.out"
            subprocess.run([str(linzip), "-i", str(chunk), "-dict", str(args.dict),
                            "-table", str(huff_tbl), "-o", str(out)],
                           capture_output=True, check=True)
            return out.stat().st_size

        rows = []
        for payload in sorted(args.payloads.glob("*.payload.bin")):
            window = payload.name.removesuffix(".payload.bin")
            print(f"=== {window} ===", flush=True)
            chunk_dir = pathlib.Path(tmp) / window
            chunk_dir.mkdir()

            raw, lzss_real = lzss_chunked(payload, args.dict, chunk_dir)
            _, lzss_none = lzss_chunked(payload, empty_dict, pathlib.Path(tmp) / (window + "_z"))
            chunks = sorted(chunk_dir.glob("chunk_*.bin"))

            rows.append((window, "lzss(deployed)", args.dict.name, len(chunks), raw, lzss_real))
            rows.append((window, "lzss(deployed)", "none", len(chunks), raw, lzss_none))

            # (name, argv, supports an external dictionary)
            schemes = [
                ("zstd-19",   ["zstd", "-19", "-q"],   True),
                ("lz4-9",     ["lz4", "-9", "-q"],     True),
                ("brotli-11", ["brotli", "-q", "11"],  True),
                ("deflate-9", None,                    True),   # in-process zlib
                ("bzip2-9",   ["bzip2", "-9"],         False),  # BWT: no dictionary
            ]
            if linzip:
                schemes.append(("lzss+huffman", None, True))
            for scheme, cmd, takes_dict in schemes:
                variants = ([(args.dict.name, args.dict), ("none", None)]
                            if takes_dict else [("n/a", None)])
                if scheme == "lzss+huffman":
                    variants = [(args.dict.name, args.dict)]   # table is fixed
                for label, d in variants:
                    if scheme == "lzss+huffman":
                        sizer = linzip_size
                    elif cmd is None:
                        sizer = deflate_size
                    else:
                        sizer = (lambda c, dd, _cmd=cmd: compressed_size(_cmd, c, dd))
                    total = sum(sizer(c, d) for c in chunks)
                    rows.append((window, scheme, label, len(chunks), raw, total))

        # Dictionary status is the major key, scheme the minor one: adjacent rows
        # are then different schemes at the SAME dictionary setting, which is the
        # comparison you actually want to make by eye.
        # Group 0 is each scheme at its BEST available configuration; group 1 is
        # the dictionary ablation. bzip2 sits in group 0 with dictionary "n/a":
        # block sorting has no external-dictionary mechanism, so dictionary-less
        # IS its best configuration, and pairing it with everyone else's ablated
        # runs would understate it. It is absent from group 1 because it has no
        # dictionary contribution to ablate.
        #
        # Within a group, order by measured size rather than a fixed scheme list.
        # raw_bytes is constant per window, so ascending compressed bytes is
        # descending ratio, i.e. best first. A hardcoded order would also go stale
        # as schemes are added.
        DICT_ORDER = {"none": 1}
        rows.sort(key=lambda r: (r[0], DICT_ORDER.get(r[2], 0), r[5]))

        with args.out.open("w", newline="") as fh:
            w = csv.writer(fh)
            w.writerow(["window", "scheme", "dictionary", "chunks",
                        "raw_bytes", "compressed_bytes", "ratio"])
            for window, scheme, d, n, raw, comp in rows:
                w.writerow([window, scheme, d, n, raw, comp, f"{raw / comp:.4f}"])
    print(f"wrote {args.out}")


if __name__ == "__main__":
    sys.exit(main())
