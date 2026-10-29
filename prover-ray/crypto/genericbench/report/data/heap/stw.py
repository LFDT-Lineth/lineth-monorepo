"""Per stop-the-world window in a `go tool trace -d=parsed` dump: duration,
how long until the last running goroutine stopped, and where it was."""
import collections, re, statistics, sys

ev_re = re.compile(r'^M=(-?\d+) P=(-?\d+) G=(-?\d+) (\w+) Time=(\d+)(.*)$')

def events(path):
    cur = None
    for line in open(path):
        if line.startswith('M='):
            if cur: yield cur
            m = ev_re.match(line.rstrip())
            cur = None if not m else dict(m=m[1], p=m[2], g=m[3], kind=m[4], t=int(m[5]), rest=m[6], stack=[])
        elif cur is not None and line.startswith('\t') and not line.startswith('\t\t'):
            if len(cur['stack']) < 12 and '@' in line:
                cur['stack'].append(line.strip().split(' @')[0])
    if cur: yield cur

stw = None
wins = []
for e in events(sys.argv[1]):
    if e['kind'] == 'RangeBegin' and 'stop-the-world (GC' in e['rest']:
        stw = dict(name=re.search(r'\((GC [^)]*)\)', e['rest'])[1], t0=e['t'], stops=[], g=e['g'])
    elif e['kind'] == 'RangeEnd' and stw and 'stop-the-world (GC' in e['rest']:
        stw['t1'] = e['t']; wins.append(stw); stw = None
    elif stw and e['kind'] == 'StateTransition' and 'Running->' in e['rest'] and 'GoID=' in e['rest']:
        stw['stops'].append((e['t'] - stw['t0'], e['stack']))

# Skip the forced cycles between benchmark runs (started by testing.runN).
print(f'{len(wins)} STW windows')
durs = sorted((w['t1'] - w['t0']) / 1e3 for w in wins)
q = lambda v, f: v[min(len(v) - 1, int(f * len(v)))]
print('STW duration us: median %.0f  p90 %.0f  max %.0f  total %.1f ms' % (q(durs, .5), q(durs, .9), durs[-1], sum(durs) / 1e3))
lag = []
where = collections.Counter()
nstop = []
for w in wins:
    if not w['stops']: continue
    nstop.append(len(w['stops']))
    t, st = max(w['stops'], key=lambda s: s[0])
    lag.append(t / 1e3)
    frames = [f for f in st if not f.startswith('runtime.') ] 
    top = st[0] if st else '?'
    where[(top, frames[0] if frames else '?')] += 1
lag.sort()
print('goroutines stopped per window: median %d' % statistics.median(nstop))
print('time until last goroutine stopped, us: median %.0f  p90 %.0f  max %.0f' % (q(lag, .5), q(lag, .9), lag[-1]))
print('where the last goroutine was when it stopped (top frame | first non-runtime frame):')
for (a, b), n in where.most_common(8):
    print(f'  {n:4d}  {a} | {b}')
