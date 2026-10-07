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
		entries := make([]sEntry, n)
		for i := range entries {
			entries[i] = sEntry{frag: uint32(rng.IntN(4)), row: uint32(i)}
			entries[i].val.B0.A0.SetUint64(rng.Uint64())
		}
		for _, numBuckets := range []int{1, 8, 1024} {
			mask := uint32(numBuckets - 1)
			bucket := func(e *sEntry) uint32 { return extHash(&e.val) & mask }
			want := make([][]sEntry, numBuckets)
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
