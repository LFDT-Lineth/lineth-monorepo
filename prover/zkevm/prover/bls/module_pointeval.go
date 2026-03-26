package bls

import (
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
	NAME_BLS_POINTEVAL = "BLS_POINTEVAL"
)

type BlsPointEvalDataSource struct {
	ID                 ifaces.Column
	CsPointEval        ifaces.Column
	CsPointEvalInvalid ifaces.Column
	Limb               limbs.Uint128Le
	Index              ifaces.Column
	Counter            ifaces.Column
	IsData             ifaces.Column
	IsRes              ifaces.Column
}

func newPointEvalDataSource(comp *wizard.CompiledIOP, arith *arithmetization.Arithmetization) *BlsPointEvalDataSource {
	return &BlsPointEvalDataSource{
		ID:                 arith.MashedColumnOf(comp, moduleName, "ID"),
		CsPointEval:        arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_POINT_EVALUATION"),
		CsPointEvalInvalid: arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE"),
		Limb:               arith.GetLimbsOfU128Le(comp, moduleName, "LIMB"),
		Index:              arith.ColumnOf(comp, moduleName, "INDEX"),
		Counter:            arith.ColumnOf(comp, moduleName, "CT"),
		IsData:             arith.ColumnOf(comp, moduleName, "DATA_POINT_EVALUATION_FLAG"),
		IsRes:              arith.ColumnOf(comp, moduleName, "RSLT_POINT_EVALUATION_FLAG"),
	}
}

type BlsPointEval struct {
	*BlsPointEvalDataSource
	AlignedGnarkData        *plonk.Alignment
	AlignedFailureGnarkData *plonk.Alignment
	FlattenLimbs            *common.FlattenColumn
	FlattenFailureLimbs     *common.FlattenColumn
	*Limits
}

func newPointEval(comp *wizard.CompiledIOP, limits *Limits, src *BlsPointEvalDataSource) *BlsPointEval {
	flattenLimbs := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CsPointEval)
	flattenFailureLimbs := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CsPointEvalInvalid)
	res := &BlsPointEval{
		BlsPointEvalDataSource: src,
		FlattenLimbs:           flattenLimbs,
		FlattenFailureLimbs:    flattenFailureLimbs,
		Limits:                 limits,
	}

	flattenLimbs.CsFlattenProjection(comp)
	flattenFailureLimbs.CsFlattenProjection(comp)
	return res
}

func (bp *BlsPointEval) WithPointEvalCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPointEval {
	// the gnark circuit takes exactly the same rows as provided by the arithmetization. So
	// to get the bound on the number of circuits we just need to divide by the size of the
	// addition circuit input instances
	maxNbInstancesInputs := utils.DivCeil(bp.FlattenLimbs.Mask.Size(), nbRowsPerPointEval)
	maxNbInstancesLimit := bp.LimitPointEvalCalls
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsPointEval: omitting point evaluation circuit as limit is 0")
		return bp
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, bp.Limits.NbPointEvalInputInstances)
	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_BLS_POINTEVAL,
		Round:              ROUND_NR,
		DataToCircuitMask:  bp.FlattenLimbs.Mask,
		DataToCircuit:      bp.FlattenLimbs.Limbs,
		Circuit:            newMultiPointEvalCircuit(bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     pointEvalInputFillerKey,
		PlonkOptions:       options,
	}
	bp.AlignedGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bp
}

func (bp *BlsPointEval) WithPointEvalFailureCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPointEval {
	maxNbInstancesInputs := utils.DivCeil(bp.FlattenFailureLimbs.Mask.Size(), nbRowsPerPointEval)
	maxNbInstancesLimit := bp.LimitPointEvalFailureCalls
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsPointEval: omitting point evaluation failure circuit as limit is 0")
		return bp
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, bp.Limits.NbPointEvalFailureInputInstances)
	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_BLS_POINTEVAL + "_FAILURE",
		Round:              ROUND_NR,
		DataToCircuitMask:  bp.FlattenFailureLimbs.Mask,
		DataToCircuit:      bp.FlattenFailureLimbs.Limbs,
		Circuit:            newMultiPointEvalFailureCircuit(bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     pointEvalFailureInputFillerKey,
		PlonkOptions:       options,
	}
	bp.AlignedFailureGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bp
}

func (bp *BlsPointEval) Assign(run *wizard.ProverRuntime) {
	bp.FlattenLimbs.Run(run)
	bp.FlattenFailureLimbs.Run(run)
	if bp.AlignedGnarkData != nil {
		bp.AlignedGnarkData.Assign(run)
	}
	if bp.AlignedFailureGnarkData != nil {
		bp.AlignedFailureGnarkData.Assign(run)
	}
}
