#!/usr/bin/env python3
"""Per-field zero-run anatomy for blob payload streams.

Zero bytes are near-free for any LZ stage: a run of N zeros costs one match
reference regardless of N. So the share of a field in the RAW payload overstates
how much work it gives the compressor. The zero-free ("0-free") column is the
residue that LZ and the entropy coder actually have to model, and is the better
guide to where compressed bytes come from.

Emits CSV to stdout.
"""
import os, subprocess, sys, glob

FIELDS = ["payload", "calldata_raw", "calldata_irregular", "selectors",
          "structfields", "froms", "hashes", "meta"]

def zstd(path_or_bytes):
    if isinstance(path_or_bytes, bytes):
        p = subprocess.run(["zstd","-19","-q","-c"], input=path_or_bytes,
                           capture_output=True)
        return len(p.stdout)
    p = subprocess.run(["zstd","-19","-q","-c",path_or_bytes], capture_output=True)
    return len(p.stdout)

def zero_stats(d):
    z = d.count(0)
    runs, i, n = 0, 0, len(d)
    while i < n:
        if d[i] == 0:
            runs += 1
            while i < n and d[i] == 0:
                i += 1
        else:
            i += 1
    return z, runs

def main(stream_dirs):
    w = csv_writer = None
    print("window,field,raw_bytes,zero_bytes,zero_pct,zero_runs,mean_run,"
          "nonzero_bytes,share_of_raw_pct,share_of_nonzero_pct,"
          "zstd19_bytes,share_of_compressed_pct,zstd19_of_zerofree_bytes")
    for d in stream_dirs:
        window = os.path.basename(d).replace("st_", "")
        avail = [f for f in FIELDS if os.path.exists(os.path.join(d, f + ".bin"))]
        data = {f: open(os.path.join(d, f + ".bin"), "rb").read() for f in avail}
        # denominators exclude the aggregate 'payload' row
        parts = [f for f in avail if f not in ("payload", "calldata_irregular", "selectors")]
        tot_raw = sum(len(data[f]) for f in parts)
        tot_nz = sum(len(data[f]) - data[f].count(0) for f in parts)
        comp = {f: zstd(os.path.join(d, f + ".bin")) for f in avail}
        tot_comp = sum(comp[f] for f in parts)
        for f in avail:
            b = data[f]
            z, runs = zero_stats(b)
            nz = len(b) - z
            zf = bytes(x for x in b if x != 0)
            print(",".join(str(x) for x in [
                window, f, len(b), z,
                f"{100*z/len(b):.2f}" if b else "0",
                runs,
                f"{z/runs:.1f}" if runs else "0",
                nz,
                f"{100*len(b)/tot_raw:.2f}" if f in parts else "",
                f"{100*nz/tot_nz:.2f}" if f in parts and tot_nz else "",
                comp[f],
                f"{100*comp[f]/tot_comp:.2f}" if f in parts else "",
                zstd(zf),
            ]))

if __name__ == "__main__":
    main(sorted(glob.glob(sys.argv[1] + "/*")))
