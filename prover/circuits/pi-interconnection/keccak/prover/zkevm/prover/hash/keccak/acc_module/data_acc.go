// The accumulator package is responsible for accumulating the data from different arithmetization module.
package gen_acc

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
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
			query.ProjectionInput{ColumnA: []ifaces.Column{gbm.HashNum, gbm.Limb, gbm.NBytes, gbm.Index},
				ColumnB: []ifaces.Column{d.Provider.HashNum, d.Provider.Limb, d.Provider.NBytes, d.Provider.Index},
				FilterA: gbm.ToHash,
				FilterB: d.SFilters[i]})

	}

	return d
}

// It declares the columns specific to the DataModule
func (d *GenericDataAccumulator) declareColumns(comp *wizard.CompiledIOP, nbProviders int) {
	createCol := common.CreateColFn(comp, GENERIC_ACCUMULATOR, d.Size, pragmas.RightPadded)

	d.SFilters = make([]ifaces.Column, nbProviders)
	for i := 0; i < nbProviders; i++ {
		d.SFilters[i] = createCol("sFilter_%v", i)
	}

	d.IsActive = createCol("IsActive")
	d.Provider.HashNum = createCol("sHashNum")
	d.Provider.Limb = createCol("sLimb")
	d.Provider.NBytes = createCol("sNBytes")
	d.Provider.Index = createCol("sIndex")
	d.Provider.ToHash = d.IsActive
}

// It assigns the columns specific to the submodule.
func (d *GenericDataAccumulator) Run(run *wizard.ProverRuntime) {
	// fetch the gbm witnesses
	providers := d.Inputs.ProvidersData
	asb := make([]assignmentBuilder, len(providers))
	for i := range providers {
		asb[i].HashNum = providers[i].HashNum.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].Limb = providers[i].Limb.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].NBytes = providers[i].NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		asb[i].Index = providers[i].Index.GetColAssignment(run).IntoRegVecSaveAlloc()
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
	var sHashNum, sLimb, sNBytes, sIndex []field.Element
	for i := range providers {
		filter := asb[i].TO_HASH
		hashNum := asb[i].HashNum
		limb := asb[i].Limb
		nBytes := asb[i].NBytes
		index := asb[i].Index
		for j := range filter {
			if filter[j] == field.One() {
				sHashNum = append(sHashNum, hashNum[j])
				sLimb = append(sLimb, limb[j])
				sNBytes = append(sNBytes, nBytes[j])
				sIndex = append(sIndex, index[j])

			}
		}
	}

	run.AssignColumn(d.Provider.HashNum.GetColID(), smartvectors.RightZeroPadded(sHashNum, d.Size))
	run.AssignColumn(d.Provider.Limb.GetColID(), smartvectors.RightZeroPadded(sLimb, d.Size))
	run.AssignColumn(d.Provider.NBytes.GetColID(), smartvectors.RightZeroPadded(sNBytes, d.Size))
	run.AssignColumn(d.Provider.Index.GetColID(), smartvectors.RightZeroPadded(sIndex, d.Size))

}

// GenDataModule collects the columns summarizing the informations about the
// data to hash.
type assignmentBuilder struct {
	HashNum, Index, Limb []field.Element
	NBytes, TO_HASH      []field.Element
}
