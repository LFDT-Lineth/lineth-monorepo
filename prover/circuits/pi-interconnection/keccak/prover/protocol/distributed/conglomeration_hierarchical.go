package distributed

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/consensys/gnark/frontend"
	gmimc "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
)

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
	PublicInputs LimitlessPublicInput[wizard.PublicInput]
	// VerificationKeyMerkleProofs is the list of the verification keys proving
	// the membership of the verifying keys of the instances inside the
	// VerificationKeyMerkleTree. Each merkle proof is structured as a list of
	// D columns if size 1 where D is the depth of the merkle tree.
	VerificationKeyMerkleProofs [][]ifaces.Column
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
	Tree             *smt.Tree
	VerificationKeys [][2]field.Element
}

// ConglomerationHierarchicalVerifierAction implements the [wizard.VerifierAction]
// interface for the conglomeration proof. It checks the consistency of the
// public inputs with the children instance's public inputs.
type ConglomerationHierarchicalVerifierAction struct {
	ModuleConglo
}

// LimitlessPublicInput stores the columns totalling the
// public inputs of a conglomeration node.
type LimitlessPublicInput[T any] struct {
	Functionals                  []T
	TargetNbSegments             []T
	SegmentCountGL               []T
	SegmentCountLPP              []T
	GeneralMultiSetHash          []T
	SharedRandomnessMultiSetHash []T
	VKeyMerkleRoot               T
	VerifyingKey                 [2]T
	LogDerivativeSum             T
	HornerSum                    T
	GrandProduct                 T
	SharedRandomness             T
}

// buildVerificationKeyMerkleTree builds the verification key merkle tree.
func buildVerificationKeyMerkleTree(moduleGL, moduleLPP []*RecursedSegmentCompilation, hierAgg *RecursedSegmentCompilation) VerificationKeyMerkleTree {

	var (
		leaves           = make([]types.Bytes32, 0, len(moduleGL)+len(moduleLPP)+1)
		verificationKeys = make([][2]field.Element, 0, len(moduleGL)+len(moduleLPP)+1)
		vkList           = ""
	)

	appendLeaf := func(comp *wizard.CompiledIOP) {
		var (
			vk0, vk1 = getVerifyingKeyPair(comp)
			leafF    = mimc.HashVec([]field.Element{vk0, vk1})
			leaf     types.Bytes32
		)

		leaf.SetField(leafF)
		leaves = append(leaves, leaf)
		verificationKeys = append(verificationKeys, [2]field.Element{vk0, vk1})

		vkList += fmt.Sprintf("\t%v %v\n", vk0.String(), vk1.String())
	}

	for _, module := range moduleGL {
		appendLeaf(module.RecursionComp)
	}

	for _, module := range moduleLPP {
		appendLeaf(module.RecursionComp)
	}

	appendLeaf(hierAgg.RecursionComp)

	// padding with zeroes so that the leaves number if a power-of-two
	paddedSize := utils.NextPowerOfTwo(len(leaves))
	for i := len(leaves); i < paddedSize; i++ {
		leaves = append(leaves, types.Bytes32{})
	}

	return VerificationKeyMerkleTree{
		Tree:             smt.BuildComplete(leaves, hashtypes.MiMC),
		VerificationKeys: verificationKeys,
	}
}

// GetVkMerkleProof return the merkle proof of a verification key
func (vmt VerificationKeyMerkleTree) GetVkMerkleProof(segProof SegmentProof) []field.Element {

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
	res := make([]field.Element, len(proof.Siblings))
	for i, sibling := range proof.Siblings {
		res[i].SetBytes(sibling[:])
	}

	fmt.Printf(
		"[getMerkleProof] leaf position: %v, root: %v, leaf: %v, vk: %v\n",
		leafPosition, vmt.Tree.Root.Hex(), vmt.Tree.OccupiedLeaves[leafPosition].Hex(),
		vector.Prettify(vmt.VerificationKeys[leafPosition][:]))

	return res
}

// GetRoot returns the root of the verification key merkle tree encoded as a
// field element.
func (vmt VerificationKeyMerkleTree) GetRoot() field.Element {
	root := vmt.Tree.Root
	var rootF field.Element
	rootF.SetBytes(root[:])
	return rootF
}

// CheckMembership checks if a verification key is in the merkle tree.
func checkVkMembership(t ProofType, numModule int, moduleIndex int, vk [2]field.Element, rootF field.Element, proofF []field.Element) error {

	var leafPosition = -1

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
		root        types.Bytes32
		mProof      = smt.Proof{
			Path:     leafPosition,
			Siblings: make([]types.Bytes32, merkleDepth),
		}
		smtCfg = &smt.Config{HashFunc: hashtypes.MiMC, Depth: merkleDepth}
		leafF  = mimc.HashVec(vk[:])
		leaf   = types.Bytes32{}
	)

	if merkleDepth != len(proofF) {
		panic("merkleDepth != len(proofF)")
	}

	leaf.SetField(leafF)

	for lvl := 0; lvl < merkleDepth; lvl++ {
		mProof.Siblings[lvl].SetField(proofF[lvl])
	}

	root.SetField(rootF)

	fmt.Printf("verified VK merkle proof: %v, moduleIndex: %v, proofType: %v, leaf: %v, root: %v", leafPosition, moduleIndex, t, leaf.Hex(), root.Hex())

	if !mProof.Verify(smtCfg, leaf, root) {
		return fmt.Errorf("VK is not a member of the tree: pos: %v, moduleIndex: %v, proofType: %v, leaf: %v, root: %v", leafPosition, moduleIndex, t, leaf.Hex(), root.Hex())
	}

	return nil
}

// CheckMembershipGnark checks if a verification key is in the merkle tree.
func checkVkMembershipGnark(
	api frontend.API,
	leafPosition frontend.Variable,
	numModule int,
	vk [2]frontend.Variable,
	root frontend.Variable,
	proofF []frontend.Variable,
) {

	// This part of the loop checks the membership of the VK as a member of
	// the tree using the leafPosition from above.

	var (
		merkleDepth = utils.Log2Ceil(2*numModule + 1)
		mProof      = smt.GnarkProof{
			Path:     leafPosition,
			Siblings: proofF,
		}
		leaf = mimc.GnarkHashVec(api, vk[:])
	)

	if merkleDepth != len(proofF) {
		panic("merkleDepth != len(proofF)")
	}

	h, err := gmimc.NewMiMC(api)
	if err != nil {
		panic(err)
	}

	smt.GnarkVerifyMerkleProof(api, mProof, leaf, root, &h)
}

// Conglomerate runs the conglomeration compiler and returns a pointer to the
// receiver of the method.
func (d *DistributedWizard) Conglomerate(params CompilationParams) *DistributedWizard {

	conglo := &ModuleConglo{
		ModuleNumber: len(d.CompiledGLs),
	}

	comp := wizard.NewCompiledIOP()
	conglo.Compile(comp, d.CompiledGLs[0].RecursionComp)
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

	c.Recursion = recursion.DefineRecursionOf(comp, moduleMod, recursion.Parameters{
		Name:                   "conglomeration",
		WithoutGkr:             true,
		MaxNumProof:            2,
		WithExternalHasherOpts: true,
	})

	c.Wiop = comp

	for _, pi := range scanFunctionalInputs(moduleMod) {
		c.PublicInputs.Functionals = append(c.PublicInputs.Functionals, declarePiColumn(c.Wiop, pi.Name))
	}

	c.PublicInputs.TargetNbSegments = declareListOfPiColumns(c.Wiop, 0, TargetNbSegmentPublicInputBase, c.ModuleNumber)
	c.PublicInputs.SegmentCountGL = declareListOfPiColumns(c.Wiop, 0, SegmentCountGLPublicInputBase, c.ModuleNumber)
	c.PublicInputs.SegmentCountLPP = declareListOfPiColumns(c.Wiop, 0, SegmentCountLPPPublicInputBase, c.ModuleNumber)
	c.PublicInputs.GeneralMultiSetHash = declareListOfPiColumns(c.Wiop, 0, GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
	c.PublicInputs.SharedRandomnessMultiSetHash = declareListOfPiColumns(c.Wiop, 0, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize)
	c.PublicInputs.LogDerivativeSum = declarePiColumn(c.Wiop, LogDerivativeSumPublicInput)
	c.PublicInputs.HornerSum = declarePiColumn(c.Wiop, HornerPublicInput)
	c.PublicInputs.GrandProduct = declarePiColumn(c.Wiop, GrandProductPublicInput)
	c.PublicInputs.SharedRandomness = declarePiColumn(c.Wiop, InitialRandomnessPublicInput)
	c.PublicInputs.VKeyMerkleRoot = declarePiColumn(c.Wiop, VerifyingKeyMerkleRootPublicInput)

	// vkMerkleTreeDepth is the depth of the verification key merkle tree
	vkMerkleTreeDepth := c.VKeyMTreeDepth()
	c.VerificationKeyMerkleProofs = make([][]ifaces.Column, c.ModuleNumber)
	for i := 0; i < aggregationArity; i++ {
		for j := 0; j < vkMerkleTreeDepth; j++ {
			col := comp.InsertProof(0, ifaces.ColID(fmt.Sprintf("vkMerkleProof_%d_%d", i, j)), 1)
			c.VerificationKeyMerkleProofs[i] = append(c.VerificationKeyMerkleProofs[i], col)
		}
	}

	comp.RegisterVerifierAction(0, &ConglomerationHierarchicalVerifierAction{ModuleConglo: *c})
}

// Run implements the [wizard.VerifierAction] for the
// ConglomerationHierarchicalVerifierAction.
func (c *ConglomerationHierarchicalVerifierAction) Run(run wizard.Runtime) error {

	var (
		err          error
		collectedPIs = [aggregationArity]LimitlessPublicInput[field.Element]{}
		topPIs       = collectAllPublicInputs(run)
	)

	for instance := 0; instance < aggregationArity; instance++ {
		collectedPIs[instance] = c.collectAllPublicInputsOfInstance(run, instance)
	}

	// This checks that the functional public inputs are correctly conglomerated
	// across all instances.
	for k := range topPIs.Functionals {

		summedUpValue := field.Element{}

		for instance := 0; instance < aggregationArity; instance++ {
			funcPI := collectedPIs[instance].Functionals[k]
			summedUpValue.Add(&summedUpValue, &funcPI)
		}

		if summedUpValue != topPIs.Functionals[k] {
			err = errors.Join(err, fmt.Errorf("public input mismatch for Functionals at index %d, name=%v", k, c.PublicInputs.Functionals[k].Name))
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
				err = errors.Join(err, fmt.Errorf("public input mismatch for TargetNbSegments at instance %d", instance))
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
	for k := 0; k < mimc.MSetHashSize; k++ {

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
		accGrandProduct = field.One()
		accLogDeriv     = field.Zero()
		accHornerSum    = field.Zero()
	)

	for instance := 0; instance < aggregationArity; instance++ {

		// This agglomerates the horner N0 hash checker, the grand product, the
		// log derivative sum and the horner sum.
		accGrandProduct.Mul(&accGrandProduct, &collectedPIs[instance].GrandProduct)
		accLogDeriv.Add(&accLogDeriv, &collectedPIs[instance].LogDerivativeSum)
		accHornerSum.Add(&accHornerSum, &collectedPIs[instance].HornerSum)

		if !collectedPIs[instance].SharedRandomness.IsZero() && collectedPIs[instance].SharedRandomness != topPIs.SharedRandomness {
			err = errors.Join(err, fmt.Errorf("public input mismatch for SharedRandomness for instance %d", instance))
		}

		if collectedPIs[instance].VKeyMerkleRoot != topPIs.VKeyMerkleRoot {
			err = errors.Join(err, fmt.Errorf("public input mismatch for VKeyMerkleRoot for instance %d, sub-value=%v, top-value=%v",
				instance, collectedPIs[instance].VKeyMerkleRoot.String(), topPIs.VKeyMerkleRoot.String(),
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

		mProof := make([]field.Element, c.ModuleConglo.VKeyMTreeDepth())
		for i := range mProof {
			mProof[i] = c.VerificationKeyMerkleProofs[instance][i].GetColAssignmentAt(run, 0)
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

	var (
		collectedPIs = [aggregationArity]LimitlessPublicInput[frontend.Variable]{}
		topPIs       = c.collectAllPublicInputsGnark(api, run)
		hasher       = run.GetHasherFactory().NewHasher()
	)

	for instance := 0; instance < aggregationArity; instance++ {
		collectedPIs[instance] = c.collectAllPublicInputsOfInstanceGnark(api, run, instance)
	}

	// This checks that the functional public inputs are correctly conglomerated
	// across all instances.
	for k := range topPIs.Functionals {
		summedUpValue := frontend.Variable(0)
		for instance := 0; instance < aggregationArity; instance++ {
			funcPI := collectedPIs[instance].Functionals[k]
			summedUpValue = api.Add(summedUpValue, funcPI)
		}
		api.AssertIsEqual(summedUpValue, topPIs.Functionals[k])
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
		generalSum = mimc.EmptyMSetHashGnark(hasher)
		sharedSum  = mimc.EmptyMSetHashGnark(hasher)
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
		accGrandProduct = frontend.Variable(1)
		accLogDeriv     = frontend.Variable(0)
		accHornerSum    = frontend.Variable(0)
	)

	for instance := 0; instance < aggregationArity; instance++ {

		// This agglomerates the horner N0 hash checker, the grand product, the
		// log derivative sum and the horner sum.
		accGrandProduct = api.Mul(accGrandProduct, collectedPIs[instance].GrandProduct)
		accLogDeriv = api.Add(accLogDeriv, collectedPIs[instance].LogDerivativeSum)
		accHornerSum = api.Add(accHornerSum, collectedPIs[instance].HornerSum)

		api.AssertIsEqual(
			api.Mul(
				api.Sub(1, api.IsZero(collectedPIs[instance].SharedRandomness)),
				api.Sub(collectedPIs[instance].SharedRandomness, topPIs.SharedRandomness),
			),
			0,
		)

		api.AssertIsEqual(collectedPIs[instance].VKeyMerkleRoot, topPIs.VKeyMerkleRoot)
	}

	api.AssertIsEqual(accGrandProduct, topPIs.GrandProduct)
	api.AssertIsEqual(accLogDeriv, topPIs.LogDerivativeSum)
	api.AssertIsEqual(accHornerSum, topPIs.HornerSum)

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
		mProof := make([]frontend.Variable, c.ModuleConglo.VKeyMTreeDepth())
		for i := range mProof {
			mProof[i] = c.VerificationKeyMerkleProofs[instance][i].GetColAssignmentGnarkAt(run, 0)
		}

		checkVkMembershipGnark(
			api,
			leafPosition,
			c.ModuleNumber,
			collectedPIs[instance].VerifyingKey,
			collectedPIs[instance].VKeyMerkleRoot,
			mProof,
		)
	}
}

// declarePi declares a column with the requested name as proof column and length
// one and also declare a public input from that column with the same provided
// name.
func declarePiColumn(comp *wizard.CompiledIOP, name string) wizard.PublicInput {
	return declarePiColumnAtRound(comp, 0, name)
}

// declarePiColumn at round declares a column at the requested round to generate
// a public input with the requested name.
func declarePiColumnAtRound(comp *wizard.CompiledIOP, round int, name string) wizard.PublicInput {
	col := comp.InsertProof(round, ifaces.ColID(name+"_PI_COLUMN"), 1)
	return comp.InsertPublicInput(name, accessors.NewFromPublicColumn(col, 0))
}

// assignPiColumn assigns the column of a public input with the requested name
// to the provided column.
func assignPiColumn(run *wizard.ProverRuntime, name string, val field.Element) {
	run.AssignColumn(
		ifaces.ColID(name+"_PI_COLUMN"),
		smartvectors.NewConstant(val, 1),
	)
}

// declareListOfPiColumns declares a list of columns with the requested name as
// proof columns and length provided.
func declareListOfPiColumns(comp *wizard.CompiledIOP, round int, name string, length int) []wizard.PublicInput {
	var cols []wizard.PublicInput
	for i := 0; i < length; i++ {
		cols = append(cols, declarePiColumnAtRound(comp, round, name+"_"+strconv.Itoa(i)))
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
		res = append(res, run.GetPublicInput(name))
	}
	return res
}

// getPublicInputListOfInstance returns a list of public inputs of the provided
// name and instance.
func getPublicInputListOfInstance(rec *recursion.Recursion, run wizard.Runtime, name string, instance int, nb int) []field.Element {
	var res []field.Element
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		res = append(res, rec.GetPublicInputOfInstance(run, name, instance))
	}
	return res
}

// GetPublicInputListGnark is as [getPublicInputList] but for gnark.
func GetPublicInputListGnark(api frontend.API, run wizard.GnarkRuntime, name string, nb int) []frontend.Variable {
	var res []frontend.Variable
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		res = append(res, run.GetPublicInput(api, name))
	}
	return res
}

// getPublicInputListOfInstance returns a list of public inputs of the provided
// name and instance.
func getPublicInputListOfInstanceGnark(rec *recursion.Recursion, api frontend.API, run wizard.GnarkRuntime, name string, instance int, nb int) []frontend.Variable {
	var res []frontend.Variable
	for i := 0; i < nb; i++ {
		name := name + "_" + strconv.Itoa(i)
		res = append(res, rec.GetPublicInputOfInstanceGnark(api, run, name, instance))
	}
	return res
}

// collectAllPublicInputsOfInstance returns a structured object representing
// the public inputs of the given instance.
func (c ModuleConglo) collectAllPublicInputsOfInstance(run wizard.Runtime, instance int) LimitlessPublicInput[field.Element] {

	res := LimitlessPublicInput[field.Element]{
		TargetNbSegments:             getPublicInputListOfInstance(c.Recursion, run, TargetNbSegmentPublicInputBase, instance, c.ModuleNumber),
		SegmentCountGL:               getPublicInputListOfInstance(c.Recursion, run, SegmentCountGLPublicInputBase, instance, c.ModuleNumber),
		SegmentCountLPP:              getPublicInputListOfInstance(c.Recursion, run, SegmentCountLPPPublicInputBase, instance, c.ModuleNumber),
		GeneralMultiSetHash:          getPublicInputListOfInstance(c.Recursion, run, GeneralMultiSetPublicInputBase, instance, mimc.MSetHashSize),
		SharedRandomnessMultiSetHash: getPublicInputListOfInstance(c.Recursion, run, SharedRandomnessMultiSetPublicInputBase, instance, mimc.MSetHashSize),
		LogDerivativeSum:             c.Recursion.GetPublicInputOfInstance(run, LogDerivativeSumPublicInput, instance),
		HornerSum:                    c.Recursion.GetPublicInputOfInstance(run, HornerPublicInput, instance),
		GrandProduct:                 c.Recursion.GetPublicInputOfInstance(run, GrandProductPublicInput, instance),
		SharedRandomness:             c.Recursion.GetPublicInputOfInstance(run, InitialRandomnessPublicInput, instance),
		VKeyMerkleRoot:               c.Recursion.GetPublicInputOfInstance(run, VerifyingKeyMerkleRootPublicInput, instance),
		VerifyingKey: [2]field.Element{
			c.Recursion.GetPublicInputOfInstance(run, VerifyingKeyPublicInput, instance),
			c.Recursion.GetPublicInputOfInstance(run, VerifyingKey2PublicInput, instance),
		},
	}

	for _, pi := range c.PublicInputs.Functionals {
		res.Functionals = append(res.Functionals, c.Recursion.GetPublicInputOfInstance(run, pi.Name, instance))
	}

	return res
}

// collectAllPublicInputs returns a structured object representing the public
// inputs of all the instances.
func collectAllPublicInputs(run wizard.Runtime) LimitlessPublicInput[field.Element] {

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

	res := LimitlessPublicInput[field.Element]{
		TargetNbSegments:             GetPublicInputList(run, TargetNbSegmentPublicInputBase, moduleNumber),
		SegmentCountGL:               GetPublicInputList(run, SegmentCountGLPublicInputBase, moduleNumber),
		SegmentCountLPP:              GetPublicInputList(run, SegmentCountLPPPublicInputBase, moduleNumber),
		GeneralMultiSetHash:          GetPublicInputList(run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize),
		SharedRandomnessMultiSetHash: GetPublicInputList(run, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize),
		LogDerivativeSum:             run.GetPublicInput(LogDerivativeSumPublicInput),
		HornerSum:                    run.GetPublicInput(HornerPublicInput),
		GrandProduct:                 run.GetPublicInput(GrandProductPublicInput),
		SharedRandomness:             run.GetPublicInput(InitialRandomnessPublicInput),
		VKeyMerkleRoot:               run.GetPublicInput(VerifyingKeyMerkleRootPublicInput),
	}

	for _, pi := range scanFunctionalInputs(run.GetSpec()) {
		res.Functionals = append(res.Functionals, run.GetPublicInput(pi.Name))
	}

	return res
}

// collectAllPublicInputsOfInstanceGnark returns a structured object representing
// the public inputs of the given instance.
func (c ModuleConglo) collectAllPublicInputsOfInstanceGnark(api frontend.API, run wizard.GnarkRuntime, instance int) LimitlessPublicInput[frontend.Variable] {

	res := LimitlessPublicInput[frontend.Variable]{
		TargetNbSegments:             getPublicInputListOfInstanceGnark(c.Recursion, api, run, TargetNbSegmentPublicInputBase, instance, c.ModuleNumber),
		SegmentCountGL:               getPublicInputListOfInstanceGnark(c.Recursion, api, run, SegmentCountGLPublicInputBase, instance, c.ModuleNumber),
		SegmentCountLPP:              getPublicInputListOfInstanceGnark(c.Recursion, api, run, SegmentCountLPPPublicInputBase, instance, c.ModuleNumber),
		GeneralMultiSetHash:          getPublicInputListOfInstanceGnark(c.Recursion, api, run, GeneralMultiSetPublicInputBase, instance, mimc.MSetHashSize),
		SharedRandomnessMultiSetHash: getPublicInputListOfInstanceGnark(c.Recursion, api, run, SharedRandomnessMultiSetPublicInputBase, instance, mimc.MSetHashSize),
		LogDerivativeSum:             c.Recursion.GetPublicInputOfInstanceGnark(api, run, LogDerivativeSumPublicInput, instance),
		HornerSum:                    c.Recursion.GetPublicInputOfInstanceGnark(api, run, HornerPublicInput, instance),
		GrandProduct:                 c.Recursion.GetPublicInputOfInstanceGnark(api, run, GrandProductPublicInput, instance),
		SharedRandomness:             c.Recursion.GetPublicInputOfInstanceGnark(api, run, InitialRandomnessPublicInput, instance),
		VKeyMerkleRoot:               c.Recursion.GetPublicInputOfInstanceGnark(api, run, VerifyingKeyMerkleRootPublicInput, instance),
		VerifyingKey: [2]frontend.Variable{
			c.Recursion.GetPublicInputOfInstanceGnark(api, run, VerifyingKeyPublicInput, instance),
			c.Recursion.GetPublicInputOfInstanceGnark(api, run, VerifyingKey2PublicInput, instance),
		},
	}

	for _, pi := range c.PublicInputs.Functionals {
		res.Functionals = append(res.Functionals, c.Recursion.GetPublicInputOfInstanceGnark(api, run, pi.Name, instance))
	}

	return res
}

// collectAllPublicInputsGnark returns a structured object representing the public
// inputs of all the instances.
//
// In the returned object, the verifying key public inputs are not populated.
func (c ModuleConglo) collectAllPublicInputsGnark(api frontend.API, run wizard.GnarkRuntime) LimitlessPublicInput[frontend.Variable] {

	res := LimitlessPublicInput[frontend.Variable]{
		TargetNbSegments:             GetPublicInputListGnark(api, run, TargetNbSegmentPublicInputBase, c.ModuleNumber),
		SegmentCountGL:               GetPublicInputListGnark(api, run, SegmentCountGLPublicInputBase, c.ModuleNumber),
		SegmentCountLPP:              GetPublicInputListGnark(api, run, SegmentCountLPPPublicInputBase, c.ModuleNumber),
		GeneralMultiSetHash:          GetPublicInputListGnark(api, run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize),
		SharedRandomnessMultiSetHash: GetPublicInputListGnark(api, run, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize),
		LogDerivativeSum:             run.GetPublicInput(api, LogDerivativeSumPublicInput),
		HornerSum:                    run.GetPublicInput(api, HornerPublicInput),
		GrandProduct:                 run.GetPublicInput(api, GrandProductPublicInput),
		SharedRandomness:             run.GetPublicInput(api, InitialRandomnessPublicInput),
		VKeyMerkleRoot:               run.GetPublicInput(api, VerifyingKeyMerkleRootPublicInput),
	}

	for _, pi := range scanFunctionalInputs(c.Recursion.InputCompiledIOP) {
		res.Functionals = append(res.Functionals, run.GetPublicInput(api, pi.Name))
	}

	return res
}

// VKeyMTreeDepth returns the depth of the verification key merkle tree.
func (c ModuleConglo) VKeyMTreeDepth() int {
	return utils.Log2Ceil(2*c.ModuleNumber + 1)
}

// findProofTypeAndModule returns the proofType and the module index of the
// provided instance given collected public inputs of the instances.
func findProofTypeAndModule(instance LimitlessPublicInput[field.Element]) (ProofType, int) {

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

func findVkPositionGnark(api frontend.API, instance LimitlessPublicInput[frontend.Variable]) frontend.Variable {

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
func getVerifyingKeyPair(wiop *wizard.CompiledIOP) (vkGL, vkLPP field.Element) {
	return wiop.ExtraData[VerifyingKeyPublicInput].(field.Element),
		wiop.ExtraData[VerifyingKey2PublicInput].(field.Element)
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

	w0 := d.CompiledConglomeration.RecursionComp

	for i := range d.CompiledGLs {
		diff1, diff2 := cmpWizardIOP(w0, d.CompiledGLs[i].RecursionComp)
		if len(diff1) > 0 || len(diff2) > 0 {
			dumpWizardIOP(w0, "conglomeration-debug/iop-conglo.csv")
			dumpWizardIOP(d.CompiledGLs[i].RecursionComp, fmt.Sprintf("conglomeration-debug/iop-gl-%d.csv", i))
			utils.Panic("incompatible IOPs i=%v\n\t+++=%v\n\t---=%v", i, diff1, diff2)
		}
	}

	for i := range d.CompiledLPPs {
		diff1, diff2 := cmpWizardIOP(w0, d.CompiledLPPs[i].RecursionComp)
		if len(diff1) > 0 || len(diff2) > 0 {
			dumpWizardIOP(w0, "conglomeration-debug/iop-conglomeration.csv")
			dumpWizardIOP(d.CompiledLPPs[i].RecursionComp, fmt.Sprintf("conglomeration-debug/iop-lpp-%d.csv", i))
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
