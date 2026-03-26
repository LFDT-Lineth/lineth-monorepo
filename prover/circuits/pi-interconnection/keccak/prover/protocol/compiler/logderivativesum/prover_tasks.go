package logderivativesum

import (
	"fmt"
	"runtime/debug"
	"sync"

	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

// proverTaskAtRound implements the [wizard.ProverAction] interface. It gathers
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
		// the passing of the index `i` is there to ensure that the go-routine
		// is running over a local copy of `i` which is not incremented every
		// time the loop goes to the next iteration.
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
		// the passing of the index `i` is there to ensure that the go-routine
		// is running over a local copy of `i` which is not incremented every
		// time the loop goes to the next iteration.
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

// mAssignmentWork specifically represent the prover task of computing and
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
// In the case where the table has a single column, the execution path is
// straightforward: simply counting values in a hashmap. In the multi-column
// case there is a trick going on. The prover samples a random value and does
// the counting over a linear combination of the rows using the powers of the
// randomness as coefficient. We refer to this step as "collapsing" the columns.
//
// This crucially relies on the actual randomness of the sampled randomness.
// Without this, a malicious actor may send a proof request which will
// invalidate the counting (and thus, the whole proof later on).
//
// Note that this trick is completely distinct from the sampling of the coin
// Alpha and is purely internal to the prover's work. Therefore, it cannot be
// a soundness concern.
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

		// fragmentUnionSize contains the total number of rows contained in all
		// the fragments of T combined.
		fragmentUnionSize int
	)

	if !isMultiColumn {
		for frag := range a.T {
			tCollapsed[frag] = a.T[frag][0].GetColAssignment(run)
			fragmentUnionSize += a.T[frag][0].Size()
		}

		for i := range a.S {
			sCollapsed[i] = a.S[i][0].GetColAssignment(run)
		}
	}

	if isMultiColumn {
		// collapsingRandomness is the randomness used in the collapsing trick.
		// It is sampled via `crypto/rand` internally to ensure it cannot be
		// predicted ahead of time by an adversary.
		var collapsingRandomness field.Element
		if _, err := collapsingRandomness.SetRandom(); err != nil {
			utils.Panic("could not sample the collapsing randomness: %v", err.Error())
		}

		for frag := range a.T {
			tCollapsed[frag] = wizardutils.RandLinCombColAssignment(run, collapsingRandomness, a.T[frag])
			fragmentUnionSize += tCollapsed[frag].Len()
		}

		for i := range a.S {
			sCollapsed[i] = wizardutils.RandLinCombColAssignment(run, collapsingRandomness, a.S[i])
		}
	}

	var (
		// m  is associated with tCollapsed
		// m stores the assignment to the column M as we build it.
		m = make([][]field.Element, len(a.T))

		// mapm collects the entries in the inclusion set to their positions
		// in tCollapsed. If T contains duplicates, the first position is the
		// one that is kept in mapM.
		//
		// It is used to let us know where an entry of S appears in T. The stored
		// 2-uple of integers indicate [fragment, row]
		mapM = make(map[field.Element][2]uint32, fragmentUnionSize)
	)

	// This loops initializes mapM so that it tracks to the positions of the
	// entries of T. It also preinitializes the values of ms
	for frag := range a.T {

		size := tCollapsed[frag].Len()
		start, end := 0, tCollapsed[frag].Len()

		// The segment tells us what range of T[frag] will be actually
		// included in the segments after the segmentation. It range can be
		// either larger or smaller than the size of T[frag]. In the former case
		// we can just index the full size of T[frag] and decide that the
		// multiplicity associated with the extension of T[frag] are all zeroes
		// 0. In the latter case, we only index the segmented part of T[frag]
		// (implictly, the remaining part of T[frag] are all padding).
		if a.Segmenter != nil {
			root, ok := column.RootsOf(a.T[frag], true)[0].(column.Natural)
			if !ok {
				utils.Panic("col %v should be a column.Natural %++v", root.ID, root)
			}
			start, end = a.Segmenter.SegmentBoundaryOf(run, root)
		}

		m[frag] = make([]field.Element, tCollapsed[frag].Len())

		for k := max(0, start); k < min(size, end); k++ {
			v := tCollapsed[frag].Get(k)
			mapM[v] = [2]uint32{uint32(frag), uint32(k)}
		}
	}

	// This loops counts all the occurences of the rows of T within S and store
	// them into S.
	for i := range sCollapsed {

		var (
			size        = sCollapsed[i].Len()
			start, stop = 0, size
			hasFilter   = a.SFilter[i] != nil
			filter      []field.Element
		)

		if a.Segmenter != nil {
			sCol := column.RootsOf(a.S[i], true)[0].(column.Natural)
			start, stop = a.Segmenter.SegmentBoundaryOf(run, sCol)
		}

		if hasFilter {
			filter = a.SFilter[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		}

		for k := max(0, start); k < min(stop, size); k++ {

			// Implicitly, continuing here means that we exclude the whole
			// "extended" part of S from the lookup.
			if hasFilter && filter[k].IsZero() {
				continue
			}

			if hasFilter && !filter[k].IsOne() {
				err := fmt.Errorf(
					"the filter column `%v` has a non-binary value at position `%v`: (%v)",
					a.SFilter[i].GetColID(),
					k,
					filter[k].String(),
				)

				// Even if this is unconstrained, this is still worth interrupting the
				// prover because it "should" be a binary column.
				exit.OnUnsatisfiedConstraints(err)
			}

			var (
				// v stores the entry of S that we are examining and looking for
				// in the look up table.
				v = sCollapsed[i].Get(k)

				// posInM stores the position of `v` in the look-up table
				posInM, ok = mapM[v]
			)

			if !ok {
				tableRow := make([]field.Element, len(a.S[i]))
				for j := range tableRow {
					tableRow[j] = a.S[i][j].GetColAssignmentAt(run, k)
				}

				err := fmt.Errorf(
					"entry %v of the table %v is not included in the table. tableRow=%v T-mapSize=%v T-name=%v",
					k, NameTable([][]ifaces.Column{a.S[i]}), vector.Prettify(tableRow), len(mapM), NameTable(a.T),
				)

				exit.OnUnsatisfiedConstraints(err)
			}

			mFrag, posInFragM := posInM[0], posInM[1]

			// In case, the S table gets virtually expanded we account for it
			// by adding multiplicities for the first value is the table is
			// left-padded orthe last value if the table is right padded. This
			// corresponds to the behaviour that the module segmenter will have.
			//
			// Note: that if we reach the current segment, it implicly means
			// that can can't have filter[k] == 0.
			mk := field.One()
			switch {
			case k == 0 && start < 0 && !hasFilter:
				mk = field.NewElement(uint64(-start + 1))
			case k == size-1 && stop > size && !hasFilter:
				mk = field.NewElement(uint64(stop - size + 1))
			}

			m[mFrag][posInFragM].Add(&m[mFrag][posInFragM], &mk)
		}

	}

	for frag := range m {
		run.AssignColumn(a.M[frag].GetColID(), sv.NewRegular(m[frag]), wizard.DisableAssignmentSizeReduction)
	}

}

// ZAssignmentTask represents a prover task of assignming the columns
// SigmaS and SigmaT for a specific lookup table.
// sigmaAssignment
type ZAssignmentTask ZCtx

func (z ZAssignmentTask) Run(run *wizard.ProverRuntime) {
	parallel.Execute(len(z.ZDenominatorBoarded), func(start, stop int) {
		for frag := start; frag < stop; frag++ {

			var (
				numeratorMetadata = z.ZNumeratorBoarded[frag].ListVariableMetadata()
				denominator       = column.EvalExprColumn(run, z.ZDenominatorBoarded[frag]).IntoRegVecSaveAlloc()
				numerator         []field.Element
				packedZ           = field.BatchInvert(denominator)
			)

			if len(numeratorMetadata) == 0 {
				numerator = vector.Repeat(field.One(), z.Size)
			}

			if len(numeratorMetadata) > 0 {
				numerator = column.EvalExprColumn(run, z.ZNumeratorBoarded[frag]).IntoRegVecSaveAlloc()
			}

			for k := range packedZ {
				packedZ[k].Mul(&numerator[k], &packedZ[k])
				if k > 0 {
					packedZ[k].Add(&packedZ[k], &packedZ[k-1])
				}
			}

			run.AssignColumn(z.Zs[frag].GetColID(), sv.NewRegular(packedZ))
			run.AssignLocalPoint(z.ZOpenings[frag].ID, packedZ[len(packedZ)-1])
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
