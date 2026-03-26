package execution

import (
	"bytes"
	"fmt"
	"math/big"
	"path"
	"strings"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/config"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Craft prover's functional inputs
func CraftProverOutput(
	cfg *config.Config,
	req *Request,
) Response {

	var (
		l2BridgeAddress = cfg.Layer2.MsgSvcContract
		blocks          = req.Blocks()
	)

	// Sanity-check that the layer2 config matches the block data.
	// This catches config mismatches early instead of failing deep
	// in the circuit verification.
	sanityCheckChainConfig(cfg, blocks)

	var (
		execDataBuf = &bytes.Buffer{}
		rsp         = Response{
			BlocksData:           make([]BlockData, len(blocks)),
			ChainID:              cfg.Layer2.ChainID,
			BaseFee:              cfg.Layer2.BaseFee,
			CoinBase:             types.EthAddress(cfg.Layer2.CoinBase),
			L2BridgeAddress:      types.EthAddress(cfg.Layer2.MsgSvcContract),
			MaxNbL2MessageHashes: cfg.TracesLimits.BlockL2L1Logs(),
		}
		// execution prover performance metadta accumulators
		totalTxs     uint64
		totalGasUsed uint64
	)

	// Extract the data from the block
	for i := range blocks {

		var (
			// The assignment is not made in the loop directly otherwise the
			// linter will detect "memory aliasing in a for loop" even though
			// we do not modify the data of the block.
			block = &blocks[i]

			// Fetch the transaction indices
			logs = req.LogsForBlock(i)

			// Filter the logs L2 to L1, and hash them before sending them
			// back to the coordinator.
			l2l1MessageHashes = bridge.L2L1MessageHashes(logs, l2BridgeAddress)
		)

		// This encodes the block as it will be by the compressor before running
		// the compression algorithm.
		blob.EncodeBlockForCompression(block, execDataBuf)

		// Encode the transactions
		rsp.BlocksData[i].RlpEncodedTransactions = RlpTransactions(block)
		rsp.BlocksData[i].FromAddresses = FromAddresses(block)
		rsp.BlocksData[i].TimeStamp = block.Time()
		rsp.BlocksData[i].L2ToL1MsgHashes = l2l1MessageHashes
		rsp.BlocksData[i].BlockHash = types.FullBytes32(block.Hash())

		// Also filters the RollingHashUpdated logs
		events := bridge.ExtractRollingHashUpdated(logs, l2BridgeAddress)
		if len(events) > 0 {
			rsp.BlocksData[i].LastRollingHashUpdatedEvent = events[len(events)-1]
		}
		rsp.AllRollingHashEvent = append(rsp.AllRollingHashEvent, events...)

		// This collects the L2 message hashes
		rsp.AllL2L1MessageHashes = append(
			rsp.AllL2L1MessageHashes,
			l2l1MessageHashes...,
		)

		// execution prover performance metadta
		totalTxs += uint64(len(block.Transactions()))
		totalGasUsed += block.GasUsed()
	}

	execDataMultiCommitment := public_input.ComputeExecutionDataMultiCommitment(execDataBuf.Bytes())
	rsp.ExecDataChecksum = execDataMultiCommitment.Bls12377
	rsp.execDataMultiCommitment = execDataMultiCommitment

	logrus.Infof("Conflation stats - totalTxs: %d, totalGasUsed: %d", totalTxs, totalGasUsed)

	// Add into that the data of the state-manager
	// Run the inspector and pass the parsed traces back to the caller.
	// These traces may be used by the state-manager module depending on
	// if the flag `PROVER_WITH_STATE_MANAGER`
	inspectStateManagerTraces(req, &rsp)

	// Value of the first blocks
	rsp.FirstBlockNumber = utils.ToInt(blocks[0].NumberU64())

	// Set the public input as part of the response immediately so that we can
	// easily debug issues during the proving.
	rsp.PublicInput = types.Bls12377Fr(rsp.FuncInput().Sum())

	return rsp
}

// inspectStateManagerTraces parsed the state-manager traces from the given
// input and inspect them to see if they are self-consistent and if they match
// the parentStateRootHash. This behaviour can be altered by setting the field
// `tolerate_state_root_hash_mismatch`, see its documentation. In case of
// success, the function returns the decoded state-manager traces. Otherwise, it
// panics.
func inspectStateManagerTraces(
	req *Request,
	resp *Response,
) {

	// Extract the traces from the inputs
	var (
		traces      = req.StateManagerTraces()
		firstParent = req.ZkParentStateRootHash
		parent      = req.ZkParentStateRootHash
	)

	for i := range traces {

		if len(traces[i]) > 0 {
			// Run the trace inspection routine
			old, new, err := statemanager.CheckTraces(traces[i])
			// The trace must have been validated
			if err != nil {
				utils.Panic("error parsing the state manager traces : %v", err)
			}

			// The "old of a block" must equal the parent
			if old != parent {
				utils.Panic("old does not match with parent root hash")
			}

			// Populate the prover's output with the recovered root hash
			resp.BlocksData[i].RootHash = new
			parent = new
		} else {
			// This can happen when there are no transaction in a block
			// In this case, we do not need to do anything
			resp.BlocksData[i].RootHash = parent
		}

	}

	resp.ParentStateRootHash = firstParent
}

func (req *Request) collectSignatures() ([]ethereum.Signature, [][32]byte) {

	var (
		signatures = []ethereum.Signature{}
		txHashes   = [][32]byte{}
		blocks     = req.Blocks()
		currTx     = 0
	)

	for i := range blocks {
		for _, tx := range blocks[i].Transactions() {

			var (
				txHash      = ethereum.GetTxHash(tx)
				txSignature = ethereum.GetJsonSignature(tx)
			)

			signatures = append(signatures, txSignature)
			txHashes = append(txHashes, txHash)

			currTx++
		}
	}

	return signatures, txHashes
}

// FuncInput are all the relevant fields parsed by the prover that
// are functionally useful to contextualize what the proof is proving. This
// is used by the aggregation circuit to ensure that the execution proofs
// relate to consecutive Linea block execution.
func (rsp *Response) FuncInput() *public_input.Execution {

	var (
		firstBlock = &rsp.BlocksData[0]
		lastBlock  = &rsp.BlocksData[len(rsp.BlocksData)-1]
		fi         = &public_input.Execution{
			L2MessageServiceAddr:  rsp.L2BridgeAddress,
			ChainID:               uint64(rsp.ChainID),
			BaseFee:               uint64(rsp.BaseFee),
			CoinBase:              types.EthAddress(rsp.CoinBase),
			FinalBlockTimestamp:   lastBlock.TimeStamp,
			FinalBlockNumber:      uint64(rsp.FirstBlockNumber + len(rsp.BlocksData) - 1),
			InitialBlockTimestamp: firstBlock.TimeStamp,
			InitialBlockNumber:    uint64(rsp.FirstBlockNumber),
			DataChecksum:          rsp.ExecDataChecksum,
			L2MessageHashes:       types.AsByteArrSlice(rsp.AllL2L1MessageHashes),
			InitialStateRootHash:  rsp.ParentStateRootHash.ToBytes32(),
			FinalStateRootHash:    lastBlock.RootHash.ToBytes32(),
		}
	)

	if len(rsp.AllRollingHashEvent) > 0 {
		var (
			firstRHEvent = rsp.AllRollingHashEvent[0]
			lastRHEvent  = rsp.AllRollingHashEvent[len(rsp.AllRollingHashEvent)-1]
		)

		fi.InitialRollingHashUpdate = firstRHEvent.RollingHash
		fi.LastRollingHashUpdate = lastRHEvent.RollingHash
		fi.FirstRollingHashUpdateNumber = uint64(firstRHEvent.MessageNumber)
		fi.LastRollingHashUpdateNumber = uint64(lastRHEvent.MessageNumber)
	}

	return fi
}

func NewWitness(cfg *config.Config, req *Request, rsp *Response) *Witness {
	txSignatures, txHashes := req.collectSignatures()
	return &Witness{
		ZkEVM: &zkevm.Witness{
			ExecTracesFPath:        path.Join(cfg.Execution.ConflatedTracesDir, req.ConflatedExecutionTracesFile),
			SMTraces:               req.StateManagerTraces(),
			TxSignatures:           txSignatures,
			TxHashes:               txHashes,
			L2BridgeAddress:        cfg.Layer2.MsgSvcContract,
			ChainID:                cfg.Layer2.ChainID,
			BaseFee:                cfg.Layer2.BaseFee,
			CoinBase:               types.EthAddress(cfg.Layer2.CoinBase),
			BlockHashList:          getBlockHashList(rsp),
			ExecDataSchwarzZipfelX: rsp.execDataMultiCommitment.X,
			ExecData:               rsp.execDataMultiCommitment.Data,
		},
		FuncInp: rsp.FuncInput(),
	}
}

func getBlockHashList(rsp *Response) []types.FullBytes32 {
	res := []types.FullBytes32{}
	for i := range rsp.BlocksData {
		res = append(res, rsp.BlocksData[i].BlockHash)
	}
	return res
}

// sanityCheckChainConfig sanity-checks that the [layer2] config values (chainID,
// baseFee, coinBase) are consistent with the block headers and transactions in
// the request. This catches configuration mismatches early (e.g. running with
// the wrong config file) instead of failing deep inside the circuit prover.
func sanityCheckChainConfig(cfg *config.Config, blocks []ethtypes.Block) {
	if len(blocks) == 0 {
		return
	}

	cfgChainID := new(big.Int).SetUint64(uint64(cfg.Layer2.ChainID))
	cfgBaseFee := new(big.Int).SetUint64(uint64(cfg.Layer2.BaseFee))
	cfgCoinBase := ethcommon.Address(cfg.Layer2.CoinBase)
	cfgMsgSvc := ethcommon.Address(cfg.Layer2.MsgSvcContract)

	for i := range blocks {
		block := &blocks[i]
		header := block.Header()

		var mismatches []string

		// Check coinbase
		if header.Coinbase != cfgCoinBase {
			mismatches = append(mismatches, "coinBase")
		}

		// Check baseFee (may be nil for pre-EIP-1559 blocks)
		if header.BaseFee != nil && header.BaseFee.Cmp(cfgBaseFee) != 0 {
			mismatches = append(mismatches, "baseFee")
		}

		// Check chainID from transactions (skip legacy txs which derive
		// chainID from V and may panic if unsigned).
		var blockChainID string
		for _, tx := range block.Transactions() {
			if tx.Type() == ethtypes.LegacyTxType {
				continue
			}
			txChainID := tx.ChainId()
			if txChainID != nil && txChainID.Sign() > 0 {
				blockChainID = txChainID.String()
				if txChainID.Cmp(cfgChainID) != 0 {
					mismatches = append(mismatches, "chainID")
				}
				break
			}
		}

		if len(mismatches) > 0 {
			blockBaseFee := "(nil)"
			if header.BaseFee != nil {
				blockBaseFee = header.BaseFee.String()
			}
			if blockChainID == "" {
				blockChainID = "(no typed tx)"
			}

			utils.Panic(
				"chain config mismatch at block %d — check your config file!\n\n"+
					"  mismatched fields: %s\n\n"+
					"  %-16s %-44s %s\n"+
					"  %-16s %-44s %s\n"+
					"  %-16s %-44s %s\n"+
					"  %-16s %-44s %s\n"+
					"  %-16s %-44s %s\n",
				header.Number,
				strings.Join(mismatches, ", "),
				"", "CONFIG", "BLOCK",
				"chainID", fmt.Sprint(cfg.Layer2.ChainID), blockChainID,
				"baseFee", fmt.Sprint(cfg.Layer2.BaseFee), blockBaseFee,
				"coinBase", cfgCoinBase.Hex(), header.Coinbase.Hex(),
				"msgSvcContract", cfgMsgSvc.Hex(), "(n/a)",
			)
		}
	}

	logrus.Infof("Chain config validation passed: chainID=%d baseFee=%d coinBase=%s msgSvcContract=%s",
		cfg.Layer2.ChainID, cfg.Layer2.BaseFee, cfgCoinBase.Hex(), cfgMsgSvc.Hex())
}
