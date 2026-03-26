// The accumulator package is responsible for accumulating the data from different arithmetization module.
package gen_acc

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
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

const (
	blockSize           = 136 // number of bytes in the block
	GENERIC_ACCUMULATOR = "GENERIC_ACCUMULATOR"
)

type GenericAccumulatorInputs struct {
	MaxNumKeccakF int
	ProvidersData []generic.GenDataModule
	ProvidersInfo []generic.GenInfoModule
}

// The sub-module GenericDataAccumulator filters the data from different [generic.GenDataModule],
//
//	and stitch them together to build a single module.
type GenericDataAccumulator struct {
	Inputs *GenericAccumulatorInputs
	// stitching of modules together
	Provider generic.GenDataModule

	// filter indicating where each original module is located over the stitched one
	SFilters []ifaces.Column

	// the active part of the stitching module
	IsActive ifaces.Column

	// max number of rows for the stitched module
	Size int
}

// It declares the new columns and the constraints among them
func NewGenericDataAccumulator(comp *wizard.CompiledIOP, inp GenericAccumulatorInputs) *GenericDataAccumulator {

	d := &GenericDataAccumulator{
		Size:   utils.NextPowerOfTwo(inp.MaxNumKeccakF * blockSize),
		Inputs: &inp,
	}
	// declare the new columns per gbm
	d.declareColumns(comp, len(inp.ProvidersData))

	// sFilter[i] starts immediately after sFilters[i-1].
	s := sym.NewConstant(0)
	for i := 0; i < len(d.SFilters); i++ {
		commonconstraints.MustBeActivationColumns(comp, d.SFilters[i], sym.Sub(1, s))
		s = sym.Add(s, d.SFilters[i])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("ADDs_UP_TO_IS_ACTIVE_DATA"),
		sym.Sub(s, d.IsActive))

	// by the constraints over sFilter, and the following, we have that isActive is an Activation column.
	commonconstraints.MustBeBinary(comp, d.IsActive)

	// projection among providers and stitched module
	for i, gbm := range d.Inputs.ProvidersData {

		comp.InsertProjection(ifaces.QueryIDf("Stitch_Modules_%v", i),
			query.ProjectionInput{
				ColumnA: append(
					gbm.Limbs.ToBigEndianLimbs().GetLimbs(),
					gbm.HashNum,
					gbm.NBytes,
					gbm.Index,
				),
				ColumnB: append(
					d.Provider.Limbs.ToBigEndianLimbs().GetLimbs(),
					d.Provider.HashNum,
					d.Provider.NBytes,
					d.Provider.Index,
				),
				FilterA: gbm.ToHash,
				FilterB: d.SFilters[i],
			},
		)
	}

	return d
}

// It declares the columns specific to the DataModule
func (d *GenericDataAccumulator) declareColumns(comp *wizard.CompiledIOP, nbProviders int) {
	var (
		createCol = common.CreateColFn(comp, GENERIC_ACCUMULATOR, d.Size, pragmas.RightPadded)
		numLimbs  = d.Inputs.ProvidersData[0].Limbs.NumLimbs()
	)

	// sanity check; all providers must have the same number of chunks
	for i := 1; i < len(d.Inputs.ProvidersData); i++ {
		if d.Inputs.ProvidersData[i].Limbs.NumLimbs() != numLimbs {
			panic("all providers must have the same number of chunks")
		}
	}

	d.SFilters = make([]ifaces.Column, nbProviders)
	for i := 0; i < nbProviders; i++ {
		d.SFilters[i] = createCol("sFilter_%v", i)
	}

	d.IsActive = createCol("IsActive")
	d.Provider = generic.GenDataModule{
		HashNum: createCol("sHashNum"),
		Limbs:   limbs.NewUint128Be(comp, GENERIC_ACCUMULATOR+"_sLimb", d.Size, pragmas.RightPaddedPair),
		NBytes:  createCol("sNBytes"),
		Index:   createCol("sIndex"),
		ToHash:  d.IsActive,
	}
}

// It assigns the columns specific to the submodule.
func (d *GenericDataAccumulator) Run(run *wizard.ProverRuntime) {
	// fetch the gbm witnesses
	providers := d.Inputs.ProvidersData
	asb := make([]assignmentBuilder, len(providers))
	for i := range providers {
		asb[i].HashNum = expr_handle.GetExprHandleAssignment(run, providers[i].HashNum).IntoRegVecSaveAlloc()
		asb[i].Limbs = providers[i].Limbs.GetAssignment(run)
		asb[i].NBytes = providers[i].NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].Index = expr_handle.GetExprHandleAssignment(run, providers[i].Index).IntoRegVecSaveAlloc()
		asb[i].TO_HASH = providers[i].ToHash.GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	sFilters := make([][]field.Element, len(providers))
	for i := range providers {
		filter := asb[i].TO_HASH

		// populate sFilters
		for j := range sFilters {
			for k := range filter {
				if filter[k] == field.One() {
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
		run.AssignColumn(d.SFilters[i].GetColID(), smartvectors.RightZeroPadded(sFilters[i], d.Size))
	}

	// populate and assign isActive
	isActive := vector.Repeat(field.One(), len(sFilters[0]))
	run.AssignColumn(d.IsActive.GetColID(), smartvectors.RightZeroPadded(isActive, d.Size))

	// populate Provider
	var (
		sHashNum, sNBytes, sIndex []field.Element
		sLimb                     = make([][]field.Element, providers[0].Limbs.NumLimbs())
	)
	for i := range providers {
		filter := asb[i].TO_HASH
		hashNum := asb[i].HashNum
		limb := asb[i].Limbs
		nBytes := asb[i].NBytes
		index := asb[i].Index
		for j := range filter {
			if filter[j].IsOne() {
				sHashNum = append(sHashNum, hashNum[j])
				sNBytes = append(sNBytes, nBytes[j])
				sIndex = append(sIndex, index[j])
				for k := range sLimb {
					sLimb[k] = append(sLimb[k], limb[j].ToRawUnsafe()[k])
				}

			}
		}
	}

	run.AssignColumn(d.Provider.HashNum.GetColID(), smartvectors.RightZeroPadded(sHashNum, d.Size))
	run.AssignColumn(d.Provider.NBytes.GetColID(), smartvectors.RightZeroPadded(sNBytes, d.Size))
	run.AssignColumn(d.Provider.Index.GetColID(), smartvectors.RightZeroPadded(sIndex, d.Size))
	common.AssignMultiColumn(run, d.Provider.Limbs.GetLimbs(), sLimb, d.Size)

}

// GenDataModule collects the columns summarizing the informations about the
// data to hash.
type assignmentBuilder struct {
	HashNum, Index  []field.Element
	NBytes, TO_HASH []field.Element
	Limbs           limbs.VecRow[limbs.BigEndian]
}
