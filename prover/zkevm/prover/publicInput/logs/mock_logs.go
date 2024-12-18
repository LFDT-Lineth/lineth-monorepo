package logs

import (
	eth "github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	// types of logs
	LOG0        int = 0
	LOG1        int = 1
	LOG2        int = 2
	LOG3        int = 3
	LOG4        int = 4
	MISSING_LOG int = 5
)

func noTopics(logType int) int {
	return logType
}

// LogInfo will be a mock data structure containing the minimal amount of information
// needed to generate test logs
type LogInfo struct {
	LogType              int
	DataSize, noTopics   field.Element
	AddressHi, AddressLo field.Element
	TopicsHi, TopicsLo   []field.Element
}

// LogColumns represents the relevant columns for l2l1logs/RollingHash logs from the LogInfo module.
type LogColumns struct {
	IsLog0, IsLog1, IsLog2, IsLog3, IsLog4 ifaces.Column
	AbsLogNum                              ifaces.Column
	AbsLogNumMax                           ifaces.Column // total number of logs in the conflated batch
	Ct                                     ifaces.Column // counter column used inside a column segment used for one specific log
	DataHi, DataLo                         ifaces.Column // the Hi and Lo parts of outgoing data
	TxEmitsLogs                            ifaces.Column
}

// ConvertAddress converts a 20 bytes address into the HI and LO parts on the arithmetization side
func ConvertAddress(address eth.Address) (field.Element, field.Element) {
	var hi, lo field.Element
	hi.SetBytes(address[:4])
	lo.SetBytes(address[4:])
	return hi, lo
}

// ComputeSize computes the size of columns that have the same shape as the ones in the LogInfo module
// by taking into account the number of topics in each subsegment
func ComputeSize(logs []LogInfo) int {
	size := 0
	for _, log := range logs {
		if log.LogType == MISSING_LOG {
			size++
		} else {
			size += 2 + len(log.TopicsHi)
		}
	}
	return size
}

// ConvertToL2L1Log converts LogInfo structs into Log ones by adding dummy information for
// fees, value, salt, offset and calldata
func (logInfo LogInfo) ConvertToL2L1Log() types.Log {

	// compute the topics
	var topics []ethCommon.Hash
	for i := 0; i < len(logInfo.TopicsHi); i++ {
		bytesHi := logInfo.TopicsHi[i].Bytes()
		bytesLo := logInfo.TopicsLo[i].Bytes()
		hashBytes := make([]byte, 0, 32)
		hashBytes = append(hashBytes, bytesHi[16:]...)
		hashBytes = append(hashBytes, bytesLo[16:]...)
		topics = append(topics, ethCommon.BytesToHash(hashBytes))
	}
	var data []byte

	// add dummy bytes for fees, value and salt
	var dummy field.Element
	dummy.SetInt64(21) // 21 is a dummy value
	dummyBytes := dummy.Bytes()
	data = append(data, dummyBytes[:]...)
	data = append(data, dummyBytes[:]...)
	data = append(data, dummyBytes[:]...)

	// add bytes for the offset, offset must have the following for
	var offsetBytes = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4 * 32}
	data = append(data, offsetBytes[:]...)
	// add calldata
	var callData = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	data = append(data, callData[:]...)

	res := types.Log{
		Address:     ethCommon.HexToAddress("DUMMY"),
		Topics:      topics,
		Data:        data,
		BlockNumber: 0,
		TxHash:      ethCommon.Hash{},
		TxIndex:     0,
		BlockHash:   ethCommon.Hash{},
		Index:       0,
		Removed:     false,
	}
	return res
}

// NewLogColumns returns a new LogColumns with initialized
// columns that are not constrained.
func NewLogColumns(comp *wizard.CompiledIOP, size int, name string) LogColumns {

	createCol := func(subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("LOG_COLUMNS_%v_%v", name, subName),
			size,
		)
	}

	res := LogColumns{
		IsLog0:       createCol("IS_LOG_0"),
		IsLog1:       createCol("IS_LOG_1"),
		IsLog2:       createCol("IS_LOG_2"),
		IsLog3:       createCol("IS_LOG_3"),
		IsLog4:       createCol("IS_LOG_4"),
		AbsLogNum:    createCol("ABS_LOG_NUM"),
		AbsLogNumMax: createCol("ABS_LOG_NUM_MAX"),
		Ct:           createCol("CT"),
		DataHi:       createCol("OUTGOING_HI"),
		DataLo:       createCol("OUTGOING_LO"),
		TxEmitsLogs:  createCol("TX_EMITS_LOGS"),
	}

	return res
}

// LogColumnsAssignmentBuilder is a convenience structure storing
// the column builders relating to a LogColumns.
type LogColumnsAssignmentBuilder struct {
	IsLog0, IsLog1, IsLog2, IsLog3, IsLog4 *common.VectorBuilder
	AbsLogNum, AbsLogNumMax                *common.VectorBuilder
	Ct                                     *common.VectorBuilder
	OutgoingHi, OutgoingLo                 *common.VectorBuilder
	TxEmitsLogs                            *common.VectorBuilder
}

// NewLogColumnsAssignmentBuilder initializes a fresh LogColumnsAssignmentBuilder
func NewLogColumnsAssignmentBuilder(lc *LogColumns) LogColumnsAssignmentBuilder {
	return LogColumnsAssignmentBuilder{
		IsLog0:       common.NewVectorBuilder(lc.IsLog0),
		IsLog1:       common.NewVectorBuilder(lc.IsLog1),
		IsLog2:       common.NewVectorBuilder(lc.IsLog2),
		IsLog3:       common.NewVectorBuilder(lc.IsLog3),
		IsLog4:       common.NewVectorBuilder(lc.IsLog4),
		AbsLogNum:    common.NewVectorBuilder(lc.AbsLogNum),
		AbsLogNumMax: common.NewVectorBuilder(lc.AbsLogNumMax),
		Ct:           common.NewVectorBuilder(lc.Ct),
		OutgoingHi:   common.NewVectorBuilder(lc.DataHi),
		OutgoingLo:   common.NewVectorBuilder(lc.DataLo),
		TxEmitsLogs:  common.NewVectorBuilder(lc.TxEmitsLogs),
	}

}

// PushLogSelectors populates the IsLogX and TxEmitsLogs columns in what will become LogColumns
func (lc *LogColumnsAssignmentBuilder) PushLogSelectors(logType int) {
	switch logType {
	case LOG0:
		lc.IsLog0.PushOne()
		lc.IsLog1.PushZero()
		lc.IsLog2.PushZero()
		lc.IsLog3.PushZero()
		lc.IsLog4.PushZero()
		lc.TxEmitsLogs.PushOne()
	case LOG1:
		lc.IsLog0.PushZero()
		lc.IsLog1.PushOne()
		lc.IsLog2.PushZero()
		lc.IsLog3.PushZero()
		lc.IsLog4.PushZero()
		lc.TxEmitsLogs.PushOne()
	case LOG2:
		lc.IsLog0.PushZero()
		lc.IsLog1.PushZero()
		lc.IsLog2.PushOne()
		lc.IsLog3.PushZero()
		lc.IsLog4.PushZero()
		lc.TxEmitsLogs.PushOne()
	case LOG3:
		lc.IsLog0.PushZero()
		lc.IsLog1.PushZero()
		lc.IsLog2.PushZero()
		lc.IsLog3.PushOne()
		lc.IsLog4.PushZero()
		lc.TxEmitsLogs.PushOne()
	case LOG4:
		lc.IsLog0.PushZero()
		lc.IsLog1.PushZero()
		lc.IsLog2.PushZero()
		lc.IsLog3.PushZero()
		lc.IsLog4.PushOne()
		lc.TxEmitsLogs.PushOne()
	case MISSING_LOG:
		lc.IsLog0.PushZero()
		lc.IsLog1.PushZero()
		lc.IsLog2.PushZero()
		lc.IsLog3.PushZero()
		lc.IsLog4.PushZero()
		lc.TxEmitsLogs.PushZero()
	}
}

// PushCounters populates the counter columns in what will become LogColumns
func (lc *LogColumnsAssignmentBuilder) PushCounters(absLogNum, absLogNumMax, ct int) {
	lc.AbsLogNum.PushInt(absLogNum)
	lc.AbsLogNumMax.PushInt(absLogNumMax)
	lc.Ct.PushInt(ct)
}

// PadAndAssign pads all the column in `as` and assign them into `run`
func (lc *LogColumnsAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	lc.IsLog0.PadAndAssign(run)
	lc.IsLog1.PadAndAssign(run)
	lc.IsLog2.PadAndAssign(run)
	lc.IsLog3.PadAndAssign(run)
	lc.IsLog4.PadAndAssign(run)
	lc.AbsLogNum.PadAndAssign(run)
	lc.AbsLogNumMax.PadAndAssign(run)
	lc.Ct.PadAndAssign(run)
	lc.OutgoingHi.PadAndAssign(run)
	lc.OutgoingLo.PadAndAssign(run)
	lc.TxEmitsLogs.PadAndAssign(run)
}

// LogColumnsAssign uses test samples from LogInfo to populate LogColumns uses for testing
// in the fetching of messages from L2L1/RollingHash logs
func LogColumnsAssign(run *wizard.ProverRuntime, logCols *LogColumns, logs []LogInfo) {
	builder := NewLogColumnsAssignmentBuilder(logCols)
	for i := 0; i < len(logs); i++ {
		logType := logs[i].LogType
		// row 0
		builder.PushLogSelectors(logs[i].LogType)
		builder.PushCounters(i, len(logs), 0)
		builder.OutgoingHi.PushField(logs[i].DataSize)
		builder.OutgoingLo.PushField(logs[i].noTopics)

		if logType != MISSING_LOG {
			// row 1 has a special form
			builder.PushLogSelectors(logs[i].LogType)
			builder.PushCounters(i, len(logs), 1)
			builder.OutgoingHi.PushField(logs[i].AddressHi)
			builder.OutgoingLo.PushField(logs[i].AddressLo)

			// subsequent rows contain the topic data
			for topicNo := 0; topicNo < noTopics(logType); topicNo++ {
				builder.PushLogSelectors(logs[i].LogType)
				builder.PushCounters(i, len(logs), topicNo+2) // topicNo+2, starting at row index 2
				builder.OutgoingHi.PushField(logs[i].TopicsHi[topicNo])
				builder.OutgoingLo.PushField(logs[i].TopicsLo[topicNo])
			}
		}
	}
	builder.PadAndAssign(run)
}
