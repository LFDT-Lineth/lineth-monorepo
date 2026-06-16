package fri

import (
	"math/bits"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// RSEncoder is a Reed-Solomon error correcting-code encoder and decoder.
type RSEncoder struct {
	// Domain is the codeword domain (cardinality N), used for the forward FFT.
	Domain *fft.Domain
	// smallDomain has cardinality PlainTextSize and is used for the interpolating
	// inverse FFT. The inverse FFT must run over the plaintext-sized domain so it
	// uses ωₙ (not ω_N) as its root and scales by 1/n (not 1/N).
	smallDomain   *fft.Domain
	PlainTextSize int
}

// NewEncoder constructs a ReedSolomonCodec and its inside domain.
func NewEncoder(N uint64, plainTextSize int) RSEncoder {

	// TODO: rename N
	if plainTextSize >= int(N) {
		panic("plainTextSize > N")
	}

	return RSEncoder{
		Domain:        fft.NewDomain(N),
		smallDomain:   fft.NewDomain(uint64(plainTextSize)),
		PlainTextSize: plainTextSize,
	}
}

// NewEncoderWithDomain constructs a ReedSolomonCodec.
func NewEncoderWithDomain(domain *fft.Domain, plainTextSize int) RSEncoder {
	return RSEncoder{
		Domain:        domain,
		smallDomain:   fft.NewDomain(uint64(plainTextSize)),
		PlainTextSize: plainTextSize,
	}
}

// RSEncode evalutes p on the N-th roots of unity (N must be > len(p))
// p is in Lagrange form
// it returns a copy of p
//
// The returned codeword is in N-bit-reversed order, not natural order: the
// evaluation at the natural-order point of index i is stored at position
// bitReverse(i). This is deliberate, it places the FRI conjugate pairs (the
// evaluations at x and -x folded together) at adjacent positions 2j and 2j+1,
// which is exactly the layout [foldLayerInternally] and the Merkle commitment
// expect. To recover natural order, bit-reverse the result.
//
// Optional fftOpts are forwarded to both internal FFTs (e.g. to cap inner
// parallelism with fft.WithNbTasks when Encode is itself called inside a
// parallel.Execute loop).
//
// TODO: codec -> enc for all functions
func (codec *RSEncoder) Encode(p []field.Element, fftOpt ...fft.Option) []field.Element {

	// get the size of p
	n := len(p)

	// create _p, a copy of p of size N (zero-padded)
	N := codec.Domain.Cardinality
	_p := make([]field.Element, N)
	copy(_p, p)

	// Lagrange normal to canonical bit-reversed (w.r.t. n). We place those
	// coefficients directly in N-bit-reversed order and use a DIT FFT, avoiding
	// the two explicit BitReverse passes previously needed for normal order.
	codec.smallDomain.FFTInverse(_p[:n], fft.DIF, fftOpt...)
	scatterBitReversedCoeffs(_p, n, int(N))
	codec.Domain.FFT(_p, fft.DIT, fftOpt...)

	// return _p
	return _p
}

// EncodeExt evaluates an extension-field polynomial on the codec domain.
// The input p is in Lagrange normal form over d; the output is a fresh
// extension polynomial in Lagrange normal form over codec.Domain.
//
// As with [RSEncoder.Encode], the returned codeword is in N-bit-reversed order
// (the evaluation of natural-order index i lives at position bitReverse(i)), so
// that FRI conjugate pairs land at adjacent positions 2j and 2j+1.
func (codec *RSEncoder) EncodeExt(p []field.Ext, fftOpt ...fft.Option) []field.Ext {
	n := len(p)

	N := codec.Domain.Cardinality
	_p := make([]field.Ext, N)
	copy(_p, p)

	codec.smallDomain.FFTInverseExt6(_p[:n], fft.DIF, fftOpt...)
	scatterBitReversedCoeffs(_p, n, int(N))
	codec.Domain.FFTExt6(_p, fft.DIT, fftOpt...)

	return _p
}

// InverseRate returns the inverse-rate of the code
func (codec *RSEncoder) InverseRate() int {
	return int(codec.Domain.Cardinality) / codec.PlainTextSize
}

// scatterBitReversedCoeffs expands n-bit-reversed coefficients into the
// matching N-bit-reversed zero-padded slots, in place.
func scatterBitReversedCoeffs[T any](p []T, n, N int) {
	if n <= 1 {
		return
	}
	shift := bits.TrailingZeros64(uint64(N)) - bits.TrailingZeros64(uint64(n))
	stride := 1 << shift
	for i := n - 1; i >= 0; i-- {
		p[i<<shift] = p[i]
	}
	if stride == 1 {
		return
	}
	var zero T
	for i := 1; i < n; i++ {
		if i&(stride-1) != 0 {
			p[i] = zero
		}
	}
}
