package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/google/uuid"
)

// Permutation is a predicate that assess that two tables contains the same rows
// up to a permutation. The tables can contain several columns and they can be
// described in a fractioned way: the table is the union of the rows of several
// tables.
type Permutation struct {
	// A and B represent the tables on both sides of the argument. The
	// permutation can be fractionned (len(A) = len(B) > 1) and it can be
	// multi-column (len(A[*]) = len(B[*]) > 1.
	A, B [][]ifaces.Column
	// ID is the string indentifier of the query.
	ID   ifaces.QueryID
	uuid uuid.UUID `serde:"omit"`
}

// NewPermutation constructs a new permutation query and performs all the
// well-formedness sanity-checks. In case of failure, it will panic.
func NewPermutation(id ifaces.QueryID, a, b [][]ifaces.Column) Permutation {

	var (
		nCol     = len(a[0])
		totalRow = [2]int{}
	)

	for side, aOrB := range [2][][]ifaces.Column{a, b} {
		for frag := range aOrB {

			if len(aOrB[frag]) != nCol {
				utils.Panic("all tables must have the same number of columns")
			}

			for _, col := range aOrB[frag] {
				col.MustExists()
			}

			sizeFrag, err := utils.AllReturnEqual(
				ifaces.Column.Size,
				aOrB[frag],
			)

			if err != nil {
				utils.Panic("all tables must be sets of columns with the same size")
			}

			totalRow[side] += sizeFrag
		}
	}

	if totalRow[0] != totalRow[1] {
		utils.Panic("a (numRows: %v, colId: %v) and b (numRows: %v, colId: %v) must have the same total number of rows, query id: %v", totalRow[0], a[0][0].GetColID(), totalRow[1], b[0][0].GetColID(), id)
	}

	return Permutation{A: a, B: b, ID: id, uuid: uuid.New()}
}

// Name implements the [ifaces.Query] interface
func (r Permutation) Name() ifaces.QueryID {
	return r.ID
}

// Check probabilistically checks whether the permutation predicates holds. The
// test works by incrementally computes the products:
//
//	\prod_i (\gamma + \sum_j C{i, j} \alpha^j)
//
// With overhelming probability, if the predicate is wrong then then the
// products will be unequal and this will be equal if the predicate is
// satisfied.
func (r Permutation) Check(run ifaces.Runtime) error {

	var (
		numCol    = len(r.A[0])
		prods     = []field.Element{field.One(), field.One()}
		randGamma = field.Element{}
		randAlpha = field.Element{}
	)

	randGamma.SetRandom()
	randAlpha.SetRandom()

	for k, aOrB := range [2][][]ifaces.Column{r.A, r.B} {
		for frag := range aOrB {
			var (
				tab        = make([]ifaces.ColAssignment, numCol)
				numRowFrag = aOrB[frag][0].Size()
				gamma      = smartvectors.NewConstant(randGamma, numRowFrag)
			)

			for col := range aOrB[frag] {
				tab[col] = aOrB[frag][col].GetColAssignment(run)
			}

			collapsed := smartvectors.PolyEval(append(tab, gamma), randAlpha)

			for row := 0; row < collapsed.Len(); row++ {
				tmp := collapsed.Get(row)
				prods[k].Mul(&prods[k], &tmp)
			}
		}
	}

	if prods[0] != prods[1] {
		return fmt.Errorf("the permutation query %v is not satisfied", r.ID)
	}

	return nil
}

// CheckPermutation manually checks that a permutation argument is satisfied.
func CheckPermutation(a, b []ifaces.ColAssignment) error {
	/*
		Sample a random element alpha, usefull for multivalued inclusion checks
		It allows to reference multiple number through a linear combination
	*/
	var alpha, beta field.Element
	_, err := alpha.SetRandom()
	_, err2 := beta.SetRandom()
	if err != nil || err2 != nil {
		utils.Panic("Could not generate a random number %v %v", err, err2)
	}

	/*
		Sanity-check both sides should have the same number of cols
	*/
	if len(a) != len(b) {
		utils.Panic("Not the same number of columns %v %v", len(a), len(b))
	}

	nRow := a[0].Len()
	/*
		Sanity-check, all sample should have the same number of rows.
		This might become an error later, but this is easy to change.
	*/
	for i := range a {
		if a[i].Len() != nRow {
			utils.Panic("Row %v of a has an inconsistent size. Expected %v but got %v", i, nRow, a[i].Len())
		}
		if b[i].Len() != nRow {
			utils.Panic("Row %v of b has an inconsistent size. Expected %v but got %v", i, nRow, b[i].Len())
		}
	}
	if nRow != b[0].Len() {
		return fmt.Errorf("a and b do not have the same length : %v != %v", a[0].Len(), b[0].Len())
	}

	prodA := field.One()
	prodB := field.One()

	for i := 0; i < nRow; i++ {
		// The product for a
		tmp := rowLinComb(alpha, i, a)
		tmp.Add(&tmp, &beta)
		prodA.Mul(&prodA, &tmp)

		// The product for b
		tmp = rowLinComb(alpha, i, b)
		tmp.Add(&tmp, &beta)
		prodB.Mul(&prodB, &tmp)
	}

	// At the end, the two product should be equals
	if prodA != prodB {
		return fmt.Errorf("the permutation check rejected")
	}

	return nil
}

// GnarkCheck will panic in this construction because we do not have a good way
// to check the query within a circuit
func (p Permutation) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("UNSUPPORTED : can't check an permutation query directly into the circuit")
}

// GetShiftedRelatedColumns returns the list of the [HornerParts.Selectors]
// found in the query. This is used to check if the query is compatible with
// Wizard distribution.
//
// Note: the fact that this method is implemented makes [Inclusion] satisfy
// an anonymous interface that is matched to detect queries that are
// incompatible with wizard distribution. So we should not rename or remove
// this implementation without doing the corresponding changes in the
// distributed package. Otherwise, this will silence the checks that we are
// doing.
func (p Permutation) GetShiftedRelatedColumns() []ifaces.Column {

	res := []ifaces.Column{}

	for frag := range p.A {
		for _, col := range p.A[frag] {
			if col.IsComposite() {
				res = append(res, col)
			}
		}
	}

	for frag := range p.B {
		for _, col := range p.B[frag] {
			if col.IsComposite() {
				res = append(res, col)
			}
		}
	}

	return res
}

func (p Permutation) UUID() uuid.UUID {
	return p.uuid
}
