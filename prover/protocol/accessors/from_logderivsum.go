package accessors

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

const (
	LOGDERIVSUM_ACCESSOR = "LOGDERIVSUM_ACCESSOR"
)

// FromLogDerivSumAccessor implements [ifaces.Accessor] and accesses the result of
// a [query.LogDerivativeSum].
type FromLogDerivSumAccessor struct {
	// Q is the underlying query whose parameters are accessed by the current
	// [ifaces.Accessor].
	Q query.LogDerivativeSum
}

func (l *FromLogDerivSumAccessor) IsBase() bool {
	return false
}

func (l *FromLogDerivSumAccessor) GetValBase(run ifaces.Runtime) (field.Element, error) {
	//TODO implement me
	panic("implement me")
}

func (l *FromLogDerivSumAccessor) GetValExt(run ifaces.Runtime) fext.Element {
	params := run.GetParams(l.Q.ID).(query.LogDerivSumParams)
	return params.Sum.GetExt()
}

func (l *FromLogDerivSumAccessor) GetFrontendVariableBase(api frontend.API, c ifaces.GnarkRuntime) (koalagnark.Element, error) {
	panic("unsupported, log derivative sums are always over field extensions")
}

func (l *FromLogDerivSumAccessor) GetFrontendVariable(api frontend.API, c ifaces.GnarkRuntime) koalagnark.Element {
	panic("unsupported, log derivative sums are always over field extensions")
}

// NewLogDerivSumAccessor creates an [ifaces.Accessor] returning the opening
// point of a [query.LogDerivativeSum].
func NewLogDerivSumAccessor(q query.LogDerivativeSum) ifaces.Accessor {
	return &FromLogDerivSumAccessor{Q: q}
}

// Name implements [ifaces.Accessor]
func (l *FromLogDerivSumAccessor) Name() string {
	return fmt.Sprintf("%v_%v", LOGDERIVSUM_ACCESSOR, l.Q.ID)
}

// String implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (l *FromLogDerivSumAccessor) String() string {
	return l.Name()
}

// GetVal implements [ifaces.Accessor]
func (l *FromLogDerivSumAccessor) GetVal(run ifaces.Runtime) field.Element {
	utils.Panic("Called GetVal on a FromLogDerivSumAccessor, %v, but it should call GetValExt", l.Name())
	return field.Element{} // not reachable
}

// GetFrontendVariable implements [ifaces.Accessor]
func (l *FromLogDerivSumAccessor) GetFrontendVariableExt(_ frontend.API, circ ifaces.GnarkRuntime) koalagnark.Ext {
	params := circ.GetParams(l.Q.ID).(query.GnarkLogDerivSumParams)
	return params.Sum
}

// AsVariable implements the [ifaces.Accessor] interface
func (l *FromLogDerivSumAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(l)
}

// Round implements the [ifaces.Accessor] interface
func (l *FromLogDerivSumAccessor) Round() int {
	return l.Q.Round
}
