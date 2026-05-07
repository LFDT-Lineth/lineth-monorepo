package distributed

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	multsethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// DistributedWizard represents a wizard protocol that has undergone a
// distributed compilation process.
type DistributedWizard struct {

	// ModuleNames is the list of the names of the modules that compose
	// the distributed protocol.
	ModuleNames []ModuleName

	// LPPs is the list of the LPP parts for every modules
	LPPs []*ModuleLPP

	// GLs is the list of the GL parts for every modules
	GLs []*ModuleGL

	// DebugLPPs is the list of the LPP modules compiles with the dummy, this is
	// convenient for debugging the distributed wizard after segmentation.
	DebugLPPs []*ModuleLPP

	// DebugGLs is the list of the GL modules compiles with the dummy, this is
	// convenient for debugging the distributed wizard after segmentation.
	DebugGLs []*ModuleGL

	// BlueprintGLs is the list of the blueprints for each GL module
	BlueprintGLs []ModuleSegmentationBlueprint

	// BlueprintLPPs is the list of the blueprints for each LPP module
	BlueprintLPPs []ModuleSegmentationBlueprint

	// Bootstrapper is the original compiledIOP precompiled with a few
	// preparation steps.
	Bootstrapper *wizard.CompiledIOP

	// Disc is the [*StandardModuleDiscoverer] used to delimitate the scope for
	// each module.
	Disc *StandardModuleDiscoverer

	// CompiledGLs stores the compiled GL modules and is set by calling
	// [DistributedWizard.Compile]
	CompiledGLs []*RecursedSegmentCompilation

	// CompiledLPPs stores the compiled LPP modules and is set by calling
	// [DistributedWizard.Compile]
	CompiledLPPs []*RecursedSegmentCompilation

	// CompiledConglomeration stores the compilation context of the
	// conglomeration wizard.
	CompiledConglomeration *RecursedSegmentCompilation

	// VerificationKeyMerkleTree
	VerificationKeyMerkleTree VerificationKeyMerkleTree

	precompiledConglomeration     *RecursedSegmentCompilation
	precompiledConglomerationDone chan struct{}
	precompiledConglomerationOnce sync.Once
}

type DistributeWizardOptions struct {
	BuildDebugModules bool
}

// DistributeWizard returns a [DistributedWizard] from a [wizard.CompiledIOP]. It
// takes ownership of the input [wizard.CompiledIOP]. And uses disc to design
// the scope of each module.
func DistributeWizard(comp *wizard.CompiledIOP, disc *StandardModuleDiscoverer) *DistributedWizard {
	return DistributeWizardWithOptions(comp, disc, DistributeWizardOptions{
		BuildDebugModules: true,
	})
}

// DistributeWizardWithOptions is like [DistributeWizard] but allows callers to
// skip optional work such as debug-module construction.
func DistributeWizardWithOptions(
	comp *wizard.CompiledIOP,
	disc *StandardModuleDiscoverer,
	opts DistributeWizardOptions,
) *DistributedWizard {

	// We complie the comp object to manually shift all the columns that need to be shifted. This is due to the
	// fact that distributed wizard does not support shifted columns.
	CompileManualShifter(comp)

	bootstrapper := PrecompileInitialWizard(comp, disc)

	if err := auditInitialWizard(comp); err != nil {
		utils.Panic("improper initial wizard for distribution: %v", err)
	}

	distributedWizard := &DistributedWizard{
		Bootstrapper: bootstrapper,
		Disc:         disc,
	}

	disc.Analyze(distributedWizard.Bootstrapper)
	distributedWizard.ModuleNames = disc.ModuleList()

	buildModuleWorkers := 4
	if v := os.Getenv("LIMITLESS_BUILD_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			buildModuleWorkers = n
		}
	} else {
		buildModuleWorkers = min(4, runtime.GOMAXPROCS(0))
	}

	numModules := len(distributedWizard.ModuleNames)
	allFilteredModuleInputs := make([]FilteredModuleInputs, numModules)
	distributedWizard.GLs = make([]*ModuleGL, numModules)
	if opts.BuildDebugModules {
		distributedWizard.DebugGLs = make([]*ModuleGL, numModules)
	}
	distributedWizard.BlueprintGLs = make([]ModuleSegmentationBlueprint, numModules)

	var glBuildGroup errgroup.Group
	glBuildGroup.SetLimit(buildModuleWorkers)
	for i, moduleName := range distributedWizard.ModuleNames {
		i, moduleName := i, moduleName
		glBuildGroup.Go(func() error {
			moduleFilter := moduleFilter{
				Disc:   disc,
				Module: moduleName,
			}

			filteredModuleInputs := moduleFilter.FilterCompiledIOP(
				distributedWizard.Bootstrapper,
			)
			moduleGL := BuildModuleGL(&filteredModuleInputs)

			allFilteredModuleInputs[i] = filteredModuleInputs
			distributedWizard.GLs[i] = moduleGL
			if opts.BuildDebugModules {
				moduleGLForDebug := BuildModuleGL(&filteredModuleInputs)

				// This add sanity-checking the module initial assignment. Without
				// this compilation, the module would not check anything.
				wizard.ContinueCompilation(
					moduleGLForDebug.Wiop,
					dummy.CompileAtProverLvl(dummy.WithMsg(fmt.Sprintf("GL module (debug) %v", moduleName))),
				)
				distributedWizard.DebugGLs[i] = moduleGLForDebug
			}
			distributedWizard.BlueprintGLs[i] = moduleGL.Blueprint()
			return nil
		})
	}
	if err := glBuildGroup.Wait(); err != nil {
		utils.Panic("GL module build failed: %v", err)
	}

	distributedWizard.LPPs = make([]*ModuleLPP, numModules)
	if opts.BuildDebugModules {
		distributedWizard.DebugLPPs = make([]*ModuleLPP, numModules)
	}
	distributedWizard.BlueprintLPPs = make([]ModuleSegmentationBlueprint, numModules)
	var lppBuildGroup errgroup.Group
	lppBuildGroup.SetLimit(buildModuleWorkers)
	for i := range numModules {
		i := i
		lppBuildGroup.Go(func() error {
			moduleLPP := BuildModuleLPP(allFilteredModuleInputs[i])

			distributedWizard.LPPs[i] = moduleLPP
			if opts.BuildDebugModules {
				moduleLPPForDebug := BuildModuleLPP(allFilteredModuleInputs[i])

				// This add sanity-checking the module initial assignment. Without
				// this compilation, the module would not check anything.
				wizard.ContinueCompilation(
					moduleLPPForDebug.Wiop,
					dummy.CompileAtProverLvl(dummy.WithMsg(fmt.Sprintf("LPP module (debug) %d", i))),
				)
				distributedWizard.DebugLPPs[i] = moduleLPPForDebug
			}
			distributedWizard.BlueprintLPPs[i] = moduleLPP.Blueprint()
			return nil
		})
	}
	if err := lppBuildGroup.Wait(); err != nil {
		utils.Panic("LPP module build failed: %v", err)
	}

	logrus.Infof("Built %d modules (debug=%v)", len(distributedWizard.ModuleNames), opts.BuildDebugModules)

	return distributedWizard
}

// CompileSegments applies the compilation steps to each module. It uses
// parallel workers when LIMITLESS_COMPILE_JOBS > 1 (default: 4).
func (dist *DistributedWizard) CompileSegments(params CompilationParams) *DistributedWizard {
	logrus.Infoln("Compiling distributed wizard default module")

	numWorkers := 4 // default: 4 parallel compilations
	if v := os.Getenv("LIMITLESS_COMPILE_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			numWorkers = n
		}
	}

	logrus.Infof("Compiling %d GL + %d LPP modules with %d workers\n", len(dist.GLs), len(dist.LPPs), numWorkers)

	dist.CompiledGLs = make([]*RecursedSegmentCompilation, len(dist.GLs))
	dist.CompiledLPPs = make([]*RecursedSegmentCompilation, len(dist.LPPs))

	// Build a unified job list: GL first, then LPP
	type compileJob struct {
		isLPP bool
		index int
	}

	jobs := make([]compileJob, 0, len(dist.GLs)+len(dist.LPPs))
	for i := range dist.GLs {
		jobs = append(jobs, compileJob{isLPP: false, index: i})
	}
	for i := range dist.LPPs {
		jobs = append(jobs, compileJob{isLPP: true, index: i})
	}

	var wg errgroup.Group
	wg.SetLimit(numWorkers)

	for _, job := range jobs {
		job := job
		wg.Go(func() error {
			if job.isLPP {
				moduleName := dist.LPPs[job.index].ModuleName()
				logrus.WithField("module-name", moduleName).
					WithField("module-type", "LPP").
					Info("compiling module")
				dist.CompiledLPPs[job.index] = CompileSegment(dist.LPPs[job.index], params)
				logrus.WithField("module-name", moduleName).
					WithField("module-type", "LPP").
					Info("compiled module")
			} else {
				moduleName := dist.GLs[job.index].DefinitionInput.ModuleName
				logrus.WithField("module-name", moduleName).
					WithField("module-type", "GL").
					Info("compiling module")
				dist.CompiledGLs[job.index] = CompileSegment(dist.GLs[job.index], params)
				if job.index == 0 {
					dist.startPrecompilingConglomeration(params)
				}
				logrus.WithField("module-name", moduleName).
					WithField("module-type", "GL").
					Info("compiled module")
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		utils.Panic("compilation failed: %v", err)
	}

	return dist
}

func (dist *DistributedWizard) startPrecompilingConglomeration(params CompilationParams) {
	dist.precompiledConglomerationOnce.Do(func() {
		dist.precompiledConglomerationDone = make(chan struct{})
		go func() {
			defer close(dist.precompiledConglomerationDone)

			conglo := &ModuleConglo{
				ModuleNumber: len(dist.CompiledGLs),
			}

			comp := wizard.NewCompiledIOP()
			conglo.Compile(comp, dist.CompiledGLs[0].RecursionCompKoala)
			dist.precompiledConglomeration = CompileSegment(conglo, params)

		}()
	})
}

// GetSharedRandomnessFromSegmentProofs returns the shared randomness used by
// the protocol to generate the LPP proofs. The LPP commitments are supposed to
// be the one extractable from the [recursion.Witness] of the LPPs.
//
// The result of this function is to be used as the shared randomness for
// the LPP provers.
func GetSharedRandomnessFromSegmentProofs(gLWitnesses []*SegmentProof) field.Octuplet {

	mset := multsethashing.MSetHash{}

	for i := range gLWitnesses {

		var (
			moduleIndex   = field.NewElement(uint64(gLWitnesses[i].ModuleIndex))
			segmentIndex  = field.NewElement(uint64(gLWitnesses[i].SegmentIndex))
			lppCommitment = gLWitnesses[i].LppCommitment
		)

		mset.Insert(append([]field.Element{moduleIndex, segmentIndex}, lppCommitment[:]...)...)
	}

	return poseidon2_koalabear.HashVec(mset[:]...)
}

// getLppCommitmentFromRuntime returns the LPP commitment from the runtime
func getLppCommitmentFromRuntime(runtime *wizard.ProverRuntime) field.Octuplet {
	merkleRoot := field.Octuplet{}
	for i := range merkleRoot {
		merkleRoot[i] = runtime.GetPublicInput(fmt.Sprintf("%v_%v_%v", lppMerkleRootPublicInput, 0, i)).Base // index 0 stands for the round index.
	}
	return merkleRoot
}

// PrecompileInitialWizard pre-compiles the initial wizard protocol by applying all the
// compilation steps needing to be run before the splitting phase. Its role is to
// ensure that all of the queries that cannot be processed by the splitting phase
// are removed from the compiled IOP.
func PrecompileInitialWizard(comp *wizard.CompiledIOP, disc *StandardModuleDiscoverer) *wizard.CompiledIOP {

	return wizard.ContinueCompilation(
		comp,
		poseidon2.CompilePoseidon2,
		logderivativesum.LookupIntoLogDerivativeSumWithSegmenter(
			&LPPSegmentBoundaryCalculator{
				Disc: disc,
			},
		),
		permutation.CompileIntoGdProduct,
		horner.ProjectionToHorner,
		tagFunctionalInputs,
	)
}

// auditInitialWizard scans the initial compiled-IOP and checks if the provided
// wizard is compatible with the [DistributedWizard]. This includes:
//
//   - Absence of precomputed columns in the wizard (except as fixed lookup tables)
//   - Absence of [verifiercol.VerifierCol] (except [verifiercol.ConstCol]) in all
//     queries.
//   - Selectors for Inclusions or Projections must be either [column.Natural] or
//     [verifiercol.ConstCol]
func auditInitialWizard(comp *wizard.CompiledIOP) error {

	var err error

	allQueriesNoParams := comp.QueriesNoParams.AllUnignoredKeys()
	for _, qname := range allQueriesNoParams {

		q := comp.QueriesNoParams.Data(qname)

		if glob, isGlob := q.(query.GlobalConstraint); isGlob {
			var (
				cols     = column.ColumnsOfExpression(glob.Expression)
				rootCols = column.RootsOf(cols, true)
			)

			for _, col := range rootCols {
				if comp.Precomputed.Exists(col.GetColID()) {
					err = errors.Join(err, fmt.Errorf("found precomputed column: %v", col))
				}
			}
		}

		// Since we group all the columns of a permutation query to get in the same
		// module we as well need them to have the same size.
		if perm, isPerm := q.(query.Permutation); isPerm {

			size := perm.A[0][0].Size()
			for i := range perm.A {
				for j := range perm.A[i] {
					if perm.A[i][j].Size() != size {
						err = errors.Join(err, fmt.Errorf("incompatible permutation sizes: %v, column %v has the wrong size", perm.ID, perm.A[i][j].GetColID()))
					}
				}
			}

			for i := range perm.B {
				for j := range perm.B[i] {
					if perm.B[i][j].Size() != size {
						err = errors.Join(err, fmt.Errorf("incompatible permutation sizes: %v, column %v has the wrong size", perm, perm.B[i][j].GetColID()))
					}
				}
			}
		}

		switch q_ := q.(type) {

		case interface{ GetShiftedRelatedColumns() []ifaces.Column }:
			shfted := q_.GetShiftedRelatedColumns()
			if len(shfted) > 0 {
				err = errors.Join(err, fmt.Errorf("inclusion query %v with shifted selectors %v", qname, shfted))
			}
		}
	}

	return err
}

// tagFunctionalInputs takes the public inputs of the original wizard and
// changes their name to "functional". This is meant to help us recognize them
func tagFunctionalInputs(comp *wizard.CompiledIOP) {
	for i := range comp.PublicInputs {
		comp.PublicInputs[i].Name = "functional." + comp.PublicInputs[i].Name
	}
}
