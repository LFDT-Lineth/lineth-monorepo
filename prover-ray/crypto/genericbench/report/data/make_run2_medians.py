"""Builds paths-run2-medians.csv from the raw output of run 2.

Input (in this directory):
  medium-run2.txt  raw `go test -bench` output of run 2 (commit 6b3d84ad3,
                   medium scale, KoalaBear only)

Usage (from this directory): python3 make_run2_medians.py
"""
import csv

from make_tables import label, load_raw, sort_key

IMPLS = ['baseline', 'generic', 'generic-vec', 'generic-asm', 'current']


def main():
    raw = load_raw('medium-run2.txt')
    keys = sorted({(fam, key) for fam, key, _ in raw if 'field=koalabear' in key}, key=lambda k: sort_key(*k))
    with open('paths-run2-medians.csv', 'w', newline='') as f:
        w = csv.writer(f)
        w.writerow(['# KoalaBear medium-scale run 2 (commit 6b3d84ad3, before the gnark-crypto diagonal was replicated); medians in ns, from medium-run2.txt'])
        w.writerow(['benchmark', 'baseline', 'branches', 'generic-vec', 'generic-asm', 'current'])
        for fam, key in keys:
            vals = [raw.get((fam, key, i)) for i in IMPLS]
            w.writerow([label(fam, key)] + [f'{v[0]:.4g}' if v else '' for v in vals])
    print('wrote paths-run2-medians.csv')


if __name__ == '__main__':
    main()
