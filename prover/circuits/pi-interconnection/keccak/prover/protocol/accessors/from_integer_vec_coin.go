package accessors

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// FromIntVecCoinPositionAccessor refers to a position of a random coin of type
// [coin.IntegerVec].
type FromIntVecCoinPositionAccessor struct {
	// Info points to the underlying coin on which the accessor points to.
	Info coin.Info
	// Pos indexes the pointed position in the coin.
	Pos int
}

// NewFromIntegerVecCoinPosition constructs an [ifaces.Accessor] object refering
// to a specific position of a coin of the [coin.IntegerVec]. It is used to build
// generic verifier columns.
func NewFromIntegerVecCoinPosition(info coin.Info, pos int) ifaces.Accessor {
	if info.Type != coin.IntegerVec {
		panic("expected an coin.IntegerVec")
	}
	if info.Size <= pos {
		utils.Panic("the coin has size %v, but requested position %v", info.Size, pos)
	}
	return &FromIntVecCoinPositionAccessor{
		Info: info,
		Pos:  pos,
	}
}

// Name implements [ifaces.Accessor]
func (c *FromIntVecCoinPositionAccessor) Name() string {
	return fmt.Sprintf("INT_VEC_COIN_ACCESSOR_%v_%v", c.Info.Name, c.Pos)
}

// String implements [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic.Metadata]
func (c *FromIntVecCoinPositionAccessor) String() string {
	return c.Name()
}

// GetVal implements [ifaces.Accessor]
func (c *FromIntVecCoinPositionAccessor) GetVal(run ifaces.Runtime) field.Element {
	return field.NewElement(uint64(run.GetRandomCoinIntegerVec(c.Info.Name)[c.Pos]))
}

// GetFrontendVariable implements [ifaces.Accessor]
func (c *FromIntVecCoinPositionAccessor) GetFrontendVariable(_ frontend.API, circ ifaces.GnarkRuntime) frontend.Variable {
	return circ.GetRandomCoinIntegerVec(c.Info.Name)[c.Pos]
}

// AsVariable implements the [ifaces.Accessor] interface
func (c *FromIntVecCoinPositionAccessor) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(c)
}

// Round implements the [ifaces.Accessor] interface
func (c *FromIntVecCoinPositionAccessor) Round() int {
	return c.Info.Round
}
