// Command gcharness runs KoalaBear compressions on N goroutines and reports
// what the Go runtime's own metrics say the GC cost, per configuration.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/consensys/gnark-crypto/field/koalabear"
	kbvortex "github.com/consensys/gnark-crypto/field/koalabear/vortex"

	gp2 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/poseidon2"
)

type E = koalabear.Element

func compressor(impl string) gp2.CompressFunc[E] {
	if impl == "baseline" {
		return kbvortex.CompressPoseidon2
	}
	p := gp2.NewPermutation(gp2.NewParameters[E, *E](16, 6, 21, gp2.DefaultSeed("koalabear", 16, 6, 21)))
	switch impl {
	case "generic-asm":
		p.SetKoalaBearPath(gp2.KoalaBearAsm)
	case "generic":
	default:
		panic(impl)
	}
	return gp2.Compressor(p)
}

var names = []string{
	"/gc/cycles/total:gc-cycles",
	"/gc/heap/allocs:bytes",
	"/gc/heap/live:bytes",
	"/gc/heap/goal:bytes",
	"/cpu/classes/total:cpu-seconds",
	"/cpu/classes/user:cpu-seconds",
	"/cpu/classes/idle:cpu-seconds",
	"/cpu/classes/gc/total:cpu-seconds",
	"/cpu/classes/gc/mark/assist:cpu-seconds",
	"/cpu/classes/gc/mark/dedicated:cpu-seconds",
	"/cpu/classes/gc/mark/idle:cpu-seconds",
	"/cpu/classes/gc/pause:cpu-seconds",
	"/sched/pauses/total/gc:seconds",
	"/sched/pauses/stopping/gc:seconds",
}

func read() map[string]metrics.Sample {
	s := make([]metrics.Sample, len(names))
	for i, n := range names {
		s[i].Name = n
	}
	metrics.Read(s)
	m := map[string]metrics.Sample{}
	for _, x := range s {
		m[x.Name] = x
	}
	return m
}

func num(m map[string]metrics.Sample, n string) float64 {
	v := m[n].Value
	switch v.Kind() {
	case metrics.KindUint64:
		return float64(v.Uint64())
	case metrics.KindFloat64:
		return v.Float64()
	}
	return 0
}

// histDelta returns count, approximate sum and max (bucket upper bound) of the
// samples added to a histogram between a and b.
func histDelta(a, b map[string]metrics.Sample, n string) (cnt uint64, sum, max float64) {
	ha, hb := a[n].Value.Float64Histogram(), b[n].Value.Float64Histogram()
	for i := range hb.Counts {
		d := hb.Counts[i] - ha.Counts[i]
		if d == 0 {
			continue
		}
		lo, hi := hb.Buckets[i], hb.Buckets[i+1]
		if lo < 0 || lo != lo || lo < -1e300 {
			lo = 0
		}
		if hi > 1e300 {
			hi = lo
		}
		cnt += d
		sum += float64(d) * (lo + hi) / 2
		max = hi
	}
	return
}

func ints(s string) []int {
	var r []int
	for _, x := range strings.Split(s, ",") {
		v, err := strconv.Atoi(x)
		if err != nil {
			panic(err)
		}
		r = append(r, v)
	}
	return r
}

func main() {
	impls := flag.String("impl", "generic-asm", "comma-separated implementations")
	threads := flag.String("threads", "64", "comma-separated thread counts")
	gogcs := flag.String("gogc", "100", "comma-separated GOGC values (-1 = off)")
	dur := flag.Duration("dur", 2*time.Second, "duration per configuration")
	reps := flag.Int("reps", 3, "repetitions per configuration")
	flag.Parse()

	fmt.Println("impl\tthreads\tgogc\trep\tns_per_op\tB_per_op\tgc_cycles\tcycle_ms\tlive_MB\tgoal_MB\tbusy_threads\tuser_pct\tidle_pct\tgc_pct\tassist_pct\tdedicated_pct\tidlemark_pct\tpause_cpu_pct\tstw_count\tstw_total_ms\tstw_pct_wall\tstw_max_ms\tstopping_total_ms")
	in := make([]E, 16)
	for i := range in {
		in[i].SetUint64(uint64(i)*0x9E3779B97F4A7C15 + 1)
	}
	for _, impl := range strings.Split(*impls, ",") {
		c := compressor(impl)
		for _, t := range ints(*threads) {
			for _, g := range ints(*gogcs) {
				for rep := range *reps {
					runtime.GOMAXPROCS(t)
					debug.SetGCPercent(g)
					runtime.GC()
					var ops atomic.Int64
					var stop atomic.Bool
					var wg sync.WaitGroup
					a := read()
					start := time.Now()
					for range t {
						wg.Go(func() {
							st, blk := gp2.Octuplet[E](in[:8]), gp2.Octuplet[E](in[8:])
							n := int64(0)
							for !stop.Load() {
								for range 64 {
									st = c(st, blk)
								}
								n += 64
							}
							ops.Add(n)
						})
					}
					time.Sleep(*dur)
					stop.Store(true)
					wg.Wait()
					wall := time.Since(start).Seconds()
					b := read()
					d := func(n string) float64 { return num(b, n) - num(a, n) }
					o := float64(ops.Load())
					total := d("/cpu/classes/total:cpu-seconds")
					pct := func(n string) float64 { return 100 * d(n) / total }
					cyc := d("/gc/cycles/total:gc-cycles")
					cycMs := 0.0
					if cyc > 0 {
						cycMs = 1000 * wall / cyc
					}
					sc, ss, sm := histDelta(a, b, "/sched/pauses/total/gc:seconds")
					_, ps, _ := histDelta(a, b, "/sched/pauses/stopping/gc:seconds")
					fmt.Printf("%s\t%d\t%d\t%d\t%.2f\t%.1f\t%.0f\t%.2f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%d\t%.1f\t%.1f\t%.2f\t%.1f\n",
						impl, t, g, rep, 1e9*wall/o, d("/gc/heap/allocs:bytes")/o, cyc, cycMs,
						num(b, "/gc/heap/live:bytes")/1e6, num(b, "/gc/heap/goal:bytes")/1e6,
						(total-d("/cpu/classes/idle:cpu-seconds"))/wall,
						pct("/cpu/classes/user:cpu-seconds"), pct("/cpu/classes/idle:cpu-seconds"),
						pct("/cpu/classes/gc/total:cpu-seconds"), pct("/cpu/classes/gc/mark/assist:cpu-seconds"),
						pct("/cpu/classes/gc/mark/dedicated:cpu-seconds"), pct("/cpu/classes/gc/mark/idle:cpu-seconds"),
						pct("/cpu/classes/gc/pause:cpu-seconds"),
						sc, 1000*ss, 100*ss/wall, 1000*sm, 1000*ps)
				}
			}
		}
	}
	_ = os.Stdout.Sync()
}
