package logs

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/projection"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// ExtractedData contains the data extracted from the arithmetization logs:
// L2L1 case: already Keccak-hashed messages, which will be hashed again using MiMC
// RollingHash case: either the message number stored in the Lo part
// or the RollingHash stored in both Hi/Lo
type ExtractedData struct {
	Hi, Lo        ifaces.Column
	filterArith   ifaces.Column
	filterFetched ifaces.Column
}

// NewExtractedData initializes a NewExtractedData struct, registering columns that are not yet constrained.
func NewExtractedData(comp *wizard.CompiledIOP, size int, name string) ExtractedData {
	res := ExtractedData{
		// register Hi, Lo, the columns in which we embed the message we want to fetch from LogColumns
		Hi: util.CreateCol(name, "EXTRACTED_HI", size, comp),
		Lo: util.CreateCol(name, "EXTRACTED_LO", size, comp),
		// register the filter on the arithmetization log columns
		filterArith: util.CreateCol(name, "FILTER", size, comp),
		// a filter on the columns with fetched data
		filterFetched: util.CreateCol(name, "FILTER_ON_FETCHED", size, comp),
	}
	return res
}

// DefineExtractedData uses LogColumns and helper Selectors to define columns that contain the ExtractedData of L2L1/Rolling Hash logs
// along with filters to select only L2L1/Rolling Hash logs.
// DefineExtractedData then uses a projection query to check that the data was fetched appropriately
func DefineExtractedData(comp *wizard.CompiledIOP, logCols LogColumns, sel Selectors, fetched ExtractedData, logType int) {
	// the table with the data we fetch from the arithmetization columns LogColumns
	fetchedTable := []ifaces.Column{
		fetched.Hi,
		fetched.Lo,
	}
	// the LogColumns we extract data from, and which we will use to check for consistency
	logsTable := []ifaces.Column{
		logCols.DataHi,
		logCols.DataLo,
	}

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_CONSTRAINT_CHECK_LOG_OF_TYPE", GetName(logType)),
		sym.Sub(
			fetched.filterArith,
			sym.Mul(
				IsLogType(logCols, logType),      // IsLogType returns either isLog3 or isLog4 depending on the case
				GetSelectorCounter(sel, logType), // GetSelectorCounter returns 1 when one of the following holds:
				// logCols.Ct = 5 (an L2L1 message) or logCols.Ct = 3 (RollingMsgNo) or logCols.Ct = 4 (RollingHashNo)
				// now we check that the first topics are computed properly in the log, by inspecting a previous row at a certain offset
				// the offset is -3 (an L2L1 message) or -1 (RollingMsgNo) or -2 (RollingHashNo)
				column.Shift(GetSelectorFirstTopicHi(sel, logType), GetOffset(logType, FirstTopic)),
				column.Shift(GetSelectorFirstTopicLo(sel, logType), GetOffset(logType, FirstTopic)),
				// now we check that the address of this log is indeed the L2BridgeAddress
				// the offset is -4 (an L2L1 message) or -2 (RollingMsgNo) or -3 (RollingHashNo)
				column.Shift(sel.SelectorL2BridgeAddressHi, GetOffset(logType, L2BridgeAddress)),
				column.Shift(sel.SelectorL2BridgeAddressLo, GetOffset(logType, L2BridgeAddress)),
			),
		),
	)
	// require that the filter on fetched data is a binary column
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_ON_FETCHED_CONSTRAINT_MUST_BE_BINARY", GetName(logType)),
		sym.Mul(
			fetched.filterFetched,
			sym.Sub(fetched.filterFetched, 1),
		),
	)

	// require that the filter on fetched data only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_ON_FETCHED_CONSTRAINT_NO_0_TO_1", GetName(logType)),
		sym.Sub(
			fetched.filterFetched,
			sym.Mul(
				column.Shift(fetched.filterFetched, -1),
				fetched.filterFetched),
		),
	)
	// a projection query to check that the messages are fetched correctly
	projection.InsertProjection(comp, ifaces.QueryIDf("%s_LOGS_PROJECTION", GetName(logType)), fetchedTable, logsTable, fetched.filterFetched, fetched.filterArith)
}

// CheckBridgeAddress checks if a row does indeed contain the data corresponding to a the bridge address
func CheckBridgeAddress(run *wizard.ProverRuntime, lCols LogColumns, sel Selectors, pos int) bool {
	outHi := lCols.DataHi.GetColAssignmentAt(run, pos)
	outLo := lCols.DataLo.GetColAssignmentAt(run, pos)
	bridgeAddrHi := sel.L2BridgeAddressColHI.GetColAssignmentAt(run, 0)
	bridgeAddrLo := sel.L2BridgeAddressColLo.GetColAssignmentAt(run, 0)
	if outHi.Equal(&bridgeAddrHi) && outLo.Equal(&bridgeAddrLo) {
		return true
	}
	return false
}

// CheckFirstTopic checks if a row does indeed contain the data corresponding to a first topic in a L2l1/Rolling hash log
func CheckFirstTopic(run *wizard.ProverRuntime, lCols LogColumns, pos int, logType int) bool {
	var firstTopicHi, firstTopicLo field.Element
	firstTopicBytes := GetFirstTopic(logType) // fixed expected value for the topic on the first topic row
	firstTopicHi.SetBytes(firstTopicBytes[:16])
	firstTopicLo.SetBytes(firstTopicBytes[16:])
	outHi := lCols.DataHi.GetColAssignmentAt(run, pos)
	outLo := lCols.DataLo.GetColAssignmentAt(run, pos)
	if firstTopicHi.Equal(&outHi) && firstTopicLo.Equal(&outLo) {
		return true
	}
	return false
}

// IsPositionTargetMessage checks if a row does indeed contain the relevant messages corresponding to L2l1/RollingHash logs
func IsPositionTargetMessage(run *wizard.ProverRuntime, lCols LogColumns, sel Selectors, pos, logType int) bool {
	// isLogX is expected to be isLog4 for L2L1 logs and isLog3 for RollingHashes
	isLogX := IsLogType(lCols, logType).GetColAssignmentAt(run, pos)
	// get the current counter
	ct := lCols.Ct.GetColAssignmentAt(run, pos)
	var ctMinusTarget field.Element
	targetCt := GetPositionCounter(logType) // the counter at which we will find the target value
	fieldTarget := field.NewElement(uint64(targetCt))
	ctMinusTarget.Sub(&ct, &fieldTarget) // if ctMinusTarget is zero, then we are at the appropriate counter
	if isLogX.IsOne() && ctMinusTarget.IsZero() {
		// now check if the first topic is assigned correctly at pos-3/pos-1/pos-2 depending on the type
		firstOffset := GetOffset(logType, FirstTopic)
		if CheckFirstTopic(run, lCols, pos+firstOffset, logType) {
			// now check if we are dealing indeed with the proper bridge address at pos-4/pos-2/pos-3 depending on the type
			addrOffset := GetOffset(logType, L2BridgeAddress)
			if CheckBridgeAddress(run, lCols, sel, pos+addrOffset) {
				return true
			}
		}
	}
	return false
}

// AssignExtractedData fetches data from the LogColumns and uses it to populate the ExtractedData columns
func AssignExtractedData(run *wizard.ProverRuntime, lCols LogColumns, sel Selectors, fetched ExtractedData, logType int) {
	filterLogs := make([]field.Element, lCols.Ct.Size())
	Hi := make([]field.Element, lCols.Ct.Size())
	Lo := make([]field.Element, lCols.Ct.Size())
	filterFetched := make([]field.Element, lCols.Ct.Size())
	counter := 0 // counter used to incrementally populate Hi, Lo of the ExtractedData and their associated filterFetched
	for i := 0; i < lCols.Ct.Size(); i++ {
		// the following conditional checks if row i contains a message that should be picked
		if IsPositionTargetMessage(run, lCols, sel, i, logType) {
			hi := lCols.DataHi.GetColAssignmentAt(run, i)
			lo := lCols.DataLo.GetColAssignmentAt(run, i)
			// pick the messages and add them to the msgHi/Lo ExtractedData columns
			Hi[counter].Set(&hi)
			Lo[counter].Set(&lo)
			// now set the filter on ExtractedData columns to be 1
			filterFetched[counter].SetOne()
			// set the filter on the LogColumns to be 1, at position i
			filterLogs[i].SetOne()
			counter++
		}
	}
	// assign our fetched data
	run.AssignColumn(fetched.Hi.GetColID(), smartvectors.NewRegular(Hi))
	run.AssignColumn(fetched.Lo.GetColID(), smartvectors.NewRegular(Lo))
	// assign filters for original log columns and fetched ExtractedData
	run.AssignColumn(fetched.filterArith.GetColID(), smartvectors.NewRegular(filterLogs))      // filter on LogColumns
	run.AssignColumn(fetched.filterFetched.GetColID(), smartvectors.NewRegular(filterFetched)) // filter on fetched data
}
