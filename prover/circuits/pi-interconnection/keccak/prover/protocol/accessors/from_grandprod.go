package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

const (
	GRANDPRODUCT_ACCESSOR = "GRANDPRODUCT_ACCESSOR"
)

// FromGrandProductAccessor implements [ifaces.Accessor] and accesses the result of
// a [query.GrandProduct].
type FromGrandProductAccessor struct {
	// Q is the underlying query whose parameters are accessed by the current
	// [ifaces.Accessor].
	Q query.GrandProduct
}

// NewGrandProductAccessor creates an [ifaces.Accessor] returning the opening
// point of a [query.GrandProduct].
func NewGrandProductAccessor(q query.GrandProduct) ifaces.Accessor {
	return &FromGrandProductAccessor{Q: q}
}

// Name implements [ifaces.Accessor]
func (l *FromGrandProductAccessor) Name() string {
	return fmt.Sprintf("%v_%v", GRANDPRODUCT_ACCESSOR, l.Q.ID)
}

// String implements [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic.Metadata]
func (l *FromGrandProductAccessor) String() string {
	return l.Name()
}

// GetVal implements [ifaces.Accessor]
func (l *FromGrandProductAccessor) GetVal(run ifaces.Runtime) field.Element {
	params := run.GetParams(l.Q.ID).(query.GrandProductParams)
	return params.Y
}

// GetFrontendVariable implements [ifaces.Accessor]
func (l *FromGrandProductAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	params := circ.GetParams(l.Q.ID).(query.GnarkGrandProductParams)
	return params.Prod
}

// AsVariable implements the [ifaces.Accessor] interface
func (l *FromGrandProductAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(l)
}

// Round implements the [ifaces.Accessor] interface
func (l *FromGrandProductAccessor) Round() int {
	return l.Q.Round
}
