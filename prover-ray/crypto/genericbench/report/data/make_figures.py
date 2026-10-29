"""Builds the pgfplots figures ../fig_*.tex from the tables ../tab_*.tex and the
CompressParallel scaling runs in this directory.

The tab_*.tex files hold medians rounded to three significant figures; the
ratios plotted are recomputed from them.

Usage (from this directory): python3 make_figures.py
"""
import csv
import re

import make_tables

from make_parallel_table import load

# Categorical slots 1-5 of the dataviz reference palette (light mode), in
# fixed order.
COLORS = {'c1': '2a78d6', 'c2': 'eb6834', 'c3': '1baf7a', 'c4': 'eda100', 'c5': '8f5bd6'}


def parse_time(cell):
    m = re.match(r'([\d.]+)\\,(ns|\\textmu s|ms|s)$', cell.strip())
    if not m:
        return None
    scale = {'ns': 1, r'\textmu s': 1e3, 'ms': 1e6, 's': 1e9}[m.group(2)]
    return float(m.group(1)) * scale


def parse_table(path):
    """Returns (column names, rows) where each row is (label, {column: ns})."""
    lines = open(path).read().splitlines()
    header = next(l for l in lines if l.startswith('Benchmark & '))
    cols = [c.strip() for c in header.split(r'\\')[0].split('&')]
    rows = []
    start = lines.index(header) + 1
    for line in lines[start:]:
        if line.startswith(r'\bottomrule'):
            break
        cells = [c.strip() for c in line.rstrip('\\ ').split('&')]
        times = {}
        for name, cell in zip(cols[1:], cells[1:]):
            if name == 'ratio' or name.startswith('allocs'):
                continue
            t = parse_time(cell)
            if t is not None:
                times[name] = t
        rows.append((cells[0], times))
    return cols, rows


def bar_figure(path, caption, label, rows, series, xmax):
    """Horizontal grouped bars of time/baseline for each row, log x axis."""
    n = len(rows)
    ticks = ','.join(str(i) for i in range(n))
    labels = ','.join('{' + r + '}' for r, _ in rows)
    plots = []
    for col, legend, color in series:
        coords = ' '.join(f'({t[col] / t["baseline"]:.4g},{n - 1 - i})'
                          for i, (_, t) in enumerate(rows) if col in t)
        plots.append(rf'\addplot+[fill={color}, draw=none] coordinates {{{coords}}};'
                     + '\n' + rf'\addlegendentry{{{legend}}}')
    bar_w = {1: 7, 2: 4.5, 3: 3.2, 4: 2.6, 5: 2.1}[len(series)]
    out = rf'''\begin{{figure}}[htbp]\centering
\begin{{tikzpicture}}
\begin{{axis}}[
  xbar, xmode=log, log basis x=10,
  width=0.74\linewidth, height={0.42 * n + 1.2:.1f}cm,
  bar width={bar_w}pt, bar shift auto, enlarge y limits={{abs=0.6}},
  ytick={{{ticks}}}, yticklabels={{{','.join('{' + r + '}' for r, _ in reversed(rows))}}},
  y tick label style={{font=\scriptsize}}, x tick label style={{font=\scriptsize}},
  xmin=0.5, xmax={xmax},
  xtick={{0.5,1,2,5,10,20,50,100,200,500}},
  xticklabels={{0.5$\times$,1$\times$,2$\times$,5$\times$,10$\times$,20$\times$,50$\times$,100$\times$,200$\times$,500$\times$}},
  xlabel={{time relative to \texttt{{baseline}} (log scale, lower is better)}},
  xlabel style={{font=\small}},
  xmajorgrids, major grid style={{gray!20}}, axis line style={{gray!60}},
  ytick style={{draw=none}},
  legend style={{font=\scriptsize, draw=none, fill=none, at={{(0.5,1.01)}}, anchor=south, legend columns=-1}},
  legend image code/.code={{\draw[#1, draw=none] (0cm,-0.08cm) rectangle (0.3cm,0.08cm);}},
  clip=false,
]
\draw[gray!80, thick] (axis cs:1,-0.6) -- (axis cs:1,{n - 0.4});
{chr(10).join(plots)}
\end{{axis}}
\end{{tikzpicture}}
\caption{{{caption}}}\label{{{label}}}
\end{{figure}}
'''
    del labels
    open(path, 'w').write(out)


def colors_preamble():
    return '\n'.join(rf'\definecolor{{{k}}}{{HTML}}{{{v.upper()}}}' for k, v in COLORS.items()) + '\n'


open('../fig_colors.tex', 'w').write(colors_preamble())

# Figure 1: fully generic, KoalaBear vs MamaBear (same rows in both runs).
_, kb = parse_table('../tab_mid_kb.tex')
_, mb = parse_table('../tab_mid_mb.tex')
mbt = dict(mb)
rows = [(r, {'baseline': 1.0, 'kb': t['fully generic'] / t['baseline'],
             'mb': mbt[r]['fully generic'] / mbt[r]['baseline']}) for r, t in kb]
bar_figure('../fig_generic.tex',
           r'Fully generic code: time relative to each field\'s \texttt{baseline} (Tables~\ref{tab:mid-kb} and \ref{tab:mid-mb}). The vertical line marks parity.',
           'fig:generic', rows, [('kb', 'KoalaBear', 'c1'), ('mb', 'MamaBear', 'c2')], 500)

# Figure 2: the hot-loop branches before and after replicating gnark-crypto's
# internal-matrix diagonal (runs 3 and 4, same sizes), each against its own
# run's baseline.
raw3 = make_tables.load_raw('medium-diag.txt')
run2 = {r[0]: r for r in csv.reader(open('paths-run2-medians.csv')) if r and not r[0].startswith('#')}
keys = sorted({(f, k) for f, k, _ in raw3 if 'field=koalabear' in k}, key=lambda k: make_tables.sort_key(*k))
rows = []
for fam, key in keys:
    lab = make_tables.label(fam, key)
    rows.append((lab, {'baseline': 1.0, 'initial': float(run2[lab][2]) / float(run2[lab][1]),
                       'diag': raw3[(fam, key, 'generic')][0] / raw3[(fam, key, 'baseline')][0]}))
bar_figure('../fig_branch.tex',
           r"KoalaBear hot-loop branches, time relative to \texttt{baseline}, before and after replicating gnark-crypto's internal-matrix diagonal (Table~\ref{tab:kbb}).",
           'fig:branch', rows, [('initial', 'branches (initial)', 'c1'), ('diag', 'branches, gnark diagonal', 'c2')], 50)

# Figure 3: the three KoalaBear paths, plus current and, from run 5,
# generic-asm-static-call. The latter is scaled so that its ratio is against
# run 5's own baseline, as in the table.
_, paths = parse_table('../tab_paths.tex')
run5 = make_tables.load_raw('medium-static-call.txt')
static = {make_tables.label(f, k): v[0] / run5[(f, k, 'baseline')][0]
          for (f, k, i), v in run5.items() if i == 'generic-asm-static-call'}
rows = [(r, {'baseline': t['baseline'], 'br': t['generic + hot-loop branches'],
             'vec': t[r'\texttt{generic-vec}'], 'asm': t[r'\texttt{generic-asm}'],
             **({'cur': t[r'\texttt{current}']} if r'\texttt{current}' in t else {}),
             **({'sc': static[r] * t['baseline']} if r in static else {})})
        for r, t in paths]
bar_figure('../fig_paths.tex',
           r'KoalaBear code paths, time relative to \texttt{baseline} (Table~\ref{tab:paths}). \texttt{current} exists only for MDHasher and FiatShamir; \texttt{generic-asm-static-call} (run 5, against run 5\textquotesingle s \texttt{baseline}) only changes the compression, so it has no Permutation bars.',
           'fig:paths', rows, [('br', 'hot-loop branches', 'c1'), ('vec', r'\texttt{generic-vec}', 'c2'),
                               ('asm', r'\texttt{generic-asm}', 'c3'), ('cur', r'\texttt{current}', 'c4'),
                               ('sc', r'\texttt{generic-asm-static-call}', 'c5')], 200)

# Figure 4: CompressParallel scaling, log-log.
on = load('compress-parallel-scaling.txt')
off = load('compress-parallel-gcoff.txt')
# generic-asm-static-call (run 6)
s_on = load('compress-parallel-static-call-gcdefault.txt')
s_off = load('compress-parallel-static-call-gcoff.txt')
cpus = sorted({c for _, c in on})


def line(d, impl, style, legend):
    coords = ' '.join(f'({c},{d[(impl, c)]:.4g})' for c in cpus)
    return rf'\addplot[{style}] coordinates {{{coords}}};' + '\n' + rf'\addlegendentry{{{legend}}}'


lines = [
    line(on, 'baseline', 'black!70, thick, mark=*, mark size=1.6pt', r'\texttt{baseline}'),
    line(on, 'generic-asm', 'c1, thick, mark=square*, mark size=1.6pt', r'\texttt{generic-asm}, GC on'),
    line(off, 'generic-asm', 'c1, thick, dashed, mark=square, mark size=1.6pt', r'\texttt{generic-asm}, GC off'),
    line(on, 'generic', 'c2, thick, mark=triangle*, mark size=2pt', r'hot-loop branches, GC on'),
    line(off, 'generic', 'c2, thick, dashed, mark=triangle, mark size=2pt', r'hot-loop branches, GC off'),
    line(s_on, 'generic-asm-static-call', 'c3, thick, mark=diamond*, mark size=2pt', r'\texttt{generic-asm-static-call}, GC on'),
    line(s_off, 'generic-asm-static-call', 'c3, thick, dashed, mark=diamond, mark size=2pt', r'\texttt{generic-asm-static-call}, GC off'),
]
fig = rf'''\begin{{figure}}[htbp]\centering
\begin{{tikzpicture}}
\begin{{axis}}[
  xmode=log, ymode=log, log basis x=2,
  width=0.92\linewidth, height=8cm,
  xtick={{{','.join(str(c) for c in cpus)}}}, xticklabels={{{','.join(str(c) for c in cpus)}}},
  xlabel={{threads (\texttt{{-cpu}})}}, ylabel={{ns per compression (log scale, lower is better)}},
  label style={{font=\small}}, tick label style={{font=\scriptsize}},
  grid=major, major grid style={{gray!20}}, axis line style={{gray!60}},
  legend style={{font=\scriptsize, draw=none, fill=white, fill opacity=0.85, text opacity=1, at={{(0.98,0.98)}}, anchor=north east}},
  legend cell align=left,
]
{chr(10).join(lines)}
\end{{axis}}
\end{{tikzpicture}}
\caption{{KoalaBear CompressParallel scaling (Table~\ref{{tab:parallel}}). Solid: garbage collector on (default); dashed: off. Colour follows the implementation. Green: \texttt{{generic-asm-static-call}} (Section~\ref{{sec:fix}}, run 6), which lies on the baseline with the GC on or off.}}\label{{fig:parallel}}
\end{{figure}}
'''
open('../fig_parallel.tex', 'w').write(fig)
print('wrote fig_colors, fig_generic, fig_branch, fig_paths, fig_parallel')
