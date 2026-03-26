package serialization

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	cmimc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/cleanup"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/globalcs"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/horner"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/innerproduct"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/logderivativesum"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/mpts"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/permutation"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/plonkinwizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/stitchsplit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/univariates"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/bigrange"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/functionals"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/merkle"
	dmimc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/reedsolomon"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/selector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/internal/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/importpad"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak"
	gen_acc "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak/acc_module"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/keccak/base_conversion"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/packing"
	ded "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/packing/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/packing/dedicated/spaghettifier"
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
	RegisterImplementation(query.MiMC{})
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

	// Variables
	RegisterImplementation(variables.X{})
	RegisterImplementation(variables.PeriodicSample{})

	// Dedicated and common types
	RegisterImplementation(byte32cmp.MultiLimbCmp{})
	RegisterImplementation(byte32cmp.OneLimbCmpCtx{})
	RegisterImplementation(byte32cmp.DecompositionCtx{})
	RegisterImplementation(dedicated.IsZeroCtx{})
	RegisterImplementation(common.HashingCtx{})

	// Prover actions (added to fix missing concrete type warnings)
	RegisterImplementation(byte32cmp.Bytes32CmpProverAction{})
	RegisterImplementation(bigrange.BigRangeProverAction{})
	RegisterImplementation(ded.AssignPIPProverAction{})
	RegisterImplementation(keccak.ShakiraProverAction{})
	RegisterImplementation(vortex.ColumnAssignmentProverAction{})
	RegisterImplementation(vortex.LinearCombinationComputationProverAction{})

	// Smartvectors
	RegisterImplementation(smartvectors.Regular{})
	RegisterImplementation(smartvectors.PaddedCircularWindow{})
	RegisterImplementation(smartvectors.Constant{})
	RegisterImplementation(smartvectors.Pooled{})

	RegisterImplementation(stitchsplit.ProveRoundProverAction{})
	RegisterImplementation(stitchsplit.AssignLocalPointProverAction{})
	RegisterImplementation(stitchsplit.StitchColumnsProverAction{})
	RegisterImplementation(stitchsplit.StitchSubColumnsProverAction{})
	RegisterImplementation(stitchsplit.QueryVerifierAction{})
	RegisterImplementation(stitchsplit.SplitProverAction{})

	RegisterImplementation(cleanup.CleanupProverAction{})

	RegisterImplementation(dmimc.LinearHashProverAction{})
	RegisterImplementation(merkle.MerkleProofProverAction{})

	RegisterImplementation(univariates.NaturalizeProverAction{})
	RegisterImplementation(univariates.NaturalizeVerifierAction{})

	RegisterImplementation(gen_acc.GenericDataAccumulator{})
	RegisterImplementation(gen_acc.GenericInfoAccumulator{})

	RegisterImplementation(keccak.KeccakSingleProvider{})

	RegisterImplementation(packing.Packing{})

	RegisterImplementation(ded.LengthConsistencyCtx{})
	RegisterImplementation(ded.AccumulateUpToMaxCtx{})

	RegisterImplementation(spaghettifier.Spaghettification{})

	RegisterImplementation(importpad.Sha2Padder{})
	RegisterImplementation(importpad.MimcPadder{})
	RegisterImplementation(importpad.KeccakPadder{})
	RegisterImplementation(importpad.Importation{})
	RegisterImplementation(importpad.Importation{})

	RegisterImplementation(base_conversion.HashBaseConversion{})
	RegisterImplementation(base_conversion.BlockBaseConversion{})
	RegisterImplementation(base_conversion.DecompositionCtx{})

	RegisterImplementation(cleanup.CleanupProverAction{})
	RegisterImplementation(dummy.DummyVerifierAction{})
	RegisterImplementation(dummy.DummyProverAction{})

	RegisterImplementation(globalcs.EvaluationProver{})
	RegisterImplementation(globalcs.EvaluationVerifier{})
	RegisterImplementation(globalcs.QuotientCtx{})

	RegisterImplementation(horner.AssignHornerCtx{})
	RegisterImplementation(horner.AssignHornerIP{})
	RegisterImplementation(horner.AssignHornerQuery{})
	RegisterImplementation(horner.CheckHornerQuery{})
	RegisterImplementation(horner.CheckHornerResult{})

	RegisterImplementation(innerproduct.ProverTask{})
	RegisterImplementation(innerproduct.VerifierForSize{})

	RegisterImplementation(logderivativesum.AssignLogDerivativeSumProverAction{})
	RegisterImplementation(logderivativesum.CheckLogDerivativeSumMustBeZero{})
	RegisterImplementation(logderivativesum.ProverTaskAtRound{})
	RegisterImplementation(logderivativesum.FinalEvaluationCheck{})

	RegisterImplementation(mimc.MimcContext{})
	RegisterImplementation(mpts.QuotientAccumulation{})
	RegisterImplementation(mpts.RandomPointEvaluation{})
	RegisterImplementation(mpts.ShadowRowProverAction{})
	RegisterImplementation(mpts.VerifierAction{})

	RegisterImplementation(permutation.ProverTaskAtRound{})
	RegisterImplementation(permutation.AssignPermutationGrandProduct{})
	RegisterImplementation(permutation.FinalProductCheck{})
	RegisterImplementation(permutation.CheckGrandProductIsOne{})

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

	RegisterImplementation(cmimc.ExternalHasherBuilder{})
	RegisterImplementation(cmimc.ExternalHasherFactory{})

	RegisterImplementation(plonkinternal.CheckingActivators{})
	RegisterImplementation(plonkinternal.InitialBBSProverAction{})
	RegisterImplementation(plonkinternal.PlonkNoCommitProverAction{})
	RegisterImplementation(plonkinternal.LROCommitProverAction{})

	RegisterImplementation(fr.Element{})

	RegisterImplementation(dedicated.StackedColumn{})
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
// implementation within the registry.
//
// If the provided instance is a pointer type, then the registry will only store
// the base type of the instance.
//
// If the provided type is an interface or a pointer to interface, then the
// function will refuse.
//
// If the provided type was already registered, then this function is a no-op.
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

// returns a reflect.Type registered in the registry for the provided type-string
// the function will modify the provided string in case the string represents a
// pointer type and will add the levels of indirections to the returned
// reflect.Type.
func findRegisteredImplementation(pkgTypeName string) (reflect.Type, error) {

	// The typename may contain indirections. In that case, our goal is to
	// resolve the name of the concrete underlying type since this is the one
	// that is registered. The format of the registered name is a#b and the
	// format of the caller's pkgTypeName is a#b#n
	split := strings.Split(pkgTypeName, "#")
	if len(split) != 3 {
		return nil, fmt.Errorf("provided type `%v` does not have the format `a#b#n`", pkgTypeName)
	}

	var (
		registeredName     = split[0] + "#" + split[1]
		nbIndirection, err = strconv.ParseInt(split[2], 10, 64)
	)

	if err != nil {
		return nil, fmt.Errorf("could not parse the number of indirection from the string `%v` : %w", pkgTypeName, err)
	}

	// We need an explicit check because the getter panics. Also it makes for a
	// nicer error message.
	if !implementationRegistry.Exists(registeredName) {
		return nil, fmt.Errorf("unregistered type %s", registeredName)
	}

	foundType := implementationRegistry.MustGet(registeredName)

	// This readds the levels of indirection that the caller requested
	for i := int64(0); i < nbIndirection; i++ {
		foundType = reflect.PointerTo(foundType)
	}

	// This sanity-checks an invariant of the function. No matter what is
	// returned, it must return a type whose pkgPathTypeName matches the
	// requested one.
	if getPkgPathAndTypeNameIndirect(foundType) != pkgTypeName {
		utils.Panic("caller requested `%v` and got `%v`", pkgTypeName, getPkgPathAndTypeNameIndirect(foundType))
	}

	return foundType, nil
}

// Returns the full `<Type.PkgPath>#<Type.Name>#<nbIndirection>` of a type.
// Caller can either provide an instance of the desired type or a reflect.Type
// of it.
//
// The function only supports concrete named types or pointers to them. If the
// caller provides an interface, an anonymous (aside from pointers) type or
// pointers to them. The function will panic.
//
// This is used for naming the types that we would want to resolve. But this is
// not what is concretely registered. The reason for the difference is that
// we don't want to force the user to register every possible types AND their
// pointers.
func getPkgPathAndTypeNameIndirect(x any) string {

	refType := reflect.TypeOf(x)

	// If provided a reflect.Type, don't use the TypeOf of that. Instead directly
	// use the provided Type.
	if xAsRefType, ok := x.(reflect.Type); ok {
		refType = xAsRefType
	}

	nbIndirection := 0
	for refType.Kind() == reflect.Pointer {
		nbIndirection++
		refType = refType.Elem()
	}

	var (
		pkgPath  = refType.PkgPath()
		typeName = refType.Name()
	)

	if len(typeName) == 0 {
		utils.Panic("got an untyped parameter `(%T)(%v)`; this is not supported", x, x)
	}

	// The parenthesis are needed to ensure that the returned string is parseable
	return strings.TrimPrefix(pkgPath, pkgPathPrefixToRemove) + "#" + typeName + "#" + strconv.Itoa(nbIndirection)
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
