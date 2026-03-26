package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commoncs "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

var (
	SOURCE_ECRECOVER = field.NewElement(0)
	SOURCE_TX        = field.NewElement(1)
)

const (
	NB_ECRECOVER_INPUTS = 24
	NB_TX_INPUTS        = 15
)

type EcRecover struct {
	EcRecoverID     ifaces.Column
	Limb            limbs.Uint128Le
	SuccessBit      ifaces.Column
	EcRecoverIndex  ifaces.Column
	EcRecoverIsData ifaces.Column
	EcRecoverIsRes  ifaces.Column

	// mask for selecting columns in this submodule for projection query
	AuxProjectionMask ifaces.Column

	*Settings
}

type ecDataSource struct {
	CsEcrecover ifaces.Column
	ID          ifaces.Column
	Limb        limbs.Uint128Le
	SuccessBit  ifaces.Column
	Index       ifaces.Column
	IsData      ifaces.Column
	IsRes       ifaces.Column
}

func (ecSrc *ecDataSource) nbActualInstances(run *wizard.ProverRuntime) int {
	var (
		count     int = 0
		csCol         = ecSrc.CsEcrecover.GetColAssignment(run)
		indexCol      = ecSrc.Index.GetColAssignment(run)
		isDataCol     = ecSrc.IsData.GetColAssignment(run)
	)

	for i := 0; i < csCol.Len(); i++ {
		var (
			cs     = csCol.Get(i)
			index  = indexCol.Get(i)
			isData = isDataCol.Get(i)
		)

		if cs.IsOne() && index.IsZero() && isData.IsOne() {
			count++
		}
	}
	return count
}

func newEcRecover(comp *wizard.CompiledIOP, limits *Settings, src *ecDataSource) *EcRecover {
	size := limits.sizeAntichamber()
	createCol := createColFn(comp, NAME_ECRECOVER, limits.sizeAntichamber())
	res := &EcRecover{
		EcRecoverID:       createCol("ECRECOVER_ID"),
		SuccessBit:        createCol("SUCCESS_BIT"),
		EcRecoverIndex:    createCol("ECRECOVER_INDEX"),
		EcRecoverIsData:   createCol("ECRECOVER_IS_DATA"),
		EcRecoverIsRes:    createCol("ECRECOVER_IS_RES"),
		AuxProjectionMask: createCol("AUX_PROJECTION_MASK"),
		Limb:              limbs.NewUint128Le(comp, NAME_ECRECOVER+"_LIMB", size),
		Settings:          limits,
	}

	res.csEcDataProjection(comp, src)
	res.csConstraintAuxProjectionMask(comp)

	return res
}

func (ec *EcRecover) Assign(run *wizard.ProverRuntime, src *ecDataSource) {
	ec.assignFromEcDataSource(run, src)
}

func (ec *EcRecover) assignFromEcDataSource(run *wizard.ProverRuntime, src *ecDataSource) {

	var (
		sourceLimb = src.Limb.GetAssignmentAsByte16Exact(run)

		nbInstances       = src.nbActualInstances(run)
		currRow           = int(0)
		sourceCsEcRecover = run.GetColumn(src.CsEcrecover.GetColID())
		sourceID          = expr_handle.GetExprHandleAssignment(run, src.ID)
		sourceSuccessBit  = run.GetColumn(src.SuccessBit.GetColID())
		sourceIndex       = run.GetColumn(src.Index.GetColID())
		sourceIsData      = run.GetColumn(src.IsData.GetColID())
		sourceIsRes       = run.GetColumn(src.IsRes.GetColID())

		resEcRecoverID       = common.NewVectorBuilder(ec.EcRecoverID)
		resSuccessBit        = common.NewVectorBuilder(ec.SuccessBit)
		resEcRecoverIndex    = common.NewVectorBuilder(ec.EcRecoverIndex)
		resEcRecoverIsData   = common.NewVectorBuilder(ec.EcRecoverIsData)
		resEcRecoverIsRes    = common.NewVectorBuilder(ec.EcRecoverIsRes)
		resAuxProjectionMask = common.NewVectorBuilder(ec.AuxProjectionMask)
		resLimb              = limbs.NewVectorBuilder(ec.Limb.AsDynSize())
	)

	if sourceID.Len() != len(sourceLimb) || len(sourceLimb) != sourceSuccessBit.Len() {
		panic("all source limb columns must have the same length")
	}

	if sourceCsEcRecover.Len() != sourceID.Len() ||
		sourceSuccessBit.Len() != sourceIndex.Len() ||
		sourceIndex.Len() != sourceIsData.Len() ||
		sourceIsData.Len() != sourceIsRes.Len() {
		panic("all source columns must have the same length")
	}

	for i := 0; i < nbInstances; i++ {

		// Advance currRow to the next ECRECOVER segment without pushing zeros.
		// The ecrecover data is packed contiguously in the antichamber columns,
		// so we only need to skip non-ecrecover rows in the source — not mirror
		// them. The projection constraint uses filters, so layout alignment with
		// the source is not required.
		for ; currRow < sourceCsEcRecover.Len(); currRow++ {
			selected := sourceCsEcRecover.Get(currRow)
			if selected.IsOne() {
				break
			}
		}

		for j := 0; j < nbRowsPerEcRecFetching; j++ {
			sourceIdx := currRow + j
			resEcRecoverID.PushField(sourceID.Get(sourceIdx))
			resLimb.PushBytes16(sourceLimb[sourceIdx])
			resSuccessBit.PushField(sourceSuccessBit.Get(sourceIdx))
			resEcRecoverIndex.PushField(sourceIndex.Get(sourceIdx))
			resEcRecoverIsData.PushField(sourceIsData.Get(sourceIdx))
			resEcRecoverIsRes.PushField(sourceIsRes.Get(sourceIdx))
			resAuxProjectionMask.PushField(sourceCsEcRecover.Get(sourceIdx))
		}

		// Assign everything with zeroes outside of the fetching phase
		for j := nbRowsPerEcRecFetching; j < nbRowsPerEcRec; j++ {
			resEcRecoverID.PushZero()
			resLimb.PushZero()
			resSuccessBit.PushZero()
			resEcRecoverIndex.PushZero()
			resEcRecoverIsData.PushZero()
			resEcRecoverIsRes.PushZero()
			resAuxProjectionMask.PushZero()
		}

		// This ensures that the next iteration starts from the first position
		// after the ECRECOVER segment we just imported.
		currRow += nbRowsPerEcRecFetching
	}

	// assign this submodule components
	resEcRecoverID.PadAndAssign(run)
	resLimb.PadAndAssignZero(run)
	resSuccessBit.PadAndAssign(run)
	resEcRecoverIndex.PadAndAssign(run)
	resEcRecoverIsData.PadAndAssign(run)
	resEcRecoverIsRes.PadAndAssign(run)
	resAuxProjectionMask.PadAndAssign(run)
}

func (ec *EcRecover) csEcDataProjection(comp *wizard.CompiledIOP, src *ecDataSource) {
	columnsA := append(
		ec.Limb.ToLittleEndianLimbs().GetLimbs(),
		ec.EcRecoverID, ec.SuccessBit, ec.EcRecoverIndex, ec.EcRecoverIsData, ec.EcRecoverIsRes,
	)

	columnsB := append(
		src.Limb.ToLittleEndianLimbs().GetLimbs(),
		src.ID, src.SuccessBit, src.Index, src.IsData, src.IsRes,
	)

	comp.InsertProjection(ifaces.QueryIDf("%v_PROJECT_ECDATA", NAME_ECRECOVER),
		query.ProjectionInput{
			ColumnA: columnsA,
			ColumnB: columnsB,
			FilterA: ec.AuxProjectionMask,
			FilterB: src.CsEcrecover,
		},
	)
}

func (ec *EcRecover) csConstraintAuxProjectionMask(comp *wizard.CompiledIOP) {
	commoncs.MustBeBinary(comp, ec.AuxProjectionMask)
}

// TODO: must be called from the antichamber to ensure that the mask is consistent with the column in the root antichamber
func (ec *EcRecover) csConstrainAuxProjectionMaskConsistency(comp *wizard.CompiledIOP, sourceCol, isFetchingCol ifaces.Column) {
	comp.InsertGlobal(ROUND_NR, ifaces.QueryIDf("%v_%v", NAME_ECRECOVER, "CONSISTENCY_AUX_PROJECTION_MASK"),
		sym.Sub(sym.Mul(isFetchingCol, sym.Sub(1, sourceCol)), ec.AuxProjectionMask),
	)
}
