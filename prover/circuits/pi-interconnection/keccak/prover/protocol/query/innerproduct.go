package query

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/google/uuid"
)

// Represent a batch of inner-product <a, b0>, <a, b1>, <a, b2> ...
type InnerProduct struct {
	A    ifaces.Column
	Bs   []ifaces.Column
	ID   ifaces.QueryID
	uuid uuid.UUID `serde:"omit"`
}

// Inner product params
type InnerProductParams struct {
	Ys []field.Element
}

// Update the fiat-shamir state with inner-product params
func (ipp InnerProductParams) UpdateFS(state *fiatshamir.State) {
	state.UpdateVec(ipp.Ys)
}

// Constructor for inner-product.
// The list of polynomial Bs must be deduplicated.
func NewInnerProduct(id ifaces.QueryID, a ifaces.Column, bs ...ifaces.Column) InnerProduct {
	// Panics if there is a duplicate
	bsSet := collection.NewSet[ifaces.ColID]()

	if len(bs) == 0 {
		utils.Panic("Inner-product %v declared without bs", id)
	}

	a.MustExists()
	length := a.Size()

	for _, b := range bs {

		b.MustExists()
		if b.Size() != length {
			utils.Panic("bad size for %v, expected %v but got %v", b.GetColID(), b.Size(), length)
		}

		if len(b.GetColID()) == 0 {
			utils.Panic("Assigned a polynomial ifaces.QueryID with an empty length")
		}

		// Note: we used to check and panic for duplicates in the Horner
		// query.
		bsSet.Insert(b.GetColID())
	}

	return InnerProduct{ID: id, A: a, Bs: bs, uuid: uuid.New()}
}

// Constructor for fixed point univariate evaluation query parameters
func NewInnerProductParams(ys ...field.Element) InnerProductParams {
	return InnerProductParams{Ys: ys}
}

// Name implements the [ifaces.Query] interface
func (r InnerProduct) Name() ifaces.QueryID {
	return r.ID
}

// Check the inner-product manually
func (r InnerProduct) Check(run ifaces.Runtime) error {

	expecteds := run.GetParams(r.ID).(InnerProductParams)
	computed := r.Compute(run)

	// Prepare a nice error message in case we need it
	errMsg := fmt.Sprintf("Inner-product %v\n", r.ID)
	errorFlag := false

	for i := range computed {

		if expecteds.Ys[i] != computed[i] {
			errorFlag = true
			errMsg = fmt.Sprintf("%v\tFor witness <%v, %v> the alleged value is %v but the correct value is %v\n",
				errMsg, r.A.GetColID(), r.Bs[i].GetColID(), expecteds.Ys[i].String(), computed[i].String(),
			)
		}
	}

	if errorFlag {
		return errors.New(errMsg)
	}

	return nil
}

func (r InnerProduct) Compute(run ifaces.Runtime) []field.Element {

	res := make([]field.Element, len(r.Bs))
	a := r.A.GetColAssignment(run)

	for i := range r.Bs {

		b := r.Bs[i].GetColAssignment(run)
		ab := smartvectors.Mul(a, b)
		res[i] = smartvectors.Sum(ab)
	}

	return res
}

// Check the inner-product manually
func (r InnerProduct) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {

	wA := r.A.GetColAssignmentGnark(run)
	expecteds := run.GetParams(r.ID).(GnarkInnerProductParams)

	for i, b := range r.Bs {
		wB := b.GetColAssignmentGnark(run)

		// mul <- \sum_j wA * wB
		actualIP := frontend.Variable(0)
		for j := range wA {
			tmp := api.Mul(wA[j], wB[j])
			actualIP = api.Add(actualIP, tmp)
		}

		api.AssertIsEqual(expecteds.Ys[i], actualIP)
	}
}

func (r InnerProduct) UUID() uuid.UUID {
	return r.uuid
}
