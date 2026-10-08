package lookuptologderivsum

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// partitionByBucket must group exactly like appending every entry to its
// bucket in input order, whatever the number of workers.
func TestPartitionByBucket_MatchesSerialAppend(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	for _, n := range []int{0, 1, 100, 3 * rowsPerChunk, 10*rowsPerChunk + 17} {
		entries := make([]tEntry, n)
		for i := range entries {
			entries[i] = tEntry{idx: uint32(i)}
			entries[i].val.B0.A0.SetUint64(rng.Uint64())
		}
		for _, numBuckets := range []int{1, 8, 1024} {
			mask := uint32(numBuckets - 1)
			bucket := func(e *tEntry) uint32 { return extHash(&e.val) & mask }
			want := make([][]tEntry, numBuckets)
			for _, e := range entries {
				want[bucket(&e)] = append(want[bucket(&e)], e)
			}
			for _, workers := range []int{1, 3, 16} {
				got := partitionByBucket(entries, numBuckets, bucket, workers)
				require.Len(t, got, numBuckets)
				for b := range want {
					require.Lenf(t, got[b], len(want[b]), "n=%d buckets=%d workers=%d bucket %d", n, numBuckets, workers, b)
					for i := range want[b] {
						require.Equal(t, want[b][i], got[b][i])
					}
				}
			}
		}
	}
}

// The M columns of a lookup group must not depend on the worker budget of the
// M-assignment: tables large enough to be split into many chunks, with
// duplicated table values, a filtered including fragment and a filtered
// included fragment, give the same multiplicities with one worker as with
// many. The multiplicities do not depend on the random hashing scalar, so
// separate runs are comparable.
func TestMAssignment_IndependentOfWorkers(t *testing.T) {
	const tSize, sSize = 1 << 15, 1 << 16
	rng := rand.New(rand.NewPCG(11, 13))

	sys := wiop.NewSystemf("m-assign-workers")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), tSize, wiop.PaddingDirectionRight)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), sSize, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), r0)
	selT := modT.NewColumn(sys.Context.Childf("selT"), r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), r0)
	selS := modS.NewColumn(sys.Context.Childf("selS"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(selS.View(), colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(selT.View(), colT.View())},
	)
	Compile(sys)

	var batch *mAssignmentBatch
	for _, a := range r0.ProverActions {
		if b, ok := a.(*mAssignmentBatch); ok {
			batch = b
		}
	}
	require.NotNil(t, batch, "Compile must register an M-assignment batch on the witness round")

	// T holds values 0..1023, each repeated; every active S row looks up an
	// active T value.
	tVals, tSel := make([]field.Element, tSize), make([]field.Element, tSize)
	for i := range tVals {
		tVals[i].SetUint64(uint64(i % 1024))
		tSel[i].SetOne()
	}
	sVals, sSel := make([]field.Element, sSize), make([]field.Element, sSize)
	for i := range sVals {
		sVals[i].SetUint64(uint64(rng.IntN(1024)))
		if rng.IntN(3) != 0 {
			sSel[i].SetOne()
		}
	}
	vec := func(v []field.Element) *wiop.ConcreteVector { return &wiop.ConcreteVector{Plain: field.VecFromBase(v)} }

	mColumns := func(workers int) [][]field.Element {
		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colT, vec(tVals))
		rt.AssignColumn(selT, vec(tSel))
		rt.AssignColumn(colS, vec(sVals))
		rt.AssignColumn(selS, vec(sSel))
		var out [][]field.Element
		for _, task := range batch.tasks {
			task.run(rt, workers)
			for _, m := range task.ms {
				out = append(out, rt.GetColumnAssignment(m).Plain.AsBase())
			}
		}
		return out
	}

	want := mColumns(1)
	var active, total uint64
	for _, s := range sSel {
		active += s.Uint64()
	}
	for _, m := range want {
		for _, v := range m {
			total += v.Uint64()
		}
	}
	require.Equal(t, active, total, "the multiplicities must count every active S row once")
	for _, workers := range []int{2, 7, 64} {
		require.Equal(t, want, mColumns(workers), "workers=%d", workers)
	}
}

// The M columns must charge every active A row to the latest occurrence
// (highest fragment, then highest row) of its value in the union of the B
// fragments, checked against a naive tuple-by-tuple count. The B fragments
// are padded on different sides, one is filtered, values repeat within and
// across fragments, and the A side reads a shifted column under a filter.
func TestMAssignment_MatchesNaive(t *testing.T) {
	const n1, n2, an = 1 << 10, 1 << 9, 1 << 12
	rng := rand.New(rand.NewPCG(17, 19))
	sys := wiop.NewSystemf("m-assign-naive")
	r0 := sys.NewRound()
	modB1 := sys.NewSizedModule(sys.Context.Childf("B1"), n1, wiop.PaddingDirectionLeft)
	modB2 := sys.NewSizedModule(sys.Context.Childf("B2"), n2, wiop.PaddingDirectionRight)
	modA := sys.NewSizedModule(sys.Context.Childf("A"), an, wiop.PaddingDirectionNone)
	b1x, b1y, b1s := modB1.NewColumn(sys.Context.Childf("b1x"), r0), modB1.NewColumn(sys.Context.Childf("b1y"), r0),
		modB1.NewColumn(sys.Context.Childf("b1s"), r0)
	b2x, b2y := modB2.NewColumn(sys.Context.Childf("b2x"), r0), modB2.NewColumn(sys.Context.Childf("b2y"), r0)
	ax, ay, as := modA.NewColumn(sys.Context.Childf("ax"), r0), modA.NewColumn(sys.Context.Childf("ay"), r0),
		modA.NewColumn(sys.Context.Childf("as"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(as.View(), ax.View(), ay.View().Shift(3))},
		[]wiop.Table{
			wiop.NewFilteredTable(b1s.View(), b1x.View(), b1y.View()),
			wiop.NewTable(b2x.View(), b2y.View()),
		},
	)
	Compile(sys)
	var batch *mAssignmentBatch
	for _, a := range r0.ProverActions {
		if b, ok := a.(*mAssignmentBatch); ok {
			batch = b
		}
	}
	require.NotNil(t, batch)

	// B values are pairs (x, y) with x, y < 8, so they repeat; B1 holds 3/4
	// of its rows (left-padded with zeros) and filters a third of them out.
	small := func(k int) []field.Element {
		v := make([]field.Element, k)
		for i := range v {
			v[i].SetUint64(uint64(rng.IntN(8)))
		}
		return v
	}
	b1Len := 3 * n1 / 4
	b1xv, b1yv, b2xv, b2yv := small(b1Len), small(b1Len), small(n2), small(n2)
	b1sv := make([]field.Element, b1Len)
	for i := range b1sv {
		if rng.IntN(3) != 0 {
			b1sv[i].SetOne()
		}
	}
	// Every pair (x, y) with x, y < 8 appears in B2 (unfiltered), so any
	// A row built from small values has a match.
	for i := range 64 {
		b2xv[i].SetUint64(uint64(i / 8))
		b2yv[i].SetUint64(uint64(i % 8))
	}
	axv, ayv := small(an), small(an)
	asv := make([]field.Element, an)
	for i := range asv {
		if rng.IntN(4) != 0 {
			asv[i].SetOne()
		}
	}
	vec := func(v []field.Element) *wiop.ConcreteVector { return &wiop.ConcreteVector{Plain: field.VecFromBase(v)} }
	rt := wiop.NewRuntime(sys)
	for c, v := range map[*wiop.Column][]field.Element{
		b1x: b1xv, b1y: b1yv, b1s: b1sv, b2x: b2xv, b2y: b2yv, ax: axv, ay: ayv, as: asv,
	} {
		rt.AssignColumn(c, vec(v))
	}
	require.Len(t, batch.tasks, 1)
	task := batch.tasks[0]
	task.run(rt, 4)

	// Naive: the effective B tuples are (selector, x, y) for B1 (padded rows
	// read as zeros) and (1, x, y) for B2; the A tuples are (1, x, y[i+3]).
	type tuple [3]uint64
	padded := func(c *wiop.Column) []field.Element { return c.View().EvaluateVector(rt).Plain.AsBase() }
	b1 := [3][]field.Element{padded(b1s), padded(b1x), padded(b1y)}
	latest := map[tuple][2]int{}
	for i := range n1 {
		latest[tuple{b1[0][i].Uint64(), b1[1][i].Uint64(), b1[2][i].Uint64()}] = [2]int{0, i}
	}
	for i := range n2 {
		latest[tuple{1, b2xv[i].Uint64(), b2yv[i].Uint64()}] = [2]int{1, i}
	}
	want := [][]uint64{make([]uint64, n1), make([]uint64, n2)}
	for i := range an {
		if asv[i].IsZero() {
			continue
		}
		pos := latest[tuple{1, axv[i].Uint64(), ayv[(i+3)%an].Uint64()}]
		want[pos[0]][pos[1]]++
	}
	for frag, m := range task.ms {
		got := rt.GetColumnAssignment(m).Plain.AsBase()
		for i := range got {
			require.Equal(t, want[frag][i], got[i].Uint64(), "fragment %d row %d", frag, i)
		}
	}
}
