package logderivativesum

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// ProverTaskAtRound implements the [wizard.ProverAction] interface. It gathers
// all the operations related to all compiled tables altogether that have to be
// done at a particular round.
//
// Namely, if applied to the round N. The action will be responsible for
// assigning the M column for tables compiled on round N and the SigmaS/SigmaT
// and their respective LocalOpening for the tables compiled at round N-1.
//
// All these actions are performed in parallel.
type ProverTaskAtRound struct {

	// MAssignmentTasks lists all the tasks consisting of assigning the column
	// M related to table that are scheduled in the current interaction round.
	MAssignmentTasks []MAssignmentTask

	// ZAssignmentTasks lists all the tasks consisting of assigning the
	// columns SigmaS and SigmaT for the given round.
	ZAssignmentTasks []ZAssignmentTask
}

// Run implements the [wizard.ProverAction interface]. The tasks will spawn
// a goroutine for each tasks and wait for all of them to finish. The approach
// for parallelization can be justified if the number of go-routines stays low
// (e.g. less than 1000s).
func (p ProverTaskAtRound) Run(run *wizard.ProverRuntime) {

	wg := &sync.WaitGroup{}
	wg.Add(p.numTasks())

	var (
		panicTrace []byte
		panicMsg   any
		panicOnce  = &sync.Once{}
	)

	for i := range p.MAssignmentTasks {
		go func(i int) {

			// In case the subtask panics, we recover so that we can repanic in
			// the main goroutine. Simplifying the process of tracing back the
			// error and allowing to test the panics.
			defer func() {
				if r := recover(); r != nil {
					panicOnce.Do(func() {
						panicMsg = r
						panicTrace = debug.Stack()

						inspectWiop(run)
					})
				}

				wg.Done()
			}()

			p.MAssignmentTasks[i].Run(run)
		}(i)
	}

	for i := range p.ZAssignmentTasks {
		go func(i int) {

			// In case the subtask panics, we recover so that we can repanic in
			// the main goroutine. Simplifying the process of tracing back the
			// error and allowing to test the panics.
			defer func() {
				if r := recover(); r != nil {
					panicOnce.Do(func() {
						panicMsg = r
						panicTrace = debug.Stack()
					})
				}

				wg.Done()
			}()
			p.ZAssignmentTasks[i].Run(run)
		}(i)
	}

	wg.Wait()

	if len(panicTrace) > 0 {
		utils.Panic("Had a panic: %v\nStack: %v\n", panicMsg, string(panicTrace))
	}
}

// pushMAssignment appends an [mAssignmentTask] to the list of tasks
func (p *ProverTaskAtRound) pushMAssignment(m MAssignmentTask) {
	p.MAssignmentTasks = append(p.MAssignmentTasks, m)
}

// pushZAssignment appends an [sigmaAssignmentTask] to the list of tasks
func (p *ProverTaskAtRound) pushZAssignment(s ZAssignmentTask) {
	p.ZAssignmentTasks = append(p.ZAssignmentTasks, s)
}

// numTasks returns the total number of tasks that are scheduled in the
// [proverTaskAtRound].
func (p *ProverTaskAtRound) numTasks() int {
	return len(p.MAssignmentTasks) + len(p.ZAssignmentTasks)
}

// MAssignmentTask specifically represent the prover task of computing and
// assigning the [singleTableCtx.M] for a particular table. M is computing the
// appearance of the rows of T in the rows of S.
type MAssignmentTask struct {

	// M is the column that the assignMWork
	M []ifaces.Column

	// T the lookup table to which the task is related
	T []table

	// S is the list of checked tables for which inclusion within T is enforced
	// by a compiled query.
	S []table

	// SFilter stores the filters that are applied for each table S.
	SFilter []ifaces.Column

	// Segmenter is the segmenter that will be used to omit some of the values
	// from the "S" column.
	Segmenter ColumnSegmenter
}

// Run executes the task represented by the receiver of the method. Namely, it
// actually computes the value of M.
//
// The implementation uses a Partitioned Hash Join (or Radix Join) strategy to
// efficiently count occurrences of rows from S in T. This approach is chosen
// over a single large hash map to improve memory locality and allow for
// fine-grained parallelism.
//
// The process involves:
//  1. Collapsing columns: If tables have multiple columns, they are collapsed
//     into a single column using a random linear combination.
//  2. Partitioning T: The rows of T are hashed and distributed into buckets.
//  3. Partitioning S: The rows of S are hashed and distributed into buckets.
//  4. Processing Buckets: Each bucket is processed independently to count
//     occurrences using a local hash map.
//
// In case one of the Ss contains an entry that does not appear in T, the
// function panics. This aims at early detecting that the lookup query is not
// satisfied.
func (a MAssignmentTask) Run(run *wizard.ProverRuntime) {

	var (
		// isMultiColumn flags whether the table have multiple column and
		// whether the "collapsing" trick is needed.
		isMultiColumn = len(a.T[0]) > 1

		// tCollapsed contains either the assignment of T if the table is a
		// single column (e.g. isMultiColumn=false) or its collapsed version
		// otherwise.
		tCollapsed = make([]sv.SmartVector, len(a.T))

		// sCollapsed contains either the assignment of the Ss if the table is a
		// single column (e.g. isMultiColumn=false) or their collapsed version
		// otherwise.
		sCollapsed = make([]sv.SmartVector, len(a.S))
	)

	if !isMultiColumn {
		for frag := range a.T {
			tCollapsed[frag] = a.T[frag][0].GetColAssignment(run)
		}

		for i := range a.S {
			sCollapsed[i] = a.S[i][0].GetColAssignment(run)
		}
	}

	if isMultiColumn {
		// collapsingRandomness is the randomness used in the collapsing trick.
		// It is sampled via `crypto/rand` internally to ensure it cannot be
		// predicted ahead of time by an adversary.
		var collapsingRandomness fext.Element
		collapsingRandomness.MustSetRandom()

		for frag := range a.T {
			tCollapsed[frag] = wizardutils.RandLinCombColAssignment(run, collapsingRandomness, a.T[frag])
		}

		for i := range a.S {
			sCollapsed[i] = wizardutils.RandLinCombColAssignment(run, collapsingRandomness, a.S[i])
		}
	}

	var (
		// m  is associated with tCollapsed
		// m stores the assignment to the column M as we build it.
		m = make([][]field.Element, len(a.T))
	)

	for frag := range a.T {
		m[frag] = make([]field.Element, tCollapsed[frag].Len())
	}

	// Partitioning configuration
	numCPU := runtime.NumCPU()
	numBuckets := 1
	for numBuckets < numCPU*4 {
		numBuckets *= 2
	}

	// Power-of-two bucket count so we can mask instead of modulo on the hot path.
	mask := uint32(numBuckets - 1)

	type tEntry struct {
		val  fext.Element
		frag uint32
		row  uint32
	}

	type sEntry struct {
		val          fext.Element
		multiplicity field.Element
	}

	hash := func(v *fext.Element) uint32 {
		h := v.B0.A0.Uint64()
		h = (h * 31) ^ v.B0.A1.Uint64()
		h = (h * 31) ^ v.B1.A0.Uint64()
		h = (h * 31) ^ v.B1.A1.Uint64()
		return uint32(h)
	}

	// --- Partition T ---
	// We split T into chunks to process them in parallel.
	numChunksT := numCPU * 4
	if numChunksT > len(a.T) {
		numChunksT = len(a.T)
	}
	if numChunksT == 0 {
		numChunksT = 1
	}

	tRanges := make([]struct{ start, end int }, numChunksT)
	{
		base := len(a.T) / numChunksT
		rem := len(a.T) % numChunksT
		current := 0
		for i := 0; i < numChunksT; i++ {
			end := current + base
			if i < rem {
				end++
			}
			tRanges[i] = struct{ start, end int }{current, end}
			current = end
		}
	}

	tChunkCounts := make([][]int, numChunksT)

	// Pass 1: Count
	// Count how many items fall into each bucket for each chunk of T.
	parallel.Execute(numChunksT, func(startChunk, endChunk int) {
		for c := startChunk; c < endChunk; c++ {
			localCounts := make([]int, numBuckets)
			start, end := tRanges[c].start, tRanges[c].end
			for frag := start; frag < end; frag++ {
				size := tCollapsed[frag].Len()
				rangeStart, rangeEnd := 0, size
				if a.Segmenter != nil {
					root, ok := column.RootsOf(a.T[frag], true)[0].(column.Natural)
					if !ok {
						utils.Panic("col %v should be a column.Natural %++v", root.ID, root)
					}
					rangeStart, rangeEnd = a.Segmenter.SegmentBoundaryOf(run, root)
				}
				// Segment boundaries can extend past the physical size; clamp here and
				// handle out-of-range multiplicities later when processing S.
				rangeStart = max(0, rangeStart)
				rangeEnd = min(size, rangeEnd)

				for k := rangeStart; k < rangeEnd; k++ {
					v := tCollapsed[frag].GetExt(k)
					h := hash(&v)
					localCounts[h&mask]++
				}
			}
			tChunkCounts[c] = localCounts
		}
	})

	// Pass 2: Offsets
	// Calculate the starting offset for each bucket in the global tBuckets array.
	tBuckets := make([][]tEntry, numBuckets)
	bucketSizes := make([]int, numBuckets)
	for _, counts := range tChunkCounts {
		for b, c := range counts {
			bucketSizes[b] += c
		}
	}
	for b := 0; b < numBuckets; b++ {
		tBuckets[b] = make([]tEntry, bucketSizes[b])
	}
	tChunkOffsets := make([][]int, numChunksT)
	currentOffsets := make([]int, numBuckets)
	for c := 0; c < numChunksT; c++ {
		offsets := make([]int, numBuckets)
		copy(offsets, currentOffsets)
		tChunkOffsets[c] = offsets
		for b, count := range tChunkCounts[c] {
			currentOffsets[b] += count
		}
	}

	// Pass 3: Fill
	// Populate the buckets with the actual entries from T.
	parallel.Execute(numChunksT, func(startChunk, endChunk int) {
		for c := startChunk; c < endChunk; c++ {
			offsets := tChunkOffsets[c]
			localOffsets := make([]int, numBuckets)
			copy(localOffsets, offsets)

			start, end := tRanges[c].start, tRanges[c].end
			for frag := start; frag < end; frag++ {
				size := tCollapsed[frag].Len()
				rangeStart, rangeEnd := 0, size
				if a.Segmenter != nil {
					root, ok := column.RootsOf(a.T[frag], true)[0].(column.Natural)
					if !ok {
						utils.Panic("col %v should be a column.Natural %++v", root.ID, root)
					}
					rangeStart, rangeEnd = a.Segmenter.SegmentBoundaryOf(run, root)
				}
				rangeStart = max(0, rangeStart)
				rangeEnd = min(size, rangeEnd)

				for k := rangeStart; k < rangeEnd; k++ {
					v := tCollapsed[frag].GetExt(k)
					h := hash(&v)
					b := h & mask
					pos := localOffsets[b]
					tBuckets[b][pos] = tEntry{val: v, frag: uint32(frag), row: uint32(k)}
					localOffsets[b]++
				}
			}
		}
	})

	// --- Partition S ---
	// We split S into chunks to process them in parallel.
	numChunksS := numCPU * 4
	if numChunksS > len(sCollapsed) {
		numChunksS = len(sCollapsed)
	}
	if numChunksS == 0 {
		numChunksS = 1
	}

	sRanges := make([]struct{ start, end int }, numChunksS)
	{
		base := len(sCollapsed) / numChunksS
		rem := len(sCollapsed) % numChunksS
		current := 0
		for i := 0; i < numChunksS; i++ {
			end := current + base
			if i < rem {
				end++
			}
			sRanges[i] = struct{ start, end int }{current, end}
			current = end
		}
	}

	sChunkCounts := make([][]int, numChunksS)

	// Pass 1: Count
	// Count how many items fall into each bucket for each chunk of S.
	parallel.Execute(numChunksS, func(startChunk, endChunk int) {
		for c := startChunk; c < endChunk; c++ {
			localCounts := make([]int, numBuckets)
			start, end := sRanges[c].start, sRanges[c].end
			for i := start; i < end; i++ {
				size := sCollapsed[i].Len()
				startSeg, stopSeg := 0, size
				hasFilter := a.SFilter[i] != nil
				var filter []field.Element

				if a.Segmenter != nil {
					sCol := column.RootsOf(a.S[i], true)[0].(column.Natural)
					startSeg, stopSeg = a.Segmenter.SegmentBoundaryOf(run, sCol)
				}

				if hasFilter {
					filter = a.SFilter[i].GetColAssignment(run).IntoRegVecSaveAlloc()
				}

				for k := max(0, startSeg); k < min(stopSeg, size); k++ {
					if hasFilter {
						if filter[k].IsZero() {
							continue
						}
						if !filter[k].IsOne() {
							err := fmt.Errorf(
								"the filter column `%v` has a non-binary value at position `%v`: (%v)",
								a.SFilter[i].GetColID(),
								k,
								filter[k].String(),
							)
							exit.OnUnsatisfiedConstraints(err)
						}
					}

					v := sCollapsed[i].GetExt(k)
					h := hash(&v)
					localCounts[h&mask]++
				}
			}
			sChunkCounts[c] = localCounts
		}
	})

	// Pass 2: Offsets
	// Calculate the starting offset for each bucket in the global sBuckets array.
	sBuckets := make([][]sEntry, numBuckets)
	sBucketSizes := make([]int, numBuckets)
	for _, counts := range sChunkCounts {
		for b, c := range counts {
			sBucketSizes[b] += c
		}
	}
	for b := 0; b < numBuckets; b++ {
		sBuckets[b] = make([]sEntry, sBucketSizes[b])
	}

	sChunkOffsets := make([][]int, numChunksS)
	sCurrentOffsets := make([]int, numBuckets)
	for c := 0; c < numChunksS; c++ {
		offsets := make([]int, numBuckets)
		copy(offsets, sCurrentOffsets)
		sChunkOffsets[c] = offsets
		for b, count := range sChunkCounts[c] {
			sCurrentOffsets[b] += count
		}
	}

	// Pass 3: Fill
	// Populate the buckets with the actual entries from S.
	parallel.Execute(numChunksS, func(startChunk, endChunk int) {
		for c := startChunk; c < endChunk; c++ {
			offsets := sChunkOffsets[c]
			localOffsets := make([]int, numBuckets)
			copy(localOffsets, offsets)

			start, end := sRanges[c].start, sRanges[c].end
			for i := start; i < end; i++ {
				size := sCollapsed[i].Len()
				startSeg, stopSeg := 0, size
				hasFilter := a.SFilter[i] != nil
				var filter []field.Element

				if a.Segmenter != nil {
					sCol := column.RootsOf(a.S[i], true)[0].(column.Natural)
					startSeg, stopSeg = a.Segmenter.SegmentBoundaryOf(run, sCol)
				}

				if hasFilter {
					filter = a.SFilter[i].GetColAssignment(run).IntoRegVecSaveAlloc()
				}

				for k := max(0, startSeg); k < min(stopSeg, size); k++ {
					if hasFilter {
						if filter[k].IsZero() {
							continue
						}
					}

					v := sCollapsed[i].GetExt(k)

					// multiplicity handles the case where the segment boundary
					// extends beyond the physical table size. In this case, the
					// boundary elements are treated as repeated for the
					// out-of-bounds indices.
					multiplicity := field.One()
					switch {
					case k == 0 && startSeg < 0:
						multiplicity = field.NewElement(uint64(-startSeg + 1))
					case k == size-1 && stopSeg > size:
						multiplicity = field.NewElement(uint64(stopSeg - size + 1))
					}

					h := hash(&v)
					b := h & mask
					pos := localOffsets[b]
					sBuckets[b][pos] = sEntry{val: v, multiplicity: multiplicity}
					localOffsets[b]++
				}
			}
		}
	})

	// Process Buckets
	// Each bucket contains a subset of T and S that share the same hash values.
	// We can now process each bucket independently to count occurrences.
	parallel.Execute(numBuckets, func(start, end int) {
		for b := start; b < end; b++ {
			count := len(tBuckets[b])

			mapM := make(map[fext.Element][2]uint32, count)

			for _, entry := range tBuckets[b] {
				existing, ok := mapM[entry.val]
				if !ok {
					mapM[entry.val] = [2]uint32{entry.frag, entry.row}
					continue
				}

				// Preserve the latest occurrence to mirror the pre-optimization
				// behavior, where the last seen duplicate overwrote earlier ones.
				if entry.frag > existing[0] || (entry.frag == existing[0] && entry.row >= existing[1]) {
					mapM[entry.val] = [2]uint32{entry.frag, entry.row}
				}
			}

			for _, entry := range sBuckets[b] {
				pos, ok := mapM[entry.val]
				if !ok {
					utils.Panic(
						"entry %v is not included in the table. T-mapSize=%v T-name=%v S-tables=%v",
						entry.val, len(mapM), NameTable(a.T), NameTable(a.S),
					)
				}

				mFrag, posInFragM := pos[0], pos[1]
				m[mFrag][posInFragM].Add(&m[mFrag][posInFragM], &entry.multiplicity)
			}
		}
	})

	for frag := range m {
		run.AssignColumn(a.M[frag].GetColID(), sv.NewRegular(m[frag]), wizard.DisableAssignmentSizeReduction)
	}

}

// ZAssignmentTask represents a prover task of assignming the columns
// SigmaS and SigmaT for a specific lookup table.
type ZAssignmentTask ZCtx

// columnWindowRange determines the active (non-padding) window across all
// column inputs referenced by the given expression boards. It returns
// (0, size) if the window cannot be narrowed (e.g. columns are dense).
func columnWindowRange(run *wizard.ProverRuntime, size int, boards ...sym.ExpressionBoard) (start, stop int) {
	start, stop = 0, size
	foundWindow := false
	seen := make(map[ifaces.ColID]bool)

	for _, board := range boards {
		for _, m := range board.ListVariableMetadata() {
			col, ok := m.(ifaces.Column)
			if !ok {
				continue
			}
			id := col.GetColID()
			if seen[id] {
				continue
			}
			seen[id] = true

			assignment := col.GetColAssignment(run)
			if assignment == nil {
				return 0, size
			}

			switch a := assignment.(type) {
			case *sv.PaddedCircularWindow:
				wStart := a.Offset()
				wStop := a.Offset() + len(a.Window())
				if wStop > a.Len() {
					return 0, size
				}
				if !foundWindow {
					start = wStart
					stop = wStop
					foundWindow = true
				} else {
					start = min(start, wStart)
					stop = max(stop, wStop)
				}
			case *sv.PaddedCircularWindowExt:
				wStart := a.Offset
				wStop := a.Offset + len(a.Window_)
				if wStop > a.Len() {
					return 0, size
				}
				if !foundWindow {
					start = wStart
					stop = wStop
					foundWindow = true
				} else {
					start = min(start, wStart)
					stop = max(stop, wStop)
				}
			case *sv.Constant, *sv.ConstantExt:
				continue // constants span the full range, don't constrain window
			default:
				return 0, size // unknown type, can't optimize
			}
		}
	}

	return start, stop
}

func (z ZAssignmentTask) Run(run *wizard.ProverRuntime) {

	parallel.Execute(len(z.ZDenominatorBoarded), func(start, stop int) {

		sb0 := make(field.Vector, z.Size)
		se0 := make(extensions.Vector, z.Size)

		for frag := start; frag < stop; frag++ {

			numeratorMetadata := z.ZNumeratorBoarded[frag].ListVariableMetadata()

			svDenominator := column.EvalExprColumn(run, z.ZDenominatorBoarded[frag])

			// This case does not corresponds to an actual production case
			// because log-derivative sums are always defined in such a way that
			// the denominator depends on a randomness. The case is still here
			// for completeness but we don't optimize for it.
			if sv.IsBase(svDenominator) {
				numerator := sb0
				denominator := svDenominator.IntoRegVecSaveAlloc()
				packedZ := field.BatchInvert(denominator)
				if len(numeratorMetadata) == 0 {
					for i := range numerator {
						numerator[i].SetOne()
					}
				} else {
					evalResult := column.EvalExprColumn(run, z.ZNumeratorBoarded[frag])
					evalResult.WriteInSlice(numerator)
				}
				vp := field.Vector(packedZ)
				vp.Mul(vp, numerator)
				for k := 1; k < len(packedZ); k++ {
					packedZ[k].Add(&packedZ[k], &packedZ[k-1])
				}

				run.AssignColumn(z.Zs[frag].GetColID(), sv.NewRegular(packedZ))
				run.AssignLocalPointExt(z.ZOpenings[frag].ID, fext.Lift(packedZ[len(packedZ)-1]))
				continue
			}

			// We are dealing with extension denominators.
			// Detect the active (non-padding) window from the input columns.
			// If columns are PaddedCircularWindow with low utilization, we can
			// batch-invert only the active sub-range and handle padding cheaply.
			wStart, wEnd := columnWindowRange(run, z.Size,
				z.ZDenominatorBoarded[frag], z.ZNumeratorBoarded[frag])
			activeLen := wEnd - wStart

			denominator := svDenominator.IntoRegVecSaveAllocExt()
			totalLen := len(denominator)

			// Evaluate numerator
			var numerator extensions.Vector
			if len(numeratorMetadata) == 0 {
				numerator = se0
				for i := range numerator {
					numerator[i].SetOne()
				}
			} else {
				evalResult := column.EvalExprColumn(run, z.ZNumeratorBoarded[frag])
				if vr, ok := evalResult.(*sv.RegularExt); ok {
					numerator = extensions.Vector(*vr)
				} else {
					numerator = se0
					evalResult.WriteInSliceExt(numerator)
				}
			}

			if activeLen < totalLen {
				// SPARSE PATH: batch-invert only the active window and compute
				// the constant padding fraction once.
				//
				// For padding positions, denominator and numerator are constant
				// (they arise from constant padding values in the lookup
				// columns + the random coin). We compute padFrac = padNum/padDen
				// once and accumulate it per position in the prefix sum.
				padPos := 0
				if wStart == 0 {
					padPos = totalLen - 1
				}
				var padInvDen, padFrac fext.Element
				padInvDen.Inverse(&denominator[padPos])
				padFrac.Mul(&numerator[padPos], &padInvDen)

				// Batch-invert only the active sub-range.
				activeInv := fext.ParBatchInvert(denominator[wStart:wEnd], 0)

				// Build the Z prefix sum.
				packedZ := make(extensions.Vector, totalLen)
				var running fext.Element

				// Phase 1: pre-padding [0, wStart)
				for i := 0; i < wStart; i++ {
					running.Add(&running, &padFrac)
					packedZ[i] = running
				}

				// Phase 2: active window [wStart, wEnd)
				for i := 0; i < activeLen; i++ {
					var frac fext.Element
					frac.Mul(&numerator[wStart+i], &activeInv[i])
					running.Add(&running, &frac)
					packedZ[wStart+i] = running
				}

				// Phase 3: post-padding [wEnd, totalLen)
				for i := wEnd; i < totalLen; i++ {
					running.Add(&running, &padFrac)
					packedZ[i] = running
				}

				run.AssignColumn(z.Zs[frag].GetColID(), sv.NewRegularExt([]fext.Element(packedZ)))
				run.AssignLocalPointExt(z.ZOpenings[frag].ID, packedZ[totalLen-1])
			} else {
				// DENSE PATH: original algorithm.
				packedZ := fext.ParBatchInvert(denominator, 0)

				vp := extensions.Vector(packedZ)
				vp.Mul(vp, numerator)

				for k := 1; k < len(packedZ); k++ {
					packedZ[k].Add(&packedZ[k], &packedZ[k-1])
				}

				run.AssignColumn(z.Zs[frag].GetColID(), sv.NewRegularExt(packedZ))
				run.AssignLocalPointExt(z.ZOpenings[frag].ID, packedZ[len(packedZ)-1])
			}
		}
	})

}

func inspectWiop(run *wizard.ProverRuntime) {

	if true {
		return
	}

	columns := run.Spec.Columns.AllKeys()

	fmt.Printf("Name; HasPragmaFullCol; HasPragmaLeftPadded; HasPragmaRightPadded; RangeStart; RangeEnd; Size")

	for _, colID := range columns {

		var (
			col                     = run.Spec.Columns.GetHandle(colID).(column.Natural)
			_, hasPragmaFullCol     = col.GetPragma(pragmas.FullColumnPragma)
			_, hasPragmaLeftPadded  = col.GetPragma(pragmas.LeftPadded)
			_, hasPragmaRightPadded = col.GetPragma(pragmas.RightPadded)
		)

		if !run.Columns.Exists(colID) {
			fmt.Printf("%v; %v; %v; %v; %v; %v; %v\n", colID, hasPragmaFullCol, hasPragmaLeftPadded, hasPragmaRightPadded, "N/A", "N/A", col.Size())
			continue
		}

		var (
			v                    = col.GetColAssignment(run)
			rangeStart, rangeEnd = sv.CoCompactRange(v)
			size                 = v.Len()
		)

		fmt.Printf("%v; %v; %v; %v; %v; %v; %v\n", colID, hasPragmaFullCol, hasPragmaLeftPadded, hasPragmaRightPadded, rangeStart, rangeEnd, size)
	}
}
