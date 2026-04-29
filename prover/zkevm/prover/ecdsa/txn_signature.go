package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// TxSignature is responsible for assigning the relevant columns for transaction-Hash,
// and checking their consistency with the data coming from rlp_txn.
//
// columns for transaction-hash are native columns,
//
// columns for rlp-txn lives on the arithmetization side.
type TxSignature struct {
	Inputs   *txSignatureInputs
	TxHash   limbs.Uint256Le
	IsTxHash ifaces.Column

	// Provider for keccak, Provider contains the inputs and outputs of keccak hash.
	Provider generic.GenericByteModule
}

type txSignatureInputs struct {
	RlpTxn generic.GenDataModule
	Ac     *Antichamber
}

func newTxSignatures(comp *wizard.CompiledIOP, inp txSignatureInputs) *TxSignature {
	var createCol = createColFn(comp, NAME_TXSIGNATURE, inp.Ac.Size)

	var res = &TxSignature{
		IsTxHash: createCol("TX_IS_HASH_HI"),
		Inputs:   &inp,
		TxHash:   limbs.NewUint256Le(comp, NAME_TXSIGNATURE+"_TX_HASH", inp.Ac.Size),
	}

	commonconstraints.MustBeBinary(comp, res.IsTxHash)

	// isTxHash = 1 if isFeching = 1 and Source = 1
	comp.InsertGlobal(0, ifaces.QueryIDf("IS_TX_HASH"),
		sym.Mul(inp.Ac.IsFetching, inp.Ac.Source,
			sym.Sub(1, res.IsTxHash),
		),
	)

	// txHashHi remains the same between two fetchings.
	limbs.NewGlobal(comp, ifaces.QueryID("txHash_REMAIN_SAME"),
		sym.Mul(inp.Ac.IsActive,
			sym.Sub(1, inp.Ac.IsFetching),
			sym.Sub(res.TxHash, limbs.Shift(res.TxHash.AsDynSize(), -1))))

	res.Provider = res.GetProvider(comp, inp.RlpTxn)

	return res
}

// It builds a provider from rlp-txn (as hash input) and native columns of TxSignature (as hash output)
// the consistency check is then deferred to the keccak module.
func (txn *TxSignature) GetProvider(comp *wizard.CompiledIOP, rlpTxn generic.GenDataModule) generic.GenericByteModule {
	provider := generic.GenericByteModule{}

	// pass rlp-txn as DataModule.
	provider.Data = rlpTxn

	// generate infoModule from native columns
	provider.Info = txn.buildInfoModule()

	return provider
}

// it builds an infoModule from native columns
func (txn *TxSignature) buildInfoModule() generic.GenInfoModule {
	txHashHi, txHashLo := txn.TxHash.SplitOnBit(128)
	info := generic.GenInfoModule{
		HashHi:   txHashHi.ToBigEndianLimbs().AssertUint128(),
		HashLo:   txHashLo.ToBigEndianLimbs().AssertUint128(),
		IsHashHi: txn.IsTxHash,
		IsHashLo: txn.IsTxHash,
	}
	return info
}

// it assign the native columns
func (txn *TxSignature) assignTxSignature(run *wizard.ProverRuntime, nbActualEcRecover int) {

	var (
		nbEcRecover = nbActualEcRecover
		n           = startingRowOfTxnSignature(nbEcRecover)
		permTrace   = keccak.GenerateTrace(txn.Inputs.RlpTxn.ScanStreams(run))
		isTxHash    = common.NewVectorBuilder(txn.IsTxHash)
		hashColumns = limbs.NewVectorBuilder(txn.TxHash.AsDynSize())
	)

	hashColumns.PushSeqOfZeroes(n)
	isTxHash.PushSeqOfZeroes(n)

	for _, digest := range permTrace.HashOutPut {
		hashColumns.PushRepeatBytes(digest[:], nbRowsPerTxSign)
		isTxHash.PushOne()
		isTxHash.PushSeqOfZeroes(nbRowsPerTxSign - 1)
	}

	isTxHash.PadAndAssign(run)
	hashColumns.PadAndAssignZero(run)
}

func startingRowOfTxnSignature(nbEcRecover int) int {
	return nbEcRecover * nbRowsPerEcRec
}
