package query

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/google/uuid"
)

/*
Enforces a range constraint, that all elements in the handle must
be within range [0, B)

Where B is a power of two
*/
type Range struct {
	ID ifaces.QueryID
	// Maybe we should enforce that the handle is a natural one here
	Handle ifaces.Column
	// Upper-bound
	B    int
	uuid uuid.UUID `serde:"omit"`
}

/*
Constructor for range constraints also makes the input validation
*/
func NewRange(id ifaces.QueryID, h ifaces.Column, b int) Range {
	return Range{
		ID:     id,
		B:      b,
		Handle: h,
		uuid:   uuid.New(),
	}
}

// Name implements the [ifaces.Query] interface
func (r Range) Name() ifaces.QueryID {
	return r.ID
}

/*
Test that the range checks hold
*/
func (r Range) Check(run ifaces.Runtime) error {

	b := field.NewElement(uint64(r.B))

	if run == nil {
		panic("got a nil runtime")
	}

	if r.Handle == nil {
		utils.Panic("handle was poisoned")
	}

	wit := r.Handle.GetColAssignment(run)
	for i := 0; i < wit.Len(); i++ {
		v := wit.Get(i)
		if v.Cmp(&b) >= 0 {
			return fmt.Errorf("range check failed %v (bound %v on %v)", r.ID, r.B, r.Handle.GetColID())
		}
	}

	return nil
}

// CheckGnark will panic in this construction because we do not have a good way
// to check the query within a circuit
func (r Range) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("UNSUPPORTED : can't check an inclusion query directly into the circuit")
}

func (r Range) UUID() uuid.UUID {
	return r.uuid
}
