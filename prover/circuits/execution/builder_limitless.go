package execution

import (
	"fmt"
	"strings"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/config"
	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

type limitlessBuilder struct {
	congWIOP     *wizard.CompiledIOP
	traceLimits  *config.TracesLimits
	vkMerkleRoot field.Octuplet
	WizardAssets *zkevm.LimitlessZkEVM
}

func NewBuilderLimitless(congWIOP *wizard.CompiledIOP, traceLimits *config.TracesLimits, vkMerkleRoot field.Octuplet) *limitlessBuilder {
	return &limitlessBuilder{congWIOP: congWIOP, traceLimits: traceLimits, vkMerkleRoot: vkMerkleRoot}
}

func (b *limitlessBuilder) Compile() (constraint.ConstraintSystem, error) {
	return makeCSLimitless(b), nil
}

// Makes the constraint system for the execution-limitless circuit
func makeCSLimitless(b *limitlessBuilder) constraint.ConstraintSystem {
	circuit := AllocateLimitless(b.congWIOP, b.traceLimits, b.vkMerkleRoot)

	scs, err := frontend.Compile(fr.Modulus(), scs.NewBuilder, &circuit, frontend.WithCapacity(1<<24))
	if err != nil {
		panic(err)
	}
	return scs
}

// checkLimitlessConglomerationCompletion checks that the conglomeration proof
// is complete:
//
// - all the segments have been aggregated
// - the horner/log-derivative/grand-product cancelled out
// - the shared randomness is correctly computed
// - the general multiset cancels out
// - the verification keys and merkle root match the expected values
func (c *CircuitExecution) CheckLimitlessConglomerationCompletion(api frontend.API) {

	wvc := c.WizardVerifier
	koalaAPI := koalagnark.NewAPI(api)

	// This loops counts the number of modules by counting the number of public
	// inputs starting with [TargetNbSegmentPublicInputBase]
	numModule := 0
	for _, pub := range wvc.Spec.PublicInputs {
		// In principle, IsPrefix should be the right one but Contains has more
		// chances to stand the test of time if we decide to play with prefixes.
		if strings.Contains(pub.Name, distributed.TargetNbSegmentPublicInputBase) {
			numModule++
		}
	}

	// This checks that the GL and LPP segment counts match the target number of
	// segment for each module.
	var (
		target         = distributed.GetPublicInputListGnark(api, &wvc, distributed.TargetNbSegmentPublicInputBase, numModule)
		countGL        = distributed.GetPublicInputListGnark(api, &wvc, distributed.SegmentCountGLPublicInputBase, numModule)
		countLPP       = distributed.GetPublicInputListGnark(api, &wvc, distributed.SegmentCountLPPPublicInputBase, numModule)
		generalMSet    = distributed.GetPublicInputListGnark(api, &wvc, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
		sharedRandMSet = distributed.GetPublicInputListGnark(api, &wvc, distributed.SharedRandomnessMultiSetPublicInputBase, multisethashing.MSetHashSize)
	)

	// InitialRandomness is an octuplet (8 indexed public inputs)
	var initRandomness [8]koalagnark.Element
	for i := 0; i < 8; i++ {
		initRandomness[i] = wvc.GetPublicInput(api, fmt.Sprintf("%s_%d", distributed.InitialRandomnessPublicInput, i))
	}

	// LogDerivativeSum, GrandProduct, HornerSum are extension field elements
	logDerivativeSum := wvc.GetPublicInputExt(api, distributed.LogDerivativeSumPublicInput)
	gdProduct := wvc.GetPublicInputExt(api, distributed.GrandProductPublicInput)
	hornerSum := wvc.GetPublicInputExt(api, distributed.HornerPublicInput)

	// VK and VKMerkleRoot are octuplets (8 indexed public inputs each)
	var vk0, vk1, vkMerkleRoot [8]koalagnark.Element
	for i := 0; i < 8; i++ {
		vk0[i] = wvc.GetPublicInput(api, fmt.Sprintf("%s_%d", distributed.VerifyingKeyPublicInput, i))
		vk1[i] = wvc.GetPublicInput(api, fmt.Sprintf("%s_%d", distributed.VerifyingKey2PublicInput, i))
		vkMerkleRoot[i] = wvc.GetPublicInput(api, fmt.Sprintf("%s_%d", distributed.VerifyingKeyMerkleRootPublicInput, i))
	}

	for module := 0; module < numModule; module++ {
		api.AssertIsEqual(target[module], countGL[module])
		api.AssertIsEqual(target[module], countLPP[module])
	}

	for k := range generalMSet {
		api.AssertIsEqual(generalMSet[k], 0)
	}

	// Extension field assertions for log-derivative, horner and grand-product
	koalaAPI.AssertIsEqualExt(logDerivativeSum, koalaAPI.ZeroExt())
	koalaAPI.AssertIsEqualExt(hornerSum, koalaAPI.ZeroExt())
	koalaAPI.AssertIsEqualExt(gdProduct, koalaAPI.OneExt())

	// Hash the shared randomness multiset using Poseidon2 over KoalaBear
	// (matching the native poseidon2_koalabear.HashVec used in
	// GetSharedRandomnessFromSegmentProofs)
	hasher := poseidon2_koalabear.NewKoalagnarkMDHasher(koalaAPI)
	sharedRandElements := make([]koalagnark.Element, len(sharedRandMSet))
	for i, v := range sharedRandMSet {
		sharedRandElements[i] = koalagnark.WrapFrontendVariable(v)
	}
	hasher.Write(sharedRandElements...)
	newRand := hasher.Sum()
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(initRandomness[i], newRand[i])
	}

	// VK checks against compile-time constants from the conglomeration compiled IOP
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(vk0[i], koalaAPI.Const(int64(c.CongloVK[0][i].Uint64())))
		koalaAPI.AssertIsEqual(vk1[i], koalaAPI.Const(int64(c.CongloVK[1][i].Uint64())))
		koalaAPI.AssertIsEqual(vkMerkleRoot[i], koalaAPI.Const(int64(c.VKMerkleRoot[i].Uint64())))
	}
}
