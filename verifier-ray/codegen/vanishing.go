package codegen

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
)

// UnsupportedExpressionError reports an expression leaf or operation that the
// first verifier-ray vanishing checker intentionally does not evaluate yet.
type UnsupportedExpressionError struct {
	Type string
}

func (e *UnsupportedExpressionError) Error() string {
	return fmt.Sprintf("unsupported vanishing expression %s", e.Type)
}

func IsUnsupportedExpression(err error) bool {
	var unsupported *UnsupportedExpressionError
	return errors.As(err, &unsupported)
}

type VanishingSystem struct {
	SourceName          string
	Modules             []VanishingModule
	DynamicModuleCount  int
	TotalWitnessClaims  int
	TotalQuotientClaims int
}

type ModuleSize struct {
	Dynamic      bool
	StaticSize   int
	DynamicIndex int
}

type VanishingModule struct {
	SourceName         string
	Size               ModuleSize
	Expressions        []ExprNode
	Buckets            []VanishingBucket
	WitnessClaimOffset int
	MergeCoinIndex     int
	EvalCoinIndex      int
}

type VanishingBucket struct {
	Ratio               int
	Vanishings          []Vanishing
	QuotientClaimOffset int
}

type Vanishing struct {
	SourceName         string
	Expression         int
	CancelledPositions []int
}

type ExprNode struct {
	Kind             ExprKind
	ColumnClaim      int
	ColumnSourceName string
	Cell             ScalarRef
	Coin             ScalarRef
	Constant         field.Element
	Operator         Operator
	Operands         []int
	SelectorPosition int
}

type ExprKind int

const (
	ExprColumnClaim ExprKind = iota
	ExprCellValue
	ExprCoinValue
	ExprConstant
	ExprOp
	ExprLagrangeSelector
)

type ScalarRef struct {
	Round      int
	Index      int
	FlatIndex  int
	SourceName string
}

type Operator string

const (
	OperatorAdd     Operator = "add"
	OperatorMul     Operator = "mul"
	OperatorSub     Operator = "sub"
	OperatorDiv     Operator = "div"
	OperatorDouble  Operator = "double"
	OperatorSquare  Operator = "square"
	OperatorNegate  Operator = "negate"
	OperatorInverse Operator = "inverse"
)

type viewKey struct {
	id    wiop.ObjectID
	shift int
}

// BuildVanishingSystem extracts only compiled global.Verifier actions from sys
// and converts them to the compact data representation consumed by Zig.
func BuildVanishingSystem(sys *wiop.System, routing CoinRouting) (VanishingSystem, error) {
	out := VanishingSystem{
		SourceName: sys.Context.Path(),
	}
	// DynamicIndex must point into `module_sizes` using the canonical
	// sys.Modules order the transcript absorption uses (see DynamicModuleOrder),
	// NOT verifier-action-registration order — otherwise a multi-dynamic-module
	// protocol would index the wrong size and desync from the prover.
	dynamicIndices := DynamicModuleIndex(sys)

	for _, round := range sys.Rounds {
		for _, action := range round.VerifierActions {
			verifier, ok := action.(*global.Verifier)
			if !ok {
				continue
			}
			moduleRef := verifier.Module
			module := VanishingModule{SourceName: moduleRef.Context.Label, WitnessClaimOffset: out.TotalWitnessClaims}
			if moduleRef.IsDynamic() {
				idx, ok := dynamicIndices[moduleRef]
				if !ok {
					return VanishingSystem{}, fmt.Errorf("codegen: dynamic module %q not found in sys.Modules order", moduleRef.Context.Path())
				}
				module.Size = ModuleSize{Dynamic: true, DynamicIndex: idx}
			} else {
				// The verifier needs a domain of this size, so it must be a power
				// of two within the field's 2-adicity. Rejecting here keeps a
				// malformed System from being emitted at all: `system` is runtime
				// data in the verifier, so this cannot be a Zig comptime assertion
				// any more, and the verifier's own check would only fire at
				// verification time.
				n := moduleRef.Size()
				if n <= 0 || n&(n-1) != 0 {
					return VanishingSystem{}, fmt.Errorf(
						"codegen: static vanishing module %q has size %d, which is not a non-zero power of two",
						moduleRef.Context.Path(), n)
				}
				if log2 := bits.TrailingZeros(uint(n)); uint64(log2) > field.MaxOrderRoot {
					return VanishingSystem{}, fmt.Errorf(
						"codegen: static vanishing module %q has size 2^%d, exceeding the KoalaBear 2-adicity 2^%d",
						moduleRef.Context.Path(), log2, field.MaxOrderRoot)
				}
				module.Size = ModuleSize{StaticSize: n}
			}

			views := make(map[viewKey]int, len(verifier.WitnessViews))
			for i, view := range verifier.WitnessViews {
				views[viewKey{id: view.Column.Context.ID, shift: view.ShiftingOffset}] = i
			}

			// Indices are module-local, so nodes are shared within a module only.
			nodes := exprNodes{leaves: make(map[leafKey]int), memo: make(map[wiop.Expression]int)}

			out.TotalWitnessClaims += len(verifier.WitnessClaims)
			for _, bucket := range verifier.Buckets {
				b := VanishingBucket{
					Ratio:               bucket.Ratio,
					QuotientClaimOffset: out.TotalQuotientClaims,
				}
				out.TotalQuotientClaims += len(bucket.QuotientClaims)

				for _, v := range bucket.Vanishings {
					exprIdx, err := appendExpr(&module, views, &nodes, routing, v.Expression)
					if err != nil {
						return VanishingSystem{}, err
					}
					b.Vanishings = append(b.Vanishings, Vanishing{
						SourceName:         v.Context().Label,
						Expression:         exprIdx,
						CancelledPositions: append([]int(nil), v.CancelledPositions...),
					})
				}
				module.Buckets = append(module.Buckets, b)
			}

			mergeIdx, err := flatCoinIndex(routing, verifier.MergeCoin)
			if err != nil {
				return VanishingSystem{}, fmt.Errorf("module %q merge coin: %w", module.SourceName, err)
			}
			evalIdx, err := flatCoinIndex(routing, verifier.EvalCoin)
			if err != nil {
				return VanishingSystem{}, fmt.Errorf("module %q eval coin: %w", module.SourceName, err)
			}
			module.MergeCoinIndex = mergeIdx
			module.EvalCoinIndex = evalIdx

			out.Modules = append(out.Modules, module)
		}
	}

	out.DynamicModuleCount = len(dynamicIndices)
	return out, nil
}

// flatCoinIndex returns the absolute index of coin in the flat all_coins slice
// described by routing. It reads the round index and within-round position
// directly from coin.Context.ID, so it is correct regardless of which rounds
// the vanishing compiler chose for merge and eval coins.
func flatCoinIndex(routing CoinRouting, coin *wiop.CoinField) (int, error) {
	roundIdx := coin.Context.ID.Slot()
	posInRound := coin.Context.ID.Position()
	if roundIdx >= len(routing.RoundCoinOffsets) {
		return 0, fmt.Errorf("round index %d out of range (routing has %d rounds)", roundIdx, len(routing.RoundCoinOffsets))
	}
	if posInRound >= routing.RoundCoinCounts[roundIdx] {
		return 0, fmt.Errorf("position %d >= round %d coin count %d", posInRound, roundIdx, routing.RoundCoinCounts[roundIdx])
	}
	idx := routing.RoundCoinOffsets[roundIdx] + posInRound
	if idx >= routing.TotalRoundCoins {
		return 0, fmt.Errorf("flat index %d >= total_round_coins %d", idx, routing.TotalRoundCoins)
	}
	return idx, nil
}

// exprNodes holds the per-module state of appendExpr.
type exprNodes struct {
	// memo maps each emitted wiop.Expression to its index. prover-ray interns
	// every vanishing (wiop.Module.NewVanishing), so one node per pointer is
	// one node per value, and each shared node is walked once.
	memo map[wiop.Expression]int
	// leaves merges leaves of equal value that are separate wiop objects: a
	// vector and a scalar constant, which prover-ray keeps apart.
	leaves map[leafKey]int
}

// leafKey identifies a leaf node by the value it reads; only the fields
// relevant to kind are set.
type leafKey struct {
	kind     ExprKind
	index    int
	round    int
	constant field.Element
}

func makeLeafKey(node ExprNode) leafKey {
	k := leafKey{kind: node.Kind}
	switch node.Kind {
	case ExprColumnClaim:
		k.index = node.ColumnClaim
	case ExprConstant:
		k.constant = node.Constant
	case ExprCellValue:
		k.round, k.index = node.Cell.Round, node.Cell.Index
	case ExprCoinValue:
		k.index = node.Coin.FlatIndex
	case ExprLagrangeSelector:
		k.index = node.SelectorPosition
	}
	return k
}

// appendLeaf appends a leaf node unless one reading the same value exists,
// returning its index either way.
func appendLeaf(module *VanishingModule, nodes *exprNodes, node ExprNode) int {
	key := makeLeafKey(node)
	if idx, ok := nodes.leaves[key]; ok {
		return idx
	}
	module.Expressions = append(module.Expressions, node)
	idx := len(module.Expressions) - 1
	nodes.leaves[key] = idx
	return idx
}

// appendExpr returns the index of expr in module.Expressions, emitting it and
// its operands in post-order on first visit. Operands precede their parent, as
// vanishing.zig's evalExpr requires.
func appendExpr(module *VanishingModule, views map[viewKey]int, nodes *exprNodes, routing CoinRouting, expr wiop.Expression) (int, error) {
	if idx, ok := nodes.memo[expr]; ok {
		return idx, nil
	}
	idx, err := emitExpr(module, views, nodes, routing, expr)
	if err != nil {
		return 0, err
	}
	nodes.memo[expr] = idx
	return idx, nil
}

// emitExpr emits expr's node, after appending its operands via appendExpr.
func emitExpr(module *VanishingModule, views map[viewKey]int, nodes *exprNodes, routing CoinRouting, expr wiop.Expression) (int, error) {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		idx, ok := views[viewKey{id: e.Column.Context.ID, shift: e.ShiftingOffset}]
		if !ok {
			return 0, fmt.Errorf("column view %s shift %d was not exported as a witness claim", e.Column.Context.Path(), e.ShiftingOffset)
		}
		return appendLeaf(module, nodes, ExprNode{Kind: ExprColumnClaim, ColumnClaim: idx, ColumnSourceName: e.Column.Context.Label}), nil
	case *wiop.Constant:
		return appendLeaf(module, nodes, ExprNode{Kind: ExprConstant, Constant: e.Value}), nil
	case *wiop.ArithmeticOperation:
		operands := make([]int, len(e.Operands))
		for i, operand := range e.Operands {
			idx, err := appendExpr(module, views, nodes, routing, operand)
			if err != nil {
				return 0, err
			}
			operands[i] = idx
		}
		op, err := mapOperator(e.Operator)
		if err != nil {
			return 0, err
		}
		module.Expressions = append(module.Expressions, ExprNode{Kind: ExprOp, Operator: op, Operands: operands})
		return len(module.Expressions) - 1, nil
	case *wiop.Cell:
		return appendLeaf(module, nodes, ExprNode{
			Kind: ExprCellValue,
			Cell: ScalarRef{
				Round:      e.Context.ID.Slot(),
				Index:      e.Context.ID.Position(),
				SourceName: e.Context.Label,
			},
		}), nil
	case *wiop.CoinField:
		flatIdx, err := flatCoinIndex(routing, e)
		if err != nil {
			return 0, fmt.Errorf("coin %q: %w", e.Context.Path(), err)
		}
		return appendLeaf(module, nodes, ExprNode{
			Kind: ExprCoinValue,
			Coin: ScalarRef{
				FlatIndex:  flatIdx,
				SourceName: e.Context.Label,
			},
		}), nil
	case *wiop.LagrangeSelector:
		return appendLeaf(module, nodes, ExprNode{Kind: ExprLagrangeSelector, SelectorPosition: e.Position}), nil
	default:
		return 0, &UnsupportedExpressionError{Type: fmt.Sprintf("%T", expr)}
	}
}

func mapOperator(op wiop.ArithmeticOperator) (Operator, error) {
	switch op {
	case wiop.ArithmeticOperatorAdd:
		return OperatorAdd, nil
	case wiop.ArithmeticOperatorMul:
		return OperatorMul, nil
	case wiop.ArithmeticOperatorSub:
		return OperatorSub, nil
	case wiop.ArithmeticOperatorDiv:
		return OperatorDiv, nil
	case wiop.ArithmeticOperatorDouble:
		return OperatorDouble, nil
	case wiop.ArithmeticOperatorSquare:
		return OperatorSquare, nil
	case wiop.ArithmeticOperatorNegate:
		return OperatorNegate, nil
	case wiop.ArithmeticOperatorInverse:
		return OperatorInverse, nil
	default:
		return "", &UnsupportedExpressionError{Type: fmt.Sprintf("ArithmeticOperator(%d)", int(op))}
	}
}
