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
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

func init() {
	solver.RegisterHint(
		fftExtInvHintEmulated, fftExtInvHintNative,
	)
}

var (
	// ErrSizeNotAMultipleOfExtDegree is returned when the input size is not a
	// multiple of field.ExtensionDegree.
	ErrSizeNotAMultipleOfExtDegree = fmt.Errorf("size(inputs) should be a multiple of %d", field.ExtensionDegree)
	// ErrSizesDontMatch is returned when input and output slices have different sizes.
	ErrSizesDontMatch = errors.New("size(inputs) should be equal to size(outputs)")
	// ErrNotAPowerOfTwo is returned when the input size is not a power of two.
	ErrNotAPowerOfTwo = errors.New("size(inputs) should be a power of two")
)

func fftExtInvHintEmulated(_ *big.Int, inputs []*big.Int, output []*big.Int) error {
	return emulated.UnwrapHint(inputs, output, fftExtInvHintNative)
}

// Each chunk of field.ExtensionDegree (=6) inputs corresponds to one Ext element.
func fftExtInvHintNative(_ *big.Int, inputs, outputs []*big.Int) error {

	const d = field.ExtensionDegree

	if len(inputs)%d != 0 {
		return ErrSizeNotAMultipleOfExtDegree
	}

	n := len(inputs) / d

	_n := ecc.NextPowerOfTwo(uint64(n))
	if _n != uint64(n) {
		return ErrNotAPowerOfTwo
	}

	dom := fft.NewDomain(uint64(n))

	_res := make([]field.Ext, n)
	for i := 0; i < n; i++ {
		_res[i].B0.A0.SetBigInt(inputs[d*i+0])
		_res[i].B0.A1.SetBigInt(inputs[d*i+1])
		_res[i].B1.A0.SetBigInt(inputs[d*i+2])
		_res[i].B1.A1.SetBigInt(inputs[d*i+3])
		_res[i].B2.A0.SetBigInt(inputs[d*i+4])
		_res[i].B2.A1.SetBigInt(inputs[d*i+5])
	}
	dom.FFTInverseExt6(_res, fft.DIF)
	utils.BitReverse(_res)

	// we're supposed to have tail of zeros. Check
	for i := len(outputs) / d; i < len(_res); i++ {
		if !_res[i].IsZero() {
			return fmt.Errorf("fftExtInvHintNative: expected zero at position %d, got %s", i, _res[i].String())
		}
	}
	// now truncate to avoid returning the zeros. In non-native we range check
	// the results so it would lead to overhead
	_res = _res[:len(outputs)/d]

	for i := range _res {
		_res[i].B0.A0.BigInt(outputs[d*i+0])
		_res[i].B0.A1.BigInt(outputs[d*i+1])
		_res[i].B1.A0.BigInt(outputs[d*i+2])
		_res[i].B1.A1.BigInt(outputs[d*i+3])
		_res[i].B2.A0.BigInt(outputs[d*i+4])
		_res[i].B2.A1.BigInt(outputs[d*i+5])
	}

	return nil
}
