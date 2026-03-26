package logs

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// ExtractedData contains the data extracted from the arithmetization logs:
// L2L1 case: already Keccak-hashed messages, which will be hashed again using MiMC
// RollingHash case: either the message number stored in the Lo part
// or the RollingHash stored in both Hi/Lo
type ExtractedData struct {
	Data          [common.NbLimbU256]ifaces.Column
	FilterArith   ifaces.Column
	FilterFetched ifaces.Column
}

// NewExtractedData initializes a NewExtractedData struct, registering columns that are not yet constrained.
func NewExtractedData(comp *wizard.CompiledIOP, size int, name string) ExtractedData {
	res := ExtractedData{
		// register the filter on the arithmetization log columns
		FilterArith: util.CreateCol(name, "FILTER", size, comp),
		// a filter on the columns with fetched data
		FilterFetched: util.CreateCol(name, "FILTER_ON_FETCHED", size, comp),
	}

	// Tagging of "fetched" column hints the compiler on how this column should
	// be padded. Without it, it will assume that there are no padding
	// informations. The "arith" columns are already "grouped" with the columns
	// of the logdata module and the compiler will already infer that they are
	// right padded.
	pragmas.MarkRightPadded(res.FilterFetched)

	// register Data, the columns in which we embed the message we want to fetch from LogColumns
	for i := range res.Data {
		res.Data[i] = util.CreateCol(name, fmt.Sprintf("EXTRACTED_%d", i), size, comp)
	}

	return res
}

// DefineExtractedData uses LogColumns and helper Selectors to define columns that contain the ExtractedData of L2L1/Rolling HashFirst logs
// along with filters to select only L2L1/Rolling HashFirst logs.
// DefineExtractedData then uses a projection query to check that the data was fetched appropriately
func DefineExtractedData(comp *wizard.CompiledIOP, logCols LogColumns, sel Selectors, fetched ExtractedData, logType int) {
	selectors := sym.Mul(
		// IsLogType returns either isLog3 or isLog4 depending on the case
		IsLogType(logCols, logType),
		// GetSelectorCounter returns 1 when one of the following holds:
		// logCols.Ct = 5 (an L2L1 message) or logCols.Ct = 3 (RollingMsgNo) or logCols.Ct = 4 (RollingHashNo)
		GetSelectorCounter(sel, logType),
	)

	// now we check that the first topics are computed properly in the log, by inspecting a previous row at a certain offset
	// the offset is -3 (an L2L1 message) or -1 (RollingMsgNo) or -2 (RollingHashNo)
	selectorsFirstTopic := GetSelectorFirstTopic(sel, logType)
	for i := range selectorsFirstTopic {
		selectors = sym.Mul(selectors, column.Shift(selectorsFirstTopic[i], GetOffset(logType, FirstTopic)))
	}

	// now we check that the address of this log is indeed the L2BridgeAddress
	// the offset is -4 (an L2L1 message) or -2 (RollingMsgNo) or -3 (RollingHashNo)
	for i := range sel.SelectorL2BridgeAddress {
		selectors = sym.Mul(selectors, column.Shift(sel.SelectorL2BridgeAddress[i], GetOffset(logType, L2BridgeAddress)))
	}

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_CONSTRAINT_CHECK_LOG_OF_TYPE", GetName(logType)),
		sym.Sub(fetched.FilterArith, selectors),
	)

	// require that the filter on fetched data is a binary column
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_ON_FETCHED_CONSTRAINT_MUST_BE_BINARY", GetName(logType)),
		sym.Mul(
			fetched.FilterFetched,
			sym.Sub(fetched.FilterFetched, 1),
		),
	)

	// require that the filter on fetched data only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_LOGS_FILTER_ON_FETCHED_CONSTRAINT_NO_0_TO_1", GetName(logType)),
		sym.Sub(
			fetched.FilterFetched,
			sym.Mul(
				column.Shift(fetched.FilterFetched, -1),
				fetched.FilterFetched),
		),
	)
	// a projection query to check that the messages are fetched correctly
	comp.InsertProjection(
		ifaces.QueryIDf("%s_LOGS_PROJECTION", GetName(logType)),
		query.ProjectionInput{
			// the table with the data we fetch from the arithmetization columns LogColumns
			ColumnA: fetched.Data[:],
			// the LogColumns we extract data from, and which we will use to check for consistency
			ColumnB: logCols.Data[:],
			FilterA: fetched.FilterFetched,
			FilterB: fetched.FilterArith,
		},
	)
}

// CheckBridgeAddress checks if a row does indeed contain the data corresponding to a the bridge address
func CheckBridgeAddress(run *wizard.ProverRuntime, lCols LogColumns, sel Selectors, pos int) bool {
	offset := common.NbLimbU256 - common.NbLimbEthAddress

	for i := range offset {
		out := lCols.Data[i].GetColAssignmentAt(run, pos)
		if !out.IsZero() {
			return false
		}
	}

	for i := range sel.L2BridgeAddressCol {
		out := lCols.Data[i+offset].GetColAssignmentAt(run, pos)
		bridgeAddr := sel.L2BridgeAddressCol[i].GetColAssignmentAt(run, 0)

		if !out.Equal(&bridgeAddr) {
			return false
		}
	}

	return true
}

// CheckFirstTopic checks if a row does indeed contain the data corresponding to a first topic in a L2l1/Rolling hash log
func CheckFirstTopic(run *wizard.ProverRuntime, lCols LogColumns, pos int, logType int) bool {
	var firstTopicLimb field.Element
	firstTopicBytes := GetFirstTopic(logType) // fixed expected value for the topic on the first topic row
	for i := range lCols.Data {

		firstTopicLimb.SetBytes(firstTopicBytes[i*2 : (i+1)*2])
		outData := lCols.Data[i].GetColAssignmentAt(run, pos)

		if !firstTopicLimb.Equal(&outData) {
			return false
		}
	}

	return true
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

	var data [common.NbLimbU256][]field.Element
	for i := range data {
		data[i] = make([]field.Element, lCols.Ct.Size())
	}

	filterFetched := make([]field.Element, lCols.Ct.Size())
	counter := 0 // counter used to incrementally populate limbs of the ExtractedData and their associated filterFetched
	for i := 0; i < lCols.Ct.Size(); i++ {
		// the following conditional checks if row i contains a message that should be picked
		if !IsPositionTargetMessage(run, lCols, sel, i, logType) {
			continue
		}

		for j := range data {
			// pick the messages and add them to the msg limbs ExtractedData columns
			data[j][counter] = lCols.Data[j].GetColAssignmentAt(run, i)
		}

		// now set the filter on ExtractedData columns to be 1
		filterFetched[counter].SetOne()
		// set the filter on the LogColumns to be 1, at position i
		filterLogs[i].SetOne()
		counter++
	}

	// assign our fetched data
	for i := range data {
		run.AssignColumn(fetched.Data[i].GetColID(), smartvectors.NewRegular(data[i]))
	}

	// assign filters for original log columns and fetched ExtractedData

	// As the columns filterLogs is co-located with arithmetization loginfo
	// columns, it is important that the column is assigned as a left-padded
	// column. Otherwise, it will drive the segmentation to create an insane
	// number of segments. Another solution would be to tag the column with a
	// pragma but this would change the setup and this is implemented as a patch
	// on mainnet and we don't want to break the setup as we are adding the
	// patch.
	var filterLogsSV smartvectors.SmartVector = smartvectors.NewRegular(filterLogs)
	filterLogsSV, _ = smartvectors.TryReduceSizeLeft(filterLogsSV)
	run.AssignColumn(fetched.FilterArith.GetColID(), filterLogsSV)                             // filter on LogColumns
	run.AssignColumn(fetched.FilterFetched.GetColID(), smartvectors.NewRegular(filterFetched)) // filter on fetched data
}
