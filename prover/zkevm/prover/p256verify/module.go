package p256verify

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/sirupsen/logrus"
)

const (
	NAME_P256_VERIFY = "P256_VERIFY"
	moduleName       = "ecdata"
	ROUND_NR         = 0
)

type P256VerifyDataSource struct {
	ID       ifaces.Column
	CS       ifaces.Column
	Limb     limbs.Uint128Le
	Index    ifaces.Column
	IsData   ifaces.Column
	IsResult ifaces.Column
}

func newP256VerifyDataSource(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *P256VerifyDataSource {
	return &P256VerifyDataSource{
		ID:       arith.MashedColumnOf(comp, moduleName, "ID"),
		CS:       arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_P256_VERIFY"),
		Limb:     arith.GetLimbsOfU128Le(comp, moduleName, "LIMB"),
		Index:    arith.ColumnOf(comp, moduleName, "INDEX"),
		IsData:   arith.ColumnOf(comp, moduleName, "IS_P256_VERIFY_DATA"),
		IsResult: arith.ColumnOf(comp, moduleName, "IS_P256_VERIFY_RESULT"),
	}
}

type P256Verify struct {
	*P256VerifyDataSource
	P256VerifyGnarkData *plonk.Alignment
	FlattenLimbs        *common.FlattenColumn
	*Limits
}

func newP256Verify(comp *wizard.CompiledIOP, limits *Limits, src *P256VerifyDataSource) *P256Verify {
	flattenLimbs := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CS)
	res := &P256Verify{
		P256VerifyDataSource: src,
		FlattenLimbs:         flattenLimbs,
		Limits:               limits,
	}
	flattenLimbs.CsFlattenProjection(comp)

	return res
}

func (pv *P256Verify) WithCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *P256Verify {

	maxNbInstancesInputs := utils.DivCeil(pv.FlattenLimbs.Mask.Size(), nbRows)
	maxNbInstancesLimit := pv.Limits.LimitCalls
	switch maxNbInstancesLimit {
	case 0:
		// we omit the circuit entirely when limit is 0
		logrus.Warnf("P256 Verify circuit will be omitted as limit is set to 0")
		return pv
	case -1:
		// we use the trace size as limit
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, pv.Limits.NbInputInstances)

	toAlign := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_ALIGNMENT", NAME_P256_VERIFY),
		Round:              ROUND_NR,
		DataToCircuitMask:  pv.FlattenLimbs.Mask,
		DataToCircuit:      pv.FlattenLimbs.Limbs,
		Circuit:            newP256VerifyCircuit(pv.Limits),
		NbCircuitInstances: maxNbCircuits,
		PlonkOptions:       options,
		InputFillerKey:     input_filler_key,
	}
	pv.P256VerifyGnarkData = plonk.DefineAlignment(comp, toAlign)
	return pv
}

func (pv *P256Verify) Assign(run *wizard.ProverRuntime) {
	pv.FlattenLimbs.Run(run)
	if pv.P256VerifyGnarkData != nil {
		pv.P256VerifyGnarkData.Assign(run)
	}
}

func NewP256VerifyZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *P256Verify {
	return newP256Verify(comp, limits, newP256VerifyDataSource(comp, arith)).
		WithCircuit(comp, query.PlonkRangeCheckOption(16, 1, true))
}
