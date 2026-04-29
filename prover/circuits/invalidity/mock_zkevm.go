package invalidity

import (
	"fmt"
	"math/rand/v2"

	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	zkevmcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	invalidityPI "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
)

// LimitlessInputs holds the values needed to mock the conglomeration public
// inputs that CheckLimitlessConglomerationCompletion reads.
type LimitlessInputs struct {
	CongloVK     [2]field.Octuplet
	VKMerkleRoot field.Octuplet
}

// CreateLimbs32Bytes splits a 32-byte hash into 16 big-endian 2-byte limbs.
func CreateLimbs32Bytes(b [32]byte) [16]field.Element {
	var limbs [16]field.Element
	for i := 0; i < 16; i++ {
		limbs[i].SetBytes([]byte{b[i*2], b[i*2+1]})
	}
	return limbs
}

// CreateLimbs20Bytes splits a 20-byte address into 10 big-endian 2-byte limbs.
func CreateLimbs20Bytes(b [20]byte) [10]field.Element {
	var limbs [10]field.Element
	for i := 0; i < 10; i++ {
		limbs[i].SetBytes([]byte{b[i*2], b[i*2+1]})
	}
	return limbs
}

func Create8LimbsFromInt(n uint64) [8]field.Element {
	var limbs [8]field.Element
	bytes := common.SplitBigEndianUint64(n) // 4 big-endian 2-byte chunks
	for i := 0; i < 4; i++ {
		limbs[i+4].SetBytes(bytes[i])
	}
	return limbs
}

// MockZkEvmPI creates a minimal ZkEvm with limb-based public inputs for BadPrecompileCircuit.
// It registers columns and public inputs matching both the InvalidityPIExtractor
// and the execution FunctionalInputExtractor layouts, then proves them with the
// provided input values.
//
// When limitless is non-nil, the mock also registers the conglomeration public
// inputs that CheckLimitlessConglomerationCompletion reads, with values that
// satisfy all its constraints.
func MockZkEvmPI(rng *rand.Rand, in invalidityPI.Inputs, limitless *LimitlessInputs) (*wizard.CompiledIOP, wizard.Proof) {
	define := func(b *wizard.Builder) {
		comp := b.CompiledIOP

		registerLimbCols := func(baseName string, n int) []ifaces.Column {
			cols := make([]ifaces.Column, n)
			for i := range n {
				cols[i] = comp.InsertProof(0, ifaces.ColIDf("%s_%d", baseName, i), 1, true)
			}
			return cols
		}
		registerLimbPIs := func(cols []ifaces.Column, baseName string) []wizard.PublicInput {
			pis := make([]wizard.PublicInput, len(cols))
			for i, col := range cols {
				pis[i] = comp.InsertPublicInput(
					fmt.Sprintf("%s_%d", baseName, i),
					accessors.NewFromPublicColumn(col, 0),
				)
			}
			return pis
		}

		// --- Invalidity PI columns ---
		extractor := &invalidityPI.InvalidityPIExtractor{}

		txHashCols := registerLimbCols("TX_HASH", 16)
		copy(extractor.TxHash[:], registerLimbPIs(txHashCols, "TxHash_BE"))

		fromAddrCols := registerLimbCols("FROM_ADDRESS", 10)
		copy(extractor.FromAddress[:], registerLimbPIs(fromAddrCols, "From_BE"))

		hasBadPrecompileCol := comp.InsertProof(0, ifaces.ColID("HASH_BAD_PRECOMPILE_COL"), 1, true)
		nbL2LogsCol := comp.InsertProof(0, ifaces.ColID("NB_L2_LOGS_COL"), 1, true)
		extractor.HasBadPrecompile = comp.InsertPublicInput("HasBadPrecompile", accessors.NewFromPublicColumn(hasBadPrecompileCol, 0))
		extractor.NbL2Logs = comp.InsertPublicInput("NbL2Logs", accessors.NewFromPublicColumn(nbL2LogsCol, 0))
		comp.ExtraData[invalidityPI.InvalidityPIExtractorMetadata] = extractor

		// --- Execution PI columns ---
		execExtractor := &publicInput.FunctionalInputExtractor{}

		stateRootCols := registerLimbCols("EXEC_STATE_ROOT", zkevmcommon.NbElemPerHash)
		copy(execExtractor.InitialStateRootHash[:], registerLimbPIs(stateRootCols, "InitialStateRootHash"))

		coinBaseCols := registerLimbCols("EXEC_COINBASE", zkevmcommon.NbLimbEthAddress)
		copy(execExtractor.CoinBase[:], registerLimbPIs(coinBaseCols, "CoinBase"))

		baseFeeCols := registerLimbCols("EXEC_BASEFEE", zkevmcommon.NbLimbU128)
		copy(execExtractor.BaseFee[:], registerLimbPIs(baseFeeCols, "BaseFee"))

		chainIDCols := registerLimbCols("EXEC_CHAINID", zkevmcommon.NbLimbU256)
		copy(execExtractor.ChainID[:], registerLimbPIs(chainIDCols, "ChainID"))

		l2MsgSvcCols := registerLimbCols("EXEC_L2MSGSVC", zkevmcommon.NbLimbEthAddress)
		copy(execExtractor.L2MessageServiceAddr[:], registerLimbPIs(l2MsgSvcCols, "L2MessageServiceAddr"))

		blockTsCols := registerLimbCols("EXEC_BLOCKTIMESTAMP", zkevmcommon.NbLimbU128)
		copy(execExtractor.InitialBlockTimestamp[:], registerLimbPIs(blockTsCols, "InitialBlockTimestamp"))

		blockNumCols := registerLimbCols("EXEC_BLOCKNUM", zkevmcommon.NbLimbU48)
		copy(execExtractor.InitialBlockNumber[:], registerLimbPIs(blockNumCols, "InitialBlockNumber"))

		comp.ExtraData[publicInput.PublicInputExtractorMetadata] = execExtractor

		// --- Conglomeration PIs (limitless mode only) ---
		if limitless != nil {
			registerSinglePI(comp, distributed.TargetNbSegmentPublicInputBase+"_0")
			registerSinglePI(comp, distributed.SegmentCountGLPublicInputBase+"_0")
			registerSinglePI(comp, distributed.SegmentCountLPPPublicInputBase+"_0")

			for i := 0; i < multisethashing.MSetHashSize; i++ {
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.GeneralMultiSetPublicInputBase, i))
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.SharedRandomnessMultiSetPublicInputBase, i))
			}

			for i := 0; i < 8; i++ {
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.InitialRandomnessPublicInput, i))
			}
			registerSinglePI(comp, distributed.LogDerivativeSumPublicInput)
			registerSinglePI(comp, distributed.GrandProductPublicInput)
			registerSinglePI(comp, distributed.HornerPublicInput)
			for i := 0; i < 8; i++ {
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.VerifyingKeyPublicInput, i))
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.VerifyingKey2PublicInput, i))
				registerSinglePI(comp, fmt.Sprintf("%s_%d", distributed.VerifyingKeyMerkleRootPublicInput, i))
			}
		}
	}

	comp := wizard.Compile(define, dummy.Compile)

	prove := func(run *wizard.ProverRuntime) {
		assignLimbCols := func(baseName string, vals []field.Element) {
			for i, v := range vals {
				run.AssignColumn(ifaces.ColIDf("%s_%d", baseName, i), smartvectors.NewConstant(v, 1))
			}
		}

		// --- Assign invalidity PI columns ---
		assignLimbCols("TX_HASH", in.TxHashLimbs[:])
		assignLimbCols("FROM_ADDRESS", in.FromLimbs[:])

		var hasBadPrecompileVal field.Element
		if in.CaseInputs.HasBadPrecompile {
			one := field.One()
			hasBadPrecompileVal = field.PseudoRand(rng)
			hasBadPrecompileVal.Add(&hasBadPrecompileVal, &one)
		} else {
			hasBadPrecompileVal = field.Zero()
		}
		run.AssignColumn(ifaces.ColID("HASH_BAD_PRECOMPILE_COL"), smartvectors.NewConstant(hasBadPrecompileVal, 1))
		run.AssignColumn(ifaces.ColID("NB_L2_LOGS_COL"), smartvectors.NewConstant(field.NewElement(uint64(in.CaseInputs.NumL2Logs)), 1))

		// --- Assign execution PI columns ---
		assignLimbCols("EXEC_STATE_ROOT", in.StateRootHash[:])
		assignLimbCols("EXEC_COINBASE", in.CoinBase[:])
		assignLimbCols("EXEC_BASEFEE", in.BaseFee[:])
		assignLimbCols("EXEC_CHAINID", in.ChainID[:])
		assignLimbCols("EXEC_L2MSGSVC", in.L2MessageServiceAddr[:])
		assignLimbCols("EXEC_BLOCKTIMESTAMP", in.InitialBlockTimestamp[:])
		assignLimbCols("EXEC_BLOCKNUM", in.InitialBlockNumber[:])

		// --- Assign conglomeration PIs (limitless mode only) ---
		if limitless != nil {
			segCount := field.NewElement(3)
			assignSinglePI(run, distributed.TargetNbSegmentPublicInputBase+"_0", segCount)
			assignSinglePI(run, distributed.SegmentCountGLPublicInputBase+"_0", segCount)
			assignSinglePI(run, distributed.SegmentCountLPPPublicInputBase+"_0", segCount)

			for i := 0; i < multisethashing.MSetHashSize; i++ {
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.GeneralMultiSetPublicInputBase, i), field.Zero())
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.SharedRandomnessMultiSetPublicInputBase, i), field.Zero())
			}

			// initRandomness = Poseidon2(sharedRandMSet) over KoalaBear.
			zeros := make([]field.Element, multisethashing.MSetHashSize)
			initRand := poseidon2.HashVec(zeros...)
			for i := 0; i < 8; i++ {
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.InitialRandomnessPublicInput, i), initRand[i])
			}

			assignSinglePI(run, distributed.LogDerivativeSumPublicInput, field.Zero())
			assignSinglePI(run, distributed.GrandProductPublicInput, field.One())
			assignSinglePI(run, distributed.HornerPublicInput, field.Zero())
			for i := 0; i < 8; i++ {
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.VerifyingKeyPublicInput, i), limitless.CongloVK[0][i])
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.VerifyingKey2PublicInput, i), limitless.CongloVK[1][i])
				assignSinglePI(run, fmt.Sprintf("%s_%d", distributed.VerifyingKeyMerkleRootPublicInput, i), limitless.VKMerkleRoot[i])
			}
		}
	}

	proof := wizard.Prove(comp, prove)
	return comp, proof
}

// registerSinglePI registers a single base-field proof column and corresponding
// public input with the given name.
func registerSinglePI(comp *wizard.CompiledIOP, name string) {
	col := comp.InsertProof(0, ifaces.ColID(name+"_PI_COLUMN"), 1, true)
	comp.InsertPublicInput(name, accessors.NewFromPublicColumn(col, 0))
}

// assignSinglePI assigns a value to a single PI column registered by registerSinglePI.
func assignSinglePI(run *wizard.ProverRuntime, name string, val field.Element) {
	run.AssignColumn(ifaces.ColID(name+"_PI_COLUMN"), smartvectors.NewConstant(val, 1))
}
