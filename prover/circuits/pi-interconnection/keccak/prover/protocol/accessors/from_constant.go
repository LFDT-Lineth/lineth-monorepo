package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

var _ ifaces.Accessor = &FromConstAccessor{}

// FromConstAccessor implements [ifaces.Accessor]. It symbolizes a constant
// value and be used as input to functions that expects an [ifaces.Accessor] but
// where the caller is only interested in passing a constant value.
type FromConstAccessor struct {
	// F is the constant served by the accessor
	F field.Element
}

// NewConstant returns an [ifaces.Accessor] object representing a constant value.
func NewConstant(f field.Element) ifaces.Accessor {
	return &FromConstAccessor{F: f}
}

// Name implements [ifaces.Accessor]
func (c *FromConstAccessor) Name() string {
	return fmt.Sprintf("CONST_ACCESSOR_%v", c.F.String())
}

// String implements [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic.Metadata]
func (c *FromConstAccessor) String() string {
	return c.Name()
}

// GetVal implements [ifaces.Accessor]
func (c *FromConstAccessor) GetVal(run ifaces.Runtime) field.Element {
	return c.F
}

// GetFrontendVariable implements [ifaces.Accessor]
func (c *FromConstAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	return c.F
}

// AsVariable implements the [ifaces.Accessor] interface
func (c *FromConstAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewConstant(c.F)
}

// Round implements the [ifaces.Accessor] interface
func (c *FromConstAccessor) Round() int {
	return 0
}
