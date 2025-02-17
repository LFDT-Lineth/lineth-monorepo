package globalcs

import (
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/symbolic/simplify"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// factorExpressionList applies [factorExpression] over a list of expressions
func factorExpressionList(comp *wizard.CompiledIOP, exprList []*symbolic.Expression) []*symbolic.Expression {
	res := make([]*symbolic.Expression, len(exprList))
	var wg sync.WaitGroup

	for i, expr := range exprList {
		wg.Add(1)
		go func(i int, expr *symbolic.Expression) {
			defer wg.Done()
			res[i] = factorExpression(comp, expr)
		}(i, expr)
	}

	wg.Wait()
	return res
}

// factorExpression factors expr and returns the factored expression. The
// resulting factored expression is cached in the file system as this is a
// compute intensive operation.
func factorExpression(comp *wizard.CompiledIOP, expr *symbolic.Expression) *symbolic.Expression {

	var (
		flattenedExpr = flattenExpr(expr)
		eshStr        = flattenedExpr.ESHash.Text(16)
		cacheKey      = "global-cs-" + eshStr
		wrapper       = &serializableExpr{Comp: comp}
	)

	found, err := comp.Artefacts.TryLoad(cacheKey, wrapper)

	if err != nil {
		utils.Panic("could not deserialize the cached expression")
	}

	if !found {
		wrapper.Expr = simplify.AutoSimplify(flattenedExpr)
		if err := comp.Artefacts.Store(cacheKey, wrapper); err != nil {
			utils.Panic("could not cache the factored expression: %v", err.Error())
		}
	}

	return wrapper.Expr
}

// serializableExpr wraps [symbolic.Expression] and implements the [wizard.Artefact]
// interface.
type serializableExpr struct {
	// Comp points to the underlying [*wizard.CompiledIOP] object, it is necessary
	// for deserializing as the blob only contains the column names. The CompiledIOP
	// object is used to infer the concrete column and thus instantiate the encoded
	// expression.
	Comp *wizard.CompiledIOP
	Expr *symbolic.Expression
}

var (
	_ io.ReaderFrom = &serializableExpr{}
	_ io.WriterTo   = &serializableExpr{}
)

// ReadFrom implements the [io.ReaderFrom] and thus the [wizard.Artefact] interface.
func (s *serializableExpr) ReadFrom(r io.Reader) (int64, error) {

	buf, err := io.ReadAll(r)
	if err != nil {
		return 0, fmt.Errorf("error while [io.ReadAll]: %w", err)
	}

	exprVal, err := serialization.DeserializeValue(
		buf,
		serialization.ReferenceMode,
		reflect.TypeOf(&symbolic.Expression{}),
		s.Comp,
	)

	if err != nil {
		return 0, fmt.Errorf("DeserializeValue returned an error for expression: %w", err)
	}

	s.Expr = exprVal.Interface().(*symbolic.Expression)
	return int64(len(buf)), nil
}

// WriteTo implements the [io.WriterTo] and thus the [wizard.Artefact] interface.
func (s *serializableExpr) WriteTo(w io.Writer) (int64, error) {

	blob, err := serialization.SerializeValue(
		reflect.ValueOf(s.Expr),
		serialization.ReferenceMode,
	)

	if err != nil {
		return 0, fmt.Errorf("could not serialize the expression with SerializeValue: %w", err)
	}

	n, err := w.Write(blob)
	if err != nil {
		return 0, fmt.Errorf("could not write the blob: %w", err)
	}

	return int64(n), nil
}

// flattenExpr returns an expression equivalent to expr where the
// [accessors.FromExprAccessor] are inlined
func flattenExpr(expr *symbolic.Expression) *symbolic.Expression {
	return expr.ReconstructBottomUp(func(e *symbolic.Expression, children []*symbolic.Expression) (new *symbolic.Expression) {

		v, isVar := e.Operator.(symbolic.Variable)
		if !isVar {
			return e.SameWithNewChildren(children)
		}

		fea, isFEA := v.Metadata.(*accessors.FromExprAccessor)

		if isFEA {
			return flattenExpr(fea.Expr)
		}

		return e
	})
}
