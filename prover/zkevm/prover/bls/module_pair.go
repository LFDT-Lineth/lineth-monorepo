package bls

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/sirupsen/logrus"
)

const (
	NAME_BLS_PAIR       = "BLS_PAIR"
	NAME_UNALIGNED_PAIR = "UNALIGNED_BLS_PAIR"
)

type BlsPairDataSource struct {
	ID             ifaces.Column
	CsPair         ifaces.Column
	CsG1Membership ifaces.Column
	CsG2Membership ifaces.Column
	IsData         ifaces.Column
	IsRes          ifaces.Column
	Index          ifaces.Column
	Counter        ifaces.Column
	Limb           ifaces.Column
	SuccessBit     ifaces.Column
}

func newPairDataSource(comp *wizard.CompiledIOP) *BlsPairDataSource {
	return &BlsPairDataSource{
		ID:             comp.Columns.GetHandle(colNameFn("ID")),
		CsPair:         comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_BLS_PAIRING_CHECK")),
		CsG1Membership: comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_G1_MEMBERSHIP")),
		CsG2Membership: comp.Columns.GetHandle(colNameFn("CIRCUIT_SELECTOR_G2_MEMBERSHIP")),
		Limb:           comp.Columns.GetHandle(colNameFn("LIMB")),
		Index:          comp.Columns.GetHandle(colNameFn("INDEX")),
		Counter:        comp.Columns.GetHandle(colNameFn("CT")),
		IsData:         comp.Columns.GetHandle(colNameFn("DATA_BLS_PAIRING_CHECK_FLAG")),
		IsRes:          comp.Columns.GetHandle(colNameFn("RSLT_BLS_PAIRING_CHECK_FLAG")),
		SuccessBit:     comp.Columns.GetHandle(colNameFn("SUCCESS_BIT")),
	}
}

type BlsPair struct {
	*BlsPairDataSource
	*UnalignedPairData
	AlignedMillerLoopData        *plonk.Alignment
	AlignedFinalExpData          *plonk.Alignment
	AlignedG1MembershipGnarkData *plonk.Alignment
	AlignedG2MembershipGnarkData *plonk.Alignment
	*Limits
}

func newPair(comp *wizard.CompiledIOP, limits *Limits, src *BlsPairDataSource) *BlsPair {
	ucmd := newUnalignedPairData(comp, src)

	res := &BlsPair{
		BlsPairDataSource: src,
		UnalignedPairData: ucmd,
		Limits:            limits,
	}

	return res
}

func (bp *BlsPair) WithPairingCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPair {
	maxNbMillerLoopInstances := bp.LimitMillerLoopCalls
	if maxNbMillerLoopInstances == -1 {
		// if limit is -1, then we take all the inputs
		maxNbMillerLoopInstances = bp.MaxNbPairInputs
	}
	maxNbMillerLoopInstances = min(bp.MaxNbPairInputs, maxNbMillerLoopInstances)
	if maxNbMillerLoopInstances > 0 {
		// we omit the circuit when the limit is explicitly 0
		maxNbMlCircuits := utils.DivCeil(maxNbMillerLoopInstances, bp.Limits.NbMillerLoopInputInstances)
		toAlignMl := &plonk.CircuitAlignmentInput{
			Name:               fmt.Sprintf("%s_ML", NAME_BLS_PAIR),
			Round:              ROUND_NR,
			DataToCircuitMask:  bp.UnalignedPairData.GnarkIsActiveMillerLoop,
			DataToCircuit:      bp.UnalignedPairData.GnarkDataMillerLoop,
			Circuit:            newMultiMillerLoopMulCircuit(bp.Limits),
			NbCircuitInstances: maxNbMlCircuits,
			InputFillerKey:     millerLoopInputFillerKey,
			PlonkOptions:       options,
		}
		bp.AlignedMillerLoopData = plonk.DefineAlignment(comp, toAlignMl)
	} else {
		logrus.Warnf("BlsPair: omitting Miller loop circuit as limit is 0")
	}

	maxNbFinalExpInstancesInstances := bp.LimitFinalExpCalls
	if maxNbFinalExpInstancesInstances == -1 {
		// if limit is -1, then we take all the inputs
		maxNbFinalExpInstancesInstances = bp.MaxNbPairInputs
	}
	maxNbFinalExpInstancesInstances = min(bp.MaxNbPairInputs, maxNbFinalExpInstancesInstances)

	if maxNbFinalExpInstancesInstances > 0 {
		// we omit the circuit when the limit is explicitly 0
		maxNbFeCircuits := utils.DivCeil(maxNbFinalExpInstancesInstances, bp.Limits.NbFinalExpInputInstances)
		toAlignFe := &plonk.CircuitAlignmentInput{
			Name:               fmt.Sprintf("%s_FE", NAME_BLS_PAIR),
			Round:              ROUND_NR,
			DataToCircuitMask:  bp.UnalignedPairData.GnarkIsActiveFinalExp,
			DataToCircuit:      bp.UnalignedPairData.GnarkDataFinalExp,
			Circuit:            newMultiMillerLoopFinalExpCircuit(bp.Limits),
			NbCircuitInstances: maxNbFeCircuits,
			InputFillerKey:     finalExpInputFillerKey,
			PlonkOptions:       options,
		}
		bp.AlignedFinalExpData = plonk.DefineAlignment(comp, toAlignFe)
	} else {
		logrus.Warnf("BlsPair: omitting Final Exponentiation circuit as limit is 0")
	}
	return bp
}

func (bp *BlsPair) WithG1MembershipCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPair {
	maxNbInstancesInputs := utils.DivCeil(bp.UnalignedPairData.GnarkIsActiveG1Membership.Size(), nbG1Limbs)
	maxNbInstancesLimit := bp.limitGroupMembershipCalls(G1)
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsPair: omitting G1 membership circuit as limit is 0")
		return bp
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, bp.Limits.nbGroupMembershipInputInstances(G1))
	toAlignG1Ms := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_G1_MEMBERSHIP", NAME_BLS_PAIR),
		Round:              ROUND_NR,
		DataToCircuitMask:  bp.UnalignedPairData.GnarkIsActiveG1Membership,
		DataToCircuit:      bp.BlsPairDataSource.Limb,
		Circuit:            newCheckCircuit(G1, GROUP, bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     membershipInputFillerKey(G1, GROUP),
		PlonkOptions:       options,
	}
	bp.AlignedG1MembershipGnarkData = plonk.DefineAlignment(comp, toAlignG1Ms)
	return bp
}

func (bp *BlsPair) WithG2MembershipCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsPair {
	maxNbInstancesInputs := utils.DivCeil(bp.UnalignedPairData.GnarkIsActiveG2Membership.Size(), nbG2Limbs)
	maxNbInstancesLimit := bp.limitGroupMembershipCalls(G2)
	switch maxNbInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsPair: omitting G2 membership circuit as limit is 0")
		return bp
	case -1:
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbInstancesInputs
	}
	maxNbInstances := min(maxNbInstancesInputs, maxNbInstancesLimit)
	maxNbCircuits := utils.DivCeil(maxNbInstances, bp.Limits.nbGroupMembershipInputInstances(G2))
	toAlignG2Ms := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_G2_MEMBERSHIP", NAME_BLS_PAIR),
		Round:              ROUND_NR,
		DataToCircuitMask:  bp.UnalignedPairData.GnarkIsActiveG2Membership,
		DataToCircuit:      bp.BlsPairDataSource.Limb,
		Circuit:            newCheckCircuit(G2, GROUP, bp.Limits),
		NbCircuitInstances: maxNbCircuits,
		InputFillerKey:     membershipInputFillerKey(G2, GROUP),
		PlonkOptions:       options,
	}
	bp.AlignedG2MembershipGnarkData = plonk.DefineAlignment(comp, toAlignG2Ms)
	return bp
}

func (bp *BlsPair) Assign(run *wizard.ProverRuntime) {
	bp.UnalignedPairData.Assign(run)
	if bp.AlignedMillerLoopData != nil {
		bp.AlignedMillerLoopData.Assign(run)
	}
	if bp.AlignedFinalExpData != nil {
		bp.AlignedFinalExpData.Assign(run)
	}
	if bp.AlignedG1MembershipGnarkData != nil {
		bp.AlignedG1MembershipGnarkData.Assign(run)
	}
	if bp.AlignedG2MembershipGnarkData != nil {
		bp.AlignedG2MembershipGnarkData.Assign(run)
	}
}

type UnalignedPairData struct {
	*BlsPairDataSource
	IsPairingInstance   ifaces.Column
	IsPairingAndSuccess ifaces.Column
	IsActive            ifaces.Column
	IsFirstLine         ifaces.Column
	IsLastLine          ifaces.Column
	IsNotLastLine       ifaces.Column
	PointG1             [nbG1Limbs]ifaces.Column
	PointG2             [nbG2Limbs]ifaces.Column
	PrevAccumulator     [nbGtLimbs]ifaces.Column
	CurrentAccumulator  [nbGtLimbs]ifaces.Column
	ExpectedResult      [2]ifaces.Column

	GnarkIsActiveG1Membership ifaces.Column
	GnarkIsActiveG2Membership ifaces.Column

	// data which is project from above to columns going into the gnark pairing check circuit
	GnarkIsActiveMillerLoop ifaces.Column
	GnarkDataMillerLoop     ifaces.Column

	GnarkIsActiveFinalExp ifaces.Column
	GnarkDataFinalExp     ifaces.Column

	MaxNbPairInputs int
}

func newUnalignedPairData(comp *wizard.CompiledIOP, src *BlsPairDataSource) *UnalignedPairData {
	// for bounding the size of the alignment, we assume the worst case inputs where we have many pairing checks with
	// a single input. A single pairing check input is G1 and G2 element
	maxNbPairInputs := src.CsPair.Size()/(nbG1Limbs+nbG2Limbs) + 1

	createColFnUa := createColFn(comp, NAME_UNALIGNED_PAIR, utils.NextPowerOfTwo(maxNbPairInputs))
	createColFnMl := createColFn(comp, NAME_UNALIGNED_PAIR, utils.NextPowerOfTwo(maxNbPairInputs*nbRowsPerMillerLoop))
	createColFnFe := createColFn(comp, NAME_UNALIGNED_PAIR, utils.NextPowerOfTwo(maxNbPairInputs*nbRowsPerFinalExp))
	ucmd := &UnalignedPairData{
		BlsPairDataSource:         src,
		IsActive:                  createColFnUa("IS_ACTIVE"),
		IsFirstLine:               createColFnUa("IS_FIRST_LINE"),
		IsLastLine:                createColFnUa("IS_LAST_LINE"),
		IsNotLastLine:             createColFnUa("IS_NOT_LAST_LINE"),
		GnarkIsActiveMillerLoop:   createColFnMl("GNARK_IS_ACTIVE_ML"),
		GnarkDataMillerLoop:       createColFnMl("GNARK_DATA_ML"),
		GnarkIsActiveFinalExp:     createColFnFe("GNARK_IS_ACTIVE_FE"),
		GnarkDataFinalExp:         createColFnFe("GNARK_DATA_FE"),
		IsPairingInstance:         comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", NAME_BLS_PAIR, "IS_PAIRING_INSTANCE"), max(src.IsData.Size(), src.IsRes.Size())),
		IsPairingAndSuccess:       comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", NAME_BLS_PAIR, "IS_PAIRING_AND_SUCCESS"), max(src.IsData.Size(), src.IsRes.Size(), src.SuccessBit.Size())),
		GnarkIsActiveG1Membership: comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", NAME_BLS_PAIR, "GNARK_IS_ACTIVE_G1_MEMBERSHIP"), max(src.SuccessBit.Size(), src.CsG1Membership.Size())),
		GnarkIsActiveG2Membership: comp.InsertCommit(ROUND_NR, ifaces.ColIDf("%s_%s", NAME_BLS_PAIR, "GNARK_IS_ACTIVE_G2_MEMBERSHIP"), max(src.SuccessBit.Size(), src.CsG2Membership.Size())),
		MaxNbPairInputs:           maxNbPairInputs,
	}

	for i := range ucmd.PointG1 {
		ucmd.PointG1[i] = createColFnUa(fmt.Sprintf("POINT_G1_%d", i))
	}
	for i := range ucmd.PointG2 {
		ucmd.PointG2[i] = createColFnUa(fmt.Sprintf("POINT_G2_%d", i))
	}
	for i := range ucmd.PrevAccumulator {
		ucmd.PrevAccumulator[i] = createColFnUa(fmt.Sprintf("PREV_ACCUMULATOR_%d", i))
	}
	for i := range ucmd.CurrentAccumulator {
		ucmd.CurrentAccumulator[i] = createColFnUa(fmt.Sprintf("CURRENT_ACCUMULATOR_%d", i))
	}
	for i := range ucmd.ExpectedResult {
		ucmd.ExpectedResult[i] = createColFnUa(fmt.Sprintf("EXPECTED_RESULT_%d", i))
	}

	// constraint: IS_PAIR_DATA || IS_PAIR_RES
	// constraint: SUCCESS_BIT && (IS_PAIR_DATA || IS_PAIR_RES)
	ucmd.csIsPairingInstance(comp)
	// non-membership input mask
	ucmd.csInputMask(comp)
	// projection from source to unaligned
	ucmd.csProjectionUnaligned(comp)
	ucmd.csProjectionGnarkDataMillerLoop(comp)
	ucmd.csProjectionGnarkDataFinalExp(comp)
	// first line is correct
	ucmd.csAccumulatorInit(comp)
	// accumulator consistency
	ucmd.csAccumulatorConsistency(comp)

	return ucmd
}

func (c *UnalignedPairData) csIsPairingInstance(comp *wizard.CompiledIOP) {
	// IsPairingInstance = IsData OR IsRes
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_IS_PAIRING_INSTANCE", NAME_BLS_PAIR),
		sym.Sub(
			c.IsPairingInstance,
			c.IsData,
			c.IsRes,
		),
	)
	// IsPairingAndSuccess = SuccessBit AND (IsData OR IsRes)
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_IS_PAIRING_AND_SUCCESS", NAME_BLS_PAIR),
		sym.Sub(
			c.IsPairingAndSuccess,
			sym.Mul(
				c.SuccessBit,
				c.IsPairingInstance,
			),
		),
	)
}

func (d *UnalignedPairData) csInputMask(comp *wizard.CompiledIOP) {
	// assert that the GnarkIsActiveG1Membership and GnarkIsActiveG2Membership
	// are set correctly. We only call the subgroup membership for
	// non-membership checks (all membership checks are done inside Miller
	// Loop/final exp circuits). Thus it is:
	//    GnarkIsActiveG?Membership = CsG?Membership AND !SuccessBit
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_G1_MEMBERSHIP_AND_UNSUCCESSFUL", NAME_UNALIGNED_PAIR),
		sym.Sub(
			d.GnarkIsActiveG1Membership,
			sym.Mul(
				d.CsG1Membership,
				sym.Sub(1, d.IsPairingAndSuccess),
			),
		),
	)
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_G2_MEMBERSHIP_AND_UNSUCCESSFUL", NAME_UNALIGNED_PAIR),
		sym.Sub(
			d.GnarkIsActiveG2Membership,
			sym.Mul(
				d.CsG2Membership,
				sym.Sub(1, d.IsPairingAndSuccess),
			),
		),
	)
}

func (d *UnalignedPairData) csProjectionUnaligned(comp *wizard.CompiledIOP) {
	filtersB := make([]ifaces.Column, nbG1Limbs+nbG2Limbs+2)
	columnsB := make([][]ifaces.Column, nbG1Limbs+nbG2Limbs+2)
	for i := range nbG1Limbs {
		filtersB[i] = d.IsActive
		columnsB[i] = []ifaces.Column{d.PointG1[i]}
	}
	for i := range nbG2Limbs {
		filtersB[nbG1Limbs+i] = d.IsActive
		columnsB[nbG1Limbs+i] = []ifaces.Column{d.PointG2[i]}
	}
	for i := range 2 {
		filtersB[nbG1Limbs+nbG2Limbs+i] = d.IsLastLine
		columnsB[nbG1Limbs+nbG2Limbs+i] = []ifaces.Column{d.ExpectedResult[i]}
	}
	prj := query.ProjectionMultiAryInput{
		FiltersA: []ifaces.Column{d.IsPairingAndSuccess},
		FiltersB: filtersB,
		ColumnsA: [][]ifaces.Column{{d.BlsPairDataSource.Limb}},
		ColumnsB: columnsB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJECTION_DATA", NAME_UNALIGNED_PAIR), prj)
}

func (d *UnalignedPairData) csProjectionGnarkDataMillerLoop(comp *wizard.CompiledIOP) {
	// we map everything except the last input to the Miller loop circuit. We
	// need to constrain the mask
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_NOT_LAST_LINE", NAME_UNALIGNED_PAIR),
		sym.Mul(
			d.IsActive,
			sym.Sub(1, d.IsLastLine, d.IsNotLastLine)),
	)

	filtersA := make([]ifaces.Column, nbG1Limbs+nbG2Limbs+2*nbGtLimbs)
	columnsA := make([][]ifaces.Column, nbG1Limbs+nbG2Limbs+2*nbGtLimbs)
	for i := range nbGtLimbs {
		filtersA[i] = d.IsNotLastLine
		columnsA[i] = []ifaces.Column{d.PrevAccumulator[i]}
	}
	for i := range nbG1Limbs {
		filtersA[nbGtLimbs+i] = d.IsNotLastLine
		columnsA[nbGtLimbs+i] = []ifaces.Column{d.PointG1[i]}
	}
	for i := range nbG2Limbs {
		filtersA[nbGtLimbs+nbG1Limbs+i] = d.IsNotLastLine
		columnsA[nbGtLimbs+nbG1Limbs+i] = []ifaces.Column{d.PointG2[i]}
	}
	for i := range nbGtLimbs {
		filtersA[nbGtLimbs+nbG1Limbs+nbG2Limbs+i] = d.IsNotLastLine
		columnsA[nbGtLimbs+nbG1Limbs+nbG2Limbs+i] = []ifaces.Column{d.CurrentAccumulator[i]}
	}
	prj := query.ProjectionMultiAryInput{
		FiltersA: filtersA,
		FiltersB: []ifaces.Column{d.GnarkIsActiveMillerLoop},
		ColumnsA: columnsA,
		ColumnsB: [][]ifaces.Column{{d.GnarkDataMillerLoop}},
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJECTION_ML_DATA", NAME_UNALIGNED_PAIR), prj)
}

func (d *UnalignedPairData) csProjectionGnarkDataFinalExp(comp *wizard.CompiledIOP) {
	filtersA := make([]ifaces.Column, nbGtLimbs+nbG1Limbs+nbG2Limbs+2)
	columnsA := make([][]ifaces.Column, nbGtLimbs+nbG1Limbs+nbG2Limbs+2)
	for i := range nbGtLimbs {
		filtersA[i] = d.IsLastLine
		columnsA[i] = []ifaces.Column{d.PrevAccumulator[i]}
	}
	for i := range nbG1Limbs {
		filtersA[nbGtLimbs+i] = d.IsLastLine
		columnsA[nbGtLimbs+i] = []ifaces.Column{d.PointG1[i]}
	}
	for i := range nbG2Limbs {
		filtersA[nbGtLimbs+nbG1Limbs+i] = d.IsLastLine
		columnsA[nbGtLimbs+nbG1Limbs+i] = []ifaces.Column{d.PointG2[i]}
	}
	for i := range 2 {
		filtersA[nbGtLimbs+nbG1Limbs+nbG2Limbs+i] = d.IsLastLine
		columnsA[nbGtLimbs+nbG1Limbs+nbG2Limbs+i] = []ifaces.Column{d.ExpectedResult[i]}
	}
	prj := query.ProjectionMultiAryInput{
		FiltersA: filtersA,
		FiltersB: []ifaces.Column{d.GnarkIsActiveFinalExp},
		ColumnsA: columnsA,
		ColumnsB: [][]ifaces.Column{{d.GnarkDataFinalExp}},
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_PROJECTION_FE_DATA", NAME_UNALIGNED_PAIR), prj)
}

func (d *UnalignedPairData) csAccumulatorInit(comp *wizard.CompiledIOP) {
	// ensures that the first line accumulator is zero in Gt
	for i := range nbGtLimbs {
		if i == 3 {
			// the first line accumulator is zero in Gt
			comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_ACCUMULATOR_INIT_%d_ONE", NAME_UNALIGNED_PAIR, i),
				sym.Mul(sym.Sub(1, d.PrevAccumulator[i]), d.IsFirstLine))
		} else {
			comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_ACCUMULATOR_INIT_%d_ZERO", NAME_UNALIGNED_PAIR, i),
				sym.Mul(d.PrevAccumulator[i], d.IsFirstLine))
		}
	}
}

func (d *UnalignedPairData) csAccumulatorConsistency(comp *wizard.CompiledIOP) {
	// ensure that the current accumulator is equal to the next accumulator on previous line.
	// we need to cancel out if current line is the first line where the current accumulator is zero
	// (checked in [UnalignedPairData.csAccumulatorInit])
	for i := range nbGtLimbs {
		comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_ACCUMULATOR_CONSISTENCY_%d", NAME_UNALIGNED_PAIR, i),
			sym.Mul(
				d.IsActive,
				sym.Sub(1, d.IsFirstLine),
				sym.Sub(d.PrevAccumulator[i], column.Shift(d.CurrentAccumulator[i], -1)),
			),
		)
	}
}

func (d *UnalignedPairData) Assign(run *wizard.ProverRuntime) {
	d.assignMembershipMask(run)
	d.assignUnaligned(run)
	d.assignGnarkData(run)
}

func (d *UnalignedPairData) assignMembershipMask(run *wizard.ProverRuntime) {
	// assigns the masks (CS_G1_MEMBERSHIP AND !SUCCESS_BIT) and
	// (CS_G2_MEMBERSHIP AND !SUCCESS_BIT) columns which are used for filtering
	// the inputs going to group non-membership circuit.
	var (
		srcIsData         = d.IsData.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsRes          = d.IsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCsG1Membership = d.CsG1Membership.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCsG2Membership = d.CsG2Membership.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcSuccessBit     = d.SuccessBit.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	var (
		dstIsPairingInstance   = common.NewVectorBuilder(d.IsPairingInstance)
		dstIsPairingAndSuccess = common.NewVectorBuilder(d.IsPairingAndSuccess)
		dstG1Membership        = common.NewVectorBuilder(d.GnarkIsActiveG1Membership)
		dstG2Membership        = common.NewVectorBuilder(d.GnarkIsActiveG2Membership)
	)

	for i := range max(len(srcIsData), len(srcIsRes)) {
		dstIsPairingInstance.PushBoolean(srcIsData[i].IsOne() || srcIsRes[i].IsOne())
	}
	for i := range max(len(srcIsData), len(srcIsRes), len(srcSuccessBit)) {
		dstIsPairingAndSuccess.PushBoolean((srcIsData[i].IsOne() || srcIsRes[i].IsOne()) && srcSuccessBit[i].IsOne())
	}

	for i := range srcCsG1Membership {
		dstG1Membership.PushBoolean((srcIsData[i].IsOne() || srcIsRes[i].IsOne()) && srcCsG1Membership[i].IsOne() && srcSuccessBit[i].IsZero())
	}
	for i := range srcCsG2Membership {
		dstG2Membership.PushBoolean((srcIsData[i].IsOne() || srcIsRes[i].IsOne()) && srcCsG2Membership[i].IsOne() && srcSuccessBit[i].IsZero())
	}

	dstIsPairingInstance.PadAndAssign(run, field.Zero())
	dstIsPairingAndSuccess.PadAndAssign(run, field.Zero())
	dstG1Membership.PadAndAssign(run, field.Zero())
	dstG2Membership.PadAndAssign(run, field.Zero())
}

func (d *UnalignedPairData) assignUnaligned(run *wizard.ProverRuntime) {
	var (
		srcID         = d.ID.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcSuccessBit = d.SuccessBit.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcLimb       = d.Limb.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIndex      = d.Index.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCounter    = d.Counter.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsData     = d.IsData.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsRes      = d.IsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	var (
		dstIsActive      = common.NewVectorBuilder(d.IsActive)
		dstIsFirstLine   = common.NewVectorBuilder(d.IsFirstLine)
		dstIsLastLine    = common.NewVectorBuilder(d.IsLastLine)
		dstIsNotLastLine = common.NewVectorBuilder(d.IsNotLastLine)
	)

	var dstPointG1 [nbG1Limbs]*common.VectorBuilder
	var dstPointG2 [nbG2Limbs]*common.VectorBuilder
	var dstPrevAccumulator [nbGtLimbs]*common.VectorBuilder
	var dstCurrentAccumulator [nbGtLimbs]*common.VectorBuilder
	var dstExpectedResult [2]*common.VectorBuilder

	for i := range d.PointG1 {
		dstPointG1[i] = common.NewVectorBuilder(d.PointG1[i])
	}
	for i := range d.PointG2 {
		dstPointG2[i] = common.NewVectorBuilder(d.PointG2[i])
	}
	for i := range d.PrevAccumulator {
		dstPrevAccumulator[i] = common.NewVectorBuilder(d.PrevAccumulator[i])
	}
	for i := range d.CurrentAccumulator {
		dstCurrentAccumulator[i] = common.NewVectorBuilder(d.CurrentAccumulator[i])
	}
	for i := range d.ExpectedResult {
		dstExpectedResult[i] = common.NewVectorBuilder(d.ExpectedResult[i])
	}

	var ptr int

	for ptr < len(srcLimb) {
		// we detect if we're in a new pairing check instance
		if !(srcIsData[ptr].IsOne() && srcIndex[ptr].IsZero() && srcCounter[ptr].IsZero()) {
			ptr++
			continue
		}
		// the inputs which go to the pairing circuit are:
		//  - valid nontrivial input (cs_pairing_check = 1)
		//  - valid half-trivial ((0,Q) or (P,0)) input (cs_pairing_check = 0 and cs_gN_membership = 1 and success_bit = 1)
		//  - valid trivial input (0,0) (cs_pairing_check=0, cs_gn_membership=0, success_bit = 1)
		// in contrast, the inputs which go to the group membership circuit are:
		//  - invalid nontrivial input (cs_gN_membership = 1 and success_bit = 0)
		//
		// now, we don't do anything with the inputs going to the group
		// membership circuits, they are passed directly to the gnark circuit. From the above it implies that we send all
		// inputs with success_bit = 1 to here.

		// get the current ID to know when to stop
		currentID := srcID[ptr]
		// pair input index
		idx := 0
		// we initialize the current accumulator to zero
		prevAccumulator := nativeGtZero()
		var currentAccumulator []field.Element
		for ptr < len(srcID) && srcID[ptr].Equal(&currentID) {
			// if we have success_bit = 0, then we advance the pointer to the
			// next input. But we need to move the pointer according to if we
			// are in data or result part.
			if srcSuccessBit[ptr].IsZero() {
				switch {
				case srcIsData[ptr].IsOne():
					ptr += nbG1Limbs + nbG2Limbs
				case srcIsRes[ptr].IsOne():
					ptr += 2
				default:
					// otherwise, its not a given pairing instance. The input
					// should have ended, but we scan through just in case.
					ptr++
				}
				continue
			}
			// now, the success_bit is 1. If we are in the data part, then we copy the point limbs,
			// otherwise we copy the expected result limbs.
			if srcIsData[ptr].IsOne() {
				for i := range nbG1Limbs {
					dstPointG1[i].PushField(srcLimb[ptr+i])
				}
				for i := range nbG2Limbs {
					dstPointG2[i].PushField(srcLimb[ptr+nbG1Limbs+i])
				}
				dstIsLastLine.PushZero()
				dstIsNotLastLine.PushOne()
				// compute the next accumulator
				currentAccumulator = nativeMillerLoopAndSum(prevAccumulator, srcLimb[ptr:ptr+nbG1Limbs], srcLimb[ptr+nbG1Limbs:ptr+nbG1Limbs+nbG2Limbs])
				// copy the accumulator limbs
				for i := range nbGtLimbs {
					dstPrevAccumulator[i].PushField(prevAccumulator[i])
					dstCurrentAccumulator[i].PushField(currentAccumulator[i])
				}
				prevAccumulator = currentAccumulator
				for i := range 2 {
					dstExpectedResult[i].PushZero()
				}
				dstIsActive.PushOne()
				if idx == 0 {
					dstIsFirstLine.PushOne()
				} else {
					dstIsFirstLine.PushZero()
				}
				idx++
				ptr += nbG1Limbs + nbG2Limbs
			} else if srcIsRes[ptr].IsOne() {
				// we are in the result part. However, we don't add new line but reuse the last last line.
				// thus we need to pop the data before pushing the expected result limbs.
				dstIsLastLine.Pop()
				dstIsLastLine.PushOne()
				dstIsNotLastLine.Pop()
				dstIsNotLastLine.PushZero()
				for i := range 2 {
					dstExpectedResult[i].Pop()
					dstExpectedResult[i].PushField(srcLimb[ptr+i])
				}
				// additionally, we have pushed the current accumulator in the data part, but we don't need it here. So we pop it.
				for i := range nbGtLimbs {
					dstCurrentAccumulator[i].Pop()
					dstCurrentAccumulator[i].PushZero()
				}
				ptr += 2
			} else {
				utils.Panic("unexpected state in BlsPair assignUnaligned")
			}
		}
	}
	dstIsActive.PadAndAssign(run, field.Zero())
	dstIsFirstLine.PadAndAssign(run, field.Zero())
	dstIsLastLine.PadAndAssign(run, field.Zero())
	dstIsNotLastLine.PadAndAssign(run, field.Zero())
	for i := range d.PointG1 {
		dstPointG1[i].PadAndAssign(run, field.Zero())
	}
	for i := range d.PointG2 {
		dstPointG2[i].PadAndAssign(run, field.Zero())
	}
	for i := range d.PrevAccumulator {
		dstPrevAccumulator[i].PadAndAssign(run, field.Zero())
	}
	for i := range d.CurrentAccumulator {
		dstCurrentAccumulator[i].PadAndAssign(run, field.Zero())
	}
	for i := range d.ExpectedResult {
		dstExpectedResult[i].PadAndAssign(run, field.Zero())
	}
}

func (d *UnalignedPairData) assignGnarkData(run *wizard.ProverRuntime) {
	var (
		srcIsActive   = d.IsActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsLastLine = d.IsLastLine.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcPointG1    = make([][]field.Element, nbG1Limbs)
		srcPointG2    = make([][]field.Element, nbG2Limbs)
		srcPrev       = make([][]field.Element, nbGtLimbs)
		srcCurrent    = make([][]field.Element, nbGtLimbs)
		srcExpected   = make([][]field.Element, 2)
	)

	for i := range nbG1Limbs {
		srcPointG1[i] = d.PointG1[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}
	for i := range nbG2Limbs {
		srcPointG2[i] = d.PointG2[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}
	for i := range nbGtLimbs {
		srcPrev[i] = d.PrevAccumulator[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCurrent[i] = d.CurrentAccumulator[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}
	for i := range 2 {
		srcExpected[i] = d.ExpectedResult[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	var (
		dstGnarkIsActiveMillerLoop = common.NewVectorBuilder(d.GnarkIsActiveMillerLoop)
		dstGnarkDataMillerLoop     = common.NewVectorBuilder(d.GnarkDataMillerLoop)
		dstGnarkIsActiveFinalExp   = common.NewVectorBuilder(d.GnarkIsActiveFinalExp)
		dstGnarkDataFinalExp       = common.NewVectorBuilder(d.GnarkDataFinalExp)
	)

	for i := range srcIsActive {
		if !srcIsActive[i].IsOne() {
			continue
		}
		if srcIsLastLine[i].IsZero() {
			// we need to pass data to the Miller loop circuit
			for j := range nbGtLimbs {
				dstGnarkDataMillerLoop.PushField(srcPrev[j][i])
				dstGnarkIsActiveMillerLoop.PushOne()
			}
			for j := range nbG1Limbs {
				dstGnarkDataMillerLoop.PushField(srcPointG1[j][i])
				dstGnarkIsActiveMillerLoop.PushOne()
			}
			for j := range nbG2Limbs {
				dstGnarkDataMillerLoop.PushField(srcPointG2[j][i])
				dstGnarkIsActiveMillerLoop.PushOne()
			}
			for j := range nbGtLimbs {
				dstGnarkDataMillerLoop.PushField(srcCurrent[j][i])
				dstGnarkIsActiveMillerLoop.PushOne()
			}
		} else {
			// we need to pass data to the final exponentiation circuit
			for j := range nbGtLimbs {
				dstGnarkDataFinalExp.PushField(srcPrev[j][i])
				dstGnarkIsActiveFinalExp.PushOne()
			}
			for j := range nbG1Limbs {
				dstGnarkDataFinalExp.PushField(srcPointG1[j][i])
				dstGnarkIsActiveFinalExp.PushOne()
			}
			for j := range nbG2Limbs {
				dstGnarkDataFinalExp.PushField(srcPointG2[j][i])
				dstGnarkIsActiveFinalExp.PushOne()
			}
			for j := range 2 {
				dstGnarkDataFinalExp.PushField(srcExpected[j][i])
				dstGnarkIsActiveFinalExp.PushOne()
			}
		}
	}

	dstGnarkIsActiveMillerLoop.PadAndAssign(run, field.Zero())
	dstGnarkDataMillerLoop.PadAndAssign(run, field.Zero())
	dstGnarkIsActiveFinalExp.PadAndAssign(run, field.Zero())
	dstGnarkDataFinalExp.PadAndAssign(run, field.Zero())
}
