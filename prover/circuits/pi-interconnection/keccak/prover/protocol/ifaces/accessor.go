package ifaces

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

// Accessor represents a function that can be used to retrieve a field element
// value from a [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard.VerifierRuntime].
// It also satisfies the [symbolic.Metadata] interface so that it can be
// used within arithmetic expression. A good use-case example is using the
// evaluation point of a [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query.UnivariateEval]
// as part of an arithmetic expression.
type Accessor interface {
	symbolic.Metadata
	// Name returns a unique identifier for the accessor.
	Name() string
	// GetVal returns the value represented by the Accessor from a [Runtime]
	// object.
	GetVal(run Runtime) field.Element
	// GetFrontendVariable is as [Accessor.GetVal] but in a gnark circuit.
	GetFrontendVariable(api frontend.API, c GnarkRuntime) frontend.Variable
	// Round returns the definition round of the accessor.
	Round() int
	// AsVariable converts the accessor to a variable object.
	// Deprecated: use the new [symbolic] API, this function won't be needed anymore. We keep it since most uses of the symbolic package within this repository uses the old API, but this will be removed in the future.
	AsVariable() *symbolic.Expression
}
