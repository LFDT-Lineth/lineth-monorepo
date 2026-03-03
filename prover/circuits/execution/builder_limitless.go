package execution

import (
	"strings"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

type limitlessBuilder struct {
	compiledConglo            *distributed.RecursedSegmentCompilation
	verificationKeyMerkleRoot field.Element
	traceLimits               *config.TracesLimits
	WizardAssets              *zkevm.LimitlessZkEVM
}

func NewBuilderLimitless(
	compilerConglo *distributed.RecursedSegmentCompilation,
	verificationKeyMerkleRoot field.Element,
	traceLimits *config.TracesLimits,
) *limitlessBuilder {
	return &limitlessBuilder{
		compiledConglo:            compilerConglo,
		verificationKeyMerkleRoot: verificationKeyMerkleRoot,
		traceLimits:               traceLimits,
	}
}

func (b *limitlessBuilder) Compile() (constraint.ConstraintSystem, error) {
	return makeCSLimitless(b), nil
}

// Makes the constraint system for the execution-limitless circuit
func makeCSLimitless(b *limitlessBuilder) constraint.ConstraintSystem {
	circuit := AllocateLimitless(b)

	scs, err := frontend.Compile(fr.Modulus(), scs.NewBuilder, &circuit, frontend.WithCapacity(1<<24))
	if err != nil {
		panic(err)
	}
	return scs
}

// AllocateLimitless allocates the outer-proof circuit in the context of a
// limitless execution. It works as [Allocate] but takes the conglomeration
// wizard as input and uses it to allocate the outer circuit. The trace-limits
// file is used to derive the maximal number of L2L1 logs.
//
// The proof generation can be done using the [MakeProof] function as we would
// do for the non-limitless execution proof.
func AllocateLimitless(b *limitlessBuilder) CircuitExecution {

	congWiop := b.compiledConglo.RecursionComp
	limits := b.traceLimits

	logrus.Infof("Allocating the outer circuit with params: no_of_cong_wiop_rounds=%d "+
		"limits_block_l2l1_logs=%d", congWiop.NumRounds(), limits.BlockL2L1Logs())

	wverifier := wizard.AllocateWizardCircuit(congWiop, congWiop.NumRounds())
	return CircuitExecution{
		LimitlessMode:  true,
		VKMerkleRoot:   b.verificationKeyMerkleRoot,
		CongloVK:       b.compiledConglo.GetVerifyingKeyPair(),
		WizardVerifier: *wverifier,
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, limits.BlockL2L1Logs()),
					Length: nil,
				},
			},
		},
	}
}

// checkLimitlessConglomerationCompletion checks that the conglomeration proof
// is complete:
//
// - all the segments have been aggregated
// - the horner/log-derivative/grand-product cancelled out
// - the shared randomness is correctly computed
// - the general multiset cancels out
func (c *CircuitExecution) checkLimitlessConglomerationCompletion(api frontend.API) {

	wvc := c.WizardVerifier

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
		target           = distributed.GetPublicInputListGnark(api, &wvc, distributed.TargetNbSegmentPublicInputBase, numModule)
		countGL          = distributed.GetPublicInputListGnark(api, &wvc, distributed.SegmentCountGLPublicInputBase, numModule)
		countLPP         = distributed.GetPublicInputListGnark(api, &wvc, distributed.SegmentCountLPPPublicInputBase, numModule)
		generalMSet      = distributed.GetPublicInputListGnark(api, &wvc, distributed.GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
		sharedRandMSet   = distributed.GetPublicInputListGnark(api, &wvc, distributed.SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize)
		initRandomness   = wvc.GetPublicInput(api, distributed.InitialRandomnessPublicInput)
		logDerivativeSum = wvc.GetPublicInput(api, distributed.LogDerivativeSumPublicInput)
		gdProduct        = wvc.GetPublicInput(api, distributed.GrandProductPublicInput)
		hornerSum        = wvc.GetPublicInput(api, distributed.HornerPublicInput)
		vk0              = wvc.GetPublicInput(api, distributed.VerifyingKeyPublicInput)
		vk1              = wvc.GetPublicInput(api, distributed.VerifyingKey2PublicInput)
		vkMerkleRoot     = wvc.GetPublicInput(api, distributed.VerifyingKeyMerkleRootPublicInput)
	)

	for module := 0; module < numModule; module++ {
		api.AssertIsEqual(target[module], countGL[module])
		api.AssertIsEqual(target[module], countLPP[module])
	}

	for k := range generalMSet {
		api.AssertIsEqual(generalMSet[k], 0)
	}

	api.AssertIsEqual(logDerivativeSum, 0)
	api.AssertIsEqual(hornerSum, 0)
	api.AssertIsEqual(gdProduct, 1)
	newRand := mimc.GnarkHashVec(api, sharedRandMSet[:])
	api.AssertIsEqual(initRandomness, newRand)
	api.AssertIsEqual(vk0, c.CongloVK[0])
	api.AssertIsEqual(vk1, c.CongloVK[1])
	api.AssertIsEqual(vkMerkleRoot, c.VKMerkleRoot)

}
