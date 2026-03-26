package distributed

import (
	"fmt"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

var (
	// moduleWitnessKey is the key used to store the witness of a module
	// in the [wizard.ProverRuntime.State]
	moduleWitnessKey = "MODULE_WITNESS"
)

// ModuleSegmentationBlueprint is a blueprint for the segmentation of a
// module. It contains the informations of ModuleGL or ModuleLPP that are
// relevant to performing the module segmentation. The raison d'être of this
// structure is to avoid having to deal with the the complete [ModuleGL] or
// [ModuleLPP] structure which are massive compared to what is actually required
// to perform the segmentation.
type ModuleSegmentationBlueprint struct {
	// ModuleName indicates the name of the module
	ModuleName ModuleName
	// ModuleIndex is the integer index of the module
	ModuleIndex int
	// ReceivedValuesGlobalRoots stores the list of the root column for the
	// [ModuleGL.ReceivedValuesGlobalAccs] for each received value.
	ReceivedValuesGlobalAccsRoots []ifaces.ColID
	// ReceivedValuesGlobalPosition stores the list of the position of the
	// [ModuleGL.ReceivedValuesGlobalAccs] for each received value.
	ReceivedValuesGlobalAccsPositions []int
	// NextN0SelectorRoots stores the list of the selector columns ID for the
	// Horner queries.
	NextN0SelectorRoots [][]ifaces.ColID
	// NextN0SelectorIsConst stores a list of boolean indicating if the the
	// selector is constant.
	NextN0SelectorIsConsts [][]bool
	// NextN0SelectorConsts stores the list of the constants for the Horner
	// queries.
	NextN0SelectorConsts [][]field.Element
	// NextN0SelectorConstSize lists the size of the constants column that are
	// used for the Horner queries.
	NextN0SelectorConstSizes [][]int
	// LPPColumnSets stores the list of the columns that are used for the
	// LPP segments.
	LPPColumnSets []ifaces.ColID
}

// ModuleWitnessGL is a structure collecting the witness of a module. And
// stores all the informations that are necessary to build the witness.
type ModuleWitnessGL struct {
	// ModuleName indicates the name of the module
	ModuleName ModuleName
	// ModuleIndex indicates the ID of the module in the module list
	ModuleIndex int
	// SegmentModuleIndex indicates the vertical split of the current module
	SegmentModuleIndex int
	// SegmentCount counts the number of segments for each modules
	TotalSegmentCount []int
	// Columns maps the column id to their witness values
	Columns map[ifaces.ColID]smartvectors.SmartVector
	// ReceivedValuesGlobal stores the received values (for the global
	// constraints) of the current segment.
	ReceivedValuesGlobal []field.Element
	// VkMerkleRoot is the merkle root of a merkle tree storing the verification
	// key.
	VkMerkleRoot field.Element
}

// ModuleWitnessLPP is a structure collecting the witness of a module. The
// difference with a ModuleWitnessGL is that the witness is that the witness
// can be for a group of modules.
type ModuleWitnessLPP struct {
	// ModuleName indicates the name of the module
	ModuleName ModuleName
	// ModuleIndex indicates the vertical split of the current module
	ModuleIndex int
	// SegmentCount counts the total number of segments for each modules and is
	// used to populate the [targetNbSegment] public input.
	TotalSegmentCount []int
	// SegmentModuleIndex indicates the vertical split of the current module
	SegmentModuleIndex int
	// InitialFiatShamirState is the initial FiatShamir state to set at
	// round 1.
	InitialFiatShamirState field.Element
	// N0 values are the parameters to the Horner queries in the same order
	// as in the [FilteredModuleInputs.HornerArgs]
	N0Values []int
	// Columns maps the column id to their witness values
	Columns map[ifaces.ColID]smartvectors.SmartVector
	// VkMerkleRoot is the merkle root of a merkle tree storing the verification
	// key.
	VkMerkleRoot field.Element
}

// SegmentRuntime scans a [wizard.ProverRuntime] and returns a list of
// [ModuleWitness] that contains the witness for each segment of each
// module.
func SegmentRuntime(
	runtime *wizard.ProverRuntime,
	disc *StandardModuleDiscoverer,
	blueprintGLs, blueprintLPPs []ModuleSegmentationBlueprint,
	vkMerkleRoot field.Element,
) (
	witnessesGL []*ModuleWitnessGL,
	witnessesLPP []*ModuleWitnessLPP,
) {

	logger := logrus.WithField("type", "module-segmentation-stats")

	// Since the total segment count is (forcibly) the same for each module,
	// we need to compute it before generating all the witnesses.
	totalSegmentCount := make([]int, len(disc.Modules))

	for i := range disc.Modules {
		totalSegmentCount[i] = NbSegmentOfModule(runtime, disc, disc.Modules[i].ModuleName)
	}

	for i := range blueprintGLs {

		wGL := segmentModuleGL(runtime, disc, &blueprintGLs[i], totalSegmentCount, vkMerkleRoot)
		witnessesGL = append(witnessesGL, wGL...)

		// Expectedly, the blueprintGLs[i] is for the disc.Module[i] as the
		// blueprintGLs list is constructed from them in the same order.  We
		// sanity-check this assumption to ensure the line-up is correct.
		if blueprintGLs[i].ModuleName != disc.Modules[i].ModuleName {
			utils.Panic("blueprintGLs[i].ModuleNames[0] != disc.Modules[i].Name")
		}

		qbmStats := disc.Modules[i].RecordAssignmentStats(runtime)
		loggableQbmStats := map[string]map[string]any{}

		for _, stats := range qbmStats {
			loggableQbmStats[string(stats.ModuleName)] = map[string]any{
				"segment-size":     stats.SegmentSize,
				"nb-segment":       utils.DivCeil(stats.NbActiveRows, stats.SegmentSize),
				"total-number-row": stats.NbActiveRows,
			}
		}

		logger = logger.WithField(string(blueprintGLs[i].ModuleName), map[string]any{
			"qbm-stats":   loggableQbmStats,
			"nb-segments": len(wGL),
		})
	}

	for i := range blueprintLPPs {
		wLPP := segmentModuleLPP(runtime, disc, &blueprintLPPs[i], totalSegmentCount, vkMerkleRoot)
		witnessesLPP = append(witnessesLPP, wLPP...)
	}

	logger.Info("Performed module segmentation")

	return witnessesGL, witnessesLPP
}

// SegmentModule produces the list of the [ModuleWitness] for a given module
func segmentModuleGL(
	runtime *wizard.ProverRuntime,
	disc *StandardModuleDiscoverer,
	blueprintGL *ModuleSegmentationBlueprint,
	totalNbSegment []int,
	vkMerkleRoot field.Element,
) (witnessesGL []*ModuleWitnessGL) {

	var (
		moduleName           = blueprintGL.ModuleName
		cols                 = runtime.Spec.Columns.AllKeys()
		nbSegmentModule      = totalNbSegment[blueprintGL.ModuleIndex]
		receivedValuesGlobal = make([]field.Element, len(blueprintGL.ReceivedValuesGlobalAccsRoots))
	)

	witnessesGL = make([]*ModuleWitnessGL, nbSegmentModule)

	for moduleIndex := range witnessesGL {

		moduleWitnessGL := &ModuleWitnessGL{
			ModuleName:           moduleName,
			ModuleIndex:          blueprintGL.ModuleIndex,
			Columns:              make(map[ifaces.ColID]smartvectors.SmartVector),
			ReceivedValuesGlobal: receivedValuesGlobal,
			SegmentModuleIndex:   moduleIndex,
			TotalSegmentCount:    totalNbSegment,
			VkMerkleRoot:         vkMerkleRoot,
		}

		for _, col := range cols {

			col := runtime.Spec.Columns.GetHandle(col)

			if ModuleOfColumn(disc, col) != moduleName {
				continue
			}

			segment := SegmentOfColumn(runtime, disc, col, moduleIndex, nbSegmentModule)
			moduleWitnessGL.Columns[col.GetColID()] = segment
		}

		witnessesGL[moduleIndex] = moduleWitnessGL
		receivedValuesGlobal = moduleWitnessGL.NextReceivedValuesGlobal(blueprintGL)
	}

	return witnessesGL
}

// segmentModuleLPP produces the list of the [ModuleWitness] for a given module
func segmentModuleLPP(
	runtime *wizard.ProverRuntime,
	disc *StandardModuleDiscoverer,
	moduleLPP *ModuleSegmentationBlueprint,
	totalNbSegment []int,
	vkMerkleProof field.Element,
) (witnessesLPP []*ModuleWitnessLPP) {

	var (
		n0            = make([]int, len(moduleLPP.NextN0SelectorRoots))
		columnsLPPSet = make(map[ifaces.ColID]struct{})
	)

	for _, col := range moduleLPP.LPPColumnSets {
		columnsLPPSet[col] = struct{}{}
	}

	var (
		nbSegmentModule = NbSegmentOfModule(runtime, disc, moduleLPP.ModuleName)
		witnessesLPPs   = make([]*ModuleWitnessLPP, nbSegmentModule)
	)

	for segment := range witnessesLPPs {

		moduleWitnessLPP := &ModuleWitnessLPP{
			ModuleName:         moduleLPP.ModuleName,
			ModuleIndex:        moduleLPP.ModuleIndex,
			SegmentModuleIndex: segment,
			TotalSegmentCount:  totalNbSegment,
			Columns:            make(map[ifaces.ColID]smartvectors.SmartVector),
			N0Values:           n0,
			VkMerkleRoot:       vkMerkleProof,
		}

		witnessCols := make([]ifaces.ColID, 0)

		for _, col := range moduleLPP.LPPColumnSets {
			col := runtime.Spec.Columns.GetHandle(col)
			segment := SegmentOfColumn(runtime, disc, col, segment, nbSegmentModule)
			moduleWitnessLPP.Columns[col.GetColID()] = segment
			witnessCols = append(witnessCols, col.GetColID())
		}

		witnessesLPPs[segment] = moduleWitnessLPP
		n0 = moduleWitnessLPP.NextN0s(moduleLPP)
	}

	return witnessesLPPs
}

var (
	segmentWarningCache = &sync.Map{}
)

// NbSegmentOfModule returns the number of segments for a given module
func NbSegmentOfModule(runtime *wizard.ProverRuntime, disc *StandardModuleDiscoverer, moduleName ModuleName) (nbSegment int) {

	var (
		cols            = runtime.Spec.Columns.AllKeys()
		nbSegmentModule = -1
	)

	for _, col := range cols {

		var (
			col = runtime.Spec.Columns.GetHandle(col)
			mn  = disc.ModuleOf(col.(column.Natural))
		)

		if len(mn) == 0 {
			disc := disc
			utils.Panic("one column does not belong to any module: %v, disc: %v, mn: %v", col.GetColID(), disc.ColumnsToModule, mn)
		}

		if mn != moduleName {
			continue
		}

		var (
			newSize                  = NewSizeOfColumn(disc, col)
			start, stop, paddingInfo = disc.SegmentBoundaryOf(runtime, col.(column.Natural))
			nbSegmentForCol          = utils.DivExact(stop-start, newSize)
		)

		if nbSegmentForCol >= nbSegmentModule {

			if nbSegmentForCol >= 4 {
				col := col.(column.Natural)
				qbm, _ := disc.QbmOf(col)

				if _, ok := segmentWarningCache.Load(qbm.ModuleName); !ok {
					fmt.Printf("[large nb segment] module=%v qbm=%v column=%v nbSegment=%v paddingInfo=%v start=%v stop=%v newSize=%v originalSize=%v\n",
						mn, qbm.ModuleName, col.ID, nbSegmentForCol, paddingInfo, start, stop, newSize, col.Size(),
					)
					segmentWarningCache.Store(qbm.ModuleName, struct{}{})
				}
			}

			nbSegmentModule = nbSegmentForCol
		}
	}

	if nbSegmentModule == -1 {
		utils.Panic("could not resolve the number of segment for module %v", moduleName)
	}

	return nbSegmentModule
}

// SegmentColumn returns the segment of a given column for given index. The
// function also takes a maxNbSegment value which is useful in case
func SegmentOfColumn(runtime *wizard.ProverRuntime, disc *StandardModuleDiscoverer, col ifaces.Column, index, totalNbSegment int) smartvectors.SmartVector {

	if status := col.(column.Natural).Status(); status == column.Precomputed || status == column.VerifyingKey {
		return col.GetColAssignment(runtime)
	}

	var (
		newSize = NewSizeOfColumn(disc, col)
		// This returns the start and stop index of the segment for the current
		// standard module. But we might need to adjust for the LPP columns because
		// LPP modules group several standard modules together and they might have
		// different number of segments.
		startSeg, stopSeg, paddingInfo = disc.SegmentBoundaryOf(runtime, col.(column.Natural))

		// IsZeroPaddedIndicates whether the column is tagged as zero padded
		isZeroPadded = pragmas.IsZeroPadded(col)
	)

	if paddingInfo == leftPaddingInformation && (stopSeg-startSeg) < totalNbSegment*newSize {
		startSeg = stopSeg - totalNbSegment*newSize
	}

	var (
		start      = startSeg + index*newSize
		end        = start + newSize
		assignment = col.GetColAssignment(runtime)
	)

	// This switch case corresponds to a dirty-hack where the original column.
	// It is unexpected to have start < 0 and end > 0 due to the fact that the
	// columns and segment size are all power of two. And same observation for
	// stop > size and start < size.
	switch {

	// This is the regular case and there is no hack going on here.
	case start >= 0 && end <= col.Size():
		return assignment.SubVector(start, end)

	case start < 0 && end <= 0:
		// Otherwise, the padding technique is completely fine.
		// if startSeg == 0 {
		// 	logrus.Warnf("[ModuleWitnessOverflow] start and end are both negative, "+
		// 		"name=%v length=%v start=%v stop=%v sub-module-segment=[%v - %v]. "+
		// 		"Going to use the first value of the vector as a constant but this might fail.",
		// 		col.GetColID(), col.Size(), start, end, startSeg, stopSeg)
		// }

		if isZeroPadded {
			return smartvectors.NewConstant(field.Zero(), newSize)
		}

		// At this point, we are sure that the correct padding value is the
		// first value.
		padding := assignment.Get(0)
		return smartvectors.NewConstant(padding, newSize)

	case start >= col.Size() && end > col.Size():
		// Otherwise, the padding technique is completely fine.
		// if stopSeg == col.Size() {
		// 	logrus.Warnf("[ModuleWitnessOverflow] start and end are both greater than the length of the vector, "+
		// 		"name=%v length=%v start=%v stop=%v sub-module-segment=[%v - %v]. "+
		// 		"Going to use the last value of the vector as a constant but this might fail.",
		// 		col.GetColID(), col.Size(), start, end, startSeg, stopSeg)
		// }

		if isZeroPadded {
			return smartvectors.NewConstant(field.Zero(), newSize)
		}

		// At this point, we are sure that the correct padding value is the
		// last value (otherwise, we would have no way of guessing it).
		padding := assignment.Get(col.Size() - 1)
		return smartvectors.NewConstant(padding, newSize)

	case start == 0 && end > col.Size():

		if isZeroPadded {
			return smartvectors.RightPadded(
				assignment.IntoRegVecSaveAlloc(),
				field.Zero(),
				newSize,
			)
		}

		// logrus.Warnf("[ModuleWitnessOverflow] the segment is larger than the segment size. "+
		// 	"name=%v length=%v start=%v stop=%v sub-module-segment=[%v - %v]. "+
		// 	"Going to extend the column on the right by repeating the last value but this might fail. You may want to increase the bootstrapper size for this column so that it is always larger than the new size",
		// 	col.GetColID(), col.Size(), start, end, startSeg, stopSeg)

		return smartvectors.RightPadded(
			assignment.IntoRegVecSaveAlloc(),
			assignment.Get(col.Size()-1),
			newSize,
		)

	case start < 0 && end == col.Size():

		if isZeroPadded {
			return smartvectors.LeftPadded(
				assignment.IntoRegVecSaveAlloc(),
				field.Zero(),
				newSize,
			)
		}

		// logrus.Warnf("[ModuleWitnessOverflow] the segment is larger than the segment size. "+
		// 	"name=%v length=%v start=%v stop=%v sub-module-segment=[%v - %v]. "+
		// 	"Going to extend the column on the left by repeating the first value but this might fail. You may want to increase the bootstrapper size for this column so that it is always larger than the new size",
		// 	col.GetColID(), col.Size(), start, end, startSeg, stopSeg)

		return smartvectors.LeftPadded(
			assignment.IntoRegVecSaveAlloc(),
			assignment.Get(0),
			newSize,
		)

	default:
		utils.Panic(
			"unexpected case, col=%v, start=%v, end=%v, size=%v, startSeg=%v, stopSeg=%v",
			col, start, end, col.Size(), startSeg, stopSeg,
		)
		return nil
	}
}

// Blueprint returns the blueprint for the current module.
func (moduleGL *ModuleGL) Blueprint() ModuleSegmentationBlueprint {

	blueprintGL := ModuleSegmentationBlueprint{
		ModuleIndex:                       moduleGL.Disc.IndexOf(moduleGL.DefinitionInput.ModuleName),
		ModuleName:                        moduleGL.DefinitionInput.ModuleName,
		ReceivedValuesGlobalAccsRoots:     make([]ifaces.ColID, len(moduleGL.SentValuesGlobal)),
		ReceivedValuesGlobalAccsPositions: make([]int, len(moduleGL.SentValuesGlobal)),
	}

	for i, loc := range moduleGL.SentValuesGlobal {

		var (
			col     = column.RootParents(loc.Pol)
			pos     = column.StackOffsets(loc.Pol)
			colName = col.GetColID()
		)

		blueprintGL.ReceivedValuesGlobalAccsRoots[i] = colName
		blueprintGL.ReceivedValuesGlobalAccsPositions[i] = pos
	}

	return blueprintGL
}

// Blueprint returns the blueprint for the current module.
func (moduleLPP *ModuleLPP) Blueprint() ModuleSegmentationBlueprint {

	var (
		hornerParts   = []query.HornerPart{}
		numHornerPart = 0
	)

	if moduleLPP.Horner != nil {
		hornerParts = moduleLPP.Horner.Parts
		numHornerPart = len(moduleLPP.Horner.Parts)
	}

	res := ModuleSegmentationBlueprint{
		ModuleName:               moduleLPP.ModuleName(),
		ModuleIndex:              moduleLPP.Disc.IndexOf(moduleLPP.DefinitionInput.ModuleName),
		NextN0SelectorRoots:      make([][]ifaces.ColID, numHornerPart),
		NextN0SelectorIsConsts:   make([][]bool, numHornerPart),
		NextN0SelectorConsts:     make([][]field.Element, numHornerPart),
		NextN0SelectorConstSizes: make([][]int, numHornerPart),
		LPPColumnSets:            []ifaces.ColID{},
	}

	res.LPPColumnSets = make([]ifaces.ColID, len(moduleLPP.DefinitionInput.ColumnsLPP))
	for j := range moduleLPP.DefinitionInput.ColumnsLPP {
		res.LPPColumnSets[j] = moduleLPP.DefinitionInput.ColumnsLPP[j].GetColID()
	}

	for i := range hornerParts {

		partArity := len(hornerParts[i].Selectors)
		res.NextN0SelectorConstSizes[i] = make([]int, partArity)
		res.NextN0SelectorRoots[i] = make([]ifaces.ColID, partArity)
		res.NextN0SelectorIsConsts[i] = make([]bool, partArity)
		res.NextN0SelectorConsts[i] = make([]field.Element, partArity)

		for k := range hornerParts[i].Selectors {

			// Note: the selector might be a non-natural column. Possibly a const-col.
			selCol := hornerParts[i].Selectors[k]
			res.NextN0SelectorRoots[i][k] = selCol.GetColID()

			if constCol, isConstCol := selCol.(verifiercol.ConstCol); isConstCol {

				if !constCol.F.IsZero() || constCol.F.IsOne() {
					utils.Panic("the selector column has non-binary values: %v", constCol.F.String())
				}

				res.NextN0SelectorConsts[i][k] = constCol.F
				res.NextN0SelectorIsConsts[i][k] = true
				res.NextN0SelectorConstSizes[i][k] = constCol.Size()

				continue
			}

			// Expectedly, at this point. The column must be a natural column. We can't support
			// shifted selector columns.
			_ = selCol.(column.Natural)
		}
	}

	return res
}

// NextN0s returns the next value of N0, from the current one and the witness
// of the current module.
func (mw *ModuleWitnessLPP) NextN0s(blueprintLPP *ModuleSegmentationBlueprint) []int {

	// This is a deep-copy of the current N0s, so that we can ensure that we do
	// not modify the receiver witness when computing the updated value.
	newN0s := append([]int{}, mw.N0Values...)

	for i := range blueprintLPP.NextN0SelectorRoots {
		for k := range blueprintLPP.NextN0SelectorRoots[i] {

			var (
				selColID        = blueprintLPP.NextN0SelectorRoots[i][k]
				selColIsConst   = blueprintLPP.NextN0SelectorIsConsts[i][k]
				selColConst     = blueprintLPP.NextN0SelectorConsts[i][k]
				selColConstSize = blueprintLPP.NextN0SelectorConstSizes[i][k]
			)

			if selColIsConst {

				if selColConst.IsZero() {
					continue
				}

				if selColConst.IsOne() {
					newN0s[i] += selColConstSize
					continue
				}

				utils.Panic("the selector column has non-zero values: %v", selColConst.String())
			}

			selSV, ok := mw.Columns[selColID]
			if !ok {
				utils.Panic("selector: %v is missing from witness columns for module: %v index: %v, segment-index: %v", selColID, mw.ModuleName, mw.ModuleIndex, mw.SegmentModuleIndex)
			}

			sel := selSV.IntoRegVecSaveAlloc()

			for j := range sel {
				if sel[j].IsOne() {
					newN0s[i]++
				}
			}
		}
	}

	return newN0s
}

// NextReceivedValuesGlobal returns the next value of ReceivedValuesGlobal, from
// the witness of the current module.
func (mw *ModuleWitnessGL) NextReceivedValuesGlobal(blueprintGL *ModuleSegmentationBlueprint) []field.Element {

	newReceivedValuesGlobal := make([]field.Element, len(mw.ReceivedValuesGlobal))

	for i := range blueprintGL.ReceivedValuesGlobalAccsRoots {

		var (
			rootName        = blueprintGL.ReceivedValuesGlobalAccsRoots[i]
			loc             = blueprintGL.ReceivedValuesGlobalAccsPositions[i]
			smartvec, found = mw.Columns[rootName]
		)

		if !found {
			utils.Panic("could not find smartvector: %v in the columns of the module", rootName)
		}

		loc = utils.PositiveMod(loc, smartvec.Len())
		newReceivedValuesGlobal[i] = smartvec.Get(loc)
	}

	return newReceivedValuesGlobal
}
