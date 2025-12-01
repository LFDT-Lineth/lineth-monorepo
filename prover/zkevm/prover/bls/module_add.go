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
	NAME_BLS_ADD = "BLS_ADD"
)

type BlsAddDataSource struct {
	ID                ifaces.Column
	CsAdd             ifaces.Column
	CsCurveMembership ifaces.Column
	Limb              ifaces.Column
	Index             ifaces.Column
	Counter           ifaces.Column
	IsData            ifaces.Column
	IsRes             ifaces.Column
}

func newAddDataSource(comp *wizard.CompiledIOP, g Group) *BlsAddDataSource {
	return &BlsAddDataSource{
		ID:                comp.Columns.GetHandle(colNameFn("ID")),
		CsAdd:             comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_BLS_" + g.String() + "_ADD")),
		Limb:              comp.Columns.GetHandle(colNameFn("LIMB")),
		Index:             comp.Columns.GetHandle(colNameFn("INDEX")),
		Counter:           comp.Columns.GetHandle(colNameFn("CT")),
		CsCurveMembership: comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_" + g.StringCurve() + "_MEMBERSHIP")),
		IsData:            comp.Columns.GetHandle(colNameFn("DATA_BLS_" + g.String() + "_ADD_FLAG")),
		IsRes:             comp.Columns.GetHandle(colNameFn("RSLT_BLS_" + g.String() + "_ADD_FLAG")),
	}
}

type BlsAdd struct {
	*BlsAddDataSource
	AlignedAddGnarkData             *plonk.Alignment
	AlignedCurveMembershipGnarkData *plonk.Alignment

	*Limits
	Group
}

func newAdd(_ *wizard.CompiledIOP, g Group, limits *Limits, src *BlsAddDataSource) *BlsAdd {
	res := &BlsAdd{
		BlsAddDataSource: src,
		Limits:           limits,
		Group:            g,
	}

	return res
}

func (ba *BlsAdd) WithAddCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsAdd {
	// the gnark circuit takes exactly the same rows as provided by the arithmetization. So
	// to get the bound on the number of circuits we just need to divide by the size of the
	// addition circuit input instances
	maxNbInstancesInputs := utils.DivCeil(ba.BlsAddDataSource.CsAdd.Size(), nbRowsPerAdd(ba.Group))
	maxNbInstancesLimit := ba.limitAddCalls(ba.Group)
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsAdd: omitting addition circuit for group %s as limit is 0", ba.Group.String())
		return ba
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, ba.Limits.nbAddInputInstances(ba.Group))

	toAlignAdd := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_%s_ALIGNMENT", NAME_BLS_ADD, ba.Group.String()),
		Round:              ROUND_NR,
		DataToCircuitMask:  ba.BlsAddDataSource.CsAdd,
		DataToCircuit:      ba.BlsAddDataSource.Limb,
		Circuit:            newAddCircuit(ba.Group, ba.Limits),
		NbCircuitInstances: maxNbCircuits,
		PlonkOptions:       options,
	}
	ba.AlignedAddGnarkData = plonk.DefineAlignment(comp, toAlignAdd)
	return ba
}

func (ba *BlsAdd) WithCurveMembershipCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsAdd {
	maxNbInstancesInputs := utils.DivCeil(ba.BlsAddDataSource.CsCurveMembership.Size(), nbRowsPerCurveMembership(ba.Group))
	maxNbInstancesLimit := ba.limitCurveMembershipCalls(ba.Group)
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsAdd: omitting curve membership circuit for group %s as limit is 0", ba.Group.String())
		return ba
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, ba.Limits.nbCurveMembershipInputInstances(ba.Group))

	toAlignCurveMembership := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_%s_CURVE_MEMBERSHIP_ALIGNMENT", NAME_BLS_ADD, ba.Group.StringCurve()),
		Round:              ROUND_NR,
		DataToCircuitMask:  ba.BlsAddDataSource.CsCurveMembership,
		DataToCircuit:      ba.BlsAddDataSource.Limb,
		Circuit:            newCheckCircuit(ba.Group, CURVE, ba.Limits),
		NbCircuitInstances: maxNbCircuits,
		PlonkOptions:       options,
		InputFillerKey:     membershipInputFillerKey(ba.Group, CURVE),
	}

	ba.AlignedCurveMembershipGnarkData = plonk.DefineAlignment(comp, toAlignCurveMembership)
	return ba
}

func (ba *BlsAdd) Assign(run *wizard.ProverRuntime) {
	if ba.AlignedAddGnarkData != nil {
		ba.AlignedAddGnarkData.Assign(run)
	}
	if ba.AlignedCurveMembershipGnarkData != nil {
		ba.AlignedCurveMembershipGnarkData.Assign(run)
	}
}
