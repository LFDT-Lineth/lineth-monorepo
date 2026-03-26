package logs

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// RollingSelector is used to fetch the last rolling hash and its associated message number
type RollingSelector struct {
	// Exists contains a 1 if there exists at least one rolling hash log
	ExistsHash, ExistsMsg ifaces.Column
	// the first/last Rolling HashFirst found in the logs
	First, Last [common.NbLimbU256]ifaces.Column
	// the first/last message number of the last Rolling hash log
	FirstMessageNo, LastMessageNo [common.NbLimbU128]ifaces.Column
}

// NewRollingSelector returns a new RollingSelector with initialized columns that are not constrained.
func NewRollingSelector(comp *wizard.CompiledIOP, name string, size int) *RollingSelector {
	res := &RollingSelector{
		ExistsHash: util.CreateCol(name, "EXISTS_HASH", size, comp),
		ExistsMsg:  util.CreateCol(name, "EXISTS_MSG", size, comp),
	}

	for i := range res.First {
		res.First[i] = util.CreateCol(name, fmt.Sprintf("FIRST_%d", i), size, comp)
		res.Last[i] = util.CreateCol(name, fmt.Sprintf("LAST_%d", i), size, comp)
	}

	for i := range res.FirstMessageNo {
		res.FirstMessageNo[i] = util.CreateCol(name, fmt.Sprintf("FIRST_MESSAGE_NO_%d", i), size, comp)
		res.LastMessageNo[i] = util.CreateCol(name, fmt.Sprintf("LAST_MESSAGE_NO_%d", i), size, comp)
	}

	return res
}

// DefineRollingSelector specifies the constraints of the RollingSelector with respect to the ExtractedData fetched from the arithmetization
func DefineRollingSelector(comp *wizard.CompiledIOP, sel *RollingSelector, name string, fetchedHash, fetchedMsg ExtractedData) {
	// fetchedHash.filterFetched is an isActiveHash pattern filter, and is constrained as such in DefineExtractedData
	isActiveHash := fetchedHash.FilterFetched
	isActiveMsg := fetchedMsg.FilterFetched

	// set the RollingSelector columns as public in order to get accessors
	var allCols = []ifaces.Column{
		sel.ExistsHash,
		sel.ExistsMsg,
	}
	allCols = append(allCols, sel.First[:]...)
	allCols = append(allCols, sel.Last[:]...)
	allCols = append(allCols, sel.FirstMessageNo[:]...)
	allCols = append(allCols, sel.LastMessageNo[:]...)

	for _, col := range allCols {
		commonconstraints.MustBeConstant(comp, col)
	}

	// set the exists flag
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "EXISTS"),
		sym.Sub(
			isActiveHash,
			sel.ExistsHash,
		),
	)
	// local openings for the first values
	for i := range sel.First {
		comp.InsertLocal(0, ifaces.QueryIDf("%s_FIRST_%d", name, i),
			sym.Sub(
				fetchedHash.Data[i],
				sel.First[i],
			),
		)
	}
	for i := range sel.FirstMessageNo {
		comp.InsertLocal(0, ifaces.QueryIDf("%s_FIRST_MSG_NO_%d", name, i),
			sym.Sub(
				fetchedMsg.Data[common.NbLimbU128+i],
				sel.FirstMessageNo[i],
			),
		)
	}

	// define the consistency constraints
	for i := range sel.Last {
		util.CheckLastELemConsistency(comp, isActiveHash, fetchedHash.Data[i], sel.Last[i], name)
	}
	for i := range sel.LastMessageNo {
		util.CheckLastELemConsistency(comp, isActiveMsg, fetchedMsg.Data[i], sel.LastMessageNo[i], name)
	}
}

// AssignRollingSelector assigns the data in the RollingSelector using the ExtractedData fetched from the arithmetization
func AssignRollingSelector(run *wizard.ProverRuntime, selector *RollingSelector, fetchedHash, fetchedMsg ExtractedData) {
	sizeHash := fetchedHash.Data[0].Size()
	sizeMsg := fetchedMsg.Data[0].Size()

	exists := run.GetColumnAt(fetchedHash.FilterFetched.GetColID(), 0)
	var last, first [common.NbLimbU256]field.Element
	// contains messageNo which is stores in the Lo part of the message
	var lastMsg, firstMsg [common.NbLimbU128]field.Element
	for i := 0; i < sizeHash; i++ {
		isActive := run.GetColumnAt(fetchedHash.FilterFetched.GetColID(), i)
		if isActive.IsZero() {
			break
		}

		for j := range last {
			last[j] = run.GetColumnAt(fetchedHash.Data[j].GetColID(), i)
		}
		for j := range lastMsg {
			lastMsg[j] = run.GetColumnAt(fetchedMsg.Data[j].GetColID(), i)
		}
	}

	// compute first values
	for j := range first {
		first[j] = run.GetColumnAt(fetchedHash.Data[j].GetColID(), 0)
	}
	for j := range firstMsg {
		firstMsg[j] = run.GetColumnAt(fetchedMsg.Data[common.NbLimbU128+j].GetColID(), 0)
	}

	// assign the RollingSelector columns
	run.AssignColumn(selector.ExistsHash.GetColID(), smartvectors.NewConstant(exists, sizeHash))
	run.AssignColumn(selector.ExistsMsg.GetColID(), smartvectors.NewConstant(exists, sizeMsg))

	for i := range first {
		run.AssignColumn(selector.First[i].GetColID(), smartvectors.NewConstant(first[i], sizeHash))
		run.AssignColumn(selector.Last[i].GetColID(), smartvectors.NewConstant(last[i], sizeHash))
	}

	for i := range firstMsg {
		run.AssignColumn(selector.FirstMessageNo[i].GetColID(), smartvectors.NewConstant(firstMsg[i], sizeMsg))
		run.AssignColumn(selector.LastMessageNo[i].GetColID(), smartvectors.NewConstant(lastMsg[i], sizeMsg))
	}
}
