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
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

type TxnDataFetcher struct {
	RelBlock            ifaces.Column
	AbsTxNum            ifaces.Column
	FromHi              ifaces.Column
	FromLo              ifaces.Column
	FilterFetched       ifaces.Column
	SelectorFromAddress ifaces.Column
	// prover action to compute SelectorFromAddress
	ComputeSelectorFromAddress wizard.ProverAction
}

// NewTxnDataFetcher returns a new TxnDataFetcher with initialized columns that are not constrained.
func NewTxnDataFetcher(comp *wizard.CompiledIOP, name string, td *arith.TxnData) TxnDataFetcher {
	size := td.Ct.Size()
	res := TxnDataFetcher{
		RelBlock:      util.CreateCol(name, "REL_BLOCK", size, comp),
		AbsTxNum:      util.CreateCol(name, "ABS_TX_NUM", size, comp),
		FromHi:        util.CreateCol(name, "FROM_HI", size, comp),
		FromLo:        util.CreateCol(name, "FROM_LO", size, comp),
		FilterFetched: util.CreateCol(name, "FILTER_FETCHED", size, comp),
	}
	return res
}

// DefineTxnDataFetcher specifies the constraints of the TxnDataFetcher with respect to the arithmetization's TxnData
func DefineTxnDataFetcher(comp *wizard.CompiledIOP, fetcher *TxnDataFetcher, name string, td *arith.TxnData) {
	fetcher.SelectorFromAddress, fetcher.ComputeSelectorFromAddress = dedicated.IsZero(
		comp,
		sym.Sub(
			td.Ct,
			1, // check that the Ct field is 1 (we use 1 rather than 0, as on prepend columns, ct is 0).
			// Moreover, all transaction segments have a row with Ct = 1
		),
	)

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

	// the table with the data we fetch from the arithmetization's TxnData columns
	fetcherTable := []ifaces.Column{
		fetcher.FromHi,
		fetcher.FromLo,
	}
	// the TxnData we extract sender addresses from, and which we will use to check for consistency
	arithTable := []ifaces.Column{
		td.FromHi,
		td.FromLo,
	}

	// a projection query to check that the sender addresses are fetched correctly
	comp.InsertProjection(
		ifaces.QueryIDf("%s_TXN_DATA_FETCHER_PROJECTION", name),
		query.ProjectionInput{ColumnA: fetcherTable,
			ColumnB: arithTable,
			FilterA: fetcher.FilterFetched,
			// filter lights up on the arithmetization's TxnData rows that contain sender address data
			FilterB: fetcher.SelectorFromAddress})
}

// AssignTxnDataFetcher assigns the data in the TxnDataFetcher using data fetched from the TxnData
func AssignTxnDataFetcher(run *wizard.ProverRuntime, fetcher TxnDataFetcher, td *arith.TxnData) {

	var (
		// Those are the assignments from the arithmetization
		ct              = td.Ct.GetColAssignment(run)
		fetchedAbsTxNum = td.AbsTxNum.GetColAssignment(run)
		fetchedRelBlock = td.RelBlock.GetColAssignment(run)
		arithFromHi     = td.FromHi.GetColAssignment(run)
		arithFromLo     = td.FromLo.GetColAssignment(run)
		start, stop     = smartvectors.CoCompactRange(ct)
		size            = td.Ct.Size()
		density         = stop - start

		// Those are the ongoing assignment slices
		relBlock      = make([]field.Element, density)
		absTxNum      = make([]field.Element, density)
		fromHi        = make([]field.Element, density)
		fromLo        = make([]field.Element, density)
		filterFetched = make([]field.Element, density)
		counter       = 0
	)

	for i := start; i < stop; i++ {

		var (
			ct              = ct.GetPtr(i)
			fetchedAbsTxNum = fetchedAbsTxNum.GetPtr(i)
			fetchedRelBlock = fetchedRelBlock.GetPtr(i)
			arithFromHi     = arithFromHi.GetPtr(i)
			arithFromLo     = arithFromLo.GetPtr(i)
		)

		if ct.IsOne() && !fetchedAbsTxNum.IsZero() { // absTxNum starts from 1, ct starts from 0 but always touches 1
			absTxNum[counter].Set(fetchedAbsTxNum)
			relBlock[counter].Set(fetchedRelBlock)
			fromHi[counter].Set(arithFromHi)
			fromLo[counter].Set(arithFromLo)
			// update counters
			filterFetched[counter].SetOne()
			counter++
		}
	}

	// assign the fetcher columns
	run.AssignColumn(fetcher.RelBlock.GetColID(), smartvectors.RightZeroPadded(relBlock[:counter], size))
	run.AssignColumn(fetcher.AbsTxNum.GetColID(), smartvectors.RightZeroPadded(absTxNum[:counter], size))
	run.AssignColumn(fetcher.FromHi.GetColID(), smartvectors.RightZeroPadded(fromHi[:counter], size))
	run.AssignColumn(fetcher.FromLo.GetColID(), smartvectors.RightZeroPadded(fromLo[:counter], size))
	run.AssignColumn(fetcher.FilterFetched.GetColID(), smartvectors.RightZeroPadded(filterFetched[:counter], size))
	// assign the SelectorFromAddress using the ComputeSelectorFromAddress prover action
	fetcher.ComputeSelectorFromAddress.Run(run)
}
