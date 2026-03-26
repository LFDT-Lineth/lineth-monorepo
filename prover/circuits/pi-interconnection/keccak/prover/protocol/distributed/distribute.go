package distributed

import (
	"errors"
	"fmt"

	cmimc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
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
	serialization.RegisterImplementation(ConglomerationHierarchicalVerifierAction{})
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

	for i := 0; i < nbLPP; i++ {

		var (
			moduleLPP         = BuildModuleLPP(allFilteredModuleInputs[i])
			moduleLPPForDebug = BuildModuleLPP(allFilteredModuleInputs[i])
		)

		// This add sanity-checking the module initial assignment. Without
		// this compilation, the module would not check anything.
		wizard.ContinueCompilation(
			moduleLPPForDebug.Wiop,
			dummy.CompileAtProverLvl(dummy.WithMsg(fmt.Sprintf("LPP module (debug) %d", i))),
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

	return distributedWizard
}

// CompileModules applies the compilation steps to each modules identically.
func (dist *DistributedWizard) CompileSegments(params CompilationParams) *DistributedWizard {
	logrus.Infoln("Compiling distributed wizard default module")

	logrus.Infof("Number of GL modules to compile:%d\n", len(dist.GLs))
	dist.CompiledGLs = make([]*RecursedSegmentCompilation, len(dist.GLs))
	for i := range dist.GLs {

		logrus.
			WithField("module-name", dist.GLs[i].DefinitionInput.ModuleName).
			WithField("module-type", "GL").
			Info("compiling module")

		dist.CompiledGLs[i] = CompileSegment(dist.GLs[i], params)
	}

	logrus.Infof("Number of LPP modules to compile:%d\n", len(dist.LPPs))
	dist.CompiledLPPs = make([]*RecursedSegmentCompilation, len(dist.LPPs))
	for i := range dist.LPPs {
		logrus.
			WithField("module-name", dist.LPPs[i].ModuleName()).
			WithField("module-type", "LPP").
			Info("compiling module")

		dist.CompiledLPPs[i] = CompileSegment(dist.LPPs[i], params)
	}

	return dist
}

// GetSharedRandomnessFromSegmentProofs returns the shared randomness used by
// the protocol to generate the LPP proofs. The LPP commitments are supposed to
// be the one extractable from the [recursion.Witness] of the LPPs.
//
// The result of this function is to be used as the shared randomness for
// the LPP provers.
func GetSharedRandomnessFromSegmentProofs(gLWitnesses []*SegmentProof) field.Element {

	mset := cmimc.MSetHash{}

	for i := range gLWitnesses {

		var (
			moduleIndex, _  = new(field.Element).SetInterface(gLWitnesses[i].ModuleIndex)
			segmentIndex, _ = new(field.Element).SetInterface(gLWitnesses[i].SegmentIndex)
			lppCommitment   = gLWitnesses[i].LppCommitment
		)

		mset.Insert(*moduleIndex, *segmentIndex, lppCommitment)
	}

	return cmimc.HashVec(mset[:])
}

// getLppCommitmentFromRuntime returns the LPP commitment from the runtime
func getLppCommitmentFromRuntime(runtime *wizard.ProverRuntime) field.Element {
	name := fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, 0)
	return runtime.GetPublicInput(name)
}

// PrecompileInitialWizard pre-compiles the initial wizard protocol by applying all the
// compilation steps needing to be run before the splitting phase. Its role is to
// ensure that all of the queries that cannot be processed by the splitting phase
// are removed from the compiled IOP.
func PrecompileInitialWizard(comp *wizard.CompiledIOP, disc *StandardModuleDiscoverer) *wizard.CompiledIOP {

	return wizard.ContinueCompilation(
		comp,
		mimc.CompileMiMC,
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

// tagFunctionalInputs takes the public inputs of the original wizard and
// changes their name to "functional". This is meant to help us recognize them
func tagFunctionalInputs(comp *wizard.CompiledIOP) {
	for i := range comp.PublicInputs {
		comp.PublicInputs[i].Name = "functional." + comp.PublicInputs[i].Name
	}
}
