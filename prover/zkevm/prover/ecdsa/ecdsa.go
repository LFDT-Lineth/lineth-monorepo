package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

type EcdsaZkEvm struct {
	Ant *Antichamber
}

func NewEcdsaZkEvm(
	comp *wizard.CompiledIOP,
	settings *Settings,
	arith *arithmetization.Arithmetization,
) *EcdsaZkEvm {
	return &EcdsaZkEvm{
		Ant: newAntichamber(
			comp,
			&antichamberInput{
				Settings:     settings,
				EcSource:     getEcdataArithmetization(comp, arith),
				TxSource:     getTxnDataArithmetization(comp, arith),
				RlpTxn:       getRlpTxnArithmetization(comp, arith),
				PlonkOptions: []query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)},
				WithCircuit:  true,
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

func getEcdataArithmetization(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *ecDataSource {
	src := &ecDataSource{
		CsEcrecover: arith.ColumnOf(comp, "ecdata", "CIRCUIT_SELECTOR_ECRECOVER"),
		ID:          arith.MashedColumnOf(comp, "ecdata", "ID"),
		SuccessBit:  arith.ColumnOf(comp, "ecdata", "SUCCESS_BIT"),
		Index:       arith.ColumnOf(comp, "ecdata", "INDEX"),
		IsData:      arith.ColumnOf(comp, "ecdata", "IS_ECRECOVER_DATA"),
		IsRes:       arith.ColumnOf(comp, "ecdata", "IS_ECRECOVER_RESULT"),
		Limb:        arith.GetLimbsOfU128Le(comp, "ecdata", "LIMB"),
	}

	return src
}

func getTxnDataArithmetization(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *txnData {

	td := &txnData{
		Ct:       arith.ColumnOf(comp, "txndata", "CT"),
		User:     arith.ColumnOf(comp, "txndata", "USER"),
		Selector: arith.ColumnOf(comp, "txndata", "HUB"),
		From: limbs.FuseLimbs(
			arith.GetLimbsOfU32Le(comp, "txndata.hub", "FROM_ADDRESS_HI").AsDynSize(),
			arith.GetLimbsOfU128Le(comp, "txndata.hub", "FROM_ADDRESS_LO").AsDynSize(),
		).ZeroExtendToSize(16).AssertUint256(),
	}

	return td
}

func getRlpTxnArithmetization(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) generic.GenDataModule {
	limbs := arith.GetLimbsOfU128Le(comp, "rlptxn", "cmpLIMB")
	res := generic.GenDataModule{
		HashNum: arith.ColumnOf(comp, "rlptxn", "USER_TXN_NUMBER"),
		Index:   arith.MashedColumnOf(comp, "rlptxn", "INDEX_LX"),
		NBytes:  arith.ColumnOf(comp, "rlptxn", "cmpLIMB_SIZE"),
		ToHash:  arith.ColumnOf(comp, "rlptxn", "TO_HASH_BY_PROVER"),
		Limbs:   limbs.ToBigEndianUint(),
	}

	return res
}
