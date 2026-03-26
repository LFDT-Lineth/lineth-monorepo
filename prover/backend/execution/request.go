package execution

import (
	"bytes"
	"path"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/sirupsen/logrus"
)

type Request struct {
	ZkParentStateRootHash        types.KoalaOctuplet           `json:"zkParentStateRootHash"`
	ConflatedExecutionTracesFile string                        `json:"conflatedExecutionTracesFile"`
	TracesEngineVersion          string                        `json:"tracesEngineVersion"`
	Type2StateManagerVersion     string                        `json:"type2StateManagerVersion"`
	ZkStateMerkleProof           [][]statemanager.DecodedTrace `json:"zkStateMerkleProof"`
	BlocksData                   []struct {
		Rlp        string         `json:"rlp"`
		BridgeLogs []ethtypes.Log `json:"bridgeLogs"`
	} `json:"blocksData"`
}

// Returns the parsed state-manager traces
func (req *Request) StateManagerTraces() [][]statemanager.DecodedTrace {
	return req.ZkStateMerkleProof
}

// Returns the parsed block data
func (req *Request) Blocks() []ethtypes.Block {
	// Allocate the result
	res := make([]ethtypes.Block, len(req.BlocksData))

	for i, blockdata := range req.BlocksData {
		// Attempt to parse the block as an hexstring
		blockRLPBytes, err := utils.HexDecodeString(blockdata.Rlp)
		if err != nil {
			utils.Panic("error while parsing the block RLP #%v : %v", i, err)
		}
		buffer := bytes.NewReader(blockRLPBytes)

		// Attempt to parse the RLP
		err = rlp.Decode(buffer, &res[i])
		if err != nil {
			utils.Panic("Could not RLP decode the blockRLP 0x%x (block #%v)", blockRLPBytes, i)
		}

	}

	return res
}

// Returns the transactions RLP encoded
func RlpTransactions(block *ethtypes.Block) []string {
	res := []string{}
	for _, tx := range block.Transactions() {
		txRlp := ethereum.EncodeTxForSigning(tx)
		res = append(res, hexutil.Encode(txRlp))
	}
	logrus.Tracef("computed the RLP of #%v transactions", len(block.Transactions()))
	return res
}

// Returns the list of the From addresses for each
// transaction in the block
func FromAddresses(block *ethtypes.Block) []types.EthAddress {
	froms := []types.EthAddress{}
	for _, tx := range block.Transactions() {
		from := ethereum.GetFrom(tx)
		froms = append(froms, from)
	}
	return froms
}

// Returns the array of logs
func (req *Request) LogsForBlock(i int) []ethtypes.Log {
	return req.BlocksData[i].BridgeLogs
}

// ConflatedExecTraceFilepath returns the filepath of the conflated execution
// trace files to pass to the witness.
func (req *Request) ConflatedExecTraceFilepath(traceDir string) string {
	return path.Join(traceDir, req.ConflatedExecutionTracesFile)
}

// GetRawSignaturesVerification returns the raw signatures verification claims
func (req *Request) GetRawSignaturesVerificationInputs() (txHashes [][32]byte, pubKeys [][64]byte, signatures [][65]byte) {

	blocks := req.Blocks()

	// initialize the return values
	txHashes = [][32]byte{}
	pubKeys = [][64]byte{}
	signatures = [][65]byte{}

	for i := 0; i < len(blocks); i++ {
		block := &blocks[i]
		for _, tx := range block.Transactions() {
			// compute the verification claims from the transaction and its signature
			txhash := ethereum.GetTxHash(tx)
			sig := ethereum.GetJsonSignature(tx)
			pubkey, encodedSig, err := ethereum.RecoverPublicKey(txhash, sig)
			if err != nil {
				utils.Panic("error recovering public key from transaction: %v", err)
			}

			// append the claims to the return arguments
			txHashes = append(txHashes, txhash)
			signatures = append(signatures, encodedSig)
			pubKeys = append(pubKeys, pubkey)
		}
	}

	return txHashes, pubKeys, signatures
}
