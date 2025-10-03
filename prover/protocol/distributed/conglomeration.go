package distributed

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	cmimc "github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/sirupsen/logrus"
)

// The prerecursion prefix is a prefix to apply to the name of the public
// inputs to be able to access them in the conglomerated wizard-IOP.
const preRecursionPrefix = "wizard-recursion-0."

// ConglomeratorCompilation hold the compilation context of the conglomeration
// proof. It stores pointers to the type of proof it can conglomerate and
// pointers of the resulting compiled IOP object.
type ConglomeratorCompilation struct {
	// MaxNbProofs is the maximum number of proofs that can be conglomerated
	// by the conglomeration proof at once.
	MaxNbProofs int

	// ModuleGLIops and ModuleLPPIops lists the wizard whose proof are supported by the current
	// instance of the conglomerator.
	ModuleGLIops  []*wizard.CompiledIOP `serde:"omit"`
	ModuleLPPIops []*wizard.CompiledIOP `serde:"omit"`

	// DefaultIops is the wizard IOP used for filling
	DefaultIops *RecursedSegmentCompilation

	// DefaultWitness is the assignment of the default IOP.
	DefaultWitness recursion.Witness

	// Wiop is the compiled IOP of the conglomeration wizard.
	Wiop *wizard.CompiledIOP
	// Recursion is the recursion context used to compile the conglomeration
	// proof.
	Recursion *recursion.Recursion

	// HolisticLookupMappedLPPVK is a pair of column corresponding such that
	// row "i" is equal to the LPP vk of the GL module of the i-th proof.
	HolisticLookupMappedLPPVK [2]ifaces.Column

	// HolisticLookupMappedLPPPostion is complementary to [HolisticLookupMappedLPPVK]
	// and indicates for the corresponding GL verifying key, which column of the
	// corresponding LPP module to look for when doing the LPP commitment consistency
	// check. The column takes values in [0, lppGroupingArity) and is constrained by
	// the same lookup as [HolisticLookupMappedLPPVK].
	HolisticLookupMappedLPPPostion ifaces.Column

	// PrecomputedLPPVks is a pair of precomputed column listing the whitelisted
	// LPP columns.
	PrecomputedLPPVks [2]ifaces.Column

	// PrecomputedGLVks is a pair of precomputed column listing the whitelisted
	// GL columns. Each entry i corresponds to the GK vks that can be mapped to
	// the i-th LPP module of the same row of [PrecomputedLPPVks].
	PrecomputedGLVks [lppGroupingArity][2]ifaces.Column

	// IsGL is a column constructing by agglomerating accessors the IsGL public
	// input of every segment.
	IsGL ifaces.Column

	// VerifyingKeyColumns is a pair of column constructing by agglomerating the
	// public inputs corresponding to the verifying key for every segment.
	VerifyingKeyColumns [2]ifaces.Column
}

// ConglomerationHolisticCheck is a [wizard.VerifierAction] checking that all
// the public inputs of the subproofs are the right ones.
type ConglomerateHolisticCheck struct {
	ConglomeratorCompilation
}

// ConglomerationAssignHolisticCheckColumn is a [wizard.ProverAction] responsible
// for assigning the [HolisticLookupMappedLPPVK] columns.
type ConglomerationAssignHolisticCheckColumn struct {
	ConglomeratorCompilation
}

// conglomerate constructs and returns a new ConglomeratorCompilation object.
// The Wiop of the returned object is compiled with iterative layers of
// self-recursion.
func conglomerate(maxNbProofs int, moduleGLs, moduleLpps []*RecursedSegmentCompilation, moduleDefault *RecursedSegmentCompilation) *ConglomeratorCompilation {

	cong := &ConglomeratorCompilation{
		MaxNbProofs: maxNbProofs,
		DefaultIops: moduleDefault,
	}

	for i := range moduleGLs {
		cong.ModuleGLIops = append(cong.ModuleGLIops, moduleGLs[i].RecursionComp)
	}

	for i := range moduleLpps {
		cong.ModuleLPPIops = append(cong.ModuleLPPIops, moduleLpps[i].RecursionComp)
	}

	sisInstance := ringsis.Params{LogTwoBound: 16, LogTwoDegree: 6}

	cong.Wiop = wizard.Compile(
		func(build *wizard.Builder) {
			cong.Compile(build.CompiledIOP)
		},
		mimc.CompileMiMC,
		plonkinwizard.Compile,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<17),
		),
		logdata.Log("before first vortex"),
		vortex.Compile(
			2,
			vortex.ForceNumOpenedColumns(256),
			vortex.WithSISParams(&sisInstance),
			vortex.WithOptionalSISHashingThreshold(64),
		),
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<15),
		),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(32),
			vortex.WithSISParams(&sisInstance),
			vortex.WithOptionalSISHashingThreshold(64),
		),
		selfrecursion.SelfRecurse,
		cleanup.CleanUp,
		mimc.CompileMiMC,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<13),
		),
		vortex.Compile(
			8,
			vortex.ForceNumOpenedColumns(32),
			vortex.WithOptionalSISHashingThreshold(1<<20),
		),
	)

	return cong
}

// Compile compiles the conglomeration proof. The function first checks if the public
// inputs are compatible and then compiles the conglomeration proof.
func (c *ConglomeratorCompilation) Compile(comp *wizard.CompiledIOP) {

	var (
		wiops = slices.Concat(c.ModuleGLIops, c.ModuleLPPIops, []*wizard.CompiledIOP{c.DefaultIops.RecursionComp})
		w0    = wiops[0]
	)

	for i := 1; i < len(wiops); i++ {
		diff1, diff2 := cmpWizardIOP(w0, wiops[i])
		if len(diff1) > 0 || len(diff2) > 0 {

			for i, modIOP := range wiops {
				dumpWizardIOP(modIOP, fmt.Sprintf("conglomeration-debug/iop-%d.csv", i))
			}

			utils.Panic("incompatible IOPs i=%v\n\t+++=%v\n\t---=%v", i, diff1, diff2)
		}
	}

	defaultRun := c.DefaultIops.ProveSegment(nil)
	c.DefaultWitness = recursion.ExtractWitness(defaultRun)

	c.Recursion = recursion.DefineRecursionOf(comp, w0, recursion.Parameters{
		Name:                   "conglomeration",
		WithoutGkr:             true,
		MaxNumProof:            c.MaxNbProofs,
		WithExternalHasherOpts: true,
	})

	c.Wiop = comp
	c.PrecomputedLPPVks, c.PrecomputedGLVks = c.precomputeToTheWhiteListVKeys()
	c.declareLookups()

	comp.RegisterVerifierAction(0, &ConglomerateHolisticCheck{ConglomeratorCompilation: *c})
	comp.RegisterProverAction(0, &ConglomerationAssignHolisticCheckColumn{ConglomeratorCompilation: *c})
}

// Run implements the [wizard.VerifierAction] interface.
func (c *ConglomerateHolisticCheck) Run(run wizard.Runtime) error {

	var (
		allGrandProduct           = field.NewElement(1)
		allLogDerivativeSum       = field.Element{}
		allHornerSum              = field.Element{}
		prevGlobalSent            = field.Element{}
		prevHornerN1Hash          = field.Element{}
		usedSharedRandomness      = field.Element{}
		usedSharedRandomnessFound bool
		collectedLPPCommitments   = make([]field.Element, 0)
		mainErr                   error
	)

	type proofPublicInput struct {
		LPPCommitment    field.Element
		SharedRandomness field.Element
		VerifyingKey     field.Element
		VerifyingKey2    field.Element
		LogDerivativeSum field.Element
		GrandProduct     field.Element
		HornerSum        field.Element
		HornerN0Hash     field.Element
		HornerN1Hash     field.Element
		GlobalReceived   field.Element
		GlobalSent       field.Element
		IsFirst          bool
		IsLast           bool
		IsLPP            bool
		IsGL             bool
		SameVkAsPrev     bool
		SameVkAsNext     bool
	}

	allPis := []proofPublicInput{}

	for i := 0; i < c.MaxNbProofs; i++ {

		pi := proofPublicInput{
			LPPCommitment:    c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, 0), i),
			SharedRandomness: c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+InitialRandomnessPublicInput, i),
			VerifyingKey:     c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+verifyingKeyPublicInput, i),
			VerifyingKey2:    c.Recursion.GetPublicInputOfInstance(run, verifyingKey2PublicInput, i),
			LogDerivativeSum: c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+LogDerivativeSumPublicInput, i),
			GrandProduct:     c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+GrandProductPublicInput, i),
			HornerSum:        c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+HornerPublicInput, i),
			HornerN0Hash:     c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+HornerN0HashPublicInput, i),
			HornerN1Hash:     c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+HornerN1HashPublicInput, i),
			GlobalReceived:   c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+GlobalReceiverPublicInput, i),
			GlobalSent:       c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+GlobalSenderPublicInput, i),
			IsFirst:          c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+IsFirstPublicInput, i) == field.One(),
			IsLast:           c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+IsLastPublicInput, i) == field.One(),
			IsLPP:            c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+IsLppPublicInput, i) == field.One(),
			IsGL:             c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+IsGlPublicInput, i) == field.One(),
		}

		allPis = append(allPis, pi)

		var (
			sameVerifyingKeyAsPrev, sameVerifyingKeyAsNext bool
		)

		if i > 0 {
			prevVerifyingKey := c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+verifyingKeyPublicInput, i-1)
			prevVerifyingKey2 := c.Recursion.GetPublicInputOfInstance(run, verifyingKey2PublicInput, i-1)
			sameVerifyingKeyAsPrev = pi.VerifyingKey == prevVerifyingKey && pi.VerifyingKey2 == prevVerifyingKey2
		}

		if i < c.MaxNbProofs-1 {
			nextVerifyingKey := c.Recursion.GetPublicInputOfInstance(run, preRecursionPrefix+verifyingKeyPublicInput, i+1)
			nextVerifyingKey2 := c.Recursion.GetPublicInputOfInstance(run, verifyingKey2PublicInput, i+1)
			sameVerifyingKeyAsNext = pi.VerifyingKey == nextVerifyingKey && pi.VerifyingKey2 == nextVerifyingKey2
		}

		if pi.IsLPP && sameVerifyingKeyAsPrev && pi.HornerN0Hash != prevHornerN1Hash {
			mainErr = errors.Join(mainErr, errors.New("horner-n0-hash mismatch"))
		}

		if pi.IsGL && !sameVerifyingKeyAsPrev != pi.IsFirst {
			mainErr = errors.Join(mainErr, errors.New("isFirst is inconsistent with the verifying keys"))
		}

		if pi.IsGL && !sameVerifyingKeyAsNext != pi.IsLast {
			mainErr = errors.Join(mainErr, errors.New("isLast is inconsistent with the verifying keys"))
		}

		if pi.IsGL && sameVerifyingKeyAsPrev && pi.GlobalReceived != prevGlobalSent {
			mainErr = errors.Join(mainErr, errors.New("global sent and receive don't match"))
		}

		if pi.IsLPP && !usedSharedRandomnessFound {
			usedSharedRandomness = pi.SharedRandomness
			usedSharedRandomnessFound = true
		}

		if pi.IsGL && usedSharedRandomnessFound {
			if usedSharedRandomness != pi.SharedRandomness {
				mainErr = errors.Join(mainErr, fmt.Errorf("shared randomness mismatch between different LPP segments: %v and %v", usedSharedRandomness.String(), pi.SharedRandomness.String()))
			}
		}

		if pi.IsGL {
			collectedLPPCommitments = append(collectedLPPCommitments, pi.LPPCommitment)
		}

		prevHornerN1Hash = pi.HornerN1Hash
		prevGlobalSent = pi.GlobalSent

		if pi.IsLPP {
			allGrandProduct.Mul(&allGrandProduct, &pi.GrandProduct)
			allHornerSum.Add(&allHornerSum, &pi.HornerSum)
			allLogDerivativeSum.Add(&allLogDerivativeSum, &pi.LogDerivativeSum)
		}
	}

	computedSharedRandomness := GetSharedRandomness(collectedLPPCommitments)
	if computedSharedRandomness != usedSharedRandomness {
		mainErr = errors.Join(mainErr, fmt.Errorf("shared randomness mismatch, between the one used in LPP and the hash of the LPP commitments computed by the GL: %v and %v", usedSharedRandomness.String(), computedSharedRandomness.String()))
	}

	if !allGrandProduct.IsOne() {
		mainErr = errors.Join(mainErr, fmt.Errorf("grand product is not one: %v", allGrandProduct.String()))
	}

	if !allHornerSum.IsZero() {
		mainErr = errors.Join(mainErr, fmt.Errorf("horner sum is not zero: %v", allHornerSum.String()))
	}

	if !allLogDerivativeSum.IsZero() {
		mainErr = errors.Join(mainErr, fmt.Errorf("log derivative sum is not zero: %v", allLogDerivativeSum.String()))
	}

	if mainErr != nil {
		fmt.Printf("conglomeration failed: err=%v pis=%++v\n", mainErr, allPis)
	}

	return mainErr
}

// RunGnark is as [Run] but in a gnark circuit
func (c *ConglomeratorCompilation) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	var (
		allGrandProduct           = frontend.Variable(1)
		allLogDerivativeSum       = frontend.Variable(0)
		allHornerSum              = frontend.Variable(0)
		prevGlobalSent            = frontend.Variable(0)
		prevHornerN1Hash          = frontend.Variable(0)
		usedSharedRandomness      = frontend.Variable(0)
		usedSharedRandomnessFound = frontend.Variable(0)
		accumulativeLppHash       = frontend.Variable(0)
	)

	for i := 0; i < c.MaxNbProofs; i++ {

		var (
			lppCommitment    = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, 0), i)
			sharedRandomness = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+InitialRandomnessPublicInput, i)
			verifyingKey     = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+verifyingKeyPublicInput, i)
			verifyingKey2    = c.Recursion.GetPublicInputOfInstanceGnark(api, run, verifyingKey2PublicInput, i)
			logDerivativeSum = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+LogDerivativeSumPublicInput, i)
			grandProduct     = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+GrandProductPublicInput, i)
			hornerSum        = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+HornerPublicInput, i)
			hornerN0Hash     = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+HornerN0HashPublicInput, i)
			hornerN1Hash     = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+HornerN1HashPublicInput, i)
			globalReceived   = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+GlobalReceiverPublicInput, i)
			globalSent       = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+GlobalSenderPublicInput, i)
			isFirst          = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+IsFirstPublicInput, i)
			isLast           = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+IsLastPublicInput, i)
			isLPP            = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+IsLppPublicInput, i)
			isGL             = c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+IsGlPublicInput, i)

			sameVerifyingKeyAsPrev, sameVerifyingKeyAsNext = frontend.Variable(0), frontend.Variable(0)
		)

		if i > 0 {
			prevVerifyingKey := c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+verifyingKeyPublicInput, i-1)
			prevVerifyingKey2 := c.Recursion.GetPublicInputOfInstanceGnark(api, run, verifyingKey2PublicInput, i-1)

			sameVerifyingKeyAsPrev = api.Mul(
				api.IsZero(api.Sub(prevVerifyingKey, verifyingKey)),
				api.IsZero(api.Sub(prevVerifyingKey2, verifyingKey2)),
			)
		}

		if i < c.MaxNbProofs-1 {
			nextVerifyingKey := c.Recursion.GetPublicInputOfInstanceGnark(api, run, preRecursionPrefix+verifyingKeyPublicInput, i+1)
			nextVerifyingKey2 := c.Recursion.GetPublicInputOfInstanceGnark(api, run, verifyingKey2PublicInput, i+1)

			sameVerifyingKeyAsNext = api.Mul(
				api.IsZero(api.Sub(nextVerifyingKey, verifyingKey)),
				api.IsZero(api.Sub(nextVerifyingKey2, verifyingKey2)),
			)
		}

		// This emulates the following check:
		//
		// if isLPP.IsOne() && sameVerifyingKeyAsPrev && hornerN0Hash != prevHornerN1Hash {
		// 	mainErr = errors.Join(mainErr, errors.New("horner-n0-hash mismatch"))
		// }
		api.AssertIsEqual(0,
			api.Mul(
				isLPP,
				sameVerifyingKeyAsPrev,
				api.Sub(hornerN0Hash, prevHornerN1Hash),
			),
		)

		// This emulates the following check:
		//
		// if isGL.IsOne() && !sameVerifyingKeyAsPrev != isFirst.IsOne() {
		// 	mainErr = errors.Join(mainErr, errors.New("isFirst is inconsistent with the verifying keys"))
		// }
		api.AssertIsEqual(0,
			api.Mul(
				isGL,
				// these are already ensured to be boolean, that's why the check
				// works. sameVerifyingKeyAsPrev is boolean as the product of two booleans
				// and isFirst is enforced to be boolean thanks to being a public input
				// of a whitelisted circuit which enforces it to be a boolean.
				api.Sub(1, sameVerifyingKeyAsPrev, isFirst),
			),
		)

		// This emulates the following check:
		//
		// if isGL.IsOne() && !sameVerifyingKeyAsNext != isLast.IsOne() {
		// 	mainErr = errors.Join(mainErr, errors.New("isLast is inconsistent with the verifying keys"))
		// }
		api.AssertIsEqual(0,
			api.Mul(
				isGL,
				// these are already ensured to be boolean, that's why the check
				// works. sameVerifyingKeyAsNext is boolean as the product of two booleans
				// and isLast is enforced to be boolean thanks to being a public input
				// of a whitelisted circuit which enforces it to be a boolean.
				api.Sub(1, sameVerifyingKeyAsNext, isLast),
			),
		)

		// This emulates the following check:
		//
		// if isGL.IsOne() && sameVerifyingKeyAsPrev && globalReceived != prevGlobalSent {
		// 	mainErr = errors.Join(mainErr, errors.New("global sent and receive don't match"))
		// }
		api.AssertIsEqual(0,
			api.Mul(
				isGL,
				sameVerifyingKeyAsPrev,
				api.Sub(globalReceived, prevGlobalSent),
			),
		)

		// This emulates the following conditional updates:
		//
		// if isLPP.IsOne() && !usedSharedRandomnessFound {
		// 	usedSharedRandomness = sharedRandomness
		// 	usedSharedRandomnessFound = true
		// }
		isFirstUseOfRandomness := api.Mul(isLPP, api.Sub(1, usedSharedRandomnessFound))
		usedSharedRandomness = api.Select(isFirstUseOfRandomness, sharedRandomness, usedSharedRandomness)
		usedSharedRandomnessFound = api.Add(usedSharedRandomnessFound, isFirstUseOfRandomness)

		// This emulates the following check:
		//
		// if isLPP.IsOne() && usedSharedRandomnessFound {
		// 	if usedSharedRandomness != sharedRandomness {
		// 		mainErr = errors.Join(mainErr, fmt.Errorf("shared randomness mismatch between different LPP segments: %v and %v", usedSharedRandomness.String(), sharedRandomness.String()))
		// 	}
		// }
		api.AssertIsEqual(0,
			api.Mul(
				isLPP,
				usedSharedRandomnessFound,
				api.Sub(usedSharedRandomness, sharedRandomness),
			),
		)

		// This emulates the following conditional append. As in a gnark circuit it is
		// complicated to do a conditional append, we instead do a conditional hasher
		// update.
		//
		// if isGL.IsOne() {
		// 	collectedLPPCommitments = append(collectedLPPCommitments, lppCommitment)
		// }
		newAccIfUpdateNeeded := cmimc.GnarkBlockCompression(api, accumulativeLppHash, lppCommitment)
		accumulativeLppHash = api.Select(isGL, newAccIfUpdateNeeded, accumulativeLppHash)

		prevHornerN1Hash = hornerN1Hash
		prevGlobalSent = globalSent

		// Note: although the native version of the update conditions the update to
		// be done only if we are scanning through an LPP instance, the other circuits
		// are guaranteed to give the neutral element for the update. So, we can simplify
		// the circuit a little bit.
		allGrandProduct = api.Mul(allGrandProduct, grandProduct)
		allHornerSum = api.Add(allHornerSum, hornerSum)
		allLogDerivativeSum = api.Add(allLogDerivativeSum, logDerivativeSum)
	}

	api.AssertIsEqual(accumulativeLppHash, usedSharedRandomness)
	api.AssertIsEqual(allGrandProduct, 1)
	api.AssertIsEqual(allHornerSum, 0)
	api.AssertIsEqual(allLogDerivativeSum, 0)
}

// BubbleUpPublicInput bubbles up the public inputs of a given name.
func (c *ConglomeratorCompilation) BubbleUpPublicInput(name string) wizard.PublicInput {

	pubInputSum := symbolic.NewConstant(0)
	for i := 0; i < c.MaxNbProofs; i++ {
		subPubInput := c.Recursion.GetPublicInputAccessorOfInstance(c.Wiop, preRecursionPrefix+name, i)
		isFirst := c.Recursion.GetPublicInputAccessorOfInstance(c.Wiop, preRecursionPrefix+IsFirstPublicInput, i)
		pubInputSum = symbolic.Add(pubInputSum, symbolic.Mul(isFirst, subPubInput))
	}

	return c.Wiop.InsertPublicInput(name, accessors.NewFromExpression(pubInputSum, name+"_SUMMATION_ACCESSOR"))
}

// Prove is the main entry point for the prover. It takes a compiled IOP and
// returns a proof.
func (c *ConglomeratorCompilation) Prove(moduleGlProofs, moduleLppProofs []recursion.Witness) wizard.Proof {

	var proof wizard.Proof
	recursionTime := profiling.TimeIt(func() {
		proof = wizard.Prove(
			c.Wiop,
			c.Recursion.GetMainProverStep(slices.Concat(moduleGlProofs, moduleLppProofs), &c.DefaultWitness),
		)
	})

	logrus.
		WithField("time", recursionTime).
		WithField("nb_lpp_proofs", len(moduleLppProofs)).
		WithField("nb_gl_proofs", len(moduleGlProofs)).
		Info("recursion done")

	return proof
}

// Run implements the [wizard.ProverAction] interface.
func (cong *ConglomerationAssignHolisticCheckColumn) Run(run *wizard.ProverRuntime) {

	var (
		vkMapping          = map[[2]field.Element][2]field.Element{}
		lppPositionMapping = map[[2]field.Element]int{}
		assignment         = [2][]field.Element{}
		isGL               = cong.IsGL.GetColAssignment(run).IntoRegVecSaveAlloc()
		verifyingKey       = [2][]field.Element{
			cong.VerifyingKeyColumns[0].GetColAssignment(run).IntoRegVecSaveAlloc(),
			cong.VerifyingKeyColumns[1].GetColAssignment(run).IntoRegVecSaveAlloc(),
		}
		numPrecomputedRow = cong.PrecomputedGLVks[0][0].Size()
	)

	for i := 0; i < numPrecomputedRow; i++ {

		vk0LPP := cong.PrecomputedLPPVks[0].GetColAssignmentAt(run, i)
		vk1LPP := cong.PrecomputedLPPVks[1].GetColAssignmentAt(run, i)

		for j := 0; j < lppGroupingArity; j++ {

			vk0GL := cong.PrecomputedGLVks[j][0].GetColAssignmentAt(run, i)
			vk1GL := cong.PrecomputedGLVks[j][1].GetColAssignmentAt(run, i)
			vkMapping[[2]field.Element{vk0GL, vk1GL}] = [2]field.Element{vk0LPP, vk1LPP}
			lppPositionMapping[[2]field.Element{vk0GL, vk1GL}] = j
		}
	}

	assignment[0] = make([]field.Element, len(verifyingKey[0]))
	assignment[1] = make([]field.Element, len(verifyingKey[1]))
	lppColumnIndex := make([]field.Element, len(verifyingKey[0]))

	for i := 0; i < len(verifyingKey[0]); i++ {

		if !isGL[i].IsOne() {
			continue
		}

		var (
			glKey            = [2]field.Element{verifyingKey[0][i], verifyingKey[1][i]}
			mappedLPP, found = vkMapping[glKey]
			mappedPosition   = lppPositionMapping[glKey]
		)

		if !found {

			// Before panicking we unfold the list of the available vkeys from
			// the mapping to try to make it easier to debug.

			var (
				glKeysFmtted = []string{}
				glKeys       = utils.SortedKeysOf(vkMapping, func(a, b [2]field.Element) bool {
					if a[0].Cmp(&b[0]) != 0 {
						return a[0].Cmp(&b[0]) < 0
					}
					return a[1].Cmp(&b[1]) < 0
				})
			)

			for i := range glKeys {
				glKeysFmtted = append(glKeysFmtted, fmt.Sprintf("[%v %v]", glKeys[i][0].Text(16), glKeys[i][1].Text(16)))
			}

			utils.Panic("verifying key not found missing=[%v %v], row=%v availble-keys=%v",
				verifyingKey[0][i].Text(16), verifyingKey[1][i].Text(16),
				i,
				glKeysFmtted,
			)
		}

		assignment[0][i] = mappedLPP[0]
		assignment[1][i] = mappedLPP[1]
		lppColumnIndex[i] = field.NewElement(uint64(mappedPosition))

		continue
	}

	colToAssign := cong.HolisticLookupMappedLPPVK
	posToAssign := cong.HolisticLookupMappedLPPPostion

	run.AssignColumn(colToAssign[0].GetColID(), smartvectors.RightZeroPadded(assignment[0], colToAssign[0].Size()))
	run.AssignColumn(colToAssign[1].GetColID(), smartvectors.RightZeroPadded(assignment[1], colToAssign[1].Size()))
	run.AssignColumn(posToAssign.GetColID(), smartvectors.RightZeroPadded(lppColumnIndex, posToAssign.Size()))
}

// cmpWizardIOP compares two compiled IOPs. The function is here to help ensuring
// that all the conglomerated wizard IOPs have the same structure and help
// figuring out inconsistencies if there are.
func cmpWizardIOP(c1, c2 *wizard.CompiledIOP) (diff1, diff2 []string) {

	var (
		stringB1 = &strings.Builder{}
		stringB2 = &strings.Builder{}
	)

	logdata.GenCSV(stringB1, logdata.IncludeAllFilter)(c1)
	logdata.GenCSV(stringB2, logdata.IncludeAllFilter)(c2)

	var (
		c1Formatted = strings.Split(stringB1.String(), "\n")
		c2Formatted = strings.Split(stringB2.String(), "\n")
	)

	diff1, diff2 = utils.SetDiff(c1Formatted, c2Formatted)
	lessFunc := func(a, b string) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		} else {
			return 0
		}
	}

	slices.SortFunc(diff1, lessFunc)
	slices.SortFunc(diff2, lessFunc)

	return diff1, diff2
}

// dumpWizardIOP dumps a compiled IOP to a file.
func dumpWizardIOP(c *wizard.CompiledIOP, name string) {
	logdata.GenCSV(files.MustOverwrite(name), logdata.IncludeAllFilter)(c)
}

// precomputeToTheWhiteListVKeys declares the precomputed columns needed to
// perform the white-list-check of the verifying keys.
func (cong *ConglomeratorCompilation) precomputeToTheWhiteListVKeys() ([2]ifaces.Column, [lppGroupingArity][2]ifaces.Column) {

	var (
		comp = cong.Wiop

		// vkMappingWhiteList is a list of pairs representing the correspondance
		// table between the VKs for the GL and the LPP modules. Namely, it
		// represents the list of the GL modules that can be linked to an LPP
		// module.
		vkMappingPaddedSize   = utils.NextPowerOfTwo(len(cong.ModuleLPPIops))
		vkMappingWhiteListLPP = [2][]field.Element{}
		vkMappingWhiteListGL  = [lppGroupingArity][2][]field.Element{}

		// The columns for the vkMapping
		vkMappingColumnsLPP = [2]ifaces.Column{}
		vkMappingColumnsGL  = [lppGroupingArity][2]ifaces.Column{}
	)

	//
	// Collect the content of the lookup tables
	//

	for i := range cong.ModuleLPPIops {

		vk0, vk1 := getVerifyingKeyPair(cong.ModuleLPPIops[i])
		vkMappingWhiteListLPP[0] = append(vkMappingWhiteListLPP[0], vk0)
		vkMappingWhiteListLPP[1] = append(vkMappingWhiteListLPP[1], vk1)

		for j := 0; j < lppGroupingArity; j++ {

			vk0, vk1 := field.Zero(), field.Zero()
			if i*lppGroupingArity+j < len(cong.ModuleGLIops) {
				vk0, vk1 = getVerifyingKeyPair(cong.ModuleGLIops[i*lppGroupingArity+j])
			}

			vkMappingWhiteListGL[j][0] = append(vkMappingWhiteListGL[j][0], vk0)
			vkMappingWhiteListGL[j][1] = append(vkMappingWhiteListGL[j][1], vk1)
		}
	}

	//
	// Declare the whiteListed VKs as precomputed columns representing the correspondance
	// table between the VKs for the GL and the LPP modules.
	//

	vkMappingColumnsLPP[0] = comp.InsertPrecomputed(
		"CONG_VK_LPP_0",
		smartvectors.RightPadded(vkMappingWhiteListLPP[0], vkMappingWhiteListLPP[0][0], vkMappingPaddedSize),
	)

	vkMappingColumnsLPP[1] = comp.InsertPrecomputed(
		"CONG_VK_LPP_1",
		smartvectors.RightPadded(vkMappingWhiteListLPP[1], vkMappingWhiteListLPP[1][0], vkMappingPaddedSize),
	)

	for j := 0; j < lppGroupingArity; j++ {

		vkMappingColumnsGL[j][0] = comp.InsertPrecomputed(
			ifaces.ColIDf("CONG_VK_GL_%d_0", j),
			smartvectors.RightPadded(vkMappingWhiteListGL[j][0], vkMappingWhiteListGL[j][0][0], vkMappingPaddedSize),
		)

		vkMappingColumnsGL[j][1] = comp.InsertPrecomputed(
			ifaces.ColIDf("CONG_VK_GL_%d_1", j),
			smartvectors.RightPadded(vkMappingWhiteListGL[j][1], vkMappingWhiteListGL[j][1][0], vkMappingPaddedSize),
		)
	}

	return vkMappingColumnsLPP, vkMappingColumnsGL
}

// declareLookups declares the lookup constraints needed to complete the
// holistic checks. The role of these lookups is to:
//
// 1. Ensures that the LPP commitments are correctly passed from GL to LPP
// 2. Ensures that all the verifying keys are whitelisted.
func (cong *ConglomeratorCompilation) declareLookups() {

	var (
		comp = cong.Wiop

		// The effective assignments of the VKs and the LPP columns
		effectiveColumnSize   = utils.NextPowerOfTwo(cong.MaxNbProofs)
		effectiveVksAccessors = [2][]ifaces.Accessor{}
		isGLAccessors         = []ifaces.Accessor{}
		isLPPAccessors        = []ifaces.Accessor{}
		lppColumnsAccessors   = [lppGroupingArity][]ifaces.Accessor{}
	)

	cong.HolisticLookupMappedLPPVK = [2]ifaces.Column{
		comp.InsertCommit(0, "CONG_MAPPED_LPP_VK_0", effectiveColumnSize),
		comp.InsertCommit(0, "CONG_MAPPED_LPP_VK_1", effectiveColumnSize),
	}

	cong.HolisticLookupMappedLPPPostion = comp.InsertCommit(0, "CONG_MAPPED_LPP_POSITION", effectiveColumnSize)

	//
	// Collect the accessors of the public inputs
	//

	for i := 0; i < cong.MaxNbProofs; i++ {

		var (
			verifyingKey  = cong.Recursion.GetPublicInputAccessorOfInstance(comp, preRecursionPrefix+verifyingKeyPublicInput, i)
			verifyingKey2 = cong.Recursion.GetPublicInputAccessorOfInstance(comp, verifyingKey2PublicInput, i)
			isLPP         = cong.Recursion.GetPublicInputAccessorOfInstance(comp, preRecursionPrefix+IsLppPublicInput, i)
			isGL          = cong.Recursion.GetPublicInputAccessorOfInstance(comp, preRecursionPrefix+IsGlPublicInput, i)
		)

		effectiveVksAccessors[0] = append(effectiveVksAccessors[0], verifyingKey)
		effectiveVksAccessors[1] = append(effectiveVksAccessors[1], verifyingKey2)
		isGLAccessors = append(isGLAccessors, isGL)
		isLPPAccessors = append(isLPPAccessors, isLPP)

		for j := 0; j < lppGroupingArity; j++ {
			pubInputName := fmt.Sprintf("%v_%v", lppMerkleRootPublicInput, j)
			lppColumnsAccessors[j] = append(
				lppColumnsAccessors[j],
				cong.Recursion.GetPublicInputAccessorOfInstance(comp, preRecursionPrefix+pubInputName, i),
			)
		}
	}

	//
	// Declare columns for the collected accessors
	//

	var (
		vkColums = [2]ifaces.Column{
			verifiercol.NewFromAccessors(effectiveVksAccessors[0], field.Zero(), effectiveColumnSize),
			verifiercol.NewFromAccessors(effectiveVksAccessors[1], field.Zero(), effectiveColumnSize),
		}

		isGLCol  = verifiercol.NewFromAccessors(isGLAccessors, field.Zero(), effectiveColumnSize)
		isLPPCol = verifiercol.NewFromAccessors(isLPPAccessors, field.Zero(), effectiveColumnSize)
	)

	cong.IsGL = isGLCol
	cong.VerifyingKeyColumns = vkColums

	//
	// This constraints checks the validity of the mapped VK and LPP columns
	//

	var (
		includingLppLookup       = [][]ifaces.Column{}
		includingFilterLppLookup = []ifaces.Column{}
		includingVKeyMatching    = [][]ifaces.Column{}
	)

	for j := 0; j < lppGroupingArity; j++ {

		includingVKeyMatching = append(includingVKeyMatching, []ifaces.Column{
			cong.PrecomputedLPPVks[0],
			cong.PrecomputedLPPVks[1],
			cong.PrecomputedGLVks[j][0],
			cong.PrecomputedGLVks[j][1],
		})

		includingLppLookup = append(includingLppLookup, []ifaces.Column{
			verifiercol.NewConstantCol(field.NewElement(uint64(j)), effectiveColumnSize, ""),
			verifiercol.NewFromAccessors(lppColumnsAccessors[j], field.Zero(), effectiveColumnSize),
			vkColums[0],
			vkColums[1],
		})

		includingFilterLppLookup = append(includingFilterLppLookup, isLPPCol)
	}

	comp.GenericFragmentedConditionalInclusion(
		0,
		ifaces.QueryID("CONG_LPP_CONSISTENCY"),
		includingLppLookup,
		[]ifaces.Column{
			cong.HolisticLookupMappedLPPPostion,
			verifiercol.NewFromAccessors(lppColumnsAccessors[0], field.Zero(), effectiveColumnSize),
			cong.HolisticLookupMappedLPPVK[0],
			cong.HolisticLookupMappedLPPVK[1],
		},
		includingFilterLppLookup,
		isGLCol,
	)

	comp.GenericFragmentedConditionalInclusion(
		0,
		ifaces.QueryID("CONG_VK_CONSISTENCY"),
		includingVKeyMatching,
		[]ifaces.Column{
			cong.HolisticLookupMappedLPPVK[0],
			cong.HolisticLookupMappedLPPVK[1],
			vkColums[0],
			vkColums[1],
		},
		nil,
		isGLCol,
	)
}

// getVerifyingKeyPair extracts the verifyingKeys from the compiled IOP.
func getVerifyingKeyPair(wiop *wizard.CompiledIOP) (vkGL, vkLPP field.Element) {
	return wiop.ExtraData[verifyingKeyPublicInput].(field.Element),
		wiop.ExtraData[verifyingKey2PublicInput].(field.Element)
}

// SanityCheckPublicInputsForConglo checks that a list of runtime is compatible
// with each other. The function will perform the same checks that the
// conglomerator but can be used on debugging-circuits.
func SanityCheckPublicInputsForConglo(runtimes []*wizard.ProverRuntime) error {

	var (
		allGrandProduct           = field.NewElement(1)
		allLogDerivativeSum       = field.Element{}
		allHornerSum              = field.Element{}
		prevGlobalSent            = field.Element{}
		usedSharedRandomness      = field.Element{}
		usedSharedRandomnessFound bool
		mainErr                   error
	)

	type proofPublicInput struct {
		LPPCommitment    field.Element
		SharedRandomness field.Element
		LogDerivativeSum field.Element
		GrandProduct     field.Element
		HornerSum        field.Element
		HornerN0Hash     field.Element
		HornerN1Hash     field.Element
		GlobalReceived   field.Element
		GlobalSent       field.Element
		IsFirst          bool
		IsLast           bool
		IsLPP            bool
		IsGL             bool
		SameVkAsPrev     bool
		SameVkAsNext     bool
	}

	allPis := []proofPublicInput{}

	for _, run := range runtimes {

		pi := proofPublicInput{
			LogDerivativeSum: run.GetPublicInput(LogDerivativeSumPublicInput),
			GrandProduct:     run.GetPublicInput(GrandProductPublicInput),
			HornerSum:        run.GetPublicInput(HornerPublicInput),
			HornerN0Hash:     run.GetPublicInput(HornerN0HashPublicInput),
			HornerN1Hash:     run.GetPublicInput(HornerN1HashPublicInput),
			GlobalReceived:   run.GetPublicInput(GlobalReceiverPublicInput),
			GlobalSent:       run.GetPublicInput(GlobalSenderPublicInput),
			IsFirst:          run.GetPublicInput(IsFirstPublicInput) == field.One(),
			IsLast:           run.GetPublicInput(IsLastPublicInput) == field.One(),
			IsLPP:            run.GetPublicInput(IsLppPublicInput) == field.One(),
			IsGL:             run.GetPublicInput(IsGlPublicInput) == field.One(),
		}

		allPis = append(allPis, pi)

		var (
			sameVerifyingKeyAsPrev = !pi.IsFirst
			sameVerifyingKeyAsNext = !pi.IsLast
		)

		// @alex: actually IsFirst and IsLast are not set for LPP segments so
		// this check is moot.
		// if pi.IsLPP && sameVerifyingKeyAsPrev && pi.HornerN0Hash != prevHornerN1Hash {
		// 	mainErr = errors.Join(mainErr, fmt.Errorf("horner-n0-hash mismatch: %v != %v; i=%v isFirst: %v; isLast: %v", i, pi.IsFirst, pi.IsLast, pi.HornerN0Hash.String(), prevHornerN1Hash.String()))
		// }

		if pi.IsGL && !sameVerifyingKeyAsPrev != pi.IsFirst {
			mainErr = errors.Join(mainErr, errors.New("isFirst is inconsistent with the verifying keys"))
		}

		if pi.IsGL && !sameVerifyingKeyAsNext != pi.IsLast {
			mainErr = errors.Join(mainErr, errors.New("isLast is inconsistent with the verifying keys"))
		}

		if pi.IsGL && sameVerifyingKeyAsPrev && pi.GlobalReceived != prevGlobalSent {
			mainErr = errors.Join(mainErr, errors.New("global sent and receive don't match"))
		}

		if pi.IsLPP && !usedSharedRandomnessFound {
			usedSharedRandomness = pi.SharedRandomness
			usedSharedRandomnessFound = true
		}

		if pi.IsGL && usedSharedRandomnessFound {
			if usedSharedRandomness != pi.SharedRandomness {
				mainErr = errors.Join(mainErr, fmt.Errorf("shared randomness mismatch between different LPP segments: %v and %v", usedSharedRandomness.String(), pi.SharedRandomness.String()))
			}
		}

		prevGlobalSent = pi.GlobalSent

		if pi.IsLPP {
			allGrandProduct.Mul(&allGrandProduct, &pi.GrandProduct)
			allHornerSum.Add(&allHornerSum, &pi.HornerSum)
			allLogDerivativeSum.Add(&allLogDerivativeSum, &pi.LogDerivativeSum)
		}
	}

	if !allGrandProduct.IsOne() {
		mainErr = errors.Join(mainErr, fmt.Errorf("grand product is not one: %v", allGrandProduct.String()))
	}

	if !allHornerSum.IsZero() {
		mainErr = errors.Join(mainErr, fmt.Errorf("horner sum is not zero: %v", allHornerSum.String()))
	}

	if !allLogDerivativeSum.IsZero() {
		mainErr = errors.Join(mainErr, fmt.Errorf("log derivative sum is not zero: %v", allLogDerivativeSum.String()))
	}

	if mainErr != nil {
		fmt.Printf("conglomeration failed: err=%v pis=%++v\n", mainErr, allPis)
	}

	return mainErr

}
