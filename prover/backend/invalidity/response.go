package invalidity

import (
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type Response struct {
	// the transaction that was attempted to be included in the current aggregation
	Transaction *ethtypes.Transaction
	// signer of the transaction
	Signer types.EthAddress `json:"signer"`
	// hash of the transaction (before signing)
	TxHash string `json:"txHash"`
	//Rlp encoding of signed transaction
	RLPEncodedTx string `json:"rlpEncodedTx"`
	// Transaction number assigned by L1 contract (decimal encoding)
	ForcedTransactionNumber uint64 `json:"ftxNumber"`
	// Previous FTX rolling hash, i.e. the FTX stream hash of the previous forced transaction.
	PrevFtxRollingHash types.Bls12377Fr `json:"prevFtxRollingHash"`
	// The block number deadline before which one expects to see the transaction (decimal encoding)
	DeadlineBlockHeight uint64 `json:"ftxBlockNumberDeadline"`
	// The type of invalidity for the forced transaction.
	InvalidityType invalidity.InvalidityType `json:"invalidityType"`
	// ZK parent state root hash
	ZkParentStateRootHash            types.KoalaOctuplet `json:"zkParentStateRootHash"`
	SimulatedExecutionBlockNumber    uint64              `json:"simulatedExecutionBlockNumber"`
	SimulatedExecutionBlockTimestamp uint64              `json:"simulatedExecutionBlockTimestamp"`

	// Dynamic chain configuration (mirrors execution Response fields)
	ChainID  uint             `json:"chainID"`
	BaseFee  uint             `json:"baseFee"`
	CoinBase types.EthAddress `json:"coinBase"`

	L2BridgeAddress types.EthAddress `json:"l2BridgeAddress"`

	// the FtxRollingHash of the forced transaction
	FtxRollingHash types.Bls12377Fr `json:"ftxRollingHash"`
	// PublicInput is the final value public input of the current proof. This
	PublicInput types.Bls12377Fr `json:"publicInput"`
	// Proof in 0x prefixed hexstring format
	Proof string `json:"proof"`
	// The shasum of the verifier key to use to verify the proof. This is used
	// by the aggregation circuit to identify the circuit ID to use in the proof.
	VerifyingKeyShaSum string            `json:"verifyingKeyShaSum"`
	ProverMode         config.ProverMode `json:"proverMode"`
	ProverVersion      string            `json:"proverVersion"`
}

// FuncInput reconstructs the functional public inputs from the response fields.
func (resp *Response) FuncInput() *public_input.Invalidity {
	req := &Request{
		RlpEncodedTx:                     resp.RLPEncodedTx,
		ForcedTransactionNumber:          resp.ForcedTransactionNumber,
		PrevFtxRollingHash:               resp.PrevFtxRollingHash,
		DeadlineBlockHeight:              resp.DeadlineBlockHeight,
		InvalidityType:                   resp.InvalidityType,
		ZkParentStateRootHash:            resp.ZkParentStateRootHash,
		SimulatedExecutionBlockNumber:    resp.SimulatedExecutionBlockNumber,
		SimulatedExecutionBlockTimestamp: resp.SimulatedExecutionBlockTimestamp,
	}
	cfg := &config.Config{}
	cfg.Layer2.ChainID = resp.ChainID
	cfg.Layer2.BaseFee = resp.BaseFee
	cfg.Layer2.CoinBase = common.Address(resp.CoinBase)
	cfg.Layer2.MsgSvcContract = common.Address(resp.L2BridgeAddress)

	return FuncInput(req, cfg)
}
