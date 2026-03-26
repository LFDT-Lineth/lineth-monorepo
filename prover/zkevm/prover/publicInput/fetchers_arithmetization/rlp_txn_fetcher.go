package fetchers_arithmetization

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

type RlpTxnFetcher struct {
	// Absolute number of the transaction (starts from 1 and acts as an Active Filter), and the maximum number of
	// transactions
	AbsTxNum, AbsTxNumMax ifaces.Column
	Limbs                 [common.NbLimbU128]ifaces.Column
	NBytes                ifaces.Column
	// isActive filter pattern that lights up on the area containing relevant data
	FilterFetched ifaces.Column
	// lights up on active rows i for which AbsTxNum[i]!=AbsTxNum[i+1]
	EndOfRlpSegment ifaces.Column
	// prover action selectors
	// used to compute EndOfRlpSegment, lights up on active rows i for which AbsTxNum[i]!=AbsTxNum[i+1]
	SelectorDiffAbsTxId        ifaces.Column
	ComputeSelectorDiffAbsTxId wizard.ProverAction
}

func NewRlpTxnFetcher(comp *wizard.CompiledIOP, name string, rt *arith.RlpTxn) RlpTxnFetcher {
	size := rt.Limbs[0].Size()
	res := RlpTxnFetcher{
		AbsTxNum:        util.CreateCol(name, "ABS_TX_NUM", size, comp),
		AbsTxNumMax:     util.CreateCol(name, "ABS_TX_NUM_MAX", size, comp),
		NBytes:          util.CreateCol(name, "NBYTES", size, comp),
		FilterFetched:   util.CreateCol(name, "FILTER_FETCHED", size, comp),
		EndOfRlpSegment: util.CreateCol(name, "END_OF_RLP_SEGMENT", size, comp),
	}

	for i := range res.Limbs {
		res.Limbs[i] = util.CreateCol(name, fmt.Sprintf("LIMB_%d", i), size, comp)
	}

	return res
}

func DefineRlpTxnFetcher(comp *wizard.CompiledIOP, fetcher *RlpTxnFetcher, name string, rlpTxnArith *arith.RlpTxn) {
	fetcher.SelectorDiffAbsTxId, fetcher.ComputeSelectorDiffAbsTxId = dedicated.IsZero(
		comp,
		sym.Sub(
			fetcher.AbsTxNum,
			column.Shift(fetcher.AbsTxNum, 1),
		),
	).GetColumnAndProverAction()

	// require that the filter on fetched data is a binary column
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

	// Constrain EndOfRlpSegment
	util.MustBeBinary(comp, fetcher.EndOfRlpSegment)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_GLOBAL_CONSTRAINT_ON_END_RLP_SEGMENT", name),
		sym.Mul(
			fetcher.FilterFetched, // constrain only on the active part of the module
			sym.Sub(
				fetcher.EndOfRlpSegment, // constrain EndOfRlpSegment
				sym.Sub(
					1,
					fetcher.SelectorDiffAbsTxId,
				),
			),
		),
	)

	// the table with the data we fetch from the arithmetization columns RlpTxn
	fetcherTable := append(fetcher.Limbs[:],
		fetcher.AbsTxNum,
		fetcher.AbsTxNumMax,
		fetcher.NBytes,
	)
	// the RlpTxn we extract timestamp data from, and which we will use to check for consistency
	arithTable := append(rlpTxnArith.Limbs[:],
		rlpTxnArith.AbsTxNum,
		rlpTxnArith.AbsTxNumMax,
		rlpTxnArith.NBytes,
	)

	// a projection query to check that the timestamp data is fetched correctly
	comp.InsertProjection(
		ifaces.QueryIDf("%s_RLP_TXN_PROJECTION", name),
		query.ProjectionInput{ColumnA: fetcherTable,
			ColumnB: arithTable,
			FilterA: fetcher.FilterFetched,
			// filter lights up on the arithmetization's RlpTxn rows that contain rlp transaction data
			FilterB: rlpTxnArith.ToHashByProver})
}

func AssignRlpTxnFetcher(run *wizard.ProverRuntime, fetcher *RlpTxnFetcher, rlpTxnArith *arith.RlpTxn) {

	size := rlpTxnArith.Limbs[0].Size()

	absTxNum := make([]field.Element, size)
	absTxNumMax := make([]field.Element, size)
	limbs := make([][]field.Element, len(rlpTxnArith.Limbs))
	nBytes := make([]field.Element, size)
	filterFetched := make([]field.Element, size)
	endOfRlpSegment := make([]field.Element, size)

	for i := range limbs {
		limbs[i] = make([]field.Element, size)
	}

	// counter is used to populate filter.Data and will increment every time we find a new timestamp
	counter := 0

	for i := 0; i < size; i++ {
		toHashByProver := rlpTxnArith.ToHashByProver.GetColAssignmentAt(run, i)
		// process the RLP limb, by inspecting AbsTxNum, AbsTxNumMax, Limb, NBytes
		// and populating a row of the fetcher with these values.
		if toHashByProver.IsOne() {
			arithAbsTxNum := rlpTxnArith.AbsTxNum.GetColAssignmentAt(run, i)
			arithAbsTxNumMax := rlpTxnArith.AbsTxNumMax.GetColAssignmentAt(run, i)
			arithNBytes := rlpTxnArith.NBytes.GetColAssignmentAt(run, i)

			absTxNum[counter].Set(&arithAbsTxNum)
			absTxNumMax[counter].Set(&arithAbsTxNumMax)
			nBytes[counter].Set(&arithNBytes)
			filterFetched[counter].SetOne()

			for j := range rlpTxnArith.Limbs {
				arithLimb := rlpTxnArith.Limbs[j].GetColAssignmentAt(run, i)
				limbs[j][counter].Set(&arithLimb)
			}

			counter++
		}

	}

	for i := 0; i < size-1; i++ {
		if filterFetched[i].IsOne() {
			// only set end of segments in the active area
			if !absTxNum[i].Equal(&absTxNum[i+1]) {
				endOfRlpSegment[i].SetOne()
			}
		}
	}

	// assign the fetcher columns
	run.AssignColumn(fetcher.AbsTxNum.GetColID(), smartvectors.RightZeroPadded(absTxNum[:counter], size))
	run.AssignColumn(fetcher.AbsTxNumMax.GetColID(), smartvectors.RightZeroPadded(absTxNumMax[:counter], size))
	run.AssignColumn(fetcher.NBytes.GetColID(), smartvectors.RightZeroPadded(nBytes[:counter], size))
	run.AssignColumn(fetcher.FilterFetched.GetColID(), smartvectors.RightZeroPadded(filterFetched[:counter], size))
	run.AssignColumn(fetcher.EndOfRlpSegment.GetColID(), smartvectors.NewRegular(endOfRlpSegment), wizard.DisableAssignmentSizeReduction)

	for i := range rlpTxnArith.Limbs {
		run.AssignColumn(fetcher.Limbs[i].GetColID(), smartvectors.RightZeroPadded(limbs[i][:counter], size))
	}

	fetcher.ComputeSelectorDiffAbsTxId.Run(run)
}
