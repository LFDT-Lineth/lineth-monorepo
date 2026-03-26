package gen_acc

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// The sub-module GenericInfoAccumulator filters the data from different [generic.GenInfoModule],
//
//	and stitch them together to build a single module.
type GenericInfoAccumulator struct {
	Inputs *GenericAccumulatorInputs
	// stitching of modules together
	// HashHi and HashLo are over the same row
	// isHashHi = isHashLo = IsActive
	Provider generic.GenInfoModule

	// filter indicating where each original module is located over the stitched one
	SFilters []ifaces.Column

	// the active part of the stitching module
	IsActive ifaces.Column

	// max number of rows for the stitched module
	Size int
}

func NewGenericInfoAccumulator(comp *wizard.CompiledIOP, inp GenericAccumulatorInputs) *GenericInfoAccumulator {
	info := &GenericInfoAccumulator{
		Size:   utils.NextPowerOfTwo(inp.MaxNumKeccakF),
		Inputs: &inp,
	}

	// declare columns
	info.declareColumns(comp, len(inp.ProvidersInfo))

	// sFilter[i] starts immediately after sFilters[i-1].
	s := sym.NewConstant(0)
	for i := 0; i < len(info.SFilters); i++ {
		commonconstraints.MustBeActivationColumns(comp, info.SFilters[i], sym.Sub(1, s))
		s = sym.Add(s, info.SFilters[i])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("ADDs_UP_TO_IS_ACTIVE_Info"),
		sym.Sub(s, info.IsActive))

	// by the constraints over sFilter, and the following, we have that isActive is an Activation column.
	commonconstraints.MustBeBinary(comp, info.IsActive)

	// projection among providers and stitched module
	for i, gbm := range info.Inputs.ProvidersInfo {

		comp.InsertProjection(ifaces.QueryIDf("Stitch_Modules_Hi_%v", i),
			query.ProjectionInput{
				ColumnA: gbm.HashHi.ToBigEndianLimbs().GetLimbs(),
				ColumnB: info.Provider.HashHi.ToBigEndianLimbs().GetLimbs(),
				FilterA: gbm.IsHashHi,
				FilterB: info.SFilters[i],
			},
		)

		comp.InsertProjection(ifaces.QueryIDf("Stitch_Modules_Lo_%v", i),
			query.ProjectionInput{
				ColumnA: gbm.HashLo.ToBigEndianLimbs().GetLimbs(),
				ColumnB: info.Provider.HashLo.ToBigEndianLimbs().GetLimbs(),
				FilterA: gbm.IsHashLo,
				FilterB: info.SFilters[i],
			},
		)
	}
	return info
}

// declare columns
func (info *GenericInfoAccumulator) declareColumns(comp *wizard.CompiledIOP, nbProviders int) {
	var (
		nbChunks  = info.Inputs.ProvidersInfo[0].HashHi.NumLimbs()
		createCol = common.CreateColFn(comp, GENERIC_ACCUMULATOR, info.Size, pragmas.RightPadded)
	)

	// sanity check, all providers should have the same number of chunks
	for i := 1; i < nbProviders; i++ {
		if info.Inputs.ProvidersInfo[i].HashHi.NumLimbs() != nbChunks ||
			info.Inputs.ProvidersInfo[i].HashLo.NumLimbs() != nbChunks ||
			info.Inputs.ProvidersInfo[0].HashLo.NumLimbs() != nbChunks {
			utils.Panic("all providers should have the same number of chunks")
		}
	}

	info.IsActive = createCol("IsActive_Info")

	info.SFilters = make([]ifaces.Column, nbProviders)
	for i := 0; i < nbProviders; i++ {
		info.SFilters[i] = createCol("sFilterOut_%v", i)
	}

	info.Provider.HashHi = limbs.NewUint128Be(comp, GENERIC_ACCUMULATOR+"_sHash_Hi", info.Size, pragmas.RightPaddedPair)
	info.Provider.HashLo = limbs.NewUint128Be(comp, GENERIC_ACCUMULATOR+"_sHash_Lo", info.Size, pragmas.RightPaddedPair)

	info.Provider.IsHashHi = info.IsActive
	info.Provider.IsHashLo = info.IsActive
}

func (info *GenericInfoAccumulator) Run(run *wizard.ProverRuntime) {
	// fetch the witnesses of gbm

	providers := info.Inputs.ProvidersInfo
	asb := make([]infoAssignmentBuilder, len(providers))
	for i := range providers {
		asb[i].isHashHi = providers[i].IsHashHi.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].isHashLo = providers[i].IsHashLo.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].hashHi = providers[i].HashHi.GetAssignment(run)
		asb[i].hashLo = providers[i].HashLo.GetAssignment(run)
	}

	sFilters := make([][]field.Element, len(providers))
	for i := range providers {

		filter := asb[i].isHashHi
		// populate sFilters
		for j := range sFilters {
			for k := range filter {
				if filter[k].IsOne() {
					if j == i {
						sFilters[j] = append(sFilters[j], field.One())
					} else {
						sFilters[j] = append(sFilters[j], field.Zero())
					}
				}
			}

		}

	}

	//assign sFilters
	for i := range providers {
		run.AssignColumn(info.SFilters[i].GetColID(), smartvectors.RightZeroPadded(sFilters[i], info.Size))
	}

	// populate and assign isActive
	isActive := vector.Repeat(field.One(), len(sFilters[0]))
	run.AssignColumn(info.IsActive.GetColID(), smartvectors.RightZeroPadded(isActive, info.Size))

	// populate Provider
	var (
		sHashHi = make(limbs.VecRow[limbs.BigEndian], 0, providers[0].HashHi.NumRow())
		sHashLo = make(limbs.VecRow[limbs.BigEndian], 0, providers[0].HashLo.NumRow())
	)

	for i := range providers {
		filterHi := asb[i].isHashHi
		filterLo := asb[i].isHashLo
		hashHi := asb[i].hashHi
		hashLo := asb[i].hashLo
		for j := range filterHi {
			if filterHi[j].IsOne() {
				sHashHi = append(sHashHi, hashHi[j])
			}
			if filterLo[j].IsOne() {
				sHashLo = append(sHashLo, hashLo[j])
			}
		}
	}

	info.Provider.HashHi.AssignAndPadRows(run, sHashHi)
	info.Provider.HashLo.AssignAndPadRows(run, sHashLo)

}

type infoAssignmentBuilder struct {
	hashHi, hashLo     limbs.VecRow[limbs.BigEndian]
	isHashHi, isHashLo []field.Element
}
