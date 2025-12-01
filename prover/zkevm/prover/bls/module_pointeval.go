package bls

import (
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

const (
	NAME_BLS_POINTEVAL = "BLS_POINTEVAL"
)

type BlsPointEvalDataSource struct {
	ID                 ifaces.Column
	CsPointEval        ifaces.Column
	CsPointEvalInvalid ifaces.Column
	Limb               ifaces.Column
	Index              ifaces.Column
	Counter            ifaces.Column
	IsData             ifaces.Column
	IsRes              ifaces.Column
}

func newPointEvalDataSource(comp *wizard.CompiledIOP) *BlsPointEvalDataSource {
	return &BlsPointEvalDataSource{
		ID:                 comp.Columns.GetHandle(colNameFn("ID")),
		CsPointEval:        comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_POINT_EVALUATION")),
		CsPointEvalInvalid: comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE")),
		Limb:               comp.Columns.GetHandle(colNameFn("LIMB")),
		Index:              comp.Columns.GetHandle(colNameFn("INDEX")),
		Counter:            comp.Columns.GetHandle(colNameFn("CT")),
		IsData:             comp.Columns.GetHandle(colNameFn("DATA_POINT_EVALUATION_FLAG")),
		IsRes:              comp.Columns.GetHandle(colNameFn("RSLT_POINT_EVALUATION_FLAG")),
	}
}

type BlsPointEval struct {
	*BlsPointEvalDataSource
	AlignedGnarkData        *plonk.Alignment
	AlignedFailureGnarkData *plonk.Alignment
	*Limits
}

func newPointEval(_ *wizard.CompiledIOP, limits *Limits, src *BlsPointEvalDataSource) *BlsPointEval {
	res := &BlsPointEval{
		BlsPointEvalDataSource: src,
		Limits:                 limits,
	}
	return res
}

func (bp *BlsPointEval) WithPointEvalCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPointEval {
	// the gnark circuit takes exactly the same rows as provided by the arithmetization. So
	// to get the bound on the number of circuits we just need to divide by the size of the
	// addition circuit input instances
	maxNbInstancesInputs := utils.DivCeil(bp.BlsPointEvalDataSource.CsPointEval.Size(), nbRowsPerPointEval)
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
		DataToCircuitMask:  bp.BlsPointEvalDataSource.CsPointEval,
		DataToCircuit:      bp.BlsPointEvalDataSource.Limb,
		Circuit:            newMultiPointEvalCircuit(bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     pointEvalInputFillerKey,
		PlonkOptions:       options,
	}
	bp.AlignedGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bp
}

func (bp *BlsPointEval) WithPointEvalFailureCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPointEval {
	maxNbInstancesInputs := utils.DivCeil(bp.BlsPointEvalDataSource.CsPointEvalInvalid.Size(), nbRowsPerPointEval)
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
		DataToCircuitMask:  bp.BlsPointEvalDataSource.CsPointEvalInvalid,
		DataToCircuit:      bp.BlsPointEvalDataSource.Limb,
		Circuit:            newMultiPointEvalFailureCircuit(bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     pointEvalFailureInputFillerKey,
		PlonkOptions:       options,
	}
	bp.AlignedFailureGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bp
}

func (bp *BlsPointEval) Assign(run *wizard.ProverRuntime) {
	if bp.AlignedGnarkData != nil {
		bp.AlignedGnarkData.Assign(run)
	}
	if bp.AlignedFailureGnarkData != nil {
		bp.AlignedFailureGnarkData.Assign(run)
	}
}
