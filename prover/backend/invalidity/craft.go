package invalidity

import (
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	circuitInvalidity "github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// FuncInput are all the relevant fields parsed by the prover that
// are functionally useful to contextualize what the proof is proving. This
// is used by the aggregation circuit to ensure that the invalidity proofs
// relate to consecutive Linea forced transactions.
func FuncInput(req *Request, cfg *config.Config) *public_input.Invalidity {

	tx, err := ethereum.RlpDecodeWithSignature(req.RlpEncodedTx)
	if err != nil {
		utils.Panic("could not decode the RlpEncodedTx: %v", err)
	}
	fromAddress := ethereum.GetFrom(tx)

	// Compute the unsigned transaction hash
	txHash := ethereum.GetTxHash(tx)

	// Compute the FtxRollingHash from the previous rolling hash
	ftxRollingHash := circuitInvalidity.UpdateFtxRollingHash(
		req.PrevFtxRollingHash,
		txHash,
		req.DeadlineBlockHeight,
		fromAddress,
	)

	// Extract the To address from the transaction
	var toAddress types.EthAddress
	if to := tx.To(); to != nil {
		toAddress = types.EthAddress(*to)
	} else {
		panic("to address is nil")
	}

	fi := &public_input.Invalidity{
		TxHash:                  txHash,
		TxNumber:                uint64(req.ForcedTransactionNumber),
		FromAddress:             fromAddress,
		DeadLineBlockNumber:     uint64(req.DeadlineBlockHeight),
		StateRootHash:           req.ZkParentStateRootHash,
		FtxRollingHash:          ftxRollingHash,
		ToAddress:               toAddress,
		FromIsFiltered:          req.InvalidityType == circuitInvalidity.FilteredAddressFrom,
		ToIsFiltered:            req.InvalidityType == circuitInvalidity.FilteredAddressTo,
		SimulatedBlockNumber:    req.SimulatedExecutionBlockNumber,
		SimulatedBlockTimestamp: req.SimulatedExecutionBlockTimestamp,
		CoinBase:                types.EthAddress(cfg.Layer2.CoinBase),
		BaseFee:                 uint64(cfg.Layer2.BaseFee),
		ChainID:                 uint64(cfg.Layer2.ChainID),
		L2MessageServiceAddr:    types.EthAddress(cfg.Layer2.MsgSvcContract),
	}
	return fi

}
