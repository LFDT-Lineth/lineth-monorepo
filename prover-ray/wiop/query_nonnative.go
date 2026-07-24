package wiop

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// enforce that NonNative implements Query
var _ Query = (*NonNative)(nil)

// NonNative is a [Query] asserting that a set of limb-decomposed columns
// satisfy the non-native modular multiplication relation
//
//	Left * Right = Quotient * Modulus + Result
//
// where each of Left, Right, Modulus, Result and Quotient is represented as a
// little-endian sequence of limb columns of NbBitsPerLimb bits each.
//
// The query is only declarative, the user must run the compiler pass in package
// wiop/compilers/nonnative to reduce it to a Schwartz-Zippel polynomial
// identity checked at a shared random point.
//
// The compiler pass allocates the carry columns and samples the coin itself so that
// multiple [NonNative] queries can share the same coin.
//
// The query is checkable via limb composition, but it doesn't enforce the
// soundness of the proof against a malicious prover. The compiler pass reduces
// the query to a polynomial identity check that is enforced by the global
// compiler.
//
// Use [Module.NewNonNative] to construct and register an instance.
type NonNative struct {
	baseQuery
	// Module is the module all operand columns belong to.
	Module *Module
	// InputRound is the round in which every operand column is committed.
	InputRound *Round
	// NbBitsPerLimb is the number of bits represented by each limb.
	NbBitsPerLimb int
	// Left, Right and Modulus are the caller-provided operands, little-endian
	// (Left[0] is the least significant limb).
	Left, Right, Modulus []*Column
	// Result is the caller-provided Left*Right mod Modulus.
	Result []*Column
	// Quotient is the caller-provided (Left*Right) / Modulus.
	Quotient []*Column
}

// Round returns InputRound. We assume that the caller has provided all inputs
// columns in the same round.
func (q *NonNative) Round() *Round { return q.InputRound }

// Check verifies row by row that the limb-composed operands satisfy
//
//	Left*Right = Quotient*Modulus + Result.
//
// It does not rely on the vanishing relation the compiler produces (no coin
// exists yet at this stage); it recomposes each operand from its limbs and
// checks the relation directly over big.Int arithmetic.
//
// It is a convenience runtime check only; soundness against a malicious prover is
// established by compiling the system using nonnative and global compilers.
//
// NB! It is not registered as a verifier action, it is only provided for
// implementing [Query] interfaces and testing.
func (q *NonNative) Check(rt *Runtime) error {
	m := q.Module
	nbRows := m.RuntimeSize(rt)

	// getColumnAssignments returns the concrete column assignments for the given
	// limb columns, or nil if any of them is unassigned.
	//
	// they are nil when we check the query from the proof, not from the full trace.
	leftCols, ok := getColumnAssignments(rt, q.Left)
	if !ok {
		return nil
	}
	rightCols, ok := getColumnAssignments(rt, q.Right)
	if !ok {
		return nil
	}
	modCols, ok := getColumnAssignments(rt, q.Modulus)
	if !ok {
		return nil
	}
	resCols, ok := getColumnAssignments(rt, q.Result)
	if !ok {
		return nil
	}
	quoCols, ok := getColumnAssignments(rt, q.Quotient)
	if !ok {
		return nil
	}

	product := new(big.Int)
	expected := new(big.Int)

	// we iterate over nbRows which is the runtime size of the module. But as
	// 0*0 = 0 + 0*0 holds, we can skip the rows that are not assigned in the
	// input round.
	for row := range nbRows {
		left := composeLimbsRow(leftCols, q.NbBitsPerLimb, nbRows, row, m.Padding)
		right := composeLimbsRow(rightCols, q.NbBitsPerLimb, nbRows, row, m.Padding)
		mod := composeLimbsRow(modCols, q.NbBitsPerLimb, nbRows, row, m.Padding)
		res := composeLimbsRow(resCols, q.NbBitsPerLimb, nbRows, row, m.Padding)
		quo := composeLimbsRow(quoCols, q.NbBitsPerLimb, nbRows, row, m.Padding)

		product.Mul(left, right)
		expected.Mul(quo, mod)
		expected.Add(expected, res)

		if product.Cmp(expected) != 0 {
			return fmt.Errorf(
				"wiop: NonNative(%s).Check: row %d: left*right (%s) != quotient*modulus+result (%s)",
				q.context.Path(), row, product.String(), expected.String(),
			)
		}
		if mod.Sign() != 0 && res.Cmp(mod) >= 0 {
			return fmt.Errorf(
				"wiop: NonNative(%s).Check: row %d: result (%s) is not reduced modulo modulus (%s)",
				q.context.Path(), row, res.String(), mod.String(),
			)
		}
	}
	return nil
}

func getColumnAssignments(rt *Runtime, cols []*Column) ([]*ConcreteVector, bool) {
	out := make([]*ConcreteVector, len(cols))
	for i, c := range cols {
		if !rt.HasColumnAssignment(c) {
			return nil, false
		}
		out[i] = rt.GetColumnAssignment(c)
	}
	return out, true
}

// composeLimbsRow recomposes the little-endian limbs of a single row into a
// big.Int, base 2^nbBitsPerLimb.
func composeLimbsRow(cols []*ConcreteVector, nbBitsPerLimb, nbRows, row int, pd PaddingDirection) *big.Int {
	res := new(big.Int)
	limb := new(big.Int)
	for i := len(cols) - 1; i >= 0; i-- {
		// we need to assign `f` here because `Uint64()` below is defined on
		// pointer receivers, and `cols[i].ElementAtN(...)` returns a value, not
		// a pointer.
		f := cols[i].ElementAtN(pd, nbRows, row).AsBase()
		limb.SetUint64(f.Uint64())
		res.Lsh(res, uint(nbBitsPerLimb))
		res.Or(res, limb)
	}
	return res
}

// NewNonNative constructs and registers a [NonNative] query on module, asserting
//
//	Left*Right = Quotient*Modulus + Result
//
// over limb-decomposed columns, each limb with nbBitsPerLimb bits, in little-endian ordering.
//
// Panics if ctx or module is nil, if nbBitsPerLimb is not positive, if any of
// the five limb slices is empty, if limb slices are of different length, if any
// column does not belong to m or is not committed in inputRound.
//
// See [NonNative] for more details.
func (m *Module) NewNonNative(
	ctx *ContextFrame,
	nbBitsPerLimb int,
	left, right, modulus, result, quotient []*Column,
) *NonNative {
	if m == nil {
		panic("wiop: Module.NewNonNative requires a non-nil Module")
	}
	if ctx == nil {
		panic("wiop: Module.NewNonNative requires a non-nil ContextFrame")
	}
	if nbBitsPerLimb <= 0 || nbBitsPerLimb >= field.Bits-3 { // we need some headroom for ops
		panic(fmt.Sprintf(
			"wiop: Module.NewNonNative requires a NbBitsPerLimb in [1, %d), got %d", field.Bits-3, nbBitsPerLimb,
		))
	}

	nbLimbs := 0
	var inputRound *Round

	groups := [5]struct {
		name string
		cols []*Column
	}{
		{"Left", left},
		{"Right", right},
		{"Modulus", modulus},
		{"Result", result},
		{"Quotient", quotient},
	}
	for _, g := range groups {
		if len(g.cols) == 0 {
			panic(fmt.Sprintf("wiop: Module.NewNonNative requires a non-empty %s limb slice", g.name))
		}
		if nbLimbs != 0 && len(g.cols) != nbLimbs {
			panic(fmt.Sprintf(
				"wiop: Module.NewNonNative requires all limb slices to have the same length, got %d for %s, expected %d",
				len(g.cols), g.name, nbLimbs,
			))
		}
		nbLimbs = len(g.cols)
		for i, c := range g.cols {
			if c == nil {
				panic(fmt.Sprintf("wiop: Module.NewNonNative: %s limb %d is nil", g.name, i))
			}
			if c.Module != m {
				panic(fmt.Sprintf(
					"wiop: Module.NewNonNative: %s limb %d belongs to module %q, expected %q",
					g.name, i, c.Module.Context.Path(), m.Context.Path(),
				))
			}
			if inputRound == nil {
				inputRound = c.Round()
			} else if c.Round() != inputRound {
				panic(fmt.Sprintf(
					"wiop: Module.NewNonNative: %s limb %d is committed in round %d, expected round %d",
					g.name, i, c.Round().ID, inputRound.ID,
				))
			}
		}
	}

	q := &NonNative{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Module:        m,
		InputRound:    inputRound,
		NbBitsPerLimb: nbBitsPerLimb,
		Left:          left,
		Right:         right,
		Modulus:       modulus,
		Result:        result,
		Quotient:      quotient,
	}
	m.NonNatives = append(m.NonNatives, q)
	return q
}
