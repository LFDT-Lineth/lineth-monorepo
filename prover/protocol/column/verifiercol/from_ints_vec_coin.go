package verifiercol

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Represents a columns instantiated by the values of a random indices list.
type fromIntVecCoinSettings struct {
	padding struct {
		IsPadded   bool
		PaddingVal field.Element
	}
}

// Options to pass to FIVC
type FivcOp func(*fromIntVecCoinSettings)

// Construct a new column from a `IntegerVec` coin
func NewFromIntVecCoin(comp *wizard.CompiledIOP, info coin.Info, ops ...FivcOp) ifaces.Column {

	// Sanity-checks the coin to have the right type
	if info.Type != coin.IntegerVec {
		utils.Panic("FromIntVecCoin : expected type integer vec")
	}

	// And apply the options
	settings := &fromIntVecCoinSettings{}
	for _, op := range ops {
		op(settings)
	}

	access := []ifaces.Accessor{}

	for i := 0; i < info.Size; i++ {
		access = append(access, accessors.NewFromIntegerVecCoinPosition(info, i))
	}

	if settings.padding.IsPadded {
		return NewFromAccessors(access, fext.Zero(), utils.NextPowerOfTwo(len(access)))
	}

	return NewFromAccessors(access, fext.Zero(), len(access))
}

// Passes a padding value to the Fivc
func RightPadZeroToNextPowerOfTwo(settings *fromIntVecCoinSettings) {
	// For sanity, the paddding can never happen over a split FIVC
	if settings.padding.IsPadded {
		panic("tried to pad a split FIVC vector")
	}

	settings.padding.IsPadded = true
}
