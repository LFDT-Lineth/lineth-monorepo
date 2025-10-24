package arith_struct

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

// BlockDataCols models the arithmetization's BlockData module
type BlockDataCols struct {
	// RelBlock is the relative block number, ranging from 1 to the total number of blocks
	RelBlock ifaces.Column
	// Inst encodes the type of the row
	Inst ifaces.Column
	// Ct is a counter column
	Ct ifaces.Column
	// DataHi/DataLo encode the data, for example the timestamps
	DataHi, DataLo ifaces.Column
	// FirstBlock contains the absolute ID of the first block
	FirstBlock ifaces.Column
}

// TxnData models the arithmetization's TxnData module
type TxnData struct {
	AbsTxNum, AbsTxNumMax ifaces.Column // Absolute number of the transaction (starts from 1 and acts as an Active Filter), and the maximum number of transactions
	RelTxNum, RelTxNumMax ifaces.Column // Relative TxNum inside the block,
	FromHi, FromLo        ifaces.Column // Sender address
	IsLastTxOfBlock       ifaces.Column // 1 if this is the last transaction inside the block
	RelBlock              ifaces.Column // Relative Block number inside the batch
	Ct                    ifaces.Column
	USER                  ifaces.Column // 1 if this is a user transaction, 0 otherwise
	Selector              ifaces.Column // we require an additional selector to identify which data to fetch
	SYSI                  ifaces.Column
	SYSF                  ifaces.Column
}

// RlpTxn models the arithmetization's RlpTxn module
type RlpTxn struct {
	AbsTxNum, AbsTxNumMax ifaces.Column // Absolute number of the transaction (starts from 1 and acts as an Active Filter), and the maximum number of transactions
	ToHashByProver        ifaces.Column // Relative TxNum inside the block,
	Limb                  ifaces.Column
	NBytes                ifaces.Column // the number of bytes to load from the limb
	TxnPerspective        ifaces.Column // indicator column for the transaction perspective, which we will use to obtain the ChainID
	ChainID               ifaces.Column // dedicated column for the ChainID
}

// DefineTestingArithModules defines the BlockDataCols, TxnData and RlpTxn modules based on csv traces.
// if a csvTrace is nil, the corresponding module will be missing in the return data.
func DefineTestingArithModules(b *wizard.Builder, ctBlockData, ctTxnData, ctRlpTxn *csvtraces.CsvTrace) (*BlockDataCols, *TxnData, *RlpTxn) {
	var (
		blockDataCols *BlockDataCols
		txnDataCols   *TxnData
		rlpTxn        *RlpTxn
	)
	if ctBlockData != nil {
		blockDataCols = &BlockDataCols{
			RelBlock:   ctBlockData.GetCommit(b, "REL_BLOCK"),
			Inst:       ctBlockData.GetCommit(b, "INST"),
			Ct:         ctBlockData.GetCommit(b, "CT"),
			DataHi:     ctBlockData.GetCommit(b, "DATA_HI"),
			DataLo:     ctBlockData.GetCommit(b, "DATA_LO"),
			FirstBlock: ctBlockData.GetCommit(b, "FIRST_BLOCK_NUMBER"),
		}
	}
	if ctTxnData != nil {
		txnDataCols = &TxnData{
			AbsTxNum:        ctTxnData.GetCommit(b, "TD.ABS_TX_NUM"),
			AbsTxNumMax:     ctTxnData.GetCommit(b, "TD.ABS_TX_NUM_MAX"),
			RelTxNum:        ctTxnData.GetCommit(b, "TD.REL_TX_NUM"),
			RelTxNumMax:     ctTxnData.GetCommit(b, "TD.REL_TX_NUM_MAX"),
			Ct:              ctTxnData.GetCommit(b, "TD.CT"),
			FromHi:          ctTxnData.GetCommit(b, "TD.FROM_HI"),
			FromLo:          ctTxnData.GetCommit(b, "TD.FROM_LO"),
			IsLastTxOfBlock: ctTxnData.GetCommit(b, "TD.IS_LAST_TX_OF_BLOCK"),
			RelBlock:        ctTxnData.GetCommit(b, "TD.REL_BLOCK"),
			USER:            ctTxnData.GetCommit(b, "TD.USER"),
			Selector:        ctTxnData.GetCommit(b, "TD.SELECTOR"),
			SYSI:            ctTxnData.GetCommit(b, "TD.SYSI"),
			SYSF:            ctTxnData.GetCommit(b, "TD.SYSF"),
		}
	}
	if ctRlpTxn != nil {
		rlpTxn = &RlpTxn{
			AbsTxNum:       ctRlpTxn.GetCommit(b, "RT.ABS_TX_NUM"),
			AbsTxNumMax:    ctRlpTxn.GetCommit(b, "RT.ABS_TX_NUM_MAX"),
			ToHashByProver: ctRlpTxn.GetCommit(b, "RL.TO_HASH_BY_PROVER"),
			Limb:           ctRlpTxn.GetCommit(b, "RL.LIMB"),
			NBytes:         ctRlpTxn.GetCommit(b, "RL.NBYTES"),
			TxnPerspective: ctRlpTxn.GetCommit(b, "RL.TXN"),
			ChainID:        ctRlpTxn.GetCommit(b, "RL.CHAIN_ID"),
		}
	}

	return blockDataCols, txnDataCols, rlpTxn
}

// AssignTestingArithModules assigns the BlockDataCols, TxnData and RlpTxn modules based on csv traces.
// if a module is missing,the corresponding assignment is skipped
func AssignTestingArithModules(run *wizard.ProverRuntime, ctBlockData, ctTxnData, ctRlpTxn *csvtraces.CsvTrace) {
	// assign the CSV data for the mock BlockData, TxnData and RlpTxn arithmetization modules
	if ctBlockData != nil {
		ctBlockData.Assign(
			run,
			"REL_BLOCK",
			"INST",
			"CT",
			"DATA_HI",
			"DATA_LO",
			"FIRST_BLOCK_NUMBER",
		)
	}
	if ctTxnData != nil {
		ctTxnData.Assign(
			run,
			"TD.ABS_TX_NUM",
			"TD.ABS_TX_NUM_MAX",
			"TD.REL_TX_NUM",
			"TD.REL_TX_NUM_MAX",
			"TD.CT",
			"TD.FROM_HI",
			"TD.FROM_LO",
			"TD.IS_LAST_TX_OF_BLOCK",
			"TD.REL_BLOCK",
			"TD.USER",
			"TD.SELECTOR",
			"TD.SYSI",
			"TD.SYSF",
		)
	}
	if ctRlpTxn != nil {
		ctRlpTxn.Assign(
			run,
			"RT.ABS_TX_NUM",
			"RT.ABS_TX_NUM_MAX",
			"RL.TO_HASH_BY_PROVER",
			"RL.LIMB",
			"RL.NBYTES",
			"RL.TXN",
			"RL.CHAIN_ID",
		)
	}

}
