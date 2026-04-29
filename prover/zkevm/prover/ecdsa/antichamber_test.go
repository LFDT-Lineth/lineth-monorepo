package ecdsa

import (
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic/testdata"
)

func TestAntichamber(t *testing.T) {
	f, err := os.Open("testdata/antichamber.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	var ac *Antichamber
	var ecSrc *ecDataSource
	var txSrc *txnData
	limits := &Settings{
		MaxNbEcRecover:     4,
		MaxNbTx:            2,
		NbInputInstance:    6,
		NbCircuitInstances: 1,
	}
	var rlpTxn generic.GenDataModule

	// to cover edge-cases, leaves some rows empty
	c := testCaseAntiChamber
	// random value for testing edge cases
	nbRowsPerTxInTxnData := 3
	cmp := wizard.Compile(
		func(b *wizard.Builder) {
			comp := b.CompiledIOP
			// declare rlp_txn module
			rlpTxn = testdata.CreateGenDataModule(comp, "TXN_RLP", 32, common.NbLimbU128)

			// declar txn_data module
			txSrc = commitTxnData(comp, limits, nbRowsPerTxInTxnData)

			// declare ecdata (ecRecover)
			ecSrc = &ecDataSource{
				CsEcrecover: ct.GetCommit(b, "EC_DATA_CS_ECRECOVER"),
				ID:          ct.GetCommit(b, "EC_DATA_ID"),
				SuccessBit:  ct.GetCommit(b, "EC_DATA_SUCCESS_BIT"),
				Index:       ct.GetCommit(b, "EC_DATA_INDEX"),
				IsData:      ct.GetCommit(b, "EC_DATA_IS_DATA"),
				IsRes:       ct.GetCommit(b, "EC_DATA_IS_RES"),
				Limb:        ct.GetLimbsLe(b, "EC_DATA_LIMB", common.NbLimbU128).AssertUint128(),
			}

			ac = newAntichamber(
				b.CompiledIOP,
				&antichamberInput{
					EcSource:     ecSrc,
					TxSource:     txSrc,
					RlpTxn:       rlpTxn,
					PlonkOptions: []query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)},
					Settings:     limits,
					WithCircuit:  true,
				},
			)
		},
		dummy.Compile,
	)
	proof := wizard.Prove(cmp,
		func(run *wizard.ProverRuntime) {

			// assign data to rlp_txn module
			testdata.GenerateAndAssignGenDataModule(run, &rlpTxn, c.HashNum, c.ToHash, true)
			trace := keccak.GenerateTrace(rlpTxn.ScanStreams(run))

			// assign txn_data module from pk
			txSrc.assignTxnDataFromPK(run, ac, trace.HashOutPut, nbRowsPerTxInTxnData)

			ct.Assign(run,
				ecSrc.CsEcrecover,
				ecSrc.ID,
				ecSrc.Limb,
				ecSrc.SuccessBit,
				ecSrc.Index,
				ecSrc.IsData,
				ecSrc.IsRes,
			)
			ac.assign(run, DummyTxSignatureGetter, limits.MaxNbTx)
		})

	if err := wizard.Verify(cmp, proof); err != nil {
		t.Fatal("proof failed", err)
	}
}

var testCaseAntiChamber = struct {
	HashNum []int
	ToHash  []int
}{
	HashNum: []int{1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2},
	ToHash:  []int{1, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1, 1},
}
