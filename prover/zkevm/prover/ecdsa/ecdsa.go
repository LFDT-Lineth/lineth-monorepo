package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

type EcdsaZkEvm struct {
	Ant *antichamber
}

func NewEcdsaZkEvm(
	comp *wizard.CompiledIOP,
	settings *Settings,
) *EcdsaZkEvm {
	return &EcdsaZkEvm{
		Ant: newAntichamber(
			comp,
			&antichamberInput{
				Settings:     settings,
				EcSource:     getEcdataArithmetization(comp),
				TxSource:     getTxnDataArithmetization(comp),
				RlpTxn:       getRlpTxnArithmetization(comp),
				PlonkOptions: []query.PlonkOption{query.PlonkRangeCheckOption(16, 6, true)},
			},
		),
	}
}

func (e *EcdsaZkEvm) Assign(run *wizard.ProverRuntime, txSig TxSignatureGetter, nbTx int) {
	e.Ant.assign(run, txSig, nbTx)
}

func (e *EcdsaZkEvm) GetProviders() []generic.GenericByteModule {
	return e.Ant.Providers
}

func getEcdataArithmetization(comp *wizard.CompiledIOP) *ecDataSource {
	return &ecDataSource{
		CsEcrecover: comp.Columns.GetHandle("ecdata.CIRCUIT_SELECTOR_ECRECOVER"),
		ID:          comp.Columns.GetHandle("ecdata.ID"),
		Limb:        comp.Columns.GetHandle("ecdata.LIMB"),
		SuccessBit:  comp.Columns.GetHandle("ecdata.SUCCESS_BIT"),
		Index:       comp.Columns.GetHandle("ecdata.INDEX"),
		IsData:      comp.Columns.GetHandle("ecdata.IS_ECRECOVER_DATA"),
		IsRes:       comp.Columns.GetHandle("ecdata.IS_ECRECOVER_RESULT"),
	}
}

func getTxnDataArithmetization(comp *wizard.CompiledIOP) *txnData {
	td := &txnData{
		FromHi: comp.Columns.GetHandle("txndata.FROM_HI"),
		FromLo: comp.Columns.GetHandle("txndata.FROM_LO"),
		Ct:     comp.Columns.GetHandle("txndata.CT"),
	}
	return td
}

func getRlpTxnArithmetization(comp *wizard.CompiledIOP) generic.GenDataModule {
	return generic.GenDataModule{
		HashNum: comp.Columns.GetHandle("rlptxn.ABS_TX_NUM"),
		Index:   comp.Columns.GetHandle("rlptxn.INDEX_LX"),
		Limb:    comp.Columns.GetHandle("rlptxn.LIMB"),
		NBytes:  comp.Columns.GetHandle("rlptxn.nBYTES"),
		ToHash:  comp.Columns.GetHandle("rlptxn.TO_HASH_BY_PROVER"),
	}
}
