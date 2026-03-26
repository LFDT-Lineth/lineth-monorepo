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

// FromLocalOpeningYAccessor implements [ifaces.Accessor] and accesses the result of
// a [query.LocalOpening].
type FromLocalOpeningYAccessor struct {
	// Q is the underlying query whose parameters are accessed by the current
	// [ifaces.Accessor].
	Q query.LocalOpening
	// QRound is the declaration round of the query
	QRound int
}

func (l *FromLocalOpeningYAccessor) IsBase() bool {
	return l.Q.Pol.IsBase()
}

func (l *FromLocalOpeningYAccessor) GetValBase(run ifaces.Runtime) (field.Element, error) {
	params := run.GetParams(l.Q.ID).(query.LocalOpeningParams)
	if !l.IsBase() {
		panic("not base")
	}
	return params.BaseY, nil
}

func (l *FromLocalOpeningYAccessor) GetValExt(run ifaces.Runtime) fext.Element {
	params := run.GetParams(l.Q.ID).(query.LocalOpeningParams)
	return params.ExtY
}

func (l *FromLocalOpeningYAccessor) GetFrontendVariableBase(api frontend.API, c ifaces.GnarkRuntime) (koalagnark.Element, error) {
	p := c.GetParams(l.Q.ID).(query.GnarkLocalOpeningParams)
	if !l.IsBase() {
		panic("not base")
	}
	return p.BaseY, nil
}

func (l *FromLocalOpeningYAccessor) GetFrontendVariableExt(api frontend.API, c ifaces.GnarkRuntime) koalagnark.Ext {
	p := c.GetParams(l.Q.ID).(query.GnarkLocalOpeningParams)
	if p.IsBase {
		return koalagnark.FromBaseVar(p.BaseY)
	}
	return p.ExtY
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

// String implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (l *FromLocalOpeningYAccessor) String() string {
	return l.Name()
}

// GetVal implements [ifaces.Accessor]
func (l *FromLocalOpeningYAccessor) GetVal(run ifaces.Runtime) field.Element {
	params := run.GetParams(l.Q.ID).(query.LocalOpeningParams)
	return params.BaseY
}

// GetFrontendVariable implements [ifaces.Accessor]
func (l *FromLocalOpeningYAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) koalagnark.Element {
	params := circ.GetParams(l.Q.ID).(query.GnarkLocalOpeningParams)
	if !l.IsBase() {
		utils.Panic("not base: %v", l.Q.ID)
	}
	return params.BaseY
}

// AsVariable implements the [ifaces.Accessor] interface
func (l *FromLocalOpeningYAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(l)
}

// Round implements the [ifaces.Accessor] interface
func (l *FromLocalOpeningYAccessor) Round() int {
	return l.QRound
}
