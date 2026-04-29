package invalidity

import (
	"reflect"

	"github.com/consensys/gnark/frontend"
	execCirc "github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	invalidity "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
)

const MAX_L2_LOGS = 16

// BadPrecompileCircuit defines the circuit for the transaction with a bad precompile.
type BadPrecompileCircuit struct {
	//  simulated execution context.
	ExecutionCtx ExecutionCtx

	// Derived from invalidity PI extractor (gnark:"-" = no wires, set during Define)
	txHash           [2]frontend.Variable `gnark:"-"`
	fromAddress      frontend.Variable    `gnark:"-"`
	hasBadPrecompile frontend.Variable    `gnark:"-"`
	NbL2Logs         frontend.Variable    `gnark:"-"`

	// Derived from execution PI extractor (gnark:"-" = no wires, set during Define)
	stateRootHash      [2]frontend.Variable `gnark:"-"`
	initialBlockNumber frontend.Variable    `gnark:"-"`

	// Witness fields, cross-checked against extraction
	// when the extracted value is non-zero.
	CoinBase              frontend.Variable
	BaseFee               frontend.Variable
	ChainID               frontend.Variable
	L2MessageServiceAddr  frontend.Variable
	InitialBlockTimestamp frontend.Variable
	// Witness fields not constrained by wizard, but flowing into public input hash
	ToAddress      frontend.Variable
	ToIsFiltered   frontend.Variable
	FromIsFiltered frontend.Variable
	// Invalidity type: 2 = BadPrecompile, 3 = TooManyLogs
	InvalidityType frontend.Variable
}

// ExecutionCtx is the simulated execution for the bad precompile/log circuit.
type ExecutionCtx struct {
	// LimitlessMode is set to true if the outer proof is generated for the
	// limitless prover mode.
	LimitlessMode bool `gnark:"-"`
	// CongloVK is used when the [LimitlessMode] is on and is helps checking
	// the validity of the inner-proofs verification-key public input.
	CongloVK [2]field.Octuplet
	// VKMerkleRoot is used when the [LimitlessMode] is on and is helps checking
	// the validity of the inner-proofs verification-key merkle root public
	// input.
	VKMerkleRoot field.Octuplet
	// The wizard verifier circuit
	WizardVerifier wizard.VerifierCircuit `gnark:",secret"`
}

func (circuit *BadPrecompileCircuit) Allocate(config Config) {

	wverifier := wizard.AllocateWizardCircuit(
		config.ZkEvmComp,
		config.ZkEvmComp.NumRounds(),
		true,
	)

	circuit.ExecutionCtx.WizardVerifier = *wverifier
}

func (circuit *BadPrecompileCircuit) Define(api frontend.API) error {

	circuit.ExecutionCtx.WizardVerifier.BLSFS = fiatshamir.NewGnarkFSBLS12377(api)
	circuit.ExecutionCtx.WizardVerifier.Verify(api)

	pie := getInvalidityPIExtractor(&circuit.ExecutionCtx.WizardVerifier)
	execPie := execCirc.GetPublicInputExtractor(&circuit.ExecutionCtx.WizardVerifier)
	circuit.checkPublicInputs(api, &circuit.ExecutionCtx.WizardVerifier, pie, execPie)

	if circuit.ExecutionCtx.LimitlessMode {
		execCircuit := execCirc.CircuitExecution{
			LimitlessMode:  true,
			CongloVK:       circuit.ExecutionCtx.CongloVK,
			VKMerkleRoot:   circuit.ExecutionCtx.VKMerkleRoot,
			WizardVerifier: circuit.ExecutionCtx.WizardVerifier,
		}
		execCircuit.CheckLimitlessConglomerationCompletion(api)
	}

	// check that invalidity type is valid, it should be 2 or 3
	// binaryType = 0 when BadPrecompile (type 2), binaryType = 1 when TooManyLogs (type 3)
	binaryType := api.Sub(circuit.InvalidityType, 2)
	api.AssertIsBoolean(binaryType)

	// check that hasBadPrecompile is non-zero, if invalidityType == 2 (BadPrecompile)
	// When binaryType=0: (1-0)*hasBadPrecompile + 0 = hasBadPrecompile != 0
	// When binaryType=1: (1-1)*hasBadPrecompile + 1 = 1 != 0  (always passes)
	api.AssertIsDifferent(
		api.Add(
			api.Mul(api.Sub(1, binaryType), circuit.hasBadPrecompile),
			binaryType),
		0)

	// check that NbL2Logs is greater than MAX_L2_LOGS, if invalidityType == 3 (TooManyLogs)
	// When binaryType=1: 17 <= 1*NbL2Logs + 0 = NbL2Logs
	// When binaryType=0: 17 <= 0*NbL2Logs + 17 = 17  (always passes)
	api.AssertIsLessOrEqual(MAX_L2_LOGS+1,
		api.Add(
			api.Mul(binaryType, circuit.NbL2Logs),
			api.Mul(api.Sub(1, binaryType), MAX_L2_LOGS+1),
		),
	)

	return nil
}

// checkPublicInputs extracts public inputs from both the invalidity and
// execution PI wizard extractors. Both modules always run together.
func (circuit *BadPrecompileCircuit) checkPublicInputs(api frontend.API, wvc *wizard.VerifierCircuit, pie *invalidity.InvalidityPIExtractor, execPie *publicInput.FunctionalInputExtractor) {
	// From invalidity PI extractor
	extrTxHashLimbs := execCirc.GetPublicInputArr(api, wvc, pie.TxHash[:])
	circuit.txHash[0] = combine16BitLimbs(api, extrTxHashLimbs[:8])
	circuit.txHash[1] = combine16BitLimbs(api, extrTxHashLimbs[8:])

	extrFromAddressLimbs := execCirc.GetPublicInputArr(api, wvc, pie.FromAddress[:])
	circuit.fromAddress = combine16BitLimbs(api, extrFromAddressLimbs)

	circuit.hasBadPrecompile = execCirc.GetPublicInput(api, wvc, pie.HasBadPrecompile)
	circuit.NbL2Logs = execCirc.GetPublicInput(api, wvc, pie.NbL2Logs)

	// From execution PI extractor
	extrStateRootHashWords := execCirc.GetPublicInputArr(api, wvc, execPie.InitialStateRootHash[:])
	circuit.stateRootHash[0] = combine32BitLimbs(api, extrStateRootHashWords[:4])
	circuit.stateRootHash[1] = combine32BitLimbs(api, extrStateRootHashWords[4:])

	extrCoinBase := combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.CoinBase[:]))
	extrBaseFee := combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.BaseFee[:]))
	extrChainID := combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.ChainID[:]))
	extrL2MsgServiceAddr := combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.L2MessageServiceAddr[:]))
	extrBlockTimestamp := combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.InitialBlockTimestamp[:]))

	// Cross-check witness values against extraction when extracted is non-zero.
	// The extraction may return zero if the relevant opcode was never called.
	internal.AssertEqualIf(api, extrCoinBase, circuit.CoinBase, extrCoinBase)
	internal.AssertEqualIf(api, extrBaseFee, circuit.BaseFee, extrBaseFee)
	internal.AssertEqualIf(api, extrChainID, circuit.ChainID, extrChainID)
	internal.AssertEqualIf(api, extrL2MsgServiceAddr, circuit.L2MessageServiceAddr, extrL2MsgServiceAddr)
	internal.AssertEqualIf(api, extrBlockTimestamp, circuit.InitialBlockTimestamp, extrBlockTimestamp)

	circuit.initialBlockNumber = combine16BitLimbs(api, execCirc.GetPublicInputArr(api, wvc, execPie.InitialBlockNumber[:]))
}

func getInvalidityPIExtractor(wvc *wizard.VerifierCircuit) *invalidity.InvalidityPIExtractor {
	extraData, extraDataFound := wvc.Spec.ExtraData[invalidity.InvalidityPIExtractorMetadata]
	if !extraDataFound {
		panic("invalidity PI extractor not found")
	}
	pie, ok := extraData.(*invalidity.InvalidityPIExtractor)
	if !ok {
		panic("invalidity PI extractor not of the right type: " + reflect.TypeOf(extraData).String())
	}
	return pie
}

// Assign assigns the inputs to the circuit
func (circuit *BadPrecompileCircuit) Assign(assi AssigningInputs) {

	circuit.ExecutionCtx.WizardVerifier = *wizard.AssignVerifierCircuit(assi.ZkEvmComp, assi.ZkEvmWizardProof, assi.ZkEvmComp.NumRounds(), true)

	circuit.InvalidityType = int(assi.InvalidityType) // cast to int for gnark witness

	circuit.ToAddress = assi.Transaction.To()[:]
	circuit.ToIsFiltered = 0
	circuit.FromIsFiltered = 0

	circuit.CoinBase = assi.FuncInputs.CoinBase[:]
	circuit.BaseFee = assi.FuncInputs.BaseFee
	circuit.ChainID = assi.FuncInputs.ChainID
	circuit.L2MessageServiceAddr = assi.FuncInputs.L2MessageServiceAddr[:]
	circuit.InitialBlockTimestamp = assi.FuncInputs.SimulatedBlockTimestamp
}

func (c *BadPrecompileCircuit) FunctionalPIQGnark() FunctionalPIQGnark {
	return FunctionalPIQGnark{
		FromAddress:             c.fromAddress,
		TxHash:                  c.txHash,
		StateRootHash:           c.stateRootHash,
		ToAddress:               c.ToAddress,
		ToIsFiltered:            c.ToIsFiltered,
		FromIsFiltered:          c.FromIsFiltered,
		CoinBase:                c.CoinBase,
		BaseFee:                 c.BaseFee,
		ChainID:                 c.ChainID,
		L2MessageServiceAddr:    c.L2MessageServiceAddr,
		SimulatedBlockTimestamp: c.InitialBlockTimestamp,
		SimulatedBlockNumber:    c.initialBlockNumber,
	}
}
