package execution_data_collector

import (
	"fmt"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// PadderPacker is used to format the data so that we can compute a Poseidon-hash
// of the limbs in the execution data collector.
// It works in three steps:
// first, the input limbs are rearranged into a single column (OneColumn) with a filter (OneColumnFilter)
// that indicates which rows are active. In this step, we also track the number of bytes and add it to a
// OneColumnBytes column. We also compute the sum of bytes in each segment of 8 limbs in the OneColumnBytesSum column,
// which we cross check with the input number of bytes at the start of each segment of 8.

// In the second step, we remove the inactive gaps of the OneColumn using a projection query into
// the OneColumnWithoutGaps column, with a corresponding FilterWithoutGaps column. This is done by using the OneColumnBytes as a
// filter for the projection query, since it indicates which limbs are non-zero and which are zero.

// In the third step, we pad with zeroes until reaching a multiple of 8 limbs, which is the size of the input for the Poseidon hash.
// This is done by using a CounterColumn that counts the number of active limbs in the OneColumnWithoutGaps column for each segment of 8,
// and then filling a CounterColumnPadded column with the padded counter values.
// We also have a FilterWithoutGapsPadded column which extends the FilterWithoutGaps column to cover the padded rows,
// Note that FilterWithoutGapsPadded will initially have the usual shape of an isActive filter,
// potentially followed by non-binary values in the padded area.
// FilterWithoutGapsPadded is used as a filter for the final projection query that outputs the OuterColumns,
// which are the columns that will be used as input for the Poseidon hash.
// We also output an OuterIsActive column that indicates which rows in the OuterColumns are active.

// PadderPacker will padd pack the data of the ExecutionDataCollector into 8 columns
// without gaps with 0 limbs, so that they can be Poseidon-hashed.
type PadderPacker struct {
	// the input limbs to be packed and padded
	InputLimbs [common.NbLimbU128]ifaces.Column
	// The number of bytes in the limbs.
	InputNoBytes ifaces.Column
	// the is active part of the input
	InputIsActive ifaces.Column
	// Step 0 columns
	SplitInputLimbs [2 * common.NbLimbU128]ifaces.Column
	// Step 1 columns
	// OneColumn is a column that contains the limbs of the input consecutively, so that we can apply the periodic filters on it and get the sum of bytes in each segment of 8 limbs.
	OneColumn         ifaces.Column
	OneColumnFilter   ifaces.Column
	OneColumnBytes    ifaces.Column
	OneColumnBytesSum ifaces.Column
	// Helper filters that do not depend on the data. These are periodic filters with period 8.
	PeriodicFilter [2 * common.NbLimbU128]ifaces.Column
	// for the 0 index period filter, we need a trimmed version of it which only lights up on OneColumnFilter active rows
	TrimmedPeriodicFilter ifaces.Column
	// Step 2 columns. The inactive gaps of the OneColumn are removed here.
	OneColumnWithoutGaps ifaces.Column
	FilterWithoutGaps    ifaces.Column
	// Helper columns that allow to pad with zeroes up to a multiple of 8.
	CounterColumn           ifaces.Column
	CounterColumnPadded     ifaces.Column
	FilterWithoutGapsPadded ifaces.Column
	// selectors for the prover to compute the values in the SelectorCounterColumnPadded column
	SelectorCounterColumnPadded        ifaces.Column
	ComputeSelectorCounterColumnPadded wizard.ProverAction
	// Step 3 columns. The final output columns after padding and packing
	// OuterColumns are the output columns that will be used to compute the Poseidon hash.
	SplitOuter   [2 * common.NbLimbU128]ifaces.Column
	OuterColumns [common.NbLimbU128]ifaces.Column
	// the isActive part of the output
	OuterIsActive ifaces.Column
}

// NewPoseidonPadderPacker returns a new GenericPadderPacker with initialized columns that are not constrained.
func NewPadderPacker(comp *wizard.CompiledIOP, inputLimbs [common.NbLimbU128]ifaces.Column, inputNoBytes, inputIsActive ifaces.Column, name string) PadderPacker {
	var (
		res     PadderPacker
		newSize int
	)
	res.InputLimbs = inputLimbs
	res.InputNoBytes = inputNoBytes
	res.InputIsActive = inputIsActive

	oldSize := res.InputLimbs[0].Size()
	newSize = oldSize * 2 * common.NbLimbU128
	res.OneColumn = util.CreateCol(name, "ONE_COLUMN", newSize, comp)
	res.OneColumnFilter = util.CreateCol(name, "ONE_COLUMN_FILTER", newSize, comp)
	res.OneColumnBytes = util.CreateCol(name, "ONE_COLUMN_BYTES", newSize, comp)
	res.OneColumnBytesSum = util.CreateCol(name, "ONE_COLUMN_BYTE_SUM", newSize, comp)
	res.OneColumnWithoutGaps = util.CreateCol(name, "ONE_COLUMN_WITHOUT_GAPS", newSize, comp)
	res.FilterWithoutGaps = util.CreateCol(name, "FILTER_WITHOUT_GAPS", newSize, comp)
	res.CounterColumn = util.CreateCol(name, "COUNTER_COLUMN", newSize, comp)
	res.CounterColumnPadded = util.CreateCol(name, "COUNTER_COLUMN_PADDED", newSize, comp)
	res.FilterWithoutGapsPadded = util.CreateCol(name, "FILTER_WITHOUT_GAPS_PADDED", newSize, comp)

	for i := range res.PeriodicFilter {
		res.PeriodicFilter[i] = util.CreateCol(name, fmt.Sprintf("PERIODIC_FILTER_%d", i), newSize, comp)
	}
	res.TrimmedPeriodicFilter = util.CreateCol(name, fmt.Sprintf("TRIMMED_PERIODIC_FILTER"), newSize, comp)

	for i := range res.SplitInputLimbs {
		res.SplitInputLimbs[i] = util.CreateCol(name, fmt.Sprintf("SPLIT_INPUT_LIMBS_%d", i), oldSize, comp)
	}

	for i := range res.SplitOuter {
		res.SplitOuter[i] = util.CreateCol(name, fmt.Sprintf("SPLIT_OUTER_COLUMN_%d", i), oldSize, comp)
	}

	for i := range res.OuterColumns {
		res.OuterColumns[i] = util.CreateCol(name, fmt.Sprintf("OUTER_COLUMN_%d", i), oldSize, comp)
	}
	res.OuterIsActive = util.CreateCol(name, "OUTPUT_IS_ACTIVE", oldSize, comp)
	return res
}

// DefineHasher specifies the constraints of the GenericPadderPacker with respect to the ExtractedData fetched from the arithmetization
func DefinePadderPacker(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	DefinePeriodicFilters(comp, ppp, name)
	// Step 0
	SplitLimbsCopyConstraint(comp, ppp, name)
	// Step 1
	DefineOneColumnNBytesConstraints(comp, ppp, name)
	DefineTrimmedPeriodicFilter(comp, ppp, name)
	DefineOneColumnFilter(comp, ppp, name)
	DefineStepOneProjectionQueries(comp, ppp, name)
	// Step 2
	DefineFilterWithoutGaps(comp, ppp, name)
	DefineStepTwoProjectionQueries(comp, ppp, name)
	// Step 3
	DefinePadderPackerSelectorConstraints(comp, ppp, name)
	DefineCounterPadding(comp, ppp, name)
	DefineFilterWithoutGapsPadded(comp, ppp, name)
	DefineOuterActiveFilter(comp, ppp, name)
	DefineStepThreeProjectionQueries(comp, ppp, name)
	SplitOuterCopyConstraint(comp, ppp, name)
}

func DefinePeriodicFilters(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	for j := range ppp.PeriodicFilter {
		// constraints for the filter at index j
		util.MustBeBinary(comp, ppp.PeriodicFilter[j])

		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_PERIODIC_PATTERN_SAME_VALUE_AFTER_EACH_SEGMENT_%d", name, j),
			sym.Mul(
				ppp.PeriodicFilter[j],
				sym.Sub(
					ppp.PeriodicFilter[j],
					column.Shift(ppp.PeriodicFilter[j], -2*common.NbLimbU128),
				),
			),
		)

		comp.InsertLocal(0,
			ifaces.QueryIDf("%s_PERIODIC_PATTERN_INIT_ONE_%d", name, j),
			sym.Sub(
				column.Shift(ppp.PeriodicFilter[j], j),
				1),
		)
		for index := 0; index < 2*common.NbLimbU128; index++ {
			if index != j {
				// for all these positions, we must have zeroes
				comp.InsertLocal(0,
					ifaces.QueryIDf("%s_PERIODIC_PATTERN_INIT_ZERO_%d_%d", name, j, index),
					ifaces.ColumnAsVariable(column.Shift(ppp.PeriodicFilter[j], index)),
				)
			}
		}
	}
}

func SplitLimbsCopyConstraint(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	for i := 0; i < 8; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_SPLIT_BYTES_COPY_CONSTRAINT_%d", name, i),
			sym.Sub(
				ppp.InputLimbs[i],
				sym.Add(
					sym.Mul(
						ppp.SplitInputLimbs[2*i],
						field.NewElement(0x100),
					),
					ppp.SplitInputLimbs[2*i+1],
				),
			))
	}
}

func DefineCounterPadding(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	// Constrain CounterColumn
	for i := 0; i < 2*common.NbLimbU128; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_COUNTER_VALUE_%d", name, i),
			sym.Mul(
				ppp.FilterWithoutGaps, // on the active part of the column without gaps
				ppp.PeriodicFilter[i], // at position i in each segment of 8
				sym.Sub(ppp.CounterColumn, field.NewElement(uint64(i))), // CounterColumn must be equal to i
			),
		)
	}

	// Constraint: After last active row, the counter filling continues until reaching 2*0 (multiple of 2*8)
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_COUNTER_PADDING_BORDER", name),
		sym.Mul(
			sym.Sub(field.NewElement(1), column.Shift(ppp.FilterWithoutGaps, 1)),
			ppp.FilterWithoutGaps,          // at the border of FilterWithoutGaps, the active part of the column without gaps
			sym.Sub(15, ppp.CounterColumn), // must be !=15, if the CounterColumn is 15, it means we are in the lucky case where we are already at a multiple of 2*8 and we do not need to fill in more
			sym.Sub(
				// CounterColumnPadded increases by 1 compared to CounterColumn
				// at the border
				column.Shift(ppp.CounterColumnPadded, 1),
				ppp.CounterColumn,
				1,
			),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_COUNTER_PADDING_FILLING_CORRECTNESS", name),
		sym.Mul(
			ppp.CounterColumnPadded,            // CounterColumnPadded > 0
			sym.Sub(1, ppp.PeriodicFilter[15]), // we are not at the end of the size 8 block
			sym.Sub(
				// CounterColumnPadded increases by 1 compared to CounterColumnPadded
				column.Shift(ppp.CounterColumnPadded, 1),
				ppp.CounterColumnPadded,
				1,
			),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_COUNTER_PADDING_FILLING_ZEROIZATION", name),
		sym.Mul(
			ppp.PeriodicFilter[0],                  // we are at the end of the size 8 block
			ifaces.Column(ppp.CounterColumnPadded), // force CounterColumnPadded to be zero
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_COUNTER_PADDING_FILLING_NEVER_REACTIVATES", name),
		sym.Mul(
			ppp.PeriodicFilter[0],                  // we are at the end of the size 8 block
			ifaces.Column(ppp.CounterColumnPadded), // force CounterColumnPadded to be zero
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_COUNTER_PADDING_FILLING_NEVER_INCREASES_FROM_O_TO_1_AGAIN", name),
		sym.Mul(
			sym.Sub(1, ppp.FilterWithoutGaps),        // on the inactive part of the column without gaps
			ppp.SelectorCounterColumnPadded,          // 1 when CounterColumnPadded is 0
			column.Shift(ppp.CounterColumnPadded, 1), // require that when CounterColumnPadded is 0, the next one is also 0
		),
	)
}

func DefinePadderPackerSelectorConstraints(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	// We first compute the prover actions
	ppp.SelectorCounterColumnPadded, ppp.ComputeSelectorCounterColumnPadded = dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(ppp.CounterColumnPadded),
	).GetColumnAndProverAction()
}

func DefineOneColumnNBytesConstraints(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_ONE_COLUMN_NBYTES_SUM", name),
		sym.Mul(
			ppp.PeriodicFilter[0], // when the Periodic filter is at the beginning of a segment of 8
			sym.Sub(
				ppp.OneColumnBytesSum, // OneColumnBytesSum must be equal to the sum of OneColumnBytes over the current and next 7 rows
				ppp.OneColumnBytes,
				column.Shift(ppp.OneColumnBytes, 1),
				column.Shift(ppp.OneColumnBytes, 2),
				column.Shift(ppp.OneColumnBytes, 3),
				column.Shift(ppp.OneColumnBytes, 4),
				column.Shift(ppp.OneColumnBytes, 5),
				column.Shift(ppp.OneColumnBytes, 6),
				column.Shift(ppp.OneColumnBytes, 7),
				column.Shift(ppp.OneColumnBytes, 8),
				column.Shift(ppp.OneColumnBytes, 9),
				column.Shift(ppp.OneColumnBytes, 10),
				column.Shift(ppp.OneColumnBytes, 11),
				column.Shift(ppp.OneColumnBytes, 12),
				column.Shift(ppp.OneColumnBytes, 13),
				column.Shift(ppp.OneColumnBytes, 14),
				column.Shift(ppp.OneColumnBytes, 15),
			),
		),
	)

	for j := 0; j < 2*common.NbLimbU128-1; j++ {
		// for all the periodic filters except the last one
		// the last one being excluded ensures that the blocks get reset
		// the number of bytes loaded must have the pattern 2, 2, 2, 0, 0... in a segment of 8
		// once it reaches 0, it must stay 0 until the end of the segment of 8
		// j stops before common.NbLimbU128-1, because we allow for a reset at the start of the next segment
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_NBYTES_ZERO_PROPAGATION_%d", name, j),
			sym.Mul(
				ppp.PeriodicFilter[j],
				sym.Mul( // (2-OneColumnBytes[i])*OneColumnBytes(i+1)=0
					sym.Sub(
						field.NewElement(1),
						ppp.OneColumnBytes,
					),
					column.Shift(ppp.OneColumnBytes, 1),
				),
			),
		)
	}
}

func DefineTrimmedPeriodicFilter(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_TRIMMED_FIRST_PERIODIC_FILTER", name),
		sym.Sub(
			ppp.TrimmedPeriodicFilter, // when the Periodic filter is at the beginning of a segment of 8
			sym.Mul(
				ppp.OneColumnFilter,
				ppp.PeriodicFilter[0],
			),
		),
	)
}

func DefineOneColumnFilter(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	// the OneColumnFilter is also implicitly constrained by the step 1 projection queries
	util.MustBeBinary(comp, ppp.OneColumnFilter)
	IsActivePattern(comp, ppp.OneColumnFilter)
}

func DefineStepOneProjectionQueries(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	// copy the number of bytes in a row of the execution data collector into the OneColumnBytesSum column at the start of each segment of 8 rows
	comp.InsertProjection(
		ifaces.QueryIDf("%s_PADDER_PACKER_ONE_COLUMN_PROJECTION_BYTES_SUM", name),
		query.ProjectionInput{
			ColumnA: []ifaces.Column{ppp.InputNoBytes},
			ColumnB: []ifaces.Column{ppp.OneColumnBytesSum},
			FilterA: ppp.InputIsActive,
			FilterB: ppp.TrimmedPeriodicFilter})

	// projection query to copy the limbs into the OneColumn column
	// this also helps to ensure that the OneColumnFilter is correctly set
	comp.InsertProjection(
		ifaces.QueryIDf("%s_PROJECTION_STEP_1", name),
		query.ProjectionMultiAryInput{
			ColumnsA: [][]ifaces.Column{
				[]ifaces.Column{ppp.SplitInputLimbs[0]},
				[]ifaces.Column{ppp.SplitInputLimbs[1]},
				[]ifaces.Column{ppp.SplitInputLimbs[2]},
				[]ifaces.Column{ppp.SplitInputLimbs[3]},
				[]ifaces.Column{ppp.SplitInputLimbs[4]},
				[]ifaces.Column{ppp.SplitInputLimbs[5]},
				[]ifaces.Column{ppp.SplitInputLimbs[6]},
				[]ifaces.Column{ppp.SplitInputLimbs[7]},
				[]ifaces.Column{ppp.SplitInputLimbs[8]},
				[]ifaces.Column{ppp.SplitInputLimbs[9]},
				[]ifaces.Column{ppp.SplitInputLimbs[10]},
				[]ifaces.Column{ppp.SplitInputLimbs[11]},
				[]ifaces.Column{ppp.SplitInputLimbs[12]},
				[]ifaces.Column{ppp.SplitInputLimbs[13]},
				[]ifaces.Column{ppp.SplitInputLimbs[14]},
				[]ifaces.Column{ppp.SplitInputLimbs[15]},
			},
			ColumnsB: [][]ifaces.Column{
				[]ifaces.Column{ppp.OneColumn},
			},
			FiltersA: []ifaces.Column{
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
				ppp.InputIsActive,
			},
			FiltersB: []ifaces.Column{
				ppp.OneColumnFilter,
			},
		},
	)
}

func DefineStepTwoProjectionQueries(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	comp.InsertProjection(
		ifaces.QueryIDf("%s_PADDER_PACKER_STEP_TWO_ONE_COLUMN_NO_GAPS_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: []ifaces.Column{ppp.OneColumn},
			ColumnB: []ifaces.Column{ppp.OneColumnWithoutGaps},
			FilterA: ppp.OneColumnBytes, // we use OneColumnBytes, since this will represent which limbs are 0 and non-zero
			FilterB: ppp.FilterWithoutGaps})
}

func DefineFilterWithoutGaps(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	util.MustBeBinary(comp, ppp.FilterWithoutGaps)
	IsActivePattern(comp, ppp.FilterWithoutGaps)
}

func DefineStepThreeProjectionQueries(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	comp.InsertProjection(
		ifaces.QueryIDf("%s_PADDER_PACKER_STEP_3_FROM_ONE_COLUMN_TO_16_COLUMNS", name),
		query.ProjectionMultiAryInput{
			ColumnsA: [][]ifaces.Column{
				[]ifaces.Column{ppp.OneColumnWithoutGaps},
			},
			ColumnsB: [][]ifaces.Column{
				[]ifaces.Column{ppp.SplitOuter[0]},
				[]ifaces.Column{ppp.SplitOuter[1]},
				[]ifaces.Column{ppp.SplitOuter[2]},
				[]ifaces.Column{ppp.SplitOuter[3]},
				[]ifaces.Column{ppp.SplitOuter[4]},
				[]ifaces.Column{ppp.SplitOuter[5]},
				[]ifaces.Column{ppp.SplitOuter[6]},
				[]ifaces.Column{ppp.SplitOuter[7]},
				[]ifaces.Column{ppp.SplitOuter[8]},
				[]ifaces.Column{ppp.SplitOuter[9]},
				[]ifaces.Column{ppp.SplitOuter[10]},
				[]ifaces.Column{ppp.SplitOuter[11]},
				[]ifaces.Column{ppp.SplitOuter[12]},
				[]ifaces.Column{ppp.SplitOuter[13]},
				[]ifaces.Column{ppp.SplitOuter[14]},
				[]ifaces.Column{ppp.SplitOuter[15]},
			},
			FiltersA: []ifaces.Column{
				ppp.FilterWithoutGapsPadded,
			},
			FiltersB: []ifaces.Column{
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
				ppp.OuterIsActive,
			},
		},
	)
}

func DefineFilterWithoutGapsPadded(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	// FilterWithoutGapsPadded is a binary filter covering both active rows
	// (where FilterWithoutGaps=1) and padded rows (where CounterColumnPadded>0).
	// We use (1 - SelectorCounterColumnPadded) which is 1 when
	// CounterColumnPadded != 0, and 0 otherwise. Since the active and padded
	// regions don't overlap, their sum is always 0 or 1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_FILTER_WITHOUT_GAPS_PADDED", name),
		sym.Sub(
			ppp.FilterWithoutGapsPadded,
			ppp.FilterWithoutGaps,
			sym.Sub(1, ppp.SelectorCounterColumnPadded),
		),
	)
}

func DefineOuterActiveFilter(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	IsActivePattern(comp, ppp.OuterIsActive)
}

func IsActivePattern(comp *wizard.CompiledIOP, col ifaces.Column) {
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_ACTIVE_CONSTRAINT_NO_0_TO_1", col.GetColID()),
		sym.Sub(
			col,
			sym.Mul(
				column.Shift(col, -1),
				col,
			),
		),
	)
}

func SplitOuterCopyConstraint(comp *wizard.CompiledIOP, ppp *PadderPacker, name string) {
	for i := 0; i < 8; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_SPLIT_OUTER_BYTES_COPY_CONSTRAINT_%d", name, i),
			sym.Sub(
				ppp.OuterColumns[i],
				sym.Add(
					sym.Mul(
						ppp.SplitOuter[2*i],
						field.NewElement(0x100),
					),
					ppp.SplitOuter[2*i+1],
				),
			))
	}
}

// AssignHasher assigns the data in the GenericPadderPacker using the ExtractedData fetched from the arithmetization
func AssignPadderPacker(run *wizard.ProverRuntime, ppp PadderPacker) {
	oneColumn := make([]field.Element, ppp.OneColumn.Size())
	oneColumnWithoutGaps := make([]field.Element, ppp.OneColumnWithoutGaps.Size())
	filterWithoutGaps := make([]field.Element, ppp.FilterWithoutGaps.Size())
	oneColumnFilter := make([]field.Element, ppp.OneColumn.Size())
	periodicFilter := make([][]field.Element, len(ppp.PeriodicFilter))
	oneColumnBytes := make([]field.Element, ppp.OneColumn.Size())
	oneColumnBytesSum := make([]field.Element, ppp.OneColumn.Size())
	counterColumn := make([]field.Element, ppp.CounterColumn.Size())
	counterColumnPadded := make([]field.Element, ppp.CounterColumnPadded.Size())
	for j := range ppp.PeriodicFilter {
		periodicFilter[j] = make([]field.Element, ppp.PeriodicFilter[j].Size())
	}

	splitInput := make([][]field.Element, len(ppp.SplitInputLimbs))
	for j := range ppp.SplitInputLimbs {
		splitInput[j] = make([]field.Element, ppp.SplitInputLimbs[j].Size())
	}

	splitOuter := make([][]field.Element, len(ppp.SplitOuter))
	for j := range ppp.SplitOuter {
		splitOuter[j] = make([]field.Element, ppp.SplitOuter[j].Size())
	}
	outer := make([][]field.Element, len(ppp.OuterColumns))
	for j := range ppp.OuterColumns {
		outer[j] = make([]field.Element, ppp.OuterColumns[j].Size())
	}
	outerIsActive := make([]field.Element, ppp.OuterIsActive.Size())

	for i := 0; i < ppp.SplitInputLimbs[0].Size(); i++ {
		for j := 0; j < common.NbLimbU128; j++ {
			limbValue := ppp.InputLimbs[j].GetColAssignmentAt(run, i)
			limbBytes := limbValue.Bytes()
			splitInput[2*j][i].SetBytes(limbBytes[2:3])
			splitInput[2*j+1][i].SetBytes(limbBytes[3:4])
		}
	}

	counterRow := 0
	for i := 0; i < ppp.SplitInputLimbs[0].Size(); i++ {
		for j := 0; j < 2*common.NbLimbU128; j++ {
			periodicFilter[j][counterRow].SetOne()
			counterRow++
		}
	}

	for j := range ppp.SplitInputLimbs {
		run.AssignColumn(ppp.SplitInputLimbs[j].GetColID(), sv.NewRegular(splitInput[j]))
	}

	counterRow = 0

	for i := 0; i < ppp.SplitInputLimbs[0].Size(); i++ {
		isActive := ppp.InputIsActive.GetColAssignmentAt(run, i)
		nBytesLimb := ppp.InputNoBytes.GetColAssignmentAt(run, i)
		nBytesLimbInt := int(nBytesLimb.Uint64())
		remainingNBytes := nBytesLimbInt
		if isActive.IsOne() {
			for j := 0; j < 2*common.NbLimbU128; j++ {
				if j == 0 {
					oneColumnBytesSum[counterRow].SetUint64(uint64(nBytesLimbInt))
				}
				limbValue := ppp.SplitInputLimbs[j].GetColAssignmentAt(run, i)
				limbBytes := limbValue.Bytes()
				oneColumnFilter[counterRow].SetOne()
				if remainingNBytes > 0 {
					oneColumnBytes[counterRow].SetUint64(1)
					oneColumn[counterRow].SetBytes(limbBytes[3:4])
					remainingNBytes--
				}
				counterRow++
			}
		}
	}

	counterRow = 0
	for i := 0; i < len(oneColumn); i++ {
		if !oneColumnBytes[i].IsZero() {
			oneColumnWithoutGaps[counterRow].Set(&oneColumn[i])
			filterWithoutGaps[counterRow].SetOne()
			counterRow++
		}
	}

	lastRow := 0
	for i := 0; i < len(oneColumnWithoutGaps); i++ {
		if filterWithoutGaps[i].IsOne() {
			splitOuter[i%(2*common.NbLimbU128)][i/(2*common.NbLimbU128)].Set(&oneColumnWithoutGaps[i])
			// outerIsActive[i%common.NbLimbU128][i/common.NbLimbU128].SetOne()
			outerIsActive[i/(2*common.NbLimbU128)].SetOne()
			counterColumn[i].SetUint64(uint64(i % (2 * common.NbLimbU128)))
			// counterColumnPadded[i].SetUint64(uint64(i % common.NbLimbU128))
			lastRow = i
		}
	}
	// Pad counterColumnPadded to fill the current block of 16 rows.
	// The check uses (lastRow+1) because lastRow is 0-indexed: e.g. 17 active
	// rows means lastRow=16, and (16+1)%16=1 ≠ 0, so padding is needed.
	// We also guard against the edge case where there are no active rows.
	if filterWithoutGaps[lastRow].IsOne() && (lastRow+1)%(2*common.NbLimbU128) != 0 {
		for i := lastRow + 1; i%(2*common.NbLimbU128) != 0; i++ {
			counterColumnPadded[i].SetUint64(uint64(i % (2 * common.NbLimbU128)))
		}
	}

	for j := range ppp.SplitOuter {
		run.AssignColumn(ppp.SplitOuter[j].GetColID(), sv.NewRegular(splitOuter[j]))
	}

	for i := range ppp.OuterColumns[0].Size() {
		for j := 0; j < common.NbLimbU128; j++ {
			fieldMostSignificant := ppp.SplitOuter[2*j].GetColAssignmentAt(run, i)
			fieldLeastSignificant := ppp.SplitOuter[2*j+1].GetColAssignmentAt(run, i)
			digitFactor := field.NewElement(0x100)
			var limbValue field.Element
			limbValue.Mul(&fieldMostSignificant, &digitFactor)
			limbValue.Add(&limbValue, &fieldLeastSignificant)
			outer[j][i].Set(&limbValue)
		}
	}

	run.AssignColumn(ppp.OneColumn.GetColID(), sv.NewRegular(oneColumn))
	run.AssignColumn(ppp.OneColumnFilter.GetColID(), sv.NewRegular(oneColumnFilter))
	run.AssignColumn(ppp.OneColumnWithoutGaps.GetColID(), sv.NewRegular(oneColumnWithoutGaps))
	run.AssignColumn(ppp.FilterWithoutGaps.GetColID(), sv.NewRegular(filterWithoutGaps))
	run.AssignColumn(ppp.OneColumnBytes.GetColID(), sv.NewRegular(oneColumnBytes))
	run.AssignColumn(ppp.OneColumnBytesSum.GetColID(), sv.NewRegular(oneColumnBytesSum))
	run.AssignColumn(ppp.CounterColumn.GetColID(), sv.NewRegular(counterColumn))
	run.AssignColumn(ppp.CounterColumnPadded.GetColID(), sv.NewRegular(counterColumnPadded))
	// FilterWithoutGapsPadded is binary: 1 where either filterWithoutGaps or counterColumnPadded is non-zero
	filterWithoutGapsPadded := make([]field.Element, len(filterWithoutGaps))
	for i := range filterWithoutGaps {
		if !filterWithoutGaps[i].IsZero() || !counterColumnPadded[i].IsZero() {
			filterWithoutGapsPadded[i].SetOne()
		}
	}
	run.AssignColumn(ppp.FilterWithoutGapsPadded.GetColID(), sv.NewRegular(filterWithoutGapsPadded))
	for j := range ppp.PeriodicFilter {
		run.AssignColumn(ppp.PeriodicFilter[j].GetColID(), sv.NewRegular(periodicFilter[j]))
	}
	run.AssignColumn(ppp.TrimmedPeriodicFilter.GetColID(), sv.Mul(sv.NewRegular(periodicFilter[0]), sv.NewRegular(oneColumnFilter)))

	for j := range ppp.OuterColumns {
		run.AssignColumn(ppp.OuterColumns[j].GetColID(), sv.NewRegular(outer[j]))
	}
	run.AssignColumn(ppp.OuterIsActive.GetColID(), sv.NewRegular(outerIsActive))

	ppp.ComputeSelectorCounterColumnPadded.Run(run)
}
