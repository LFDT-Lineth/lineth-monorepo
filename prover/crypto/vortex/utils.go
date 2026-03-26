package vortex

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

func init() {
	solver.RegisterHint(
		fftExtInvHintEmulated, fftExtInvHintNative,
	)
}

var (
	ErrSizeNotAMultipleOfFour = errors.New("size(inputs) should be a multiple of 4")
	ErrSizesDontMatch         = errors.New("size(inputs) should be equal to size(outputs)")
	ErrNotAPowerOfTwo         = errors.New("size(inputs) should be a power of two")
)

func fftInvHint(t koalagnark.VType) solver.Hint {
	if t == koalagnark.Native {
		return fftExtInvHintNative
	} else {
		return fftExtInvHintEmulated
	}
}

func fftExtInvHintEmulated(_ *big.Int, inputs []*big.Int, output []*big.Int) error {
	return emulated.UnwrapHint(inputs, output, fftExtInvHintNative)
}

// Each chunk of 4 inputs corresponds to a E4 element.
func fftExtInvHintNative(scalarField *big.Int, inputs, outputs []*big.Int) error {

	if len(inputs)%4 != 0 {
		return ErrSizeNotAMultipleOfFour
	}

	n := len(inputs) / 4

	_n := ecc.NextPowerOfTwo(uint64(n))
	if _n != uint64(n) {
		return ErrNotAPowerOfTwo
	}

	d := fft.NewDomain(uint64(n))

	_res := make([]fext.Element, n)
	for i := 0; i < n; i++ {
		_res[i].B0.A0.SetBigInt(inputs[4*i])
		_res[i].B0.A1.SetBigInt(inputs[4*i+1])
		_res[i].B1.A0.SetBigInt(inputs[4*i+2])
		_res[i].B1.A1.SetBigInt(inputs[4*i+3])
	}
	d.FFTInverseExt(_res, fft.DIF)
	utils.BitReverse(_res)

	// we're supposed to have tail of zeros. Check
	for i := len(outputs) / 4; i < len(_res); i++ {
		if !_res[i].IsZero() {
			return fmt.Errorf("fftExtInvHintNative: expected zero at position %d, got %s", i, _res[i].String())
		}
	}
	// now truncate to avoid returning the zeros. In non-native we range check
	// the results so it would lead to overhead
	_res = _res[:len(outputs)/4]

	for i := range _res {
		_res[i].B0.A0.BigInt(outputs[4*i])
		_res[i].B0.A1.BigInt(outputs[4*i+1])
		_res[i].B1.A0.BigInt(outputs[4*i+2])
		_res[i].B1.A1.BigInt(outputs[4*i+3])
	}

	return nil
}
