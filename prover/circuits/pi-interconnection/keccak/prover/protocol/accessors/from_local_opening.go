package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

// FromLocalOpeningYAccessor implements [ifaces.Accessor] and accesses the result of
// a [query.LocalOpening].
type FromLocalOpeningYAccessor struct {
	// Q is the underlying query whose parameters are accessed by the current
	// [ifaces.Accessor].
	Q query.LocalOpening
	// QRound is the declaration round of the query
	QRound int
}

// NewLocalOpeningAccessor creates an [ifaces.Accessor] returning the opening
// point of a [query.LocalOpening].
func NewLocalOpeningAccessor(q query.LocalOpening, qRound int) ifaces.Accessor {
	return &FromLocalOpeningYAccessor{Q: q, QRound: qRound}
}

// Name implements [ifaces.Accessor]
func (l *FromLocalOpeningYAccessor) Name() string {
	return fmt.Sprintf("LOCAL_OPENING_ACCESSOR_%v", l.Q.ID)
}

// String implements [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic.Metadata]
func (l *FromLocalOpeningYAccessor) String() string {
	return l.Name()
}

// GetVal implements [ifaces.Accessor]
func (l *FromLocalOpeningYAccessor) GetVal(run ifaces.Runtime) field.Element {
	params := run.GetParams(l.Q.ID).(query.LocalOpeningParams)
	return params.Y
}

// GetFrontendVariable implements [ifaces.Accessor]
func (l *FromLocalOpeningYAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	params := circ.GetParams(l.Q.ID).(query.GnarkLocalOpeningParams)
	return params.Y
}

// AsVariable implements the [ifaces.Accessor] interface
func (l *FromLocalOpeningYAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(l)
}

// Round implements the [ifaces.Accessor] interface
func (l *FromLocalOpeningYAccessor) Round() int {
	return l.QRound
}
