package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"golang.org/x/crypto/sha3"
)

func commitEcRecTxnData(comp *wizard.CompiledIOP, size1 int, size int, ac *antichamber) (td *txnData, ecRec *EcRecover) {
	td = &txnData{
		Ct:       comp.InsertCommit(0, ifaces.ColIDf("txn_data.CT"), size1, true),
		User:     comp.InsertCommit(0, ifaces.ColIDf("txn_data.USER"), size1, true),
		Selector: comp.InsertCommit(0, ifaces.ColIDf("txn_data.SELECTOR"), size1, true),
		From:     limbs.NewUint256Le(comp, "txn_data.From", size1),
	}

	ecRec = &EcRecover{
		EcRecoverIsRes: comp.InsertCommit(0, ifaces.ColIDf("ECRECOVER_ISRES"), size, true),
		Limb:           limbs.NewUint128Le(comp, "ECRECOVER_LIMB", size),
	}

	ac.IsActive = comp.InsertCommit(0, "AntiChamber_IsActive", size, true)
	return td, ecRec
}

func AssignEcRecTxnData(
	run *wizard.ProverRuntime,
	gbm generic.GenDataModule,
	nbEcRec, nbTxS int,
	sizeTxnData, size int,
	td *txnData, ecRec *EcRecover,
	ac *antichamber,
) {

	var (
		nbRowsPerTxInTxnData = 9

		streams   = gbm.ScanStreams(run)
		permTrace = keccak.GenerateTrace(streams)

		isEcRecRes  = common.NewVectorBuilder(ecRec.EcRecoverIsRes)
		ecRecLimb   = limbs.NewVectorBuilder(ecRec.Limb.AsDynSize())
		txnCt       = common.NewVectorBuilder(td.Ct)
		txnUser     = common.NewVectorBuilder(td.User)
		txnSelector = common.NewVectorBuilder(td.Selector)
		txnFrom     = limbs.NewVectorBuilder(td.From.AsDynSize())
	)

	if nbEcRec+nbTxS != len(permTrace.HashOutPut) {
		utils.Panic("the number of generated hash %v should be %v + %v", len(permTrace.HashOutPut), nbEcRec, nbTxS)
	}

	for i := 0; i < nbTxS; i++ {
		for j := 0; j < nbRowsPerTxInTxnData; j++ {
			txnCt.PushInt(j)
			txnUser.PushOne()
			txnSelector.PushOne()
			txnFrom.PushBytes(permTrace.HashOutPut[nbEcRec+i][:])
		}
	}

	for i := 0; i < nbEcRec; i++ {

		hashRes := permTrace.HashOutPut[i]

		// Sanity-check that the heights of the builder is the expected one
		if currHeight := i * nbRowsPerEcRec; isEcRecRes.Height() != currHeight || ecRecLimb.Height() != currHeight {
			utils.Panic("isEcRecRes.Height() || ecRecLimb.Height() != %v, %v", isEcRecRes.Height(), ecRecLimb.Height())
		}

		// Pushing the hi part
		isEcRecRes.PushOne()
		ecRecLimb.PushLeftPaddedBytes(hashRes[12:16])

		// Pushing the lo part
		isEcRecRes.PushOne()
		ecRecLimb.PushBytes(hashRes[16:])

		// Fill the rest of the frame with zeroes
		isEcRecRes.PushSeqOfZeroes(nbRowsPerEcRec - 2)
		ecRecLimb.PushSeqOfZeroes(nbRowsPerEcRec - 2)
	}

	ecRecLimb.PadAndAssignZero(run)
	isEcRecRes.PadAndAssign(run)
	txnCt.PadAndAssign(run)
	txnUser.PadAndAssign(run)
	txnSelector.PadAndAssign(run)
	txnFrom.PadAndAssignZero(run)

	effectiveSize := nbEcRec*nbRowsPerEcRec + nbTxS*nbRowsPerTxSign
	isActive := vector.Repeat(field.One(), effectiveSize)
	run.AssignColumn(ac.IsActive.GetColID(), smartvectors.RightZeroPadded(isActive, size))
}

// it estimates the required number of number of keccakF.
func (l *Settings) nbKeccakF(nbKeccakFPerTxn int) int {
	return l.MaxNbTx*nbKeccakFPerTxn + l.MaxNbEcRecover
}

func (l *Settings) sizeTxnData(nbRowsPerTxInTxnData int) int {
	return utils.NextPowerOfTwo(l.MaxNbTx * nbRowsPerTxInTxnData)
}

// It receives a set of public keys, and assigns the txn_data
func (td *txnData) assignTxnDataFromPK(
	run *wizard.ProverRuntime,
	ac *antichamber,
	rlpTxnHashes [][32]byte,
	nbRowsPerTxInTxnData int,
) {

	var (
		hasher      = sha3.NewLegacyKeccak256()
		maxNbTx     = ac.Inputs.Settings.MaxNbTx
		txnCt       = common.NewVectorBuilder(td.Ct)
		txnUser     = common.NewVectorBuilder(td.User)
		txnSelector = common.NewVectorBuilder(td.Selector)
		txnFrom     = limbs.NewVectorBuilder(td.From.AsDynSize())
	)

	// compute the hash of public keys
	pkHash := make([][32]byte, 0, len(rlpTxnHashes))
	for i := range rlpTxnHashes {
		pk, _, _, _, err := generateDeterministicSignature(rlpTxnHashes[i][:])
		if err != nil {
			utils.Panic("error generating signature")
		}
		buf := pk.A.RawBytes()

		hasher.Write(buf[:])
		res := hasher.Sum(nil)
		hasher.Reset()
		pkHash = append(pkHash, [32]byte(res))
	}

	// now assign  txn_data from the hash results.
	// populate the CT, User, and Selector columns
	for i := 0; i < maxNbTx; i++ {
		for j := 0; j < nbRowsPerTxInTxnData; j++ {
			txnCt.PushInt(j)
			txnUser.PushOne()
			txnSelector.PushOne()
			if i < len(pkHash) {
				txnFrom.PushBytes(pkHash[i][:])
			} else {
				txnFrom.PushZero()
			}
		}
	}

	txnCt.PadAndAssign(run)
	txnUser.PadAndAssign(run)
	txnSelector.PadAndAssign(run)
	txnFrom.PadAndAssignZero(run)
}

// it commits to the txn_data
func commitTxnData(comp *wizard.CompiledIOP, limits *Settings, nbRowsPerTxInTxnData int) (td *txnData) {
	size := limits.sizeTxnData(nbRowsPerTxInTxnData)
	return &txnData{
		Ct:       comp.InsertCommit(0, ifaces.ColIDf("txn_data.CT"), size, true),
		User:     comp.InsertCommit(0, ifaces.ColIDf("txn_data.USER"), size, true),
		Selector: comp.InsertCommit(0, ifaces.ColIDf("txn_data.SELECTOR"), size, true),
		From:     limbs.NewUint256Le(comp, "txn_data.From", size),
	}
}
