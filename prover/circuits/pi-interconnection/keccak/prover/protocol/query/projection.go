package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/google/uuid"
)

// Projection represents a projection query. A projection query enforces that
// two sides A and B contains the same values on the same order at positions
// marked with a 1 in the corresponding filter. The query supports multi-ary
// projections: meaning that the A and B sides can have multiple columns and
// the query will read both sides left-to-right then top-to-bottom (e.g. like
// reading text in english). In that context, the two sides can have different
// "width": this allows the query to be used for flattening an array of columns
// or widening it.
//
// And on top of that, the query can also "tables": the query look over vector
// of values instead of just values. In that case, all parts of the two sides
// of multi-ary projection must have the same number of columns.
type Projection struct {
	Round int
	ID    ifaces.QueryID
	Inp   ProjectionMultiAryInput
	uuid  uuid.UUID `serde:"omit"`
}

// ProjectionInput is a collection of parameters to provide to a [Projection]
// query. It corresponds to the case where the projection query is "unary".
type ProjectionInput struct {
	// ColumnA and ColumnB are the columns of the left and right side. Each
	// entry of either corresponds to a column in the projected table.
	ColumnA, ColumnB []ifaces.Column
	// FilterA and FilterB are the filters of the ColumnA and ColumnB
	FilterA, FilterB ifaces.Column
}

// ProjectionMultiAryInput is a collection of parameters to provide to a
// [Projection] query in the general case.
type ProjectionMultiAryInput struct {
	// ColumnsA and ColumnsB are the columns of the left and right side.
	// The lists are structured as list of projected tables, each list
	// is processed left-to-right then top-to-bottom.
	ColumnsA, ColumnsB [][]ifaces.Column
	// FiltersA and FiltersB are the filters of the left-to-right side.
	FiltersA, FiltersB []ifaces.Column
}

// NewProjection constructs a projection. Will panic if it is mal-formed
func NewProjection(round int, id ifaces.QueryID, inp ProjectionInput) Projection {
	return NewProjectionMultiAry(round, id, ProjectionMultiAryInput{
		ColumnsA: [][]ifaces.Column{inp.ColumnA},
		ColumnsB: [][]ifaces.Column{inp.ColumnB},
		FiltersA: []ifaces.Column{inp.FilterA},
		FiltersB: []ifaces.Column{inp.FilterB},
	})
}

// NewProjectionMultiAry returns a new [Projection] query object.
func NewProjectionMultiAry(
	round int,
	id ifaces.QueryID,
	inp ProjectionMultiAryInput,
) Projection {

	if len(inp.ColumnsA) == 0 || len(inp.ColumnsB) == 0 {
		utils.Panic("A and B must have at least one table: len(A)=%v, len(B)=%v", len(inp.ColumnsA), len(inp.ColumnsB))
	}

	if len(inp.ColumnsA[0]) == 0 || len(inp.ColumnsB[0]) == 0 {
		utils.Panic("A and B must have at least one column: len(A[0])=%v, len(B[0])=%v", len(inp.ColumnsA[0]), len(inp.ColumnsB[0]))
	}

	var (
		numCols   = len(inp.ColumnsA[0])
		numPartsA = len(inp.ColumnsA)
		numPartsB = len(inp.ColumnsB)
		sizeA     = inp.ColumnsA[0][0].Size()
		sizeB     = inp.ColumnsB[0][0].Size()
	)

	if len(inp.FiltersA) != numPartsA || len(inp.FiltersB) != numPartsB {
		utils.Panic("A and B must have the same number of filters: len(A)=%v, len(B)=%v", len(inp.FiltersA), len(inp.FiltersB))
	}

	for i := range inp.ColumnsA {
		if len(inp.ColumnsA[i]) != numCols {
			utils.Panic("All table must have the same number of columns: len(A[%v])=%v, len(A[0])=%v", i, len(inp.ColumnsA[i]), numCols)
		}

		size := ifaces.AssertSameLength(inp.ColumnsA[i]...)

		if i == 0 {
			sizeA = size
		}

		if size != sizeA {
			utils.Panic("All table must have the same number of columns: len(A[%v])=%v, len(A[0])=%v", i, len(inp.ColumnsA), numCols)
		}

		if sizeA != inp.FiltersA[i].Size() {
			utils.Panic("A[%v] and its filter do not have the same column sizes", i)
		}
	}

	for i := range inp.ColumnsB {
		if len(inp.ColumnsB[i]) != numCols {
			utils.Panic("All table must have the same number of columns: len(B[%v])=%v, numCols=%v", i, len(inp.ColumnsB[i]), numCols)
		}

		size := ifaces.AssertSameLength(inp.ColumnsB[i]...)

		if i == 0 {
			sizeB = size
		}

		if size != sizeB {
			utils.Panic("All table must have the same number of columns: len(B[%v])=%v, len(B[0])=%v", i, len(inp.ColumnsB), numCols)
		}

		if sizeB != inp.FiltersB[i].Size() {
			utils.Panic("B[%v] and its filter do not have the same column sizes", i)
		}
	}

	return Projection{Round: round, ID: id, Inp: inp, uuid: uuid.New()}
}

// Name implements the [ifaces.Query] interface
func (p Projection) Name() ifaces.QueryID {
	return p.ID
}

// Check implements the [ifaces.Query] interface
func (p Projection) Check(run ifaces.Runtime) error {

	// The function is implemented by creating two iterator functions and
	// checking that they yield the same values.
	var (
		aCurrPart, aCurrRow = 0, 0
		bCurrPart, bCurrRow = 0, 0
		err                 error
	)

	nextA := func() ([]field.Element, bool) {
		for {

			if aCurrRow >= p.Inp.FiltersA[aCurrPart].Size() {
				return nil, false
			}

			fa := p.Inp.FiltersA[aCurrPart].GetColAssignmentAt(run, aCurrRow)

			if fa.IsZero() {
				aCurrPart++
				if aCurrPart == len(p.Inp.ColumnsA) {
					aCurrPart = 0
					aCurrRow++
				}
				continue
			}

			var res []field.Element
			for _, col := range p.Inp.ColumnsA[aCurrPart] {
				res = append(res, col.GetColAssignmentAt(run, aCurrRow))
			}

			aCurrPart++
			if aCurrPart == len(p.Inp.ColumnsA) {
				aCurrPart = 0
				aCurrRow++
			}

			return res, true
		}
	}

	nextB := func() ([]field.Element, bool) {
		for {

			if bCurrRow >= p.Inp.FiltersB[bCurrPart].Size() {
				return nil, false
			}

			fb := p.Inp.FiltersB[bCurrPart].GetColAssignmentAt(run, bCurrRow)

			if fb.IsZero() {
				bCurrPart++
				if bCurrPart == len(p.Inp.ColumnsB) {
					bCurrPart = 0
					bCurrRow++
				}
				continue
			}

			var res []field.Element
			for _, col := range p.Inp.ColumnsB[bCurrPart] {
				res = append(res, col.GetColAssignmentAt(run, bCurrRow))
			}

			bCurrPart++
			if bCurrPart == len(p.Inp.ColumnsB) {
				bCurrPart = 0
				bCurrRow++
			}

			return res, true
		}
	}

mainLoop:
	for {
		a, aOk := nextA()
		b, bOk := nextB()

		if !aOk && !bOk {
			break mainLoop
		}

		if aOk != bOk {
			err = fmt.Errorf("a and b must yield the same number of rows, a %v b %v", aOk, bOk)
			break mainLoop
		}

		if len(a) != len(b) {
			// Note: this is redundant with the constructor's check, this should
			// not be a runtime error.
			panic("A and B must yield the same number of columns")
		}

		for i := range a {
			if !a[i].Equal(&b[i]) {
				err = fmt.Errorf("a and b must yield the same values, a=%v b=%v", vector.Prettify(a), vector.Prettify(b))
				break mainLoop
			}
		}
	}

	if err == nil {
		return nil
	}

	// Error mode, we print everything side by side. We reinitialize the iterator
	aCurrPart, aCurrRow = 0, 0
	bCurrPart, bCurrRow = 0, 0
	numSoFar := 0

	fmt.Printf("\n==============================\n")
	fmt.Printf("DEBUG FOR %v\n", p.ID)
	fmt.Printf("==============================\n")

	for {
		a, aOk := nextA()
		b, bOk := nextB()
		numSoFar++

		if !aOk && !bOk {
			break
		}

		aMsg := "<N/A>"
		bMsg := "<N/A>"

		if aOk {
			aMsg = vector.Prettify(a)
		}

		if bOk {
			bMsg = vector.Prettify(b)
		}

		fmt.Printf("%v | %v | %v\n", numSoFar, aMsg, bMsg)
	}

	fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxxxxxxx\n")

	return err
}

// GnarkCheck implements the [ifaces.Query] interface. It will panic in this
// construction because we do not have a good way to check the query within a
// circuit
func (i Projection) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("UNSUPPORTED : can't check an Projection query directly into the circuit")
}

// GetShiftedRelatedColumns returns the list of the [HornerParts.Selectors] found
// in the query. This is used to check if the query is compatible with
// Wizard distribution.
//
// Note: the fact that this method is implemented makes [Inclusion] satisfy
// an anonymous interface that is matched to detect queries that are
// incompatible with wizard distribution. So we should not rename or remove
// this implementation without doing the corresponding changes in the
// distributed package. Otherwise, this will silence the checks that we are
// doing.
func (p Projection) GetShiftedRelatedColumns() []ifaces.Column {

	res := []ifaces.Column{}

	for _, f := range p.Inp.FiltersA {
		if f.IsComposite() {
			res = append(res, f)
		}
	}

	for _, f := range p.Inp.FiltersB {
		if f.IsComposite() {
			res = append(res, f)
		}
	}

	for i := range p.Inp.ColumnsA {
		for _, col := range p.Inp.ColumnsA[i] {
			if col.IsComposite() {
				res = append(res, col)
			}
		}
	}

	for i := range p.Inp.ColumnsB {
		for _, col := range p.Inp.ColumnsB[i] {
			if col.IsComposite() {
				res = append(res, col)
			}
		}
	}

	return res
}

func (p Projection) UUID() uuid.UUID {
	return p.uuid
}
