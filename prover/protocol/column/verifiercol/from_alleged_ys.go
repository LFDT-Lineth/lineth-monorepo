package verifiercol

import (
	"errors"
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = FromYs{}

// Represents a column populated by alleged evaluations of arrange of columns
type FromYs struct {
	// Positions returns the positions in Query.Pols mapped to the positions
	// of the current [FromYs].
	Positions []int
	// The Query from which we shall select the evaluations
	Query query.UnivariateEval
	// Remember the round in which the query was made
	Round_ int
}

// Construct a new column from a univariate query and a list of of ifaces.ColID
// If passed a column that is not part of the query. It will not panic but it will
// return a zero entry. This is the expected behavior when given a shadow column
// from the vortex compiler but otherwise this is a bug.
func NewFromYs(comp *wizard.CompiledIOP, q query.UnivariateEval, ranges []ifaces.ColID) ifaces.Column {

	// All the names in the range should also be part of the query.
	// To make sure of this, we build the following map.
	nameMap := make(map[ifaces.ColID]int, len(q.Pols))
	for i, polName := range q.Pols {
		nameMap[polName.GetColID()] = i
	}

	positions := make([]int, len(ranges))

	for i, rangeName := range ranges {

		pos, ok := nameMap[rangeName]
		switch {
		case ok:
			positions[i] = pos
		case strings.Contains(string(rangeName), "SHADOW"):
			positions[i] = -1
		default:
			utils.Panic("NewFromYs : %v is not part of the query %v", rangeName, q.QueryID)
		}
	}

	// Make sure that the query is indeed registered in the current wizard.
	comp.QueriesParams.MustExists(q.QueryID)
	round := comp.QueriesParams.Round(q.QueryID)

	res := FromYs{
		Positions: positions,
		Query:     q,
		Round_:    round,
	}

	return res
}

// IsBase always returns false because we assume the values are always field
// extensions as they are the result of a univariate evaluation, normally done
// over a field extension X point.
func (fys FromYs) IsBase() bool {
	return false
}

// Always error out because we assume the values are always field extensions
func (fys FromYs) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {
	return field.Element{}, errors.New("not base")
}

func (fys FromYs) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {
	queryParams := run.GetParams(fys.Query.QueryID).(query.UnivariateEvalParams)
	p := fys.Positions[pos]
	if p < 0 {
		return fext.Zero()
	}
	return queryParams.ExtYs[p]
}

func (fys FromYs) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	return nil, errors.New("not base")
}

func (fys FromYs) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	queryParams := run.GetParams(fys.Query.QueryID).(query.GnarkUnivariateEvalParams)

	zeroExt := koalagnark.NewExt(fext.Zero())

	// This will leave some of the columns to nil
	res := make([]koalagnark.Ext, len(fys.Positions))
	for i, p := range fys.Positions {
		if p < 0 {
			res[i] = zeroExt
		} else {
			res[i] = queryParams.ExtYs[p]
		}
	}

	return res
}

func (fys FromYs) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	return koalagnark.Element{}, errors.New("not base")
}

func (fys FromYs) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	queryParams := run.GetParams(fys.Query.QueryID).(query.GnarkUnivariateEvalParams)
	p := fys.Positions[pos]
	if p < 0 {
		return koalagnark.NewExt(fext.Zero())
	}
	return queryParams.ExtYs[p]
}

// Returns the round of definition of the column
func (fys FromYs) Round() int {
	return fys.Round_
}

// Returns a generic name from the column. Defined from the coin's.
func (fys FromYs) GetColID() ifaces.ColID {
	return ifaces.ColIDf("FYS_%v", fys.Query.QueryID)
}

// Always return true. We sanity-check the existence of the
// random coin prior to constructing the object.
func (fys FromYs) MustExists() {}

// Return the size of the fys
func (fys FromYs) Size() int {
	return len(fys.Positions)
}

// Returns the coin's value as a column assignment
func (fys FromYs) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	// This will leaves the columns missing from the query to zero.
	queryParams := run.GetParams(fys.Query.QueryID).(query.UnivariateEvalParams)
	res := make([]fext.Element, len(fys.Positions))
	for i, p := range fys.Positions {
		// p = -1 indicates that the position should be zeroed.
		if p >= 0 {
			res[i] = queryParams.ExtYs[p]
		}
	}
	return smartvectors.NewRegularExt(res)
}

// Returns the coin's value as a column assignment
func (fys FromYs) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {
	panic("not base element")
}

// Returns a particular position of the coin value
func (fys FromYs) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	return fys.GetColAssignment(run).Get(pos)
}

// Returns a particular position of the coin value
func (fys FromYs) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	panic("not a base element")
}

func (fys FromYs) IsComposite() bool {
	return false
}

// Returns the name of the column.
func (fys FromYs) String() string {
	return string(fys.GetColID())
}

// Split the FromYs by restricting to a range
func (fys FromYs) Split(comp *wizard.CompiledIOP, from, to int) ifaces.Column {
	return FromYs{Query: fys.Query, Positions: fys.Positions[from:to], Round_: fys.Round_}
}
