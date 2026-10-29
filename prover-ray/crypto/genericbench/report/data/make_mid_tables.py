"""Builds ../tab_mid_kb.tex and ../tab_mid_mb.tex (run 1, fully generic code).

Input (in this directory):
  full-run1.txt  raw `go test -bench` output of run 1 (commit 3ce6cc6e5,
                 full scale minus FiatShamir rounds=4096/update=16384)

Usage (from this directory): python3 make_mid_tables.py
"""
from make_tables import fmt_a, fmt_r, fmt_t, label, load_raw, sort_key, table


def rows_for(raw, field, with_current):
    keys = sorted({(fam, key) for fam, key, _ in raw if f'field={field}' in key}, key=lambda k: sort_key(*k))
    rows = []
    for fam, key in keys:
        b, ba = raw[(fam, key, 'baseline')]
        g, ga = raw[(fam, key, 'generic')]
        row = [label(fam, key), fmt_t(b), fmt_t(g), fmt_r(g / b)]
        cur = raw.get((fam, key, 'current'))
        if with_current:
            row += [fmt_t(cur[0]), fmt_r(cur[0] / b)] if cur else ['--', '--']
        row += [fmt_a(ba), fmt_a(ga)]
        if with_current:
            row.append(fmt_a(cur[1]) if cur else '--')
        rows.append(row)
    return rows


def main():
    raw = load_raw('full-run1.txt')
    kb = table(r'KoalaBear, fully generic code (before any branching). Ratios are time relative to \texttt{baseline}.',
               'tab:mid-kb',
               ['Benchmark', 'baseline', 'fully generic', 'ratio', r'\texttt{current}', 'ratio',
                r'allocs \texttt{baseline}', 'allocs fully generic', r'allocs \texttt{current}'],
               rows_for(raw, 'koalabear', True))
    # keep the column grouping of the original tables
    kb = kb.replace('{lrrrrrrrr}', '{lrrrrr|rrr}')
    open('../tab_mid_kb.tex', 'w').write(kb)
    mb = table(r"MamaBear, fully generic code. The MamaBear baseline is gnark-crypto's (non-assembly) permutation under the generic hasher.",
               'tab:mid-mb',
               ['Benchmark', 'baseline', 'fully generic', 'ratio', r'allocs \texttt{baseline}', 'allocs fully generic'],
               rows_for(raw, 'mamabear', False))
    mb = mb.replace('{lrrrrr}', '{lrrr|rr}')
    open('../tab_mid_mb.tex', 'w').write(mb)
    print('wrote tab_mid_kb.tex, tab_mid_mb.tex')


if __name__ == '__main__':
    main()
