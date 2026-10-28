"""Builds ../tab_kbb.tex and ../tab_paths.tex.

Inputs (in this directory):
  medium-diag.txt          raw `go test -bench` output of run 3 (medium scale,
                           gnark-crypto internal-matrix diagonal replicated)
  paths-run2-medians.csv   medians of run 2 (medium scale, before that change)

Usage (from this directory): python3 make_tables.py
"""
import collections
import csv
import re
import statistics

FAMILIES = ['Permutation', 'Compress', 'CompressParallel', 'MDHasher', 'FiatShamir']


def load_raw(path):
    """Returns {(label, impl): (median ns/op, median allocs/op)}."""
    d = collections.defaultdict(list)
    for line in open(path):
        if not line.startswith('Benchmark'):
            continue
        f = line.split()
        m = re.match(r'Benchmark(\w+)/(.*)/impl=([\w-]+?)-\d+$', f[0])
        fam, key, impl = m.groups()
        vals = dict(zip(f[3::2], f[2::2]))
        d[(fam, key, impl)].append((float(vals['ns/op']), float(vals['allocs/op'])))
    return {(fam, key, impl): (statistics.median(v[0] for v in vs), statistics.median(v[1] for v in vs))
            for (fam, key, impl), vs in d.items()}


def label(fam, key):
    key = re.sub(r'field=\w+/?', '', key).replace('update=', 'upd=').replace('/', ', ')
    return fam + (f' ({key})' if key else '')


def sort_key(fam, key):
    return FAMILIES.index(fam), [int(x) for x in re.findall(r'=(\d+)', key)]


def fmt_t(ns):
    for unit, scale in (('s', 1e9), ('ms', 1e6), (r'\textmu s', 1e3)):
        if ns >= scale:
            return f'{ns / scale:.3g}\\,{unit}'
    return f'{ns:.3g}\\,ns'


def fmt_r(x):
    if x >= 100:
        return f'{x:.0f}$\\times$'
    return f'{x:.1f}$\\times$' if x >= 10 else f'{x:.2f}$\\times$'


def fmt_a(a):
    return f'{a:,.0f}'.replace(',', '\\,')


def table(caption, lbl, header, rows):
    cols = 'l' + 'r' * (len(header) - 1)
    out = [r'\begin{table}[htbp]\centering\small', f'\\caption{{{caption}}}\\label{{{lbl}}}',
           r'\begin{adjustbox}{max width=\linewidth}', f'\\begin{{tabular}}{{{cols}}}', r'\toprule',
           ' & '.join(header) + r' \\ \midrule']
    out += [' & '.join(r) + r' \\' for r in rows]
    out += [r'\bottomrule', r'\end{tabular}', r'\end{adjustbox}', r'\end{table}']
    return '\n'.join(out) + '\n'


def main():
    raw = load_raw('medium-diag.txt')
    run2 = {r[0]: r for r in csv.reader(open('paths-run2-medians.csv')) if r and not r[0].startswith('#')}
    keys = sorted({(fam, key) for fam, key, _ in raw if 'field=koalabear' in key}, key=lambda k: sort_key(*k))

    # Table 3: the hot-loop branches before and after replicating gnark-crypto's
    # internal-matrix diagonal. Each ratio is against its own run's baseline.
    rows = []
    for fam, key in keys:
        lab = label(fam, key)
        b2, br2 = float(run2[lab][1]), float(run2[lab][2])
        b3, br3 = raw[(fam, key, 'baseline')][0], raw[(fam, key, 'generic')][0]
        rows.append([lab, fmt_t(b3), fmt_t(br2), fmt_r(br2 / b2), fmt_t(br3), fmt_r(br3 / b3),
                     fmt_a(raw[(fam, key, 'generic')][1])])
    open('../tab_kbb.tex', 'w').write(table(
        r"KoalaBear, the hot-loop branches before (run 2) and after (run 3) replicating gnark-crypto's internal-matrix diagonal. Each ratio is relative to the \texttt{baseline} of the same run; the baseline column shows run 3.",
        'tab:kbb',
        ['Benchmark', 'baseline', 'branches (initial)', 'ratio', 'branches, gnark diagonal', 'ratio',
         'allocs, gnark diagonal'], rows))

    # Table 4: every KoalaBear implementation in run 3.
    impls = [('generic', 'generic + hot-loop branches'), ('generic-vec', r'\texttt{generic-vec}'),
             ('generic-asm', r'\texttt{generic-asm}'), ('current', r'\texttt{current}')]
    header = ['Benchmark', 'baseline'] + [x for _, name in impls for x in (name, 'ratio')] + \
             [r'allocs \texttt{generic-asm}']
    rows = []
    for fam, key in keys:
        b = raw[(fam, key, 'baseline')][0]
        row = [label(fam, key), fmt_t(b)]
        for impl, _ in impls:
            v = raw.get((fam, key, impl))
            row += [fmt_t(v[0]), fmt_r(v[0] / b)] if v else ['--', '--']
        row.append(fmt_a(raw[(fam, key, 'generic-asm')][1]))
        rows.append(row)
    open('../tab_paths.tex', 'w').write(table(
        r'KoalaBear, all code paths side by side (run 3, \texttt{medium} scale). The hot-loop branches include the gnark-crypto internal-matrix diagonal.',
        'tab:paths', header, rows))
    print('wrote tab_kbb.tex, tab_paths.tex')


if __name__ == '__main__':
    main()
