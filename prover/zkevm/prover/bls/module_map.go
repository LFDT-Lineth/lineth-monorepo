package bls

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

const (
	NAME_BLS_MAP = "BLS_MAP"
)

type BlsMapDataSource struct {
	ID      ifaces.Column
	CsMap   ifaces.Column
	Limb    ifaces.Column
	Index   ifaces.Column
	Counter ifaces.Column
	IsData  ifaces.Column
	IsRes   ifaces.Column
}

func newMapDataSource(comp *wizard.CompiledIOP, g Group) *BlsMapDataSource {
	var mapString string
	if g == G1 {
		mapString = "MAP_FP_TO_G1"
	} else {
		mapString = "MAP_FP2_TO_G2"
	}
	return &BlsMapDataSource{
		ID:      comp.Columns.GetHandle(colNameFn("ID")),
		CsMap:   comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_BLS_" + mapString)),
		Index:   comp.Columns.GetHandle(colNameFn("INDEX")),
		Counter: comp.Columns.GetHandle(colNameFn("CT")),
		Limb:    comp.Columns.GetHandle(colNameFn("LIMB")),
		IsData:  comp.Columns.GetHandle(colNameFn("DATA_BLS_" + mapString + "_FLAG")),
		IsRes:   comp.Columns.GetHandle(colNameFn("RSLT_BLS_" + mapString + "_FLAG")),
	}
}

type BlsMap struct {
	*BlsMapDataSource
	AlignedGnarkData *plonk.Alignment
	*Limits
	Group
}

func newMap(_ *wizard.CompiledIOP, g Group, limits *Limits, src *BlsMapDataSource) *BlsMap {
	res := &BlsMap{
		BlsMapDataSource: src,
		Limits:           limits,
		Group:            g,
	}
	return res
}

func (bm *BlsMap) WithMapCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsMap {
	// gnark circuits takes the inputs as is. To get the bound on the number of circuits we need
	// to divide the maximum number of inputs instances with the number of instances per circuit
	maxNbInstancesInputs := utils.DivCeil(bm.BlsMapDataSource.CsMap.Size(), nbRowsPerMap(bm.Group))
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
		DataToCircuitMask:  bm.BlsMapDataSource.CsMap,
		DataToCircuit:      bm.BlsMapDataSource.Limb,
		Circuit:            NewMapCircuit(bm.Group, bm.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     mapToGroupInputFillerKey(bm.Group),
		PlonkOptions:       options,
	}
	bm.AlignedGnarkData = plonk.DefineAlignment(comp, toAlign)
	return bm
}

func (bm *BlsMap) Assign(run *wizard.ProverRuntime) {
	if bm.AlignedGnarkData != nil {
		bm.AlignedGnarkData.Assign(run)
	}
}
