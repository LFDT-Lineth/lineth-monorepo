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
	NAME_BLS_ADD = "BLS_ADD"
)

type BlsAddDataSource struct {
	ID                ifaces.Column
	CsAdd             ifaces.Column
	CsCurveMembership ifaces.Column
	Limb              limbs.Uint128Le
	Index             ifaces.Column
	Counter           ifaces.Column
	IsData            ifaces.Column
	IsRes             ifaces.Column
}

func newAddDataSource(comp *wizard.CompiledIOP, g Group, arith *arithmetization.Arithmetization) *BlsAddDataSource {
	return &BlsAddDataSource{
		ID:                arith.MashedColumnOf(comp, moduleName, "ID"),
		CsAdd:             arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_BLS_"+g.String()+"_ADD"),
		Limb:              arith.GetLimbsOfU128Le(comp, moduleName, "LIMB"),
		Index:             arith.ColumnOf(comp, moduleName, "INDEX"),
		Counter:           arith.ColumnOf(comp, moduleName, "CT"),
		CsCurveMembership: arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_"+g.StringCurve()+"_MEMBERSHIP"),
		IsData:            arith.ColumnOf(comp, moduleName, "DATA_BLS_"+g.String()+"_ADD_FLAG"),
		IsRes:             arith.ColumnOf(comp, moduleName, "RSLT_BLS_"+g.String()+"_ADD_FLAG"),
	}
}

type BlsAdd struct {
	*BlsAddDataSource
	AlignedAddGnarkData             *plonk.Alignment
	AlignedCurveMembershipGnarkData *plonk.Alignment

	FlattenLimbsAdd             *common.FlattenColumn
	FlattenLimbsCurveMembership *common.FlattenColumn

	*Limits
	Group
}

func newAdd(comp *wizard.CompiledIOP, g Group, limits *Limits, src *BlsAddDataSource) *BlsAdd {
	flattenLimbsAdd := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CsAdd)
	flattenLimbsCurveMembership := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), src.CsCurveMembership)

	res := &BlsAdd{
		BlsAddDataSource:            src,
		Limits:                      limits,
		Group:                       g,
		FlattenLimbsAdd:             flattenLimbsAdd,
		FlattenLimbsCurveMembership: flattenLimbsCurveMembership,
	}

	flattenLimbsAdd.CsFlattenProjection(comp)
	flattenLimbsCurveMembership.CsFlattenProjection(comp)

	return res
}

func (ba *BlsAdd) WithAddCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsAdd {
	// the gnark circuit takes exactly the same rows as provided by the arithmetization. So
	// to get the bound on the number of circuits we just need to divide by the size of the
	// addition circuit input instances
	maxNbInstancesInputs := utils.DivCeil(ba.FlattenLimbsAdd.Mask.Size(), nbRowsPerAdd(ba.Group))
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
		DataToCircuitMask:  ba.FlattenLimbsAdd.Mask,
		DataToCircuit:      ba.FlattenLimbsAdd.Limbs,
		Circuit:            newAddCircuit(ba.Group, ba.Limits),
		NbCircuitInstances: maxNbCircuits,
		PlonkOptions:       options,
	}
	ba.AlignedAddGnarkData = plonk.DefineAlignment(comp, toAlignAdd)
	return ba
}

func (ba *BlsAdd) WithCurveMembershipCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsAdd {
	maxNbInstancesInputs := utils.DivCeil(ba.FlattenLimbsCurveMembership.Mask.Size(), nbRowsPerCurveMembership(ba.Group))
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
		DataToCircuitMask:  ba.FlattenLimbsCurveMembership.Mask,
		DataToCircuit:      ba.FlattenLimbsCurveMembership.Limbs,
		Circuit:            newCheckCircuit(ba.Group, CURVE, ba.Limits),
		NbCircuitInstances: maxNbCircuits,
		PlonkOptions:       options,
		InputFillerKey:     membershipInputFillerKey(ba.Group, CURVE),
	}

	ba.AlignedCurveMembershipGnarkData = plonk.DefineAlignment(comp, toAlignCurveMembership)
	return ba
}

func (ba *BlsAdd) Assign(run *wizard.ProverRuntime) {
	ba.FlattenLimbsAdd.Run(run)
	ba.FlattenLimbsCurveMembership.Run(run)
	if ba.AlignedAddGnarkData != nil {
		ba.AlignedAddGnarkData.Assign(run)
	}
	if ba.AlignedCurveMembershipGnarkData != nil {
		ba.AlignedCurveMembershipGnarkData.Assign(run)
	}
}
