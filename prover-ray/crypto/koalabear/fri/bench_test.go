package fri

// Standard-library port of bench/main.go: same synthetic workload (multi-size
// batch of base and extension rows, per-row shift schedules, deterministic
// xorshift inputs), split into one Benchmark per PCS phase so each can be
// profiled independently:
//
//	go test -run=NONE -bench=BenchmarkPCS -benchmem ./prover-ray/crypto/koalabear/fri/
//	go test -run=NONE -bench=BenchmarkPCSCommit/sizes=8..12 -cpuprofile=cpu.out ...
//
// Unlike bench/main.go, the claimed-value computation (caller-side Lagrange
// evaluations, not PCS work) is measured by its own benchmark instead of being
// folded into the Open phase.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
)

type benchConfig struct {
	minLog2    int
	maxLog2    int
	basePolys  int
	extPolys   int
	rate       int
	numQueries int
	maxShifts  int
	// sharedShifts, when non-nil, makes every row of every size open at exactly
	// this fixed shift set (clamped to the size). This mimics a real protocol,
	// where a handful of distinct claim points are shared by all rows, instead
	// of the 1–3 uniformly-random shifts per row the default configs draw.
	sharedShifts []int
	seed         uint64
}

func (c benchConfig) name() string {
	suffix := ""
	if c.sharedShifts != nil {
		suffix = "/shifts=shared"
	}
	return fmt.Sprintf("sizes=%d..%d/base=%d/ext=%d/rate=%d/queries=%d%s",
		c.minLog2, c.maxLog2, c.basePolys, c.extPolys, c.rate, c.numQueries, suffix)
}

// benchConfigs mirrors bench/main.go's defaults (the large one) plus a small
// config for quick iteration, and a production-like variant of the large
// config whose rows all share a tiny fixed shift set {0,1,2}.
var benchConfigs = []benchConfig{
	{minLog2: 8, maxLog2: 10, basePolys: 64, extPolys: 64, rate: 4, numQueries: 32, maxShifts: 3, seed: 1},
	{minLog2: 8, maxLog2: 12, basePolys: 400, extPolys: 400, rate: 4, numQueries: 32, maxShifts: 3, seed: 1},
	{minLog2: 8, maxLog2: 12, basePolys: 400, extPolys: 400, rate: 4, numQueries: 32, sharedShifts: []int{0, 1, 2}, seed: 1},
}

// benchFixture bundles everything the phase benchmarks need, built once per
// config outside the timed loops.
type benchFixture struct {
	cfg        benchConfig
	pcs        *PCS
	batch      Batch
	shifts     BatchShifts
	zeta       field.Ext
	challenges Challenges
	shapes     []Shape

	committed CommitterState
	claimed   BatchClaimedValues
	proof     OpeningProof
	roots     []field.Octuplet
}

func newBenchFixture(tb testing.TB, cfg benchConfig) *benchFixture {
	tb.Helper()

	maxN := 1 << cfg.maxLog2
	params, err := NewParams(uint8(log2int(cfg.rate*maxN)), uint8(cfg.maxLog2), uint(cfg.numQueries))
	if err != nil {
		tb.Fatalf("NewParams: %v", err)
	}
	encoders := benchEncoders(cfg.maxLog2+1, cfg.rate)
	pcs, err := NewPCS(params, encoders)
	if err != nil {
		tb.Fatalf("NewPCS: %v", err)
	}

	batch := benchBatch(cfg)
	var shifts BatchShifts
	if cfg.sharedShifts != nil {
		shifts = benchSharedShifts(batch, cfg.sharedShifts)
	} else {
		shifts = benchShifts(batch, cfg.maxShifts, cfg.seed^0x5eed)
	}
	zeta := benchChallengePoint(cfg.seed ^ 0x7e7a)
	challenges := benchChallenges(cfg.rate*maxN, cfg.maxLog2, cfg.numQueries, cfg.seed^0xc0ffee)

	fx := &benchFixture{
		cfg:        cfg,
		pcs:        pcs,
		batch:      batch,
		shifts:     shifts,
		zeta:       zeta,
		challenges: challenges,
		shapes:     []Shape{batch.Shape()},
	}

	fx.committed = pcs.Commit(batch)
	fx.roots = []field.Octuplet{fx.committed.Tree.Root()}
	fx.claimed = benchClaimedValues(batch, shifts, zeta)
	fx.proof = fx.open(tb)
	return fx
}

// open runs the full prover opening flow (AddOpening + NewProverState + all
// folds + query phase) against the fixture's precomputed claims.
func (fx *benchFixture) open(tb testing.TB) OpeningProof {
	tb.Helper()
	fx.pcs.Reset()
	defer fx.pcs.Reset()

	if err := fx.pcs.AddOpening(fx.committed, fx.zeta, fx.shifts, fx.claimed); err != nil {
		tb.Fatalf("AddOpening: %v", err)
	}
	state, err := fx.pcs.NewProverState()
	if err != nil {
		tb.Fatalf("NewProverState: %v", err)
	}
	for round := 0; state.HasNext(); round++ {
		state.Fold(fx.challenges.FoldAlphas[round])
	}
	return fx.pcs.Open(state, fx.challenges.QueryPositions[:fx.pcs.Params.NumQueries])
}

func (fx *benchFixture) verify(tb testing.TB) {
	tb.Helper()
	if err := fx.pcs.Verify(VerifyInputs{
		Roots:         fx.roots,
		Shapes:        fx.shapes,
		Shifts:        []BatchShifts{fx.shifts},
		ClaimedValues: []BatchClaimedValues{fx.claimed},
		Zeta:          fx.zeta,
		Challenges:    fx.challenges,
	}, fx.proof); err != nil {
		tb.Fatalf("Verify: %v", err)
	}
}

func BenchmarkPCSCommit(b *testing.B) {
	for _, cfg := range benchConfigs {
		b.Run(cfg.name(), func(b *testing.B) {
			fx := newBenchFixture(b, cfg)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				committed := fx.pcs.Commit(fx.batch)
				if committed.Tree.Root() != fx.roots[0] {
					b.Fatal("commit root mismatch")
				}
			}
		})
	}
}

func BenchmarkPCSOpen(b *testing.B) {
	for _, cfg := range benchConfigs {
		b.Run(cfg.name(), func(b *testing.B) {
			fx := newBenchFixture(b, cfg)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				proof := fx.open(b)
				if len(proof.FRIProof.RunningQueries) != cfg.numQueries {
					b.Fatal("unexpected query count")
				}
			}
		})
	}
}

func BenchmarkPCSVerify(b *testing.B) {
	for _, cfg := range benchConfigs {
		b.Run(cfg.name(), func(b *testing.B) {
			fx := newBenchFixture(b, cfg)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fx.verify(b)
			}
		})
	}
}

// BenchmarkPCSComputeClaims measures the caller-side claimed-value computation
// (one Lagrange evaluation per (row, shift)); bench/main.go folded this into
// its Open phase.
func BenchmarkPCSComputeClaims(b *testing.B) {
	for _, cfg := range benchConfigs {
		b.Run(cfg.name(), func(b *testing.B) {
			fx := newBenchFixture(b, cfg)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				claimed := benchClaimedValues(fx.batch, fx.shifts, fx.zeta)
				if len(claimed) != len(fx.claimed) {
					b.Fatal("unexpected claim shape")
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Synthetic-input builders (ported from bench/main.go, same xorshift streams)
// ─────────────────────────────────────────────────────────────────────────────

func log2int(n int) int {
	l := 0
	for 1<<l < n {
		l++
	}
	return l
}

func benchEncoders(numSizes, rate int) []*RSEncoder {
	encoders := make([]RSEncoder, numSizes)
	refs := make([]*RSEncoder, numSizes)
	for i := range encoders {
		encoders[i] = NewEncoder(uint64(rate*(1<<i)), 1<<i)
		refs[i] = &encoders[i]
	}
	return refs
}

func benchBatch(cfg benchConfig) Batch {
	batch := make(Batch, cfg.maxLog2+1)
	for logN := cfg.minLog2; logN <= cfg.maxLog2; logN++ {
		size := 1 << logN
		table := SizedTable{
			Base: make([][]field.Element, cfg.basePolys),
			Ext:  make([][]field.Ext, cfg.extPolys),
		}
		for i := range table.Base {
			table.Base[i] = benchBasePoly(size, cfg.seed, logN, i)
		}
		for i := range table.Ext {
			table.Ext[i] = benchExtPoly(size, cfg.seed, logN, i)
		}
		batch[logN] = table
	}
	return batch
}

func benchBasePoly(size int, seed uint64, logN, polyIdx int) []field.Element {
	out := make([]field.Element, size)
	x := seed ^ uint64(logN+1)*0x9e3779b185ebca87 ^ uint64(polyIdx+1)*0xc2b2ae3d27d4eb4f
	for i := range out {
		x = benchRand(x)
		out[i].SetUint64(x)
	}
	return out
}

func benchExtPoly(size int, seed uint64, logN, polyIdx int) []field.Ext {
	out := make([]field.Ext, size)
	x := seed ^ uint64(logN+1)*0x165667b19e3779f9 ^ uint64(polyIdx+1)*0x85ebca77c2b2ae63
	for i := range out {
		out[i], x = benchExt(x)
	}
	return out
}

func benchShifts(batch Batch, maxShifts int, seed uint64) BatchShifts {
	rng := seed
	out := make(BatchShifts, len(batch))
	for sizeLog2, table := range batch {
		if len(table.Base) == 0 && len(table.Ext) == 0 {
			continue
		}
		size := 1 << sizeLog2
		out[sizeLog2].Base = make([][]int, len(table.Base))
		for i := range table.Base {
			out[sizeLog2].Base[i], rng = benchShiftList(size, maxShifts, rng)
		}
		out[sizeLog2].Ext = make([][]int, len(table.Ext))
		for i := range table.Ext {
			out[sizeLog2].Ext[i], rng = benchShiftList(size, maxShifts, rng)
		}
	}
	return out
}

// benchSharedShifts gives every row of every present size the same fixed shift
// set (clamped to the size), mimicking a production workload where all rows
// share a handful of distinct claim points (see the sharedShifts field).
func benchSharedShifts(batch Batch, set []int) BatchShifts {
	out := make(BatchShifts, len(batch))
	for sizeLog2, table := range batch {
		if len(table.Base) == 0 && len(table.Ext) == 0 {
			continue
		}
		size := 1 << sizeLog2
		clamped := make([]int, 0, len(set))
		for _, s := range set {
			if s < size {
				clamped = append(clamped, s)
			}
		}
		if len(clamped) == 0 {
			clamped = []int{0}
		}
		out[sizeLog2].Base = make([][]int, len(table.Base))
		for i := range table.Base {
			out[sizeLog2].Base[i] = cloneInts(clamped)
		}
		out[sizeLog2].Ext = make([][]int, len(table.Ext))
		for i := range table.Ext {
			out[sizeLog2].Ext[i] = cloneInts(clamped)
		}
	}
	return out
}

func benchShiftList(size, maxShifts int, rng uint64) ([]int, uint64) {
	if maxShifts > size {
		maxShifts = size
	}
	rng = benchRand(rng)
	count := 1 + int(rng%uint64(maxShifts))
	out := make([]int, 0, count)
	seen := make(map[int]struct{}, count)
	for len(out) < count {
		rng = benchRand(rng)
		shift := int(rng % uint64(size))
		if _, ok := seen[shift]; ok {
			continue
		}
		seen[shift] = struct{}{}
		out = append(out, shift)
	}
	return out, rng
}

func benchChallenges(domainSize, numRounds, numQueries int, seed uint64) Challenges {
	rng := seed
	_, rng = benchExt(rng) // keep stream aligned with bench/main.go (alphaDeep draw)
	foldAlphas := make([]field.Ext, numRounds)
	for i := range foldAlphas {
		foldAlphas[i], rng = benchExt(rng)
	}
	return Challenges{
		FoldAlphas:     foldAlphas,
		QueryPositions: benchQueryPositions(domainSize, numQueries, rng),
	}
}

func benchQueryPositions(domainSize, numQueries int, rng uint64) []int {
	out := make([]int, 0, numQueries)
	seen := make(map[int]struct{}, numQueries)
	for len(out) < numQueries {
		rng = benchRand(rng)
		pos := int(rng % uint64(domainSize))
		if _, ok := seen[pos]; ok {
			continue
		}
		seen[pos] = struct{}{}
		out = append(out, pos)
	}
	return out
}

func benchChallengePoint(seed uint64) field.Ext {
	zeta, _ := benchExt(seed)
	if zeta.B0.A1.IsZero() {
		zeta.B0.A1.SetOne()
	}
	return zeta
}

func benchExt(rng uint64) (field.Ext, uint64) {
	var z field.Ext
	rng = benchRand(rng)
	z.B0.A0.SetUint64(rng)
	rng = benchRand(rng)
	z.B0.A1.SetUint64(rng)
	rng = benchRand(rng)
	z.B1.A0.SetUint64(rng)
	rng = benchRand(rng)
	z.B1.A1.SetUint64(rng)
	rng = benchRand(rng)
	z.B2.A0.SetUint64(rng)
	rng = benchRand(rng)
	z.B2.A1.SetUint64(rng)
	return z, rng
}

func benchRand(x uint64) uint64 {
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	return x * 2685821657736338717
}

func benchClaimedValues(batch Batch, shifts BatchShifts, zeta field.Ext) BatchClaimedValues {
	claimed := make(BatchClaimedValues, len(shifts))
	for sizeLog2, sizedShifts := range shifts {
		sizedWitness := batch[sizeLog2]
		sized := SizedClaimedValues{
			Base: make([][]field.Ext, len(sizedShifts.Base)),
			Ext:  make([][]field.Ext, len(sizedShifts.Ext)),
		}
		for rowIdx, rowShifts := range sizedShifts.Base {
			sized.Base[rowIdx] = benchEvalRow(field.VecFromBase(sizedWitness.Base[rowIdx]), sizeLog2, rowShifts, zeta)
		}
		for rowIdx, rowShifts := range sizedShifts.Ext {
			sized.Ext[rowIdx] = benchEvalRow(field.VecFromExt(sizedWitness.Ext[rowIdx]), sizeLog2, rowShifts, zeta)
		}
		claimed[sizeLog2] = sized
	}
	return claimed
}

func benchEvalRow(poly field.Vec, sizeLog2 int, rowShifts []int, zeta field.Ext) []field.Ext {
	values := make([]field.Ext, len(rowShifts))
	for i, shift := range rowShifts {
		omega := field.RootOfUnityBy(1 << sizeLog2)
		var rotation field.Element
		rotation.Exp(omega, big.NewInt(int64(shift)))
		var point field.Ext
		point.MulByElement(&zeta, &rotation)
		values[i] = polynomials.EvalLagrange(poly, field.ElemFromExt(point)).AsExt()
	}
	return values
}
