package codegen

import (
	"errors"
	"fmt"

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
	dynamicIndices := map[*wiop.Module]int{}

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
					idx = len(dynamicIndices)
					dynamicIndices[moduleRef] = idx
				}
				module.Size = ModuleSize{Dynamic: true, DynamicIndex: idx}
			} else {
				module.Size = ModuleSize{StaticSize: moduleRef.Size()}
			}

			views := make(map[viewKey]int, len(verifier.WitnessViews))
			for i, view := range verifier.WitnessViews {
				views[viewKey{id: view.Column.Context.ID, shift: view.ShiftingOffset}] = i
			}

			out.TotalWitnessClaims += len(verifier.WitnessClaims)
			for _, bucket := range verifier.Buckets {
				b := VanishingBucket{
					Ratio:               bucket.Ratio,
					QuotientClaimOffset: out.TotalQuotientClaims,
				}
				out.TotalQuotientClaims += len(bucket.QuotientClaims)

				for _, v := range bucket.Vanishings {
					exprIdx, err := appendExpr(&module, views, routing, v.Expression)
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

func appendExpr(module *VanishingModule, views map[viewKey]int, routing CoinRouting, expr wiop.Expression) (int, error) {
	switch e := expr.(type) {
	case *wiop.ColumnView:
		idx, ok := views[viewKey{id: e.Column.Context.ID, shift: e.ShiftingOffset}]
		if !ok {
			return 0, fmt.Errorf("column view %s shift %d was not exported as a witness claim", e.Column.Context.Path(), e.ShiftingOffset)
		}
		module.Expressions = append(module.Expressions, ExprNode{Kind: ExprColumnClaim, ColumnClaim: idx, ColumnSourceName: e.Column.Context.Label})
		return len(module.Expressions) - 1, nil
	case *wiop.Constant:
		module.Expressions = append(module.Expressions, ExprNode{Kind: ExprConstant, Constant: e.Value})
		return len(module.Expressions) - 1, nil
	case *wiop.ArithmeticOperation:
		operands := make([]int, len(e.Operands))
		for i, operand := range e.Operands {
			idx, err := appendExpr(module, views, routing, operand)
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
		module.Expressions = append(module.Expressions, ExprNode{
			Kind: ExprCellValue,
			Cell: ScalarRef{
				Round:      e.Context.ID.Slot(),
				Index:      e.Context.ID.Position(),
				SourceName: e.Context.Label,
			},
		})
		return len(module.Expressions) - 1, nil
	case *wiop.CoinField:
		flatIdx, err := flatCoinIndex(routing, e)
		if err != nil {
			return 0, fmt.Errorf("coin %q: %w", e.Context.Path(), err)
		}
		module.Expressions = append(module.Expressions, ExprNode{
			Kind: ExprCoinValue,
			Coin: ScalarRef{
				Round:      e.Context.ID.Slot(),
				Index:      e.Context.ID.Position(),
				FlatIndex:  flatIdx,
				SourceName: e.Context.Label,
			},
		})
		return len(module.Expressions) - 1, nil
	case *wiop.LagrangeSelector:
		module.Expressions = append(module.Expressions, ExprNode{Kind: ExprLagrangeSelector, SelectorPosition: e.Position})
		return len(module.Expressions) - 1, nil
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
