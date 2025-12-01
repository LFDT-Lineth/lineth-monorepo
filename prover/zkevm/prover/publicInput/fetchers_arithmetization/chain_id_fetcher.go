package fetchers_arithmetization

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	arith "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/arith_struct"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

const (
	// ChainIDOffset is the corresponding offset position for the chain ID
	// since it is a shift, -1 means no offset.
	ChainIDOffset = -3
)

// ChainIDFetcher is a struct used to fetch the chainIDs from the arithmetization's BlockDataCols
type ChainIDFetcher struct {
	// chainID
	ChainID       ifaces.Column // a column used to fetch the ChainID. The implementation is currently unaligned with respect to the number of limbs.
	NBytesChainID ifaces.Column // a column used to fetch the number of bytes of the ChainID limb data
}

// NewChainIDFetcher returns a new ChainIDFetcher with initialized columns that are not constrained.
func NewChainIDFetcher(comp *wizard.CompiledIOP, name string, bdc *arith.BlockDataCols) ChainIDFetcher {
	size := bdc.Ct.Size()
	res := ChainIDFetcher{
		ChainID:       util.CreateCol(name, "CHAIN_ID", size, comp),
		NBytesChainID: util.CreateCol(name, "N_BYTES_CHAIN_ID", size, comp), // 2 bytes for chainID, we will constrain it later
	}

	return res
}

// DefineChainIDFetcher specifies the constraints of the ChainIDFetcher with respect to the BlockDataCols
func DefineChainIDFetcher(comp *wizard.CompiledIOP, fetcher *ChainIDFetcher, name string, bdc *arith.BlockDataCols) {
	// Constrain fetcher ChainID to equal the last block's chainID from BlockDataCols.
	// Both sides are shifted by ChainIDOffset=-3 to maintain consistent negative offsets,
	// which is required by the distributed module's limitless feature. The constraint
	// ChainID[-3] == DataLo[-3] is enforced, and ChainID must be constant (see below),
	// ensuring the chain ID value is consistent across all positions.
	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%s_%s", name, "LAST_LOCAL"),
		sym.Sub(
			column.Shift(fetcher.ChainID, ChainIDOffset), // ChainID at offset -3
			column.Shift(bdc.DataLo, ChainIDOffset),      // DataLo at offset -3 (last block's chain ID)
		),
	)

	// constrain the N_BYTES_CHAIN_ID to have only two BYTES
	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%s_%s", name, "N_BYTES_CHAIN_ID_CONSTRAINT"),
		sym.Sub(
			fetcher.NBytesChainID,
			2, // hardcode 2 bytes for the N_BYTES_CHAIN_ID
		),
	)

	// require both ChainID and NBytesChainID to be constant columns.
	commonconstraints.MustBeConstant(comp, fetcher.ChainID)
	commonconstraints.MustBeConstant(comp, fetcher.NBytesChainID)
}

// AssignChainIDFetcher assigns the data in the ChainIDFetcher using data fetched from the BlockDataCols
func AssignChainIDFetcher(run *wizard.ProverRuntime, fetcher *ChainIDFetcher, bdc *arith.BlockDataCols) {
	size := bdc.Ct.Size()
	var (
		chainID = make([]field.Element, size)
	)
	fetchedChainID := bdc.DataLo.GetColAssignmentAt(run, size+ChainIDOffset)
	chainID[0].Set(&fetchedChainID)
	run.AssignColumn(fetcher.ChainID.GetColID(), smartvectors.NewConstant(chainID[0], size))
	run.AssignColumn(fetcher.NBytesChainID.GetColID(), smartvectors.NewConstant(field.NewElement(2), size))
}
