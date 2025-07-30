package distributed

import (
	"errors"
	"fmt"

	cmimc "github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// lppGroupingArity indicates how many GL modules an LPP module relates to. The
// value is fixed to 1 not for efficiency concern but putting it to a higher
// value creates edge-cases for the FS security that are not fully-addressed yet.
const lppGroupingArity = 1

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

	// DefaultModule is the module used for filling when the number of
	// effective segment is smaller than the maximum number of segment to
	// conglomerate.
	DefaultModule *DefaultModule

	// Bootstrapper is the original compiledIOP precompiled with a few
	// preparation steps.
	Bootstrapper *wizard.CompiledIOP

	// Disc is the [*StandardModuleDiscoverer] used to delimitate the scope for
	// each module.
	Disc *StandardModuleDiscoverer

	// CompiledDefault stores the compiled default module and is set by calling
	// [DistributedWizard.Compile]
	CompiledDefault *RecursedSegmentCompilation

	// CompiledGLs stores the compiled GL modules and is set by calling
	// [DistributedWizard.Compile]
	CompiledGLs []*RecursedSegmentCompilation

	// CompiledLPPs stores the compiled LPP modules and is set by calling
	// [DistributedWizard.Compile]
	CompiledLPPs []*RecursedSegmentCompilation

	// CompiledConglomeration stores the compilation context of the
	// conglomeration wizard.
	CompiledConglomeration *ConglomeratorCompilation
}

func init() {

	serialization.RegisterImplementation(AssignLPPQueries{})
	serialization.RegisterImplementation(SetInitialFSHash{})
	serialization.RegisterImplementation(CheckNxHash{})
	serialization.RegisterImplementation(StandardModuleDiscoverer{})
	serialization.RegisterImplementation(LppWitnessAssignment{})
	serialization.RegisterImplementation(ModuleGLAssignGL{})
	serialization.RegisterImplementation(ModuleGLAssignSendReceiveGlobal{})
	serialization.RegisterImplementation(ModuleGLCheckSendReceiveGlobal{})
	serialization.RegisterImplementation(LPPSegmentBoundaryCalculator{})
	serialization.RegisterImplementation(ConglomerateHolisticCheck{})
	serialization.RegisterImplementation(ConglomerationAssignHolisticCheckColumn{})
}

// DistributeWizard returns a [DistributedWizard] from a [wizard.CompiledIOP]. It
// takes ownership of the input [wizard.CompiledIOP]. And uses disc to design
// the scope of each module.
func DistributeWizard(comp *wizard.CompiledIOP, disc *StandardModuleDiscoverer) *DistributedWizard {

	if err := auditInitialWizard(comp); err != nil {
		utils.Panic("improper initial wizard for distribution: %v", err)
	}

	distributedWizard := &DistributedWizard{
		Bootstrapper: PrecompileInitialWizard(comp, disc),
		Disc:         disc,
	}

	disc.Analyze(distributedWizard.Bootstrapper)
	distributedWizard.ModuleNames = disc.ModuleList()

	allFilteredModuleInputs := make([]FilteredModuleInputs, 0)

	for _, moduleName := range distributedWizard.ModuleNames {

		moduleFilter := moduleFilter{
			Disc:   disc,
			Module: moduleName,
		}

		filteredModuleInputs := moduleFilter.FilterCompiledIOP(
			distributedWizard.Bootstrapper,
		)

		logrus.Infof("Compiling GL module %v", moduleName)

		var (
			moduleGL         = BuildModuleGL(&filteredModuleInputs)
			moduleGLForDebug = BuildModuleGL(&filteredModuleInputs)
		)

		// This add sanity-checking the module initial assignment. Without
		// this compilation, the module would not check anything.
		wizard.ContinueCompilation(
			moduleGLForDebug.Wiop,
			dummy.CompileAtProverLvl(dummy.WithMsg(fmt.Sprintf("GL module (debug) %v", moduleName))),
		)

		distributedWizard.DebugGLs = append(
			distributedWizard.DebugGLs,
			moduleGLForDebug,
		)

		distributedWizard.GLs = append(
			distributedWizard.GLs,
			moduleGL,
		)

		distributedWizard.BlueprintGLs = append(
			distributedWizard.BlueprintGLs,
			moduleGL.Blueprint(),
		)

		allFilteredModuleInputs = append(
			allFilteredModuleInputs,
			filteredModuleInputs,
		)
	}

	var (
		nbLPP = len(distributedWizard.ModuleNames)
	)

	for i := 0; i < nbLPP; i += lppGroupingArity {

		stop := min(len(distributedWizard.ModuleNames), i+lppGroupingArity)

		logrus.Infof("Compiling LPP modules [%d .. %d]", i, stop)

		var (
			moduleLPP         = BuildModuleLPP(allFilteredModuleInputs[i:stop])
			moduleLPPForDebug = BuildModuleLPP(allFilteredModuleInputs[i:stop])
		)

		// This add sanity-checking the module initial assignment. Without
		// this compilation, the module would not check anything.
		wizard.ContinueCompilation(
			moduleLPPForDebug.Wiop,
			dummy.CompileAtProverLvl(dummy.WithMsg(fmt.Sprintf("LPP module (debug) [%d .. %d]", i, stop))),
		)

		distributedWizard.DebugLPPs = append(
			distributedWizard.DebugLPPs,
			moduleLPPForDebug,
		)

		distributedWizard.LPPs = append(
			distributedWizard.LPPs,
			moduleLPP,
		)

		distributedWizard.BlueprintLPPs = append(
			distributedWizard.BlueprintLPPs,
			moduleLPP.Blueprint(),
		)
	}

	distributedWizard.DefaultModule = BuildDefaultModule(&allFilteredModuleInputs[0])

	return distributedWizard
}

// CompileModules applies the compilation steps to each modules identically.
func (dist *DistributedWizard) CompileSegments() *DistributedWizard {
	logrus.Infoln("Compiling distributed wizard default module")
	dist.CompiledDefault = CompileSegment(dist.DefaultModule)

	logrus.Infof("Number of GL modules to compile:%d\n", len(dist.GLs))
	dist.CompiledGLs = make([]*RecursedSegmentCompilation, len(dist.GLs))
	for i := range dist.GLs {
		logrus.
			WithField("module-name", dist.GLs[i].DefinitionInput.ModuleName).
			WithField("module-type", "GL").
			Info("compiling module")

		dist.CompiledGLs[i] = CompileSegment(dist.GLs[i])
	}

	logrus.Infof("Number of LPP modules to compile:%d\n", len(dist.LPPs))
	dist.CompiledLPPs = make([]*RecursedSegmentCompilation, len(dist.LPPs))
	for i := range dist.LPPs {
		logrus.
			WithField("module-name", dist.LPPs[i].ModuleNames()).
			WithField("module-type", "LPP").
			Info("compiling module")

		dist.CompiledLPPs[i] = CompileSegment(dist.LPPs[i])
	}

	return dist
}

// Conglomerate registers the conglomeration wizard and compiles it.
func (dist *DistributedWizard) Conglomerate(maxNumSegment int) *DistributedWizard {
	dist.CompiledConglomeration = conglomerate(
		maxNumSegment,
		dist.CompiledGLs,
		dist.CompiledLPPs,
		dist.CompiledDefault,
	)
	return dist
}

// GetSharedRandomness returns the shared randomness used by the protocol
// to generate the LPP proofs. The LPP commitments are supposed to be the
// one extractable from the [recursion.Witness] of the LPPs.
//
// The result of this function is to be used as the shared randomness for
// the LPP provers.
func GetSharedRandomness(lppCommitments []field.Element) field.Element {
	return cmimc.HashVec(lppCommitments)
}

// GetSharedRandomnessFromRuntime returns the shared randomness used by the protocol
// to generate the LPP proofs. The LPP commitments are supposed to be the
// one extractable from the [recursion.Witness] of the LPPs.
//
// The result of this function is to be used as the shared randomness for
// the LPP provers.
func GetSharedRandomnessFromWitnesses(comp []*wizard.CompiledIOP, gLWitnesses []recursion.Witness) field.Element {
	lppCommitments := []field.Element{}
	for i := range gLWitnesses {
		name := fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, 0)
		lpp := gLWitnesses[i].Proof.GetPublicInput(comp[i], preRecursionPrefix+name)
		lppCommitments = append(lppCommitments, lpp)
	}
	return GetSharedRandomness(lppCommitments)
}

// GetLppCommitmentFromRuntime returns the LPP commitment from the runtime
func GetLppCommitmentFromRuntime(runtime *wizard.ProverRuntime) field.Element {
	name := fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, 0)
	return runtime.GetPublicInput(preRecursionPrefix + name)
}

// PrecompileInitialWizard pre-compiles the initial wizard protocol by applying all the
// compilation steps needing to be run before the splitting phase. Its role is to
// ensure that all of the queries that cannot be processed by the splitting phase
// are removed from the compiled IOP.
func PrecompileInitialWizard(comp *wizard.CompiledIOP, disc *StandardModuleDiscoverer) *wizard.CompiledIOP {
	mimc.CompileMiMC(comp)
	// specialqueries.RangeProof(comp)
	// specialqueries.CompileFixedPermutations(comp)
	logderivativesum.LookupIntoLogDerivativeSumWithSegmenter(
		&LPPSegmentBoundaryCalculator{
			Disc:     disc,
			LPPArity: lppGroupingArity,
		},
	)(comp)
	permutation.CompileIntoGdProduct(comp)
	horner.ProjectionToHorner(comp)
	return comp
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

	allQueriesNoParams := comp.QueriesNoParams.AllKeys()
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
						err = errors.Join(err, fmt.Errorf("incompatible permutation sizes: %v, column %v has the wrong size", perm, perm.A[i][j].GetColID()))
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

		err = errors.Join(err)
	}

	return err
}
