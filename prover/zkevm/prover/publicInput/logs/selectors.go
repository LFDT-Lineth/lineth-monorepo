package logs

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	pcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/ethereum/go-ethereum/common"
)

// constants used to retrieve row offsets for various types of hashes
const (
	L2L1            int = 0
	RollingMsgNo    int = 1
	RollingHash     int = 2
	FirstTopic      int = 0
	L2BridgeAddress int = 1
)

// GetOffset returns relevant offsets depending on the data we want to fetch from the arithmetization
// Case 1: L2L1 log, we want to extract the keccak-hashed messages.
// The first topic can be found 3 rows before, and the bridge address 4 rows before.
// Case 2: Rolling HashFirst Log: we extract either the message number or the Rolling hash itself
// the offsets for the first topic/bridgeAddress are -1/-2 and -2/-3
func GetOffset(logType, offsetType int) int {
	switch logType {
	case L2L1:
		switch offsetType {
		case FirstTopic:
			return -3
		case L2BridgeAddress:
			return -4
		}
	case RollingMsgNo:
		switch offsetType {
		case FirstTopic:
			return -1
		case L2BridgeAddress:
			return -2
		}
	case RollingHash:
		switch offsetType {
		case FirstTopic:
			return -2
		case L2BridgeAddress:
			return -3
		}
	}
	panic("Arithmetization log fetcher: wrong arguments in log type")
}

// GetFirstTopic returns the first topic of either an L2L1 or RollingHash log
func GetFirstTopic(logType int) [32]byte {
	switch logType {
	case L2L1:
		return bridge.L2L1Topic0()
	default: // RollingMsgNo or RollingHash
		return bridge.GetRollingHashUpdateTopic0()
	}
}

// IsLogType is used to select the proper isLogX column,
// it returns isLog4 for L2L1 logs and isLog3 for RollingHash logs
func IsLogType(columns LogColumns, logType int) ifaces.Column {
	switch logType {
	case L2L1:
		return columns.IsLog4
	default: // RollingMsgNo or RollingHash
		return columns.IsLog3
	}
}

// GetSelectorFirstTopic returns the appropriate selector column for L2L1 or RollingHash logs
func GetSelectorFirstTopic(sel Selectors, logType int) [pcommon.NbLimbU256]ifaces.Column {
	switch logType {
	case L2L1:
		return sel.SelectFirstTopicL2L1
	default: // RollingMsgNo or RollingHash
		return sel.SelectFirstTopicRolling
	}
}

// GetPositionCounter returns the expected counter value for the data type we want to fetch
func GetPositionCounter(dataType int) int {
	switch dataType {
	case L2L1:
		return 5
	case RollingMsgNo:
		return 3
	case RollingHash:
		return 4
	}
	panic("Arithmetization log fetcher: wrong arguments in log dataType")
}

// GetSelectorCounter returns the appropriate counter selector column for L2L1 or RollingHash logs
func GetSelectorCounter(sel Selectors, logType int) ifaces.Column {
	switch logType {
	case L2L1:
		return sel.SelectorCounter5
	case RollingMsgNo:
		return sel.SelectorCounter3
	case RollingHash:
		return sel.SelectorCounter4
	}
	panic("Arithmetization log fetcher: wrong arguments in log type")
}

// GetName is an utility name function to generate column/constraint names
func GetName(logType int) string {
	switch logType {
	case L2L1:
		return "L2L1"
	case RollingMsgNo:
		return "ROLLING_MSG_NO"
	case RollingHash:
		return "ROLLING_HASH"
	}
	panic("Arithmetization log fetcher: wrong arguments in log type")
}

// Selectors contains helper columns for extracting the messages in a secure manner
type Selectors struct {
	// size of selector columns
	Size int

	// SelectorCounterX is 1 when the counter Ct column in the logs satisfies Ct = X and 0 otherwise
	SelectorCounter0, SelectorCounter1, SelectorCounter3, SelectorCounter4, SelectorCounter5                                    ifaces.Column
	ComputeSelectorCounter0, ComputeSelectorCounter1, ComputeSelectorCounter3, ComputeSelectorCounter4, ComputeSelectorCounter5 wizard.ProverAction
	// SelectFirstTopicL2L1Hi/Lo is 1 on rows where the first topic has the shape expected from L2L1 logs
	SelectFirstTopicL2L1        [pcommon.NbLimbU256]ifaces.Column
	ComputeSelectFirstTopicL2L1 [pcommon.NbLimbU256]wizard.ProverAction
	// SelectFirstTopicRollingHi/Lo is 1 on rows where the first topic has the shape expected from L2L1 logs
	SelectFirstTopicRolling        [pcommon.NbLimbU256]ifaces.Column
	ComputeSelectFirstTopicRolling [pcommon.NbLimbU256]wizard.ProverAction

	// columns containing the hi and lo parts of l2BridgeAddress
	L2BridgeAddressCol [pcommon.NbLimbEthAddress]ifaces.Column
	// SelectorL2BridgeAddressHi/Lo is 1 on rows where the OutgoingHi/Lo columns contain the bridge address,
	// as expected from L2L1 logs
	SelectorL2BridgeAddress        [pcommon.NbLimbU256]ifaces.Column
	ComputeSelectorL2BridgeAddress [pcommon.NbLimbU256]wizard.ProverAction
}

/*
NewSelectorColumns creates the selector columns used to fetch data from LogColumns
*/
func NewSelectorColumns(comp *wizard.CompiledIOP, lc LogColumns) Selectors {
	// first compute selectors that light up when Ct=0, Ct=1, and Ct=5
	SelectorCounter0, ComputeSelectorCounter0 := dedicated.IsZero(
		comp,
		lc.Ct,
	).GetColumnAndProverAction()
	SelectorCounter1, ComputeSelectorCounter1 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 1),
	).GetColumnAndProverAction()

	SelectorCounter3, ComputeSelectorCounter3 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 3),
	).GetColumnAndProverAction()

	SelectorCounter4, ComputeSelectorCounter4 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 4),
	).GetColumnAndProverAction()

	SelectorCounter5, ComputeSelectorCounter5 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 5),
	).GetColumnAndProverAction()

	// compute the expected data in the first topic of a L2L1 log
	var firstTopicL2L1 [pcommon.NbLimbU256]field.Element
	firstTopicBytes := bridge.L2L1Topic0() // fixed expected value for the topic on the first topic row
	for i := range firstTopicL2L1 {
		firstTopicL2L1[i].SetBytes(firstTopicBytes[i*2 : (i+1)*2])
	}

	// selectors that light up when OutgoingHi/OutgoingLo contain the expected first topic data
	var SelectFirstTopicL2L1 [pcommon.NbLimbU256]ifaces.Column
	var ComputeSelectFirstTopicL2L1 [pcommon.NbLimbU256]wizard.ProverAction
	for i := range SelectFirstTopicL2L1 {
		SelectFirstTopicL2L1[i], ComputeSelectFirstTopicL2L1[i] = dedicated.IsZero(
			comp,
			sym.Sub(lc.Data[i], firstTopicL2L1[i]),
		).GetColumnAndProverAction()
	}

	// compute the expected data in the first topic of a rolling hash log
	var firstTopicRolling [pcommon.NbLimbU256]field.Element
	firstTopicRollingBytes := bridge.GetRollingHashUpdateTopic0() // fixed expected value for the topic on the first topic row
	for i := range firstTopicRolling {
		firstTopicRolling[i].SetBytes(firstTopicRollingBytes[i*2 : (i+1)*2])
	}

	// selectors that light up when OutgoingHi/OutgoingLo contain the expected first topic data
	var SelectFirstTopicRolling [pcommon.NbLimbU256]ifaces.Column
	var ComputeSelectFirstTopicRolling [pcommon.NbLimbU256]wizard.ProverAction
	for i := range SelectFirstTopicRolling {
		SelectFirstTopicRolling[i], ComputeSelectFirstTopicRolling[i] = dedicated.IsZero(
			comp,
			sym.Sub(lc.Data[i], firstTopicRolling[i]),
		).GetColumnAndProverAction()
	}

	var bridgeAddrCol [pcommon.NbLimbEthAddress]ifaces.Column
	// selectors that light up when OutgoingHi/OutgoingLo contain the Hi/Lo parts of the l2BridgeAddress
	var SelectorL2BridgeAddress [pcommon.NbLimbU256]ifaces.Column
	var ComputeSelectorL2BridgeAddress [pcommon.NbLimbU256]wizard.ProverAction

	offset := pcommon.NbLimbU256 - pcommon.NbLimbEthAddress
	for i := range bridgeAddrCol {
		bridgeAddrCol[i] = comp.InsertCommit(0, ifaces.ColIDf("LOGS_FETCHER_BRIDGE_ADDRESS_%d", i), lc.Data[i].Size(), true)
		commonconstraints.MustBeConstant(comp, bridgeAddrCol[i])

		iOffset := i + offset
		SelectorL2BridgeAddress[iOffset], ComputeSelectorL2BridgeAddress[iOffset] =
			dedicated.IsZero(comp, sym.Sub(lc.Data[iOffset], bridgeAddrCol[i])).GetColumnAndProverAction()
	}

	// first limbs are zeroes as the address is 20 bytes long, while the data can be up to 32 bytes long
	for i := 0; i < offset; i++ {
		SelectorL2BridgeAddress[i], ComputeSelectorL2BridgeAddress[i] = dedicated.IsZero(comp, lc.Data[i]).GetColumnAndProverAction()
	}

	// generate the final selector object
	res := Selectors{
		Size: lc.Ct.Size(),
		// selectors that light up when Ct = 0, 1, or 5
		SelectorCounter0:        SelectorCounter0,
		SelectorCounter1:        SelectorCounter1,
		SelectorCounter3:        SelectorCounter3,
		SelectorCounter4:        SelectorCounter4,
		SelectorCounter5:        SelectorCounter5,
		ComputeSelectorCounter0: ComputeSelectorCounter0,
		ComputeSelectorCounter1: ComputeSelectorCounter1,
		ComputeSelectorCounter3: ComputeSelectorCounter3,
		ComputeSelectorCounter4: ComputeSelectorCounter4,
		ComputeSelectorCounter5: ComputeSelectorCounter5,
		// selectors that light up on rows that contain the expected first topic for L2L1 logs
		SelectFirstTopicL2L1:        SelectFirstTopicL2L1,
		ComputeSelectFirstTopicL2L1: ComputeSelectFirstTopicL2L1,
		// selectors that light up on rows that contain the expected first topic for Rolling hashes
		SelectFirstTopicRolling:        SelectFirstTopicRolling,
		ComputeSelectFirstTopicRolling: ComputeSelectFirstTopicRolling,
		// columns and a helper field which contain the l2bridgeAddress
		L2BridgeAddressCol: bridgeAddrCol,
		// selectors that light up on rows that contain the expected l2bridgeAddress
		SelectorL2BridgeAddress:        SelectorL2BridgeAddress,
		ComputeSelectorL2BridgeAddress: ComputeSelectorL2BridgeAddress,
	}
	return res
}

// Assign values for the selectors
func (sel Selectors) Assign(run *wizard.ProverRuntime, l2BridgeAddress common.Address) {

	addr := ConvertAddress(statemanager.Address(l2BridgeAddress))
	size := sel.L2BridgeAddressCol[0].Size()

	// assign the columns that contain the l2 bridge address
	for i := range addr {
		run.AssignColumn(sel.L2BridgeAddressCol[i].GetColID(), smartvectors.NewConstant(addr[i], size))
	}

	// now we assign the dedicated selectors for counters
	sel.ComputeSelectorCounter0.Run(run)
	sel.ComputeSelectorCounter1.Run(run)
	sel.ComputeSelectorCounter3.Run(run)
	sel.ComputeSelectorCounter4.Run(run)
	sel.ComputeSelectorCounter5.Run(run)

	// now we assign the dedicated selectors for the two type of first topic
	for i := range sel.SelectFirstTopicL2L1 {
		sel.ComputeSelectFirstTopicL2L1[i].Run(run)
		sel.ComputeSelectFirstTopicRolling[i].Run(run)
	}

	// now we assign the dedicated selectors for the bridge address
	for i := range sel.SelectorL2BridgeAddress {
		sel.ComputeSelectorL2BridgeAddress[i].Run(run)
	}
}
