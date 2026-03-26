package logs

import (
	"fmt"
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
	LOG0                  int = 0
	LOG1                  int = 1
	LOG2                  int = 2
	LOG3                  int = 3
	LOG4                  int = 4
	MISSING_LOG           int = 5
	BRIDGE_LOG_SLICE_SIZE     = 32
)

func noTopics(logType int) int {
	return logType
}

// LogInfo will be a mock data structure containing the minimal amount of information
// needed to generate test logs
type LogInfo struct {
	LogType            int
	DataSize, noTopics [common.NbLimbU128]field.Element
	Address            [common.NbLimbEthAddress]field.Element
	Topics             [][common.NbLimbU256]field.Element
}

// LogColumns represents the relevant columns for l2l1logs/RollingHash logs from the LogInfo module.
type LogColumns struct {
	IsLog0, IsLog1, IsLog2, IsLog3, IsLog4 ifaces.Column
	AbsLogNum                              ifaces.Column
	// total number of logs in the conflated batch
	AbsLogNumMax ifaces.Column
	// counter column used inside a column segment used for one specific log
	Ct ifaces.Column
	// the logs outgoing data
	Data        [common.NbLimbU256]ifaces.Column
	TxEmitsLogs ifaces.Column
}

// ConvertAddress converts a 20 bytes address into the 10 16-bit limbs on the arithmetization side
func ConvertAddress(address eth.Address) [common.NbLimbEthAddress]field.Element {
	var res [common.NbLimbEthAddress]field.Element

	for i := range res {
		res[i].SetBytes(address[i*2 : (i+1)*2])
	}

	return res
}

// ComputeSize computes the size of columns that have the same shape as the ones in the LogInfo module
// by taking into account the number of topics in each subsegment
func ComputeSize(logs []LogInfo) int {
	size := 0
	for _, log := range logs {
		if log.LogType == MISSING_LOG {
			size++
		} else {
			size += 2 + len(log.Topics)
		}
	}
	return size
}

// ConvertToL2L1Log converts LogInfo structs into Log ones by adding dummy information for
// fees, value, salt, offset and calldata
func (logInfo LogInfo) ConvertToL2L1Log() types.Log {

	// compute the topics
	var topics []ethCommon.Hash
	for i := 0; i < len(logInfo.Topics); i++ {
		var hashBytes [32]byte
		for j := range logInfo.Topics[i] {
			bytes := logInfo.Topics[i][j].Bytes()
			// in the second term, we have field.Bytes-2 as the lower bound
			// because the first part of the arithmetization bytes are leading zeros
			copy(hashBytes[j*2:(j+1)*2], bytes[field.Bytes-2:])
		}
		topics = append(topics, ethCommon.BytesToHash(hashBytes[:]))
	}
	var data []byte

	// add dummy bytes for fees, value and salt
	// dummyBytesFunc returns the value of z as a big-endian byte array
	dummyBytesFunc := func() (res [BRIDGE_LOG_SLICE_SIZE]byte) {
		var dummy field.Element
		dummy.SetInt64(21) // 21 is a dummy value
		aux := dummy.Bytes()
		temp := make([]byte, BRIDGE_LOG_SLICE_SIZE-field.Bytes)
		res = [32]byte(append(temp, aux[:]...))
		return res
	}
	dummyBytes := dummyBytesFunc()

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
			true,
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
		TxEmitsLogs:  createCol("TX_EMITS_LOGS"),
	}

	for i := range res.Data {
		res.Data[i] = createCol(fmt.Sprintf("OUTGOING_%d", i))
	}

	return res
}

// LogColumnsAssignmentBuilder is a convenience structure storing
// the column builders relating to a LogColumns.
type LogColumnsAssignmentBuilder struct {
	IsLog0, IsLog1, IsLog2, IsLog3, IsLog4 *common.VectorBuilder
	AbsLogNum, AbsLogNumMax                *common.VectorBuilder
	Ct                                     *common.VectorBuilder
	Outgoing                               [common.NbLimbU256]*common.VectorBuilder
	TxEmitsLogs                            *common.VectorBuilder
}

// NewLogColumnsAssignmentBuilder initializes a fresh LogColumnsAssignmentBuilder
func NewLogColumnsAssignmentBuilder(lc *LogColumns) LogColumnsAssignmentBuilder {
	res := LogColumnsAssignmentBuilder{
		IsLog0:       common.NewVectorBuilder(lc.IsLog0),
		IsLog1:       common.NewVectorBuilder(lc.IsLog1),
		IsLog2:       common.NewVectorBuilder(lc.IsLog2),
		IsLog3:       common.NewVectorBuilder(lc.IsLog3),
		IsLog4:       common.NewVectorBuilder(lc.IsLog4),
		AbsLogNum:    common.NewVectorBuilder(lc.AbsLogNum),
		AbsLogNumMax: common.NewVectorBuilder(lc.AbsLogNumMax),
		Ct:           common.NewVectorBuilder(lc.Ct),
		TxEmitsLogs:  common.NewVectorBuilder(lc.TxEmitsLogs),
	}

	for i := range res.Outgoing {
		res.Outgoing[i] = common.NewVectorBuilder(lc.Data[i])
	}

	return res
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
	lc.TxEmitsLogs.PadAndAssign(run)

	for i := range lc.Outgoing {
		lc.Outgoing[i].PadAndAssign(run)
	}
}

// LogColumnsAssign uses test samples from LogInfo to populate LogColumns uses for testing
// in the fetching of messages from L2L1/RollingHash logs
func LogColumnsAssign(run *wizard.ProverRuntime, logCols *LogColumns, logs []LogInfo) {
	builder := NewLogColumnsAssignmentBuilder(logCols)

	addrOffset := common.NbLimbU256 - common.NbLimbEthAddress

	for i := 0; i < len(logs); i++ {
		logType := logs[i].LogType
		// row 0
		builder.PushLogSelectors(logs[i].LogType)
		builder.PushCounters(i, len(logs), 0)

		for j := range common.NbLimbU128 {
			builder.Outgoing[j].PushField(logs[i].DataSize[j])
			builder.Outgoing[common.NbLimbU128+j].PushField(logs[i].noTopics[j])
		}

		if logType != MISSING_LOG {
			// row 1 has a special form
			builder.PushLogSelectors(logs[i].LogType)
			builder.PushCounters(i, len(logs), 1)

			for j := 0; j < addrOffset; j++ {
				builder.Outgoing[j].PushField(field.Zero())
			}
			for j := 0; j < common.NbLimbEthAddress; j++ {
				builder.Outgoing[j+addrOffset].PushField(logs[i].Address[j])
			}

			// subsequent rows contain the topic data
			for topicNo := 0; topicNo < noTopics(logType); topicNo++ {
				builder.PushLogSelectors(logs[i].LogType)
				builder.PushCounters(i, len(logs), topicNo+2) // topicNo+2, starting at row index 2
				for j := range common.NbLimbU256 {
					builder.Outgoing[j].PushField(logs[i].Topics[topicNo][j])
				}
			}
		}
	}

	builder.PadAndAssign(run)
}
