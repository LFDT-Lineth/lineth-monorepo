package zkevm

import (
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

// Witness is a collection of prover inputs used to derive an assignment to the
// full proving scheme.
type Witness struct {
	// ExecTracesFPath is the filepath toward the execution traces to use for
	// proof trace generation.
	ExecTracesFPath string
	// StateManager traces
	SMTraces [][]statemanager.DecodedTrace
	// TxSignatures lists the signatures of the transaction as found
	// chronologically in the block.
	TxSignatures []ethereum.Signature
	// TxHashes lists the hash of the transactions in the order found in the
	// block.
	TxHashes        [][32]byte
	L2BridgeAddress common.Address
	ChainID         uint
	// BlockHashList is the list of the block-hashes of the proven blocks
	BlockHashList []types.FullBytes32
}

// TxSignatureGetter implements the ecdsa.TxSignatureGetter interface
func (w Witness) TxSignatureGetter(i int, txHash []byte) (r, s, v *big.Int, err error) {

	if i > len(w.TxHashes) {
		return nil, nil, nil, fmt.Errorf("requested txID outgoes the total number of transactions we found in the conflation")
	}

	if utils.HexEncodeToString(txHash) != utils.HexEncodeToString(w.TxHashes[i][:]) {
		return nil, nil, nil, fmt.Errorf(
			"requested txID=%v while txnrlp expects it to have it for txhash=%v but the blocks transaction has hash=%v",
			i, utils.HexEncodeToString(w.TxHashes[i][:]), utils.HexEncodeToString(txHash),
		)
	}

	sig := w.TxSignatures[i]

	r, _ = new(big.Int).SetString(sig.R, 0)
	s, _ = new(big.Int).SetString(sig.S, 0)
	v, _ = new(big.Int).SetString(sig.V, 0)

	return r, s, v, nil
}
