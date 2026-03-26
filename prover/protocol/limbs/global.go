package limbs

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// NewGlobal generates a new global constraint that spans over the coordinates
// of limb split object. The constraint is only repeated over the coordinate,
// e.g. it does emulate big-integer arithmetic.
func NewGlobal(comp *wizard.CompiledIOP, name ifaces.QueryID, expr *symbolic.Expression) []query.GlobalConstraint {
	splittedExpressions := splitExpressions(expr)
	res := make([]query.GlobalConstraint, len(splittedExpressions))
	for i := range splittedExpressions {
		res[i] = comp.InsertGlobal(0, ifaces.QueryIDf("%v_%v", name, i), splittedExpressions[i])
	}
	return res
}

// NewLocal generates a new local constraint that spans over the coordinates
// of a limb split object.
func NewLocal(comp *wizard.CompiledIOP, name ifaces.QueryID, expr *symbolic.Expression) []query.LocalConstraint {
	splittedExpressions := splitExpressions(expr)
	res := make([]query.LocalConstraint, len(splittedExpressions))
	for i := range splittedExpressions {
		res[i] = comp.InsertLocal(0, ifaces.QueryIDf("%v_%v", name, i), splittedExpressions[i])
	}
	return res
}

// Shift converts the Limbed object into one whose columns are shifted by the
// provided offset.
func Shift[E Endianness](l Limbs[E], offset int) Limbs[E] {
	var (
		newName = ifaces.ColIDf("%v_SHIFTED_%v", l.Name, offset)
		new     = Limbs[E]{Name: ifaces.ColID(newName), C: make([]ifaces.Column, len(l.C))}
	)

	for i := range l.C {
		new.C[i] = column.Shift(l.C[i], offset)
	}

	return new
}

func splitExpressions(expr *symbolic.Expression) []*symbolic.Expression {

	// First we iterate over the inputs of the expressions to flip all inputs
	// in big-endian order and check that the number of limbs match
	var (
		metadata = expr.ListBoardVariableMetadata()
		limbs    = map[string][]ifaces.Column{}
		rows     = map[string][]field.Element{}
		numLimbs = -1
	)

	for _, m := range metadata {

		switch m := m.(type) {

		case Limbed:
			if numLimbs < 0 {
				numLimbs = m.NumLimbs()
			}
			name := m.String()
			limbs[name] = m.ToBigEndianLimbs().GetLimbs()
			if numLimbs != m.NumLimbs() {
				utils.Panic("all limbs must have the same number of limbs, got %v and %v", numLimbs, m.NumLimbs())
			}

		case row[LittleEndian]:
			if numLimbs < 0 {
				numLimbs = m.NumLimbs()
			}
			name := m.String()
			rows[name] = m.ToBigEndianLimbs().T
			if numLimbs != m.NumLimbs() {
				utils.Panic("all limbs must have the same number of limbs, got %v and %v", numLimbs, m.NumLimbs())
			}

		case row[BigEndian]:
			if numLimbs < 0 {
				numLimbs = m.NumLimbs()
			}
			name := m.String()
			rows[name] = m.ToBigEndianLimbs().T
			if numLimbs != m.NumLimbs() {
				utils.Panic("all limbs must have the same number of limbs, got %v and %v", numLimbs, m.NumLimbs())
			}
		}
	}

	// Then we generate the new global constraints and return them
	res := make([]*symbolic.Expression, numLimbs)
	for i := 0; i < numLimbs; i++ {

		constructor := func(e *symbolic.Expression, children []*symbolic.Expression) (new *symbolic.Expression) {

			vari, ok := e.Operator.(symbolic.Variable)
			if !ok {
				// Nothing to change here
				return e.SameWithNewChildren(children)
			}

			name := vari.Metadata.String()

			if e, ok := limbs[name]; ok {
				// The expression is a limb object variable
				return symbolic.NewVariable(e[i])
			}

			if e, ok := rows[name]; ok {
				// The expression is a row object variable
				return symbolic.NewConstant(e[i])
			}

			// The expression is a variable but not a limbed object
			return e
		}

		res[i] = expr.ReconstructBottomUpSingleThreaded(constructor)
	}

	return res
}
