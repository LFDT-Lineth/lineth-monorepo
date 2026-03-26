package stitchsplit

import (
	"strings"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/profiling"
)

type StitchingContext struct {
	// The compiled IOP
	Comp *wizard.CompiledIOP
	// All columns under the minSize are ignored.
	// No stitching goes beyond MaxSize.
	MinSize, MaxSize int
	// It collects the information about subColumns and their stitchings.
	// The index of Stitchings is over the rounds.
	Stitchings []SummerizedAlliances
}

type StitchSubColumnsProverAction struct {
	Stitchings []SummerizedAlliances
}

func (a *StitchSubColumnsProverAction) Run(run *wizard.ProverRuntime) {
	for round := range a.Stitchings {
		// This loop is not in deterministic order but this does not matter
		// as this is purely for cleaning up. After stitching, the big column
		// should live and the sub columns should be deleted.
		for subCol := range a.Stitchings[round].BySubCol {
			run.Columns.TryDel(subCol)
		}
	}
}

// Stitcher applies the stitching over the eligible sub columns and adjusts the constraints accordingly.
func Stitcher(minSize, maxSize int) func(comp *wizard.CompiledIOP) {

	return func(comp *wizard.CompiledIOP) {
		// it creates stitchings from the eligible columns and commits to the them.
		ctx := newStitcher(comp, minSize, maxSize)

		// adjust the constraints accordingly over the stitchings of the sub columns.
		ctx.constraints()

		// it assigns the stitching columns and delete the assignment of the sub columns.
		comp.RegisterProverAction(comp.NumRounds()-1, &StitchSubColumnsProverAction{
			Stitchings: ctx.Stitchings,
		})
	}
}

// it commits to the stitchings of the eligible sub columns.
func newStitcher(comp *wizard.CompiledIOP, minSize, maxSize int) StitchingContext {
	numRounds := comp.NumRounds()
	res := StitchingContext{
		Comp:    comp,
		MinSize: minSize,
		MaxSize: maxSize,
		// initialize the stitichings
		Stitchings: make([]SummerizedAlliances, numRounds),
	}
	// it scans the compiler trace for the eligible columns, creates stitchings from the sub columns and commits to the them.
	res.ScanStitchCommit()
	return res
}

type StitchColumnsProverAction struct {
	Ctx   *StitchingContext
	Round int
}

func (a *StitchColumnsProverAction) Run(run *wizard.ProverRuntime) {
	stopTimer := profiling.LogTimer("stitching compiler")
	defer stopTimer()
	var maxSizeGroup int

	// The sorting is necessary to ensure that the iteration below
	// happens in deterministic order over the [ByBigCol] map.
	idBigCols := utils.SortedKeysOf(a.Ctx.Stitchings[a.Round].ByBigCol, func(a, b ifaces.ColID) bool {
		return a < b
	})

	for _, idBigCol := range idBigCols {

		subColumns := a.Ctx.Stitchings[a.Round].ByBigCol[idBigCol]
		maxSizeGroup = a.Ctx.MaxSize / subColumns[0].Size()

		// Sanity-check
		sizeBigCol := a.Ctx.Comp.Columns.GetHandle(idBigCol).Size()
		if sizeBigCol != a.Ctx.MaxSize {
			utils.Panic("Unexpected size %v != %v", sizeBigCol, a.Ctx.MaxSize)
		}

		// If the column is precomputed, it is already assigned
		if a.Ctx.Comp.Precomputed.Exists(idBigCol) {
			continue
		}

		// get the assignment of the subColumns and interleave them
		witnesses := make([]smartvectors.SmartVector, len(subColumns))
		for i := range witnesses {
			witnesses[i] = subColumns[i].GetColAssignment(run)
		}

		assignement := smartvectors.
			AllocateRegular(maxSizeGroup * witnesses[0].Len()).(*smartvectors.Regular)

		const tileSize = 128
		rows := witnesses[0].Len()
		cols := len(subColumns)

		parallel.Execute(rows, func(start, end int) {
			for tileStart := start; tileStart < end; tileStart += tileSize {
				tileEnd := min(tileStart+tileSize, end)
				for j := tileStart; j < tileEnd; j++ {
					base := j * maxSizeGroup
					for i := 0; i < cols; i++ {
						(*assignement)[base+i] = witnesses[i].Get(j)
					}
				}
			}
		})
		run.AssignColumn(idBigCol, assignement)
	}
}

// ScanStitchCommit scans compiler trace and classifies the sub columns eligible
// to the stitching. It then stitches the sub columns, commits to them and
// update stitchingContext. It also forces the compiler to set the status of the
// sub columns to 'ignored'. since the sub columns are technically replaced with
// their stitching.
func (ctx *StitchingContext) ScanStitchCommit() {

	for round := 0; round < ctx.Comp.NumRounds(); round++ {

		// scan the compiler trace to find the eligible columns for stitching.
		// The sorting is critical to ensure that the stitching happens in
		// deterministic order and that the columns are created in the same
		// order.
		columnsBySize := scanAndClassifyEligibleColumns(*ctx, round)
		sizes := utils.SortedKeysOf(columnsBySize, func(a, b int) bool {
			return a < b
		})

		for _, size := range sizes {

			var (
				cols            = columnsBySize[size]
				precomputedCols = make([]ifaces.Column, 0, len(cols))
				committedCols   = make([]ifaces.Column, 0, len(cols))
			)

			// collect the the columns with valid status; Precomputed, committed
			// verifierDefined is valid but is not present in the compiler trace
			// we handle it directly during the constraints.
			for _, col := range cols {
				status := ctx.Comp.Columns.Status(col.GetColID())
				switch status {
				case column.Precomputed:
					precomputedCols = append(precomputedCols, col)
				case column.Committed:
					committedCols = append(committedCols, col)
				default:
					// note that status of verifercol/ veriferDefined is not
					// available via compiler trace.
					utils.Panic("found the column %v with the invalid status %v for stitching", col.GetColID(), status.String())
				}

				// Mark it as ignored, so that it is no longer considered as
				// queryable (since we are replacing it with its stitching).
				ctx.Comp.Columns.MarkAsIgnored(col.GetColID())
			}

			if len(precomputedCols) != 0 {
				// classify the columns to the groups, each of size ctx.MaxSize
				preComputedGroups := groupCols(precomputedCols, ctx.MaxSize/size)

				for _, group := range preComputedGroups {
					// prepare a group for stitching
					stitching := Alliance{
						SubCols: group,
						Round:   round,
						Status:  column.Precomputed,
					}
					// stitch the group
					ctx.stitchGroup(stitching)
				}
			}

			if len(committedCols) != 0 {
				committedGroups := groupCols(committedCols, ctx.MaxSize/size)

				for _, group := range committedGroups {
					stitching := Alliance{
						SubCols: group,
						Round:   round,
						Status:  column.Committed,
					}
					ctx.stitchGroup(stitching)
				}
			}
		}

		if len(ctx.Stitchings[round].ByBigCol) == 0 {
			continue
		}

		ctx.Comp.RegisterProverAction(round, &StitchColumnsProverAction{
			Ctx:   ctx,
			Round: round,
		})
	}
}

// It scan the compiler trace for a given round and classifies the columns eligible to the stitching, by their size.
// It also declares a column with size < minSize a public column (to be verified by the verifier)
func scanAndClassifyEligibleColumns(ctx StitchingContext, round int) map[int][]ifaces.Column {
	columnsBySize := map[int][]ifaces.Column{}

	for _, colName := range ctx.Comp.Columns.AllKeysAt(round) {

		status := ctx.Comp.Columns.Status(colName)
		col := ctx.Comp.Columns.GetHandle(colName)

		// We do not make proof and verifying key column eligible for stitching.
		// We expand them directly during the constraints.
		if status == column.Ignored || status == column.Proof || status == column.VerifyingKey {
			continue
		}

		// If the column is too big, the stitcher does not manipulate the column.
		if col.Size() >= ctx.MaxSize {
			continue
		}

		//  If the column is very small, make it public.
		if col.Size() < ctx.MinSize {
			if status.IsPublic() {
				// Nothing to do : the column is already public and we will ask the
				// verifier to perform the operation itself.
				continue
			}
			ctx.makeColumnPublic(col, status)
			continue
		}

		// Initialization clause of `sizes`
		if _, ok := columnsBySize[col.Size()]; !ok {
			columnsBySize[col.Size()] = []ifaces.Column{}
		}

		columnsBySize[col.Size()] = append(columnsBySize[col.Size()], col)
	}
	return columnsBySize
}

// group the cols with the same size
func groupCols(cols []ifaces.Column, numToStitch int) (groups [][]ifaces.Column) {

	numGroups := utils.DivCeil(len(cols), numToStitch)
	groups = make([][]ifaces.Column, numGroups)

	size := cols[0].Size()

	for i, col := range cols {
		if col.Size() != size {
			utils.Panic(
				"column %v of size %v has been grouped with %v of size %v",
				col.GetColID(), col.Size(), cols[0].GetColID(), cols[0].Size(),
			)
		}
		groups[i/numToStitch] = append(groups[i/numToStitch], col)
	}

	return groups
}

func groupedName(group []ifaces.Column) ifaces.ColID {
	fmtted := make([]string, len(group))
	for i := range fmtted {
		fmtted[i] = group[i].String()
	}
	return ifaces.ColIDf("STITCHER_%v", strings.Join(fmtted, "_"))
}

// for a group of sub columns it creates their stitching.
func (ctx *StitchingContext) stitchGroup(s Alliance) {
	var (
		group        = s.SubCols
		stitchingCol ifaces.Column
		status       = s.Status
	)
	// Declare the new columns
	switch status {
	case column.Precomputed:
		maxSizeGroup := ctx.MaxSize / group[0].Size()
		actualSize := len(group)

		// get the assignment of the subColumns and interleave them
		witnesses := make([]smartvectors.SmartVector, actualSize)
		for i := range witnesses {
			witnesses[i] = ctx.Comp.Precomputed.MustGet(group[i].GetColID())
		}
		assignement := smartvectors.
			AllocateRegular(maxSizeGroup * witnesses[0].Len()).(*smartvectors.Regular)
		for i := range witnesses {
			for j := 0; j < witnesses[0].Len(); j++ {
				(*assignement)[i+j*maxSizeGroup] = witnesses[i].Get(j)
			}
		}

		if assignement.Len() != ctx.MaxSize {
			sizes := []int{}
			sizes2 := []int{}
			for i := range witnesses {
				sizes = append(sizes, witnesses[i].Len())
				sizes2 = append(sizes2, group[i].Size())
			}
			utils.Panic("creating a column bigger than it should: maxsize=%v totalsize=%v sizes=%v sizes2=%v inputs=%v", ctx.MaxSize, assignement.Len(), sizes, sizes2, group)
		}

		stitchingCol = ctx.Comp.InsertPrecomputed(
			groupedName(group),
			assignement)
	case column.Committed:
		stitchingCol = ctx.Comp.InsertCommit(
			s.Round,
			groupedName(s.SubCols),
			ctx.MaxSize,
		)

	default:
		panic("The status is not valid for the stitching")

	}

	s.BigCol = stitchingCol
	(MultiSummary)(ctx.Stitchings).InsertNew(s)
}

// it checks if the column belongs to a stitching.
func isColEligibleStitching(stitchings MultiSummary, col ifaces.Column) bool {
	natural := column.RootParents(col)
	_, found := stitchings[col.Round()].BySubCol[natural.GetColID()]
	return found
}

// It makes the given colum public.
// If the colum is Precomputed it becomes the VerifierKey, otherwise it becomes Proof.
func (ctx StitchingContext) makeColumnPublic(col ifaces.Column, status column.Status) {

	switch status {
	case column.Precomputed:
		// send it to the verifier directly as part of the verifying key
		ctx.Comp.Columns.SetStatus(col.GetColID(), column.VerifyingKey)
	case column.Committed:
		// send it to the verifier directly as part of the proof
		ctx.Comp.Columns.SetStatus(col.GetColID(), column.Proof)
	default:
		utils.Panic("Unknown status : %v", status.String())
	}
}
