#!/usr/bin/env python3
"""Fill real blobs and report payload bytes per blob.

The other table compresses fixed-size uncompressed chunks. Production does the
opposite: it adds L2 blocks until the COMPRESSED output no longer fits in a
blob. That difference matters twice over.

  * Fixed 780 kB chunks are not self-consistent. On `recent` they report a ratio
    implying ~537 kB of payload per blob, but compressing 537 kB yields a lower
    ratio, so only ~492 kB actually fits. The fixed-input numbers are ~9% high.

  * `payload bytes per blob` is exactly the `rho * B` term the cost model in
    ../blob-compression-evaluation-plan.md consumes, so measuring it directly
    removes a multiplication and the assumption behind it.

Method. The search is over BLOCK counts, not bytes: production cannot split a
block across blobs, so block count is the real decision variable and the fit
predicate is monotone in it. Every probe is a ONE-SHOT compression -- the frame
production would actually emit. Streaming APIs were rejected deliberately: the
flush needed to observe output size forces block boundaries and changes the
frame, and it would flatter LZSS (whose Len()/Revert() are built for exactly
this) over everything else.

Budget. blob_maker.go checks PackAlignSize(header + compressed, 254) <= 131072.
Inverting the 254-bits-per-32-byte-element packing gives 130048 bytes for header
plus compressed body, not 131072.
"""
import argparse
import csv
import pathlib
import statistics
import subprocess
import sys
import tempfile
import zlib

PROVER_DIR = pathlib.Path(__file__).resolve().parents[2] / "prover"
BLOB_USABLE = 4096 * 254 // 8            # 130048: header + compressed body
HEADER_BYTES = 2 + 32 + 2 + 3            # version, dictChecksum, nbBatches, one batch length
BUDGET = BLOB_USABLE - HEADER_BYTES

# NOT applied: MaxUncompressedBytes (780 kB). It is a capacity limit of the
# current hand-arithmetized Plonk circuit, which this whole evaluation exists to
# replace. Capping the measurement at it would bake in a constraint we are in
# the process of removing, and would understate exactly the schemes that
# compress best. Blob capacity is the only bound applied here.

# Seed for the galloping search. This only affects how many probes are needed,
# never the result. Set it from the mean this script reports.
INITIAL_BLOCKS_PER_BLOB = 300


def make_sizers(dict_path, tmp):
    """Return {scheme: fn(bytes) -> compressed length}, all one-shot."""
    lzss_bin = pathlib.Path(tmp) / "lzss-size"
    subprocess.run(["go", "build", "-o", str(lzss_bin), "./cmd/dev-tools/lzss-size"],
                   cwd=PROVER_DIR, check=True)
    slice_path = pathlib.Path(tmp) / "slice.bin"
    dict_bytes = dict_path.read_bytes()

    def cli(cmd, use_dict=True):
        def fn(buf):
            argv = list(cmd) + (["-D", str(dict_path)] if use_dict else []) + ["-c"]
            return len(subprocess.run(argv, input=buf, capture_output=True,
                                      check=True).stdout)
        return fn

    def lzss(buf):
        slice_path.write_bytes(buf)
        out = subprocess.run([str(lzss_bin), str(slice_path), str(dict_path)],
                             capture_output=True, text=True, check=True).stdout
        return int(out.strip())

    def deflate(buf):
        co = zlib.compressobj(level=9, method=zlib.DEFLATED, wbits=-15,
                              memLevel=9, zdict=dict_bytes)
        return len(co.compress(buf) + co.flush())

    return {
        "brotli-11": cli(["brotli", "-q", "11"]),
        "zstd-19": cli(["zstd", "-19", "-q"]),
        "lzss(deployed)": lzss,
        "bzip2-9": cli(["bzip2", "-9"], use_dict=False),   # no dictionary mechanism
        "lz4-9": cli(["lz4", "-9", "-q"]),
        "deflate-9": deflate,
    }


def fill_one_blob(payload, ends, first_block, sizer, seed, probes):
    """Largest block count from first_block whose compressed payload fits.

    Gallops out from `seed`, then bisects. Returns (n_blocks, payload_bytes,
    compressed_bytes) or None if even one block overflows.
    """
    base = ends[first_block - 1] if first_block else 0
    remaining = len(ends) - first_block
    if remaining <= 0:
        return None

    def payload_len(n):
        return ends[first_block + n - 1] - base

    def fits(n):
        probes[0] += 1
        return sizer(payload[base:ends[first_block + n - 1]]) <= BUDGET

    def size_of(n):
        probes[0] += 1
        return sizer(payload[base:ends[first_block + n - 1]])

    if not fits(1):
        return None

    lo = 1                                    # known to fit
    hi = None                                 # known not to fit
    n = max(1, min(seed, remaining))
    while True:
        if fits(n):
            lo = n
            if n == remaining:
                hi = remaining + 1
                break
            n = min(remaining, n * 2) if hi is None else n
            if hi is not None:
                break
        else:
            hi = n
            break
        if n == lo:                            # doubling did not move us
            break

    if hi is None:
        hi = remaining + 1
    while lo + 1 < hi:                        # bisect the monotone predicate
        mid = (lo + hi) // 2
        if fits(mid):
            lo = mid
        else:
            hi = mid

    return lo, payload_len(lo), size_of(lo)


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--streams", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "streams")
    ap.add_argument("--dict", type=pathlib.Path, required=True)
    ap.add_argument("--out", type=pathlib.Path,
                    default=pathlib.Path(__file__).parent / "results" / "blob-fill.csv")
    ap.add_argument("--max-blobs", type=int, default=8,
                    help="blobs per window per scheme; brotli -q11 is slow")
    args = ap.parse_args()
    args.dict = args.dict.resolve()
    args.out = args.out.resolve()
    args.out.parent.mkdir(parents=True, exist_ok=True)

    rows = []
    with tempfile.TemporaryDirectory() as tmp:
        sizers = make_sizers(args.dict, tmp)
        for wdir in sorted(p for p in args.streams.iterdir() if p.is_dir()):
            payload = (wdir / "payload.bin").read_bytes()
            ends = [int(x) for x in (wdir / "block_ends.txt").read_text().split()]
            print(f"=== {wdir.name}: {len(ends)} blocks, {len(payload):,} B ===",
                  flush=True)
            for scheme, sizer in sizers.items():
                blk = 0
                seed = INITIAL_BLOCKS_PER_BLOB
                probes = [0]
                per_blob = []
                for _ in range(args.max_blobs):
                    got = fill_one_blob(payload, ends, blk, sizer, seed, probes)
                    if got is None:
                        break
                    n, pbytes, cbytes = got
                    per_blob.append((n, pbytes, cbytes))
                    blk += n
                    seed = n           # next blob starts from the last answer
                if not per_blob:
                    continue
                mean_payload = statistics.mean(p for _, p, _ in per_blob)
                mean_blocks = statistics.mean(n for n, _, _ in per_blob)
                print(f"  {scheme:16} {len(per_blob):>2} blobs  "
                      f"payload/blob {mean_payload:>9,.0f}  blocks/blob {mean_blocks:>7.1f}  "
                      f"({probes[0]} probes)", flush=True)
                for i, (n, pbytes, cbytes) in enumerate(per_blob):
                    rows.append([wdir.name, scheme, i, n, pbytes, cbytes,
                                 f"{pbytes / cbytes:.4f}"])

    with args.out.open("w", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(["window", "scheme", "blob_index", "blocks", "payload_bytes",
                    "compressed_bytes", "ratio"])
        w.writerows(rows)

    allblocks = [r[3] for r in rows]
    print(f"\nwrote {args.out}")
    if allblocks:
        print(f"mean blocks/blob across everything: {statistics.mean(allblocks):.0f} "
              f"(INITIAL_BLOCKS_PER_BLOB is {INITIAL_BLOCKS_PER_BLOB}; "
              f"set it to this to cut probe counts)")


if __name__ == "__main__":
    sys.exit(main())
