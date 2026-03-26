package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

var _ ifaces.Accessor = &FromUnivXAccessor{}

// FromUnivXAccessor implements [ifaces.Accessor]. It represents the "X" of a
// univariate evaluation query (see [query.UnivariateEval]).
type FromUnivXAccessor struct {
	// Q is the underlying univariate evaluation query
	Q query.UnivariateEval
	// Round is the declaration round of Q
	QRound int
}

// NewUnivariateX returns an [ifaces.Accessor] object symbolizing the evaluation
// point (the "X" value) of a [query.UnivariateEval]. `qRound` is must be the
// underlying declaration round of the query object.
func NewUnivariateX(q query.UnivariateEval, qround int) ifaces.Accessor {
	return &FromUnivXAccessor{
		Q:      q,
		QRound: qround,
	}
}

// Name implements [ifaces.Accessor]
func (u *FromUnivXAccessor) Name() string {
	return fmt.Sprintf("UNIV_X_ACCESSOR_%v", u.Q.QueryID)
}

// String implements [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic.Metadata]
func (u *FromUnivXAccessor) String() string {
	return u.Name()
}

// GetVal implements [ifaces.Accessor]
func (u *FromUnivXAccessor) GetVal(run ifaces.Runtime) field.Element {
	params := run.GetParams(u.Q.QueryID).(query.UnivariateEvalParams)
	return params.X
}

// GetFrontendVariable implements [ifaces.Accessor]
func (u *FromUnivXAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	params := circ.GetParams(u.Q.QueryID).(query.GnarkUnivariateEvalParams)
	return params.X
}

// AsVariable implements the [ifaces.Accessor] interface
func (u *FromUnivXAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(u)
}

// Round implements the [ifaces.Accessor] interface
func (u *FromUnivXAccessor) Round() int {
	return u.QRound
}
