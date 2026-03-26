package column

import (
	"reflect"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	shift       string = "SHIFT"
	interleaved string = "INTERLEAVED"
	// Generalizes the concept of Natural to
	// the case of verifier defined columns
	nonComposite string = "NONCOMPOSITE"
)

// Constructs a natural column. input validation. Not exported, use
// [store.AddToRound] which will output a well-formed object. This ensures the
// invariant that a `Natural` always have a store.
func newNatural(name ifaces.ColID, position columnPosition, store *Store) Natural {
	if len(name) == 0 {
		utils.Panic("empty name")
	}
	if store == nil {
		utils.Panic("null store (%v)", name)
	}
	return Natural{ID: name, position: position, store: store}
}

// RootParents returns the underlying base [Natural] of the current handle.
func RootParents(h ifaces.Column) ifaces.Column {

	if !h.IsComposite() {
		return h
	}

	switch inner := h.(type) {
	case Natural:
		// No changes
		return h
	case Shifted:
		return RootParents(inner.Parent)
	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

// RootsOf returns a deduplicated list of [column.Natural] (or
// [verifiercol.VerifierCol]] underlying the definition of the input columns.
// If `skipVCol` is true, the function will skip the verifiercol and not
// the [column.Natural] columns. The function also skips nil columns.
func RootsOf(cols []ifaces.Column, skipVCol bool) []ifaces.Column {

	var (
		roots    = make([]ifaces.Column, 0, len(cols))
		rootSets = make(map[ifaces.ColID]struct{}, len(cols))
	)

	for i := range cols {

		if cols[i] == nil {
			continue
		}

		root := RootParents(cols[i])

		// This clause checks if the root is a verifiercol and if it
		// should be skipped following "skipVCol". However, testing this
		// directly would create a circular dependency. So we instead use
		// [IsComposite] == false and hasType([column.Natural]) == true
		// to perform the test.
		_, isNat := root.(Natural)
		if !isNat && !root.IsComposite() && skipVCol {
			continue
		}

		if _, ok := rootSets[root.GetColID()]; ok {
			continue
		}

		roots = append(roots, root)
		rootSets[root.GetColID()] = struct{}{}
	}

	return roots
}

// StackOffset sums all the offsets contained in the handle and return the result
func StackOffsets(h ifaces.Column) int {

	if !h.IsComposite() {
		return 0
	}

	switch inner := h.(type) {
	case Natural:
		// No changes
		return 0
	case Shifted:
		return inner.Offset + StackOffsets(inner.Parent)
	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

// NbLeaves returns the number of underlying [Natural] columns for `h`. If `h`
// is neither an [Interleaved] nor derived from an [Interleaved], the function
// returns 1.
func NbLeaves(h ifaces.Column) int {
	switch inner := h.(type) {
	case Natural:
		// No changes
		return 1
	case Shifted:
		return NbLeaves(inner.Parent)
	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

// EvalExprColumn resolves an expression to a column assignment. The expression
// must be converted to a board prior to evaluating the expression.
//
//   - If the expression does not uses ifaces.Column as metadata, the function
//     will panic.
//
//   - If the expression contains several columns and they don't contain all
//     have the same size.
func EvalExprColumn(run ifaces.Runtime, board symbolic.ExpressionBoard) smartvectors.SmartVector {

	var (
		metadata = board.ListVariableMetadata()
		inputs   = make([]smartvectors.SmartVector, len(metadata))
		length   = ExprIsOnSameLengthHandles(&board)
	)

	// Attempt to recover the size of the
	for i := range inputs {
		switch m := metadata[i].(type) {
		case ifaces.Column:
			inputs[i] = m.GetColAssignment(run)
		case coin.Info:
			if m.IsBase() {
				utils.Panic("unsupported, coins are always over field extensions")
			} else {
				vExt := run.GetRandomCoinFieldExt(m.Name)
				inputs[i] = smartvectors.NewConstantExt(vExt, length)
			}

		case ifaces.Accessor:
			if m.IsBase() {
				v, _ := m.GetValBase(run)
				inputs[i] = smartvectors.NewConstant(v, length)
			} else {
				v := m.GetValExt(run)
				inputs[i] = smartvectors.NewConstantExt(v, length)
			}

		case variables.PeriodicSample:
			v := m.EvalCoset(length, 0, 1, false)
			inputs[i] = v
		case variables.X:
			v := m.EvalCoset(length, 0, 1, false)
			inputs[i] = v
		}
	}

	return board.Evaluate(inputs)
}

// GnarkEvalExprColumn evaluates an expression in a gnark circuit setting
func GnarkEvalExprColumn(api frontend.API, run ifaces.GnarkRuntime, board symbolic.ExpressionBoard) []koalagnark.Ext {

	var (
		metadata = board.ListVariableMetadata()
		length   = ExprIsOnSameLengthHandles(&board)
		res      = make([]koalagnark.Ext, length)
	)

	for k := 0; k < length; k++ {

		inputs := make([]any, len(metadata))

		for i := range inputs {
			switch m := metadata[i].(type) {
			case ifaces.Column:
				if m.IsBase() {
					inputs[i] = m.GetColAssignmentGnarkAt(run, k)
				} else {
					inputs[i] = m.GetColAssignmentGnarkAtExt(run, k)
				}
			case coin.Info:
				if m.IsBase() {
					utils.Panic("unsupported, coins are always over field extensions")
				} else {
					inputs[i] = run.GetRandomCoinFieldExt(m.Name)
				}
			case ifaces.Accessor:
				if m.IsBase() {
					inputs[i] = m.GetFrontendVariable(api, run)
				} else {
					inputs[i] = m.GetFrontendVariableExt(api, run)
				}
			case variables.PeriodicSample:
				tmp := m.EvalAtOnDomain(k)
				inputs[i] = koalagnark.NewElementFromKoala(tmp)
			case variables.X:
				// there is no theoritical problem with this but there are
				// no cases known where this happens so we just don't
				// support it.
				panic("unexpected")
			}
		}

		res[k] = board.GnarkEvalExt(api, inputs)
	}

	return res
}

// ExprIsOnSameLengthHandles checks that all the variables of the expression
// that are [ifaces.Column] have the same size (and panics if it does not), then
// returns the match.
func ExprIsOnSameLengthHandles(board *symbolic.ExpressionBoard) int {

	var (
		metadatas = board.ListVariableMetadata()
		length    = 0
	)

	for _, m := range metadatas {
		switch metadata := m.(type) {
		case ifaces.Column:
			// Initialize the length with the first commitment
			if length == 0 {
				length = metadata.Size()
			}

			// Sanity-check the vector should all have the same length
			if length != metadata.Size() {
				utils.Panic("Inconsistent length for %v (has size %v, but expected %v)", metadata.GetColID(), metadata.Size(), length)
			}
		// The expression can involve random coins
		case coin.Info, variables.X, variables.PeriodicSample, ifaces.Accessor, symbolic.EliminatedVarMetadata:
			// Do nothing
		default:
			utils.Panic("unknown type %T", metadata)
		}
	}

	// No commitment were found in the metadata, thus this call is broken
	if length == 0 {
		utils.Panic("declared a handle from an expression which does not contains any handle")
	}

	return length
}

// return the runtime assignments of a linear combination column
// that is computed on the fly from the columns stored in hs
func RandLinCombColAssignment(run ifaces.Runtime, coinVal field.Element, hs []ifaces.Column) smartvectors.SmartVector {
	var colTableWit smartvectors.SmartVector
	var witnessCollapsed smartvectors.SmartVector
	x := field.One()
	witnessCollapsed = smartvectors.NewConstant(field.Zero(), hs[0].Size())
	for tableCol := range hs {
		colTableWit = hs[tableCol].GetColAssignment(run)
		witnessCollapsed = smartvectors.Add(witnessCollapsed, smartvectors.Mul(colTableWit, smartvectors.NewConstant(x, hs[0].Size())))
		x.Mul(&x, &coinVal)
	}
	return witnessCollapsed
}

// maximal round of declaration for a list of commitment
func MaxRound(handles ...ifaces.Column) int {
	res := 0
	for _, handle := range handles {
		res = utils.Max(res, handle.Round())
	}
	return res
}

// ShiftExpr returns a shifted version of the expression. The function will
// panic if called with an expression that uses [variables.PeriodicSampling]
func ShiftExpr(expr *symbolic.Expression, offset int) *symbolic.Expression {

	return expr.ReconstructBottomUp(func(e *symbolic.Expression, children []*symbolic.Expression) (new *symbolic.Expression) {

		vari, isVar := e.Operator.(symbolic.Variable)
		if !isVar {
			return e.SameWithNewChildren(children)
		}

		if per, isPeriodic := vari.Metadata.(variables.PeriodicSample); isPeriodic {
			return symbolic.NewVariable(variables.PeriodicSample{T: per.T, Offset: utils.PositiveMod(per.Offset-offset, per.T)})
		}

		col, isCol := vari.Metadata.(ifaces.Column)
		if !isCol {
			return e.SameWithNewChildren(children)
		}

		return symbolic.NewVariable(Shift(col, offset))
	})
}

// StatusOf returns the status of the column
func StatusOf(c ifaces.Column) Status {

	switch col := c.(type) {
	case Natural:
		return col.Status()
	case Shifted:
		return StatusOf(col.Parent)
	default:
		// Expectedly, a verifier col
		return VerifierDefined
	}
}

// IsPublicExpression returns true if the expression is fully public
func IsPublicExpression(expr *symbolic.Expression) bool {

	var (
		meta         = expr.ListBoardVariableMetadata()
		numPublic    = 0
		numNonPublic = 0
		statusMap    = map[ifaces.ColID]string{}
	)

	for _, m := range meta {
		if c, ok := m.(ifaces.Column); ok {
			status := StatusOf(c)
			statusMap[c.GetColID()] = status.String()
			if status.IsPublic() {
				numPublic++
			} else {
				numNonPublic++
			}
		}
	}

	if numNonPublic > 0 && numPublic > 0 {
		utils.Panic("the expression is mixed: %v", statusMap)
	}

	return numPublic > 0
}

// ColumnsOfExpression returns the list of all the columns occuring as variables of
// the provided expression.
func ColumnsOfExpression(expr *symbolic.Expression) []ifaces.Column {

	var (
		metadata = expr.ListBoardVariableMetadata()
		res      []ifaces.Column
	)

	for i := range metadata {
		if _, ok := metadata[i].(ifaces.Column); ok {
			res = append(res, metadata[i].(ifaces.Column))
		}
	}

	return res
}

// GetColAssignmentOctuplet returns the assignment of the column as an octuplet.
func GetColAssignmentOctuplet(col ifaces.Column, run ifaces.Runtime) field.Octuplet {
	assignment := col.GetColAssignment(run)
	if assignment.Len() < 8 {
		panic("column size must be at least 8 for Octuplet")
	}
	var result field.Octuplet
	for i := 0; i < 8; i++ {
		result[i] = assignment.Get(i)
	}
	return result
}

func GetColAssignmentGnarkOctuplet(col ifaces.Column, run ifaces.GnarkRuntime) koalagnark.Octuplet {
	assignment := col.GetColAssignmentGnark(run)
	if len(assignment) < 8 {
		panic("column size must be at least 8 for Octuplet")
	}
	var result koalagnark.Octuplet
	for i := 0; i < 8; i++ {
		result[i] = assignment[i]
	}
	return result
}

// GetColAssignmentBase retrieves the base assignment of a column. This is specially used when the column is over field extensions while it is storing the base elements.
func GetColAssignmentBase(run ifaces.Runtime, col ifaces.Column) (res []field.Element, err error) {
	res = make([]field.Element, col.Size())
	for i := 0; i < col.Size(); i++ {
		base, err := col.GetColAssignmentAtBase(run, i)
		if err != nil {
			return nil, err
		}
		res[i] = base
	}
	return res, nil
}
