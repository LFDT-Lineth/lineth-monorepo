package logs

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// RollingSelector is used to fetch the last rolling hash and its associated message number
type RollingSelector struct {
	// Exists contains a 1 if there exists at least one rolling hash log
	ExistsHash, ExistsMsg ifaces.Column
	// the Hi/Lo part of the first Rolling Hash found in the logs
	FirstHi, FirstLo ifaces.Column
	// the Hi/Lo part of the last Rolling Hash found in the logs
	LastHi, LastLo ifaces.Column
	// the first/last message number of the last Rolling hash log
	FirstMessageNo, LastMessageNo ifaces.Column
}

// NewRollingSelector returns a new RollingSelector with initialized columns that are not constrained.
func NewRollingSelector(comp *wizard.CompiledIOP, name string, sizeHash, sizeMsg int) *RollingSelector {
	return &RollingSelector{
		ExistsHash:     util.CreateCol(name, "EXISTS_HASH", sizeHash, comp),
		ExistsMsg:      util.CreateCol(name, "EXISTS_MSG", sizeMsg, comp),
		FirstHi:        util.CreateCol(name, "FIRST_HI", sizeHash, comp),
		FirstLo:        util.CreateCol(name, "FIRST_LO", sizeHash, comp),
		LastHi:         util.CreateCol(name, "LAST_HI", sizeHash, comp),
		LastLo:         util.CreateCol(name, "LAST_LO", sizeHash, comp),
		FirstMessageNo: util.CreateCol(name, "FIRST_MESSAGE_NO", sizeMsg, comp),
		LastMessageNo:  util.CreateCol(name, "LAST_MESSAGE_NO", sizeMsg, comp),
	}
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
		sel.FirstHi,
		sel.FirstLo,
		sel.LastHi,
		sel.LastLo,
		sel.FirstMessageNo,
		sel.LastMessageNo,
	}

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
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "FIRST_HI"),
		sym.Sub(
			fetchedHash.Hi,
			sel.FirstHi,
		),
	)
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "FIRST_LO"),
		sym.Sub(
			fetchedHash.Lo,
			sel.FirstLo,
		),
	)
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "FIRST_MSG_NO"),
		sym.Sub(
			fetchedMsg.Lo,
			sel.FirstMessageNo,
		),
	)

	// define the consistency constraints
	util.CheckLastELemConsistency(comp, isActiveHash, fetchedHash.Hi, sel.LastHi, name)
	util.CheckLastELemConsistency(comp, isActiveHash, fetchedHash.Lo, sel.LastLo, name)
	util.CheckLastELemConsistency(comp, isActiveMsg, fetchedMsg.Lo, sel.LastMessageNo, name)
}

// AssignRollingSelector assigns the data in the RollingSelector using the ExtractedData fetched from the arithmetization
func AssignRollingSelector(run *wizard.ProverRuntime, selector *RollingSelector, fetchedHash, fetchedMsg ExtractedData) {
	sizeHash := fetchedHash.Hi.Size()
	sizeMsg := fetchedMsg.Hi.Size()

	exists := run.GetColumnAt(fetchedHash.FilterFetched.GetColID(), 0)
	var lastHi, lastLo, lastMsg field.Element
	for i := 0; i < sizeHash; i++ {
		isActive := run.GetColumnAt(fetchedHash.FilterFetched.GetColID(), i)
		if isActive.IsOne() {
			lastHi = run.GetColumnAt(fetchedHash.Hi.GetColID(), i)
			lastLo = run.GetColumnAt(fetchedHash.Lo.GetColID(), i)
			lastMsg = run.GetColumnAt(fetchedMsg.Lo.GetColID(), i)
		} else {
			break
		}
	}
	// compute first values
	firstHi := run.GetColumnAt(fetchedHash.Hi.GetColID(), 0)
	firstLo := run.GetColumnAt(fetchedHash.Lo.GetColID(), 0)
	firstMsg := run.GetColumnAt(fetchedMsg.Lo.GetColID(), 0)

	// assign the RollingSelector columns
	run.AssignColumn(selector.ExistsHash.GetColID(), smartvectors.NewConstant(exists, sizeHash))
	run.AssignColumn(selector.ExistsMsg.GetColID(), smartvectors.NewConstant(exists, sizeMsg))
	run.AssignColumn(selector.FirstHi.GetColID(), smartvectors.NewConstant(firstHi, sizeHash))
	run.AssignColumn(selector.FirstLo.GetColID(), smartvectors.NewConstant(firstLo, sizeHash))
	run.AssignColumn(selector.FirstMessageNo.GetColID(), smartvectors.NewConstant(firstMsg, sizeMsg))
	run.AssignColumn(selector.LastHi.GetColID(), smartvectors.NewConstant(lastHi, sizeHash))
	run.AssignColumn(selector.LastLo.GetColID(), smartvectors.NewConstant(lastLo, sizeHash))
	run.AssignColumn(selector.LastMessageNo.GetColID(), smartvectors.NewConstant(lastMsg, sizeMsg))
}
