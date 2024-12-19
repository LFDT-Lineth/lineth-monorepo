package execution

import (
	"bytes"
	_ "embed"
	"fmt"
	"path"
	"regexp"
	"strings"

	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"

	"github.com/consensys/linea-monorepo/prover/config"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

// Embed the constraints-versions.txt file at compile time
//
//go:embed constraints-versions.txt
var constraintsVersionsStr string

// Craft prover's functional inputs
func CraftProverOutput(
	cfg *config.Config,
	req *Request,
) Response {

	// Split the embedded file contents into a string slice
	constraintsVersions := strings.Split(strings.TrimSpace(constraintsVersionsStr), "\n")

	// Check the arithmetization version used to generate the trace is contained in the prover request
	// and fail fast if the constraint version is not supported
	if err := checkArithmetizationVersion(req.ConflatedExecutionTracesFile, req.TracesEngineVersion, constraintsVersions); err != nil {
		panic(err.Error())
	}

	var (
		l2BridgeAddress = cfg.Layer2.MsgSvcContract
		blocks          = req.Blocks()
		execDataBuf     = &bytes.Buffer{}
		rsp             = Response{
			BlocksData:           make([]BlockData, len(blocks)),
			ChainID:              cfg.Layer2.ChainID,
			L2BridgeAddress:      types.EthAddress(cfg.Layer2.MsgSvcContract),
			MaxNbL2MessageHashes: cfg.TracesLimits.BlockL2L1Logs,
		}
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
	}

	rsp.ExecDataChecksum = mimcHashLooselyPacked(execDataBuf.Bytes())

	// Add into that the data of the state-manager
	// Run the inspector and pass the parsed traces back to the caller.
	// These traces may be used by the state-manager module depending on
	// if the flag `PROVER_WITH_STATE_MANAGER`
	inspectStateManagerTraces(req, &rsp)

	// Value of the first blocks
	rsp.FirstBlockNumber = utils.ToInt(blocks[0].NumberU64())

	// Set the public input as part of the response immediately so that we can
	// easily debug issues during the proving.
	rsp.PublicInput = types.Bytes32(rsp.FuncInput().Sum(nil))

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

	resp.ParentStateRootHash = firstParent.Hex()
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
			L2MessageServiceAddr:  types.EthAddress(rsp.L2BridgeAddress),
			ChainID:               uint64(rsp.ChainID),
			FinalBlockTimestamp:   lastBlock.TimeStamp,
			FinalBlockNumber:      uint64(rsp.FirstBlockNumber + len(rsp.BlocksData) - 1),
			InitialBlockTimestamp: firstBlock.TimeStamp,
			InitialBlockNumber:    uint64(rsp.FirstBlockNumber),
			DataChecksum:          rsp.ExecDataChecksum,
			L2MessageHashes:       types.AsByteArrSlice(rsp.AllL2L1MessageHashes),
			InitialStateRootHash:  types.Bytes32FromHex(rsp.ParentStateRootHash),
			FinalStateRootHash:    lastBlock.RootHash,
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
			ExecTracesFPath: path.Join(cfg.Execution.ConflatedTracesDir, req.ConflatedExecutionTracesFile),
			SMTraces:        req.StateManagerTraces(),
			TxSignatures:    txSignatures,
			TxHashes:        txHashes,
			L2BridgeAddress: cfg.Layer2.MsgSvcContract,
			ChainID:         cfg.Layer2.ChainID,
		},
		FuncInp: rsp.FuncInput(),
	}
}

// mimcHashLooselyPacked hashes the input stream b using the MiMC hash function
// encoding each slice of 31 bytes into a field element separately.
func mimcHashLooselyPacked(b []byte) types.Bytes32 {
	var buf [32]byte
	gnarkutil.ChecksumLooselyPackedBytes(b, buf[:], mimc.NewMiMC())
	return types.AsBytes32(buf[:])
}

// verifies the arithmetization version used to generate the trace file against the list of versions
// specified by the constraints in the file path.
func checkArithmetizationVersion(traceFileName, tracesEngineVersion string, constraintsVersions []string) error {
	logrus.Info("Verifying the arithmetization version for generating the trace file is supported by the constraints version")
	traceFileVersion, err := validateAndExtractVersion(traceFileName)
	if err != nil {
		return err
	}

	if strings.Compare(traceFileVersion, tracesEngineVersion) != 0 {
		return fmt.Errorf("version specified in the conflated trace file: %s does not match with the trace engine version: %s", traceFileVersion, tracesEngineVersion)
	}

	for _, version := range constraintsVersions {
		if version != "" && strings.Compare(traceFileVersion, version) == 0 && strings.Compare(tracesEngineVersion, version) == 0 {
			return nil
		}
	}
	return fmt.Errorf("unsupported arithmetization version:%s found in the conflated trace file: %s", traceFileVersion, traceFileName)
}

func validateAndExtractVersion(traceFileName string) (string, error) {
	logrus.Info("Validating and extracting the version from conflated trace files")
	// Define the regex pattern with a capturing group for the version part
	// The pattern accounts for an optional directory path
	traceFilePattern := `^(?:.*/)?\d+-\d+\.conflated\.(v\d+\.\d+\.\d+-[^.]+)\.lt$`
	re := regexp.MustCompile(traceFilePattern)

	// Check if the file name matches the pattern and extract the version part
	matches := re.FindStringSubmatch(traceFileName)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("conflated trace file: %s not in the appropriate format or version not found", traceFileName)
}
