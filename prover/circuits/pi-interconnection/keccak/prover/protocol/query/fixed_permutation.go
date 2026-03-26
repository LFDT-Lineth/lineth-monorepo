package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/google/uuid"
)

/*
	Enforces a fixedPermutation constraint, that two handles must
	be a fixedpermutation of each other

*/
//fix permutation over splittings of two vectors
/* here vectors A_i,B_i are splitting of target vectors A*,B*
where a fixed permutation is applied between target vectors
i.e.,A* = A0||...||An, B* = B0||...||Bn and B* = s(A*) for the fixed permutation s */
type FixedPermutation struct {
	ID ifaces.QueryID
	//splittings
	A, B []ifaces.Column
	/*
		the permutation 's' also can be defined by a set of splittings s_i and s_{id,i} where
		s(s_{id,i})=s_i. The columns s_{id,i} are called identity polynomials of permutation 's' and  are known by defult.
		So that the permutation can be determined only by the splittings 's_i'
	*/

	//fixed  permutation
	S []ifaces.ColAssignment

	// uuid is an internal UUID used for the serialization process
	uuid uuid.UUID `serde:"omit"`
}

/*
Constructor for fixedPermutation constraints also makes the input validation
*/
func NewFixedPermutation(id ifaces.QueryID, S []ifaces.ColAssignment, a, b []ifaces.Column) FixedPermutation {

	// recall that 'a' and 'b' have the same size
	for i := range a {
		a[i].MustExists()
	}

	for i := range b {
		b[i].MustExists()
	}

	return FixedPermutation{
		ID:   id,
		A:    a,
		B:    b,
		S:    S,
		uuid: uuid.New(),
	}
}

// Name implements the [ifaces.Query] interface
func (r FixedPermutation) Name() ifaces.QueryID {
	return r.ID
}

// Check implements the [ifaces.Query] interface
func (r FixedPermutation) Check(run ifaces.Runtime) error {
	/*
		They should have the same size and it should be tested
		prior to calling check
	*/
	a := make([]ifaces.ColAssignment, len(r.A))
	b := make([]ifaces.ColAssignment, len(r.B))

	// Populate the `a`
	for i, pol := range r.A {
		a[i] = pol.GetColAssignment(run)
	}

	// Populate the `b`
	for i, pol := range r.B {
		b[i] = pol.GetColAssignment(run)
	}

	return checkFixedPermutation(a, b, r.S)
}

// checkFixedPermutation checks a fixedpermutation query manually.
func checkFixedPermutation(a, b []ifaces.ColAssignment, s []ifaces.ColAssignment) error {

	var (
		a_ = make([]field.Element, 0)
		s_ = make([]field.Element, 0)
		b_ = make([]field.Element, 0)
	)

	for i := range a {
		a_ = append(a_, a[i].IntoRegVecSaveAlloc()...)

	}

	for i := range b {
		s_ = append(s_, s[i].IntoRegVecSaveAlloc()...)
		b_ = append(b_, b[i].IntoRegVecSaveAlloc()...)
	}

	for i := range b_ {

		k := int(s_[i].Uint64())
		x := b_[i]
		y := a_[k]
		if x != y {
			return fmt.Errorf("the fixed permutation does not work: a[%v] = %v but b[%v] = %v", k, y.String(), i, x.String())
		}
	}

	return nil
}

// GnarkCheck will panic in this construction because we do not have a good way
// to check the query within a circuit
func (f FixedPermutation) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("UNSUPPORTED : can't check an inclusion query directly into the circuit")
}

func (f FixedPermutation) UUID() uuid.UUID {
	return f.uuid
}
