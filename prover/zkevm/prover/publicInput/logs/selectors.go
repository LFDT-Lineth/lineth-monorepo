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
// Case 2: Rolling Hash Log: we extract either the message number or the Rolling hash itself
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

// GetSelectorFirstTopicHi returns the appropriate selector column for L2L1 or RollingHash logs
func GetSelectorFirstTopicHi(sel Selectors, logType int) ifaces.Column {
	switch logType {
	case L2L1:
		return sel.SelectFirstTopicL2L1Hi
	default: // RollingMsgNo or RollingHash
		return sel.SelectFirstTopicRollingHi
	}
}

// GetSelectorFirstTopicLo returns the appropriate selector column for L2L1 or RollingHash logs
func GetSelectorFirstTopicLo(sel Selectors, logType int) ifaces.Column {
	switch logType {
	case L2L1:
		return sel.SelectFirstTopicL2L1Lo
	default: // RollingMsgNo or RollingHash
		return sel.SelectFirstTopicRollingLo
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
	SelectFirstTopicL2L1Hi, SelectFirstTopicL2L1Lo               ifaces.Column
	ComputeSelectFirstTopicL2L1Hi, ComputeSelectFirstTopicL2L1Lo wizard.ProverAction
	// SelectFirstTopicRollingHi/Lo is 1 on rows where the first topic has the shape expected from L2L1 logs
	SelectFirstTopicRollingHi, SelectFirstTopicRollingLo               ifaces.Column
	ComputeSelectFirstTopicRollingHi, ComputeSelectFirstTopicRollingLo wizard.ProverAction

	// columns containing the hi and lo parts of l2BridgeAddress
	L2BridgeAddressColHI, L2BridgeAddressColLo ifaces.Column
	// SelectorL2BridgeAddressHi/Lo is 1 on rows where the OutgoingHi/Lo columns contain the bridge address,
	// as expected from L2L1 logs
	SelectorL2BridgeAddressHi, SelectorL2BridgeAddressLo               ifaces.Column
	ComputeSelectorL2BridgeAddressHi, ComputeSelectorL2BridgeAddressLo wizard.ProverAction
}

/*
NewSelectorColumns creates the selector columns used to fetch data from LogColumns
*/
func NewSelectorColumns(comp *wizard.CompiledIOP, lc LogColumns) Selectors {
	// first compute selectors that light up when Ct=0, Ct=1, and Ct=5
	SelectorCounter0, ComputeSelectorCounter0 := dedicated.IsZero(
		comp,
		lc.Ct,
	)
	SelectorCounter1, ComputeSelectorCounter1 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 1),
	)

	SelectorCounter3, ComputeSelectorCounter3 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 3),
	)

	SelectorCounter4, ComputeSelectorCounter4 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 4),
	)

	SelectorCounter5, ComputeSelectorCounter5 := dedicated.IsZero(
		comp,
		sym.Sub(lc.Ct, 5),
	)

	// compute the expected data in the first topic of a L2L1 log
	var firstTopicL2L1Hi, firstTopicL2L1Lo field.Element
	firstTopicBytes := bridge.L2L1Topic0() // fixed expected value for the topic on the first topic row
	firstTopicL2L1Hi.SetBytes(firstTopicBytes[:16])
	firstTopicL2L1Lo.SetBytes(firstTopicBytes[16:])

	// selectors that light up when OutgoingHi/OutgoingLo contain the expected first topic data
	SelectFirstTopicL2L1Hi, ComputeSelectFirstTopicL2L1Hi := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataHi, firstTopicL2L1Hi),
	)

	SelectFirstTopicL2L1Lo, ComputeSelectFirstTopicL2L1Lo := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataLo, firstTopicL2L1Lo),
	)

	// compute the expected data in the first topic of a rolling hash log
	var firstTopicRollingHi, firstTopicRollingLo field.Element
	firstTopicRollingBytes := bridge.GetRollingHashUpdateTopic0() // fixed expected value for the topic on the first topic row
	firstTopicRollingHi.SetBytes(firstTopicRollingBytes[:16])
	firstTopicRollingLo.SetBytes(firstTopicRollingBytes[16:])

	// selectors that light up when OutgoingHi/OutgoingLo contain the expected first topic data
	SelectFirstTopicRollingHi, ComputeSelectFirstTopicRollingHi := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataHi, firstTopicRollingHi),
	)

	SelectFirstTopicRollingLo, ComputeSelectFirstTopicRollingLo := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataLo, firstTopicRollingLo),
	)

	bridgeAddrColHi := comp.InsertCommit(0, ifaces.ColIDf("LOGS_FETCHER_BRIDGE_ADDRESS_HI"), lc.DataHi.Size())
	bridgeAddrColLo := comp.InsertCommit(0, ifaces.ColIDf("LOGS_FETCHER_BRIDGE_ADDRESS_LO"), lc.DataLo.Size())

	commonconstraints.MustBeConstant(comp, bridgeAddrColHi)
	commonconstraints.MustBeConstant(comp, bridgeAddrColLo)

	// selectors that light up when OutgoingHi/OutgoingLo contain the Hi/Lo parts of the l2BridgeAddress
	SelectorL2BridgeAddressHi, ComputeSelectorL2BridgeAddressHi := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataHi, bridgeAddrColHi),
	)

	SelectorL2BridgeAddressLo, ComputeSelectorL2BridgeAddressLo := dedicated.IsZero(
		comp,
		sym.Sub(lc.DataLo, bridgeAddrColLo),
	)

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
		SelectFirstTopicL2L1Hi:        SelectFirstTopicL2L1Hi,
		ComputeSelectFirstTopicL2L1Hi: ComputeSelectFirstTopicL2L1Hi,
		SelectFirstTopicL2L1Lo:        SelectFirstTopicL2L1Lo,
		ComputeSelectFirstTopicL2L1Lo: ComputeSelectFirstTopicL2L1Lo,
		// selectors that light up on rows that contain the expected first topic for Rolling hashes
		SelectFirstTopicRollingHi:        SelectFirstTopicRollingHi,
		ComputeSelectFirstTopicRollingHi: ComputeSelectFirstTopicRollingHi,
		SelectFirstTopicRollingLo:        SelectFirstTopicRollingLo,
		ComputeSelectFirstTopicRollingLo: ComputeSelectFirstTopicRollingLo,
		// columns and a helper field which contain the l2bridgeAddress
		L2BridgeAddressColHI: bridgeAddrColHi,
		L2BridgeAddressColLo: bridgeAddrColLo,
		// selectors that light up on rows that contain the expected l2bridgeAddress
		SelectorL2BridgeAddressHi:        SelectorL2BridgeAddressHi,
		ComputeSelectorL2BridgeAddressHi: ComputeSelectorL2BridgeAddressHi,
		SelectorL2BridgeAddressLo:        SelectorL2BridgeAddressLo,
		ComputeSelectorL2BridgeAddressLo: ComputeSelectorL2BridgeAddressLo,
	}
	return res
}

// Assign values for the selectors
func (sel Selectors) Assign(run *wizard.ProverRuntime, l2BridgeAddress common.Address) {

	addrHi, addrLo := ConvertAddress(statemanager.Address(l2BridgeAddress))
	size := sel.L2BridgeAddressColHI.Size()

	// assign the columns that contain the l2 bridge address
	run.AssignColumn(sel.L2BridgeAddressColHI.GetColID(), smartvectors.NewConstant(addrHi, size))
	run.AssignColumn(sel.L2BridgeAddressColLo.GetColID(), smartvectors.NewConstant(addrLo, size))

	// now we assign the dedicated selectors for counters
	sel.ComputeSelectorCounter0.Run(run)
	sel.ComputeSelectorCounter1.Run(run)
	sel.ComputeSelectorCounter3.Run(run)
	sel.ComputeSelectorCounter4.Run(run)
	sel.ComputeSelectorCounter5.Run(run)

	// now we assign the dedicated selectors for the two type of first topic
	sel.ComputeSelectFirstTopicL2L1Hi.Run(run)
	sel.ComputeSelectFirstTopicL2L1Lo.Run(run)
	sel.ComputeSelectFirstTopicRollingHi.Run(run)
	sel.ComputeSelectFirstTopicRollingLo.Run(run)

	// now we assign the dedicated selectors for the bridge address
	sel.ComputeSelectorL2BridgeAddressHi.Run(run)
	sel.ComputeSelectorL2BridgeAddressLo.Run(run)
}
