package limitless

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	execLimitless "github.com/consensys/linea-monorepo/prover/backend/execution/limitless"
	backendInvalidity "github.com/consensys/linea-monorepo/prover/backend/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/config"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
)

// Prove runs the limitless proving pipeline for the invalidity bad-precompile
// / too-many-logs circuit. It reuses the execution limitless distributed
// pipeline (bootstrapper → GL/LPP → conglomeration) and then generates the
// invalidity-specific outer proof.
func Prove(cfg *config.Config, req *backendInvalidity.Request) (*backendInvalidity.Response, error) {

	profiling.SetMonitorParams(cfg)

	// Initialize JSONL performance event logger
	plog := execLimitless.NewPerfLogger()
	defer plog.Close()

	exit.SetIssueHandlingMode(exit.ExitOnUnsatisfiedConstraint | exit.ExitOnMissingTraceFile)

	logrus.Infof("Starting limitless invalidity proof for type: %s", req.InvalidityType)

	if err := req.Validate(config.ProverModeLimitless); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	tx, err := ethereum.RlpDecodeWithSignature(req.RlpEncodedTx)
	if err != nil {
		return nil, fmt.Errorf("could not decode the RlpEncodedTx: %w", err)
	}

	funcInput := backendInvalidity.FuncInput(req, cfg)

	// Build the zkEVM witness for the simulated execution
	txHash := ethereum.GetTxHash(tx)
	txSignature := ethereum.GetJsonSignature(tx)
	execDataCommit := public_input.ComputeExecutionDataMultiCommitment(nil)
	// Invalidity blocks don't really exist, so use zero hashes.
	zeroHashes := make([]linTypes.FullBytes32, len(req.ZkStateMerkleProof))
	zkevmWitness := &zkevm.Witness{
		ExecTracesFPath:        cfg.Execution.ConflatedTracesDir + "/" + req.ConflatedExecutionTracesFile,
		SMTraces:               req.ZkStateMerkleProof,
		TxSignatures:           []ethereum.Signature{txSignature},
		TxHashes:               [][32]byte{txHash},
		L2BridgeAddress:        cfg.Layer2.MsgSvcContract,
		ChainID:                cfg.Layer2.ChainID,
		BaseFee:                cfg.Layer2.BaseFee,
		CoinBase:               linTypes.EthAddress(cfg.Layer2.CoinBase),
		BlockHashList:          zeroHashes,
		ExecDataSchwarzZipfelX: execDataCommit.X,
		ExecData:               execDataCommit.Data,
	}

	rsp := &backendInvalidity.Response{
		Transaction:                      tx,
		Signer:                           funcInput.FromAddress,
		TxHash:                           utils.HexEncodeToString(funcInput.TxHash[:]),
		RLPEncodedTx:                     req.RlpEncodedTx,
		ForcedTransactionNumber:          req.ForcedTransactionNumber,
		PrevFtxRollingHash:               req.PrevFtxRollingHash,
		DeadlineBlockHeight:              req.DeadlineBlockHeight,
		InvalidityType:                   req.InvalidityType,
		ZkParentStateRootHash:            req.ZkParentStateRootHash,
		SimulatedExecutionBlockNumber:    req.SimulatedExecutionBlockNumber,
		SimulatedExecutionBlockTimestamp: req.SimulatedExecutionBlockTimestamp,
		ChainID:                          cfg.Layer2.ChainID,
		BaseFee:                          cfg.Layer2.BaseFee,
		CoinBase:                         linTypes.EthAddress(cfg.Layer2.CoinBase),
		L2BridgeAddress:                  linTypes.EthAddress(cfg.Layer2.MsgSvcContract),
		ProverVersion:                    cfg.Version,
		PublicInput:                      linTypes.Bls12377Fr(funcInput.Sum(nil)),
		FtxRollingHash:                   funcInput.FtxRollingHash,
		ProverMode:                       cfg.Invalidity.ProverMode,
	}

	if cfg.Invalidity.LimitlessWithDebug {
		logrus.Info("Running limitless invalidity prover in debug mode")
		limitlessZkEVM := zkevm.NewLimitlessDebugZkEVM(cfg)
		limitlessZkEVM.RunDebug(cfg, zkevmWitness)
		return rsp, nil
	}

	// -- 1-4. Run the distributed pipeline: bootstrapper → GL/LPP segment
	// proofs → shared randomness → hierarchical conglomeration.
	pipeline, err := execLimitless.RunDistributedPipeline(cfg, zkevmWitness, plog)
	if err != nil {
		return nil, fmt.Errorf("distributed pipeline failed: %w", err)
	}

	// -- 5. Outer proof
	var (
		setup       circuits.Setup
		errSetup    error
		chSetupDone = make(chan struct{})
	)
	go func() {
		logrus.Infof("Loading setup - circuitID: %s", circuits.InvalidityPrecompileLogsLimitlessCircuitID)
		setup, errSetup = circuits.LoadSetup(cfg, circuits.InvalidityPrecompileLogsLimitlessCircuitID)
		close(chSetupDone)
	}()

	<-chSetupDone
	if errSetup != nil {
		utils.Panic("could not load setup: %v", errSetup)
	}

	fromAddress := ethereum.GetFrom(tx)

	c := invalidity.CircuitInvalidity{}
	assigningInputs := invalidity.AssigningInputs{
		RlpEncodedTx:     ethereum.EncodeTxForSigning(tx),
		Transaction:      tx,
		FromAddress:      common.Address(fromAddress),
		InvalidityType:   req.InvalidityType,
		FuncInputs:       *funcInput,
		ZkEvmComp:        pipeline.Cong.RecursionCompBLS,
		ZkEvmWizardProof: pipeline.FinalProof.GetOuterProofInput(),
	}

	serializedProof := c.MakeProof(setup, assigningInputs)

	rsp.Proof = serializedProof
	rsp.VerifyingKeyShaSum = setup.VerifyingKeyDigest()

	return rsp, nil
}
