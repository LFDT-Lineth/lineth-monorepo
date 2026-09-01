#!/usr/bin/env python3
"""Dictionary versus window: can long back-references replace a maintained dictionary?

A dictionary is a protocol artifact with a lifecycle -- train it, ship it,
version it, decide when it has gone stale. A reference window is not: under a
straddling-stream design, where a block may span the end of one blob and the
start of the next, the previous blobs' plaintext IS the dictionary, updated for
free by the stream itself. This measures whether that substitution works, by
compressing every blob-sized chunk of every corpus era four ways:

  none        no dictionary, per-blob frames -- the floor.
  dict-2025   the deployed 64 KiB dictionary (prover/lib/compressor/dict).
  dict-fresh  a dictionary trained by `zstd --train` on THIS corpus. Deliberately
              in-sample: it cannot be beaten by any dictionary a real lifecycle
              would produce, so it bounds what dictionary maintenance can ever buy.
  prefix-N    no dictionary; instead the previous N MiB of plaintext are
              referenced as a raw-content prefix (ZSTD_CCtx_refPrefix, exposed by
              the CLI as --patch-from). A raw prefix carries no entropy tables and
              no dictID, so each frame remains a plain zstd frame that decodes
              independently given the prefix -- which is exactly the shape a
              per-blob proof consumes.

The comparison is not free-versus-free. Both a dictionary and a window are
private prover context that has to be bound to the proof, and today that means
hashing it. At the measured Poseidon2 rate the binding cost per decompressed
payload byte is

    kappa_ctx = context_bytes * KAPPA_HASH / chunk_bytes

which is ~0.8 c/B for the 64 KiB dictionary but ~12.8 c/B per MiB of prefix --
linear in exactly the quantity prefix-N grows. With --cycles this is measured
in the guest (Poseidon2 over the prefix) rather than derived from that rate.

Prefix sizes are in MiB, which is the unit that means something here: it maps
directly onto zstd's windowLog (1 MiB = wlog 20, 2 = 21, 4 = 22, 8 = 23) and
does not depend on the 780,000-byte chunking, which is an artefact of the
current blob-payload cap rather than a property of the data. So the table reports kappa_ctx
beside the ratio, under two proof granularities:

  per-blob   each blob proven separately; context is re-bound every proof.
  stream     one proof spans the whole stream; context never crosses a proof
             boundary and kappa_ctx is zero.

Which of those holds is a proof-architecture decision, not a compression one,
and it changes the answer completely -- hence both columns rather than a verdict.

The compression unit is --chunk-bytes (default 1 MiB), not the deployed
compressor's 780,000-byte cap: that cap is an artefact of the current design,
and the straddling stream removes it. The trailing partial chunk of each era is
dropped so every arm sees byte-identical units.
Per-chunk rows are written, not just totals, because the cold-start effect
matters: chunk 0 has no prefix to reference, so prefix-k's advantage should grow
over the first few chunks and then flatten. That is visible in the CSV and
summarised as "first" versus "steady" below.

Usage:
    python3 dictionary_vs_window.py            # reproduces results/dictionary-vs-window.csv
    python3 dictionary_vs_window.py --cycles   # + reproduces the guest cycle table
                                                # (~20 min: 9 configs, 6 at a time)

Every argument below has a default that reproduces the committed result;
override individual ones to explore, e.g.:
    python3 dictionary_vs_window.py --eras 2026-07-28_recent --level 19
    python3 dictionary_vs_window.py --chunk-bytes 131072   # fast iteration
    python3 dictionary_vs_window.py --no-sweep --cycles --cycles-configs prefix-1MiB
"""
import argparse
import csv
import pathlib
import re
import concurrent.futures
import shutil
import subprocess
import sys
import tempfile

MIB = 1024 * 1024
KIB = 1024
# The compression unit. NOT 780,000: MaxUncompressedBytes is a cap in the
# currently deployed compressor, not a property of the data or of the
# straddling-stream design, so it is a parameter with a round default rather
# than a constant. Shrink it (--chunk-bytes) for fast iteration; cycles/byte and
# ratio are both intensive, so a smaller unit measures the same quantities, it
# just amortises per-frame overhead over less data.
DEFAULT_CHUNK = MIB
KAPPA_HASH = 9.50        # cycles/byte, Poseidon2 over the payload (bench_hash)

# Everything below is the exact configuration that produced
# results/dictionary-vs-window.csv, so `python3 dictionary_vs_window.py` with NO
# arguments reproduces it, and `--cycles` additionally reproduces the guest
# cycle table. Override any of these to explore; the point is that exploring is
# opt-in and reproducing the committed result is not -- it should never require
# reconstructing a long invocation from memory or from chat history.
CANONICAL_ERAS = [
    "2025-01-06_median", "2025-09-25_busy", "2026-04-28_quiet", "2026-07-28_recent",
]
CANONICAL_PREFIX_MIB = [1, 2]  # 4 and 8 MiB were dropped: diminishing returns were
                               # already clear by 2 MiB, and each MiB roughly
                               # doubles the --cycles wall time (context hashing
                               # is linear in context size).
CANONICAL_PREFIX_KIB = [512]  # Added to test whether most of 1 MiB's ratio gain
                              # already shows up at half the binding cost.
CANONICAL_COMBO = ["dict-fresh", "dict-2025"]
CANONICAL_CYCLES_ERA = "2026-07-28_recent"
CANONICAL_CYCLES_CONFIGS = [
    "none", "dict-2025", "dict-fresh", "prefix-1MiB", "prefix-2MiB",
    "prefix-1MiB+dict-fresh", "prefix-1MiB+dict-2025",
    "prefix-2MiB+dict-fresh", "prefix-2MiB+dict-2025",
]

REPO = pathlib.Path(__file__).resolve().parents[2]
DEFAULT_DICT = REPO / "prover" / "lib" / "compressor" / "dict" / "25-04-21.bin"
BENCH_ZSTD_C = REPO / "verifier-ray" / "bench" / "bench_zstd_c"


def run(argv, **kw):
    return subprocess.run(argv, check=True, capture_output=True, **kw)


def chunk_payload(payload, outdir, chunk):
    """Split into fixed-size chunks, dropping the trailing partial one."""
    data = payload.read_bytes()
    paths = []
    for i in range(len(data) // chunk):
        p = outdir / f"c{i:03d}.bin"
        p.write_bytes(data[i * chunk:(i + 1) * chunk])
        paths.append(p)
    return paths


def zstd_size(chunk, level, dict_path=None, prefix=None):
    argv = ["zstd", f"-{level}", "-q", "-c"]
    if dict_path:
        argv += ["-D", str(dict_path)]
    if prefix:
        argv += [f"--patch-from={prefix}"]
    argv += [str(chunk)]
    return len(run(argv).stdout)


def train_dictionary(chunks, out, size=65536):
    """Train on the corpus being measured -- in-sample on purpose (see docstring).

    zstd --train wants many samples; the 780 kB chunks are split into 16 kB
    pieces so the trainer sees enough of them to converge.
    """
    with tempfile.TemporaryDirectory() as td:
        td = pathlib.Path(td)
        n = 0
        for c in chunks:
            data = c.read_bytes()
            for off in range(0, len(data), 16384):
                (td / f"s{n:06d}").write_bytes(data[off:off + 16384])
                n += 1
        run(["zstd", "--train", "-q", f"--maxdict={size}", "-o", str(out)]
            + [str(p) for p in sorted(td.iterdir())])
    return out


def prefix_sizes(prefix_mib, prefix_kib):
    """(label, bytes) pairs, largest first within each unit so cold-start /
    steady-state tables print in a sensible order."""
    for m in sorted(prefix_mib, reverse=True):
        yield f"{m}MiB", m * MIB
    for k in sorted(prefix_kib, reverse=True):
        yield f"{k}KiB", k * KIB


def parse_prefix_bytes(cfg):
    """"prefix-512KiB" or "prefix-1MiB+dict-fresh" -> size in bytes."""
    token = cfg[len("prefix-"):]
    for unit, mult in (("KiB", KIB), ("MiB", MIB)):
        if unit in token:
            return int(token.split(unit, 1)[0]) * mult
    raise ValueError(f"no size unit in {cfg!r}")


def arms(prefix_mib, combo_dicts=(), prefix_kib=()):
    yield "none", None, 0
    yield "dict-2025", "dict-2025", 0
    yield "dict-fresh", "dict-fresh", 0
    for label, nbytes in prefix_sizes(prefix_mib, prefix_kib):
        yield f"prefix-{label}", None, nbytes
    # Synergy test: does a (possibly cheating) dictionary add anything a
    # window does not already cover? -D and --patch-from are mutually
    # exclusive in zstd's API (a loaded dictionary and a prefix share the same
    # internal slot -- confirmed directly: `-D x --patch-from=y` errors "can't
    # use -D and --patch-from=# at the same time"), so this is not literally
    # both mechanisms active at once. Instead the dictionary's serialized bytes
    # (magic + entropy tables + content section) are concatenated onto the
    # window and the whole thing goes through the single prefix path -- a raw
    # prefix is just "history to preload", and only the content section
    # provides useful match material, but that is most of a zstd dictionary's
    # bytes. This asks the question that matters: is there information in the
    # dictionary that recent history does not already contain?
    for dname in combo_dicts:
        for label, nbytes in prefix_sizes(prefix_mib, prefix_kib):
            yield f"prefix-{label}+{dname}", dname, nbytes


def sweep(eras, payload_dir, dict_2025, level, prefix_mib, chunk, rows, args_combo=(), prefix_kib=()):
    for era in eras:
        payload = payload_dir / f"{era}.payload.bin"
        if not payload.exists():
            print(f"  ! missing {payload}, skipping", file=sys.stderr)
            continue
        with tempfile.TemporaryDirectory() as td:
            td = pathlib.Path(td)
            chunks = chunk_payload(payload, td, chunk)
            fresh = train_dictionary(chunks, td / "fresh.dict")
            dicts = {"dict-2025": dict_2025, "dict-fresh": fresh}
            print(f"{era}: {len(chunks)} chunks", flush=True)

            for name, dict_key, want in arms(prefix_mib, args_combo, prefix_kib):
                total = 0
                for i, c in enumerate(chunks):
                    if want:
                        # Exactly `want` bytes of immediately preceding plaintext,
                        # independent of the chunk boundary. Cold start is kept:
                        # early chunks have less history than that available.
                        ctx = td / "prefix.bin"
                        avail = i * chunk
                        take = min(want, avail)
                        if take:
                            buf = bytearray()
                            j = i - 1
                            while len(buf) < take and j >= 0:
                                buf[:0] = chunks[j].read_bytes()
                                j -= 1
                            window = bytes(buf[-take:])
                        else:
                            window = b""
                        if dict_key:
                            # Dictionary bytes further back, window bytes right
                            # before the target: the window gets the cheap near
                            # offsets, the dictionary is still reachable behind it.
                            d = dicts.get(dict_key)
                            ctx.write_bytes((d.read_bytes() if d else b"") + window)
                        else:
                            ctx.write_bytes(window)
                        ctx_bytes = ctx.stat().st_size
                        size = zstd_size(c, level, prefix=ctx) if ctx_bytes else \
                            zstd_size(c, level)
                    else:
                        d = dicts.get(dict_key)
                        ctx_bytes = d.stat().st_size if d else 0
                        size = zstd_size(c, level, dict_path=d)
                    total += size
                    rows.append(dict(era=era, arm=name, chunk=i, raw=chunk,
                                     compressed=size, ctx_bytes=ctx_bytes))
                print(f"  {name:<12} {total:>10,}  ratio {len(chunks)*chunk/total:.4f}",
                      flush=True)


def summarise(rows, prefix_mib, chunk, args_combo=(), prefix_kib=()):
    eras = sorted({r["era"] for r in rows})
    names = [n for n, _, _ in arms(prefix_mib, args_combo, prefix_kib)]

    print(f"\n{'arm':<12}" + "".join(f"{e.split('_')[1][:8]:>10}" for e in eras)
          + f"{'aggregate':>11}{'ctx_B':>10}{'k_ctx':>8}{'vs none':>9}")
    print("-" * (12 + 10 * len(eras) + 38))

    base = None
    for name in names:
        sel = [r for r in rows if r["arm"] == name]
        if not sel:
            continue
        agg = sum(r["raw"] for r in sel) / sum(r["compressed"] for r in sel)
        if name == "none":
            base = agg
        # Steady-state context size, not the cold-start average.
        ctx = max(r["ctx_bytes"] for r in sel)
        k_ctx = ctx * KAPPA_HASH / chunk
        per_era = []
        for e in eras:
            s = [r for r in sel if r["era"] == e]
            per_era.append(sum(r["raw"] for r in s) / sum(r["compressed"] for r in s)
                           if s else float("nan"))
        gain = f"{100*(agg/base - 1):+.2f}%" if base else "-"
        print(f"{name:<12}" + "".join(f"{v:>10.4f}" for v in per_era)
              + f"{agg:>11.4f}{ctx:>10,}{k_ctx:>8.2f}{gain:>9}")

    # Cold start: prefix arms cannot reference what has not been produced yet.
    print(f"\ncold start (compressed bytes, {eras[0]}):")
    print(f"  {'arm':<12}{'chunk 0':>10}{'chunk 1':>10}{'steady (last 5 avg)':>22}")
    for name in names:
        sel = sorted((r for r in rows if r["arm"] == name and r["era"] == eras[0]),
                     key=lambda r: r["chunk"])
        if len(sel) < 6:
            continue
        steady = sum(r["compressed"] for r in sel[-5:]) / 5
        print(f"  {name:<12}{sel[0]['compressed']:>10,}{sel[1]['compressed']:>10,}"
              f"{steady:>22,.0f}")

    print("\nkappa_ctx is the cost of binding private prover context by hashing it,")
    print(f"at {KAPPA_HASH} c/B over the {chunk:,}-byte compression unit;")
    print("  per-blob proofs : add k_ctx above to kappa_dec")
    print("  one proof/stream: k_ctx is 0; context never crosses a proof boundary")


def build_fixture(cfg, chunks, idx, dict_2025, fresh, level, dest):
    """Write the three generated files a guest run needs into `dest`.

    The guest passes prefix.bin to ZSTD_decompress_usingDict as the decoding
    context, and that one argument covers both context kinds: a raw multi-MiB
    prefix (dct_auto sees no magic -> rawContent) and a trained dictionary
    (magic 0xEC30A437 -> full dictionary with entropy tables). So the dictionary
    arms must ALSO write their dictionary here -- compressing with -D while
    leaving prefix.bin empty just makes the guest fail to decode, which is what
    silently broke the first sweep.
    """
    target = chunks[idx]
    ctx = b""
    if cfg.startswith("prefix-"):
        # "prefix-512KiB" or "prefix-4MiB+dict-fresh"; parse_prefix_bytes strips
        # any trailing "+dict-..." implicitly since it only looks at the size unit.
        want = parse_prefix_bytes(cfg)
        buf = bytearray()
        j = idx - 1
        while len(buf) < want and j >= 0:
            buf[:0] = chunks[j].read_bytes()
            j -= 1
        window = bytes(buf[-want:])
        # Same synergy construction as the ratio sweep: dictionary bytes
        # further back, window immediately before the target.
        dict_bytes = b""
        if "+dict-fresh" in cfg:
            dict_bytes = fresh.read_bytes()
        elif "+dict-2025" in cfg:
            dict_bytes = dict_2025.read_bytes()
        ctx = dict_bytes + window
        (dest / "prefix.bin").write_bytes(ctx)
        comp = run(["zstd", f"-{level}", "-q", "-c",
                    f"--patch-from={dest / 'prefix.bin'}", str(target)]).stdout
    else:
        d = {"dict-2025": dict_2025, "dict-fresh": fresh}.get(cfg)
        argv = ["zstd", f"-{level}", "-q", "-c"]
        if d:
            argv += ["-D", str(d)]
            ctx = d.read_bytes()
        comp = run(argv + [str(target)]).stdout
        (dest / "prefix.bin").write_bytes(ctx)

    plaintext = target.read_bytes()
    h = 0xcbf29ce484222325
    for b in plaintext:
        h = ((h ^ b) * 0x100000001b3) & 0xFFFFFFFFFFFFFFFF
    (dest / "zstd_compressed.bin").write_bytes(comp)
    (dest / "fixture_params.zig").write_text(
        "// Regenerated by docs/blob-compression/dictionary_vs_window.py --cycles.\n"
        f"// arm {cfg}, chunk {idx}, zstd -{level}, context {len(ctx):,} B.\n"
        f"pub const decompressed_len: usize = {len(plaintext)};\n"
        f"pub const expected_fnv1a: u64 = {h};\n")
    return len(comp), len(ctx)


RESULT_RE = re.compile(r"^zstd.*?\s+(\d+)\s+(\d+)\s+([\d.]+)\s", re.M)
CTX_RE = re.compile(r"context hash\s+\S+\s+(\d+)\s+([\d.]+)")
# Binding the decompressed OUTPUT is mandatory for every arm -- it is what the
# DA circuit actually commits to -- and identical in cost across arms, so it is
# measured once per run rather than assumed from bench_hash's rate.
FIX_RE = re.compile(r"output hash\s+\S+\s+(\d+)\s+([\d.]+)")


def run_one(cfg, workdir, zkc, logdir):
    """One guest run in its own copy of the bench, so runs can go in parallel."""
    out = subprocess.run(["go", "run", "run.go", zkc], cwd=workdir,
                         capture_output=True, text=True)
    # Always keep the full transcript: the first sweep hid two failures because
    # only the parsed line was logged.
    (logdir / f"{cfg}.log").write_text(out.stdout + "\n--- stderr ---\n" + out.stderr)
    m = RESULT_RE.search(out.stdout)
    if not m:
        return None
    ctx = CTX_RE.search(out.stdout)
    fix = FIX_RE.search(out.stdout)
    return (float(m.group(3)),
            float(ctx.group(2)) if ctx else float("nan"),
            float(fix.group(2)) if fix else float("nan"))


def measure_cycles(era, chunk_index, payload_dir, dict_2025, level, configs, zkc,
                   jobs, logdir, chunk):
    """Measure guest decode and context-binding cycles, one process per config.

    Each config gets its own copy of bench_zstd_c because they would otherwise
    fight over the same fixture files, zig-out and build cache. zkc is
    single-threaded at ~770 MB, so the copies run concurrently.
    """
    payload = payload_dir / f"{era}.payload.bin"
    logdir.mkdir(parents=True, exist_ok=True)
    results = []
    with tempfile.TemporaryDirectory() as td:
        td = pathlib.Path(td)
        chunks = chunk_payload(payload, td, chunk)
        fresh = train_dictionary(chunks, td / "fresh.dict")
        idx = min(chunk_index, len(chunks) - 1)
        print(f"\nguest runs: {era} chunk {idx}, {len(configs)} configs, "
              f"{jobs} at a time", flush=True)

        # The copies must be SIBLINGS of bench_zstd_c: build.zig.zon reaches its
        # dependencies by relative path (../../../riscv-guests/...), so a copy
        # anywhere else fails to resolve build_common.
        work = {}
        try:
            for cfg in configs:
                dest = BENCH_ZSTD_C.parent / f".par-{cfg}"
                if dest.exists():
                    shutil.rmtree(dest)
                shutil.copytree(BENCH_ZSTD_C, dest,
                                ignore=shutil.ignore_patterns("zig-out", ".zig-cache", "bench"))
                csize, ctxlen = build_fixture(cfg, chunks, idx, dict_2025, fresh,
                                              level, dest)
                work[cfg] = (dest, csize, ctxlen)

            with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
                futs = {pool.submit(run_one, cfg, d, zkc, logdir): cfg
                        for cfg, (d, _, _) in work.items()}
                for fut in concurrent.futures.as_completed(futs):
                    cfg = futs[fut]
                    _, csize, ctxlen = work[cfg]
                    r = fut.result()
                    if r is None:
                        print(f"  {cfg:<14} FAILED (see {logdir / (cfg + '.log')})",
                              flush=True)
                        continue
                    k_dec, k_ctx, k_fix = r
                    k_total = k_dec + k_ctx + k_fix
                    print(f"  {cfg:<14} k_dec {k_dec:6.2f}  k_ctx {k_ctx:7.2f}"
                          f"  k_fix {k_fix:6.2f}  k_total {k_total:7.2f}", flush=True)
                    results.append((cfg, csize, ctxlen, k_dec, k_ctx, k_fix))
        finally:
            for _, (dest, _, _) in work.items():
                shutil.rmtree(dest, ignore_errors=True)
    order = {c: i for i, c in enumerate(configs)}
    results.sort(key=lambda r: order[r[0]])
    return results


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--payloads", type=pathlib.Path,
                    default=pathlib.Path.home() / "linea-blob-corpus" / "payloads")
    ap.add_argument("--eras", nargs="*", default=None,
                    help="era names; default is the 4 eras behind the committed "
                         "result (CANONICAL_ERAS), not everything in --payloads, "
                         "so an extra file dropped into the corpus dir later "
                         "doesn't silently change what a bare invocation reproduces")
    ap.add_argument("--dict", type=pathlib.Path, default=DEFAULT_DICT)
    ap.add_argument("--level", type=int, default=19)
    ap.add_argument("--chunk-bytes", type=int, default=DEFAULT_CHUNK,
                    help="bytes compressed per frame (the compression unit). "
                         "Shrink for fast iteration, e.g. --chunk-bytes 131072")
    ap.add_argument("--prefix-mib", type=int, nargs="*", default=CANONICAL_PREFIX_MIB,
                    help="prefix sizes in MiB (1 MiB = windowLog 20, 2 = 21, ...)")
    ap.add_argument("--prefix-kib", type=int, nargs="*", default=CANONICAL_PREFIX_KIB,
                    help="prefix sizes in KiB, for sub-MiB points")
    ap.add_argument("--combo", nargs="*", default=CANONICAL_COMBO,
                    choices=["dict-fresh", "dict-2025"],
                    help="also test window+dictionary together, for the named "
                         "dictionaries (see arms()). Pass --combo with no names "
                         "to disable.")
    ap.add_argument("--out", type=pathlib.Path,
                    default=pathlib.Path(__file__).parent / "results" / "dictionary-vs-window.csv")
    ap.add_argument("--no-sweep", action="store_true",
                    help="skip the host ratio sweep and only run --cycles; the "
                         "ratios do not change between cycle runs, so re-measuring "
                         "them costs ~15 min for nothing")
    ap.add_argument("--cycles", action="store_true",
                    help="also measure guest decode cycles via bench_zstd_c")
    ap.add_argument("--cycles-configs", nargs="*", default=CANONICAL_CYCLES_CONFIGS)
    ap.add_argument("--cycles-era", default=CANONICAL_CYCLES_ERA,
                    help="era to build the guest fixture from")
    ap.add_argument("--cycles-chunk", type=int, default=12,
                    help="which chunk of that era to decode in the guest")
    ap.add_argument("--jobs", type=int, default=6,
                    help="guest runs in parallel; each zkc is single-threaded, ~770 MB, "
                         "so 6 fits comfortably on a 10-core/32 GB machine")
    ap.add_argument("--zkc", default=str(REPO / "prover" / "bin" / "zkc"))
    args = ap.parse_args()

    if args.eras is None:
        args.eras = CANONICAL_ERAS
    args.dict = args.dict.resolve()
    args.out.parent.mkdir(parents=True, exist_ok=True)

    print(f"level {args.level}, chunk {args.chunk_bytes:,} B, dict {args.dict.name} "
          f"({args.dict.stat().st_size:,} B)\n")

    rows = []
    if not args.no_sweep:
        sweep(args.eras, args.payloads.resolve(), args.dict, args.level,
              args.prefix_mib, args.chunk_bytes, rows, args.combo, args.prefix_kib)
        if not rows:
            sys.exit("no data; check --payloads")

    if rows:
      with args.out.open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=["era", "arm", "chunk", "raw",
                                           "compressed", "ctx_bytes"])
        w.writeheader()
        w.writerows(rows)
      summarise(rows, args.prefix_mib, args.chunk_bytes, args.combo, args.prefix_kib)
      print(f"\nCSV written to {args.out}")

    if args.cycles:
        res = measure_cycles(args.cycles_era, args.cycles_chunk,
                             args.payloads.resolve(), args.dict, args.level,
                             args.cycles_configs, args.zkc, args.jobs,
                             args.out.parent / "guest-logs", args.chunk_bytes)
        if res:
            print(f"\n{'arm':<14}{'compressed_B':>13}{'ctx_B':>12}"
                  f"{'k_dec':>8}{'k_ctx':>8}{'k_fix':>8}{'k_total':>9}")
            for cfg, size, ctxb, k_dec, k_ctx, k_fix in res:
                k_total = k_dec + k_ctx + k_fix
                print(f"{cfg:<14}{size:>13,}{ctxb:>12,}{k_dec:>8.2f}"
                      f"{k_ctx:>8.2f}{k_fix:>8.2f}{k_total:>9.2f}")
            print("\nk_dec: decode. k_ctx: binding the context (dictionary or window),"
                  "\nmeasured in-guest; 0 under one proof/stream, since context then never"
                  "\ncrosses a proof boundary. k_fix: binding the decompressed output, "
                  "\nmandatory and identical for every arm regardless of proof granularity."
                  "\nk_total = k_dec + k_ctx + k_fix, i.e. the per-blob-proof total.")


if __name__ == "__main__":
    main()
