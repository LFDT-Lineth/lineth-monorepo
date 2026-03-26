//go:generate go run ../../cmd/dev-tools/codegen/main.go

// IMPORTANT:
// This file is an input to the interface registry code generator.
//
// Any change to this file — for example, registering a new implementation via
// RegisterImplementation — requires re-running the code generation step to keep
// the generated registry in sync.
//
// How to regenerate:
//   - From the repository root (prover/):  go generate ./...
//   - From this directory:                 go generate
//
// DO NOT rename or move this file.
// The codegen tool relies on its filename; if it must be changed, the codegen
// configuration must be updated accordingly.

package serde

import (
	"reflect"
	"strings"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/degreereduction"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/splitextension"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/stitchsplit"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/bigrange"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/emulated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	dposeidon2 "github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/reedsolomon"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/selector"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/bls"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecarith"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecdsa"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/ecpair"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/importpad"
	gen_acc "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/acc_module"
	keccak "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/glue"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing"
	zkded "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing/dedicated"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/packing/dedicated/spaghettifier"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/sha2"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/modexp"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/p256verify"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
)

func init() {
	// This registers all the types that we may need to deserialize
	// as interface implementations. Cases of interfaces that are relevant
	// are:
	//
	// 		- symbolic.Operator
	//      - symbolic.Metadata
	//      - ifaces.Column
	//  	- ifaces.Query
	//

	// The primitive types. Useful when serializing "any" objects
	RegisterImplementation(bool(false))
	RegisterImplementation(int8(0))
	RegisterImplementation(int16(0))
	RegisterImplementation(int32(0))
	RegisterImplementation(int64(0))
	RegisterImplementation(int(0))
	RegisterImplementation(uint8(0))
	RegisterImplementation(uint16(0))
	RegisterImplementation(uint32(0))
	RegisterImplementation(uint64(0))
	RegisterImplementation(uint(0))
	RegisterImplementation(string(""))

	// Interfaces
	RegisterImplementation(ifaces.ColID(""))
	RegisterImplementation(ifaces.QueryID(""))

	// Coins and Columns
	RegisterImplementation(column.Natural{})
	RegisterImplementation(column.Shifted{})
	RegisterImplementation(coin.Name(""))
	RegisterImplementation(coin.Info{})
	RegisterImplementation(column.FakeColumn{})

	// Verifier columns
	RegisterImplementation(verifiercol.ConstCol{})
	RegisterImplementation(verifiercol.FromYs{})
	RegisterImplementation(verifiercol.FromAccessors{})
	RegisterImplementation(verifiercol.ExpandedProofOrVerifyingKeyColWithZero{})
	RegisterImplementation(verifiercol.RepeatedAccessor{})

	// Queries
	RegisterImplementation(query.FixedPermutation{})
	RegisterImplementation(query.GlobalConstraint{})
	RegisterImplementation(query.Inclusion{})
	RegisterImplementation(query.InnerProduct{})
	RegisterImplementation(query.LocalConstraint{})
	RegisterImplementation(query.LocalOpening{})
	RegisterImplementation(query.Permutation{})
	RegisterImplementation(query.Range{})
	RegisterImplementation(query.UnivariateEval{})
	RegisterImplementation(query.Projection{})
	RegisterImplementation(query.PlonkInWizard{})
	RegisterImplementation(query.LocalOpening{})
	RegisterImplementation(query.LogDerivativeSum{})
	RegisterImplementation(query.GrandProduct{})
	RegisterImplementation(query.Horner{})
	RegisterImplementation(query.LocalOpeningParams{})
	RegisterImplementation(query.UnivariateEvalParams{})
	RegisterImplementation(query.GrandProductParams{})
	RegisterImplementation(query.InnerProductParams{})
	RegisterImplementation(query.LogDerivSumParams{})
	RegisterImplementation(query.Poseidon2{})

	// Symbolic
	RegisterImplementation(symbolic.Variable{})
	RegisterImplementation(symbolic.Constant{})
	RegisterImplementation(symbolic.Product{})
	RegisterImplementation(symbolic.LinComb{})
	RegisterImplementation(symbolic.PolyEval{})
	RegisterImplementation(symbolic.StringVar(""))

	// Accessors
	RegisterImplementation(accessors.FromCoinAccessor{})
	RegisterImplementation(accessors.FromConstAccessor{})
	RegisterImplementation(accessors.FromExprAccessor{})
	RegisterImplementation(accessors.FromIntVecCoinPositionAccessor{})
	RegisterImplementation(accessors.FromLocalOpeningYAccessor{})
	RegisterImplementation(accessors.FromPublicColumn{})
	RegisterImplementation(accessors.FromUnivXAccessor{})
	RegisterImplementation(accessors.FromLogDerivSumAccessor{})
	RegisterImplementation(accessors.FromGrandProductAccessor{})
	RegisterImplementation(accessors.FromHornerAccessorFinalValue{})
	RegisterImplementation(accessors.Extension{})

	// Variables
	RegisterImplementation(variables.X{})
	RegisterImplementation(variables.PeriodicSample{})

	// Circuit implementations
	RegisterImplementation(ecdsa.MultiEcRecoverCircuit{})
	RegisterImplementation(modexp.Modexp{})
	RegisterImplementation(modexp.Module{})
	RegisterImplementation(ecarith.MultiECAddCircuit{})
	RegisterImplementation(ecarith.MultiECMulCircuit{})
	RegisterImplementation(ecpair.MultiG2GroupcheckCircuit{})
	RegisterImplementation(ecpair.MultiMillerLoopMulCircuit{})
	RegisterImplementation(ecpair.MultiMillerLoopFinalExpCircuit{})
	RegisterImplementation(sha2.SHA2Circuit{})

	// Dedicated and common types
	RegisterImplementation(byte32cmp.MultiLimbCmp{})
	RegisterImplementation(byte32cmp.OneLimbCmpCtx{})
	RegisterImplementation(byte32cmp.DecompositionCtx{})
	RegisterImplementation(dedicated.IsZeroCtx{})
	RegisterImplementation(common.HashingCtx{})

	// Prover actions (added to fix missing concrete type warnings)
	RegisterImplementation(byte32cmp.Bytes32CmpProverAction{})
	RegisterImplementation(bigrange.BigRangeProverAction{})
	RegisterImplementation(keccak.ShakiraProverAction{})
	RegisterImplementation(vortex.ColumnAssignmentProverAction{})
	RegisterImplementation(vortex.LinearCombinationComputationProverAction{})

	// Smartvectors
	RegisterImplementation(smartvectors.Regular{})
	RegisterImplementation(smartvectors.RegularExt{})
	RegisterImplementation(smartvectors.PaddedCircularWindow{})
	RegisterImplementation(smartvectors.PaddedCircularWindowExt{})
	RegisterImplementation(smartvectors.Constant{})
	RegisterImplementation(smartvectors.ConstantExt{})

	RegisterImplementation(stitchsplit.ProveRoundProverAction{})
	RegisterImplementation(stitchsplit.AssignLocalPointProverAction{})
	RegisterImplementation(stitchsplit.StitchColumnsProverAction{})
	RegisterImplementation(stitchsplit.StitchSubColumnsProverAction{})
	RegisterImplementation(stitchsplit.QueryVerifierAction{})
	RegisterImplementation(stitchsplit.SplitProverAction{})
	RegisterImplementation(splitextension.AssignSplitColumnProverAction{})
	RegisterImplementation(cleanup.CleanupProverAction{})
	RegisterImplementation(dposeidon2.LinearHashProverAction{})
	RegisterImplementation(merkle.MerkleProofProverAction{})
	RegisterImplementation(univariates.NaturalizeProverAction{})
	RegisterImplementation(univariates.NaturalizeVerifierAction{})
	RegisterImplementation(gen_acc.GenericDataAccumulator{})
	RegisterImplementation(gen_acc.GenericInfoAccumulator{})
	RegisterImplementation(keccak.KeccakSingleProvider{})
	RegisterImplementation(packing.Packing{})
	RegisterImplementation(spaghettifier.Spaghettification{})
	RegisterImplementation(importpad.Sha2Padder{})
	RegisterImplementation(importpad.KeccakPadder{})
	RegisterImplementation(importpad.Importation{})
	RegisterImplementation(importpad.Importation{})
	RegisterImplementation(cleanup.CleanupProverAction{})
	RegisterImplementation(dummy.DummyVerifierAction{})
	RegisterImplementation(dummy.DummyProverAction{})
	RegisterImplementation(globalcs.EvaluationProver{})
	RegisterImplementation(globalcs.EvaluationVerifier{})
	RegisterImplementation(globalcs.QuotientCtx{})
	RegisterImplementation(globalcs.DegreeReductionStep{})
	RegisterImplementation(horner.AssignHornerCtx{})
	RegisterImplementation(horner.AssignHornerIP{})
	RegisterImplementation(horner.AssignHornerQuery{})
	RegisterImplementation(horner.CheckHornerQuery{})
	RegisterImplementation(horner.CheckHornerResult{})
	RegisterImplementation(innerproduct.ProverTask{})
	RegisterImplementation(innerproduct.VerifierForSize{})
	RegisterImplementation(innerproduct.ContextForSize{})
	RegisterImplementation(logderivativesum.AssignLogDerivativeSumProverAction{})
	RegisterImplementation(logderivativesum.CheckLogDerivativeSumMustBeZero{})
	RegisterImplementation(logderivativesum.ProverTaskAtRound{})
	RegisterImplementation(logderivativesum.FinalEvaluationCheck{})
	RegisterImplementation(logderivativesum.ZCtx{})
	RegisterImplementation(mpts.QuotientAccumulation{})
	RegisterImplementation(mpts.RandomPointEvaluation{})
	RegisterImplementation(mpts.ShadowRowProverAction{})
	RegisterImplementation(mpts.VerifierAction{})
	RegisterImplementation(permutation.ProverTaskAtRound{})
	RegisterImplementation(permutation.AssignPermutationGrandProduct{})
	RegisterImplementation(permutation.FinalProductCheck{})
	RegisterImplementation(permutation.CheckGrandProductIsOne{})
	RegisterImplementation(permutation.ZCtx{})
	RegisterImplementation(plonkinwizard.AssignSelOpening{})
	RegisterImplementation(plonkinwizard.CheckActivatorAndMask{})
	RegisterImplementation(plonkinwizard.CircAssignment{})
	RegisterImplementation(recursion.RecursionCircuit{})
	RegisterImplementation(recursion.AssignVortexOpenedCols{})
	RegisterImplementation(recursion.AssignVortexUAlpha{})
	RegisterImplementation(recursion.ConsistencyCheck{})
	RegisterImplementation(selfrecursion.ColSelectionProverAction{})
	RegisterImplementation(selfrecursion.CollapsingProverAction{})
	RegisterImplementation(selfrecursion.CollapsingVerifierAction{})
	RegisterImplementation(selfrecursion.ConsistencyYsUalphaVerifierAction{})
	RegisterImplementation(selfrecursion.FoldPhaseProverAction{})
	RegisterImplementation(selfrecursion.FoldPhaseVerifierAction{})
	RegisterImplementation(selfrecursion.LinearHashMerkleProverAction{})
	RegisterImplementation(selfrecursion.PreimageLimbsProverAction{})
	RegisterImplementation(stitchsplit.AssignLocalPointProverAction{})
	RegisterImplementation(stitchsplit.ProveRoundProverAction{})
	RegisterImplementation(stitchsplit.QueryVerifierAction{})
	RegisterImplementation(stitchsplit.SplitProverAction{})
	RegisterImplementation(stitchsplit.StitchColumnsProverAction{})
	RegisterImplementation(stitchsplit.StitchSubColumnsProverAction{})
	RegisterImplementation(univariates.NaturalizeProverAction{})
	RegisterImplementation(univariates.NaturalizeVerifierAction{})
	RegisterImplementation(vortex.Ctx{})
	RegisterImplementation(vortex.ColumnAssignmentProverAction{})
	RegisterImplementation(vortex.OpenSelectedColumnsProverAction{})
	RegisterImplementation(vortex.LinearCombinationComputationProverAction{})
	RegisterImplementation(vortex.VortexVerifierAction{})
	RegisterImplementation(vortex.ExplicitPolynomialEval{})
	RegisterImplementation(vortex.ShadowRowProverAction{})
	RegisterImplementation(vortex.ReassignPrecomputedRootAction{})
	RegisterImplementation(functionals.CoeffEvalProverAction{})
	RegisterImplementation(functionals.InterpolationProverAction{})
	RegisterImplementation(functionals.EvalBivariateProverAction{})
	RegisterImplementation(functionals.FoldProverAction{})
	RegisterImplementation(functionals.FoldOuterProverAction{})
	RegisterImplementation(functionals.FoldOuterVerifierAction{})
	RegisterImplementation(functionals.FoldVerifierAction{})
	RegisterImplementation(functionals.XYPow1MinNAccessor{})
	RegisterImplementation(reedsolomon.ReedSolomonProverAction{})
	RegisterImplementation(reedsolomon.ReedSolomonVerifierAction{})
	RegisterImplementation(column.FakeColumn{})
	RegisterImplementation(selector.SubsampleProverAction{})
	RegisterImplementation(selector.SubsampleVerifierAction{})
	RegisterImplementation(expr_handle.ExprHandleProverAction{})
	RegisterImplementation(plonkinternal.CheckingActivators{})
	RegisterImplementation(plonkinternal.CheckingActivators{})
	RegisterImplementation(plonkinternal.InitialBBSProverAction{})
	RegisterImplementation(plonkinternal.PlonkNoCommitProverAction{})
	RegisterImplementation(plonkinternal.LROCommitProverAction{})
	RegisterImplementation(fr.Element{})
	RegisterImplementation(dedicated.StackedColumn{})
	RegisterImplementation(distributed.AssignLPPQueries{})
	RegisterImplementation(distributed.SetInitialFSHash{})
	RegisterImplementation(distributed.CheckNxHash{})
	RegisterImplementation(distributed.StandardModuleDiscoverer{})
	RegisterImplementation(distributed.LppWitnessAssignment{})
	RegisterImplementation(distributed.ModuleGLAssignGL{})
	RegisterImplementation(distributed.ModuleGLAssignSendReceiveGlobal{})
	RegisterImplementation(distributed.ModuleGLCheckSendReceiveGlobal{})
	RegisterImplementation(distributed.LPPSegmentBoundaryCalculator{})
	RegisterImplementation(distributed.ConglomerationHierarchicalVerifierAction{})
	RegisterImplementation(distributed.AssignManualShifts{})
	RegisterImplementation(poseidon2.Poseidon2Context{})
	RegisterImplementation(splitextension.VerifierCtx{})
	RegisterImplementation(splitextension.AssignUnivProverAction{})

	// G1 circuit types
	RegisterImplementation(bls.MultiAddCircuitG1{})
	RegisterImplementation(bls.MultiMulCircuitG1{})
	RegisterImplementation(bls.MultiMapCircuitG1{})
	RegisterImplementation(bls.MultiCheckableG1NonGroup{})
	RegisterImplementation(bls.MultiCheckableG1NonCurve{})

	// G2 circuit types
	RegisterImplementation(bls.MultiAddCircuitG2{})
	RegisterImplementation(bls.MultiMulCircuitG2{})
	RegisterImplementation(bls.MultiMapCircuitG2{})
	RegisterImplementation(bls.MultiCheckableG2NonGroup{})
	RegisterImplementation(bls.MultiCheckableG2NonCurve{})

	// Non-generic pairing and point eval circuits
	RegisterImplementation(bls.MultiMillerLoopMulCircuit{})
	RegisterImplementation(bls.MultiMillerLoopFinalExpCircuit{})
	RegisterImplementation(bls.MultiPointEvalCircuit{})
	RegisterImplementation(bls.MultiPointEvalFailureCircuit{})

	RegisterImplementation(p256verify.MultiP256VerifyInstanceCircuit{})
	RegisterImplementation(zkded.AssignPIPProverAction{})
	RegisterImplementation(zkded.LengthConsistencyCtx{})
	RegisterImplementation(zkded.AccumulateUpToMaxCtx{})
	RegisterImplementation(byte32cmp.MultiLimbAdd{})
	RegisterImplementation(limbs.LimbsLittleEndian{})
	RegisterImplementation(limbs.LimbsBigEndian{})
	RegisterImplementation(emulated.AssignEmulatedColumnsProverAction{})
	RegisterImplementation(emulated.MultiplicationAssignmentProverAction{})
	RegisterImplementation(common.TwoByTwoCombination{})
	RegisterImplementation(dummy.DoneOperation{})
	RegisterImplementation(dummy.DoneOperationSet{})

	RegisterImplementation(degreereduction.DegreeReductionStep{})

	// Public input extractor (stored in CompiledIOP.ExtraData as any)
	RegisterImplementation(publicInput.FunctionalInputExtractor{})
	RegisterImplementation(koalabear.Element{})
	RegisterImplementation(field.Element{})
}

// In order to save some space, we trim the prefix of the package path as this
// is repetitive.
const pkgPathPrefixToRemove = "github.com/consensys/linea-monorepo/prover"

// implementationRegistry maps a string representing a string of the form
// `path/to/package#ImplementingStruct` to a [reflect.Type]
// of the struct `ImplementingStruct` where the struct can be anything we would
// like to potentially unmarshal.
var implementationRegistry = collection.NewMapping[string, reflect.Type]()

// RegisterImplementation registers the type of the provided instance. This is
// needed if the caller of the package wants to deserialize into an interface
// type. When that happens, the deserializer needs to know which type to
// concretely deserialize into. This is achieved by looking up the interface
// implementation within the registry. If the provided instance is a pointer type,
// then the registry will only store the base type of the instance.
// If the provided type is an interface or a pointer to interface, then the
// function will refuse. If the provided type was already registered, then this function is a no-op.
//
// IMPORTANT: DO NOT CHANGE the name of this function since the codegen tool depends on it.
// Incase it is changed, the codegen tool must be updated. This function is used by the codegen tool
// and runs only during the compile time and not at runtime.
func RegisterImplementation(instance any) {

	if instance == nil {
		// If nil is provided, we cannot neither derive an underlying type nor
		// use the result of the constructor as a receiver to deserialize an
		// instance.
		utils.Panic("The constructor returned nil")
	}

	// Using a loop here ensures that all the required levels of indirections are
	// done so that we are sure we don't register a pointer-type.
	typeOfInstance := reflect.TypeOf(instance)
	for typeOfInstance.Kind() == reflect.Pointer {
		typeOfInstance = typeOfInstance.Elem()
	}

	if len(typeOfInstance.Name()) == 0 {
		utils.Panic("unsupported type of instance: %T", instance)
	}

	registeredTypeName := getPkgPathAndTypeName(instance)

	if implementationRegistry.Exists(registeredTypeName) {
		return
	}

	implementationRegistry.InsertNew(registeredTypeName, reflect.TypeOf(instance))
}

// Returns the name in the format <pkg>#<type>. Used to derive the type key in
// the register. It has the following restrictions:
//   - Requires that provided type is concrete
//   - If it's a [reflect.Type] this is fine.
func getPkgPathAndTypeName(x any) string {
	refType := reflect.TypeOf(x)

	// If provided a reflect.Type, don't use the TypeOf of that. Instead directly
	// use the provided Type.
	if xAsRefType, ok := x.(reflect.Type); ok {
		refType = xAsRefType
	}

	var (
		pkgPath  = refType.PkgPath()
		typeName = refType.Name()
	)

	if len(typeName) == 0 {
		// If the type is not named, we won't be able to resolve it as an
		// interface. We still return a reflect.Repr as it is still useful in
		// case it is used for packing a struct object.
		return refType.String()
	}
	return strings.TrimPrefix(pkgPath, pkgPathPrefixToRemove) + "#" + typeName
}
