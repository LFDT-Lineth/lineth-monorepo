package distributed

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	smt_koalabear "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
)

// ProofType is an enum to identify the type of a proof.
//   - 0 = LPP
//   - 1 = GL
//   - 2 = Conglo
type ProofType int

const (

	// types enum for the proof types
	proofTypeLPP ProofType = iota
	proofTypeGL
	proofTypeConglo

	// aggregationArity is the arity of the aggregation circuit
	aggregationArity = 2

	// name of the public inputs
	TargetNbSegmentPublicInputBase          = "TARGET_NB_SEGMENTS"
	SegmentCountLPPPublicInputBase          = "LPP_SEGMENT_COUNT"
	SegmentCountGLPublicInputBase           = "GL_SEGMENT_COUNT"
	GeneralMultiSetPublicInputBase          = "GENERAL_MULTI_SET"
	SharedRandomnessMultiSetPublicInputBase = "SHARED_RANDOMNESS_MULTI_SET"
	VkMerkleProofBase                       = "VK_MERKLE_PROOF"
	InitialRandomnessPublicInput            = "INITIAL_RANDOMNESS_PUBLIC_INPUT"
	LogDerivativeSumPublicInput             = "LOG_DERIVATE_SUM_PUBLIC_INPUT"
	GrandProductPublicInput                 = "GRAND_PRODUCT_PUBLIC_INPUT"
	HornerPublicInput                       = "HORNER_FINAL_RES_PUBLIC_INPUT"
	globalHashSentPublicInput               = "GLOBAL_HASH_SENT"
	globalHashReceivedPublicInput           = "GLOBAL_HASH_RECEIVED"
	VerifyingKeyPublicInput                 = "VERIFYING_KEY"
	VerifyingKey2PublicInput                = "VERIFYING_KEY_2"
	VerifyingKeyMerkleRootPublicInput       = "VK_MERKLE_ROOT"
	lppMerkleRootPublicInput                = "LPP_COLUMNS_MERKLE_ROOTS"
)

// ConglomerationCompilation holds the compilation context of the hierarchical
// conglomeration.
type ModuleConglo struct {
	// ModuleNumber gives the number of modules of the distributed prover
	ModuleNumber int
	// Wiop is the compiled IOP of the conglomeration wizard.
	Wiop *wizard.CompiledIOP
	// Recursion is the recursion context used to compile the conglomeration
	// proof.
	Recursion *recursion.Recursion
	// PublicInputs stores the public inputs of the conglomeration proof.
	PublicInputs LimitlessPublicInput[wizard.PublicInput, wizard.PublicInput]
	// VerificationKeyMerkleProofs is the list of the verification keys proving
	// the membership of the verifying keys of the instances inside the
	// VerificationKeyMerkleTree. Each merkle proof is structured as a list of
	// D columns if size 1 where D is the depth of the merkle tree.
	VerificationKeyMerkleProofs [][][8]ifaces.Column
}

// ModuleWitnessConglo collects the witness elements of the conglomeration
// compiler
type ModuleWitnessConglo struct {
	SegmentProofs             []SegmentProof
	VerificationKeyMerkleTree VerificationKeyMerkleTree
}

// VerificationKeyMerkleTree is a Merkle tree storing a list of verification keys
// and it is meant to store the verification keys of all the moduleGL/LPP and
// and of the ConglomerationHierarchical circuit.
type VerificationKeyMerkleTree struct {
	Tree             *smt_koalabear.Tree
	VerificationKeys [][2]field.Octuplet
}

// ConglomerationHierarchicalVerifierAction implements the [wizard.VerifierAction]
// interface for the conglomeration proof. It checks the consistency of the
// public inputs with the children instance's public inputs.
type ConglomerationHierarchicalVerifierAction struct {
	ModuleConglo
}

// LimitlessPublicInput stores the columns totalling the
// public inputs of a conglomeration node.
// T is the base field type, E is the extension field type.
type LimitlessPublicInput[T, E any] struct {
	Functionals []struct {
		F *T
		E *E
	}
	TargetNbSegments             []T
	SegmentCountGL               []T
	SegmentCountLPP              []T
	GeneralMultiSetHash          []T
	SharedRandomnessMultiSetHash []T
	VKeyMerkleRoot               [8]T
	VerifyingKey                 [2][8]T
	LogDerivativeSum             E
	HornerSum                    E
	GrandProduct                 E
	SharedRandomness             [8]T
}

// buildVerificationKeyMerkleTree builds the verification key merkle tree.
func buildVerificationKeyMerkleTree(moduleGL, moduleLPP []*RecursedSegmentCompilation, hierAgg *RecursedSegmentCompilation) VerificationKeyMerkleTree {
	var (
		leaves           = make([]field.Octuplet, 0, len(moduleGL)+len(moduleLPP)+1)
		verificationKeys = make([][2]field.Octuplet, 0, len(moduleGL)+len(moduleLPP)+1)
		vkList           = ""
	)

	appendLeaf := func(comp *wizard.CompiledIOP) {
		var (
			vk0, vk1 = getVerifyingKeyPair(comp)
			leaf     = poseidon2_koalabear.HashVec(append(vk0[:], vk1[:]...)...)
		)

		leaves = append(leaves, leaf)
		verificationKeys = append(verificationKeys, [2]field.Octuplet{vk0, vk1})

		vkList += fmt.Sprintf("\t%v %v\n", vk0, vk1)
	}

	for _, module := range moduleGL {
		appendLeaf(module.RecursionCompKoala)
	}

	for _, module := range moduleLPP {
		appendLeaf(module.RecursionCompKoala)
	}

	appendLeaf(hierAgg.RecursionCompKoala)

	// padding with zeroes so that the leaves number if a power-of-two
	paddedSize := utils.NextPowerOfTwo(len(leaves))
	for i := len(leaves); i < paddedSize; i++ {
		leaves = append(leaves, field.Octuplet{})
	}

	return VerificationKeyMerkleTree{
		Tree:             smt_koalabear.NewTree(leaves),
		VerificationKeys: verificationKeys,
	}
}

// GetVkMerkleProof return the merkle proof of a verification key
func (vmt VerificationKeyMerkleTree) GetVkMerkleProof(segProof SegmentProof) []field.Octuplet {
	var (
		leafPosition = -1
		numModule    = utils.DivExact(len(vmt.VerificationKeys)-1, 2)
		moduleIndex  = segProof.ModuleIndex
	)

	switch segProof.ProofType {
	// the instance is a conglomeration proof
	case proofTypeConglo:
		leafPosition = 2 * numModule
	case proofTypeLPP:
		leafPosition = moduleIndex + numModule
	case proofTypeGL:
		leafPosition = moduleIndex
	default:
		panic("unexpected proof type")
	}

	proof := vmt.Tree.MustProve(leafPosition)

	siblings := make([]field.Octuplet, len(proof.Siblings))
	for i := range siblings {
		siblings[i] = proof.Siblings[i]
	}

	return siblings
}

// GetRoot returns the root of the verification key merkle tree encoded as a
// field element.
func (vmt VerificationKeyMerkleTree) GetRoot() field.Octuplet {

	if vmt.Tree == nil {
		panic("verification key merkle tree is nil")
	}

	root := vmt.Tree.Root
	return root
}

// CheckMembership checks if a verification key is in the merkle tree.
func checkVkMembership(t ProofType, numModule int, moduleIndex int, vk [2]field.Octuplet, rootF field.Octuplet, proofF []field.Octuplet) error {
	leafPosition := -1

	switch t {
	// the instance is a conglomeration proof
	case proofTypeConglo:
		leafPosition = 2 * numModule
	case proofTypeLPP:
		leafPosition = moduleIndex + numModule
	case proofTypeGL:
		leafPosition = moduleIndex
	default:
		panic("unexpected proof type")
	}

	// This part of the loop checks the membership of the VK as a member of
	// the tree using the leafPosition from above.

	var (
		merkleDepth = utils.Log2Ceil(2*numModule + 1)
		mProof      = smt_koalabear.Proof{
			Path:     leafPosition,
			Siblings: make([]types.KoalaOctuplet, merkleDepth),
		}
		leaf = poseidon2_koalabear.HashVec(append(vk[0][:], vk[1][:]...)...)
	)

	if merkleDepth != len(proofF) {
		panic("merkleDepth != len(proofF)")
	}

	for lvl := 0; lvl < merkleDepth; lvl++ {
		mProof.Siblings[lvl] = proofF[lvl]
	}

	if err := smt_koalabear.Verify(&mProof, leaf, rootF); err != nil {
		return fmt.Errorf(
			"VK is not a member of the tree: pos: %v, moduleIndex: %v, proofType: %v, leaf: %v, root: %v proof: %v, vk0: %v, vk1: %v",
			leafPosition, moduleIndex, t, types.KoalaOctuplet(leaf).Hex(),
			types.KoalaOctuplet(rootF).Hex(), mProof.String(),
			types.KoalaOctuplet(vk[0]).Hex(), types.KoalaOctuplet(vk[1]).Hex(),
		)
	}

	return nil
}

// CheckMembershipGnark checks if a verification key is in the merkle tree.
func checkVkMembershipGnark(
	api frontend.API,
	leafPosition frontend.Variable,
	numModule int,
	vk [2]poseidon2_koalabear.GnarkOctuplet,
	root poseidon2_koalabear.GnarkOctuplet,
	proofF []poseidon2_koalabear.GnarkOctuplet,
) {
	// This part of the loop checks the membership of the VK as a member of
	// the tree using the leafPosition from above.

	merkleDepth := utils.Log2Ceil(2*numModule + 1)

	if merkleDepth != len(proofF) {
		panic("merkleDepth != len(proofF)")
	}

	mProof := smt_koalabear.GnarkProof{
		Path:     leafPosition,
		Siblings: proofF,
	}

	// Hash the VK to get the leaf: leaf = hash(vk[0] || vk[1])
	h, err := poseidon2_koalabear.NewGnarkMDHasher(api)
	if err != nil {
		panic(err)
	}
	h.WriteOctuplet(vk[0], vk[1])
	leaf := h.Sum()

	// Verify the merkle proof
	if err := smt_koalabear.GnarkVerifyMerkleProof(api, mProof, leaf, root); err != nil {
		panic(err)
	}
}

// Conglomerate runs the conglomeration compiler and returns a pointer to the
// receiver of the method.
func (d *DistributedWizard) Conglomerate(params CompilationParams) *DistributedWizard {
	conglo := &ModuleConglo{
		ModuleNumber: len(d.CompiledGLs),
	}

	comp := wizard.NewCompiledIOP()
	conglo.Compile(comp, d.CompiledGLs[0].RecursionCompKoala)
	d.CompiledConglomeration = CompileSegment(conglo, params)
	assertCompatibleIOPs(d)

	d.VerificationKeyMerkleTree = buildVerificationKeyMerkleTree(
		d.CompiledGLs,
		d.CompiledLPPs,
		d.CompiledConglomeration,
	)

	return d
}

// Compile compiles the conglomeration proof. The function first checks if the
// public inputs are compatible and then compiles the conglomeration proof.
func (c *ModuleConglo) Compile(comp *wizard.CompiledIOP, moduleMod *wizard.CompiledIOP) {

	initialWizardStats := logdata.GetWizardStats(comp)
	logrus.Infof("[Before Conglomeration] numCellsCommitted=%v numCellsPrecomputed=%v numCellsProof=%v, totalCells=%v, numQueriesLogDerivativeSum=%v",
		initialWizardStats.NumCellsCommitted, initialWizardStats.NumCellsPrecomputed, initialWizardStats.NumCellsProof,
		initialWizardStats.TotalCells(), initialWizardStats.NumQueriesLogDerivativeSum)

	c.Recursion = recursion.DefineRecursionOf(comp, moduleMod, recursion.Parameters{
		Name:                   "conglomeration",
		MaxNumProof:            2,
		WithExternalHasherOpts: true,
	})

	c.Wiop = comp

	for _, pi := range scanFunctionalInputs(moduleMod) {
		if pi.Acc.IsBase() {
			newPi := declarePiColumn(c.Wiop, pi.Name)
			c.PublicInputs.Functionals = append(c.PublicInputs.Functionals,
				struct {
					F *wizard.PublicInput
					E *wizard.PublicInput
				}{F: &newPi})
		} else {
			newPi := declarePiColumnExt(c.Wiop, pi.Name)
			c.PublicInputs.Functionals = append(c.PublicInputs.Functionals,
				struct {
					F *wizard.PublicInput
					E *wizard.PublicInput
				}{E: &newPi})
		}
	}

	c.PublicInputs.TargetNbSegments = declareListOfPiColumns(c.Wiop, 0, TargetNbSegmentPublicInputBase, c.ModuleNumber)
	c.PublicInputs.SegmentCountGL = declareListOfPiColumns(c.Wiop, 0, SegmentCountGLPublicInputBase, c.ModuleNumber)
	c.PublicInputs.SegmentCountLPP = declareListOfPiColumns(c.Wiop, 0, SegmentCountLPPPublicInputBase, c.ModuleNumber)
	c.PublicInputs.GeneralMultiSetHash = declareListOfPiColumns(c.Wiop, 0, GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
	c.PublicInputs.SharedRandomnessMultiSetHash = declareListOfPiColumns(c.Wiop, 0, SharedRandomnessMultiSetPublicInputBase, multisethashing.MSetHashSize)
	c.PublicInputs.LogDerivativeSum = declarePiColumnExt(c.Wiop, LogDerivativeSumPublicInput)
	c.PublicInputs.HornerSum = declarePiColumnExt(c.Wiop, HornerPublicInput)
	c.PublicInputs.GrandProduct = declarePiColumnExt(c.Wiop, GrandProductPublicInput)

	for i := range c.PublicInputs.SharedRandomness {
		c.PublicInputs.SharedRandomness[i] = declarePiColumn(c.Wiop, fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i))
	}

	for i := 0; i < 8; i++ {
		c.PublicInputs.VKeyMerkleRoot[i] = declarePiColumn(c.Wiop, fmt.Sprintf("%s_%d", VerifyingKeyMerkleRootPublicInput, i))
	}

	// vkMerkleTreeDepth is the depth of the verification key merkle tree
	vkMerkleTreeDepth := c.VKeyMTreeDepth()
	c.VerificationKeyMerkleProofs = make([][][8]ifaces.Column, aggregationArity)
	for i := 0; i < aggregationArity; i++ {
		c.VerificationKeyMerkleProofs[i] = make([][8]ifaces.Column, vkMerkleTreeDepth)
		for j := 0; j < vkMerkleTreeDepth; j++ {
			for k := 0; k < 8; k++ {
				col := comp.InsertProof(0, ifaces.ColID(fmt.Sprintf("vkMerkleProof_%d_%d_%d", i, j, k)), 1, true)
				c.VerificationKeyMerkleProofs[i][j][k] = col
			}
		}
	}

	comp.RegisterVerifierAction(0, &ConglomerationHierarchicalVerifierAction{ModuleConglo: *c})
}

// GetMainProverStep returns a [wizard.ProverStep] running [Assign] passing
// the provided [ModuleWitness] argument.
func (c *ModuleConglo) GetMainProverStep(witness *ModuleWitnessConglo) wizard.MainProverStep {
	return func(run *wizard.ProverRuntime) {
		c.Assign(&witness.VerificationKeyMerkleTree, run, witness.SegmentProofs)
	}
}

// Assign assigns the public inputs for the conglomeration proof
func (c *ModuleConglo) Assign(
	mt *VerificationKeyMerkleTree,
	run *wizard.ProverRuntime,
	proofs []SegmentProof,
) {
	recursionWitnesses := []recursion.Witness{}

	// This assigns the Merkle proofs in the verification key merkle tree
	for i := range proofs {
		mProof := mt.GetVkMerkleProof(proofs[i])
		recursionWitnesses = append(recursionWitnesses, proofs[i].RecursionWitness)
		for j := range mProof {
			for k := 0; k < 8; k++ {
				run.AssignColumn(
					c.VerificationKeyMerkleProofs[i][j][k].GetColID(),
					smartvectors.NewConstant(mProof[j][k], 1),
				)
			}
		}
	}

	// This runs the recursion system. Expectedly, the filling input is never
	// used because this is pairwise aggregation and we always pass exactly pass
	// exactly 2 inputs.
	c.Recursion.Assign(run, recursionWitnesses, &recursionWitnesses[0])

	// Now, it remains to assign the public inputs for the conglomeration proof.
	var (
		collectedPIs                = [aggregationArity]LimitlessPublicInput[field.Element, fext.Element]{}
		sumCountGLs                 = []field.Element{}
		sumCountLPPs                = []field.Element{}
		mSetSharedRand              = multisethashing.MSetHash{}
		mSetGeneral                 = multisethashing.MSetHash{}
		sumLogDerivative, sumHorner fext.Element
		prodGrandProduct            = fext.One()
	)

	for instance := 0; instance < aggregationArity; instance++ {

		collectedPIs[instance] = c.collectAllPublicInputsOfInstance(run, instance)

		// This combines the query results
		sumHorner.Add(&sumHorner, &collectedPIs[instance].HornerSum)
		sumLogDerivative.Add(&sumLogDerivative, &collectedPIs[instance].LogDerivativeSum)
		prodGrandProduct.Mul(&prodGrandProduct, &collectedPIs[instance].GrandProduct)

		// This combines the multiset hashes
		subMSetGeneral := multisethashing.MSetHash(collectedPIs[instance].GeneralMultiSetHash)
		subMSetSharedRand := multisethashing.MSetHash(collectedPIs[instance].SharedRandomnessMultiSetHash)
		mSetGeneral.Add(subMSetGeneral)
		mSetSharedRand.Add(subMSetSharedRand)
	}

	// This assigns the functional public input by summing them
	for f, toAssignPi := range c.PublicInputs.Functionals {

		switch {
		case toAssignPi.F != nil:
			var sumValue field.Element
			for instance := 0; instance < aggregationArity; instance++ {
				sumValue.Add(&sumValue, collectedPIs[instance].Functionals[f].F)
			}
			assignPiColumn(run, toAssignPi.F.Name, sumValue)
		case toAssignPi.E != nil:
			var sumValue fext.Element
			for instance := 0; instance < aggregationArity; instance++ {
				sumValue.Add(&sumValue, collectedPIs[instance].Functionals[f].E)
			}
			assignPiColumnExt(run, toAssignPi.E.Name, sumValue)
		default:
			panic("unexpected: they are both nil")
		}
	}

	assignListOfPiColumns(run, GeneralMultiSetPublicInputBase, mSetGeneral[:])
	assignListOfPiColumns(run, SharedRandomnessMultiSetPublicInputBase, mSetSharedRand[:])

	assignPiColumnExt(run, LogDerivativeSumPublicInput, sumLogDerivative)
	assignPiColumnExt(run, HornerPublicInput, sumHorner)
	assignPiColumnExt(run, GrandProductPublicInput, prodGrandProduct)

	var sharedRandomness field.Octuplet
	for i := 0; i < aggregationArity; i++ {
		r := collectedPIs[i].SharedRandomness
		for j := range r {
			if !r[j].IsZero() {
				sharedRandomness = r
				break
			}
		}
	}

	for i := range sharedRandomness {
		assignPiColumn(run, fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i), sharedRandomness[i])
		assignPiColumn(run, fmt.Sprintf("%s_%d", VerifyingKeyMerkleRootPublicInput, i), collectedPIs[0].VKeyMerkleRoot[i])
	}

	for k := 0; k < c.ModuleNumber; k++ {

		var sumCountGL, sumCountLPP field.Element

		for instance := 0; instance < aggregationArity; instance++ {
			// This agglomerates the segment count for the GL and the LPPs modules. There
			// is one GL and one LPP counter for each module that's why we do them in the
			sumCountGL.Add(&sumCountGL, &collectedPIs[instance].SegmentCountGL[k])
			sumCountLPP.Add(&sumCountLPP, &collectedPIs[instance].SegmentCountLPP[k])
		}

		sumCountGLs = append(sumCountGLs, sumCountGL)
		sumCountLPPs = append(sumCountLPPs, sumCountLPP)
	}

	assignListOfPiColumns(run, TargetNbSegmentPublicInputBase, collectedPIs[0].TargetNbSegments)
	assignListOfPiColumns(run, SegmentCountGLPublicInputBase, sumCountGLs)
	assignListOfPiColumns(run, SegmentCountLPPPublicInputBase, sumCountLPPs)
}

// Run implements the [wizard.VerifierAction] for the
// ConglomerationHierarchicalVerifierAction.
func (c *ConglomerationHierarchicalVerifierAction) Run(run wizard.Runtime) error {
	var (
		err          error
		collectedPIs = [aggregationArity]LimitlessPublicInput[field.Element, fext.Element]{}
		topPIs       = collectAllPublicInputs(run)
	)

	for instance := 0; instance < aggregationArity; instance++ {
		collectedPIs[instance] = c.collectAllPublicInputsOfInstance(run, instance)
	}

	// This checks that the functional public inputs are correctly conglomerated
	// across all instances.
	for k, topPI := range topPIs.Functionals {

		switch {
		case topPI.F != nil:
			summedUpValue := field.Element{}
			for instance := 0; instance < aggregationArity; instance++ {
				funcPI := collectedPIs[instance].Functionals[k]
				summedUpValue.Add(&summedUpValue, funcPI.F)
			}
			if summedUpValue != *topPI.F {
				err = errors.Join(err, fmt.Errorf("public input mismatch for Functionals at index %d, name=%v", k, c.PublicInputs.Functionals[k].F.Name))
			}

		case topPI.E != nil:
			summedUpValue := fext.Element{}
			for instance := 0; instance < aggregationArity; instance++ {
				funcPI := collectedPIs[instance].Functionals[k]
				summedUpValue.Add(&summedUpValue, funcPI.E)
			}
			if summedUpValue != *topPI.E {
				err = errors.Join(err, fmt.Errorf("public input mismatch for Functionals at index %d, name=%v", k, c.PublicInputs.Functionals[k].E.Name))
			}

		default:
			panic("unexpected: they are both nil")
		}

	}

	for k := 0; k < c.ModuleNumber; k++ {

		var (
			sumCountGL  = field.Element{}
			sumCountLPP = field.Element{}
		)

		for instance := 0; instance < aggregationArity; instance++ {

			// This checks that the TargetNbSegments public inputs are the same for all
			// the children instances and the current node.
			if collectedPIs[instance].TargetNbSegments[k] != topPIs.TargetNbSegments[k] {
				err = errors.Join(err, fmt.Errorf(
					"public input mismatch for TargetNbSegments at instance %d, k=%v, collected.TargetNbSegments[k]=%v, assigned.TargetNbSegments[k]=%v",
					instance, k, collectedPIs[instance].TargetNbSegments[k].String(), topPIs.TargetNbSegments[k].String(),
				))
			}

			// This agglomerates the segment count for the GL and the LPPs modules. There
			// is one GL and one LPP counter for each module that's why we do them in the
			sumCountGL.Add(&sumCountGL, &collectedPIs[instance].SegmentCountGL[k])
			sumCountLPP.Add(&sumCountLPP, &collectedPIs[instance].SegmentCountLPP[k])
		}

		if sumCountGL != topPIs.SegmentCountGL[k] {
			err = errors.Join(err, fmt.Errorf("public input mismatch for SegmentCountGL for module %d", k))
		}

		if sumCountLPP != topPIs.SegmentCountLPP[k] {
			err = errors.Join(err, fmt.Errorf("public input mismatch for SegmentCountLPP for module %d", k))
		}
	}

	// This agglomerates the multiset hashes
	for k := 0; k < multisethashing.MSetHashSize; k++ {

		var (
			generalSum = field.Element{}
			sharedSum  = field.Element{}
		)

		for instance := 0; instance < aggregationArity; instance++ {
			generalSum.Add(&generalSum, &collectedPIs[instance].GeneralMultiSetHash[k])
			sharedSum.Add(&sharedSum, &collectedPIs[instance].SharedRandomnessMultiSetHash[k])
		}

		if generalSum != topPIs.GeneralMultiSetHash[k] {
			err = errors.Join(err, fmt.Errorf("public input mismatch for generalMultiSetHash for index %d", k))
		}

		if sharedSum != topPIs.SharedRandomnessMultiSetHash[k] {
			err = errors.Join(err, fmt.Errorf("public input mismatch for sharedRandomness for index %d", k))
		}
	}

	// The loop below "aggregate" the public inputs: log-derivative-sum, gd-product,
	// and horner sum of the sub-instances. The aggregation is done by multiplying/summing
	// the values. The results are then compared the top-level public inputs.
	var (
		accGrandProduct = fext.One()
		accLogDeriv     = fext.Zero()
		accHornerSum    = fext.Zero()
	)

	for instance := 0; instance < aggregationArity; instance++ {

		// This agglomerates the horner N0 hash checker, the grand product, the
		// log derivative sum and the horner sum.
		accGrandProduct.Mul(&accGrandProduct, &collectedPIs[instance].GrandProduct)
		accLogDeriv.Add(&accLogDeriv, &collectedPIs[instance].LogDerivativeSum)
		accHornerSum.Add(&accHornerSum, &collectedPIs[instance].HornerSum)

		for i := range collectedPIs[instance].SharedRandomness {
			if !collectedPIs[instance].SharedRandomness[i].IsZero() && collectedPIs[instance].SharedRandomness[i] != topPIs.SharedRandomness[i] {
				err = errors.Join(err, fmt.Errorf("public input mismatch for SharedRandomness for instance %d", instance))
			}
		}

		if collectedPIs[instance].VKeyMerkleRoot != topPIs.VKeyMerkleRoot {
			err = errors.Join(err, fmt.Errorf("public input mismatch for VKeyMerkleRoot for instance %d, sub-value=%v, top-value=%v",
				instance, collectedPIs[instance].VKeyMerkleRoot, topPIs.VKeyMerkleRoot,
			))
		}
	}

	if accGrandProduct != topPIs.GrandProduct {
		err = errors.Join(err, fmt.Errorf("public input mismatch for GrandProduct, %v != %v", accGrandProduct.String(), topPIs.GrandProduct.String()))
	}

	if accLogDeriv != topPIs.LogDerivativeSum {
		err = errors.Join(err, fmt.Errorf("public input mismatch for LogDerivativeSum, %v != %v", accLogDeriv.String(), topPIs.LogDerivativeSum.String()))
	}

	if accHornerSum != topPIs.HornerSum {
		err = errors.Join(err, fmt.Errorf("public input mismatch for HornerSum, %v != %v", accHornerSum.String(), topPIs.HornerSum.String()))
	}

	// This loop checks the VK membership in the tree. The merkle leaf position
	// is deduced from the segment count public inputs in the following way;
	//
	// 	- If segment-count-sum of the GL position is one and LPP is zero, then
	//  	the position is the position of the "count=1" GL input.
	//
	//  - If the segment segment-count-sum of the LPP positions is one and GL is
	// 		zero, then the position is the position of the "count=1" LPP input +
	// 		nb-module
	//
	// 	- Otherwise (the total sum is larger than 1), the position is 2*nb-module

	for instance := 0; instance < aggregationArity; instance++ {

		proofType, moduleIndex := findProofTypeAndModule(collectedPIs[instance])

		mProof := make([][8]field.Element, c.ModuleConglo.VKeyMTreeDepth())
		for i := range mProof {
			for j := 0; j < 8; j++ {
				mProof[i][j] = c.VerificationKeyMerkleProofs[instance][i][j].GetColAssignmentAt(run, 0)
			}
		}

		vkErr := checkVkMembership(
			proofType,
			c.ModuleNumber,
			moduleIndex,
			collectedPIs[instance].VerifyingKey,
			collectedPIs[instance].VKeyMerkleRoot,
			mProof,
		)

		if vkErr != nil {
			err = errors.Join(err, vkErr)
		}
	}

	if err != nil {
		return err
	}

	return nil
}

// RunGnark implements the [wizard.VerifierAction] interface.
func (c *ConglomerationHierarchicalVerifierAction) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	koalaAPI := koalagnark.NewAPI(api)

	var (
		collectedPIs = [aggregationArity]LimitlessPublicInput[frontend.Variable, koalagnark.Ext]{}
		topPIs       = c.collectAllPublicInputsGnark(api, run)
		hasher       = run.GetHasherFactory().NewHasher()
	)

	for instance := 0; instance < aggregationArity; instance++ {
		collectedPIs[instance] = c.collectAllPublicInputsOfInstanceGnark(api, run, instance)
	}

	// This checks that the functional public inputs are correctly conglomerated
	// across all instances.
	for k, topPI := range topPIs.Functionals {
		switch {
		case topPI.F != nil:
			summedUpValue := frontend.Variable(0)
			for instance := 0; instance < aggregationArity; instance++ {
				funcPI := *collectedPIs[instance].Functionals[k].F
				summedUpValue = api.Add(summedUpValue, funcPI)
			}
			api.AssertIsEqual(summedUpValue, *topPI.F)

		case topPI.E != nil:
			summedUpValue := koalagnark.NewExt(fext.Zero())
			for instance := 0; instance < aggregationArity; instance++ {
				funcPI := *collectedPIs[instance].Functionals[k].E
				summedUpValue = koalaAPI.AddExt(summedUpValue, funcPI)
			}
			koalaAPI.AssertIsEqualExt(summedUpValue, *topPI.E)

		default:
			panic("unexpected: they are both nil")
		}
	}

	for k := 0; k < c.ModuleNumber; k++ {

		var (
			sumCountGL  = frontend.Variable(0)
			sumCountLPP = frontend.Variable(0)
		)

		for instance := 0; instance < aggregationArity; instance++ {

			// This checks that the TargetNbSegments public inputs are the same for all
			// the children instances and the current node.
			api.AssertIsEqual(collectedPIs[instance].TargetNbSegments[k], topPIs.TargetNbSegments[k])

			// This agglomerates the segment count for the GL and the LPPs modules. There
			// is one GL and one LPP counter for each module that's why we do them in the
			sumCountGL = api.Add(sumCountGL, collectedPIs[instance].SegmentCountGL[k])
			sumCountLPP = api.Add(sumCountLPP, collectedPIs[instance].SegmentCountLPP[k])
		}

		api.AssertIsEqual(sumCountGL, topPIs.SegmentCountGL[k])
		api.AssertIsEqual(sumCountLPP, topPIs.SegmentCountLPP[k])
	}

	// This agglomerates the multiset hashes
	var (
		generalSum = multisethashing.EmptyMSetHashGnark(hasher)
		sharedSum  = multisethashing.EmptyMSetHashGnark(hasher)
	)

	for instance := 0; instance < aggregationArity; instance++ {
		generalSum.AddRaw(api, collectedPIs[instance].GeneralMultiSetHash)
		sharedSum.AddRaw(api, collectedPIs[instance].SharedRandomnessMultiSetHash)
	}

	sharedSum.AssertEqualRaw(api, topPIs.SharedRandomnessMultiSetHash)
	generalSum.AssertEqualRaw(api, topPIs.GeneralMultiSetHash)

	// The loop below "aggregate" the public inputs: log-derivative-sum, gd-product,
	// and horner sum of the sub-instances. The aggregation is done by multiplying/summing
	// the values. The results are then compared the top-level public inputs.
	var (
		accGrandProduct = koalaAPI.OneExt()
		accLogDeriv     = koalaAPI.ZeroExt()
		accHornerSum    = koalaAPI.ZeroExt()
	)

	for instance := 0; instance < aggregationArity; instance++ {

		// This agglomerates the horner N0 hash checker, the grand product, the
		// log derivative sum and the horner sum.
		accGrandProduct = koalaAPI.MulExt(accGrandProduct, collectedPIs[instance].GrandProduct)
		accLogDeriv = koalaAPI.AddExt(accLogDeriv, collectedPIs[instance].LogDerivativeSum)
		accHornerSum = koalaAPI.AddExt(accHornerSum, collectedPIs[instance].HornerSum)

		// Check shared randomness consistency - if non-zero, must match top level
		for i := 0; i < 8; i++ {
			api.AssertIsEqual(
				api.Mul(
					api.Sub(1, api.IsZero(collectedPIs[instance].SharedRandomness[i])),
					api.Sub(collectedPIs[instance].SharedRandomness[i], topPIs.SharedRandomness[i]),
				),
				0,
			)
		}

		// Check VKey merkle root consistency
		for i := 0; i < 8; i++ {
			api.AssertIsEqual(collectedPIs[instance].VKeyMerkleRoot[i], topPIs.VKeyMerkleRoot[i])
		}
	}

	koalaAPI.AssertIsEqualExt(accGrandProduct, topPIs.GrandProduct)
	koalaAPI.AssertIsEqualExt(accLogDeriv, topPIs.LogDerivativeSum)
	koalaAPI.AssertIsEqualExt(accHornerSum, topPIs.HornerSum)

	// This loop checks the VK membership in the tree. The merkle leaf position
	// is deduced from the segment count public inputs in the following way;
	//
	// 	- If segment-count-sum of the GL position is one and LPP is zero, then
	//  	the position is the position of the "count=1" GL input.
	//
	//  - If the segment segment-count-sum of the LPP positions is one and GL is
	// 		zero, then the position is the position of the "count=1" LPP input +
	// 		nb-module
	//
	// 	- Otherwise (the total sum is larger than 1), the position is 2*nb-module

	for instance := 0; instance < aggregationArity; instance++ {

		leafPosition := findVkPositionGnark(api, collectedPIs[instance])
		mProof := make([]poseidon2_koalabear.GnarkOctuplet, c.ModuleConglo.VKeyMTreeDepth())
		for i := range mProof {
			for j := 0; j < 8; j++ {
				wrapped := c.VerificationKeyMerkleProofs[instance][i][j].GetColAssignmentGnarkAt(run, 0)
				mProof[i][j] = wrapped.Native()
			}
		}

		checkVkMembershipGnark(
			api,
			leafPosition,
			c.ModuleNumber,
			[2]poseidon2_koalabear.GnarkOctuplet{
				collectedPIs[instance].VerifyingKey[0],
				collectedPIs[instance].VerifyingKey[1],
			},
			collectedPIs[instance].VKeyMerkleRoot,
			mProof,
		)
	}
}

// declarePi declares a column with the requested name as proof column and length
// one and also declare a public input from that column with the same provided
// name.
func declarePiColumn(comp *wizard.CompiledIOP, name string) wizard.PublicInput {
	return declarePiColumnAtRound(comp, 0, name, true)
}

// declarePiColumnExt is as [declarePiColumn] but for extended field elements
func declarePiColumnExt(comp *wizard.CompiledIOP, name string) wizard.PublicInput {
	return declarePiColumnAtRound(comp, 0, name, false)
}

// declarePiColumn at round declares a column at the requested round to generate
// a public input with the requested name.
func declarePiColumnAtRound(comp *wizard.CompiledIOP, round int, name string, isBase bool) wizard.PublicInput {
	col := comp.InsertProof(round, ifaces.ColID(name+"_PI_COLUMN"), 1, isBase)
	return comp.InsertPublicInput(name, accessors.NewFromPublicColumn(col, 0))
}

// assignPiColumn assigns the column of a public input with the requested name
// to the provided column.
func assignPiColumn(run *wizard.ProverRuntime, name string, val ...field.Element) {
	run.AssignColumn(
		ifaces.ColID(name+"_PI_COLUMN"),
		smartvectors.NewRegular(val[:]),
	)
}

func assignPiColumnExt(run *wizard.ProverRuntime, name string, val ...fext.Element) {
	run.AssignColumn(
		ifaces.ColID(name+"_PI_COLUMN"),
		smartvectors.NewRegularExt(val[:]),
	)
}

// declareListOfPiColumns declares a list of columns with the requested name as
// proof columns and length provided.
func declareListOfPiColumns(comp *wizard.CompiledIOP, round int, name string, length int) []wizard.PublicInput {
	var cols []wizard.PublicInput
	for i := 0; i < length; i++ {
		cols = append(cols, declarePiColumnAtRound(comp, round, name+"_"+strconv.Itoa(i), true))
	}
	return cols
}

// declareListOfConstantPi declares a list of public inputs as constant values.
// This is useful to create dummy public inputs making the aggregation process
// simpler.
func declareListOfConstantPi(comp *wizard.CompiledIOP, name string, values []field.Element) []wizard.PublicInput {
	res := make([]wizard.PublicInput, len(values))
	for i, val := range values {
		name := name + "_" + strconv.Itoa(i)
		pub := comp.InsertPublicInput(name, accessors.NewConstant(val))
		res = append(res, pub)
	}
	return res
}

// assignListOfPiColumns assigns a list of columns with the requested name using
// the provided list of values.
func assignListOfPiColumns(run *wizard.ProverRuntime, name string, values []field.Element) {
	for i, val := range values {
		name := name + "_" + strconv.Itoa(i)
		assignPiColumn(run, name, val)
	}
}

// GetPublicInputList returns a list of public input of the provided name for the
// current WIOP (e.g. not the for the children instance).
//
// @alex: would be interesting to make that a utility function in the wizard
// package because it helps whenever we want to encode stuffs as public inputs.
func GetPublicInputList(run wizard.Runtime, name string, nb int) []field.Element {
	var res []field.Element
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		if !run.GetPublicInput(name).IsBase {
			utils.Panic("public input %v is not a base element", name)
		}
		res = append(res, run.GetPublicInput(name).Base)
	}
	return res
}

// getPublicInputListOfInstance returns a list of public inputs of the provided
// name and instance.
func getPublicInputListOfInstance(rec *recursion.Recursion, run wizard.Runtime, name string, instance int, nb int) []field.Element {
	var res []field.Element
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		res = append(res, rec.GetPublicInputOfInstance(run, name, instance).Base)
	}
	return res
}

// GetPublicInputListGnark is as [getPublicInputList] but for gnark.
func GetPublicInputListGnark(api frontend.API, run wizard.GnarkRuntime, name string, nb int) []frontend.Variable {
	var res []frontend.Variable
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		wrapped := run.GetPublicInput(api, name)
		res = append(res, wrapped.Native())
	}
	return res
}

// getPublicInputListOfInstance returns a list of public inputs of the provided
// name and instance.
func getPublicInputListOfInstanceGnark(rec *recursion.Recursion, api frontend.API, run wizard.GnarkRuntime, name string, instance int, nb int) []frontend.Variable {
	var res []frontend.Variable
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		wrapped := rec.GetPublicInputOfInstanceGnark(api, run, name, instance)
		res = append(res, wrapped.Native())
	}
	return res
}

// getPublicInputExtOfInstanceGnark returns the extension field value of a public
// input for the given instance. It uses the accessor's GetFrontendVariableExt method.
func getPublicInputExtOfInstanceGnark(rec *recursion.Recursion, api frontend.API, run wizard.GnarkRuntime, name string, instance int) koalagnark.Ext {
	fullName := rec.Name + "-" + strconv.Itoa(instance) + "." + name
	acc := run.GetSpec().GetPublicInputAccessor(fullName)
	// @azam this may make recursion complicated, alex proposed to register all the public Inputs as field element on the runtime object. while we can have functions that create extension from runtime. E.g. we can have 4 calls to the above function.
	return acc.GetFrontendVariableExt(api, run)
}

// getPublicInputExtGnark returns the extension field value of a public input.
// It uses the accessor's GetFrontendVariableExt method.
func getPublicInputExtGnark(api frontend.API, run wizard.GnarkRuntime, name string) koalagnark.Ext {
	acc := run.GetSpec().GetPublicInputAccessor(name)
	return acc.GetFrontendVariableExt(api, run)
}

// collectAllPublicInputsOfInstance returns a structured object representing
// the public inputs of the given instance.
func (c ModuleConglo) collectAllPublicInputsOfInstance(run wizard.Runtime, instance int) LimitlessPublicInput[field.Element, fext.Element] {
	// Fetching the VKey Public input Merkle root
	vKeyMerkleRoot := [8]field.Element{}
	sharedRandomness := [8]field.Element{}
	for i := range vKeyMerkleRoot {
		vKeyMerkleRoot[i] = c.Recursion.GetPublicInputOfInstance(run, VerifyingKeyMerkleRootPublicInput+"_"+strconv.Itoa(i), instance).Base
		sharedRandomness[i] = c.Recursion.GetPublicInputOfInstance(run, fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i), instance).Base
	}

	vk := [2]field.Octuplet{}
	for i := range vk[0] {
		vk[0][i] = c.Recursion.GetPublicInputOfInstance(run, fmt.Sprintf("%s_%d", VerifyingKeyPublicInput, i), instance).Base
		vk[1][i] = c.Recursion.GetPublicInputOfInstance(run, fmt.Sprintf("%s_%d", VerifyingKey2PublicInput, i), instance).Base
	}

	res := LimitlessPublicInput[field.Element, fext.Element]{
		TargetNbSegments:             getPublicInputListOfInstance(c.Recursion, run, TargetNbSegmentPublicInputBase, instance, c.ModuleNumber),
		SegmentCountGL:               getPublicInputListOfInstance(c.Recursion, run, SegmentCountGLPublicInputBase, instance, c.ModuleNumber),
		SegmentCountLPP:              getPublicInputListOfInstance(c.Recursion, run, SegmentCountLPPPublicInputBase, instance, c.ModuleNumber),
		GeneralMultiSetHash:          getPublicInputListOfInstance(c.Recursion, run, GeneralMultiSetPublicInputBase, instance, multisethashing.MSetHashSize),
		SharedRandomnessMultiSetHash: getPublicInputListOfInstance(c.Recursion, run, SharedRandomnessMultiSetPublicInputBase, instance, multisethashing.MSetHashSize),
		LogDerivativeSum:             c.Recursion.GetPublicInputOfInstance(run, LogDerivativeSumPublicInput, instance).Ext,
		HornerSum:                    c.Recursion.GetPublicInputOfInstance(run, HornerPublicInput, instance).Ext,
		GrandProduct:                 c.Recursion.GetPublicInputOfInstance(run, GrandProductPublicInput, instance).Ext,
		SharedRandomness:             sharedRandomness,
		VKeyMerkleRoot:               vKeyMerkleRoot,
		VerifyingKey:                 vk,
	}

	for _, pi := range c.PublicInputs.Functionals {

		switch {
		case pi.F != nil:
			fGen := c.Recursion.GetPublicInputOfInstance(run, pi.F.Name, instance)
			res.Functionals = append(res.Functionals, struct {
				F *field.Element
				E *fext.Element
			}{F: &fGen.Base})

		case pi.E != nil:
			fGen := c.Recursion.GetPublicInputOfInstance(run, pi.E.Name, instance)
			res.Functionals = append(res.Functionals, struct {
				F *field.Element
				E *fext.Element
			}{E: &fGen.Ext})

		default:
			utils.Panic("unknown public input type")
		}
	}

	return res
}

// collectAllPublicInputs returns a structured object representing the public
// inputs of all the instances.
func collectAllPublicInputs(run wizard.Runtime) LimitlessPublicInput[field.Element, fext.Element] {
	// This function auto-detects the number of module. It counts the number of
	// public inputs with the [targetNbSegmentPublicInputBase] prefix in their
	// name.
	var (
		moduleNumber int
		pubs         = run.GetSpec().PublicInputs
	)

	for _, pub := range pubs {
		if strings.Contains(pub.Name, TargetNbSegmentPublicInputBase) && !strings.Contains(pub.Name, "conglomeration") {
			moduleNumber++
		}
	}

	// Fetching the VKey Public input Merkle root
	vKeyMerkleRoot := [8]field.Element{}
	for i := 0; i < 8; i++ {
		vKeyMerkleRoot[i] = run.GetPublicInput(VerifyingKeyMerkleRootPublicInput + "_" + strconv.Itoa(i)).Base
	}

	sharedRandomness := [8]field.Element{}
	for i := range sharedRandomness {
		sharedRandomness[i] = run.GetPublicInput(fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i)).Base
	}
	res := LimitlessPublicInput[field.Element, fext.Element]{
		TargetNbSegments:             GetPublicInputList(run, TargetNbSegmentPublicInputBase, moduleNumber),
		SegmentCountGL:               GetPublicInputList(run, SegmentCountGLPublicInputBase, moduleNumber),
		SegmentCountLPP:              GetPublicInputList(run, SegmentCountLPPPublicInputBase, moduleNumber),
		GeneralMultiSetHash:          GetPublicInputList(run, GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize),
		SharedRandomnessMultiSetHash: GetPublicInputList(run, SharedRandomnessMultiSetPublicInputBase, multisethashing.MSetHashSize),
		LogDerivativeSum:             run.GetPublicInput(LogDerivativeSumPublicInput).Ext,
		HornerSum:                    run.GetPublicInput(HornerPublicInput).Ext,
		GrandProduct:                 run.GetPublicInput(GrandProductPublicInput).Ext,
		SharedRandomness:             sharedRandomness,
		VKeyMerkleRoot:               vKeyMerkleRoot,
	}

	for _, pi := range scanFunctionalInputs(run.GetSpec()) {
		fGen := run.GetPublicInput(pi.Name)
		if pi.Acc.IsBase() {
			res.Functionals = append(res.Functionals,
				struct {
					F *field.Element
					E *fext.Element
				}{F: &fGen.Base})
		} else {
			res.Functionals = append(res.Functionals,
				struct {
					F *field.Element
					E *fext.Element
				}{E: &fGen.Ext})
		}
	}

	return res
}

// collectAllPublicInputsOfInstanceGnark returns a structured object representing
// the public inputs of the given instance.
func (c ModuleConglo) collectAllPublicInputsOfInstanceGnark(api frontend.API, run wizard.GnarkRuntime, instance int) LimitlessPublicInput[frontend.Variable, koalagnark.Ext] {
	// Fetching the VKey Public input Merkle root
	vKeyMerkleRoot := [8]frontend.Variable{}
	sharedRandomness := [8]frontend.Variable{}
	for i := 0; i < 8; i++ {
		wrapped := c.Recursion.GetPublicInputOfInstanceGnark(api, run, VerifyingKeyMerkleRootPublicInput+"_"+strconv.Itoa(i), instance)
		vKeyMerkleRoot[i] = wrapped.Native()
		wrapped = c.Recursion.GetPublicInputOfInstanceGnark(api, run, fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i), instance)
		sharedRandomness[i] = wrapped.Native()
	}

	vk := [2][8]frontend.Variable{}
	for i := range vk[0] {
		wrapped := c.Recursion.GetPublicInputOfInstanceGnark(api, run, fmt.Sprintf("%s_%d", VerifyingKeyPublicInput, i), instance)
		vk[0][i] = wrapped.Native()
		wrapped = c.Recursion.GetPublicInputOfInstanceGnark(api, run, fmt.Sprintf("%s_%d", VerifyingKey2PublicInput, i), instance)
		vk[1][i] = wrapped.Native()
	}

	res := LimitlessPublicInput[frontend.Variable, koalagnark.Ext]{
		TargetNbSegments:             getPublicInputListOfInstanceGnark(c.Recursion, api, run, TargetNbSegmentPublicInputBase, instance, c.ModuleNumber),
		SegmentCountGL:               getPublicInputListOfInstanceGnark(c.Recursion, api, run, SegmentCountGLPublicInputBase, instance, c.ModuleNumber),
		SegmentCountLPP:              getPublicInputListOfInstanceGnark(c.Recursion, api, run, SegmentCountLPPPublicInputBase, instance, c.ModuleNumber),
		GeneralMultiSetHash:          getPublicInputListOfInstanceGnark(c.Recursion, api, run, GeneralMultiSetPublicInputBase, instance, multisethashing.MSetHashSize),
		SharedRandomnessMultiSetHash: getPublicInputListOfInstanceGnark(c.Recursion, api, run, SharedRandomnessMultiSetPublicInputBase, instance, multisethashing.MSetHashSize),
		LogDerivativeSum:             getPublicInputExtOfInstanceGnark(c.Recursion, api, run, LogDerivativeSumPublicInput, instance),
		HornerSum:                    getPublicInputExtOfInstanceGnark(c.Recursion, api, run, HornerPublicInput, instance),
		GrandProduct:                 getPublicInputExtOfInstanceGnark(c.Recursion, api, run, GrandProductPublicInput, instance),
		SharedRandomness:             sharedRandomness,
		VKeyMerkleRoot:               vKeyMerkleRoot,
		VerifyingKey:                 vk,
	}

	for _, pi := range c.PublicInputs.Functionals {
		switch {
		case pi.F != nil:
			wrapped := c.Recursion.GetPublicInputOfInstanceGnark(api, run, pi.F.Name, instance)
			wrappedFE := wrapped.Native()
			res.Functionals = append(res.Functionals, struct {
				F *frontend.Variable
				E *koalagnark.Ext
			}{F: &wrappedFE})

		case pi.E != nil:
			wrapped := c.Recursion.GetPublicInputAccessorOfInstance(run.GetSpec(), pi.E.Name, instance).GetFrontendVariableExt(api, run)
			res.Functionals = append(res.Functionals, struct {
				F *frontend.Variable
				E *koalagnark.Ext
			}{E: &wrapped})

		default:
			panic("unreachable")
		}
	}

	return res
}

// collectAllPublicInputsGnark returns a structured object representing the public
// inputs of all the instances.
//
// In the returned object, the verifying key public inputs are not populated.
func (c ModuleConglo) collectAllPublicInputsGnark(api frontend.API, run wizard.GnarkRuntime) LimitlessPublicInput[frontend.Variable, koalagnark.Ext] {
	// Fetching the VKey Public input Merkle root
	vKeyMerkleRoot := [8]frontend.Variable{}
	sharedRandomness := [8]frontend.Variable{}
	for i := 0; i < 8; i++ {
		wrapped := run.GetPublicInput(api, VerifyingKeyMerkleRootPublicInput+"_"+strconv.Itoa(i))
		vKeyMerkleRoot[i] = wrapped.Native()
		wrapped = run.GetPublicInput(api, fmt.Sprintf("%s_%d", InitialRandomnessPublicInput, i))
		sharedRandomness[i] = wrapped.Native()
	}
	res := LimitlessPublicInput[frontend.Variable, koalagnark.Ext]{
		TargetNbSegments:             GetPublicInputListGnark(api, run, TargetNbSegmentPublicInputBase, c.ModuleNumber),
		SegmentCountGL:               GetPublicInputListGnark(api, run, SegmentCountGLPublicInputBase, c.ModuleNumber),
		SegmentCountLPP:              GetPublicInputListGnark(api, run, SegmentCountLPPPublicInputBase, c.ModuleNumber),
		GeneralMultiSetHash:          GetPublicInputListGnark(api, run, GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize),
		SharedRandomnessMultiSetHash: GetPublicInputListGnark(api, run, SharedRandomnessMultiSetPublicInputBase, multisethashing.MSetHashSize),
		LogDerivativeSum:             getPublicInputExtGnark(api, run, LogDerivativeSumPublicInput),
		HornerSum:                    getPublicInputExtGnark(api, run, HornerPublicInput),
		GrandProduct:                 getPublicInputExtGnark(api, run, GrandProductPublicInput),
		SharedRandomness:             sharedRandomness,
		VKeyMerkleRoot:               vKeyMerkleRoot,
	}

	for _, pi := range scanFunctionalInputs(c.Recursion.InputCompiledIOP) {

		if pi.Acc.IsBase() {
			wrapped := run.GetPublicInput(api, pi.Name)
			wrappedFE := wrapped.Native()
			res.Functionals = append(res.Functionals, struct {
				F *frontend.Variable
				E *koalagnark.Ext
			}{F: &wrappedFE})

		} else {
			wrapped := run.GetSpec().GetPublicInputAccessor(pi.Name).
				GetFrontendVariableExt(api, run)
			res.Functionals = append(res.Functionals, struct {
				F *frontend.Variable
				E *koalagnark.Ext
			}{E: &wrapped})
		}
	}

	return res
}

// VKeyMTreeDepth returns the depth of the verification key merkle tree.
func (c ModuleConglo) VKeyMTreeDepth() int {
	return utils.Log2Ceil(2*c.ModuleNumber + 1)
}

// findProofTypeAndModule returns the proofType and the module index of the
// provided instance given collected public inputs of the instances.
func findProofTypeAndModule(instance LimitlessPublicInput[field.Element, fext.Element]) (ProofType, int) {
	var (
		sumGL, sumLPP = 0, 0
		moduleIndex   = -1 // can't be -1 at the end of the "mod" loop.
		moduleNumber  = len(instance.SegmentCountGL)
	)

	for mod := 0; mod < moduleNumber; mod++ {

		var (
			segmentCountGL  = instance.SegmentCountGL[mod]
			segmentCountLPP = instance.SegmentCountLPP[mod]
		)

		sumGL += int(segmentCountGL.Uint64())
		sumLPP += int(segmentCountLPP.Uint64())

		if !segmentCountGL.IsZero() || !segmentCountLPP.IsZero() {
			moduleIndex = mod
		}
	}

	switch {
	case sumGL+sumLPP > 1:
		return proofTypeConglo, 0
	case sumGL == 1:
		return proofTypeGL, moduleIndex
	case sumLPP == 1:
		return proofTypeLPP, moduleIndex
	}

	panic("unreachable")
}

func findVkPositionGnark(api frontend.API, instance LimitlessPublicInput[frontend.Variable, koalagnark.Ext]) frontend.Variable {
	var (
		sumGL, sumLPP = frontend.Variable(0), frontend.Variable(0)
		moduleIndex   = frontend.Variable(-1) // can't be -1 at the end of the "mod" loop.
		moduleNumber  = len(instance.SegmentCountGL)
	)

	for mod := 0; mod < moduleNumber; mod++ {

		var (
			segmentCountGL  = instance.SegmentCountGL[mod]
			segmentCountLPP = instance.SegmentCountLPP[mod]
		)

		sumGL = api.Add(sumGL, segmentCountGL)
		sumLPP = api.Add(sumLPP, segmentCountLPP)
		moduleIndex = api.Select(
			api.IsZero(api.Add(segmentCountGL, segmentCountLPP)),
			moduleIndex,
			frontend.Variable(mod),
		)
	}

	var (
		hasNothing = api.IsZero(
			api.Add(sumGL, sumLPP),
		)

		isGL = api.Mul(
			api.IsZero(sumLPP),
			api.IsZero(api.Sub(sumGL, 1)),
		)

		isLPP = api.Mul(
			api.IsZero(sumGL),
			api.IsZero(api.Sub(sumLPP, 1)),
		)
	)

	api.AssertIsEqual(hasNothing, 0)

	return api.Select(isGL, moduleIndex, // when isGL
		api.Select(isLPP, api.Add(moduleNumber, moduleIndex), // when isLPP
			2*moduleNumber, // when conglomeration
		),
	)
}

// getVerifyingKeyPair extracts the verifyingKeys from the compiled IOP.
func getVerifyingKeyPair(wiop *wizard.CompiledIOP) (vk0, vk1 field.Octuplet) {
	return wiop.ExtraData[VerifyingKeyPublicInput].(field.Octuplet),
		wiop.ExtraData[VerifyingKey2PublicInput].(field.Octuplet)
}

// scanFunctionalInputs returns a list of public inputs corresponding to
// functional inputs. The function works by looking up the public inputs whose
// name starts by the string "functional".
func scanFunctionalInputs(comp *wizard.CompiledIOP) []wizard.PublicInput {
	var res []wizard.PublicInput
	for _, pub := range comp.PublicInputs {
		if strings.HasPrefix(pub.Name, "functional.") {
			res = append(res, pub)
		}
	}
	return res
}

// assertCompatibleIOPs checks that all the compiled IOPs are compatible and
// can be aggregated within the same conglomeration.
func assertCompatibleIOPs(d *DistributedWizard) {
	w0 := d.CompiledConglomeration.RecursionCompKoala

	for i := range d.CompiledGLs {
		diff1, diff2 := cmpWizardIOP(w0, d.CompiledGLs[i].RecursionCompKoala)
		if len(diff1) > 0 || len(diff2) > 0 {
			dumpWizardIOP(w0, "conglomeration-debug/iop-conglo.csv")
			dumpWizardIOP(d.CompiledGLs[i].RecursionCompKoala, fmt.Sprintf("conglomeration-debug/iop-gl-%d.csv", i))
			utils.Panic("incompatible IOPs i=%v\n\t+++=%v\n\t---=%v", i, diff1, diff2)
		}
	}

	for i := range d.CompiledLPPs {
		diff1, diff2 := cmpWizardIOP(w0, d.CompiledLPPs[i].RecursionCompKoala)
		if len(diff1) > 0 || len(diff2) > 0 {
			dumpWizardIOP(w0, "conglomeration-debug/iop-conglomeration.csv")
			dumpWizardIOP(d.CompiledLPPs[i].RecursionCompKoala, fmt.Sprintf("conglomeration-debug/iop-lpp-%d.csv", i))
			utils.Panic("incompatible IOPs i=%v\n\t+++=%v\n\t---=%v", i, diff1, diff2)
		}
	}
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
