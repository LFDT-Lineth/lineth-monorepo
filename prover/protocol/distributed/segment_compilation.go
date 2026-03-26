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
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
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

	// InitialCompilerSizeOverride is map mapping module names to integers
	// that we can use to override the [InitialCompilerSize] parameter. This
	// is an optional parameter.
	InitialCompilerSizeOverride map[string]int

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
	// RecursionCompKoala is the compiled IOP of the recursed wizard that is
	// used to generate a conglomeratable proof.
	RecursionCompKoala *wizard.CompiledIOP
	// RecursionKoala is the wizard construction context of the recursed wizard
	// used toward being reconglomerated again.
	RecursionKoala *recursion.Recursion
	// RecursionBLS is the recursion context used to generate a BLS proof. It is
	// identical to the one used to generate a conglomeratable proof but it is
	// compiled differently. This is used for the final-proof that is wrapped
	// in the inner-proof.
	RecursionBLS *recursion.Recursion
	// RecursionCompBLS is the compiled IOP of the recursed wizard that is used
	// to generate a BLS proof. This is used for the final-proof that is wrapped
	// in the inner-proof.
	RecursionCompBLS *wizard.CompiledIOP
}

// SegmentProof stores a proof for a segment or for the conglomeration proof
type SegmentProof struct {
	RecursionWitness recursion.Witness
	ProofType        ProofType
	ModuleIndex      int
	SegmentIndex     int
	// LppCommitment is the commitment of the LPP witness. It is only populated
	// for a GL segment proof.
	LppCommitment field.Octuplet

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

	if params.InitialCompilerSizeOverride != nil {
		if override, ok := params.InitialCompilerSizeOverride[subscript]; ok {
			initialCompilerSize = override
		}
	}

	sisInstance := ringsis.StdParams //ringsis.Params{LogTwoBound: 16, LogTwoDegree: 9}

	wizard.ContinueCompilation(modIOP,
		// Compile Poseidon2 queries that may have been created during pre-compilation
		poseidon2.CompilePoseidon2,
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
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_initial")),
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
				false,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
				vortex.WithOptionalSISHashingThreshold(512),
			),
		)
	} else {

		wizard.ContinueCompilation(modIOP,
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(256),
				vortex.WithSISParams(&sisInstance),
				vortex.AddMerkleRootToPublicInputs(lppMerkleRootPublicInput, []int{0}),
				vortex.WithOptionalSISHashingThreshold(512),
			),
		)
	}

	wizard.ContinueCompilation(modIOP,
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		poseidon2.CompilePoseidon2,
		cleanup.CleanUp,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<15),
			compiler.WithStitcherMinSize(2),
			// Uncomment to enable the debugging mode
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_0")),
		),
		// This extra step is to ensure the tightness of the final wizard by
		// adding an optional second layer of compilation when we have very
		// large inputs.
		vortex.Compile(
			16,
			false,
			vortex.ForceNumOpenedColumns(64),
			vortex.WithSISParams(&sisInstance),
			vortex.WithOptionalSISHashingThreshold(512),
		),
		selfrecursion.SelfRecurse,
		poseidon2.CompilePoseidon2,
		cleanup.CleanUp,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<14),
			compiler.WithStitcherMinSize(2),
			compiler.WithoutMpts(),
			compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"_2")),
		),
		// This final step expectedly always generate always the same profile.
		// Most of the time, it is ineffective and could be skipped so there is
		// a pending optimization.
		logdata.Log("just-before-recursion"),
		mpts.Compile(mpts.WithNumColumnProfileOpt(params.ColumnProfileMPTS, params.ColumnProfileMPTSPrecomputed)),
		vortex.Compile(
			16,
			false,
			vortex.ForceNumOpenedColumns(64),
			vortex.WithSISParams(&sisInstance),
			vortex.PremarkAsSelfRecursed(),
			vortex.AddPrecomputedMerkleRootToPublicInputs(VerifyingKeyPublicInput),
			vortex.WithOptionalSISHashingThreshold(512),
		),
		// dummy.CompileAtProverLvl(dummy.WithMsg("Post-vortex:just-before-recursion")),
	)

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

	makeRecursion := func(withBLS bool) (*wizard.CompiledIOP, *recursion.Recursion) {

		var recCtx *recursion.Recursion

		defineRecursion := func(build2 *wizard.Builder) {
			recCtx = recursion.DefineRecursionOf(
				build2.CompiledIOP,
				modIOP,
				recursion.Parameters{
					Name:                   "wizard-recursion",
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
			poseidon2.CompilePoseidon2,
			plonkinwizard.Compile,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<19),
				compiler.WithStitcherMinSize(2),
				compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"/post-recursion.initial/")),
			),
			logdata.Log("just-after-recursion-expanded"),
			vortex.Compile(
				2,
				false,
				vortex.ForceNumOpenedColumns(64),
				vortex.WithSISParams(&sisInstance),
				vortex.AddPrecomputedMerkleRootToPublicInputs(VerifyingKey2PublicInput),
				vortex.WithOptionalSISHashingThreshold(512),
			),
			selfrecursion.SelfRecurse,
			poseidon2.CompilePoseidon2,
			cleanup.CleanUp,
			compiler.Arcane(
				compiler.WithTargetColSize(1<<14),
				compiler.WithStitcherMinSize(2),
				compiler.MaybeWith(params.FullDebugMode, compiler.WithDebugMode(subscript+"/recursion.arcane-2/")),
			),
		)

		if withBLS {
			recursedComp = wizard.ContinueCompilation(recursedComp,
				vortex.Compile(16, true, vortex.ForceNumOpenedColumns(64)),
			)
		} else {
			recursedComp = wizard.ContinueCompilation(recursedComp,
				vortex.Compile(
					16,
					false,
					vortex.ForceNumOpenedColumns(64),
					vortex.WithSISParams(&sisInstance),
					vortex.PremarkAsSelfRecursed(),
					vortex.WithOptionalSISHashingThreshold(512),
				),
			)
		}

		return recursedComp, recCtx
	}

	res.RecursionCompKoala, res.RecursionKoala = makeRecursion(false)
	// It is necessary to add the extradata from the compiled IOP to the
	// recursed one otherwise, it will not be found by the conglomerator.
	res.RecursionCompKoala.ExtraData[VerifyingKeyPublicInput] = modIOP.ExtraData[VerifyingKeyPublicInput]

	if proofType == proofTypeConglo {
		res.RecursionCompBLS, res.RecursionBLS = makeRecursion(true)
		// It is necessary to add the extradata from the compiled IOP to the
		// recursed one otherwise, it will not be found by the conglomerator.
		// ?? Is that even needed ??
		res.RecursionCompBLS.ExtraData[VerifyingKeyPublicInput] = modIOP.ExtraData[VerifyingKeyPublicInput]
	}

	return res
}

// ProveSegmentKoala runs the prover for a segment of the protocol
func (r *RecursedSegmentCompilation) ProveSegmentKoala(wit any) *SegmentProof {
	return r.proveSegment(wit, false)
}

// ProveSegmentBLS runs the prover for a segment of the protocol with BLS. This
// is used for the outer-proof.
func (r *RecursedSegmentCompilation) ProveSegmentBLS(wit any) *SegmentProof {
	return r.proveSegment(wit, true)
}

func (r *RecursedSegmentCompilation) proveSegment(wit any, forBLS bool) *SegmentProof {

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
		moduleName = "hierarchical-conglo"
		proofType = proofTypeConglo

	default:
		utils.Panic("unexpected type")
	}

	var (
		// In order to work, the recursion circuit needs to access the query params
		stoppingRound = recursion.VortexQueryRound(comp) + 1
		proverRun     *wizard.ProverRuntime
		initialTime   = profiling.TimeIt(func() {
			proverRun = wizard.RunProverUntilRound(comp, proverStep, stoppingRound, false)
		})
		initialProof    = proverRun.ExtractProof()
		initialProofErr = wizard.VerifyUntilRound(comp, initialProof, stoppingRound, false)
	)

	if initialProofErr != nil {
		panic(initialProofErr)
	}

	var (
		recursionComp = r.RecursionCompKoala
		recursionCtx  = r.RecursionKoala
	)

	if forBLS {
		recursionComp = r.RecursionCompBLS
		recursionCtx = r.RecursionBLS
	}

	var (
		recStoppingRound = recursion.VortexQueryRound(recursionComp) + 1
		recursionWit     = recursion.ExtractWitness(proverRun)
		run              *wizard.ProverRuntime
		recursionTime    = profiling.TimeIt(func() {
			run = wizard.RunProverUntilRound(
				recursionComp,
				recursionCtx.GetMainProverStep([]recursion.Witness{recursionWit}, nil),
				recStoppingRound,
				forBLS,
			)
		})
		finalProof    = run.ExtractProof()
		finalProofErr = wizard.VerifyUntilRound(recursionComp, finalProof, recStoppingRound, forBLS)
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
		recursionRuntime: run,
	}

	if !forBLS {
		// For BLS workload, the function ExtractWitness does not work and  -
		// importantly is not well-defined import.
		segmentProof.RecursionWitness = recursion.ExtractWitness(run)
	}

	if proofType == proofTypeGL {
		segmentProof.LppCommitment = getLppCommitmentFromRuntime(proverRun)
	}

	return segmentProof
}

// GetVerifyingKeyPair returns the verifying keys of the compiled segment.
func (c *RecursedSegmentCompilation) GetVerifyingKeyPair() [2]field.Octuplet {
	vk0, vk1 := getVerifyingKeyPair(c.RecursionCompKoala)
	return [2]field.Octuplet{vk0, vk1}
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
