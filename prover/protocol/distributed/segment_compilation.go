package distributed

import (
	"fmt"
	"slices"
	"strings"

	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/sirupsen/logrus"
)

// CompilationParams gather the different compilation parameters to use to
// compile the segments.
type CompilationParams struct {

	// FixedNbRowPlonkCircuit is the number of rows in the plonk circuit, if
	// the compilation process generates a recursion circuit with more rows than
	// this number, then the compilation will fail with panic. If the number of
	// rows is less, then we add dummy padding rows.
	FixedNbRowPlonkCircuit int

	// FixedNbRowExternalHasher is the number of rows in the external hasher
	// circuit. It works the same way as [FixedNbRowPlonkCircuit] but for the
	// number of calls to the external hasher.
	FixedNbRowExternalHasher int

	// FixedNbPublicInput is the size of the public input vector of the
	// recursion circuit. It works the same way as [FixedNbRowPlonkCircuit]
	// but for the number of public inputs.
	FixedNbPublicInput int

	// InitialCompilerSize sets the target number of rows of the first
	// invokation of [compiler.Arcane] of the pre-recursion pass of
	// [CompileSegment]. It is applicable only for the GL and LPP proofs. The
	// conglomeration circuit uses a different parameter.
	InitialCompilerSize int

	// InitialCompilerSizeConglo sets the target number of rows of the first
	// invokation of [compiler.Arcane] of the pre-recursion pass of
	// [CompileSegment] for the conglomeration circuit.
	InitialCompilerSizeConglo int

	// ColumnProfileMPTS gives the number of rows for each round to target
	// before the recursion step.
	ColumnProfileMPTS []int

	// ColumnProfileMPTSPrecomputed is the number of rows for the precomputed
	// round.
	ColumnProfileMPTSPrecomputed int

	// FullDebugMode tells the compiler to add debugging steps to help track
	// errors.
	FullDebugMode bool
}

// RecursedSegmentCompilation collects all the wizard compilation artefacts
// to compile a segment of the protocol into a standardized recursed proof.
// A standardized proof has a verifier with a standard structure which has
// the same verifier circuit as other segments of the protocol.
type RecursedSegmentCompilation struct {
	// ModuleGL is optional and is set if the segment is a GL segment.
	ModuleGL *ModuleGL
	// ModuleLPP is optional and is set if the segment is a LPP segment.
	ModuleLPP *ModuleLPP
	// HierarchicalConglomeration is optional and is set if the segment is a
	// conglomerated segment.
	HierarchicalConglomeration *ModuleConglo
	// RecursionComp is the compiled IOP of the recursed wizard.
	RecursionComp *wizard.CompiledIOP
	// Recursion is the wizard construction context of the recursed wizard.
	Recursion *recursion.Recursion
}

// SegmentProof stores a proof for a segment or for the conglomeration proof
type SegmentProof struct {
	RecursionWitness recursion.Witness
	ProofType        ProofType
	ModuleIndex      int
	SegmentIndex     int
	// LppCommitment is the commitment of the LPP witness. It is only populated
	// for a GL segment proof.
	LppCommitment field.Element

	// recursionRuntime is the runtime of the recursion proof. The reason for
	// this field is that we need to generate the input proof of the outer-proof,
	// without it
	recursionRuntime *wizard.ProverRuntime `serde:"omit"`
}

// CompileSegment applies all the compilation steps required to compile an LPP
// or a GL module of the protocol. The function accepts either a *[ModuleLPP]
// or a *[ModuleGL].
func CompileSegment(mod any, params CompilationParams) *RecursedSegmentCompilation {

	var (
		modIOP              *wizard.CompiledIOP
		res                 = &RecursedSegmentCompilation{}
		proofType           ProofType
		subscript           string
		initialCompilerSize = params.InitialCompilerSize
	)

	switch m := mod.(type) {
	case *ModuleGL:
		modIOP = m.Wiop
		res.ModuleGL = m
		subscript = string(m.DefinitionInput.ModuleName) + "-GL"
		proofType = proofTypeGL

	case *ModuleLPP:
		modIOP = m.Wiop
		res.ModuleLPP = m
		proofType = proofTypeLPP
		subscript = string(m.ModuleName()) + "-LPP"

	case *ModuleConglo:
		modIOP = m.Wiop
		res.HierarchicalConglomeration = m
		subscript = "hierarchical-conglomeration"
		proofType = proofTypeConglo
		initialCompilerSize = params.InitialCompilerSizeConglo

	default:
		utils.Panic("unexpected type: %T", mod)
	}

	sisInstance := ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

	wizard.ContinueCompilation(modIOP,
		// @alex: unsure if/why we need to compile with MiMC since it should be
		// done pre-bootstrapping.
		mimc.CompileMiMC,
		// The reason why 1 works is because it will work for all the GL modules
		// and because the LPP module do not have Plonk-in-wizards query.
		plonkinwizard.CompileWithMinimalRound(1),
		compiler.Arcane(
			compiler.WithTargetColSize(initialCompilerSize),
			// Some precompiles modules consists of only microscropic columns and
			// a Plonk-in-wizard query for a giant circuit. The small columns are
			// connected to the rest via a lookup but we need to ensure that the
			// columns will not be turned into "PROOF" columns and be put out of
			// the LPP commitment.
			//
			// @alex:
			// It's quite a magic number but the choice to use it nonetheless is
			// because to set the optimal value, we would need a feature where
			// the Arcane compiler detects the smallest committed column at round
			// 0 (or any round from a list) and sets its size as the StitcherMinSize
			// internally.
			//
			// For now, the current solution is fine and we can update the value from
			// time to time if not too frequent.
			compiler.WithStitcherMinSize(2),
			compiler.WithoutMpts(),
			// @alex: in principle, the value of 1 would be used only for the GL
			// prover but AFAIK, the GL modules never have inner-products to compile.
			compiler.WithInnerProductMinimalRound(1),
			// Uncomment to enable the debugging mode
			// compiler.WithDebugMode(subscript+"_initial"),
		),
		mpts.Compile(mpts.AddUnconstrainedColumns()),
	)

	initialWizardStats := logdata.GetWizardStats(modIOP)
	logrus.Infof("[Before first Vortex] module=%v numCellsCommitted=%v numCellsPrecomputed=%v numCellsProof=%v",
		subscript, initialWizardStats.NumCellsCommitted, initialWizardStats.NumCellsPrecomputed, initialWizardStats.NumCellsProof)

	if proofType == proofTypeConglo {

		wizard.ContinueCompilation(modIOP,
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		)
	} else {

		wizard.ContinueCompilation(modIOP,
			vortex.Compile(
				2,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
				vortex.AddMerkleRootToPublicInputs(lppMerkleRootPublicInput, []int{0}),
				vortex.WithOptionalSISHashingThreshold(64),
			),
		)
	}

	wizard.ContinueCompilation(modIOP,
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<15),
			compiler.WithStitcherMinSize(2),
			// Uncomment to enable the debugging mode
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_0")),
		),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(40),
			vortex.WithSISParams(&sisInstance),
			vortex.WithOptionalSISHashingThreshold(64),
		),
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<14),
			compiler.WithStitcherMinSize(2),
			// Uncomment to enable the debugging mode
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_1")),
		),
		// This extra step is to ensure the tightness of the final wizard by
		// adding an optional second layer of compilation when we have very
		// large inputs.
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(40),
			vortex.WithSISParams(&sisInstance),
			vortex.WithOptionalSISHashingThreshold(64),
		),
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<14),
			compiler.WithStitcherMinSize(2),
			compiler.WithoutMpts(),
			// Uncomment to enable the debugging mode
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_2")),
		),
		// This final step expectedly always generate always the same profile.
		// Most of the time, it is ineffective and could be skipped so there is
		// a pending optimization.
		logdata.Log("just-before-recursion"),
		mpts.Compile(mpts.WithNumColumnProfileOpt(params.ColumnProfileMPTS, params.ColumnProfileMPTSPrecomputed)),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(40),
			vortex.WithSISParams(&sisInstance),
			vortex.PremarkAsSelfRecursed(),
			vortex.AddPrecomputedMerkleRootToPublicInputs(VerifyingKeyPublicInput),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	var recCtx *recursion.Recursion
	// The loops below are there to filter the public inputs so that
	// Important: this must remain nil by default.
	var publicInputRestriction []string

	if proofType == proofTypeConglo {
		for _, pi := range modIOP.PublicInputs {
			if strings.HasPrefix(pi.Name, "conglomeration") {
				continue
			}
			publicInputRestriction = append(publicInputRestriction, pi.Name)
		}
	}

	if proofType == proofTypeLPP || proofType == proofTypeGL {
		for _, pi := range modIOP.PublicInputs {
			if strings.Contains(pi.Name, lppMerkleRootPublicInput) {
				continue
			}
			publicInputRestriction = append(publicInputRestriction, pi.Name)
		}
	}

	sortPublicInput(modIOP, publicInputRestriction)

	defineRecursion := func(build2 *wizard.Builder) {
		recCtx = recursion.DefineRecursionOf(
			build2.CompiledIOP,
			modIOP,
			recursion.Parameters{
				Name:                   "wizard-recursion",
				WithoutGkr:             true,
				MaxNumProof:            1,
				FixedNbRowPlonkCircuit: params.FixedNbRowPlonkCircuit,
				WithExternalHasherOpts: true,
				ExternalHasherNbRows:   params.FixedNbRowExternalHasher,
				FixedNbPublicInput:     params.FixedNbPublicInput,
				Subscript:              subscript,
				SkipRecursionPrefix:    true,
				RestrictPublicInputs:   publicInputRestriction,
			},
		)
	}

	recursedComp := wizard.Compile(defineRecursion,
		mimc.CompileMiMC,
		plonkinwizard.Compile,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<15),
			compiler.WithStitcherMinSize(2),
			// Uncomment to enable the debugging mode
			// compiler.WithDebugMode("post-recursion-arcane"),
		),
		logdata.Log("just-after-recursion-expanded"),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(40),
			vortex.WithSISParams(&sisInstance),
			vortex.AddPrecomputedMerkleRootToPublicInputs(VerifyingKey2PublicInput),
			vortex.WithOptionalSISHashingThreshold(64),
		),
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<14),
			compiler.WithStitcherMinSize(2),
			// Uncomment to enable the debugging mode
			// compiler.WithDebugMode("post-recursion-arcane-2"),
		),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(40),
			vortex.WithSISParams(&sisInstance),
			vortex.PremarkAsSelfRecursed(),
			vortex.WithOptionalSISHashingThreshold(64),
		),
	)

	res.Recursion = recCtx
	res.RecursionComp = recursedComp

	// It is necessary to add the extradata from the compiled IOP to the
	// recursed one otherwise, it will not be found.
	res.RecursionComp.ExtraData[VerifyingKeyPublicInput] = modIOP.ExtraData[VerifyingKeyPublicInput]

	return res
}

// ProveSegment runs the prover for a segment of the protocol
func (r *RecursedSegmentCompilation) ProveSegment(wit any) *SegmentProof {

	var (
		comp               *wizard.CompiledIOP
		proverStep         wizard.MainProverStep
		moduleName         any
		moduleIndex        int
		segmentModuleIndex int
		proofType          ProofType
	)

	switch m := wit.(type) {

	case *ModuleWitnessLPP:
		comp = r.ModuleLPP.Wiop
		proverStep = r.ModuleLPP.GetMainProverStep(m)
		moduleName = m.ModuleName
		moduleIndex = m.ModuleIndex
		segmentModuleIndex = m.SegmentModuleIndex
		proofType = proofTypeLPP

		if m.ModuleIndex != r.ModuleLPP.DefinitionInput.ModuleIndex {
			utils.Panic("m.ModuleIndex: %v != r.ModuleLPP.ModuleIndex: %v", m.ModuleIndex, r.ModuleLPP.DefinitionInput.ModuleIndex)
		}

		if m.ModuleName != r.ModuleLPP.DefinitionInput.ModuleName {
			utils.Panic("m.ModuleName: %v != r.ModuleLPP.ModuleName: %v", m.ModuleName, r.ModuleLPP.DefinitionInput.ModuleName)
		}

	case *ModuleWitnessGL:
		comp = r.ModuleGL.Wiop
		proverStep = r.ModuleGL.GetMainProverStep(m)
		moduleName = m.ModuleName
		moduleIndex = m.ModuleIndex
		segmentModuleIndex = m.SegmentModuleIndex
		proofType = proofTypeGL

		if m.ModuleIndex != r.ModuleGL.DefinitionInput.ModuleIndex {
			utils.Panic("m.ModuleIndex: %v != r.ModuleGL.ModuleIndex: %v", m.ModuleIndex, r.ModuleGL.DefinitionInput.ModuleIndex)
		}

		if m.ModuleName != r.ModuleGL.DefinitionInput.ModuleName {
			utils.Panic("m.ModuleName: %v != r.ModuleGL.ModuleName: %v", m.ModuleName, r.ModuleGL.DefinitionInput.ModuleName)
		}

	case *ModuleWitnessConglo:
		comp = r.HierarchicalConglomeration.Wiop
		proverStep = r.HierarchicalConglomeration.GetMainProverStep(m)
		moduleName = "hierachical-conglo"
		proofType = proofTypeConglo

	default:
		utils.Panic("unexpected type")
	}

	var (
		// In order to work, the recursion circuit needs to access the query params
		stoppingRound = recursion.VortexQueryRound(comp) + 1
		proverRun     *wizard.ProverRuntime
		initialTime   = profiling.TimeIt(func() {
			proverRun = wizard.RunProverUntilRound(comp, proverStep, stoppingRound)
		})
		initialProof    = proverRun.ExtractProof()
		initialProofErr = wizard.VerifyUntilRound(comp, initialProof, stoppingRound)
	)

	if initialProofErr != nil {
		panic(initialProofErr)
	}

	var (
		recStoppingRound = recursion.VortexQueryRound(r.RecursionComp) + 1
		recursionWit     = recursion.ExtractWitness(proverRun)
		run              *wizard.ProverRuntime
		recursionTime    = profiling.TimeIt(func() {
			run = wizard.RunProverUntilRound(
				r.RecursionComp,
				r.Recursion.GetMainProverStep([]recursion.Witness{recursionWit}, nil),
				recStoppingRound,
			)
		})
		finalProof    = run.ExtractProof()
		finalProofErr = wizard.VerifyUntilRound(r.RecursionComp, finalProof, recStoppingRound)
	)

	if finalProofErr != nil {
		panic(finalProofErr)
	}

	logrus.
		WithField("moduleName", moduleName).
		WithField("moduleIndex", moduleIndex).
		WithField("segmentModuleIndex", segmentModuleIndex).
		WithField("initial-time", initialTime).
		WithField("recursion-time", recursionTime).
		WithField("segment-type", fmt.Sprintf("%T", wit)).
		Infof("Ran prover segment")

	segmentProof := &SegmentProof{
		ModuleIndex:      moduleIndex,
		SegmentIndex:     segmentModuleIndex,
		ProofType:        proofType,
		RecursionWitness: recursion.ExtractWitness(run),
		recursionRuntime: run,
	}

	if proofType == proofTypeGL {
		segmentProof.LppCommitment = getLppCommitmentFromRuntime(proverRun)
	}

	return segmentProof
}

// GetVerifyingKeyPair returns the verifying keys of the compiled segment.
func (c *RecursedSegmentCompilation) GetVerifyingKeyPair() [2]field.Element {
	vk0, vk1 := getVerifyingKeyPair(c.RecursionComp)
	return [2]field.Element{vk0, vk1}
}

// GetOuterProofInput runs the final Vortex opening in the proof.
func (c *SegmentProof) GetOuterProofInput() wizard.Proof {
	return c.recursionRuntime.Resume().ExtractProof()
}

// ClearRuntime clears the ProverRuntime from the segment proof.
func (c *SegmentProof) ClearRuntime() *SegmentProof {
	c.recursionRuntime = nil
	return c
}

// sortPublicInput is small compiler sorting the public inputs by name.
// This helps ensuring that the order of public inputs is identical between all types
// of module.
//
// The function additionally takes a "restriction" input list which contains
// a list of "restricted" public inputs. They denote the public inputs that are
// actually "bubbled up" to the next instance. If the provided list is non-nil
// then, the public inputs are sorted based on whether they are in the list or
// not and then by alphabetical order.
func sortPublicInput(comp *wizard.CompiledIOP, restrictedList []string) {

	cmpName := func(a, b wizard.PublicInput) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	}

	if restrictedList == nil {
		slices.SortStableFunc(comp.PublicInputs, cmpName)
		return
	}

	var (
		included = []wizard.PublicInput{}
		excluded = []wizard.PublicInput{}
	)

	for _, pub := range comp.PublicInputs {
		// This is a list scan per iteration. So this is O(n**2) in total but
		// this is also not worth optimizing.
		if slices.Contains(restrictedList, pub.Name) {
			included = append(included, pub)
		} else {
			excluded = append(excluded, pub)
		}
	}

	slices.SortStableFunc(included, cmpName)
	slices.SortStableFunc(excluded, cmpName)
	comp.PublicInputs = append(included, excluded...)
}
