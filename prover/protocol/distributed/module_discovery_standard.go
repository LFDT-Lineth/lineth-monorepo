package distributed

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"unsafe"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/dlclark/regexp2"
	"github.com/sirupsen/logrus"
)

// StandardModuleDiscoverer groups modules using two layers. In the first layer,
// the [QueryBasedModuleDiscoverer] groups columns that are part of the same
// queries (for global, local, plonk, log-derivative, grand-product and horner
// queries).
type StandardModuleDiscoverer struct {
	// TargetWeight is the target weight for each module.
	TargetWeight int
	// Affinities indicates groups of columns (potentially spanning over
	// multiple query-based modules) that are "alike" in the sense that
	// they would be opportunistic to group in the same StandardModule.
	Affinities [][]column.Natural
	// Predivision indicates that all the inputs column size should be
	// divided by some values before being added in a [QueryBasedModule].
	Predivision int
	// Advices is an optional list of advices for the [QueryBasedModuleDiscoverer].
	// When used, the discoverer expects that every query-based module is provided
	// with an advice otherwise, it will panic.
	Advices         []*ModuleDiscoveryAdvice
	Modules         []*StandardModule
	ColumnsToModule map[ifaces.ColID]ModuleName
	ColumnsToSize   map[ifaces.ColID]int
}

// QueryBasedModuleDiscoverer tracks modules using DisjointSet.
type QueryBasedModuleDiscoverer struct {
	Mutex           *sync.Mutex
	Modules         []*QueryBasedModule
	ModuleNames     []ModuleName
	ColumnsToModule map[ifaces.ColID]ModuleName
	Predivision     int
}

// StandardModule is a structure coalescing a set of [QueryBasedModule]s.
type StandardModule struct {
	ModuleName ModuleName
	SubModules []*QueryBasedModule
	NewSizes   []int
}

// QueryBasedModule represents a set of columns grouped by constraints.
type QueryBasedModule struct {
	ModuleName   ModuleName
	Ds           *utils.DisjointSet[ifaces.ColID] // Uses a disjoint set to track relationships among columns.
	OriginalSize int
	// NbConstraintsOfPlonkCirc counts the number of constraints in a Plonk
	// in wizard module if one is found. If several circuits are stores
	// the number is the sum for all the circuits where the nb constraint
	// for circuit is padded to the next power of two. For instance, if
	// we have a circuit with 5 constraints, and one with 33 constraints
	// the value of nbConstraintsPlonk will be 64 + 8 = 72.
	NbConstraintsOfPlonkCirc int
	// NbInstancesOfPlonkCirc indicates the max number of instances in a
	// Plonk in wizard module if one is found.
	NbInstancesOfPlonkCirc int
	// NbInstancesOfPlonkQuery indicates the number of Plonk in wizard query in the
	// present module.
	NbInstancesOfPlonkQuery int
	// NbSegmentCache caches the results of SegmentBoundaries
	NbSegmentCache      map[unsafe.Pointer][3]int
	NbSegmentCacheMutex *sync.Mutex
	Predivision         int
	HasPrecomputed      bool
}

// QueryBasedAssignmentStatsRecord is a record of one assignment of one query
// based module it features information about the padding, the number of
// assigned rows and the number of columns and the name of the first and the
// last column in alphabetical order.
type QueryBasedAssignmentStatsRecord struct {
	Request                  string
	ModuleName               ModuleName
	ClusterName              ModuleName
	SegmentSize              int
	OriginalSize             int
	NbConstraintsOfPlonkCirc int
	NbInstancesOfPlonkCirc   int
	NbInstancesOfPlonkQuery  int
	NbPrecomputed            int
	NbColumns                int
	NbPragmaLeftPadded       int
	NbPragmaRightPadded      int
	NbPragmaFullColumn       int
	NbAssignedLeftPadded     int
	NbAssignedRightPadded    int
	NbAssignedFullColumn     int
	NbAssignedConstantColumn int
	NbActiveRows             int
	FirstColumnAlphabetical  ifaces.ColID
	LastColumnAlphabetical   ifaces.ColID
	LastLeftPadded           ifaces.ColID
	LastRightPadded          ifaces.ColID
	err                      error
}

// NewQueryBasedDiscoverer initializes a new Discoverer.
func NewQueryBasedDiscoverer() *QueryBasedModuleDiscoverer {
	return &QueryBasedModuleDiscoverer{
		Mutex:           &sync.Mutex{},
		Modules:         []*QueryBasedModule{},
		ModuleNames:     []ModuleName{},
		ColumnsToModule: make(map[ifaces.ColID]ModuleName),
	}
}

func (disc *StandardModuleDiscoverer) Analyze(comp *wizard.CompiledIOP) {
	if len(disc.Advices) == 0 {
		disc.analyzeBasic(comp)
		return
	}
	disc.analyzeWithAdvices(comp)
}

func (disc *StandardModuleDiscoverer) analyzeWithAdvices(comp *wizard.CompiledIOP) {

	subDiscover := NewQueryBasedDiscoverer()
	subDiscover.Predivision = 1
	subDiscover.Analyze(comp)

	var (
		// moduleSets is the result of the current function. It will ultimately
		// contains all the
		moduleSets = map[ModuleName]*StandardModule{}
		// adviceOfColumn lists
		adviceOfColumn    = make(map[ifaces.ColID]*ModuleDiscoveryAdvice)
		adviceMappingErrs = []error{}
	)

	for _, colName := range comp.Columns.AllKeys() {
		col := comp.Columns.GetHandle(colName)
		for _, adv := range disc.Advices {

			if !adv.DoesMatch(col) {
				continue
			}

			adviceOfColumn[col.GetColID()] = adv
			if moduleSets[adv.Cluster] == nil {
				moduleSets[adv.Cluster] = &StandardModule{
					ModuleName: adv.Cluster,
				}
			}

			break
		}
	}

	// This section attempts to map each query-based module to its advice and
	// adds it to the module set based on the advice indication. For each QBM
	// we found earlier, we check if a one of the columns in the QBM is mapped
	// to an advice and we assign to the QBM. The section also checks the
	// followings properties:
	// 	- the QBM may not be provided conflicting advices.
	// 	- At least one advice is found.

	for _, qbm := range subDiscover.Modules {

		var (
			adviceFound        *ModuleDiscoveryAdvice
			conflictingAdvices []*ModuleDiscoveryAdvice
		)

		for col := range qbm.Ds.Iter() {
			if adv, found := adviceOfColumn[col]; found {

				if adviceFound == nil {
					adviceFound = adv
				}

				// At this stage, we are guaranteed that adviceFound is not nil
				// , that's why we can directly do the comparison.
				if adviceFound.AreConflicting(adv) {
					conflictingAdvices = append(conflictingAdvices, adv)
				}
			}
		}

		if adviceFound == nil {
			adviceMappingErrs = append(adviceMappingErrs, fmt.Errorf("could not find advice for QBM: %v", qbm.Ds.Rank))
			continue
		}

		if len(conflictingAdvices) > 0 {
			adviceMappingErrs = append(adviceMappingErrs, fmt.Errorf("conflicting advice for QBM: %v, first advice: %++v, conflicting advices: %++v", qbm.Ds.Rank, adviceFound, conflictingAdvices))
			continue
		}

		newModule := moduleSets[adviceFound.Cluster]
		newModule.SubModules = append(newModule.SubModules, qbm)
		newModule.NewSizes = append(newModule.NewSizes, adviceFound.BaseSize)
	}

	if len(adviceMappingErrs) > 0 {
		for _, e := range adviceMappingErrs {
			logrus.Error(e)
		}
		panic("Got errors while mapping advices to QBMs. See logs above.")
	}

	// This adds the module sets to the discovery in deterministic order
	moduleNameList := utils.SortedKeysOf(moduleSets, func(a, b ModuleName) bool { return a < b })
	for _, moduleName := range moduleNameList {
		disc.Modules = append(disc.Modules, moduleSets[moduleName])
	}

	// This resizes the modules so that their weights are close to the target
	// weight.
	for i := range disc.Modules {

		// Store the original BaseSize from advice for each submodule as the minimum allowed size.
		// For submodules with Plonk circuits, BaseSize represents the required number of public
		// inputs and must not be reduced.
		baseSizes := make([]int, len(disc.Modules[i].NewSizes))
		copy(baseSizes, disc.Modules[i].NewSizes)

		// weightTotalInitial is the weight of the module using the initial
		// number of rows.
		weightTotalInitial := 0
		// maxNumRows is the number of rows of the "shallowest" submodule of in
		// group[i]
		minNumRows := math.MaxInt

		// initializes the newSizes using the number of rows from the initial
		// comp.
		for j := range disc.Modules[i].NewSizes {
			numRows := disc.Modules[i].NewSizes[j]
			minNumRows = min(minNumRows, numRows)
			weightTotalInitial += disc.Modules[i].SubModules[j].Weight(comp, numRows)
		}

		// Computes optimal newSizes by successively dividing by two the numbers
		// of rows and checks if this brings the total weight of the module to
		// the target weight.
		var (
			bestReduction = 1
			bestWeight    = weightTotalInitial
		)

		if weightTotalInitial < disc.TargetWeight {
			continue
		}

		for reduction := 2; reduction < minNumRows; reduction *= 2 {

			currWeight := 0
			for j := range disc.Modules[i].SubModules {
				numRow := disc.Modules[i].NewSizes[j]
				if !disc.Modules[i].SubModules[j].HasPrecomputed {
					numRow /= reduction
					// Only enforce BaseSize floor for submodules with Plonk circuits, as they have
					// strict requirements for the number of public inputs matching the circuit size
					if disc.Modules[i].SubModules[j].NbConstraintsOfPlonkCirc > 0 && numRow < baseSizes[j] {
						numRow = baseSizes[j]
					}
				}

				if numRow < 1 {
					panic("the 'reduction' is bounded by the min number of rows so it should not be smaller than 1")
				}
				currWeight += disc.Modules[i].SubModules[j].Weight(comp, numRow)
			}

			currDist := utils.Abs(currWeight - disc.TargetWeight)
			bestDist := utils.Abs(bestWeight - disc.TargetWeight)

			if currDist < bestDist {
				bestReduction = reduction
				bestWeight = currWeight
			}
		}

		for j := range disc.Modules[i].SubModules {
			if !disc.Modules[i].SubModules[j].HasPrecomputed {
				disc.Modules[i].NewSizes[j] /= bestReduction
				// Only enforce BaseSize floor for submodules with Plonk circuits
				if disc.Modules[i].SubModules[j].NbConstraintsOfPlonkCirc > 0 && disc.Modules[i].NewSizes[j] < baseSizes[j] {
					disc.Modules[i].NewSizes[j] = baseSizes[j]
				}
			}
		}
	}

	disc.ColumnsToModule = make(map[ifaces.ColID]ModuleName)
	disc.ColumnsToSize = make(map[ifaces.ColID]int)

	for i := range disc.Modules {
		numColModule := 0
		moduleName := disc.Modules[i].ModuleName
		for j := range disc.Modules[i].SubModules {
			subModule := disc.Modules[i].SubModules[j]
			newSize := disc.Modules[i].NewSizes[j]
			numCol := 0
			for colID := range subModule.Ds.Iter() {
				disc.ColumnsToModule[colID] = moduleName
				disc.ColumnsToSize[colID] = newSize
				numCol++
			}
			logrus.Infof("Number of columns: %v, SubModule name: %v, ModuleName: %v", numCol, subModule.ModuleName, moduleName)
			numColModule += numCol
		}
		logrus.Infof("Total number of columns: %v, ModuleName: %v", numColModule, moduleName)
	}
}

// analyzeBasic processes scans the comp and generate the modules. It works by
// grouping columns that are part of the same query using the [QueryBasedModuleDiscoverer].
// After that it sorts the generated modules by weight and
func (disc *StandardModuleDiscoverer) analyzeBasic(comp *wizard.CompiledIOP) {

	subDiscover := NewQueryBasedDiscoverer()
	subDiscover.Predivision = disc.Predivision
	subDiscover.Analyze(comp)

	groupedByAffinity := groupQBModulesByAffinity(subDiscover.Modules, disc.Affinities)

	sort.Slice(groupedByAffinity, func(i, j int) bool {
		return weightOfGroupOfQBModules(comp, groupedByAffinity[i]) < weightOfGroupOfQBModules(comp, groupedByAffinity[j])
	})

	var (
		groups        = [][]*QueryBasedModule{{}}
		currWeightSum = 0
	)

	for i := range groupedByAffinity {

		var (
			groupNext  = groupedByAffinity[i]
			weightNext = weightOfGroupOfQBModules(comp, groupNext)
			distPrev   = utils.Abs(disc.TargetWeight - currWeightSum)
			distNext   = utils.Abs(disc.TargetWeight - currWeightSum - weightNext)
		)

		if weightNext == 0 {
			continue
		}

		if distNext > distPrev && currWeightSum > 0 {
			groups = append(groups, []*QueryBasedModule{})
			currWeightSum = 0
		}

		currWeightSum += weightNext
		groups[len(groups)-1] = append(groups[len(groups)-1], groupNext...)
	}

	disc.Modules = make([]*StandardModule, len(groups))

	for i := range disc.Modules {

		disc.Modules[i] = &StandardModule{
			ModuleName: ModuleName(fmt.Sprintf("Module_%d", i)),
			SubModules: groups[i],
			NewSizes:   make([]int, len(groups[i])),
		}

		// weightTotalInitial is the weight of the module using the initial
		// number of rows.
		weightTotalInitial := 0
		// maxNumRows is the number of rows of the "shallowest" submodule of in
		// group[i]
		minNumRows := math.MaxInt

		// initializes the newSizes using the number of rows from the initial
		// comp.
		for j := range groups[i] {
			numRows := groups[i][j].NumRow(comp)
			disc.Modules[i].NewSizes[j] = numRows
			minNumRows = min(minNumRows, numRows)
			weightTotalInitial += groups[i][j].Weight(comp, 0)
		}

		// Computes optimal newSizes by successively dividing by two the numbers
		// of rows and checks if this brings the total weight of the module to
		// the target weight.
		var (
			bestReduction = 1
			bestWeight    = weightTotalInitial
		)

		for reduction := 2; reduction < minNumRows; reduction *= 2 {

			currWeight := 0
			for j := range groups[i] {
				numRow := groups[i][j].NumRow(comp)
				if !groups[i][j].HasPrecomputed {
					numRow /= reduction
				}

				if numRow < 1 {
					panic("the 'reduction' is bounded by the min number of rows so it should not be smaller than 1")
				}
				currWeight += groups[i][j].Weight(comp, numRow)
			}

			if currWeight == 0 {
				utils.Panic("currWeight == 0, for module %v, index %d", disc.Modules[i].ModuleName, i)
			}

			currDist := utils.Abs(currWeight - disc.TargetWeight)
			bestDist := utils.Abs(bestWeight - disc.TargetWeight)

			if currDist < bestDist {
				bestReduction = reduction
				bestWeight = currWeight
			}
		}

		for j := range groups[i] {
			disc.Modules[i].NewSizes[j] /= bestReduction
		}
	}

	disc.ColumnsToModule = make(map[ifaces.ColID]ModuleName)
	disc.ColumnsToSize = make(map[ifaces.ColID]int)

	for i := range disc.Modules {

		moduleName := disc.Modules[i].ModuleName

		for j := range disc.Modules[i].SubModules {

			subModule := disc.Modules[i].SubModules[j]
			newSize := disc.Modules[i].NewSizes[j]

			for colID := range subModule.Ds.Iter() {
				disc.ColumnsToModule[colID] = moduleName
				disc.ColumnsToSize[colID] = newSize
			}
		}
	}
}

// ModuleList returns a list of all module names.
func (disc *StandardModuleDiscoverer) ModuleList() []ModuleName {
	modulesNames := make([]ModuleName, len(disc.Modules))
	for i := range disc.Modules {
		modulesNames[i] = disc.Modules[i].ModuleName
	}
	return modulesNames
}

// NumColumnOf counts the number of columns found for the current module
func (disc *StandardModuleDiscoverer) NumColumnOf(moduleName ModuleName) int {

	for i := range disc.Modules {

		if disc.Modules[i].ModuleName != moduleName {
			continue
		}

		res := 0
		for j := range disc.Modules[i].SubModules {
			res += disc.Modules[i].SubModules[j].NumColumn()
		}
		return res
	}

	utils.Panic("module not found")
	return 0
}

// ModuleOf returns the module name for a given column“
func (disc *StandardModuleDiscoverer) ModuleOf(col column.Natural) ModuleName {
	res, found := disc.ColumnsToModule[col.GetColID()]
	if !found {
		utils.Panic("column not found, col: %q, disc: %v", col.GetColID(), disc.ColumnsToModule)
	}
	return res
}

// NewSizeOf returns the size (length) of a column.
func (disc *StandardModuleDiscoverer) NewSizeOf(col column.Natural) int {
	return disc.ColumnsToSize[col.GetColID()]
}

// QbmOf returns the query-based module associated with a column and the associated new size.
func (disc *StandardModuleDiscoverer) QbmOf(col column.Natural) (*QueryBasedModule, int) {

	var (
		rootCol          = column.RootParents(col).(column.Natural)
		stdModuleName    = disc.ColumnsToModule[col.GetColID()]
		stdModule        *StandardModule
		queryBasedModule *QueryBasedModule
		segmentSize      int
	)

	for i := range disc.Modules {
		if disc.Modules[i].ModuleName == stdModuleName {
			stdModule = disc.Modules[i]
			break
		}
	}

	for i := range stdModule.SubModules {
		if stdModule.SubModules[i].Ds.Has(rootCol.ID) {
			segmentSize = stdModule.NewSizes[i]
			queryBasedModule = stdModule.SubModules[i]
			break
		}
	}

	return queryBasedModule, segmentSize
}

// SegmentBoundaryOfColumn returns the starting point and the ending point of the
// segmentation of column. The implementation works by identifying the corresponding
// [StandardModule] and then the corresponding inner [QueryBasedModule]. The function
// will then scans the assignment of the columns in the query based module and examine
// their padding to return the boundaries of the segmented area.
//
// To clarify, the segmentation of the column corresponds to the part that will be
// kept by the segmentation process of the column (e.g. the area covered by the
// reunion of all the segments). The rest of the assignment corresponds to padding.
func (disc *StandardModuleDiscoverer) SegmentBoundaryOf(run *wizard.ProverRuntime, col column.Natural) (int, int, paddingInformation) {
	qbm, segmentSize := disc.QbmOf(col)
	return qbm.SegmentBoundaries(run, segmentSize)
}

// Analyze processes columns and assigns them to modules.
//
// {1,2,3,4,5}
// {100}
// {6,7,8}
// {9,10}
// {3,6,20}
// {2,99}
//
// Processing:
// First Iteration - {1,2,3,4,5}
// No existing module.
// Create Module_0 → {1,2,3,4,5}
// Assign columns {1,2,3,4,5} to Module_0.
//
// Second Iteration - {100}
// No overlap with existing modules.
// Create Module_1 → {100}
// Assign {100} to Module_1.
//
// Third Iteration - {6,7,8}
// No overlap.
// Create Module_2 → {6,7,8}
// Assign {6,7,8} to Module_2.
//
// Fourth Iteration - {9,10}
// No overlap.
// Create Module_3 → {9,10}
// Assign {9,10} to Module_3.
//
// Fifth Iteration - {3,6,20}
// {3} is in Module_0, {6} is in Module_2 → Overlap detected.
// Merge Module_0 and Module_2 into Module_0.
// Module_0 now contains {1,2,3,4,5,6,7,8,20}.
// Remove Module_2 from moduleCandidates.
// Assign {3,6,20} to Module_0.
//
// Sixth Iteration - {2,99}
// {2} is in Module_0 → Overlap detected.
// Add {99} to Module_0.
// Module_0 now contains {1,2,3,4,5,6,7,8,20,99}.
// Assign {2,99} to Module_0.
//
// Final Modules:
// Module_0 → {1,2,3,4,5,6,7,8,20,99}
// Module_1 → {100}
// Module_3 → {9,10}
func (disc *QueryBasedModuleDiscoverer) Analyze(comp *wizard.CompiledIOP) {
	disc.Mutex.Lock()
	defer disc.Mutex.Unlock()

	disc.ColumnsToModule = make(map[ifaces.ColID]ModuleName)

	moduleCandidates := []*QueryBasedModule{}

	for _, qName := range comp.QueriesNoParams.AllKeys() {

		if comp.QueriesNoParams.IsIgnored(qName) {
			continue
		}

		queryData := comp.QueriesNoParams.Data(qName)

		var (
			// toGroup lists sets of columns who need to be grouped.
			toGroup = [][]column.Natural{}

			// nbConstraintsOfPlonkCirc tells if there are Plonk-in-Wizard overheads
			// associated with the modules columns. And how many constraints there
			// are.
			nbConstraintsOfPlonkCirc = 0
			nbInstancesOfPlonkCirc   = 0
			nbInstancesOfPlonkQuery  = 0
		)

		switch q := queryData.(type) {

		default:
			utils.Panic("unexpected query: type=%T name=%v", q, qName)

		case query.Range:
			// The query is expected but no grouping required.

		case query.GlobalConstraint:
			cols := column.ColumnsOfExpression(q.Expression)
			toGroup = append(toGroup, rootsOfColumns(cols))

		case query.LocalConstraint:
			cols := column.ColumnsOfExpression(q.Expression)
			toGroup = append(toGroup, rootsOfColumns(cols))

		case *query.PlonkInWizard:
			// Note: [q.CircuitMask] might be "nil" and it is ok. This is the reason
			// why we need that [column.RootsOf] filters out 'nil' inputs.
			cols := []ifaces.Column{q.Selector, q.Data}
			toGroup = append(toGroup, rootsOfColumns(cols))
			// Since there is only one possible option, we know that it can
			// only be a range-check.
			nbConstraintsOfPlonkCirc = utils.NextPowerOfTwo(countConstraintsOfPlonkCirc(q))
			nbInstancesOfPlonkCirc = q.GetMaxNbCircuitInstances()
			nbInstancesOfPlonkQuery = 1
		}

		if disc.Predivision > 0 {
			nbInstancesOfPlonkCirc /= disc.Predivision
		}

		for _, columns := range toGroup {
			moduleCandidates = disc.GroupColumns(
				columns,
				moduleCandidates,
				nbConstraintsOfPlonkCirc,
				nbInstancesOfPlonkCirc,
				nbInstancesOfPlonkQuery,
			)
		}
	}

	for _, qName := range comp.QueriesParams.AllUnignoredKeys() {

		var (
			// toGroup lists sets of columns who need to be grouped.
			toGroup = [][]column.Natural{}
		)

		switch q := comp.QueriesParams.Data(qName).(type) {

		default:
			utils.Panic("unexpected query: type=%T name=%v", q, qName)

		case query.LocalOpening:
			// Nothing to do

		case query.LogDerivativeSum:

			for _, part := range q.Inputs.Parts {
				colNums := column.ColumnsOfExpression(part.Num)
				colDens := column.ColumnsOfExpression(part.Den)
				rootNums, rootDens := rootsOfColumns(colNums), rootsOfColumns(colDens)
				toGroup = append(toGroup, append(rootNums, rootDens...))
			}

		// Note: this is case is already super-seded by our handling of the permutation
		// queries. So this should not bring any additional grouping.
		case query.GrandProduct:
			sizes := utils.SortedKeysOf(q.Inputs, func(a, b int) bool { return a < b })
			for _, size := range sizes {
				inpForSize := q.Inputs[size]
				for i := range inpForSize.Numerators {
					colNums := column.ColumnsOfExpression(inpForSize.Numerators[i])
					colDens := column.ColumnsOfExpression(inpForSize.Denominators[i])
					rootNums, rootDens := rootsOfColumns(colNums), rootsOfColumns(colDens)
					toGroup = append(toGroup, append(rootNums, rootDens...))
				}
			}

		case *query.Horner:
			for _, part := range q.Parts {
				group := []ifaces.Column{}
				for k := range part.Coefficients {
					group = append(group, column.ColumnsOfExpression(part.Coefficients[k])...)
					group = append(group, part.Selectors[k])
				}
				toGroup = append(toGroup, rootsOfColumns(group))
			}
		}

		for _, columns := range toGroup {
			moduleCandidates = disc.GroupColumns(
				columns,
				moduleCandidates,
				0, 0, 0,
			)
		}
	}

	// There could be some columns with no corresponding module. This happens
	// when the column is lookup table usually. In that case, we make it be
	// its own query based module.
	for _, colNames := range comp.Columns.AllKeys() {
		col := comp.Columns.GetHandle(colNames)
		foundModuleForCol := false
		for _, module := range disc.Modules {
			if module.Ds.Has(col.GetColID()) {
				foundModuleForCol = true
				break
			}
		}

		if !foundModuleForCol {
			disc.GroupColumns([]column.Natural{col.(column.Natural)}, moduleCandidates, 0, 0, 0)
		}
	}

	// Remove empty modules
	disc.RemoveNils()

	// Assign final module names after all processing
	for _, module := range disc.Modules {

		hasPrecomputed := false

		for colID := range module.Ds.Iter() {
			disc.ColumnsToModule[colID] = module.ModuleName
			status := comp.Columns.Status(colID)

			hp := status == column.Precomputed || status == column.VerifyingKey
			hasPrecomputed = hp || hasPrecomputed
		}

		module.HasPrecomputed = hasPrecomputed
		if !hasPrecomputed {
			module.Predivision = disc.Predivision
		}
		module.mustHaveConsistentLength(comp)
	}
}

// RemoveNils remove the empty modules from the list of modules.
func (disc *QueryBasedModuleDiscoverer) RemoveNils() {
	for i := len(disc.Modules) - 1; i >= 0; i-- {
		if disc.Modules[i].Ds.Size() == 0 {
			disc.Modules = append(disc.Modules[:i], disc.Modules[i+1:]...)
		}
	}
}

// CreateModule initializes a new module with a disjoint set and populates it with columns.
func (disc *QueryBasedModuleDiscoverer) CreateModule(columns []column.Natural) *QueryBasedModule {

	var colID ifaces.ColID
	if len(columns) > 0 {
		colID = columns[0].GetColID()
	} else {
		colID = "default" // columns is empty
	}

	columnIDs := make([]ifaces.ColID, len(columns))
	for i, c := range columns {
		columnIDs[i] = c.GetColID()
	}

	module := &QueryBasedModule{
		ModuleName:          ModuleName(fmt.Sprintf("Module_%d_%s", len(disc.Modules), colID)),
		Ds:                  utils.NewDisjointSetFromList(columnIDs),
		NbSegmentCache:      make(map[unsafe.Pointer][3]int),
		NbSegmentCacheMutex: &sync.Mutex{},
	}

	for i := 0; i < len(columnIDs); i++ {
		for j := i + 1; j < len(columnIDs); j++ {
			module.Ds.Union(columnIDs[i], columnIDs[j])
		}
	}

	disc.ModuleNames = append(disc.ModuleNames, module.ModuleName)
	disc.Modules = append(disc.Modules, module)
	return module
}

// MergeModules merges a list of overlapping modules into a single module.
func (disc *QueryBasedModuleDiscoverer) MergeModules(modules []*QueryBasedModule, moduleCandidates []*QueryBasedModule) *QueryBasedModule {
	if len(modules) == 0 {
		return nil
	}

	// Select the first module as the base
	mergedModule := modules[0]

	// Merge all remaining modules into the base
	for _, module := range modules[1:] {

		for col := range module.Ds.Iter() {
			mergedModule.Ds.Union(mergedModule.Ds.Find(col), col)
		}

		mergedModule.NbConstraintsOfPlonkCirc += module.NbConstraintsOfPlonkCirc
		mergedModule.NbInstancesOfPlonkCirc += module.NbInstancesOfPlonkCirc
		mergedModule.NbInstancesOfPlonkQuery += module.NbInstancesOfPlonkQuery
		mergedModule.HasPrecomputed = mergedModule.HasPrecomputed || module.HasPrecomputed

		// nilifying the module ensures it can no longer be matched for anything
		module.Nilify()
	}

	return mergedModule
}

// GroupColumns ensures that columns are grouped together, merging
// overlapping modules if necessary.
func (disc *QueryBasedModuleDiscoverer) GroupColumns(
	columns []column.Natural,
	moduleCandidates []*QueryBasedModule,
	nbConstraintsOfPlonkCirc int,
	nbInstancesOfPlonkCirc int,
	nbInstancesOfPlonkQuery int,
) []*QueryBasedModule {

	overlappingModules := []*QueryBasedModule{}

	// Find overlapping modules
	for _, module := range moduleCandidates {
		if module.HasOverlap(columns) {
			overlappingModules = append(overlappingModules, module)
		}
	}

	var assignedModule *QueryBasedModule

	columnIDs := make([]ifaces.ColID, len(columns))
	for i, c := range columns {
		columnIDs[i] = c.GetColID()
	}

	// Merge if necessary
	if len(overlappingModules) > 0 {

		assignedModule = disc.MergeModules(overlappingModules, moduleCandidates)
		assignedModule.NbConstraintsOfPlonkCirc += nbConstraintsOfPlonkCirc
		assignedModule.NbInstancesOfPlonkCirc += nbInstancesOfPlonkCirc
		assignedModule.NbInstancesOfPlonkQuery += nbInstancesOfPlonkQuery
		assignedModule.Ds.AddList(columnIDs)

	} else {

		// Create a new module
		assignedModule = disc.CreateModule(columns)
		assignedModule.NbConstraintsOfPlonkCirc += nbConstraintsOfPlonkCirc
		assignedModule.NbInstancesOfPlonkCirc += nbInstancesOfPlonkCirc
		assignedModule.NbInstancesOfPlonkQuery += nbInstancesOfPlonkQuery
		moduleCandidates = append(moduleCandidates, assignedModule)
	}

	return moduleCandidates
}

// SegmentBoundaries computes the density of a module given an assignment.
// This can be used to determine the number of segment of the module.
//
// The function works by iterating over the columns of the module and
// computing the density of each column. The density is defined as the
// number of non-padding elements. The function does a few quirks around
// the regular columns: they are not taken into account when evaluating
// the overall density of the module. That is because, it often happens
// that we suboptimally assign a whole regular column while it could in fact
// have a sparser representation. However, if all columns are regular,
// then we consider that the density is equal to the size of the module.
//
// Beside we consider that constants columns have a density of 0, and the
// function will return 0, 0 if all columns in the module are constants.
func (mod *QueryBasedModule) SegmentBoundaries(run *wizard.ProverRuntime, segmentSize int) (int, int, paddingInformation) {

	mod.NbSegmentCacheMutex.Lock()
	if res, ok := mod.NbSegmentCache[unsafe.Pointer(run)]; ok {
		mod.NbSegmentCacheMutex.Unlock()
		return res[0], res[1], paddingInformation(res[2])
	}
	mod.NbSegmentCacheMutex.Unlock()

	var (
		stats = mod.RecordAssignmentStats(run)
	)

	if stats.err != nil {
		panic(stats.err)
	}

	// As a sanity-check, there should not be contradictory pragmas
	if utils.Ternary(stats.NbPragmaFullColumn > 0, 1, 0)+
		utils.Ternary(stats.NbPragmaLeftPadded > 0, 1, 0)+
		utils.Ternary(stats.NbPragmaRightPadded > 0, 1, 0) > 1 {

		utils.Panic("there should not be contradictory pragmas, stats: %++v", stats)
	}

	// When all the assigned columns are constants (or there is no column)
	if stats.NbActiveRows == 0 {
		start, stop := 0, 0
		mod.NbSegmentCacheMutex.Lock()
		defer mod.NbSegmentCacheMutex.Unlock()
		mod.NbSegmentCache[unsafe.Pointer(run)] = [3]int{start, stop, constantPaddingInformation}
		return start, stop, constantPaddingInformation
	}

	if stats.NbPragmaRightPadded+stats.NbPragmaLeftPadded == 0 {
		// In theory, the first and the second condition are equivalent. This is a
		// double-check
		if stats.NbAssignedLeftPadded+stats.NbAssignedRightPadded == 0 ||
			stats.NbAssignedFullColumn+stats.NbAssignedConstantColumn == stats.NbColumns ||
			stats.NbPragmaFullColumn > 0 {

			start, stop := 0, stats.OriginalSize
			mod.NbSegmentCacheMutex.Lock()
			defer mod.NbSegmentCacheMutex.Unlock()
			mod.NbSegmentCache[unsafe.Pointer(run)] = [3]int{start, stop, noPaddingInformation}
			return start, stop, noPaddingInformation
		}
	}

	// If no pragma are given, we expect that there are not simultaneously
	// assigned left and right padded columns. Otherwise, we cannot guess which
	// case to refer to.
	if stats.NbPragmaLeftPadded == 0 &&
		stats.NbPragmaRightPadded == 0 &&
		stats.NbAssignedLeftPadded > 0 &&
		stats.NbAssignedRightPadded > 0 {
		utils.Panic("there should not be simultaneously assigned left and right padded columns, stats: %++v", stats)
	}

	var (
		localNbSegment     = utils.DivCeil(stats.NbActiveRows, segmentSize)
		totalSegmentedArea = segmentSize * localNbSegment
	)

	// In theory, the condition with the pragma is redundant based on the sanity
	// check we are doing.
	if stats.NbPragmaRightPadded > 0 || (stats.NbAssignedRightPadded > 0 && stats.NbPragmaLeftPadded == 0) {
		start, stop := 0, totalSegmentedArea
		mod.NbSegmentCacheMutex.Lock()
		defer mod.NbSegmentCacheMutex.Unlock()
		mod.NbSegmentCache[unsafe.Pointer(run)] = [3]int{start, stop, rightPaddingInformation}
		return start, stop, rightPaddingInformation
	}

	// In theory, the condition with the pragma is redundant based on the sanity
	// check we are doing.
	if stats.NbPragmaLeftPadded > 0 || (stats.NbAssignedLeftPadded > 0 && stats.NbPragmaRightPadded == 0) {
		start, stop := stats.OriginalSize-totalSegmentedArea, stats.OriginalSize
		mod.NbSegmentCacheMutex.Lock()
		defer mod.NbSegmentCacheMutex.Unlock()
		mod.NbSegmentCache[unsafe.Pointer(run)] = [3]int{start, stop, leftPaddingInformation}
		return start, stop, leftPaddingInformation
	}

	utils.Panic("unreachable: stats: %++v\n", stats)
	return 0, 0, noPaddingInformation // unreachable return
}

// RecordAssignmentStats scans the assignment of the module and reports stats
// related to the columns and their padding. It returns a list of records for
// each query-based submodule.
func (mod *StandardModule) RecordAssignmentStats(run *wizard.ProverRuntime) (qbmRecords []QueryBasedAssignmentStatsRecord) {
	for i, submod := range mod.SubModules {
		stats := submod.RecordAssignmentStats(run)
		stats.SegmentSize = mod.NewSizes[i]
		stats.ClusterName = mod.ModuleName
		qbmRecords = append(qbmRecords, stats)
	}
	return qbmRecords
}

// RecordAssignmentStats scans the assignment of the module and reports stats
// related to the columns and their padding.
func (mod *QueryBasedModule) RecordAssignmentStats(run *wizard.ProverRuntime) QueryBasedAssignmentStatsRecord {

	var (
		colIDs = utils.SortedKeysOf(mod.Ds.Parent, func(a, b ifaces.ColID) bool { return a < b })
		res    = QueryBasedAssignmentStatsRecord{
			ModuleName:               mod.ModuleName,
			OriginalSize:             mod.OriginalSize,
			NbConstraintsOfPlonkCirc: mod.NbConstraintsOfPlonkCirc,
			NbInstancesOfPlonkCirc:   mod.NbInstancesOfPlonkCirc,
			NbInstancesOfPlonkQuery:  mod.NbInstancesOfPlonkQuery,
			FirstColumnAlphabetical:  colIDs[0],
			LastColumnAlphabetical:   colIDs[len(colIDs)-1],
			NbColumns:                len(colIDs),
		}
		size        int
		firstColumn column.Natural
	)

	for _, colID := range colIDs {

		col := run.Spec.Columns.GetHandle(colID).(column.Natural)

		if size == 0 {
			size = col.Size()
			firstColumn = col
			res.OriginalSize = size
		}

		// This should not happen for a well-formed module, query-based modules
		// are expected to contain the same size for all columns.
		if size != col.Size() {
			res.err = errors.Join(res.err, fmt.Errorf("columns must have the same size, first=%v col=%v, sizeFirst=%v sizeCol=%v", firstColumn.ID, col.ID, size, col.Size()))
			continue
		}

		// As the function is meant to be called **during** the bootstrapping
		// compilation, we cannot expect all columns to be available at this
		// point. This is for instance the case with the lookup "M" columns.
		if !run.HasColumn(colID) {
			continue
		}

		if run.Spec.Precomputed.Exists(colID) {
			res.NbPrecomputed++
		}

		var (
			val                  = col.GetColAssignment(run)
			start, stop          = smartvectors.CoWindowRange(val)
			isRightPadded        = start == 0
			isLeftPadded         = stop == size
			density              = stop - start
			hasFullColumnPragma  = pragmas.IsFullColumn(col)
			hasLeftPaddedPragma  = pragmas.IsLeftPadded(col)
			hasRightPaddedPragma = pragmas.IsRightPadded(col)
		)

		switch {

		case hasFullColumnPragma:

			res.NbPragmaFullColumn++
			// This sets the nb active row to the maximal possible size for the
			// module.
			res.NbActiveRows = col.Size()

		case hasLeftPaddedPragma:

			// The user might provide non-left padded columns to the module but
			// then the wizard package is responsible for restructuring the
			// column so that it has the right shape within the assignment.
			if !isLeftPadded && density > 0 {
				res.err = errors.Join(res.err, fmt.Errorf("a left padded column must be left padded, col=%v", colID))
				continue
			}

			res.NbPragmaLeftPadded++
			res.LastLeftPadded = colID

		case hasRightPaddedPragma:

			// The user might provde non-right padded columns to the module but
			// then the wizard package is responsible for restructuring the
			// column so that it has the right shape within the assignment.
			if !isRightPadded && density > 0 {
				res.err = errors.Join(fmt.Errorf("a right padded column must be right padded, col=%v", colID))
				continue
			}

			res.NbPragmaRightPadded++
			res.LastRightPadded = colID
		}

		switch {

		// This case must be checked first because, the values of isLeftPadded
		// and isRightPadded are meaningless in that situation.
		case density == 0:

			res.NbAssignedConstantColumn++

		// This indicates that the column is assigned to a [Regular] vector,
		// most of the time, this indicates a sub-optimal assignment but this
		// is not a relevant indication for deducing the number of active row
		// and the padding direction of the module (unless the pragma is
		// activated).
		//
		// In case, all columns are either constant or full, the approach is
		// flawed and is "fixed" after the loop.
		case isRightPadded && isLeftPadded:

			res.NbAssignedFullColumn++
			// this avoid using the density to evaluate the density of the
			// module
			continue

		case isRightPadded:

			res.NbAssignedRightPadded++
			res.LastRightPadded = colID

		case isLeftPadded:

			res.NbAssignedLeftPadded++
			res.LastLeftPadded = colID

		case !isRightPadded && !isLeftPadded && density > 0:

			res.err = errors.Join(fmt.Errorf("column is neither left nor right padded, col=%v", colID))
			continue
		}

		if density > res.NbActiveRows {
			res.NbActiveRows = density
		}
	}

	// This covers for the case where there are only constant and/or full
	// columns in the module. In that case, the above loop does not give the
	// correct result. A special is when the module is explicitly highlighted
	// as padded with a pragma. In that situation, the number of active rows
	// is zero.
	if res.NbActiveRows == 0 && res.NbAssignedFullColumn > 0 && res.NbPragmaLeftPadded+res.NbPragmaRightPadded == 0 {
		res.NbActiveRows = size
	}

	return res
}

// Nilify a module. It empties its maps and sets its size to 0.
func (module *QueryBasedModule) Nilify() {
	module.Ds.Reset()
	module.OriginalSize = 0
	module.NbConstraintsOfPlonkCirc = 0
	module.NbInstancesOfPlonkCirc = 0
	module.NbInstancesOfPlonkQuery = 0
}

// HasOverlap checks if a module shares at least one column with a set of columns.
func (module *QueryBasedModule) HasOverlap(columns []column.Natural) bool {
	for _, col := range columns {
		if module.Ds.Has(col.ID) {
			return true
		}
	}
	return false
}

// Weight returns the weight metric of a module. The returned metric corresponds to
// a number of witness elements that the module represents. The function takes an
// optional "withNumRow" arguments to let the caller use a custom hypothetical value for
// [nbRows] in the calculation. If withNumRow=0, then the function uses the actual
// number of rows.
func (module *QueryBasedModule) Weight(comp *wizard.CompiledIOP, withNumRow int) int {

	var (
		numRow             = module.NumRow(comp)
		numCol             = module.NumColumn()
		numOfPlonkInstance = module.NbInstancesOfPlonkCirc
	)

	if withNumRow > 0 {
		numOfPlonkInstance = numOfPlonkInstance * withNumRow / numRow
		numRow = withNumRow
	}

	// The 4 and 11 are heuristic parameters to estimate the actual witness complexity
	// of having Plonk in wizards in the module.
	plonkCost := (4*numOfPlonkInstance + 11*module.NbInstancesOfPlonkQuery) * module.NbConstraintsOfPlonkCirc

	return numCol*numRow + plonkCost
}

// Weight returns the total weight of the module
func (module *StandardModule) Weight(comp *wizard.CompiledIOP) int {
	weight := 0
	for i := range module.SubModules {
		numRow := module.NewSizes[i]
		weight += module.SubModules[i].Weight(comp, numRow)
	}
	return weight
}

// NumRow returns the number of rows for the module
func (module *QueryBasedModule) NumRow(comp *wizard.CompiledIOP) int {
	if module.OriginalSize == 0 {

		for colID := range module.Ds.Iter() {

			colSize := comp.Columns.GetSize(colID)
			module.OriginalSize = colSize
			if module.HasPrecomputed {
				break
			}

			if module.Predivision > colSize || module.Predivision == 0 {
				break
			}

			module.OriginalSize /= module.Predivision
			break
		}
	}

	return module.OriginalSize
}

// NumColumn returns the number of columns for the module
func (module *QueryBasedModule) NumColumn() int {
	return module.Ds.Size()
}

// NewSizeOf returns the size (length) of a column.
func (disc *QueryBasedModuleDiscoverer) NewSizeOf(col column.Natural) int {
	size := col.Size()

	mod := disc.ModuleOf(col)
	for i := range disc.Modules {
		if disc.Modules[i].ModuleName == mod {
			qbm := disc.Modules[i]
			if qbm.HasPrecomputed {
				return size
			}
		}
	}

	if disc.Predivision > 0 && disc.Predivision < col.Size() {
		return size / disc.Predivision
	}

	return size
}

// ModuleList returns a list of all module names.
func (disc *QueryBasedModuleDiscoverer) ModuleList() []ModuleName {
	disc.Mutex.Lock()
	defer disc.Mutex.Unlock()
	return disc.ModuleNames
}

// ModuleOf returns the module name for a given column.
func (disc *QueryBasedModuleDiscoverer) ModuleOf(col column.Natural) ModuleName {
	disc.Mutex.Lock()
	defer disc.Mutex.Unlock()

	if moduleName, exists := disc.ColumnsToModule[col.ID]; exists {
		return moduleName
	}
	return ""
}

// countConstraintsOfPlonkCirc returns the number of constraint of the circuit. The function is
// quite expensive as this requires to compile the circuit. So it's better to cache
// the result when possible.
func countConstraintsOfPlonkCirc(piw *query.PlonkInWizard) int {

	hasAddGates := false
	if len(piw.PlonkOptions) > 0 {
		hasAddGates = piw.PlonkOptions[0].RangeCheckAddGateForRangeCheck
	}

	ccs, _, err := plonkinternal.CompileCircuitWithRangeCheck(piw.Circuit, hasAddGates)
	if err != nil {
		utils.Panic("unable to compile plonk-in-wizard circuit %s: %v", piw.ID, err)
	}
	nbConstraints := ccs.GetNbConstraints()
	return nbConstraints
}

// rootsOfColumns returns a clean and deduplicated list of the roots of the
// columns as [column.Natural] and not [ifaces.Column]
func rootsOfColumns(cols []ifaces.Column) []column.Natural {
	roots := column.RootsOf(cols, true)
	nats := make([]column.Natural, len(roots))
	for i := range roots {
		nats[i] = roots[i].(column.Natural)
	}
	return nats
}

// audit checks that all the columns taking part in the [QueryBasedModule] have
// the same length. It panics
func (m *QueryBasedModule) mustHaveConsistentLength(comp *wizard.CompiledIOP) {

	size := -1

	for colID := range m.Ds.Iter() {

		colSize := comp.Columns.GetSize(colID)

		if size == -1 {
			size = colSize
		}

		if size != colSize {
			utils.Panic("col=%v does not have a consistent size %v != %v", colID, size, colSize)
		}
	}
}

// groupQBModulesByAffinity groups the [QueryBasedModule] by affinity. Affinity
// consists in a group of columns that are to be grouped in the same standard
// module.
func groupQBModulesByAffinity(qbModules []*QueryBasedModule, affinities [][]column.Natural) (groups [][]*QueryBasedModule) {

	sets := make([]*collection.Set[*QueryBasedModule], len(qbModules))

	for i := range qbModules {
		s := collection.NewSet[*QueryBasedModule]()
		sets[i] = &s
		sets[i].Insert(qbModules[i])
	}

	for _, aff := range affinities {

		matched := make([]*collection.Set[*QueryBasedModule], 0)
		for i := range sets {

			isSetMatched := false

			for k := range aff {
				for qbm := range sets[i].Iter() {
					if qbm.Ds.Has(aff[k].ID) {
						isSetMatched = true
						continue
					}
				}
			}

			if isSetMatched {
				matched = append(matched, sets[i])
			}
		}

		if len(matched) <= 1 {
			continue
		}

		mergedModule := matched[0]

		for i := 1; i < len(matched); i++ {
			mergedModule.Merge(matched[i])
			matched[i].Clear()
		}
	}

	groups = make([][]*QueryBasedModule, 0, len(sets))

	for i := range sets {
		if sets[i].Size() == 0 {
			continue
		}

		groups = append(
			groups,
			sets[i].SortKeysBy(
				func(qbm1, qbm2 *QueryBasedModule) bool {
					return string(qbm1.ModuleName) < string(qbm2.ModuleName)
				},
			),
		)
	}

	return groups
}

func weightOfGroupOfQBModules(comp *wizard.CompiledIOP, group []*QueryBasedModule) int {
	var weight int
	for i := range group {
		weight += group[i].Weight(comp, 0)
	}
	return weight
}

// LPPSegmentBoundaryCalculator is an ad-hoc structure used to help the
// inclusion compiler understanding which parts of the S columns to use
// to compute the M columns.
type LPPSegmentBoundaryCalculator struct {
	Disc   *StandardModuleDiscoverer
	Cached map[*wizard.ProverRuntime]map[ifaces.ColID][2]int
}

func (ls *LPPSegmentBoundaryCalculator) SegmentBoundaryOf(run *wizard.ProverRuntime, col column.Natural) (int, int) {

	var (
		module                   = ls.Disc.ModuleOf(col)
		moduleNames              = ls.Disc.ModuleList()
		nbSegment                = -1
		start, stop, paddingInfo = ls.Disc.SegmentBoundaryOf(run, col)
		sizeOfSegment            = ls.Disc.NewSizeOf(col)
		fullSize                 = col.Size()
	)

	for i := range ls.Disc.Modules {
		if ls.Disc.Modules[i].ModuleName != module {
			continue
		}

		nbSegment = NbSegmentOfModule(run, ls.Disc, moduleNames[i])
	}

	// NewLen correspond to the sum of the size of all the segments that will be
	// generated for the provided column.
	newLen := nbSegment * sizeOfSegment

	// The newLen cannot be smaller than the stop-start because the number of LPP
	// segment for a column is always larger than the number of GL segments. This
	// is a consequence of the fact that LPP modules are aggregates of GL modules
	// and #moduleLpp.segments = max_i(#moduleGL_i.segments)
	if newLen < stop-start {
		utils.Panic("newLen=%v, stop=%v, start=%v, col=%v", newLen, stop, start, col.ID)
	}

	// This corresponds to the OOB case. In that situation, we have to resolve
	// the padding of the corresponding QBM to deduce the exact range to return.
	// The padding is left-wise, we will return size-newLen:size and 0:newLen if
	// the module is right padded. In case the module is "full", we panic as
	// there are no clear ways to guess what range to return.
	if newLen > fullSize {

		switch paddingInfo {
		case noPaddingInformation:
			qbm, _ := ls.Disc.QbmOf(col)
			utils.Panic("cannot guess how to pad the column, you may want to recheck your module grouping because the current one is not padded but grouped with padded modules, newLen=%v, stop=%v, start=%v, col=%v, qbm=%v module=%v", newLen, stop, start, col.ID, qbm.ModuleName, module)
		case constantPaddingInformation:
			return 0, 0 // There should not be anyway to end up in that situation
		case leftPaddingInformation:
			return fullSize - newLen, fullSize
		case rightPaddingInformation:
			return 0, newLen
		}

		panic("unreachable")
	}

	// Everything will be used. The case where newLen < stop-start is impossible due
	// to the above sanity-check. If newLen > stop-start, then it means we need to
	// extend the column. In the current situation, we only handle the case
	if start == 0 && stop == fullSize {
		if stop != newLen {
			utils.Panic("start=%v stop=%v, nbSegment=%v, newSize=%v, newLen=%v, col=%v", start, stop, nbSegment, sizeOfSegment, newLen, col.ID)
		}
		return start, stop
	}

	if start == 0 {
		return 0, newLen
	}

	if stop == fullSize {
		return fullSize - newLen, fullSize
	}

	utils.Panic("start=%v, stop=%v, nbSegment=%v, newSize=%v, newLen=%v, col=%v", start, stop, nbSegment, sizeOfSegment, newLen, col.ID)
	return -1, -1 // Unreachable
}

// IndexOf returns the module index for a given module name
func (disc *StandardModuleDiscoverer) IndexOf(moduleName ModuleName) int {
	for i := range disc.Modules {
		if disc.Modules[i].ModuleName == moduleName {
			return i
		}
	}
	panic("module not found")
}

// ModuleDiscoveryAdvice is an advice provided by the user and allows having
// fine-grained control over the assignment of columns to modules and their
// sizes.
type ModuleDiscoveryAdvice struct {
	Column    ifaces.Column
	Regexp    string
	ModuleRef string
	Cluster   ModuleName
	BaseSize  int
	rgxp      *regexp2.Regexp
}

// SameSizeAdvice returns an advice from a column where the base-size equals
// the one of the provided column.
func SameSizeAdvice(cls ModuleName, column ifaces.Column) *ModuleDiscoveryAdvice {
	return &ModuleDiscoveryAdvice{Column: column, Cluster: cls, BaseSize: column.Size()}
}

// DoesMatch returns true if the present advices matches the provided column.
func (ad *ModuleDiscoveryAdvice) DoesMatch(column ifaces.Column) bool {

	ad.assertWellFormed()

	if ad.Column != nil {
		// We should not have two columns with the same column ID. So that's a
		// valid and less-edge-case-ish way to compare two columns.
		return ad.Column.GetColID() == column.GetColID()
	}

	if len(ad.ModuleRef) > 0 {
		modRef, ok := pragmas.TryGetModuleRef(column)
		if !ok {
			return false
		}
		return modRef == ad.ModuleRef
	}

	// Assertedly, the regexp is provided and we already checked that in
	// [assertWellFormed].
	if ad.rgxp == nil {
		rgxp, err := regexp2.Compile(ad.Regexp, regexp2.Singleline)
		if err != nil {
			utils.Panic("failed to compile regexp: %++v", err)
		}
		ad.rgxp = rgxp
	}

	res, err := ad.rgxp.MatchString(string(column.GetColID()))
	if err != nil {
		utils.Panic("failed to match regexp: %s", err.Error())
	}

	return res
}

// AreConflicting returns true if the two advices conflict. Namely, if their
// BaseSize or Cluster differ.
func (ad ModuleDiscoveryAdvice) AreConflicting(other *ModuleDiscoveryAdvice) bool {
	return ad.BaseSize != other.BaseSize || ad.Cluster != other.Cluster
}

// assertWellFormeded sanity-checks if the advice is well-formed.
func (ad ModuleDiscoveryAdvice) assertWellFormed() error {

	if ad.Column == nil && len(ad.Regexp) == 0 && len(ad.ModuleRef) == 0 {
		utils.Panic("advice does not specify a column or a regexp: %++v", ad)
	}

	if !utils.IsPowerOfTwo(ad.BaseSize) {
		utils.Panic("advice does not specify a base size that is a power of two, %++v", ad)
	}

	if len(ad.Cluster) == 0 {
		utils.Panic("advice does not specify a cluster: %++v", ad)
	}

	return nil
}
