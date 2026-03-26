package execution_data_collector

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
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	fetch "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/fetchers_arithmetization"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

/*
The ExecutionDataCollector module has as input data that was fetched from various modules of
the arithmetization, namely the BLOCKDATA, TXNDATA and RLPTXN modules. It constructs a series of limbs
that represent the execution data of the public input.

This module, along with its component arithmetization fetchers, is still a work in progress.

For each block with transactions tx_1...tx_n, we want to hash, in the following order:
The number of transactions in the block (2 bytes), the block timestamp (4 bytes), the blockhash (32 bytes),
and then for each transaction tx_i, the sender address (20 bytes) and the transaction RLP.
We then continue analogously for each block.

Due to design choices in the arithmetization and other submodules, we can only load at most 2 bytes
at a time. For this reason, blockhash is divided into 16 columns: [16]BlockHash (32 bytes in total).
Similarly, the sender address is divided into 10 columns: [10]Addr (20 bytes in total).


Finally, the RLP data for each transaction is stored in the RLPTXN module. We use an intermediary fetcher
which gives us the following columns:
AbsTxId, RLPLimb, NBytes,
where AbsTxId contains increasing, contiguous segments of the absolute transaction ids, followed by the
limb data and the NBytes column which indicates the number of relevant bytes stored in each row of
the RLPLimb column. To give more details, the AbsTxId is the transaction id inside the entire conflated batch,
across the multiple blocks present in the batch.

All the data to be fetched is ordered in the Limb and UnalignedLimb columns (we need both because different sources
have different data formats). We address the alignment formatting after we describe the structure of the
module. In order to load data from different sources, we use a series of indicator columns:
IsNoTx, IsBlockHashHi, IsBlockHashLo, IsTimestamp, IsTxRLP, IsAddrHi, IsAddrLo, which signal which
type of data we are loading in each row.

An example of this first structural level can be found below:

| BlockID | AbsTxID | Limb | IsNoTx | IsTimestamp | IsBlockHashHi | IsBlockHashLo | IsAddrHi | IsAddrLo | IsTxRLP |
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
|    1    |    1    | 0xab |   1    |       0     |       0       |       0       |     0    |     0    |    0    |
|    1    |    1    | 0xa  |   0    |       1     |       0       |       0       |     0    |     0    |    0    |
|    1    |    1    | 0xcc |   0    |       0     |       1       |       0       |     0    |     0    |    0    |
|    1    |    1    | 0xa  |   0    |       0     |       0       |       1       |     0    |     0    |    0    |
|    1    |    1    | 0xdd |   0    |       0     |       0       |       0       |     1    |     0    |    0    |
|    1    |    1    | 0xa  |   0    |       0     |       0       |       0       |     0    |     1    |    0    |
|    1    |    1    | 0xdd |   0    |       0     |       0       |       0       |     0    |     0    |    1    |
|    1    |    1    | 0xe  |   0    |       0     |       0       |       0       |     0    |     0    |    1    |
|    1    |    1    | 0xcf |   0    |       0     |       0       |       0       |     0    |     0    |    1    |
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
Note that the IsTxRLP column loads multiple RLP limbs for transaction with AbsTxID = 1. Once these are loaded,
we continue similarly with AbsTxID = 2.
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
|    1    |    2    | 0xff |   1    |       0     |       0       |       0       |     0    |     0    |    0    |
|   ...   |   ...   |  ... |   ...  |      ...    |      ...      |      ...      |    ...   |    ...   |   ...   |
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
When all the data for BlockID = 1 and its associated transactions has been loaded, we move to BlockID = 2
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
|    2    |    1    | 0xa  |   1    |       0     |       0       |       0       |     0    |     0    |    0    |
|   ...   |   ...   |  ... |  ...   |      ...    |      ...      |      ...      |    ...   |    ...   |   ...   |
|---------|---------|------|--------|-------------|---------------|---------------|----------|----------|---------|
And we continue similarly until all the blocks have been processed.
Note that IsNoTx, IsBlockHashHi, IsBlockHashLo, IsTimestamp, IsTxRLP, IsAddrHi, IsAddrLo are mutually exclusive,
and cannot be 1 at the same time.

We distinguish different types of segments inside our module, in three different layers:
1. The BlockId layers groups block data and transaction data belonging to that block. These rows will be
contiguous and have the same BlockId.
2. Each block has different transactions, and the data of which transaction will correspond to a contiguous
segment that has the same AbsTxID.
3. Each transaction has a segment in which its RLP data is loaded.
We need to be able to distinguish when each segment ends, for which we use the following columns:

FirstAbsTxIDBlock and LastAbsTxIDBlock are two columns that store the first and last AbsTxID inside the
corresponding block. These are constrained in two ways:
- via a projection query from the fetcher that computes block metadata out of the TXNDATA module.
- via constancy constraints that require FirstAbsTxIDBlock/LastAbsTxIDBlock to remain constant whenever
the BlockId remains constant.
Using the LastAbsTxIDBlock, we can tell when the block data ends, by checking that AbsTxId = LastAbsTxIDBlock
via a selector column SelectorLastTxBlock, with:
(SelectorLastTxBlock[i] = 1) iff (AbsTxId[i] = LastAbsTxIDBlock[i])
This explains how to check the end of the block layer.

For the transaction level, we need to distinguish when the corresponding RLP segment ends. For this purpose,
we use a EndOfRlpSegment column, which is constrained in two ways:
- via a projection query from the fetcher that processed RLP data from the RLPTXN module.
The projection inspects (abAbsTxID, AbsTxIDMax, limb, EndOfRlp) on filter IsTxRLP = 1
- via a global constraint that ensures that EndOfRlpSegment = 0 whenever IsTxRLP = 1.

Note that BlockId can only increase when EndOfRlpSegment = 1, which ensures that it is
not possible for leftover rlp limbs belonging to another transaction to end up in the next block.

If the projection query passes, we know that all RLP limbs are loaded at IsTxRLP = 1
for rows with the proper transaction number AbsTxId. AbsTxIdMax also gets enforced through this projection query.

In order to distinguish the relevant part of the module from the padding, we use an IsActive column, which
is 1 on every row that loads data and 0 otherwise.


The counters:
BlockID is constrained via a projection query on filter edc.IsNoTx = 1
it is also constrained with a local constraint, forcing it to start at value 1
it increases by 1 when isActive = 1, SelectorLastTxBlock = 1, EndOfRlpSegment = 1
and SelectorEndOfAllTx = 0

| Limb | NoBytes | UnalignedLimb | AlignedPow |

A different projection query ensures that (RelBlock, FirstAbsTxIDBlock, LastAbsTxIDBlock, TotalNoTxBlock) is
correctly fetched when IsAddrHi = 1 and separately, also when IsAddrLo = 1

The timestamp projection query on (RelBlock, Limb) ensures that when IsTimestamp = 1,
the limb contains the timestamp properly associated with that relative block number.

The AbsTxId counter starts from 1. When IsActive = 1, EndOfRlpSegment = 0 and SelectorEndOfAllTx = 0, the
AbsTxId counter is forced to remain constant. The reason to also ask that SelectorEndOfAllTx = 0, is because
the row after SelectorEndOfAllTx = 1 will have AbsTxId = 0 (enforce this)
*/

const (
	noBytesNoTxn        = 2
	noBytesTimestamp    = 4
	noBytesBlockHash    = 16
	noBytesSenderAddrHi = 4
	noBytesSenderAddrLo = 16

	// The number of meaningfull limbs of timestamp.
	nbTimestamplLimbs = common.NbLimbU32
	// The number of empty limbs of timestamp.
	nbTimestampEmpty = common.NbLimbU128 - nbTimestamplLimbs

	hashNum = 1 // the constant hashNum value needed as an input for padding and packing
)

// The ExecutionDataCollector contains columns that encode the computation and fetching of appropriate data
// from several arithmetization fetchers.
type ExecutionDataCollector struct {
	// BlockId ranges from 1 to the maximal number of blocks,
	// AbsTxID is the absolute transaction ID, which is unique among all blocks in the conflation
	// and also starts from 1.
	// AbsTxIDMax is the ID of the last transaction in the conflated batch.
	BlockID, AbsTxID, AbsTxIDMax ifaces.Column
	// The Limbs data.
	Limbs [common.NbLimbU128]ifaces.Column
	// The number of bytes in the limbs.
	NoBytes ifaces.Column
	// the total number of transactions inside the current block.
	TotalNoTxBlock ifaces.Column
	// indicator column, specifying when the module contains useful data
	IsActive ifaces.Column
	// indicator columns that light up as 1 when the corresponding value type is being loaded
	IsNoTx, IsTimestamp, IsBlockHashHi, IsBlockHashLo, IsAddrHi, IsAddrLo, IsTxRLP ifaces.Column
	// counter column, increases by 1 for every new active row. Not needed inside this module, but required
	// for invoking the padding and packing modules.
	Ct ifaces.Column
	// HashNum is a constant column only needed for invoking the padding and packing modules.
	HashNum ifaces.Column
	// The FirstAbsTxID/LastAbsTxIDBlock contain the first/last absolute transactions ID inside the current block.
	FirstAbsTxIDBlock, LastAbsTxIDBlock ifaces.Column
	// lights up as 1 on the row that contains the last RLP limb of the current transaction.
	EndOfRlpSegment ifaces.Column
	// a counter that computes the total number of bytes in all the previous rows, from the first to the current.
	TotalBytesCounter ifaces.Column
	// FinalTotalBytesCounter is a size-1 column that stores the last value of TotalBytesCounter for which the isActive filter is active.
	// In other words, FinalTotalBytesCounter contains the total number of bytes in the limbs of the ExecutionDataCollector module.
	FinalTotalBytesCounter ifaces.Column
	// Selector columns
	// SelectorBlockDiff[i]=1 if (edc.BlockId[i] = edc.BlockId[i+1]), used to enforce constancies when
	// inside a block segment
	SelectorBlockDiff        ifaces.Column
	ComputeSelectorBlockDiff wizard.ProverAction

	// SelectorLastTxBlock[i]=1 if (edc.AbsTxID[i]=edc.LastAbsTxIDBlock[i]), used to enforce constraints
	// to transition to the next block.
	SelectorLastTxBlock        ifaces.Column
	ComputeSelectorLastTxBlock wizard.ProverAction

	// SelectorEndOfAllTx[i]=1 if (edc.AbsTxID[i]=edc.AbsTxIDMax[i]), used to transition to the inactive
	// part of the module.
	SelectorEndOfAllTx        ifaces.Column
	ComputeSelectorEndOfAllTx wizard.ProverAction
	// SelectorAbsTxIDDiff[i]=1 if (edc.AbsTxID[i]=edc.AbsTxID[i+1]), used to enforce constant constraints inside a transaction segment.
	SelectorAbsTxIDDiff        ifaces.Column
	ComputeSelectorAbsTxIDDiff wizard.ProverAction

	// the following two columns are used to detect blocks that have zero user transactions
	// which is a special edge case that needs to be handled separately.
	SelectorBlockHasZeroUserTx        ifaces.Column
	ComputeSelectorBlockHasZeroUserTx wizard.ProverAction
}

// NewExecutionDataCollector instantiates an ExecutionDataCollector with unconstrained columns.
func NewExecutionDataCollector(comp *wizard.CompiledIOP, name string, size int) *ExecutionDataCollector {
	res := &ExecutionDataCollector{
		BlockID:                util.CreateCol(name, "BLOCK_ID", size, comp),
		AbsTxID:                util.CreateCol(name, "ABS_TX_ID", size, comp),
		AbsTxIDMax:             util.CreateCol(name, "ABS_TX_ID_MAX", size, comp),
		FirstAbsTxIDBlock:      util.CreateCol(name, "FIRST_ABS_TX_ID_BLOCK", size, comp),
		LastAbsTxIDBlock:       util.CreateCol(name, "LAST_ABS_TX_ID_BLOCK", size, comp),
		NoBytes:                util.CreateCol(name, "NO_BYTES", size, comp),
		TotalNoTxBlock:         util.CreateCol(name, "TOTAL_NO_TX_BLOCK", size, comp),
		IsActive:               util.CreateCol(name, "IS_ACTIVE", size, comp),
		IsNoTx:                 util.CreateCol(name, "IS_NO_TX", size, comp),
		IsBlockHashHi:          util.CreateCol(name, "IS_BLOCK_HASH_HI", size, comp),
		IsBlockHashLo:          util.CreateCol(name, "IS_BLOCK_HASH_LO", size, comp),
		IsTimestamp:            util.CreateCol(name, "IS_TIMESTAMP", size, comp),
		IsTxRLP:                util.CreateCol(name, "IS_TX_RLP", size, comp),
		IsAddrHi:               util.CreateCol(name, "IS_ADDR_HI", size, comp),
		IsAddrLo:               util.CreateCol(name, "IS_ADDR_LO", size, comp),
		Ct:                     util.CreateCol(name, "CT", size, comp),
		HashNum:                util.CreateCol(name, "HASH_NUM", size, comp),
		EndOfRlpSegment:        util.CreateCol(name, "END_OF_RLP_SEGMENT", size, comp),
		TotalBytesCounter:      util.CreateCol(name, "TOTAL_BYTES_COUNTER", size, comp),
		FinalTotalBytesCounter: util.CreateCol(name, "FINAL_TOTAL_BYTES_COUNTER", size, comp),
	}

	for i := range res.Limbs {
		res.Limbs[i] = util.CreateCol(name, fmt.Sprintf("LIMB_%d", i), size, comp)
	}

	return res
}

// GetSummarySize estimates a necessary upper bound on the ExecutionDataCollector columns
// we currently ignore the following modules btm *fetch.BlockTxnMetadata, bdc *fetch.BlockDataCols,
func GetSummarySize(td *arith.TxnData, rt *arith.RlpTxn) int {
	// number of transactions, block timestamp, blockhash + for every transaction, sender address + transaction RLP limbs
	size := td.Ct.Size()
	if size < rt.Limbs[0].Size() {
		size = rt.Limbs[0].Size()
	}
	return size
}

// DefineBlockIdCounterConstraints enforces that the BlockID starts from 1. BlockID can either increase by 1 or stay the same.
// Finally, BlockID is more finely constrained to only increase when edc.IsActive = 1, edc.SelectorLastTxBlock = 1,
// edc.EndOfRlpSegment = 1 and edc.SelectorEndOfAllTx = 0.
func DefineBlockIdCounterConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// BlockID starts from 1
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%v_COUNTER_START_LOCAL_CONSTRAINT", name, edc.BlockID.GetColID()),
		sym.Mul(
			edc.IsActive,
			sym.Sub(
				edc.BlockID,
				1, // blockIDs start from 1
			),
		),
	)

	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_COUNTER_STAYS_THE_SAME_OR_INCREASES_BY_1_GLOBAL_CONSTRAINT", name, edc.BlockID.GetColID()),
		sym.Mul(
			column.Shift(edc.IsActive, 1),
			sym.Mul(
				sym.Sub(
					column.Shift(edc.BlockID, 1),
					edc.BlockID,
					1), // increases by 1
				sym.Sub(
					column.Shift(edc.BlockID, 1),
					edc.BlockID, // or stays the same
				),
			),
		),
	)

	// this constraint might not be needed anymore
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_COUNTER_INCREASES_BY_1_GLOBAL_CONSTRAINT", name, edc.BlockID.GetColID()),
		sym.Mul(
			edc.IsActive,
			edc.SelectorLastTxBlock, // the last transaction inside the block
			edc.EndOfRlpSegment,
			sym.Sub(
				1,
				edc.SelectorEndOfAllTx,
			), // not at the end of all blocks
			sym.Sub(
				column.Shift(edc.BlockID, 1),
				edc.BlockID,
				1,
			),
		),
	)
}

// DefineAbsTxIdCounterConstraints concerns AbsTxId, which starts from 1 and subsequently, can either stay the same or increase by 1.
// AbsTxId will increase when edc.IsActive = 1 and edc.EndOfRlpSegment = 1 and edc.SelectorEndOfAllTx = 0.
// AbsTxId remains the same when edc.IsActive = 0 and edc.EndOfRlpSegment = 0 and edc.SelectorEndOfAllTx = 0.
func DefineAbsTxIdCounterConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// Counter constraints: first, the AbsTxID counter starts from 1
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%v_COUNTER_START_LOCAL_CONSTRAINT", name, edc.AbsTxID.GetColID()),
		sym.Mul(
			edc.IsActive,
			sym.Sub(
				edc.AbsTxID,
				1, // absTxId starts from 1
			),
		),
	)
	// edc.AbsTxID can only stay the same or increase exactly by 1
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_COUNTER_STAYS_THE_SAME_OR_INCREASES_BY_1_GLOBAL_CONSTRAINT", name, edc.AbsTxID.GetColID()),
		sym.Mul(
			column.Shift(edc.IsActive, 1),
			sym.Mul(
				sym.Sub(
					column.Shift(edc.AbsTxID, 1),
					edc.AbsTxID,
					1), // increases by 1
				sym.Sub(
					column.Shift(edc.AbsTxID, 1),
					edc.AbsTxID, // or stays the same
				),
			),
		),
	)
	// fine-grained control over when edc.AbsTxID increases
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_COUNTER_INCREASES_GLOBAL_CONSTRAINT", name, edc.AbsTxID.GetColID()),
		sym.Mul(
			edc.IsActive,
			edc.EndOfRlpSegment, // at the end of the RLP segment, we move to the next transaction
			sym.Sub(
				1,
				edc.SelectorEndOfAllTx,
			), // not at the end of all blocks
			sym.Sub(
				column.Shift(edc.AbsTxID, 1),
				edc.AbsTxID,
				1,
			),
		),
	)

	// fine-grained constraint over when edc.AbsTxID remains constant inside a transaction segment
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_CONSTANT_INSIDE_TRANSACTION_SEGMENTS", name, edc.AbsTxID.GetColID()),
		sym.Mul(
			edc.IsActive,
			sym.Sub(
				1,
				edc.EndOfRlpSegment,
			), // while not at the end of the RLP segment, the transaction number remains constant
			sym.Sub(
				1,
				edc.SelectorEndOfAllTx,
			), // not at the end of all blocks
			sym.Sub(
				1,
				edc.SelectorBlockHasZeroUserTx, // we do not have RLP segments when the block has no transactions
			),
			sym.Sub(
				column.Shift(edc.AbsTxID, 1),
				edc.AbsTxID,
			),
		),
	)
}

// DefineCtCounterConstraints constrains that edc.Ct starts from 0, and then remains the same or increases by 1.
func DefineCtCounterConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// First, the counter starts from 0.
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%v_COUNTER_START_LOCAL_CONSTRAINT", name, edc.Ct.GetColID()),
		ifaces.ColumnAsVariable(edc.Ct),
	)
	// Secondly, the counter increases by 1 every time.
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "COUNTER_GLOBAL"),
		sym.Mul(
			edc.IsActive,
			sym.Sub(edc.Ct,
				column.Shift(edc.Ct, -1),
				1,
			),
		),
	)
}

// DefineHashNumConstraints requires that edc.HashNum is constantly equal to hashNum, currently hardcoded to 1.
func DefineHashNumConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "HASH_NUM_GLOBAL"),
		sym.Mul(
			edc.IsActive,
			sym.Sub(edc.HashNum,
				hashNum,
			),
		),
	)
}

// DefineIndicatorOrder constrains the order of load operations.
func DefineIndicatorOrder(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// the module starts with IsNoTx[0] = 1
	comp.InsertLocal(0,
		ifaces.QueryIDf("%s_START_WITH_IS_NO_TX", name),
		sym.Sub(
			edc.IsNoTx,
			1,
		),
	)

	// IsNbTx -> isTimestamp
	// From IsNbTx[i]=1, we can only transition to isTimestamp[i+1]=1 on the next row.
	// Conversely, we have that isTimestamp[i+1]=1 implies that IsNbTx[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_NB_TX_TO_IS_TIMESTAMP", name),
		sym.Sub(
			column.Shift(edc.IsNoTx, -1),
			edc.IsTimestamp,
		),
	)

	// isTimestamp -> isBlockHashHi
	// From IsTimestamp[i]=1, we can only transition to IsBlockHashHi[i+1]=1 on the next row.
	// Conversely, we have that IsBlockHashHi[i+1]=1 implies that IsTimestamp[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_NB_TX_TO_IS_BLOCKHASH_HI", name),
		sym.Sub(
			column.Shift(edc.IsTimestamp, -1),
			edc.IsBlockHashHi,
		),
	)

	// isBlockHashHi->isBlockHashLo
	// From isBlockHashHi[i]=1, we can only transition to isBlockHashLo[i+1]=1 on the next row.
	// Conversely, we have that isBlockHashLo[i+1]=1 implies that isBlockHashHi[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_BLOCKHASH_HI_TO_BLOCKHASH_LO", name),
		sym.Sub(
			column.Shift(edc.IsBlockHashHi, -1),
			edc.IsBlockHashLo,
		),
	)

	// isBlockHashLO->IsAddrHi
	// From IsBlockHashLo[i]=1, we can only transition to IsAddrHi[i+1]=1 on the next row.
	// The converse direction does not necessarily hold,
	// we do NOT have that IsAddrHi[i+1]=1 implies that IsBlockHashLo[i]=1.
	// special case: if the block has zero user transactions, we transition directly from IsBlockHashLo to IsNoTx
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_BLOCKHASH_LO_TO_IS_ADDR_HI", name),
		sym.Mul(
			column.Shift(edc.IsBlockHashLo, -1), // this constraint says if IsBlockHashLo[i-1] =1 then IsAddrHi[i] = 1
			sym.Sub(
				column.Shift(edc.IsBlockHashLo, -1),
				edc.IsAddrHi,
			),
			// add the special case indicator
			// the constraints above should only be active when the current block has user transactions
			// and is not empty
			sym.Sub(
				1,
				column.Shift(edc.SelectorBlockHasZeroUserTx, -4), // if this selector is 0, the enclosing term becomes 1
			),
		),
	)

	// IsAddrHi -> IsAddrLo
	// From IsAddrHi[i]=1, we can only transition to IsAddrLo[i+1]=1 on the next row.
	// Conversely, we have that IsAddrLo[i+1]=1 implies that IsAddrHi[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_ADDR_HI_TO_IS_ADDR_LO", name),
		sym.Sub(
			column.Shift(edc.IsAddrHi, -1),
			edc.IsAddrLo,
		),
	)

	// IsAddrLo -> IsTxRLP
	// From IsAddrLo[i]=1, we can only transition to IsTxRLP[i+1]=1 on the next row.
	// The converse direction does not necessarily hold,
	// we do NOT have that IsTxRLP[i+1]=1 implies that IsAddrLo[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_ADDR_LO_TO_IS_TX_RLP", name),
		sym.Mul(
			column.Shift(edc.IsAddrLo, -1),
			sym.Sub(
				column.Shift(edc.IsAddrLo, -1),
				edc.IsTxRLP,
			),
		),
	)

	// IsTxRLP -> IsTxRLP while inside the transaction RLP segment
	// If IsTxRLP[i]=1 and EndOfRlpSegment[i]=0, we can transition to IsTxRLP[i+1]=1 on the next row.
	// we do NOT have that IsTxRLP[i+1]=1 implies that IsTxRLP[i]=1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_TX_RLP_REMAINS_1_INSIDE_RLP_SEGMENT", name),
		sym.Mul(
			sym.Sub(1,
				edc.EndOfRlpSegment,
			), // we are inside an RLP segment
			edc.IsTxRLP, // if IsTxRLP is 1 then on the next row IsTxRLP will be 1 if we are inside the block
			sym.Sub(
				1,
				column.Shift(edc.IsTxRLP, 1), // IsTxRLP=1 on the next row
			),
		),
	)

	// IsTxRLP -> IsNoTx, moving to the next block.
	// If IsTxRLP[i]=1 and SelectorLastTxBlock[i]=1 and EndOfRlpSegment[i]=1 and SelectorEndOfAllTx[i]=0,
	// we can transition to IsNoTx[i+1]=1 on the next row.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_TX_RLP_TO_NEXT_BLOCK", name),
		sym.Mul(
			edc.SelectorLastTxBlock, // it is the last transaction in the block
			edc.EndOfRlpSegment,     // it is the end of the RLP segment
			sym.Sub(
				1,
				edc.SelectorEndOfAllTx, // but it is not the end of all transactions
			),
			edc.IsTxRLP, // do not comment this row, due to the dependency between EndOfRlpSegment and IsTxRLP.
			// (if IsTxRLP is 1 then on the next row IsNoTx will be 1 since we move on to the next block)
			sym.Sub(
				edc.IsTxRLP,
				column.Shift(edc.IsNoTx, 1),
			),
		),
	)

	// IsTxRLP -> IS_ADDR_HI, moving to the next transaction segment, and load the next sender address.
	// If IsTxRLP[i]=1 and SelectorLastTxBlock[i]=0 and EndOfRlpSegment[i]=1,
	// we can transition to IsAddrHi[i+1]=1 on the next row.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_TX_RLP_TO_IS_ADDR_HI_GLOBAL_CONSTRAINT", name),
		sym.Mul(
			sym.Sub(
				1,
				edc.SelectorLastTxBlock, // not the end of the transactions in the current block
			),
			edc.EndOfRlpSegment, // end of RLP segment for the current transaction
			edc.IsTxRLP,         // do not comment this row, due to the dependency between EndOfRlpSegment and IsTxRLP.
			// we are inside the RLP segment
			sym.Sub(
				1,
				column.Shift(edc.IsAddrHi, 1), // load the sender address of the next transaction, since we are still inside the block
			),
		),
	)

	// from IsTxRLP=1, we can directly transition to the inactive part.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_TX_RLP_DIRECTLY_TO_IS_INACTIVE_GLOBAL_CONSTRAINT", name),
		sym.Mul(
			edc.SelectorEndOfAllTx, // 1 at the very last transaction, in the last block
			edc.EndOfRlpSegment,    // 1 at the end of RLP segment for the current transaction
			edc.IsTxRLP,            // 1 inside the RLP segment
			// either of the ones below must be 0
			column.Shift(edc.IsActive, 1), // all the above forces isActive to be 0 on the next position
			// or we have that
			column.Shift(edc.TotalNoTxBlock, 1), // the next block is empty
		),
	)

	// special case: if the block has zero user transactions, we transition directly from IsBlockHashLo to IsNoTx
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_BLOCKHASH_LO_TO_IS_NO_TX_FOR_EMPTY_BLOCKS", name),
		sym.Mul(
			edc.IsActive,                        // constraint below only valid if we are still in the active part of the collector
			column.Shift(edc.IsBlockHashLo, -1), // this constraint says if IsBlockHashLo[i-1] = 1 then ....
			sym.Sub(
				// this term enforces that IsBlockHashLo[i-1] = 1 must be equal to IsNoTx = 1 (when the other outside conditions are true)
				column.Shift(edc.IsBlockHashLo, -1),
				edc.IsNoTx,
			),
			// add the special case indicator
			// the constraints above should only be active when the current block has NO user transactions
			// and is empty
			column.Shift(edc.SelectorBlockHasZeroUserTx, -4), // if this selector is 1, then there are no user transactions in the block
		),
	)
}

// DefineIndicatorConverseOrder constrains the converse order of load operations.
// These constraints might not be needed in order for the module to be secure.
func DefineIndicatorConverseOrder(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// IsNoTx[i+1]=1 -> IsTxRLP[i]=1
	// The converse has additional conditions and is treated above.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_CONVERSE_IS_TX_RLP_TO_IS_NO_TX_NEXT_BLOCK", name),
		sym.Mul(
			column.Shift(edc.IsNoTx, 1),
			sym.Sub(
				edc.IsTxRLP,
				column.Shift(edc.IsNoTx, 1),
			), // either this term is 0, or we are in the next edge case
			sym.Mul(
				column.Shift(edc.SelectorBlockHasZeroUserTx, -3), // if this selector is 1, then there are no user transactions in the block
				sym.Sub( // IsBlockHashLo is equal to isNoTx[i+1] because we had an empty block
					edc.IsBlockHashLo,
					column.Shift(edc.IsNoTx, 1),
				),
			),
		),
	)

	// IsTxRLP[i+1]=1 -> (IsTxRLP[i]=1 or IsAddrLo[i]=1)
	// The converse has additional conditions and is treated above.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_CONVERSE_IS_TX_RLP/IS_ADDR_LO_TO_IS_TX_RLP", name),
		sym.Mul(
			column.Shift(edc.IsTxRLP, 1),
			sym.Sub(
				column.Shift(edc.IsTxRLP, 1),
				edc.IsTxRLP,
				edc.IsAddrLo,
			),
		),
	)

	// IsAddrHi[i+1]=1 -> (IsTxRLP[i]=1 or IsBlockHashLo[i]=1)
	// The converse has additional conditions and is treated above.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_CONVERSE_IS_ADDR_HI/IS_TX_RLP_TO_IS_BLOCKHASH_LO", name),
		sym.Mul(
			column.Shift(edc.IsAddrHi, 1),
			sym.Sub(
				column.Shift(edc.IsAddrHi, 1),
				edc.IsTxRLP,
				edc.IsBlockHashLo,
			),
		),
	)
	// isActive[i+1]=0 and isActive[i]=1 -> (either IsTxRLP[i]=1 or isBlockHashLo[i]=1)
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_CONVERSE_IS_ACTIVE_TO_IS_TX_RLP", name),
		sym.Mul(
			sym.Sub(1, edc.IsActive),
			column.Shift(edc.IsActive, -1),
			sym.Sub(
				1,
				column.Shift(edc.IsTxRLP, -1),
			),
			sym.Sub( // or we were in an ampty block and transitioned directly from IsBlockHashLo
				1,
				column.Shift(edc.IsBlockHashLo, -1),
			),
		),
	)
}

// DefineIndicatorExclusion enforces that indicators for different load operations cannot light up simultaneously.
func DefineIndicatorExclusion(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// IsNoTx + IsBlockHashHi + IsBlockHashLo + IsTimestamp + IsTxRLP + IsAddr = 1
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_INDICATOR_EXCLUSION_GLOBAL_CONSTRAINT", name),
		sym.Sub(
			edc.IsActive,
			sym.Add(
				edc.IsNoTx,
				edc.IsBlockHashHi,
				edc.IsBlockHashLo,
				edc.IsTimestamp,
				edc.IsTxRLP,
				edc.IsAddrHi,
				edc.IsAddrLo,
			),
		),
	)
}

// DefineNumberOfBytesConstraints defines the number of bytes loaded for each operation type.
// The RLP bytes are checked separately, in the ProjectionQueries function.
func DefineNumberOfBytesConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// noOfBytes isNoTx
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_NO_TX_NO_BYTES", name),
		sym.Mul(
			edc.IsNoTx,
			sym.Sub(
				edc.NoBytes,
				noBytesNoTxn, // 2 bytes when loading NO_TX
			),
		),
	)

	// noOfBytes isTimestamp
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_TIMESTAMP", name),
		sym.Mul(
			edc.IsTimestamp,
			sym.Sub(
				edc.NoBytes,
				noBytesTimestamp, // 6 bytes when loading a TIMESTAMP
			),
		),
	)

	// noOfBytes isBlockhashHi
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_BLOCKHASH_HI_NO_BYTES", name),
		sym.Mul(
			edc.IsBlockHashHi,
			sym.Sub(
				edc.NoBytes,
				noBytesBlockHash, // 16 bytes when loading a blockhashHi
			),
		),
	)
	// noOfBytes isBlockhashLo
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_BLOCKHASH_LO_NO_BYTES", name),
		sym.Mul(
			edc.IsBlockHashLo,
			sym.Sub(
				edc.NoBytes,
				noBytesBlockHash, // 16 bytes when loading a blockhashLo
			),
		),
	)

	// noOfBytes isAddrHi
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_ADDR_HI_NO_BYTES", name),
		sym.Mul(
			edc.IsAddrHi,
			sym.Sub(
				edc.NoBytes,
				noBytesSenderAddrHi, // 4 bytes when loading a isAddrHi
			),
		),
	)

	// noOfBytes isAddrLo
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_IS_ADDR_LO_NO_BYTES", name),
		sym.Mul(
			edc.IsAddrLo,
			sym.Sub(
				edc.NoBytes,
				noBytesSenderAddrLo, // 16 bytes when loading a isAddrLo
			),
		),
	)

	// We must also enforce noOfBytes limbs in the RLP transaction data,
	// and this will be checked with a projection query in the ProjectionQueries function.
}

// To do: To be synchronized with Bogdan/Alex before we merge
// ProjectionQueries computes projection queries to each arithmetization fetcher:
// fetch.BlockDataFetcher, fetch.BlockTxnMetadata, fetch.TxnDataFetcher and fetch.RlpTxnFetcher.
func ProjectionQueries(comp *wizard.CompiledIOP,
	edc *ExecutionDataCollector,
	name string,
	timestamps *fetch.BlockDataFetcher,
	metadata fetch.BlockTxnMetadata,
	txnData fetch.TxnDataFetcher,
	rlp fetch.RlpTxnFetcher) {

	// Prepare the projection query to the BlockData fetcher
	// compute the fetcher table, directly tied to the arithmetization.
	metadataTable := []ifaces.Column{
		metadata.BlockID,
		metadata.TotalNoTxnBlock,
		metadata.FirstAbsTxId,
		metadata.LastAbsTxId,
	}
	// compute the ExecutionDataCollector table.
	edcMetadataTable := []ifaces.Column{
		edc.BlockID,
		edc.TotalNoTxBlock,
		edc.FirstAbsTxIDBlock,
		edc.LastAbsTxIDBlock,
	}

	comp.InsertProjection(ifaces.QueryIDf("%s_BLOCK_METADATA_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: edcMetadataTable,
			ColumnB: metadataTable,
			FilterA: edc.IsNoTx, // We filter on rows where the blockdata is loaded.
			FilterB: metadata.FilterFetched,
		},
	)
	// Because we filtered on edc.IsNoTx=1, we also ensure that FirstAbsTxIDBlock and LastAbsTxIDBlock
	// remain constant in the DefineConstantConstraints function.
	// we do not need to also check the constancy of TotalNoTxBlock, as it is only used when IsNoTx=1

	// Prepare the projection query to the BlockData fetcher, but concerning timestamps
	// compute the fetcher table, directly tied to the arithmetization.
	timestampTable := make([]ifaces.Column, 0, nbTimestamplLimbs+1)
	timestampTable = append(timestampTable, timestamps.Data[nbTimestampEmpty:]...) // Only the last 32 bits stores the timestamp.
	timestampTable = append(timestampTable, timestamps.RelBlock)

	// compute the ExecutionDataCollector table.
	edcTimestamps := make([]ifaces.Column, 0, nbTimestamplLimbs+1)
	edcTimestamps = append(edcTimestamps, edc.Limbs[:nbTimestamplLimbs]...) // Only the first 32 bits stores the timestamp.
	edcTimestamps = append(edcTimestamps, edc.BlockID)

	comp.InsertProjection(ifaces.QueryIDf("%s_TIMESTAMP_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: edcTimestamps,
			ColumnB: timestampTable,
			FilterA: edc.IsTimestamp, // filter on IsTimestamp=1
			FilterB: timestamps.FilterFetched,
		},
	)

	// Check that timestamp's unsused limbs are zeroes.
	for i := 0; i < nbTimestampEmpty; i++ {
		comp.InsertGlobal(0, ifaces.QueryIDf("%s_TIMESTAMP_FETCHER_LIMBS_ZEROES_%d", name, i),
			sym.Mul(timestamps.FilterFetched, timestamps.Data[i]),
		)

		comp.InsertGlobal(0, ifaces.QueryIDf("%s_TIMESTAMP_LIMBS_ZEROES_%d", name, i),
			sym.Mul(edc.IsTimestamp, edc.Limbs[nbTimestamplLimbs+i]),
		)
	}

	// Prepare a projection query to the TxnData fetcher, to check the Hi part of the sender address.
	// compute the fetcher table, directly tied to the arithmetization.
	txnDataTableHi := make([]ifaces.Column, 0, common.NbLimbU32+2)
	txnDataTableHi = append(txnDataTableHi, txnData.From[:common.NbLimbU32]...) // checks that the Hi part of the sender address is fetched correctly.
	txnDataTableHi = append(txnDataTableHi, txnData.RelBlock, txnData.AbsTxNum)

	// compute the ExecutionDataCollector table.
	edcTxnSenderAddressTableHi := make([]ifaces.Column, 0, common.NbLimbU32+2)
	edcTxnSenderAddressTableHi = append(edcTxnSenderAddressTableHi, edc.Limbs[:common.NbLimbU32]...)
	edcTxnSenderAddressTableHi = append(edcTxnSenderAddressTableHi, edc.BlockID, edc.AbsTxID)

	comp.InsertProjection(ifaces.QueryIDf("%s_SENDER_ADDRESS_HI_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: edcTxnSenderAddressTableHi,
			ColumnB: txnDataTableHi,
			FilterA: edc.IsAddrHi, // filter on IsAddrHi=1
			FilterB: txnData.FilterFetched,
		},
	)

	// Check that the unused limbs of the sender address are zeroes.
	for i := common.NbLimbU32; i < common.NbLimbU128; i++ {
		comp.InsertGlobal(0, ifaces.QueryIDf("%s_SENDER_ADDRESS_HI_LIMBS_ZEROES_%d", name, i),
			sym.Mul(edc.IsAddrHi, edc.Limbs[i]),
		)
	}

	// Prepare the projection query to the TxnData fetcher, to check the Lo part of the sender address.
	// compute the fetcher table, directly tied to the arithmetization.
	txnDataTableLo := make([]ifaces.Column, 0, common.NbLimbU128+2)
	txnDataTableLo = append(txnDataTableLo, txnData.From[common.NbLimbU32:]...) // checks that the Lo part of the sender address is fetched correctly.
	txnDataTableLo = append(txnDataTableLo, txnData.RelBlock, txnData.AbsTxNum)

	// compute the ExecutionDataCollector table.
	edcTxnSenderAddressTableLo := make([]ifaces.Column, 0, common.NbLimbU128+2)
	edcTxnSenderAddressTableLo = append(edcTxnSenderAddressTableLo, edc.Limbs[:]...)
	edcTxnSenderAddressTableLo = append(edcTxnSenderAddressTableLo, edc.BlockID, edc.AbsTxID)

	comp.InsertProjection(ifaces.QueryIDf("%s_SENDER_ADDRESS_LO_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: edcTxnSenderAddressTableLo,
			ColumnB: txnDataTableLo,
			FilterA: edc.IsAddrLo, // filter on IsAddrLo=1
			FilterB: txnData.FilterFetched,
		},
	)

	// Prepare the projection query to the RlpTxn fetcher, to check:
	// AbsTxNum, AbsTxNumMax, Limb, NBytes and EndOfRlpSegment.
	// first compute the fetcher table, directly tied to the arithmetization.
	rlpDataTable := append(
		rlp.Limbs[:],
		rlp.AbsTxNum,
		rlp.AbsTxNumMax,
		rlp.NBytes,
		rlp.EndOfRlpSegment,
	)
	// compute the ExecutionDataCollector table.
	edcRlpDataTable := append(
		edc.Limbs[:],        // Check correctness of the limbs.
		edc.AbsTxID,         // Check correctness of the AbsTxID.
		edc.AbsTxIDMax,      // The fact that it is constant is enforced in DefineConstantConstraints.
		edc.NoBytes,         // Check correctness of the number of bytes.
		edc.EndOfRlpSegment, // This constrains EndOfRlpSegment on edc.IsTx.RLP = 1, but we still need to constrain it elsewhere
		// EndOfRlpSegment is also constrained in DefineSelectorConstraints, which requires that EndOfRlpSegment=0 when AbsTxID is constant.
		// EndOfRlpSegment is also constrained in DefineZeroizationConstraints, with respect to IsActive.
	)

	comp.InsertProjection(ifaces.QueryIDf("%s_RLP_LIMB_DATA_PROJECTION", name),
		query.ProjectionInput{
			ColumnA: edcRlpDataTable,
			ColumnB: rlpDataTable,
			FilterA: edc.IsTxRLP, // filter on IsTxRLP=1
			FilterB: rlp.FilterFetched,
		},
	)
}

// LookupQueries computes lookup queries to the BlockTxnMetadata arithmetization fetcher:
func LookupQueries(comp *wizard.CompiledIOP,
	edc *ExecutionDataCollector,
	name string,
	metadata fetch.BlockTxnMetadata,
) {

	metadataTable := []ifaces.Column{
		metadata.BlockID,
		metadata.TotalNoTxnBlock,
		metadata.FirstAbsTxId,
		metadata.LastAbsTxId,
	}
	// compute the ExecutionDataCollector table.
	edcMetadataTable := []ifaces.Column{
		edc.BlockID,
		edc.TotalNoTxBlock,
		edc.FirstAbsTxIDBlock,
		edc.LastAbsTxIDBlock,
	}

	comp.InsertInclusionDoubleConditional(0,
		ifaces.QueryIDf("%s_BLOCK_METADATA_DOUBLE_CONDITIONAL_LOOKUP", name),
		metadataTable,    // including table
		edcMetadataTable, // included table
		metadata.FilterFetched,
		edc.IsTxRLP,
	)
}

// EnforceZeroOnInactiveFilter is a generic helper function that enforces that targetCol is 0 when filterExpr = 0.
func EnforceZeroOnInactiveFilter(comp *wizard.CompiledIOP, filterExpr *sym.Expression, targetCol ifaces.Column, name, subname string) {
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_%s_%v_IS_ZERO_WHEN_INACTIVE_OR_NO_PADDING_GLOBAL_CONSTRAINT", name, subname, targetCol.GetColID()),
		sym.Sub(
			targetCol,
			sym.Mul(
				targetCol,
				filterExpr,
			),
		),
	)
}

// DefineSelectorConstraints constrains the selectors, but also the EndOfRlpSegment column.
func DefineSelectorConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// We first compute the prover actions
	edc.SelectorLastTxBlock, edc.ComputeSelectorLastTxBlock = dedicated.IsZero(
		comp,
		sym.Sub(
			edc.AbsTxID,
			edc.LastAbsTxIDBlock,
		),
	).GetColumnAndProverAction()

	edc.SelectorEndOfAllTx, edc.ComputeSelectorEndOfAllTx = dedicated.IsZero(
		comp,
		sym.Sub(
			edc.AbsTxID,
			edc.AbsTxIDMax,
		),
	).GetColumnAndProverAction()

	edc.SelectorBlockDiff, edc.ComputeSelectorBlockDiff = dedicated.IsZero(
		comp,
		sym.Sub(
			edc.BlockID,
			column.Shift(edc.BlockID, 1),
		),
	).GetColumnAndProverAction()

	edc.SelectorAbsTxIDDiff, edc.ComputeSelectorAbsTxIDDiff = dedicated.IsZero(
		comp,
		sym.Sub(
			edc.AbsTxID,
			column.Shift(edc.AbsTxID, 1),
		),
	).GetColumnAndProverAction()

	edc.SelectorBlockHasZeroUserTx, edc.ComputeSelectorBlockHasZeroUserTx = dedicated.IsZero(
		comp,
		ifaces.ColumnAsVariable(edc.TotalNoTxBlock),
	).GetColumnAndProverAction()

	// edc.EndOfRlpSegment is partially constrained in the projection queries, on areas where edc.IsTxRLP = 1
	// it is also constrained in DefineZeroizationConstraints.
	// here we require that when edc.IsTxRLP = 0, we have EndOfRlpSegment = 0
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_END_OF_RLP_SEGMENT_ZEROIZATION", name),
		sym.Mul(
			sym.Sub(
				1,
				edc.IsTxRLP,
			),
			ifaces.ColumnAsVariable(edc.EndOfRlpSegment),
		),
	)

	// Recall that edc.EndOfRlpSegment is partially constrained in the projection queries, on areas where edc.IsTxRLP = 1
	// it is also constrained in DefineZeroizationConstraints.
	// Here we ask that whenever the AbsTxID is constant, EndOfRlpSegment cannot light up (we are inside the same transaction segment).
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_END_OF_RLP_SEGMENT_IS_0_WHENEVER_TX_ID_IS_CONSTANT", name),
		sym.Mul(
			edc.SelectorAbsTxIDDiff,                      // Recall that (SelectorAbsTxIDDiff = 1) iff (AbsTxID[i]=AbsTxID[i+1])
			ifaces.ColumnAsVariable(edc.EndOfRlpSegment), // must be 0 in this case
		),
	)

}

// DefineTotalBytesCounterConstraints enforces that edc.TotalBytesCounter[0] = edc.NoBytes[0] and
// edc.TotalBytesCounter[i+1]=edc.TotalBytesCounter[i]+edc.NoBytes[i+1] for i>=0
// It also creates an accessor and constrains the size-1 column edc.FinalTotalBytesCounter to contain
// the last value of TotalBytesCounter for which the filter IsActive = 1.
func DefineTotalBytesCounterConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%v_TOTAL_BYTES_COUNTER_START_LOCAL_CONSTRAINT", name, edc.TotalBytesCounter.GetColID()),
		sym.Sub(
			edc.TotalBytesCounter,
			edc.NoBytes, // the first value of the total bytes counter must be the number of bytes on the first row.
		),
	)

	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%v_TOTAL_BYTES_COUNTER_GLOBAL_CONSTRAINT", name, edc.TotalBytesCounter.GetColID()),
		sym.Mul(
			edc.IsActive, // Here, we only consider the active part. On the inactive part, edc.TotalBytesCounter is forced to be zero in DefineZeroizationConstraints.
			sym.Sub(
				edc.TotalBytesCounter,
				edc.NoBytes,
				column.Shift(edc.TotalBytesCounter, -1), // the TotalBytes counter increases appropriately.
			),
		),
	)

	// enforce that FinalTotalBytesCounter contains the last value of TotalBytesCounter on the active part.
	util.CheckLastELemConsistency(comp, edc.IsActive, edc.TotalBytesCounter, edc.FinalTotalBytesCounter, name)
	commonconstraints.MustBeConstant(comp, edc.FinalTotalBytesCounter)
}

// DefineCounterConstraints enforces counter constraints for BlockId, AbsTxId and Ct.
func DefineCounterConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	DefineBlockIdCounterConstraints(comp, edc, name)
	DefineAbsTxIdCounterConstraints(comp, edc, name)
	DefineCtCounterConstraints(comp, edc, name)
	DefineTotalBytesCounterConstraints(comp, edc, name)
}

// DefineZeroizationConstraints enforces that multiple columns are zero when the IsActive filter is zero.
func DefineZeroizationConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// enforce zero fields when isActive is not set to 1
	var emptyWhenInactive = append(
		edc.Limbs[:],
		edc.BlockID,
		edc.AbsTxID,
		edc.AbsTxIDMax,
		edc.Ct,
		edc.TotalNoTxBlock,
		edc.IsNoTx,
		edc.IsBlockHashHi,
		edc.IsBlockHashLo,
		edc.IsTimestamp,
		edc.IsTxRLP,
		edc.IsAddrHi,
		edc.IsAddrLo,
		edc.FirstAbsTxIDBlock,
		edc.LastAbsTxIDBlock,
		edc.EndOfRlpSegment,
		edc.NoBytes,
		edc.TotalBytesCounter,
		// exclude edc.HashNum, as it is a fully constant column
	)

	for _, col := range emptyWhenInactive {
		// if isActive = 0 then the column becomes 0
		EnforceZeroOnInactiveFilter(comp, ifaces.ColumnAsVariable(edc.IsActive), col, name, "IS_ACTIVE")
	}
}

// DefineIndicatorsMustBeBinary enforces that various indicator columns are binary.
func DefineIndicatorsMustBeBinary(comp *wizard.CompiledIOP, edc *ExecutionDataCollector) {
	util.MustBeBinary(comp, edc.IsActive)
	util.MustBeBinary(comp, edc.IsNoTx)
	util.MustBeBinary(comp, edc.IsBlockHashHi)
	util.MustBeBinary(comp, edc.IsBlockHashLo)
	util.MustBeBinary(comp, edc.IsTimestamp)
	util.MustBeBinary(comp, edc.IsAddrHi)
	util.MustBeBinary(comp, edc.IsAddrLo)
	util.MustBeBinary(comp, edc.IsTxRLP)
}

// DefineConstantConstraints requires that FirstAbsTxIDBlock/LastAbsTxIDBlock remain constant inside the block.
// And that AbsTxIDMax is constant on the active part of the module.
func DefineConstantConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// in order for SelectorLastTxBlock to be constrained properly, the values of FirstAbsTxIDBlock and LastAbsTxIDBlock
	// must be constant for the entire segment defined by the block (otherwise SelectorLastTxBlock is meaningless).
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_%v_CONSTANT_FIRST_ABS_TX_ID_BLOCK_INSIDE_THE_BLOCK_SEGMENT", name, edc.FirstAbsTxIDBlock.GetColID()),
		sym.Mul(
			edc.SelectorBlockDiff, // 1 if edc.BlockId[i] = edc.BlockId[i+1]
			sym.Sub(
				edc.FirstAbsTxIDBlock,
				column.Shift(edc.FirstAbsTxIDBlock, 1),
			),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_%v_CONSTANT_LAST_ABS_TX_ID_BLOCK_INSIDE_THE_BLOCK_SEGMENT", name, edc.LastAbsTxIDBlock.GetColID()),
		sym.Mul(
			edc.SelectorBlockDiff, // 1 if edc.BlockId[i] = edc.BlockId[i+1]
			sym.Sub(
				edc.LastAbsTxIDBlock,
				column.Shift(edc.LastAbsTxIDBlock, 1),
			),
		),
	)
	// we do not contrain that FirstAbsTxIDBlock/LastAbsTxIDBlock increases only by 1, this is
	// constrained in the corresponding fetcher and enforced by the projection query (in conjunction
	// with the constancy property)
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_%v_CONSTANT_ABS_TX_ID_MAX", name, edc.AbsTxIDMax.GetColID()),
		sym.Mul(
			column.Shift(edc.IsActive, 1),
			sym.Sub(
				edc.AbsTxIDMax,
				column.Shift(edc.AbsTxIDMax, 1),
			),
		),
	)
}

// DefineLimbConsistencyConstraints makes sure that limb values are correct for the total number of transactions.
func DefineLimbConsistencyConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// The values is contained in the first limb - left aligned 16 bits.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%s_UNALIGNED_LIMB_AND_TOTAL_NO_TX", name),
		sym.Mul(
			edc.IsNoTx,
			sym.Sub(
				edc.Limbs[0],
				edc.TotalNoTxBlock,
			),
		),
	)

	// The rest of the limbs are empty - zeroes.
	for i := 1; i < len(edc.Limbs); i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%s_UNALIGNED_LIMB_AND_TOTAL_NO_TX_ZEROES_%d", name, i),
			sym.Mul(edc.IsNoTx, edc.Limbs[i]),
		)
	}
}

// DefineIsActiveConstraints requires that IsActive has the proper shape, never transitioning from 0 to 1.
// the fact that IsActive is binary is enforced in DefineIndicatorsMustBeBinary.
func DefineIsActiveConstraints(comp *wizard.CompiledIOP, edc *ExecutionDataCollector, name string) {
	// we require that isActive is binary in DefineIndicatorsMustBeBinary
	// require that the isActive filter only contains 1s followed by 0s
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%s_IS_ACTIVE_CONSTRAINT_NO_0_TO_1", name),
		sym.Sub(
			edc.IsActive,
			sym.Mul(
				column.Shift(edc.IsActive, -1),
				edc.IsActive,
			),
		),
	)
}

// DefineExecutionDataCollector is the main function that defines the constraints of the ExecutionDataCollector.
func DefineExecutionDataCollector(comp *wizard.CompiledIOP,
	edc *ExecutionDataCollector,
	name string,
	timestamps *fetch.BlockDataFetcher,
	metadata fetch.BlockTxnMetadata,
	txnData fetch.TxnDataFetcher,
	rlp fetch.RlpTxnFetcher) {
	// selector constraints cover the prover actions which use dedicated.IsZero, but also
	// constrain the EndOfRlpSegment column.
	// these prover actions must be defined first, or dependent constraints will fail.
	DefineSelectorConstraints(comp, edc, name)

	// Indicator constraints, which concern the indicators: isActive,
	// but also IsNoTx, IsTimestamp, IsBlockHashHi, IsBlockHashLo,
	// IsAddrHi, IsAddrLo and IsTxRLP.
	DefineIndicatorExclusion(comp, edc, name)
	DefineIndicatorOrder(comp, edc, name)
	DefineIndicatorConverseOrder(comp, edc, name)
	DefineIndicatorsMustBeBinary(comp, edc)
	DefineIsActiveConstraints(comp, edc, name)

	// constraints that concern the limbs, unaligned limbs, the alignment powers
	// and the number of bytes
	DefineNumberOfBytesConstraints(comp, edc, name)
	DefineLimbConsistencyConstraints(comp, edc, name)

	// zeroization constraints when the isActive filter is set to 0
	DefineZeroizationConstraints(comp, edc, name)

	// some columns must remain constant on the corresponding block/module segment, concerns:
	// edc.FirstAbsTxIDBlock, edc.LastAbsTxIDBlock, edc.AbsTxIDMax
	DefineConstantConstraints(comp, edc, name)

	// simple constraint asking that HashNum is constant
	DefineHashNumConstraints(comp, edc, name)

	// Constraints for the counters edc.Ct, edc.AbsTxID, edc.BlockID and edc.TotalBytesCounter
	DefineCounterConstraints(comp, edc, name)

	// Enforce data consistency with the arithmetization fetchers using projection queries
	ProjectionQueries(comp, edc, name, timestamps, metadata, txnData, rlp)
	// Enforce additional data consistency with the arithmetization fetchers using lookup queries
	LookupQueries(comp, edc, name, metadata)
}

// AssignExecutionDataCollector assigns the data in the ExecutionDataCollector, using
// the arithmetizationfetchers fetch.BlockDataFetcher, fetch.BlockTxnMetadata,
// fetch.TxnDataFetcher, and fetch.RlpTxnFetcher.
func AssignExecutionDataCollector(run *wizard.ProverRuntime,
	edc *ExecutionDataCollector,
	timestamps *fetch.BlockDataFetcher,
	metadata fetch.BlockTxnMetadata,
	txnData fetch.TxnDataFetcher,
	rlp fetch.RlpTxnFetcher,
	blockHashList []types.FullBytes32,
) {
	size := edc.Limbs[0].Size()
	// generate a helper struct that instantiates field element vectors for all our columns
	vect := NewExecutionDataCollectorVectors(size)

	fetchedAbsTxIdMax := rlp.AbsTxNumMax.GetColAssignmentAt(run, 0)
	absTxIdMax := field.ToInt(&fetchedAbsTxIdMax)

	absTxCt := 1
	rlpCt := 0
	totalCt := 0

	for blockCt := 0; blockCt < timestamps.Data[0].Size(); blockCt++ {
		isBlockPresent := metadata.FilterFetched.GetColAssignmentAt(run, blockCt)
		if isBlockPresent.IsOne() {
			// block-wide information
			totalTxBlockField := metadata.TotalNoTxnBlock.GetColAssignmentAt(run, blockCt)
			totalTxBlock := totalTxBlockField.Uint64()
			firstAbsTxIDBlock := metadata.FirstAbsTxId.GetColAssignmentAt(run, blockCt)
			lastAbsTxIDBlock := metadata.LastAbsTxId.GetColAssignmentAt(run, blockCt)
			fetchNoTx := metadata.TotalNoTxnBlock.GetColAssignmentAt(run, blockCt)

			// genericLoadFunction is a function that computes most of the data
			// that is computed in a similar way in each type of row.
			// opType is the type of row, and the field element value is the unaligned
			// limb value
			genericLoadFunction := func(value []field.Element) {
				vect.IsActive[totalCt].SetOne()
				vect.SetCounters(totalCt, blockCt, absTxCt, absTxIdMax)
				vect.SetBlockMetadata(totalCt, totalTxBlockField, firstAbsTxIDBlock, lastAbsTxIDBlock)
				// set limbs left aligned, the rest of the limbs are zeroes
				for i := range value {
					vect.Limbs[i][totalCt] = value[i]
				}
			}

			// row 0, load the number of transactions
			vect.IsNoTx[totalCt].SetOne()
			vect.NoBytes[totalCt].SetInt64(noBytesNoTxn)
			genericLoadFunction([]field.Element{fetchNoTx})
			totalCt++

			// row 1, fetch the timestamp
			var fetchedTimestamp [common.NbLimbU128]field.Element
			for i := range fetchedTimestamp {
				// Fetch the 64 bits of timestamp and store it left aligned in the first limbs, the rest of the limbs are zeroes.
				fetchedTimestamp[i] = timestamps.Data[i].GetColAssignmentAt(run, blockCt)
			}
			// row 1, plug in the timestamp
			vect.IsTimestamp[totalCt].SetOne()
			vect.NoBytes[totalCt].SetInt64(noBytesTimestamp)
			genericLoadFunction(fetchedTimestamp[nbTimestampEmpty:])
			totalCt++

			// row 2, fetch the Hi part of the blockhash
			var fetchedBlockhashHi [common.NbLimbU128]field.Element
			for i := range fetchedBlockhashHi {
				fetchedBlockhashHi[i].SetBytes(blockHashList[blockCt][i*2 : (i+1)*2])
			}

			// row 2, plug in the Hi part of the blockhash
			vect.IsBlockHashHi[totalCt].SetOne()
			vect.NoBytes[totalCt].SetInt64(noBytesBlockHash)
			genericLoadFunction(fetchedBlockhashHi[:])
			vect.AbsTxIDMax[totalCt].Set(&fetchedAbsTxIdMax)
			totalCt++

			// row 3, fetch the Lo part of the blockhash
			// NbLimbU128 is the number of 16-bit limbs (8), so the byte offset
			// for the second half of the 32-byte hash is 2*NbLimbU128 = 16.
			var fetchedBlockhashLo [common.NbLimbU128]field.Element
			const blockHashLoByteOffset = 2 * common.NbLimbU128 // = 16 bytes
			for i := range fetchedBlockhashLo {
				fetchedBlockhashLo[i].SetBytes(blockHashList[blockCt][blockHashLoByteOffset+i*2 : blockHashLoByteOffset+(i+1)*2])
			}

			// row 3, plug in the Lo part of the blockhash
			vect.IsBlockHashLo[totalCt].SetOne()
			vect.NoBytes[totalCt].SetInt64(noBytesBlockHash)
			genericLoadFunction(fetchedBlockhashLo[:])
			totalCt++

			if fetchNoTx.IsZero() {
				// there are no user transactions in this block
				// we skip to the next block
				continue
			} else {
				// iterate through transactions
				for txIdInBlock := uint64(1); txIdInBlock <= totalTxBlock; txIdInBlock++ {
					// fetch the sender address Hi
					var fetchedAddrHi [common.NbLimbU32]field.Element

					for i := range common.NbLimbU32 {
						fetchedAddrHi[i] = txnData.From[i].GetColAssignmentAt(run, absTxCt-1)
					}

					// plug in the sender address Hi
					vect.IsAddrHi[totalCt].SetOne()
					vect.NoBytes[totalCt].SetInt64(noBytesSenderAddrHi)
					genericLoadFunction(fetchedAddrHi[:])
					totalCt++

					// fetch the sender address Lo
					var fetchedAddrLo [common.NbLimbU128]field.Element
					for i := range common.NbLimbU128 {
						fetchedAddrLo[i] = txnData.From[common.NbLimbU32+i].GetColAssignmentAt(run, absTxCt-1)
					}
					// load the sender address Lo
					vect.IsAddrLo[totalCt].SetOne()
					vect.NoBytes[totalCt].SetInt64(noBytesSenderAddrLo)
					genericLoadFunction(fetchedAddrLo[:])
					totalCt++

					// load the RLP limbs
					currentAbsTxId := field.NewElement(uint64(absTxCt))
					rlpPointerAbsTxId := rlp.AbsTxNum.GetColAssignmentAt(run, rlpCt)
					// add RLP limbs (multiple limbs)
					for currentAbsTxId.Equal(&rlpPointerAbsTxId) {
						// while currentAbsTxId is equal to rlpPointerAbsTxId, namely we are parsing the limbs for the same AbsTxID
						// first fetch the limb data
						var rlpLimbs [common.NbLimbU128]field.Element
						for i := range rlpLimbs {
							rlpLimbs[i] = rlp.Limbs[i].GetColAssignmentAt(run, rlpCt)
						}
						// now load the RLP limb data
						rlpNBytes := rlp.NBytes.GetColAssignmentAt(run, rlpCt)
						vect.IsTxRLP[totalCt].SetOne()
						vect.NoBytes[totalCt].Set(&rlpNBytes)
						genericLoadFunction(rlpLimbs[:])
						totalCt++

						rlpCt++
						rlpPointerAbsTxId = rlp.AbsTxNum.GetColAssignmentAt(run, rlpCt)
					}
					vect.EndOfRlpSegment[totalCt-1].SetOne()
					// increase transaction counter
					absTxCt++
				}
			}

		} else {
			// finished processing all the blocks, reached the inactive part of the module.
			// therefore, we do not set the isActive filter to 1.
			// No more blocks to assign.
			// before breaking, set FinalTotalBytesCounter to correspond to TotalBytesCounter in the last active row (totalCt-1).
			vect.FinalTotalBytesCounter = vect.TotalBytesCounter[totalCt-1]
			break
		}
	}

	// assign the columns to the ExecutionDataCollector
	AssignExecutionDataColumns(run, edc, vect)
	// assign the selectors
	edc.ComputeSelectorBlockDiff.Run(run)
	edc.ComputeSelectorLastTxBlock.Run(run)
	edc.ComputeSelectorEndOfAllTx.Run(run)
	edc.ComputeSelectorAbsTxIDDiff.Run(run)
	edc.ComputeSelectorBlockHasZeroUserTx.Run(run)
}

// AssignExecutionDataColumns uses the helper struct ExecutionDataCollectorVectors to assign the columns of
// the ExecutionDataCollector
func AssignExecutionDataColumns(run *wizard.ProverRuntime, edc *ExecutionDataCollector, vect *ExecutionDataCollectorVectors) {
	run.AssignColumn(edc.BlockID.GetColID(), smartvectors.NewRegular(vect.BlockID))
	run.AssignColumn(edc.AbsTxID.GetColID(), smartvectors.NewRegular(vect.AbsTxID))
	for i := range edc.Limbs {
		run.AssignColumn(edc.Limbs[i].GetColID(), smartvectors.NewRegular(vect.Limbs[i]))
	}
	run.AssignColumn(edc.NoBytes.GetColID(), smartvectors.NewRegular(vect.NoBytes))
	run.AssignColumn(edc.TotalNoTxBlock.GetColID(), smartvectors.NewRegular(vect.TotalNoTxBlock))
	run.AssignColumn(edc.IsActive.GetColID(), smartvectors.NewRegular(vect.IsActive))
	run.AssignColumn(edc.IsNoTx.GetColID(), smartvectors.NewRegular(vect.IsNoTx))
	run.AssignColumn(edc.IsBlockHashHi.GetColID(), smartvectors.NewRegular(vect.IsBlockHashHi))
	run.AssignColumn(edc.IsBlockHashLo.GetColID(), smartvectors.NewRegular(vect.IsBlockHashLo))
	run.AssignColumn(edc.IsTimestamp.GetColID(), smartvectors.NewRegular(vect.IsTimestamp))
	run.AssignColumn(edc.IsTxRLP.GetColID(), smartvectors.NewRegular(vect.IsTxRLP))
	run.AssignColumn(edc.IsAddrHi.GetColID(), smartvectors.NewRegular(vect.IsAddrHi))
	run.AssignColumn(edc.IsAddrLo.GetColID(), smartvectors.NewRegular(vect.IsAddrLo))
	run.AssignColumn(edc.Ct.GetColID(), smartvectors.NewRegular(vect.Ct))
	run.AssignColumn(edc.HashNum.GetColID(), smartvectors.NewConstant(field.NewElement(hashNum), len(vect.Ct)))
	run.AssignColumn(edc.AbsTxIDMax.GetColID(), smartvectors.NewRegular(vect.AbsTxIDMax))
	run.AssignColumn(edc.EndOfRlpSegment.GetColID(), smartvectors.NewRegular(vect.EndOfRlpSegment))
	run.AssignColumn(edc.FirstAbsTxIDBlock.GetColID(), smartvectors.NewRegular(vect.FirstAbsTxIDBlock))
	run.AssignColumn(edc.LastAbsTxIDBlock.GetColID(), smartvectors.NewRegular(vect.LastAbsTxIDBlock))
	run.AssignColumn(edc.TotalBytesCounter.GetColID(), smartvectors.NewRegular(vect.TotalBytesCounter))
	run.AssignColumn(edc.FinalTotalBytesCounter.GetColID(), smartvectors.NewConstant(vect.FinalTotalBytesCounter, len(vect.BlockID)))
}
