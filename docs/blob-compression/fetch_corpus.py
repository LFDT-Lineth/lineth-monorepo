#!/usr/bin/env python3
"""Download every evaluation window, verify it, and derive payload + streams.

The corpus is NOT kept in the repository. It is ~100 MB of binary, and this
repo's .gitattributes sets `* text eol=lf` with no `*.bin` exception, so git
strips every CR that precedes an LF on commit and silently corrupts it (we lost
61-111 bytes per file that way, which is enough to make the outer RLP
undecodable). Everything is reproducible from the block numbers below, so the
corpus lives outside the tree instead.

Windows are sized by PAYLOAD BYTES, not block count: traffic density varies
~10x across regimes, so a fixed block count would sample them wildly unequally.
15.6 MB is 20 blob-payloads at MaxUncompressedBytes (780 kB).
"""
import argparse
import pathlib
import shutil
import subprocess
import sys

TARGET_BYTES = 15_600_000
RPC = "https://rpc.linea.build"
PROVER_DIR = pathlib.Path(__file__).resolve().parents[2] / "prover"

# name -> (start block, label). Start blocks are fixed so the corpus is
# reproducible; none is anchored to the chain tip, which would make the window
# differ on every run.
WINDOWS = {
    "2025-09-25_busy":   (23_769_747, "busy"),
    "2025-01-06_median": (14_261_848, "median"),
    "2026-04-28_quiet":  (30_425_277, "quiet"),
    "2026-07-28_recent": (31_548_567, "recent"),
}


def go_run(tool, *args):
    subprocess.run(["go", "run", f"./cmd/dev-tools/{tool}", *map(str, args)],
                   cwd=PROVER_DIR, check=True)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus")
    ap.add_argument("--skip-verify", action="store_true",
                    help="skip per-block hash verification against the node")
    ap.add_argument("--only", help="fetch a single window by name")
    args = ap.parse_args()

    for sub in ("corpus", "payloads", "streams"):
        (args.out / sub).mkdir(parents=True, exist_ok=True)

    windows = WINDOWS if not args.only else {args.only: WINDOWS[args.only]}
    for name, (start, label) in windows.items():
        print(f"\n=== {name} (from block {start}) ===", flush=True)
        go_run("blob-anatomy", "--start", start, "--name", name,
               "--label", label, "--target-bytes", TARGET_BYTES,
               "--out", args.out / "corpus")

        corpus = args.out / "corpus" / f"{name}.bin"
        if not args.skip_verify:
            # txRoot, boundary scan, decode, per-field equality and every block
            # hash re-checked against the node.
            go_run("blob-roundtrip", "--rpc", RPC, corpus)

        streams = args.out / "streams" / name
        go_run("blob-streams", corpus, streams)
        shutil.copyfile(streams / "payload.bin",
                        args.out / "payloads" / f"{name}.payload.bin")

    print(f"\ncorpus:   {args.out / 'corpus'}")
    print(f"payloads: {args.out / 'payloads'}")
    print(f"streams:  {args.out / 'streams'}")


if __name__ == "__main__":
    sys.exit(main())
