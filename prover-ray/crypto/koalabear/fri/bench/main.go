package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"runtime/metrics"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
)

// go run main.go --min-log2 8 --max-log2 12 --base-polys 400 --ext-polys 400

var (
	minLog2      = flag.Int("min-log2", 8, "smallest native polynomial size as log2(N)")
	maxLog2      = flag.Int("max-log2", 12, "largest native polynomial size as log2(N)")
	basePolys    = flag.Int("base-polys", 400, "number of base-field polynomials per size group")
	extPolys     = flag.Int("ext-polys", 400, "number of extension-field polynomials per size group")
	rate         = flag.Int("rate", 4, "Reed-Solomon blowup factor")
	numQueries   = flag.Int("queries", 32, "number of FRI queries")
	maxShifts    = flag.Int("max-shifts", 3, "maximum number of shifts per polynomial")
	seed         = flag.Uint64("seed", 1, "deterministic synthetic input seed")
	gomaxprocs   = flag.Int("gomaxprocs", 0, "override GOMAXPROCS (0 = leave default)")
	sampleMillis = flag.Int("sample-ms", 50, "heap sampling interval (ms)")
)

func main() {
	flag.Parse()
	if *gomaxprocs > 0 {
		runtime.GOMAXPROCS(*gomaxprocs)
	}
	validateConfig()

	maxN := 1 << *maxLog2
	params, err := fri.NewParams((*rate)*maxN, maxN, *numQueries)
	if err != nil {
		fail("NewParams: %v", err)
	}
	encoders := makeEncoders(*maxLog2+1, *rate)
	pcs, err := fri.NewPCS(params, encoders)
	if err != nil {
		fail("NewPCS: %v", err)
	}

	fmt.Printf(
		"fri pcs bench  sizes=2^%d..2^%d  base=%d/group  ext=%d/group  rate=%d  queries=%d  GOMAXPROCS=%d  NumCPU=%d\n\n",
		*minLog2, *maxLog2, *basePolys, *extPolys, *rate, *numQueries, runtime.GOMAXPROCS(0), runtime.NumCPU(),
	)

	batch := makeSyntheticBatch(*minLog2, *maxLog2, *basePolys, *extPolys, *seed)
	shifts := makeSyntheticShifts(batch, *maxShifts, *seed^0x5eed)
	zeta := sampleChallenge(*seed ^ 0x7e7a)
	challenges := makeChallenges(params.N, *maxLog2, *numQueries, *seed^0xc0ffee)
	shapes := []fri.Shape{shapeFromBatch(batch)}

	phases := make([]phaseReport, 0, 3)

	tr := newTracker("Commit", *sampleMillis)
	committed := pcs.Commit(batch)
	phases = append(phases, tr.stop())

	roots := []field.Octuplet{committed.Tree.Root()}
	committedStates := []fri.CommitterState{committed}

	tr = newTracker("Open", *sampleMillis)
	proof, claimed, err := open(pcs, []fri.Batch{batch}, committedStates, []fri.BatchShifts{shifts}, zeta, challenges)
	if err != nil {
		fail("Open: %v", err)
	}
	phases = append(phases, tr.stop())

	tr = newTracker("Verify", *sampleMillis)
	if err := pcs.Verify(fri.VerifyInputs{
		Roots:         roots,
		Shapes:        shapes,
		Shifts:        []fri.BatchShifts{shifts},
		ClaimedValues: claimed,
		Zeta:          zeta,
		Challenges:    challenges,
	}, proof); err != nil {
		fail("Verify: %v", err)
	}
	phases = append(phases, tr.stop())

	fmt.Println()
	printSummary(phases, runtime.GOMAXPROCS(0))
	fmt.Printf("\nproof: %d FRI roots, %d query openings\n", len(proof.FRIProof.FRIRoots), len(proof.FRIProof.FRIQueries))
}

func validateConfig() {
	if *minLog2 < 1 {
		fail("-min-log2 must be >= 1")
	}
	if *maxLog2 < *minLog2 {
		fail("-max-log2 must be >= -min-log2")
	}
	if *basePolys < 0 || *extPolys < 0 {
		fail("-base-polys and -ext-polys must be non-negative")
	}
	if *basePolys == 0 && *extPolys == 0 {
		fail("at least one of -base-polys or -ext-polys must be positive")
	}
	if *rate <= 1 || *rate&(*rate-1) != 0 {
		fail("-rate must be a power of two greater than one")
	}
	if *numQueries <= 0 {
		fail("-queries must be positive")
	}
	if *maxShifts <= 0 {
		fail("-max-shifts must be positive")
	}
}

func makeEncoders(numSizes, rate int) []*fri.RSEncoder {
	encoders := make([]fri.RSEncoder, numSizes)
	refs := make([]*fri.RSEncoder, numSizes)
	for i := range encoders {
		encoders[i] = fri.NewEncoder(uint64(rate*(1<<i)), 1<<i)
		refs[i] = &encoders[i]
	}
	return refs
}

func makeSyntheticBatch(minLog2, maxLog2, nbBase, nbExt int, seed uint64) fri.Batch {
	batch := make(fri.Batch, maxLog2+1)
	for logN := minLog2; logN <= maxLog2; logN++ {
		size := 1 << logN
		table := fri.SizedTable{
			Base: make([][]field.Element, nbBase),
			Ext:  make([][]field.Ext, nbExt),
		}
		for i := range table.Base {
			table.Base[i] = makeBasePolynomial(size, seed, logN, i)
		}
		for i := range table.Ext {
			table.Ext[i] = makeExtPolynomial(size, seed, logN, i)
		}
		batch[logN] = table
	}
	return batch
}

func makeBasePolynomial(size int, seed uint64, logN int, polyIdx int) []field.Element {
	out := make([]field.Element, size)
	x := seed ^ uint64(logN+1)*0x9e3779b185ebca87 ^ uint64(polyIdx+1)*0xc2b2ae3d27d4eb4f
	for i := range out {
		x = nextRand(x)
		out[i].SetUint64(x)
	}
	return out
}

func makeExtPolynomial(size int, seed uint64, logN int, polyIdx int) []field.Ext {
	out := make([]field.Ext, size)
	x := seed ^ uint64(logN+1)*0x165667b19e3779f9 ^ uint64(polyIdx+1)*0x85ebca77c2b2ae63
	for i := range out {
		out[i], x = nextExt(x)
	}
	return out
}

func makeSyntheticShifts(batch fri.Batch, maxShifts int, seed uint64) fri.BatchShifts {
	rng := seed
	out := make(fri.BatchShifts, len(batch))
	for sizeLog2, table := range batch {
		if len(table.Base) == 0 && len(table.Ext) == 0 {
			continue
		}
		size := 1 << sizeLog2
		out[sizeLog2].Base = make([][]int, len(table.Base))
		for i := range table.Base {
			out[sizeLog2].Base[i], rng = makeShiftList(size, maxShifts, rng)
		}
		out[sizeLog2].Ext = make([][]int, len(table.Ext))
		for i := range table.Ext {
			out[sizeLog2].Ext[i], rng = makeShiftList(size, maxShifts, rng)
		}
	}
	return out
}

func makeShiftList(size, maxShifts int, rng uint64) ([]int, uint64) {
	if maxShifts > size {
		maxShifts = size
	}
	rng = nextRand(rng)
	count := 1 + int(rng%uint64(maxShifts))
	out := make([]int, 0, count)
	seen := make(map[int]struct{}, count)
	for len(out) < count {
		rng = nextRand(rng)
		shift := int(rng % uint64(size))
		if _, ok := seen[shift]; ok {
			continue
		}
		seen[shift] = struct{}{}
		out = append(out, shift)
	}
	return out, rng
}

func makeChallenges(domainSize, numRounds, numQueries int, seed uint64) fri.Challenges {
	rng := seed
	alphaDeep, rng := nextExt(rng)
	foldAlphas := make([]field.Ext, numRounds)
	for i := range foldAlphas {
		foldAlphas[i], rng = nextExt(rng)
	}
	queryPositions := makeQueryPositions(domainSize, numQueries, rng)
	return fri.Challenges{
		AlphaDeep:      alphaDeep,
		FoldAlphas:     foldAlphas,
		QueryPositions: queryPositions,
	}
}

func makeQueryPositions(domainSize, numQueries int, rng uint64) []int {
	if numQueries > domainSize {
		fail("-queries=%d exceeds domain size %d", numQueries, domainSize)
	}
	out := make([]int, 0, numQueries)
	seen := make(map[int]struct{}, numQueries)
	for len(out) < numQueries {
		rng = nextRand(rng)
		pos := int(rng % uint64(domainSize))
		if _, ok := seen[pos]; ok {
			continue
		}
		seen[pos] = struct{}{}
		out = append(out, pos)
	}
	return out
}

func sampleChallenge(seed uint64) field.Ext {
	zeta, _ := nextExt(seed)
	if zeta.B0.A1.IsZero() {
		zeta.B0.A1.SetOne()
	}
	return zeta
}

func nextExt(rng uint64) (field.Ext, uint64) {
	var z field.Ext
	rng = nextRand(rng)
	z.B0.A0.SetUint64(rng)
	rng = nextRand(rng)
	z.B0.A1.SetUint64(rng)
	rng = nextRand(rng)
	z.B1.A0.SetUint64(rng)
	rng = nextRand(rng)
	z.B1.A1.SetUint64(rng)
	rng = nextRand(rng)
	z.B2.A0.SetUint64(rng)
	rng = nextRand(rng)
	z.B2.A1.SetUint64(rng)
	return z, rng
}

func nextRand(x uint64) uint64 {
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	return x * 2685821657736338717
}

func shapeFromBatch(batch fri.Batch) fri.Shape {
	shape := make(fri.Shape, len(batch))
	for sizeLog2 := range batch {
		shape[sizeLog2] = fri.SizedShape{
			BaseWidth: len(batch[sizeLog2].Base),
			ExtWidth:  len(batch[sizeLog2].Ext),
		}
	}
	return shape
}

func open(
	pcs *fri.PCS,
	batches []fri.Batch,
	committed []fri.CommitterState,
	shifts []fri.BatchShifts,
	zeta field.Ext,
	challenges fri.Challenges,
) (fri.OpeningProof, []fri.BatchClaimedValues, error) {
	pcs.Reset()
	defer pcs.Reset()

	batchClaims := make([]fri.BatchClaimedValues, len(batches))
	for i := range batches {
		batchClaims[i] = computeClaimedValues(batches[i], shifts[i], zeta)
		if err := pcs.AddOpening(committed[i], zeta, shifts[i], batchClaims[i]); err != nil {
			return fri.OpeningProof{}, nil, err
		}
	}

	state, err := pcs.NewProverState(challenges.AlphaDeep)
	if err != nil {
		return fri.OpeningProof{}, nil, err
	}
	for round := 0; state.HasNext(); round++ {
		state.Fold(challenges.FoldAlphas[round])
	}
	queryPositions := challenges.QueryPositions[:pcs.Params.NumQueries]
	return fri.OpeningProof{
		RowOpenings: pcs.OpenedRows(queryPositions),
		FRIProof:    state.Open(queryPositions),
	}, batchClaims, nil
}

func computeClaimedValues(batch fri.Batch, shifts fri.BatchShifts, zeta field.Ext) fri.BatchClaimedValues {
	claimed := make(fri.BatchClaimedValues, len(shifts))
	for sizeLog2, sizedShifts := range shifts {
		sizedWitness := batch[sizeLog2]
		sized := fri.SizedClaimedValues{
			Base: make([][]field.Ext, len(sizedShifts.Base)),
			Ext:  make([][]field.Ext, len(sizedShifts.Ext)),
		}
		for rowIdx, rowShifts := range sizedShifts.Base {
			sized.Base[rowIdx] = evalRow(field.VecFromBase(sizedWitness.Base[rowIdx]), sizeLog2, rowShifts, zeta)
		}
		for rowIdx, rowShifts := range sizedShifts.Ext {
			sized.Ext[rowIdx] = evalRow(field.VecFromExt(sizedWitness.Ext[rowIdx]), sizeLog2, rowShifts, zeta)
		}
		claimed[sizeLog2] = sized
	}
	return claimed
}

func evalRow(poly field.Vec, sizeLog2 int, rowShifts []int, zeta field.Ext) []field.Ext {
	values := make([]field.Ext, len(rowShifts))
	for i, shift := range rowShifts {
		point := pointAtShift(sizeLog2, shift, zeta)
		values[i] = polynomials.EvalLagrange(poly, field.ElemFromExt(point)).AsExt()
	}
	return values
}

func pointAtShift(sizeLog2, shift int, zeta field.Ext) field.Ext {
	omega := field.RootOfUnityBy(1 << sizeLog2)
	var rotation field.Element
	rotation.Exp(omega, big.NewInt(int64(shift)))
	var point field.Ext
	point.MulByElement(&zeta, &rotation)
	return point
}


type phaseReport struct {
	name string

	wall    time.Duration
	cpuBusy time.Duration
	cpuUser time.Duration
	cpuGC   time.Duration

	allocBytes   uint64
	allocObjects uint64

	heapStart uint64
	heapEnd   uint64
	heapPeak  uint64

	gcCount uint32
}

type tracker struct {
	name string

	wallStart time.Time

	cpuUserStart float64
	cpuGCStart   float64

	allocBytesStart   uint64
	allocObjectsStart uint64

	memStart runtime.MemStats

	peakHeap atomic.Uint64
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func newTracker(name string, sampleMs int) *tracker {
	t := &tracker{name: name}
	t.cpuUserStart, t.cpuGCStart = readCPU()
	t.allocBytesStart, t.allocObjectsStart = readAllocCounters()
	runtime.ReadMemStats(&t.memStart)
	t.peakHeap.Store(t.memStart.HeapAlloc)
	t.stopCh = make(chan struct{})
	t.doneCh = make(chan struct{})
	go t.sampleLoop(time.Duration(sampleMs) * time.Millisecond)
	t.wallStart = time.Now()
	return t
}

func (t *tracker) sampleLoop(interval time.Duration) {
	defer close(t.doneCh)
	if interval <= 0 {
		<-t.stopCh
		return
	}
	tk := time.NewTicker(interval)
	defer tk.Stop()
	var ms runtime.MemStats
	for {
		select {
		case <-t.stopCh:
			return
		case <-tk.C:
			runtime.ReadMemStats(&ms)
			if cur := ms.HeapAlloc; cur > t.peakHeap.Load() {
				t.peakHeap.Store(cur)
			}
		}
	}
}

func (t *tracker) stop() phaseReport {
	wall := time.Since(t.wallStart)
	close(t.stopCh)
	<-t.doneCh

	cpuUser, cpuGC := readCPU()
	allocBytes, allocObjects := readAllocCounters()
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	if memEnd.HeapAlloc > t.peakHeap.Load() {
		t.peakHeap.Store(memEnd.HeapAlloc)
	}

	dUser := secondsToDuration(cpuUser - t.cpuUserStart)
	dGC := secondsToDuration(cpuGC - t.cpuGCStart)

	return phaseReport{
		name:         t.name,
		wall:         wall,
		cpuBusy:      dUser + dGC,
		cpuUser:      dUser,
		cpuGC:        dGC,
		allocBytes:   allocBytes - t.allocBytesStart,
		allocObjects: allocObjects - t.allocObjectsStart,
		heapStart:    t.memStart.HeapAlloc,
		heapEnd:      memEnd.HeapAlloc,
		heapPeak:     t.peakHeap.Load(),
		gcCount:      memEnd.NumGC - t.memStart.NumGC,
	}
}

var cpuMetricNames = []string{
	"/cpu/classes/user:cpu-seconds",
	"/cpu/classes/gc/total:cpu-seconds",
}

var allocMetricNames = []string{
	"/gc/heap/allocs:bytes",
	"/gc/heap/allocs:objects",
}

func readCPU() (user, gc float64) {
	samples := make([]metrics.Sample, len(cpuMetricNames))
	for i, name := range cpuMetricNames {
		samples[i].Name = name
	}
	metrics.Read(samples)
	return samples[0].Value.Float64(), samples[1].Value.Float64()
}

func readAllocCounters() (bytes, objects uint64) {
	samples := make([]metrics.Sample, len(allocMetricNames))
	for i, name := range allocMetricNames {
		samples[i].Name = name
	}
	metrics.Read(samples)
	return samples[0].Value.Uint64(), samples[1].Value.Uint64()
}

func secondsToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

func printSummary(phases []phaseReport, procs int) {
	var totalWall, totalCPU, totalGC time.Duration
	var totalAllocBytes, totalAllocObjects uint64
	var peakHeap uint64
	var gcCount uint32
	for _, p := range phases {
		totalWall += p.wall
		totalCPU += p.cpuBusy
		totalGC += p.cpuGC
		totalAllocBytes += p.allocBytes
		totalAllocObjects += p.allocObjects
		gcCount += p.gcCount
		if p.heapPeak > peakHeap {
			peakHeap = p.heapPeak
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "phase\twall\tcpu\tpar\tgc%\talloc\tobjs\tpeakHeap\tGCs")
	fmt.Fprintln(w, "-----\t----\t---\t---\t---\t-----\t----\t--------\t---")
	for _, p := range phases {
		fmt.Fprintf(w, "%s\t%s\t%s\t%5.2fx\t%4.1f%%\t%s\t%s\t%s\t%d\n",
			p.name,
			fmtDur(p.wall),
			fmtDur(p.cpuBusy),
			parallelization(p.cpuBusy, p.wall),
			gcShare(p.cpuGC, p.cpuBusy),
			fmtBytes(p.allocBytes),
			fmtCount(p.allocObjects),
			fmtBytes(p.heapPeak),
			p.gcCount,
		)
	}
	fmt.Fprintln(w, "-----\t----\t---\t---\t---\t-----\t----\t--------\t---")
	fmt.Fprintf(w, "TOTAL\t%s\t%s\t%5.2fx\t%4.1f%%\t%s\t%s\t%s\t%d\n",
		fmtDur(totalWall),
		fmtDur(totalCPU),
		parallelization(totalCPU, totalWall),
		gcShare(totalGC, totalCPU),
		fmtBytes(totalAllocBytes),
		fmtCount(totalAllocObjects),
		fmtBytes(peakHeap),
		gcCount,
	)
	w.Flush()

	fmt.Println()
	fmt.Printf("cpu      = on-CPU time (user goroutines + GC); excludes idle\n")
	fmt.Printf("par      = cpu / wall   (ideal: %dx = %d cores fully busy; 1x = single-threaded)\n", procs, procs)
	fmt.Printf("gc%%      = GC CPU time / on-CPU time\n")
	fmt.Println("peakHeap = max HeapAlloc observed during phase (sampled in background)")
}

func fmtDur(d time.Duration) string {
	switch {
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	default:
		return d.String()
	}
}

func fmtBytes(b uint64) string {
	const (
		KiB = 1 << 10
		MiB = 1 << 20
		GiB = 1 << 30
	)
	switch {
	case b >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(b)/GiB)
	case b >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(b)/MiB)
	case b >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(b)/KiB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func fmtCount(n uint64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func parallelization(cpu, wall time.Duration) float64 {
	if wall <= 0 {
		return 0
	}
	return float64(cpu) / float64(wall)
}

func gcShare(gc, cpu time.Duration) float64 {
	if cpu <= 0 {
		return 0
	}
	return 100 * float64(gc) / float64(cpu)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fri bench: "+format+"\n", args...)
	os.Exit(1)
}
