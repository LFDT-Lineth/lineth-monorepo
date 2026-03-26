package fetchers_arithmetization

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

type TxnDataFetcher struct {
	RelBlock            ifaces.Column
	AbsTxNum            ifaces.Column
	From                [common.NbLimbEthAddress]ifaces.Column
	FilterFetched       ifaces.Column
	SelectorFromAddress ifaces.Column
	// prover action to compute SelectorFromAddress
	ComputeSelectorFromAddress wizard.ProverAction
	FilterArith                ifaces.Column
}

// NewTxnDataFetcher returns a new TxnDataFetcher with initialized columns that are not constrained.
func NewTxnDataFetcher(comp *wizard.CompiledIOP, name string, td *arith.TxnData) TxnDataFetcher {
	size := td.Ct.Size()
	res := TxnDataFetcher{
		RelBlock:      util.CreateCol(name, "REL_BLOCK", size, comp),
		AbsTxNum:      util.CreateCol(name, "ABS_TX_NUM", size, comp),
		FilterFetched: util.CreateCol(name, "FILTER_FETCHED", size, comp),
		FilterArith:   util.CreateCol(name, "FILTER_ARITH", size, comp),
	}

	for i := range td.From {
		res.From[i] = util.CreateCol(name, fmt.Sprintf("FROM_%d", i), size, comp)
	}

	return res
}

// DefineTxnDataFetcher specifies the constraints of the TxnDataFetcher with respect to the arithmetization's TxnData
func DefineTxnDataFetcher(comp *wizard.CompiledIOP, fetcher *TxnDataFetcher, name string, td *arith.TxnData) {
	fetcher.SelectorFromAddress, fetcher.ComputeSelectorFromAddress = dedicated.IsZero(
		comp,
		sym.Sub(
			td.Ct,
			0, // check that the Ct field is 1 (we use 1 rather than 0, as on prepend columns, ct is 0).
			// Moreover, all transaction segments have a row with Ct = 1
		),
	).GetColumnAndProverAction()

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_FILTER_ON_FETCHED_CONSTRAINT_MUST_BE_BINARY", name),
		sym.Mul(
			fetcher.FilterFetched,
			sym.Sub(fetcher.FilterFetched, 1),
		),
	)

	// require that the filter on fetched data only contains 1s followed by 0s
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

	// constrain the filter on the arithmetization
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_FILTER_ON_ARITH_CONSTRAINT", name),
		sym.Sub(
			fetcher.FilterArith,
			sym.Mul(
				fetcher.SelectorFromAddress,
				td.USER,
				td.Selector,
			),
		),
	)

	// a projection query to check that the sender addresses are fetched correctly
	comp.InsertProjection(
		ifaces.QueryIDf("%s_TXN_DATA_FETCHER_PROJECTION", name),
		query.ProjectionInput{
			// the table with the data we fetch from the arithmetization's TxnData columns
			ColumnA: fetcher.From[:],
			// the TxnData we extract sender addresses from, and which we will use to check for consistency
			ColumnB: td.From[:],
			FilterA: fetcher.FilterFetched,
			// filter lights up on the arithmetization's TxnData rows that contain sender address data
			FilterB: fetcher.FilterArith})
}

// AssignTxnDataFetcher assigns the data in the TxnDataFetcher using data fetched from the TxnData
func AssignTxnDataFetcher(run *wizard.ProverRuntime, fetcher TxnDataFetcher, td *arith.TxnData) {

	var (
		// Those are the assignments from the arithmetization
		arithFrom       [common.NbLimbEthAddress]ifaces.ColAssignment
		ct              = td.Ct.GetColAssignment(run)
		fetchedAbsTxNum = expr_handle.GetExprHandleAssignment(run, td.AbsTxNum)
		fetchedRelBlock = td.RelBlock.GetColAssignment(run)
		arithUser       = td.USER.GetColAssignment(run)
		tdSelector      = td.Selector.GetColAssignment(run)
		start, stop     = smartvectors.CoCompactRange(ct)
		size            = td.Ct.Size()
		density         = stop - start

		// Those are the ongoing assignment slices
		relBlock      = make([]field.Element, density)
		absTxNum      = make([]field.Element, density)
		from          [common.NbLimbEthAddress][]field.Element
		filterFetched = make([]field.Element, density)
		filterArith   = make([]field.Element, size)
		counter       = 0
	)

	for i := range arithFrom {
		arithFrom[i] = td.From[i].GetColAssignment(run)
		from[i] = make([]field.Element, density)
	}

	for i := start; i < stop; i++ {

		var (
			ct              = ct.GetPtr(i)
			fetchedAbsTxNum = fetchedAbsTxNum.GetPtr(i)
			fetchedRelBlock = fetchedRelBlock.GetPtr(i)
			arithUser       = arithUser.GetPtr(i)
			tdSelector      = tdSelector.GetPtr(i)
			arithFromVal    [common.NbLimbEthAddress]*field.Element
		)

		for j := range arithFrom {
			arithFromVal[j] = arithFrom[j].GetPtr(i)
		}

		if ct.IsZero() && !fetchedAbsTxNum.IsZero() && arithUser.IsOne() && tdSelector.IsOne() { // absTxNum starts from 1, ct starts from 0 but always touches 1
			absTxNum[counter].Set(fetchedAbsTxNum)
			relBlock[counter].Set(fetchedRelBlock)

			for j := range from {
				from[j][counter].Set(arithFromVal[j])
			}

			// update counters
			filterFetched[counter].SetOne()
			counter++
			// and compute the arith filter
			filterArith[i].SetOne()
		}
	}

	// assign the fetcher columns
	run.AssignColumn(fetcher.RelBlock.GetColID(), smartvectors.RightZeroPadded(relBlock[:counter], size))
	run.AssignColumn(fetcher.AbsTxNum.GetColID(), smartvectors.RightZeroPadded(absTxNum[:counter], size))
	run.AssignColumn(fetcher.FilterFetched.GetColID(), smartvectors.RightZeroPadded(filterFetched[:counter], size))
	run.AssignColumn(fetcher.FilterArith.GetColID(), smartvectors.NewRegular(filterArith))

	for i := range from {
		run.AssignColumn(fetcher.From[i].GetColID(), smartvectors.RightZeroPadded(from[i][:counter], size))
	}

	// assign the SelectorFromAddress using the ComputeSelectorFromAddress prover action
	fetcher.ComputeSelectorFromAddress.Run(run)
}
