package variables

import (
	"math/big"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/symbolic"
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
	omega, err := fft.Generator(uint64(size))
	if err != nil {
		panic(err)
	}
	omegaQNumCos, err1 := fft.Generator(uint64(size * cosetRatio))
	if err1 != nil {
		panic(err1)
	}
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
func (x X) GnarkEvalNoCoset(size int) []koalagnark.Element {
	res_ := x.EvalCoset(size, 0, 1, false)
	res := make([]koalagnark.Element, res_.Len())
	for i := range res {
		tmp := res_.Get(i)
		res[i] = koalagnark.NewElementFromKoala(tmp)
	}
	return res
}

func (x X) IsBase() bool {
	return true
}
