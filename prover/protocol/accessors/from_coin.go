package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

var _ ifaces.Accessor = &FromCoinAccessor{}

// FromCoinAccessor implements [ifaces.Accessor] and represents the value of a
// [coin.Info] of type [coin.Field]. It is sometime used to supply a coin to
// a function requiring an accessor explcitly. For [github.com/consensys/linea-monorepo/prover/symbolic.Expression]
// this should not be necessary as [coin.Info] already implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata].
type FromCoinAccessor struct {
	// Info represents the underlying [coin.Info] being wrapped by the accessor.
	Info coin.Info
}

// NewFromCoin returns an [ifaces.Accessor] object symbolizing a
// [coin.Info]. The supplied [coin.Info] must be of type [coin.Field] or the
// function panics.
func NewFromCoin(info coin.Info) ifaces.Accessor {
	if info.Type != coin.Field && info.Type != coin.FieldFromSeed {
		utils.Panic("NewFromCoin expects a [coin.Field] or a [coin.FieldFromSeed] `info`, got `%v`", info.Type)
	}
	return &FromCoinAccessor{
		Info: info,
	}
}

// Name implements [ifaces.Accessor]
func (c *FromCoinAccessor) Name() string {
	return fmt.Sprintf("COIN_AS_ACCESSOR_%v", c.Info.Name)
}

// String implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (c *FromCoinAccessor) String() string {
	return c.Name()
}

// GetVal implements [ifaces.Accessor]
func (c *FromCoinAccessor) GetVal(run ifaces.Runtime) field.Element {
	return run.GetRandomCoinField(c.Info.Name)
}

// GetFrontendVariable implements [ifaces.Accessor]
func (c *FromCoinAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	return circ.GetRandomCoinField(c.Info.Name)
}

// AsVariable implements the [ifaces.Accessor] interface
func (c *FromCoinAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(c)
}

// Round implements the [ifaces.Accessor] interface
func (c *FromCoinAccessor) Round() int {
	return c.Info.Round
}
