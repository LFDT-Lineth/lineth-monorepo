package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// RangeCheck is a [Query] asserting that every row of a column lies in [0, B).
// A compiler pass must reduce this query before proof generation by converting
// it to an Inclusion relation against a precomputed range table.
type RangeCheck struct {
	baseQuery
	// Handle is the column whose values must all lie in [0, B).
	Handle *Column
	// B is the exclusive upper bound of the valid range.
	B int
}

// Round implements [Query]. Returns the round of the checked column.
func (rc *RangeCheck) Round() *Round { return rc.Handle.Round() }

// Check implements [Query]. Verifies that every row of Handle lies in [0, B).
func (rc *RangeCheck) Check(rt Runtime) error {
	m := rc.Handle.Module
	n := m.Size()
	cv := rt.GetColumnAssignment(rc.Handle)
	for row := range n {
		elem := cv.ElementAt(m, row)
		if !elem.IsBase() {
			return fmt.Errorf(
				"wiop: RangeCheck(%s).Check: extension-field value at row %d",
				rc.context.Path(), row,
			)
		}
		base := elem.AsBase()
		v := field.ToInt(&base)
		if v >= rc.B {
			return fmt.Errorf(
				"wiop: RangeCheck(%s).Check: value %d at row %d is out of range [0, %d)",
				rc.context.Path(), v, row, rc.B,
			)
		}
	}
	return nil
}

// String returns a human-readable description of the RangeCheck for debugging.
func (rc *RangeCheck) String() string {
	return fmt.Sprintf("RangeCheck(%s, B=%d)", rc.context.Path(), rc.B)
}

// NewRangeCheck registers a new RangeCheck on module m asserting that every
// value in col lies in [0, B).
//
// Panics if ctx or col is nil, or if B is not positive.
func (m *Module) NewRangeCheck(ctx *ContextFrame, col *Column, b int) *RangeCheck {
	if ctx == nil {
		panic("wiop: Module.NewRangeCheck requires a non-nil ContextFrame")
	}
	if col == nil {
		panic("wiop: Module.NewRangeCheck requires a non-nil Column")
	}
	if b <= 0 {
		panic(fmt.Sprintf(
			"wiop: Module.NewRangeCheck requires a positive bound, got %d", b,
		))
	}
	rc := &RangeCheck{
		baseQuery: baseQuery{
			context:     ctx,
			Annotations: make(Annotations),
		},
		Handle: col,
		B:      b,
	}
	m.RangeChecks = append(m.RangeChecks, rc)
	return rc
}
