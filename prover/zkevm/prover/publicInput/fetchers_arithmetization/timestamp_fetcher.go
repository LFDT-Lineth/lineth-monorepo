package fetchers_arithmetization

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

const (
	// TimestampOffset is the corresponding offset position for the timestamp
	// since it is a shift, -1 means no offset.
	TimestampOffset = -12
)

// TimestampFetcher is a struct used to fetch the timestamps from the arithmetization's BlockDataCols
type TimestampFetcher struct {
	// RelBlock is the relative block number, ranging from 1 to the total number of blocks
	RelBlock ifaces.Column
	// timestamp data for the first and last blocks in the conflation, columns of size 1
	First, Last ifaces.Column
	// FirstArith and LastArith are identical to First and Last but are used in the constraints
	// involving arithmetization columns. They are constrained to be constant and via the
	// projection query between the fetcher and the
	FirstArith, LastArith ifaces.Column
	// Data contains all the timestamps in the conflation, ordered by block
	Data ifaces.Column
	// filter on the TimestampFetcher.Data column
	FilterFetched ifaces.Column
	// filter on the Arithmetization's columns
	FilterArith ifaces.Column
	// filter that selects only timestamp rows from the arithmetization
	SelectorTimestamp ifaces.Column
	// prover action to compute SelectorTimestamp
	ComputeSelectorTimestamp wizard.ProverAction
	// since there are two timestamp columns, we need to use Ct in order to select only one
	SelectorCt ifaces.Column
	// prover action to compute SelectorCt
	ComputeSelectorCt wizard.ProverAction
	// the absolute ID of the first block number
	FirstBlockID ifaces.Column
	// the absolute ID of the last block number
	LastBlockID ifaces.Column
	// the absolute ID of the first block number
	FirstBlockIDArith ifaces.Column
	// the absolute ID of the last block number
	LastBlockIDArith ifaces.Column
}

// NewTimestampFetcher returns a new TimestampFetcher with initialized columns that are not constrained.
func NewTimestampFetcher(comp *wizard.CompiledIOP, name string, bdc *arith.BlockDataCols) *TimestampFetcher {

	size := bdc.Ct.Size()

	res := &TimestampFetcher{
		RelBlock:          util.CreateCol(name, "REL_BLOCK", size, comp),
		Data:              util.CreateCol(name, "DATA", size, comp),
		FilterFetched:     util.CreateCol(name, "FILTER_FETCHED", size, comp),
		FilterArith:       util.CreateCol(name, "FILTER_ARITHMETIZATION", size, comp),
		FirstBlockID:      util.CreateCol(name, "FIRST_BLOCK_ID", size, comp),
		LastBlockID:       util.CreateCol(name, "LAST_BLOCK_ID", size, comp),
		First:             util.CreateCol(name, "FIRST", size, comp),
		Last:              util.CreateCol(name, "LAST", size, comp),
		FirstBlockIDArith: util.CreateCol(name, "FIRST_BLOCK_ID_ARITHMETIZATION", size, comp),
		LastBlockIDArith:  util.CreateCol(name, "LAST_BLOCK_ID_ARITHMETIZATION", size, comp),
		FirstArith:        util.CreateCol(name, "FIRST_ARITHMETIZATION", size, comp),
		LastArith:         util.CreateCol(name, "LAST_ARITHMETIZATION", size, comp),
	}

	return res
}

// ConstrainFirstAndLastBlockID constraing the values of FirstBlockID and LastBlockID
func ConstrainFirstAndLastBlockID(comp *wizard.CompiledIOP, fetcher *TimestampFetcher, name string, bdc *arith.BlockDataCols) {

	commonconstraints.MustBeConstant(comp, fetcher.FirstBlockID)
	commonconstraints.MustBeConstant(comp, fetcher.LastBlockID)
	commonconstraints.MustBeConstant(comp, fetcher.FirstBlockIDArith)
	commonconstraints.MustBeConstant(comp, fetcher.LastBlockIDArith)

	// Constrain the First Block ID
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_%s_%s", name, "FIRST_BLOCK_ID_GLOBAL", fetcher.FirstBlockID.GetColID()),
		sym.Mul(
			fetcher.FilterArith, // select only non-padding, valid rows.
			sym.Sub(
				bdc.FirstBlock,
				fetcher.FirstBlockIDArith,
			),
		),
	)

	// FilterFetched is already constrained in the fetcher, no need to constrain it again
	// two cases: Case 1: FilterFetched is not completely filled with 1s (we have a border between 1s and 0s)
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s_%s", name, "LAST_BLOCK_ID_GLOBAL", fetcher.LastBlockID.GetColID()),
		sym.Mul(
			fetcher.FilterFetched,
			sym.Sub(1,
				column.Shift(fetcher.FilterFetched, 1),
			),
			sym.Sub(
				fetcher.LastBlockID,
				sym.Add(
					fetcher.RelBlock,
					fetcher.FirstBlockID,
					-1,
				),
			),
		),
	)

	// Case 2: FilterFetched is completely filled with 1s, in which case we do not have a border between 1s and 0s
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s_%s", name, "LAST_BLOCK_ID_LOCAL", fetcher.LastBlockID.GetColID()),
		sym.Mul(
			column.Shift(fetcher.FilterFetched, -1),
			sym.Sub(
				column.Shift(fetcher.LastBlockID, -1),
				sym.Add(
					column.Shift(fetcher.RelBlock, -1),
					column.Shift(fetcher.FirstBlockID, -1),
					-1,
				),
			),
		),
	)

}

// DefineTimestampFetcher specifies the constraints of the TimestampFetcher with respect to the BlockDataCols
func DefineTimestampFetcher(comp *wizard.CompiledIOP, fetcher *TimestampFetcher, name string, bdc *arith.BlockDataCols) {
	timestampField := util.GetTimestampField()
	// constrain the fetcher.SelectorTimestamp column, which will be used to compute the filter for the arithmetization's BlockDataCols
	fetcher.SelectorTimestamp, fetcher.ComputeSelectorTimestamp = dedicated.IsZero(
		comp,
		sym.Sub(
			bdc.Inst,
			timestampField, // check that the Inst field indicates a timestamp row
		),
	)
	// constrain the fetcher.SelectorCt column, which will be used to compute the filter for the arithmetization's BlockDataCols
	fetcher.SelectorCt, fetcher.ComputeSelectorCt = dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(bdc.Ct), // pick the spots where Ct=0
	)
	// constrain the entire arithmetization filtering column, using SelectorCt and SelectorTimestamp
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_CONSTRAINT_ARITHMETIZATION_FILTERING_COLUMN", name),
		sym.Sub(
			fetcher.FilterArith, // fetcher.FilterArith must be 1 if and only if SelectorCt and SelectorTimestamp are both 1
			sym.Mul(
				fetcher.SelectorCt,
				fetcher.SelectorTimestamp,
			),
		),
	)

	commonconstraints.MustBeConstant(comp, fetcher.First)
	commonconstraints.MustBeConstant(comp, fetcher.Last)
	commonconstraints.MustBeConstant(comp, fetcher.FirstArith)
	commonconstraints.MustBeConstant(comp, fetcher.LastArith)

	// constrain fetcher.First to contain the value of the first block's timestamp, using all the timestamps in fetcher.Data
	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%s_%s", name, "FIRST_LOCAL"),
		sym.Sub(
			fetcher.First,
			fetcher.Data, // fetcher.Data is constrained in the projection query
		),
	)

	// constrain fetcher.Last to contain the value of the last block's timestamp,
	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%s_%s", name, "LAST_LOCAL"),
		sym.Sub(
			column.Shift(fetcher.LastArith, -1),
			column.Shift(bdc.DataLo, TimestampOffset),
		),
	)

	// require that the filter on fetched data is a binary column
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_FILTER_ON_FETCHED_CONSTRAINT_MUST_BE_BINARY", name),
		sym.Mul(
			fetcher.FilterFetched,
			sym.Sub(fetcher.FilterFetched, 1),
		),
	)

	// require that the filter on fetched timestamps only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_FILTER_ON_FETCHED_CONSTRAINT_NO_0_TO_1", name),
		sym.Sub(
			fetcher.FilterFetched,
			sym.Mul(
				column.Shift(fetcher.FilterFetched, -1),
				fetcher.FilterFetched),
		),
	)

	// the table with the data we fetch from the arithmetization columns BlockDataCols
	fetcherTable := []ifaces.Column{
		fetcher.RelBlock,
		fetcher.Data,
		fetcher.FirstBlockID,
		fetcher.LastBlockID,
		fetcher.First,
		fetcher.Last,
	}
	// the BlockDataCols we extract timestamp data from, and which we will use to check for consistency
	arithTable := []ifaces.Column{
		bdc.RelBlock,
		bdc.DataLo,
		fetcher.FirstBlockIDArith,
		fetcher.LastBlockIDArith,
		fetcher.FirstArith,
		fetcher.LastArith,
	}

	// a projection query to check that the timestamp data is fetched correctly
	comp.InsertProjection(
		ifaces.QueryIDf("%s_TIMESTAMP_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: fetcherTable,
			ColumnB: arithTable,
			FilterA: fetcher.FilterFetched,
			// filter lights up on the arithmetization's BlockDataCols rows that contain timestamp data
			FilterB: fetcher.FilterArith,
		})

	// constrain the First/Last Block ID counters
	ConstrainFirstAndLastBlockID(comp, fetcher, name, bdc)

}

// AssignTimestampFetcher assigns the data in the TimestampFetcher using data fetched from the BlockDataCols
func AssignTimestampFetcher(run *wizard.ProverRuntime, fetcher *TimestampFetcher, bdc *arith.BlockDataCols) {

	var (
		first, last, firstBlockID field.Element
		// get the hardcoded timestamp flag
		timestampField = util.GetTimestampField()

		// inst is the flag that specifies the row type
		inst        = bdc.Inst.GetColAssignment(run)
		ct          = bdc.Ct.GetColAssignment(run)
		start, stop = smartvectors.CoCompactRange(ct, inst)

		// initialize empty fetched data and filter on the fetched data
		size          = ct.Len()
		relBlock      = make([]field.Element, size)
		data          = make([]field.Element, size)
		filterFetched = make([]field.Element, size)
		filterArith   = make([]field.Element, stop-start)

		// counter is used to populate filter.Data and will increment every
		// time we find a new timestamp
		counter = 0
	)

	for i := start; i < stop; i++ {

		var (
			inst = inst.GetPtr(i)
			ct   = ct.GetPtr(i)
		)

		if inst.Equal(&timestampField) && ct.IsZero() {
			// the row type is a timestamp-encoding row
			timestamp := bdc.DataLo.GetColAssignmentAt(run, i)
			// in the arithmetization, relBlock is the relative block number inside the conflation
			fetchedRelBlock := bdc.RelBlock.GetColAssignmentAt(run, i)
			if fetchedRelBlock.IsOne() {
				// the first relative block has code 0x1
				first.Set(&timestamp)
				// set the first absolute block ID
				firstBlockID = bdc.FirstBlock.GetColAssignmentAt(run, i)
			}
			// continuously update the last timestamp value
			last.Set(&timestamp)
			// update counters and timestamp data
			filterFetched[counter].SetOne()
			relBlock[counter].Set(&fetchedRelBlock)
			// update the arithmetization filter
			filterArith[i-start].SetOne()

			data[counter].Set(&timestamp)
			counter++
		}
	}
	// compute the last absolute block ID
	var lastBlockID field.Element
	fieldCounter := field.NewElement(uint64(counter - 1))
	lastBlockID.Add(&firstBlockID, &fieldCounter)

	// assign the fetcher columns
	run.AssignColumn(fetcher.First.GetColID(), smartvectors.NewConstant(first, size))
	run.AssignColumn(fetcher.Last.GetColID(), smartvectors.NewConstant(last, size))
	run.AssignColumn(fetcher.FirstArith.GetColID(), smartvectors.NewConstant(first, size))
	run.AssignColumn(fetcher.LastArith.GetColID(), smartvectors.NewConstant(last, size))
	run.AssignColumn(fetcher.RelBlock.GetColID(), smartvectors.RightZeroPadded(relBlock, size))
	run.AssignColumn(fetcher.Data.GetColID(), smartvectors.RightZeroPadded(data, size))
	run.AssignColumn(fetcher.FilterFetched.GetColID(), smartvectors.RightZeroPadded(filterFetched, size))
	run.AssignColumn(fetcher.FilterArith.GetColID(), smartvectors.FromCompactWithRange(filterArith, start, stop, size))
	run.AssignColumn(fetcher.FirstBlockID.GetColID(), smartvectors.NewConstant(firstBlockID, size))
	run.AssignColumn(fetcher.LastBlockID.GetColID(), smartvectors.NewConstant(lastBlockID, size))
	run.AssignColumn(fetcher.FirstBlockIDArith.GetColID(), smartvectors.NewConstant(firstBlockID, size))
	run.AssignColumn(fetcher.LastBlockIDArith.GetColID(), smartvectors.NewConstant(lastBlockID, size))

	// assign the SelectorTimestamp using the ComputeSelectorTimestamp prover action
	fetcher.ComputeSelectorTimestamp.Run(run)
	// assign the SelectorCt using the ComputeSelectorCt prover action
	fetcher.ComputeSelectorCt.Run(run)
}
