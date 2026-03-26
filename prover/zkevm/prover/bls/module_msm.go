package bls

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/sirupsen/logrus"
)

const (
	NAME_BLS_MSM       = "BLS_MSM"
	NAME_UNALIGNED_MSM = "UNALIGNED_BLS_MSM"
)

type BlsMsmDataSource struct {
	ID           ifaces.Column
	CsMul        ifaces.Column
	CsMembership ifaces.Column
	Limb         limbs.Uint128Le
	Index        ifaces.Column
	Counter      ifaces.Column
	IsData       ifaces.Column
	IsRes        ifaces.Column
}

func newMsmDataSource(comp *wizard.CompiledIOP, g Group, arith *arithmetization.Arithmetization) *BlsMsmDataSource {
	return &BlsMsmDataSource{
		ID:           arith.MashedColumnOf(comp, moduleName, "ID"),
		CsMul:        arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_BLS_"+g.String()+"_MSM"),
		CsMembership: arith.ColumnOf(comp, moduleName, "CIRCUIT_SELECTOR_"+g.String()+"_MEMBERSHIP"),
		Limb:         arith.GetLimbsOfU128Le(comp, moduleName, "LIMB"),
		Index:        arith.ColumnOf(comp, moduleName, "INDEX"),
		Counter:      arith.ColumnOf(comp, moduleName, "CT"),
		IsData:       arith.ColumnOf(comp, moduleName, "DATA_BLS_"+g.String()+"_MSM_FLAG"),
		IsRes:        arith.ColumnOf(comp, moduleName, "RSLT_BLS_"+g.String()+"_MSM_FLAG"),
	}
}

type BlsMsm struct {
	*BlsMsmDataSource
	*UnalignedMsmData
	AlignedGnarkMsmData             *plonk.Alignment
	AlignedGnarkGroupMembershipData *plonk.Alignment
	FlattenLimbsGroupMembership     *common.FlattenColumn
	*Limits
	Group
}

func newMsm(comp *wizard.CompiledIOP, g Group, limits *Limits, src *BlsMsmDataSource) *BlsMsm {
	umsm := newUnalignedMsmData(comp, g, limits, src)
	flattenLimbsGroupMembership := common.NewFlattenColumn(comp, src.Limb.AsDynSize(), umsm.IsMsmInstanceAndMembership)

	res := &BlsMsm{
		BlsMsmDataSource:            src,
		UnalignedMsmData:            umsm,
		Limits:                      limits,
		Group:                       g,
		FlattenLimbsGroupMembership: flattenLimbsGroupMembership,
	}

	flattenLimbsGroupMembership.CsFlattenProjection(comp)
	return res
}

func (bm *BlsMsm) WithMsmCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsMsm {
	if bm.MaxNbMsmInputs == 0 {
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsMsm: omitting MSM circuit for group %s as limit is 0", bm.Group.String())
		return bm
	}
	nbCircuits := utils.DivCeil(bm.MaxNbMsmInputs, bm.Limits.nbMulInputInstances(bm.Group))
	toAlignMsm := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_%s_MSM", NAME_BLS_MSM, bm.Group.String()),
		Round:              ROUND_NR,
		DataToCircuitMask:  bm.UnalignedMsmData.GnarkIsActiveMsm,
		DataToCircuit:      bm.UnalignedMsmData.GnarkDataMsm,
		Circuit:            newMulCircuit(bm.Group, bm.Limits),
		NbCircuitInstances: nbCircuits,
		PlonkOptions:       options,
	}

	bm.AlignedGnarkMsmData = plonk.DefineAlignment(comp, toAlignMsm)
	return bm
}

func (bm *BlsMsm) WithGroupMembershipCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *BlsMsm {
	// compute the bound on the number of circuits we need. First we estimate a bound on the number of possible
	// maximum number of G1/G2 points which could go to the membership circuit.
	nbMaxInstancesInputs := utils.DivCeil(bm.FlattenLimbsGroupMembership.Mask.Size(), nbLimbs(bm.Group))
	nbMaxInstancesLimit := bm.limitGroupMembershipCalls(bm.Group)
	switch nbMaxInstancesLimit {
	case 0:
		// if limit is 0, then we omit the circuit
		logrus.Warnf("BlsMsm: omitting group membership circuit for group %s as limit is 0", bm.Group.String())
		return bm
	case -1:
		// if limit is -1, then we take all the inputs
		nbMaxInstancesLimit = nbMaxInstancesInputs
	}
	maxNbInstances := min(nbMaxInstancesInputs, nbMaxInstancesLimit)
	// and by knowing how many inputs every circuit takes, we can bound the number of circuits as well
	nbCircuits := utils.DivCeil(maxNbInstances, bm.Limits.nbGroupMembershipInputInstances(bm.Group))
	toAlignMembership := &plonk.CircuitAlignmentInput{
		Name:               fmt.Sprintf("%s_%s_GROUP_MEMBERSHIP", NAME_BLS_MSM, bm.Group.String()),
		Round:              ROUND_NR,
		DataToCircuitMask:  bm.FlattenLimbsGroupMembership.Mask,
		DataToCircuit:      bm.FlattenLimbsGroupMembership.Limbs,
		Circuit:            newCheckCircuit(bm.Group, GROUP, bm.Limits),
		NbCircuitInstances: nbCircuits,
		InputFillerKey:     membershipInputFillerKey(bm.Group, GROUP),
		PlonkOptions:       options,
	}

	bm.AlignedGnarkGroupMembershipData = plonk.DefineAlignment(comp, toAlignMembership)
	return bm
}

func (bm *BlsMsm) Assign(run *wizard.ProverRuntime) {
	bm.UnalignedMsmData.Assign(run)
	bm.FlattenLimbsGroupMembership.Run(run)
	if bm.AlignedGnarkMsmData != nil {
		bm.AlignedGnarkMsmData.Assign(run)
	}
	if bm.AlignedGnarkGroupMembershipData != nil {
		bm.AlignedGnarkGroupMembershipData.Assign(run)
	}
}

type UnalignedMsmData struct {
	*BlsMsmDataSource
	IsMsmInstanceAndMembership ifaces.Column // indicates if line is part of an MSM instance and group membership check
	IsDataAndCsMul             ifaces.Column // indicates if source is data and has CS_MSM set
	IsResultAndCsMul           ifaces.Column // indicates if source is result and has CS_MSM set
	// this part is used to define the accumulators and indicate if the
	IsActive           ifaces.Column
	IsFirstLine        ifaces.Column
	IsLastLine         ifaces.Column
	Scalar             [nbFrLimbs]ifaces.Column
	Point              []ifaces.Column // length nbG1Limbs or nbG2Limbs
	CurrentAccumulator []ifaces.Column // length nbG1Limbs or nbG2Limbs
	NextAccumulator    []ifaces.Column // length nbG1Limbs or nbG2Limbs

	// data which is projected from above columns going into the MSM circuit
	GnarkIsActiveMsm ifaces.Column
	GnarkDataMsm     ifaces.Column

	Group          Group
	MaxNbMsmInputs int
}

func newUnalignedMsmData(comp *wizard.CompiledIOP, g Group, limits *Limits, src *BlsMsmDataSource) *UnalignedMsmData {
	// obtain the maximum number of rows which are coming from the arithmetization.
	maxNbRows := max(src.CsMul.Size(), src.IsData.Size(), src.IsRes.Size())
	// assuming the worst case where there is single long MSM. Then we have
	// group element and scalar for every input. And we add one to avoid edge
	// case with 0 size.
	maxNbMsmInstancesInputs := utils.DivCeil(src.CsMul.Size(), (nbLimbs128(g) + nbFrLimbs128))
	maxNbInstancesLimit := limits.limitMulCalls(g)
	if maxNbInstancesLimit == -1 {
		// if limit is -1, then we take all the inputs
		maxNbInstancesLimit = maxNbMsmInstancesInputs
	}
	maxNbMsmInstances := min(maxNbMsmInstancesInputs, maxNbInstancesLimit)
	// and all witness elements for the gnark circuits are expanded as we have interleaved with accumulators
	maxNbRowsAligned := maxNbMsmInstancesInputs * nbRowsPerMul(g)

	createCol1 := createColFn(comp, fmt.Sprintf("UNALIGNED_%s_BLS_MSM", g.String()), utils.NextPowerOfTwo(maxNbMsmInstancesInputs))
	createCol2 := createColFn(comp, fmt.Sprintf("UNALIGNED_%s_BLS_MSM", g.String()), utils.NextPowerOfTwo(maxNbRowsAligned))
	res := &UnalignedMsmData{
		BlsMsmDataSource:           src,
		IsMsmInstanceAndMembership: comp.InsertCommit(ROUND_NR, ifaces.ColIDf("UNALIGNED_%s_BLS_MSM_IS_MSM_AND_MEMBERSHIP", g.String()), maxNbRows, true),
		IsDataAndCsMul:             comp.InsertCommit(ROUND_NR, ifaces.ColIDf("UNALIGNED_%s_BLS_MSM_SRC_IS_DATA_AND_CS_MSM", g.String()), maxNbRows, true),
		IsResultAndCsMul:           comp.InsertCommit(ROUND_NR, ifaces.ColIDf("UNALIGNED_%s_BLS_MSM_SRC_IS_RESULT_AND_CS_MSM", g.String()), maxNbRows, true),
		IsActive:                   createCol1("IS_ACTIVE"),
		Point:                      make([]ifaces.Column, nbLimbs(g)),
		CurrentAccumulator:         make([]ifaces.Column, nbLimbs(g)),
		NextAccumulator:            make([]ifaces.Column, nbLimbs(g)),
		IsFirstLine:                createCol1("IS_FIRST_LINE"),
		IsLastLine:                 createCol1("IS_LAST_LINE"),
		GnarkIsActiveMsm:           createCol2("GNARK_IS_ACTIVE_MSM"),
		GnarkDataMsm:               createCol2("GNARK_DATA_MSM"),
		Group:                      g,
		MaxNbMsmInputs:             maxNbMsmInstances,
	}

	for i := range res.Scalar {
		res.Scalar[i] = createCol1(fmt.Sprintf("SCALAR_%d", i))
	}
	for i := range res.Point {
		res.Point[i] = createCol1(fmt.Sprintf("POINT_%d", i))
	}
	for i := range res.CurrentAccumulator {
		res.CurrentAccumulator[i] = createCol1(fmt.Sprintf("CURRENT_ACCUMULATOR_%d", i))
	}
	for i := range res.NextAccumulator {
		res.NextAccumulator[i] = createCol1(fmt.Sprintf("NEXT_ACCUMULATOR_%d", i))
	}

	res.csInputMasks(comp)
	// data projection
	res.csProjectionData(comp)
	// result projection
	res.csProjectionResult(comp)
	// gnark data projection
	res.csProjectionGnarkData(comp)
	// first line accumulator zero
	res.csAccumulatorInit(comp)
	// accumulator consistency
	res.csAccumulatorConsistency(comp)

	return res
}

func (d *UnalignedMsmData) csInputMasks(comp *wizard.CompiledIOP) {
	// constraint: IS_MSM_AND_MEMBERSHIP == IS_MSM_DATA && IS_MEMBERSHIP
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_%s_IS_MSM_AND_MEMBERSHIP", NAME_UNALIGNED_MSM, d.Group.String()), sym.Sub(d.IsMsmInstanceAndMembership, sym.Mul(d.IsData, d.CsMembership)))
	// we need to compute the IS_DATA && CS_MUL column which is used for projection
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_%s_IS_DATA_AND_CS_MUL", NAME_UNALIGNED_MSM, d.Group.String()), sym.Sub(d.IsDataAndCsMul, sym.Mul(d.IsData, d.CsMul)))
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_%s_IS_RESULT_AND_CS_MUL", NAME_UNALIGNED_MSM, d.Group.String()), sym.Sub(d.IsResultAndCsMul, sym.Mul(d.IsRes, d.CsMul)))
}

func (d *UnalignedMsmData) csProjectionData(comp *wizard.CompiledIOP) {
	// ensures that the data limbs from source are projected into columns of the
	// unaligned module properly. It additionally constraints IsActive to
	// correspond to the number of lines in the source.
	//
	// The source has multiple limb columns per row (obtained via Limbs()), and we need to
	// project multiple rows into a single row of destination columns (Point and Scalar).
	// Both sides must have the same number of columns per table.
	//
	// The projection reads left-to-right then top-to-bottom. With the source having
	// nbLimbsPerRow columns, we read all columns in a row, then move to the next row.
	// For each MSM input, we have nbRowsPerInput source rows that map to 1 destination row.
	//
	// The source limbs are stored in little-endian order, but the destination (Point/Scalar)
	// expects big-endian order within each 128-bit chunk. So we convert to big-endian.
	nbL := nbLimbs(d.Group)
	srcLimbs := d.BlsMsmDataSource.Limb.ToBigEndianUint().GetLimbs()
	nbLimbsPerRow := len(srcLimbs)

	// ColumnsA: single table with nbLimbsPerRow columns, reads left-to-right then top-to-bottom
	// For each destination row, we read nbRowsPerInput source rows
	filtersA := []ifaces.Column{d.IsDataAndCsMul}
	columnsA := [][]ifaces.Column{srcLimbs}

	// ColumnsB: single table with nbLimbsPerRow columns
	// But we have nbL+nbFrLimbs destination columns total, so we need multiple tables
	// Each table corresponds to one "row" of source data mapped to part of the destination
	nbRowsPerInput := (nbL + nbFrLimbs) / nbLimbsPerRow
	allDstCols := make([]ifaces.Column, nbL+nbFrLimbs)
	copy(allDstCols[:nbL], d.Point)
	copy(allDstCols[nbL:], d.Scalar[:])

	filtersB := make([]ifaces.Column, nbRowsPerInput)
	columnsB := make([][]ifaces.Column, nbRowsPerInput)
	for i := range nbRowsPerInput {
		filtersB[i] = d.IsActive
		columnsB[i] = allDstCols[i*nbLimbsPerRow : (i+1)*nbLimbsPerRow]
	}

	prj := query.ProjectionMultiAryInput{
		FiltersA: filtersA,
		FiltersB: filtersB,
		ColumnsA: columnsA,
		ColumnsB: columnsB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_%s_PROJECTION_DATA", NAME_UNALIGNED_MSM, d.Group.String()), prj)
}

func (d *UnalignedMsmData) csProjectionResult(comp *wizard.CompiledIOP) {
	nbL := nbLimbs(d.Group)
	srcLimbs := d.BlsMsmDataSource.Limb.ToBigEndianUint().GetLimbs()
	nbLimbsPerRow := len(srcLimbs)
	nbRowsPerResult := nbL / nbLimbsPerRow

	// ColumnsA: single table with nbLimbsPerRow columns
	filtersA := []ifaces.Column{d.IsResultAndCsMul}
	columnsA := [][]ifaces.Column{srcLimbs}

	// ColumnsB: multiple tables, each with nbLimbsPerRow columns
	filtersB := make([]ifaces.Column, nbRowsPerResult)
	columnsB := make([][]ifaces.Column, nbRowsPerResult)
	for i := range nbRowsPerResult {
		filtersB[i] = d.IsLastLine
		columnsB[i] = d.NextAccumulator[i*nbLimbsPerRow : (i+1)*nbLimbsPerRow]
	}

	prj := query.ProjectionMultiAryInput{
		FiltersA: filtersA,
		FiltersB: filtersB,
		ColumnsA: columnsA,
		ColumnsB: columnsB,
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_%s_PROJECTION_RESULT", NAME_UNALIGNED_MSM, d.Group.String()), prj)
}

func (d *UnalignedMsmData) csProjectionGnarkData(comp *wizard.CompiledIOP) {
	// Projects from row-format columns (CurrentAccumulator, Scalar, Point, NextAccumulator)
	// to the single GnarkDataMsm column. The order matches assignGnarkData:
	// CurrentAccumulator, Scalar, Point, NextAccumulator
	// Within each 128-bit chunk (8 limbs), we reverse the order to convert from
	// little-endian (source) to big-endian (gnark circuit expects).
	nbL := nbLimbs(d.Group)
	totalCols := nbL + nbFrLimbs + nbL + nbL // CurrentAccumulator + Scalar + Point + NextAccumulator

	filtersA := make([]ifaces.Column, totalCols)
	columnsA := make([][]ifaces.Column, totalCols)

	// Helper to reverse endianness within each 128-bit chunk
	reversedIdx := func(j int) int {
		chunkIdx := j / limbs.NbLimbU128
		withinChunk := j % limbs.NbLimbU128
		reversedWithinChunk := limbs.NbLimbU128 - 1 - withinChunk
		return chunkIdx*limbs.NbLimbU128 + reversedWithinChunk
	}

	offset := 0
	// CurrentAccumulator - reverse within each 128-bit chunk
	for i := range nbL {
		filtersA[offset+i] = d.IsActive
		columnsA[offset+i] = []ifaces.Column{d.CurrentAccumulator[reversedIdx(i)]}
	}
	offset += nbL

	// Scalar - reverse within each 128-bit chunk
	for i := range nbFrLimbs {
		filtersA[offset+i] = d.IsActive
		columnsA[offset+i] = []ifaces.Column{d.Scalar[reversedIdx(i)]}
	}
	offset += nbFrLimbs

	// Point - reverse within each 128-bit chunk
	for i := range nbL {
		filtersA[offset+i] = d.IsActive
		columnsA[offset+i] = []ifaces.Column{d.Point[reversedIdx(i)]}
	}
	offset += nbL

	// NextAccumulator - reverse within each 128-bit chunk
	for i := range nbL {
		filtersA[offset+i] = d.IsActive
		columnsA[offset+i] = []ifaces.Column{d.NextAccumulator[reversedIdx(i)]}
	}

	prj := query.ProjectionMultiAryInput{
		FiltersA: filtersA,
		FiltersB: []ifaces.Column{d.GnarkIsActiveMsm},
		ColumnsA: columnsA,
		ColumnsB: [][]ifaces.Column{{d.GnarkDataMsm}},
	}
	comp.InsertProjection(ifaces.QueryIDf("%s_%s_PROJECTION_GNARK_DATA", NAME_UNALIGNED_MSM, d.Group.String()), prj)
}

func (d *UnalignedMsmData) csAccumulatorInit(comp *wizard.CompiledIOP) {
	// ensures that the first line accumulator is zero
	nbL := nbLimbs(d.Group)
	for i := range nbL {
		comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_%s_ACCUMULATOR_INIT_%d", NAME_UNALIGNED_MSM, d.Group.String(), i), sym.Mul(d.CurrentAccumulator[i], d.IsFirstLine))
	}
}

func (d *UnalignedMsmData) csAccumulatorConsistency(comp *wizard.CompiledIOP) {
	// ensure that the current accumulator is equal to the next accumulator on previous line.
	// we need to cancel out if current line is the first line where the current accumulator is zero
	// (checked in [UnalignedMsmData.csAccumulatorInit])
	nbL := nbLimbs(d.Group)
	for i := range nbL {
		comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%s_%s_ACCUMULATOR_CONSISTENCY_%d", NAME_UNALIGNED_MSM, d.Group.String(), i),
			sym.Mul(
				d.IsActive,
				sym.Sub(1, d.IsFirstLine),
				sym.Sub(d.CurrentAccumulator[i], column.Shift(d.NextAccumulator[i], -1)),
			),
		)
	}
}

func (d *UnalignedMsmData) Assign(run *wizard.ProverRuntime) {
	d.assignMasks(run)
	d.assignUnaligned(run)
	d.assignGnarkData(run)
}

func (d *UnalignedMsmData) assignMasks(run *wizard.ProverRuntime) {
	var (
		nbRows          = d.Limb.NumRow()
		srcIsData       = d.IsData.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCsMul        = d.CsMul.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsRes        = d.IsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsMembership = d.CsMembership.GetColAssignment(run).IntoRegVecSaveAlloc()
	)
	var (
		dstIsMsmAndMembership = common.NewVectorBuilder(d.IsMsmInstanceAndMembership)
		dstDataAndCs          = common.NewVectorBuilder(d.IsDataAndCsMul)
		dstIsResultAndCs      = common.NewVectorBuilder(d.IsResultAndCsMul)
	)
	// compute the IS_DATA && CS_MUL column which is used for projection
	for ptr := range nbRows {
		dstDataAndCs.PushBoolean(srcIsData[ptr].IsOne() && srcCsMul[ptr].IsOne())
		dstIsResultAndCs.PushBoolean(srcIsRes[ptr].IsOne() && srcCsMul[ptr].IsOne())
		dstIsMsmAndMembership.PushBoolean(srcIsMembership[ptr].IsOne() && srcIsData[ptr].IsOne())
	}
	dstDataAndCs.PadAndAssign(run, field.Zero())
	dstIsResultAndCs.PadAndAssign(run, field.Zero())
	dstIsMsmAndMembership.PadAndAssign(run, field.Zero())
}

func (d *UnalignedMsmData) assignUnaligned(run *wizard.ProverRuntime) {
	var (
		srcID      = expr_handle.GetExprHandleAssignment(run, d.ID).IntoRegVecSaveAlloc()
		srcLimb    = d.Limb.GetAssignment(run)
		nbRows     = d.Limb.NumRow()
		srcIndex   = d.Index.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCounter = d.Counter.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCsMul   = d.CsMul.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsData  = d.IsData.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcIsRes   = d.IsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	nbL := nbLimbs(d.Group)
	nbL128 := nbL / limbs.NbLimbU128
	var (
		dstIsActive = common.NewVectorBuilder(d.IsActive)

		dstIsFirstLine        = common.NewVectorBuilder(d.IsFirstLine)
		dstIsLastLine         = common.NewVectorBuilder(d.IsLastLine)
		dstScalar             = make([]*common.VectorBuilder, nbFrLimbs)
		dstPoint              = make([]*common.VectorBuilder, nbL)
		dstCurrentAccumulator = make([]*common.VectorBuilder, nbL)
		dstNextAccumulator    = make([]*common.VectorBuilder, nbL)
	)
	for i := range dstScalar {
		dstScalar[i] = common.NewVectorBuilder(d.Scalar[i])
	}
	for i := range nbL {
		dstPoint[i] = common.NewVectorBuilder(d.Point[i])
		dstCurrentAccumulator[i] = common.NewVectorBuilder(d.CurrentAccumulator[i])
		dstNextAccumulator[i] = common.NewVectorBuilder(d.NextAccumulator[i])
	}

	var ptr int
	pointLimbs := make([]field.Element, nbL)
	scalarLimbs := make([]field.Element, nbFrLimbs)

	for ptr < nbRows {
		// first detect if it is a new MSM instance.
		// We normally would only check is IsData=1 && Index=0 && Counter=0, but we also don't want to "open"
		// new instance here if the circuit selector to MSM is not set as it indicates MSM with invalid inputs, in which
		// case the invalid input will be sent to non-membership circuit.
		if !(srcIsData[ptr].IsOne() && srcIndex[ptr].IsZero() && srcCounter[ptr].IsZero() && srcCsMul[ptr].IsOne()) {
			ptr++
			continue
		}
		// we are now on the first line of a new MSM instance. Every MSM
		// instance is ((P1, n1), (P2, n2), ..., (Pk, nk), R), where Pi, ni are
		// the points and scalars and R is the result.

		// we get the current instance ID so that we know when to stop
		currentID := srcID[ptr]
		// MSM input index
		idx := 0
		// we initialize the current accumulator for computing the running sum
		currAccumulator := make([]field.Element, nbL)
		var nextAccumulator []field.Element

		// now we copy the actual
		for ptr < len(srcID) && currentID.Equal(&srcID[ptr]) {
			// we either have input or result limbs. Switch to see if we need to
			// copy point+scalar or only result limbs.
			switch {
			case srcIsData[ptr].IsOne():
				// copy the point limbs
				for i := range nbL {
					limbIndex := i / limbs.NbLimbU128
					subLimbIndex := (limbs.NbLimbU128 - 1) - (i % limbs.NbLimbU128)
					pointLimbs[i].Set(&srcLimb[ptr+limbIndex].T[subLimbIndex])
					dstPoint[i].PushField(pointLimbs[i])
				}
				// copy the scalar limbs
				for i := range nbFrLimbs {
					limbIndex := i / limbs.NbLimbU128
					subLimbIndex := (limbs.NbLimbU128 - 1) - (i % limbs.NbLimbU128)
					scalarLimbs[i].Set(&srcLimb[ptr+nbL128+limbIndex].T[subLimbIndex])
					dstScalar[i].PushField(scalarLimbs[i])
				}
				dstIsLastLine.PushZero()
				// compute the next accumulator
				nextAccumulator = nativeScalarMulAndSum(d.Group, currAccumulator, pointLimbs, scalarLimbs)
				for i := range nbL {
					// copy the next accumulator limbs
					dstNextAccumulator[i].PushField(nextAccumulator[i])
					// we also copy the current accumulator, which is the same as the next
					dstCurrentAccumulator[i].PushField(currAccumulator[i])
				}
				currAccumulator = nextAccumulator
				ptr += nbL128 + nbFrLimbs128
				dstIsActive.PushOne()
				if idx == 0 {
					dstIsFirstLine.PushOne()
				} else {
					dstIsFirstLine.PushZero()
				}
				idx++
			case srcIsRes[ptr].IsOne():
				// if it is the last line then we don't need to copy the result limbs - we have already computed it.
				// its consistency will be checked by gnark circuit and projection queries.
				dstIsLastLine.Pop()
				dstIsLastLine.PushOne()
				ptr += nbL128
			default:
				utils.Panic("unexpected state in BlsMsm assignUnaligned")
			}
		}
	}

	dstIsActive.PadAndAssign(run, field.Zero())
	dstIsFirstLine.PadAndAssign(run, field.Zero())
	dstIsLastLine.PadAndAssign(run, field.Zero())
	for i := range nbFrLimbs {
		dstScalar[i].PadAndAssign(run, field.Zero())
	}
	for i := range nbL {
		dstPoint[i].PadAndAssign(run, field.Zero())
		dstCurrentAccumulator[i].PadAndAssign(run, field.Zero())
		dstNextAccumulator[i].PadAndAssign(run, field.Zero())
	}
}

func (d *UnalignedMsmData) assignGnarkData(run *wizard.ProverRuntime) {
	nbL := nbLimbs(d.Group)

	// we now need to transpose again the limbs into the gnark input format.
	// This is essentially mapping the lines of current accumulator, point,
	// scalar and next accumulator into column.
	var (
		srcIsActive           = d.IsActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		srcScalar             = make([][]field.Element, nbFrLimbs)
		srcPoint              = make([][]field.Element, nbL)
		srcCurrentAccumulator = make([][]field.Element, nbL)
		srcNextAccumulator    = make([][]field.Element, nbL)
	)
	for i := range nbFrLimbs {
		srcScalar[i] = d.Scalar[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}
	for i := range nbL {
		srcPoint[i] = d.Point[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		srcCurrentAccumulator[i] = d.CurrentAccumulator[i].GetColAssignment(run).IntoRegVecSaveAlloc()
		srcNextAccumulator[i] = d.NextAccumulator[i].GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	var (
		dstDataMsm         = common.NewVectorBuilder(d.GnarkDataMsm)
		dstDataIsActiveMsm = common.NewVectorBuilder(d.GnarkIsActiveMsm)
	)
	// Helper to reverse endianness within each 128-bit chunk
	reversedIdx := func(j int) int {
		chunkIdx := j / limbs.NbLimbU128
		withinChunk := j % limbs.NbLimbU128
		reversedWithinChunk := limbs.NbLimbU128 - 1 - withinChunk
		return chunkIdx*limbs.NbLimbU128 + reversedWithinChunk
	}

	for i := range srcIsActive {
		if !srcIsActive[i].IsOne() {
			continue
		}
		// CurrentAccumulator - reverse within each 128-bit chunk
		for j := range nbL {
			dstDataMsm.PushField(srcCurrentAccumulator[reversedIdx(j)][i])
			dstDataIsActiveMsm.PushOne()
		}
		// Scalar - reverse within each 128-bit chunk
		for j := range nbFrLimbs {
			dstDataMsm.PushField(srcScalar[reversedIdx(j)][i])
			dstDataIsActiveMsm.PushOne()
		}
		// Point - reverse within each 128-bit chunk
		for j := range nbL {
			dstDataMsm.PushField(srcPoint[reversedIdx(j)][i])
			dstDataIsActiveMsm.PushOne()
		}
		// NextAccumulator - reverse within each 128-bit chunk
		for j := range nbL {
			dstDataMsm.PushField(srcNextAccumulator[reversedIdx(j)][i])
			dstDataIsActiveMsm.PushOne()
		}
	}

	dstDataMsm.PadAndAssign(run, field.Zero())
	dstDataIsActiveMsm.PadAndAssign(run, field.Zero())
}
