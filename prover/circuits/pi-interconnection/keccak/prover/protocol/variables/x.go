package variables

import (
	"math/big"

	"github.com/consensys/gnark/frontend"
	sv "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

/*
Refers to an abstract variable X over which all polynomials
are defined.
*/
type X struct{}

// Construct a new variable for coin
func NewXVar() *symbolic.Expression {
	return symbolic.NewVariable(X{})
}

// to implement symbolic.Metadata
func (x X) String() string {
	// Double append/prepend to avoid confusion
	return "__X__"
}

// Returns an evaluation of the X, possibly over a coset. Pass
// `EvalCoset(size, 0, 0, false)` to directly evaluate over a coset
func (x X) EvalCoset(size, cosetId, cosetRatio int, shiftGen bool) sv.SmartVector {
	omega := fft.GetOmega(size)
	omegaQNumCos := fft.GetOmega(size * cosetRatio)
	omegaQNumCos.Exp(omegaQNumCos, big.NewInt(int64(cosetId)))

	omegaI := field.One()
	if shiftGen {
		omegaI = field.NewElement(field.MultiplicativeGen)
	}
	omegaI.Mul(&omegaI, &omegaQNumCos)

	// precomputations of the powers of omega, can be optimized if useful
	omegas := make([]field.Element, size)
	for i := range omegas {
		omegas[i] = omegaI
		omegaI.Mul(&omegaI, &omega)
	}

	return sv.NewRegular(omegas)
}

// Evaluate the variable, but not over a coset
func (x X) GnarkEvalNoCoset(size int) []frontend.Variable {
	res_ := x.EvalCoset(size, 0, 1, false)
	res := make([]frontend.Variable, res_.Len())
	for i := range res {
		res[i] = res_.Get(i)
	}
	return res
}
