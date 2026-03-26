package accessors

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/symbolic"
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

// IsBase implements [ifaces.Accessor] and always returns false as grand-product
// always returns an extension field as they depends on randomness in a sound
// setting. For testing it is always possible to mock the FS to return base
// fields to make debugging easier but by default this is not possible.
func (l *FromGrandProductAccessor) IsBase() bool {
	return false
}

func (l *FromGrandProductAccessor) GetValBase(run ifaces.Runtime) (field.Element, error) {
	panic(fmt.Sprintf("Called GetValBase on a FromGrandProductAccessor, %v", l.Name()))

}

func (l *FromGrandProductAccessor) GetValExt(run ifaces.Runtime) fext.Element {
	return run.GetParams(l.Q.ID).(query.GrandProductParams).ExtY
}

func (l *FromGrandProductAccessor) GetFrontendVariableBase(api frontend.API, c ifaces.GnarkRuntime) (koalagnark.Element, error) {
	//TODO implement me
	panic("implement me")
}

func (l *FromGrandProductAccessor) GetFrontendVariable(api frontend.API, c ifaces.GnarkRuntime) koalagnark.Element {
	//TODO implement me
	panic("implement me")
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

// String implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (l *FromGrandProductAccessor) String() string {
	return l.Name()
}

// GetVal implements [ifaces.Accessor] and returns the opening point of a
// grand product opening.
func (l *FromGrandProductAccessor) GetVal(run ifaces.Runtime) field.Element {
	panic(fmt.Sprintf("Called GetVal on a FromGrandProductAccessor, %v", l.Name()))
}

// GetFrontendVariable implements [ifaces.Accessor]
func (l *FromGrandProductAccessor) GetFrontendVariableExt(_ frontend.API, circ ifaces.GnarkRuntime) koalagnark.Ext {
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
