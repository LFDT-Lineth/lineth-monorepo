package execution

import (
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// JSON schema of the message to return as an output of the prover
// Notations:
//   - N denotes the number of conflated blocks.
//   - Ti, denotes the number of transactions in the block i
//   - T denotes the total number of transactions (nimaltogether
//   - L denotes the total number of L2 to L1 logs in the conflated
type Response struct {

	// Proof in 0x prefixed hexstring format
	Proof      string            `json:"proof"`
	ProverMode config.ProverMode `json:"proverMode"`

	// VerifierIndex is a deprecated field indicating which verifier contract to
	// verify the proof.
	//
	// Deprecated: the execution proof is no longer verified on-chain.
	VerifierIndex uint `json:"verifierIndex"`

	// The shasum of the verifier key to use to verify the proof. This is used
	// by the aggregation circuit to identify the circuit ID to use in the proof.
	VerifyingKeyShaSum string `json:"verifyingKeyShaSum"`

	// Block dependant inputs for the proof
	BlocksData []BlockData `json:"blocksData"`

	// Initial root hash before executing the conflated block
	ParentStateRootHash types.KoalaOctuplet `json:"parentStateRootHash"`

	// Boolean flag indicating whether the parent root hash mismatches what we
	// found in the shomei proof for the first block. This field is only set
	// when the config field `tolerate_parent_state_root_hash_mismatch` is set
	// to true.
	HasParentStateRootHashMismatch bool `json:"hasParentStateRootHashMismatch"`

	// Version format: "vX.X.X"
	Version string `json:"proverVersion"`

	// First block number
	FirstBlockNumber int `json:"firstBlockNumber"`

	// ExecDataChecksum checksums and fingerprints for the execution data.
	ExecDataChecksum public_input.ExecDataChecksum `json:"execDataChecksum"`

	// execDataMultiCommitment stores the multi-commitment data that are used
	// to instantiate the prover.
	execDataMultiCommitment public_input.ExecDataMultiCommitment `json:"-"`

	// ChainID indicates which ChainID was used during the execution.
	ChainID uint `json:"chainID"`
	// L2BridgeAddress indicates the address of the L2 bridge was used during
	// the execution.
	L2BridgeAddress types.EthAddress `json:"l2BridgeAddress"`
	// MaxNbL2MessageHashes indicates the max number of L2 Message hashes that
	// can be processed by the execution prover at once in the config.
	MaxNbL2MessageHashes int `json:"maxNbL2MessageHashes"`
	// CoinBase indicates the coinbase of the L2 network that was used during
	// the proof generation
	CoinBase types.EthAddress `json:"coinBase"`
	// BaseFee indicates the base fee of the L2 network that was used during
	// the proof generation
	BaseFee uint `json:"baseFee"`

	// AllRollingHash stores the collection of all the rolling hash events
	// occurring during the execution frame.
	AllRollingHashEvent []bridge.RollingHashUpdated `json:"allRollingHashEvent"`
	// AllL2L1MessageHashes stores the collection of all the L2 to L1 message's
	// hashes.
	AllL2L1MessageHashes []types.FullBytes32 `json:"allL2L1MessageHashes"`
	// PublicInput is the final value public input of the current proof. This
	// field is used for debugging in case one of the proofs don't pass at the
	// aggregation level.
	PublicInput types.Bls12377Fr `json:"publicInput"`
}

type BlockData struct {

	// BlockHash is the Eths block hash
	BlockHash types.FullBytes32 `json:"blockHash"`

	// T Transaction in 0x-prefixed hex format
	RlpEncodedTransactions []string `json:"rlpEncodedTransactions"`

	// L2 to L1 message hashes
	L2ToL1MsgHashes []types.FullBytes32 `json:"l2ToL1MsgHashes"`

	// List of the N timestamps for each blocks. To optimize
	// for space we put the timestamps in uint64 form
	TimeStamp uint64 `json:"timestamp"`

	// List of the n+1 root hashes in chronological order
	// The first root hash is the initial root hash before
	// execution of the first block in the conflated batch
	// and the last one is the final root hash of the state
	// after execution of the last block in the conflated batch.
	RootHash types.KoalaOctuplet `json:"rootHash"`

	// The from addresses of the transactions in the block all concatenated
	// in a single hex string.
	FromAddresses []types.EthAddress `json:"fromAddresses"`

	// Not part of the inputs to hash. Flag indicating whether the block
	// contains a BatchL1MsgReceiptConfirmation
	BatchReceptionIndices []uint16 `json:"batchReceptionIndices"`

	// Last rolling hash update event
	LastRollingHashUpdatedEvent bridge.RollingHashUpdated `json:"lastRollingHashUpdatedEvent"`
}

type PerBlockDebugData struct {

	// TxHashes in 0x prefixed format
	TxHashes []string `json:"txHashes"`

	// Hash of the txHashes
	HashOfTxHashes string `json:"hashOfTxHashes"`

	// Hash of the log hashes
	HashOfLogHashes string `json:"hashOfLogHashes"`

	// Hash of the L1 reception txs positions
	HashOfPositions string `json:"hashOfPositions"`

	// Hash of the from addresses
	HashOfFromAddresses string `json:"hashOfFromAddresses"`

	// Final resulting hash obtained after all inputs have been hashed
	HashForBlock string `json:"HashForBlock"`
}
