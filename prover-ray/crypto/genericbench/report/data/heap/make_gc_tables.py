"""Builds ../../tab_gc_scaling.tex, ../../tab_gogc.tex, ../../fig_stw.tex and
../../fig_gogc.tex from the gcharness runs (medians of 3 repetitions of 2 s).

Inputs (in this directory):
  gc-scaling.tsv  gcharness -impl generic-asm,generic -threads 1,...,64 -gogc 100
  gc-gogc.tsv     gcharness -impl generic-asm -threads 64 -gogc 100,...,-1

Usage (from this directory): python3 make_gc_tables.py
"""
import collections
import csv
import statistics

# CompressParallel baseline at 64 threads, ../compress-parallel-scaling.txt
BASELINE_64 = 12.8


def load(path):
    d = collections.defaultdict(list)
    for r in csv.DictReader(open(path), delimiter='\t'):
        d[(r['impl'], int(r['threads']), int(r['gogc']))].append(r)
    return {k: {c: statistics.median(float(x[c]) for x in v) for c in v[0] if c not in ('impl',)}
            for k, v in d.items()}


def table(caption, label, cols, header, rows):
    out = [r'\begin{table}[htbp]\centering\small', f'\\caption{{{caption}}}\\label{{{label}}}',
           r'\begin{adjustbox}{max width=\linewidth}', f'\\begin{{tabular}}{{{cols}}}', r'\toprule',
           ' & '.join(header) + r' \\ \midrule']
    out += [' & '.join(r) + r' \\' for r in rows]
    out += [r'\bottomrule', r'\end{tabular}', r'\end{adjustbox}', r'\end{table}']
    return '\n'.join(out) + '\n'


def g(x):
    return f'{x:.3g}'


def main():
    sc = load('gc-scaling.tsv')
    rows = []
    for t in sorted({t for _, t, _ in sc}):
        r = sc[('generic-asm', t, 100)]
        rows.append([str(t), g(r['ns_per_op']), g(r['cycle_ms']), f"{r['stw_pct_wall']:.1f}\\%",
                     f"{1e3 * r['stw_total_ms'] / (2 * 2000 / r['cycle_ms']):.0f}" if r['cycle_ms'] else '--',
                     f"{r['assist_pct'] + r['dedicated_pct'] + r['idlemark_pct']:.1f}\\%"])
    open('../../tab_gc_scaling.tex', 'w').write(table(
        r'\texttt{generic-asm} compressions on $N$ goroutines, GC on, as reported by the Go runtime (\texttt{runtime/metrics}). Stop-the-world (STW) time is the share of wall-clock time with the world stopped; each GC cycle stops it twice. Mark CPU is assists plus background mark workers, as a share of CPU time.',
        'tab:gc-scaling', 'r|rrrrr',
        ['Threads', 'ns/op', 'ms between cycles', 'STW, \\% of wall', 'mean STW (\\textmu s)', 'mark CPU'], rows))
    gg = load('gc-gogc.tsv')
    rows = []
    for gc in sorted({x for _, _, x in gg}, key=lambda x: (x < 0, x)):
        r = gg[('generic-asm', 64, gc)]
        rows.append(['off' if gc < 0 else str(gc), g(r['ns_per_op']), f"{r['ns_per_op'] / BASELINE_64:.1f}$\\times$",
                     g(r['cycle_ms']) if r['cycle_ms'] else '--', g(r['goal_MB']) if gc > 0 else '--',
                     f"{r['stw_pct_wall']:.1f}\\%"])
    open('../../tab_gogc.tex', 'w').write(table(
        r'\texttt{generic-asm} compressions on 64 goroutines for several values of \texttt{GOGC}. The ratio is against \texttt{baseline} at 64 threads (12.8\,ns, Table~\ref{tab:parallel}).',
        'tab:gogc', 'r|rrrrr',
        [r'\texttt{GOGC}', 'ns/op', 'ratio', 'ms between cycles', 'heap goal (MB)', 'STW, \\% of wall'], rows))
    figures(sc, gg)
    print('wrote tab_gc_scaling.tex, tab_gogc.tex, fig_stw.tex, fig_gogc.tex')


AXIS = r'''  width=0.48\linewidth, height=6cm,
  label style={font=\small}, tick label style={font=\scriptsize},
  grid=major, major grid style={gray!20}, axis line style={gray!60},
  legend style={font=\scriptsize, draw=none, fill=white, fill opacity=0.85, text opacity=1},
  legend cell align=left,'''


def coords(pts):
    return ' '.join(f'({x},{y:.4g})' for x, y in pts)


def figures(sc, gg):
    threads = sorted({t for _, t, _ in sc})
    ticks = ','.join(map(str, threads))
    left = []
    right = []
    for impl, color, mark, name in (('generic-asm', 'c1', 'square*', r'\texttt{generic-asm}'),
                                    ('generic', 'c2', 'triangle*', 'hot-loop branches')):
        left.append(rf'\addplot[{color}, thick, mark={mark}, mark size=1.6pt] coordinates {{{coords((t, sc[(impl, t, 100)]["stw_pct_wall"]) for t in threads)}}};'
                    + '\n' + rf'\addlegendentry{{{name}}}')
        right.append(rf'\addplot[{color}, thick, mark={mark}, mark size=1.6pt] coordinates {{{coords((t, 1e3 * sc[(impl, t, 100)]["stw_total_ms"] / (2 * 2000 / sc[(impl, t, 100)]["cycle_ms"])) for t in threads)}}};'
                     + '\n' + rf'\addlegendentry{{{name}}}')
    xaxis = rf'xmode=log, log basis x=2, xtick={{{ticks}}}, xticklabels={{{ticks}}}, xlabel={{threads}},'
    out = rf'''\begin{{figure}}[htbp]\centering
\begin{{tikzpicture}}
\begin{{axis}}[{xaxis}
  ylabel={{world stopped (\% of wall-clock time)}}, ymin=0,
  legend pos=north west,
{AXIS}
]
{chr(10).join(left)}
\end{{axis}}
\end{{tikzpicture}}\hfill
\begin{{tikzpicture}}
\begin{{axis}}[{xaxis}
  ymode=log, ylabel={{mean stop-the-world pause (\textmu s, log scale)}},
  legend pos=north west,
{AXIS}
]
{chr(10).join(right)}
\end{{axis}}
\end{{tikzpicture}}
\caption{{Stop-the-world time with the GC on, by number of threads (Table~\ref{{tab:gc-scaling}}). Both generic paths jump between 8 and 16 threads.}}\label{{fig:stw}}
\end{{figure}}
'''
    open('../../fig_stw.tex', 'w').write(out)

    on = sorted(x for _, _, x in gg if x > 0)
    off = gg[('generic-asm', 64, -1)]['ns_per_op']
    ticks = ','.join(map(str, on))
    out = rf'''\begin{{figure}}[htbp]\centering
\begin{{tikzpicture}}
\begin{{axis}}[
  xmode=log, log basis x=2, ymode=log, xtick={{{ticks}}}, xticklabels={{{ticks}}},
  xlabel={{\texttt{{GOGC}}}}, ylabel={{ns per compression (log scale)}},
  width=0.7\linewidth, height=6.5cm, ymin=8, ymax=150,
  ytick={{10,20,50,100}}, yticklabels={{10,20,50,100}},
  label style={{font=\small}}, tick label style={{font=\scriptsize}},
  grid=major, major grid style={{gray!20}}, axis line style={{gray!60}},
  legend style={{font=\scriptsize, draw=none, fill=white, fill opacity=0.85, text opacity=1}},
  legend cell align=left, legend pos=north east,
]
\addplot[c1, thick, mark=square*, mark size=1.6pt] coordinates {{{coords((g, gg[('generic-asm', 64, g)]['ns_per_op']) for g in on)}}};
\addlegendentry{{\texttt{{generic-asm}}, GC on}}
\addplot[c1, thick, dashed, domain={on[0]}:{on[-1]}] {{{off:.4g}}};
\addlegendentry{{\texttt{{generic-asm}}, GC off}}
\addplot[black!70, thick, domain={on[0]}:{on[-1]}] {{{BASELINE_64}}};
\addlegendentry{{\texttt{{baseline}}}}
\end{{axis}}
\end{{tikzpicture}}
\caption{{\texttt{{generic-asm}} at 64 threads as \texttt{{GOGC}} is raised (Table~\ref{{tab:gogc}}). Fewer GC cycles, lower cost.}}\label{{fig:gogc}}
\end{{figure}}
'''
    open('../../fig_gogc.tex', 'w').write(out)


if __name__ == '__main__':
    main()
