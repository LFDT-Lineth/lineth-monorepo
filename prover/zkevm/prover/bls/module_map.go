package bls

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
	NAME_BLS_MAP = "BLS_MAP"
)

type BlsMapDataSource struct {
	ID      ifaces.Column
	CsMap   ifaces.Column
	Limb    limbs.Uint128Le
	Index   ifaces.Column
	Counter ifaces.Column
	IsData  ifaces.Column
	IsRes   ifaces.Column
}

func newMapDataSource(comp *wizard.CompiledIOP, g Group, arith *arithmetization.Arithmetization) *BlsMapDataSource {
	var mapString string
	if g == G1 {
		mapString = "MAP_FP_TO_G1"
	} else {
		mapString = "MAP_FP2_TO_G2"
	}
	return &BlsMapDataSource{
		ID:      arith.MashedColumnOf(comp, moduleName, "ID"),
		CsMap:   arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_BLS_"+mapString),
		Index:   arith.ColumnOf(comp, moduleName, "INDEX"),
		Counter: arith.ColumnOf(comp, moduleName, "CT"),
		Limb:    arith.GetLimbsOfU128Le(comp, moduleName, "LIMB"),
		IsData:  arith.ColumnOf(comp, moduleName, "DATA_BLS_"+mapString+"_FLAG"),
		IsRes:   arith.ColumnOf(comp, moduleName, "RSLT_BLS_"+mapString+"_FLAG"),
	}
}

type BlsMap struct {
	*BlsMapDataSource
	AlignedGnarkData *plonk.Alignment
	FlattenLimbs     *common.FlattenColumn
	*Limits
	Group
}

func newMap(comp *wizard.CompiledIOP, g Group, limits *Limits, src *BlsMapDataSource) *BlsMap {
	flattenLimbs := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CsMap)
	res := &BlsMap{
		BlsMapDataSource: src,
		Limits:           limits,
		Group:            g,
		FlattenLimbs:     flattenLimbs,
	}
	flattenLimbs.CsFlattenProjection(comp)
	return res
}

func (bm *BlsMap) WithMapCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsMap {
	// gnark circuits takes the inputs as is. To get the bound on the number of circuits we need
	// to divide the maximum number of inputs instances with the number of instances per circuit
	maxNbInstancesInputs := utils.DivCeil(bm.FlattenLimbs.Mask.Size(), nbRowsPerMap(bm.Group))
	maxNbInstancesLimit := bm.limitMapCalls(bm.Group)
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsMap: omitting map circuit for group %s as limit is 0", bm.Group.String())
		return bm
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, bm.Limits.nbMapInputInstances(bm.Group))
	toAlign := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_%s_ALIGNMENT", NAME_BLS_MAP, bm.Group.String()),
		Round:              ROUND_NR,
		DataToCircuitMask:  bm.FlattenLimbs.Mask,
		DataToCircuit:      bm.FlattenLimbs.Limbs,
		Circuit:            NewMapCircuit(bm.Group, bm.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     mapToGroupInputFillerKey(bm.Group),
		PlonkOptions:       options,
	}
	bm.AlignedGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bm
}

func (bm *BlsMap) Assign(run *wizard.ProverRuntime) {
	bm.FlattenLimbs.Run(run)
	if bm.AlignedGnarkData != nil {
		bm.AlignedGnarkData.Assign(run)
	}
}
