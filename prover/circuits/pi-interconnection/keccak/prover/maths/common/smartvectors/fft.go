package smartvectors

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/mempool"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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
func FFT(v SmartVector, decimation fft.Decimation, bitReverse bool, cosetRatio int, cosetID int, pool mempool.MemPool, opts ...fft.Option) SmartVector {

	// Sanity-check on the size of the vector v
	assertPowerOfTwoLen(v.Len())

	if pool != nil && pool.Size() != v.Len() {
		utils.Panic("provided a mempool with size %v but processing vectors of size %v", pool.Size(), v.Len())
	}

	/*
		Try to capture the special cases
	*/
	switch x := v.(type) {
	case *Constant:
		if x.Value.IsZero() {
			// The fft of the zero vec is zero
			return x.DeepCopy()
		}

		if cosetID == 0 && cosetRatio == 0 {
			// The FFT is a (c*N, 0, 0, ...), no matter the bitReverse or decimation
			// It's a multiple of the first Lagrange polynomial.
			constTerm := field.NewElement(uint64(x.Length))
			constTerm.Mul(&constTerm, &x.Value)
			return NewPaddedCircularWindow([]field.Element{constTerm}, field.Zero(), 0, x.Length)
		}
	case *PaddedCircularWindow:
		// The polynomial is the constant polynomial, response does not depends on the decimation
		// or bitReverse
		interval := x.Interval()
		if interval.IntervalLen == 1 && interval.Start() == 0 && x.PaddingVal_.IsZero() {
			// In this case, the response is a constant vector
			return NewConstant(x.Window_[0], x.Len())
		}
	}

	// Else : we run the FFT directly
	var res *Pooled
	if pool != nil {
		res = AllocFromPool(pool)
	} else {
		res = &Pooled{Regular: make([]field.Element, v.Len())}
	}

	v.WriteInSlice(res.Regular)

	domain := fft.NewDomain(v.Len())

	if cosetID != 0 || cosetRatio != 0 {
		opts = append(opts, fft.OnCoset())
		domain = domain.WithCustomCoset(cosetRatio, cosetID)
	}

	if decimation == fft.DIT {
		// Optionally, bitReverse the input
		if bitReverse {
			fft.BitReverse(res.Regular)
		}
		domain.FFT(res.Regular, fft.DIT, opts...)
	} else {
		// Likewise, the optionally rearrange the input in correct order
		domain.FFT(res.Regular, fft.DIF, opts...)
		if bitReverse {
			fft.BitReverse(res.Regular)
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
func FFTInverse(v SmartVector, decimation fft.Decimation, bitReverse bool, cosetRatio int, cosetID int, pool mempool.MemPool, opts ...fft.Option) SmartVector {

	// Sanity-check on the size of the vector v
	assertPowerOfTwoLen(v.Len())

	if pool != nil && pool.Size() != v.Len() {
		utils.Panic("provided a mempool with size %v but processing vectors of size %v", pool.Size(), v.Len())
	}

	/*
		Try to capture the special cases
	*/
	switch x := v.(type) {
	case *Constant:
		if x.Value.IsZero() {
			// The fft inverse of the zero vec is zero
			return x.DeepCopy()
		}

		if cosetID == 0 && cosetRatio == 0 {
			// It's the constant polynomial. If it is not on coset then there is a trick
			return NewPaddedCircularWindow([]field.Element{x.Value}, field.Zero(), 0, x.Length)
		}

	case *PaddedCircularWindow:
		// It's a multiple of the first Lagrange polynomial c * (1 + x + x^2 + x^3 + ...)
		// The response is (c) = (c/N, c/N, c/N, ...)
		interval := x.Interval()
		if interval.IntervalLen == 1 && interval.Start() == 0 && x.PaddingVal_.IsZero() {
			constTerm := field.NewElement(uint64(x.Len()))
			constTerm.Inverse(&constTerm)
			constTerm.Mul(&constTerm, &x.Window_[0])
			// In this case, the response is a constant vector
			return NewConstant(constTerm, x.Len())
		}
	}

	// Else : we run the FFTInverse directly
	var res *Pooled
	if pool != nil {
		res = AllocFromPool(pool)
	} else {
		res = &Pooled{Regular: make([]field.Element, v.Len())}
	}

	v.WriteInSlice(res.Regular)

	domain := fft.NewDomain(v.Len())
	if cosetID != 0 || cosetRatio != 0 {
		// Optionally equip the domain with a coset
		opts = append(opts, fft.OnCoset())
		domain = domain.WithCustomCoset(cosetRatio, cosetID)
	}

	if decimation == fft.DIF {
		// Optionally, bitReverse the output
		domain.FFTInverse(res.Regular, fft.DIF, opts...)
		if bitReverse {
			fft.BitReverse(res.Regular)
		}
	} else {
		// Likewise, the optionally rearrange the input in correct order
		if bitReverse {
			fft.BitReverse(res.Regular)
		}
		domain.FFTInverse(res.Regular, fft.DIT, opts...)
	}
	return res
}
