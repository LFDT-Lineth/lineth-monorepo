package smartvectorsext

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/mempoolext"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Compute the FFT of a vector
// Decimation:
//   - Either DIT : input in bit-reverse order - output in normal order
//   - Or DIF : input in normal order - output in bit reversed order
//
// BitReverse:
//   - If set to true, this cancels the decimation and
//     forces : input normal order - output normal order
//
// CosetRatio > CosetID:
//   - Specifies on which coset to perform the operation
//   - 0, 0 to assert that the transformation should not be done over a coset
func FFT(v smartvectors.SmartVector, decimation fft.Decimation, bitReverse bool, cosetRatio int, cosetID int, pool mempoolext.MemPool) smartvectors.SmartVector {

	// Sanity-check on the size of the vector v
	assertPowerOfTwoLen(v.Len())

	if pool != nil && pool.Size() != v.Len() {
		utils.Panic("provided a mempool with size %v but processing vectors of size %v", pool.Size(), v.Len())
	}

	/*
		Try to capture the special cases
	*/
	switch x := v.(type) {
	case *ConstantExt:
		if x.val.IsZero() {
			// The fft of the zero vec is zero
			return x.DeepCopy()
		}

		if cosetID == 0 && cosetRatio == 0 {
			// The FFT is a (c*N, 0, 0, ...), no matter the bitReverse or decimation
			// It's a multiple of the first Lagrange polynomial.
			constTerm := fext.NewElement(uint64(x.length), 0)
			constTerm.Mul(&constTerm, &x.val)
			return NewPaddedCircularWindowExt([]fext.Element{constTerm}, fext.Zero(), 0, x.length)
		}
	case *PaddedCircularWindowExt:
		// The polynomial is the constant polynomial, response does not depends on the decimation
		// or bitReverse
		interval := x.interval()
		if interval.IntervalLen == 1 && interval.Start() == 0 && x.paddingVal.IsZero() {
			// In this case, the response is a constant vector
			return NewConstantExt(x.window[0], x.Len())
		}
	}

	// Else : we run the FFT directly
	var res *PooledExt
	if pool != nil {
		res = AllocFromPoolExt(pool)
	} else {
		res = &PooledExt{RegularExt: make([]fext.Element, v.Len())}
	}

	v.WriteInSliceExt(res.RegularExt)

	domain := fft.NewDomain(v.Len())
	opt := fft.EmptyOption()

	if cosetID != 0 || cosetRatio != 0 {
		opt = fft.OnCoset()
		domain = domain.WithCustomCoset(cosetRatio, cosetID)
	}

	if decimation == fft.DIT {
		// Optionally, bitReverse the input
		if bitReverse {
			fft.BitReverseExt(res.RegularExt)
		}
		domain.FFTExt(res.RegularExt, fft.DIT, opt)
	} else {
		// Likewise, the optionally rearrange the input in correct order
		domain.FFTExt(res.RegularExt, fft.DIF, opt)
		if bitReverse {
			fft.BitReverseExt(res.RegularExt)
		}
	}

	return res
}

// Compute the FFT inverse of a vector
// Decimation:
//   - Either DIT : input in bit-reverse order - output in normal order
//   - Or DIF : input in normal order - output in bit reversed order
//
// BitReverse:
//   - If set to true, this cancels the decimation and
//     forces : input normal order - output normal order
//
// CosetRatio > CosetID:
//   - Specifies on which coset to perform the operation
//   - 0, 0 to assert that the transformation should not be done over a coset
func FFTInverse(v smartvectors.SmartVector, decimation fft.Decimation, bitReverse bool, cosetRatio int, cosetID int, pool mempoolext.MemPool) smartvectors.SmartVector {

	// Sanity-check on the size of the vector v
	assertPowerOfTwoLen(v.Len())

	if pool != nil && pool.Size() != v.Len() {
		utils.Panic("provided a mempool with size %v but processing vectors of size %v", pool.Size(), v.Len())
	}

	/*
		Try to capture the special cases
	*/
	switch x := v.(type) {
	case *ConstantExt:
		if x.val.IsZero() {
			// The fft inverse of the zero vec is zero
			return x.DeepCopy()
		}

		if cosetID == 0 && cosetRatio == 0 {
			// It's the constant polynomial. If it is not on coset then there is a trick
			return NewPaddedCircularWindowExt([]fext.Element{x.val}, fext.Zero(), 0, x.length)
		}

	case *PaddedCircularWindowExt:
		// It's a multiple of the first Lagrange polynomial c * (1 + x + x^2 + x^3 + ...)
		// The response is (c) = (c/N, c/N, c/N, ...)
		interval := x.interval()
		if interval.IntervalLen == 1 && interval.Start() == 0 && x.paddingVal.IsZero() {
			constTerm := fext.NewElement(uint64(x.Len()), 0)
			constTerm.Inverse(&constTerm)
			constTerm.Mul(&constTerm, &x.window[0])
			// In this case, the response is a constant vector
			return NewConstantExt(constTerm, x.Len())
		}
	}

	// Else : we run the FFTInverse directly
	var res *PooledExt
	if pool != nil {
		res = AllocFromPoolExt(pool)
	} else {
		res = &PooledExt{RegularExt: make([]fext.Element, v.Len())}
	}

	opt := fft.EmptyOption()
	v.WriteInSliceExt(res.RegularExt)

	domain := fft.NewDomain(v.Len())
	if cosetID != 0 || cosetRatio != 0 {
		// Optionally equip the domain with a coset
		opt = fft.OnCoset()
		domain = domain.WithCustomCoset(cosetRatio, cosetID)
	}

	if decimation == fft.DIF {
		// Optionally, bitReverse the output
		domain.FFTInverseExt(res.RegularExt, fft.DIF, opt)
		if bitReverse {
			fft.BitReverseExt(res.RegularExt)
		}
	} else {
		// Likewise, the optionally rearrange the input in correct order
		if bitReverse {
			fft.BitReverseExt(res.RegularExt)
		}
		domain.FFTInverseExt(res.RegularExt, fft.DIT, opt)
	}
	return res
}
