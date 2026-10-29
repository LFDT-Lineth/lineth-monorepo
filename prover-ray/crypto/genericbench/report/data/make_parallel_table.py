"""Builds ../tab_parallel.tex from the CompressParallel thread-scaling runs.

Usage (from this directory): python3 make_parallel_table.py
"""
import collections
import re
import statistics


def load(path):
    d = collections.defaultdict(list)
    for line in open(path):
        if not line.startswith('Benchmark'):
            continue
        f = line.split()
        m = re.match(r'.*impl=([\w-]+?)(?:-(\d+))?$', f[0])
        d[(m.group(1), int(m.group(2) or 1))].append(float(f[2]))
    return {k: statistics.median(v) for k, v in d.items()}


def ns(x):
    return f'{x:.0f}' if x >= 100 else f'{x:.1f}'


def ratio(x):
    return f'{x:.0f}$\\times$' if x >= 10 else f'{x:.1f}$\\times$'


TEMPLATE = r'''\begin{table}[htbp]\centering\small
\caption{KoalaBear CompressParallel, nanoseconds per compression (lower is better) by number of threads (\texttt{-cpu}), with the garbage collector on (default) and off (\texttt{GOGC=off}). Ratios are relative to \texttt{baseline} at the same thread count. \texttt{baseline} allocates nothing, so the GC setting does not affect it. \texttt{generic-asm-static-call} comes from run 6 and its ratios are relative to run 6\textquotesingle s \texttt{baseline}, measured with the same GC setting.}\label{tab:parallel}
\begin{adjustbox}{max width=\linewidth}
\begin{tabular}{rr|rrrr|rrrr|rrrr}
\toprule
 & & \multicolumn{4}{c|}{\texttt{generic-asm}} & \multicolumn{4}{c|}{generic + hot-loop branches} & \multicolumn{4}{c}{\texttt{generic-asm-static-call} (run 6)} \\
Threads & baseline & GC on & ratio & GC off & ratio & GC on & ratio & GC off & ratio & GC on & ratio & GC off & ratio \\ \midrule
ROWS
\bottomrule
\end{tabular}
\end{adjustbox}
\end{table}
'''


def main():
    on = load('compress-parallel-scaling.txt')
    off = load('compress-parallel-gcoff.txt')
    s_on = load('compress-parallel-static-call-gcdefault.txt')
    s_off = load('compress-parallel-static-call-gcoff.txt')
    rows = []
    for c in sorted({c for _, c in on}):
        b = on[('baseline', c)]
        cells = [str(c), ns(b)]
        for impl in ('generic-asm', 'generic'):
            for d in (on, off):
                cells += [ns(d[(impl, c)]), ratio(d[(impl, c)] / b)]
        for d in (s_on, s_off):
            v = d[('generic-asm-static-call', c)]
            cells += [ns(v), ratio(v / d[('baseline', c)])]
        rows.append(' & '.join(cells) + r' \\')
    out = TEMPLATE.replace('ROWS', '\n'.join(rows))
    open('../tab_parallel.tex', 'w').write(out)
    print(out)


if __name__ == '__main__':
    main()
