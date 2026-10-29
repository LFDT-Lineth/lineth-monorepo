"""Time the benchmark goroutines spend in each state, by reason, from a
`go tool trace -d=parsed` dump. Also time with the world stopped."""
import collections, re, sys
exec(open(sys.argv[0].replace('states.py', 'stw.py')).read().split('stw = None')[0])
st_re = re.compile(r'GoID=(\d+) (\w+)->(\w+) Reason="([^"]*)"')
evs = list(events(sys.argv[1]))
# worker goroutines: ones that ever ran Compress
workers = set()
for e in evs:
    m = st_re.search(e['rest']) if e['kind'] == 'StateTransition' else None
    if m and m[2] == 'Running' and m[1] == e['g'] and any('poseidon2.Compress' in f for f in e['stack']):
        workers.add(m[1])
state = {}  # g -> (state, reason, since)
acc = collections.Counter()
t_first, t_last = None, None
for e in evs:
    if e['kind'] != 'StateTransition': continue
    m = st_re.search(e['rest'])
    if not m: continue
    g, old, new, why = m.groups()
    if g not in workers: continue
    t_first = t_first or e['t']; t_last = e['t']
    if g in state:
        s, r, t0 = state[g]
        acc[(s, r)] += e['t'] - t0
    top = next((f for f in e['stack'] if f not in ('runtime.gopark', 'runtime.goparkunlock', 'runtime.semacquire1', 'runtime.systemstack_switch')), '')
    reason = why if new != 'Running' else ''
    if new == 'Waiting': reason = f'{why} [{top}]'
    if new == 'Runnable': reason = why or 'woken'
    state[g] = (new, reason, e['t'])
tot = sum(acc.values())
print(f'{len(workers)} worker goroutines, {tot/1e9:.1f} goroutine-seconds over {(t_last-t_first)/1e9:.2f} s')
for (s, r), v in acc.most_common(14):
    print(f'{100*v/tot:5.1f}%  {s:9s} {r}')
