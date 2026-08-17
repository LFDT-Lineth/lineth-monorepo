package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
)

// RSEncoder is a Reed-Solomon error correcting-code encoder and decoder.
type RSEncoder struct {
	// Domain is the codeword domain (cardinality N), used for the forward FFT.
	Domain *fft.Domain
	// smallDomain has cardinality PlainTextSize and is used for the interpolating
	// inverse FFT. The inverse FFT must run over the plaintext-sized domain so it
	// uses ωₙ (not ω_N) as its root and scales by 1/n (not 1/N).
	smallDomain *fft.Domain
	// cosetSmallDomain is only set for rate-two encoders: the plaintext-sized
	// domain shifted by ω_N (the codeword domain's generator). Evaluating the
	// interpolant on this coset yields exactly the odd codeword positions, so a
	// rate-two encode needs one half-size coset FFT instead of a full-size FFT.
	cosetSmallDomain *fft.Domain
	PlainTextSize    int
}

// NewEncoder constructs a ReedSolomonCodec and its inside domain.
func NewEncoder(n uint64, plainTextSize int) RSEncoder {

	if plainTextSize >= int(n) {
		panic("plainTextSize > N")
	}

	return newEncoderFromDomains(fft.NewDomain(n), plainTextSize)
}

// NewEncoderWithDomain constructs a ReedSolomonCodec.
func NewEncoderWithDomain(domain *fft.Domain, plainTextSize int) RSEncoder {
	return newEncoderFromDomains(domain, plainTextSize)
}

func newEncoderFromDomains(domain *fft.Domain, plainTextSize int) RSEncoder {
	enc := RSEncoder{
		Domain:        domain,
		smallDomain:   fft.NewDomain(uint64(plainTextSize)),
		PlainTextSize: plainTextSize,
	}
	if int(domain.Cardinality) == 2*plainTextSize {
		enc.cosetSmallDomain = fft.NewDomain(uint64(plainTextSize), fft.WithShift(domain.Generator))
	}
	return enc
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
func (enc *RSEncoder) Encode(p []field.Element, fftOpt ...fft.Option) []field.Element {
	_p := make([]field.Element, enc.Domain.Cardinality)
	enc.EncodeInto(p, _p, fftOpt...)
	return _p
}

// EncodeInto is [RSEncoder.Encode] writing the codeword into the caller-provided
// buffer out, whose length must be the codeword size. out must not alias p.
// Letting the caller place codewords in a shared slab avoids one large
// allocation per column; thousands of concurrent large allocations otherwise
// serialize on the runtime page heap and dominate one-shot commits.
func (enc *RSEncoder) EncodeInto(p []field.Element, out []field.Element, fftOpt ...fft.Option) {

	// get the size of p
	n := len(p)

	N := enc.Domain.Cardinality
	if uint64(len(out)) != N {
		panic("fri: EncodeInto: output length must be the codeword size")
	}
	_p := out

	// Rate-two fast path. In N-bit-reversed order the even codeword positions
	// (the evaluations on the plaintext subgroup, i.e. p itself) land in the
	// first half, at rev_n(j), and the odd positions (the evaluations on the
	// ω_N-shifted coset) land at n+rev_n(j). So the first half is a
	// bit-reversed copy of the input, and only the coset half needs FFT work:
	// interpolate with a DIT inverse FFT (which consumes the bit-reversed
	// copy directly and emits natural-order coefficients) and evaluate with a
	// half-size coset FFT. This replaces the full-size FFT and the standalone
	// coefficient bit-reversal of the generic path.
	if enc.cosetSmallDomain != nil && n == enc.PlainTextSize {
		bitReverseCopy(_p[:n], p)
		copy(_p[n:], _p[:n])
		enc.smallDomain.FFTInverse(_p[n:], fft.DIT, fftOpt...)
		enc.cosetSmallDomain.FFT(_p[n:], fft.DIF, append(fftOpt[:len(fftOpt):len(fftOpt)], fft.OnCoset())...)
		return
	}

	copy(_p, p)
	clear(_p[n:]) // the slab is caller-provided and may be dirty

	// IFFT(DIF) consumes natural-order Lagrange input and returns n-bit-reversed
	// coefficients. The n-to-N size change needs one normalization to natural
	// coefficient order, after which FFT(DIF) emits the N-bit-reversed codeword
	// directly.
	enc.smallDomain.FFTInverse(_p[:n], fft.DIF, fftOpt...)
	bitReverse(_p[:n])
	enc.Domain.FFT(_p, fft.DIF, fftOpt...)
}

// EncodeExt evaluates an extension-field polynomial on the enc domain.
// The input p is in Lagrange normal form over d; the output is a fresh
// extension polynomial in Lagrange normal form over enc.Domain.
//
// As with [RSEncoder.Encode], the returned codeword is in N-bit-reversed order
// (the evaluation of natural-order index i lives at position bitReverse(i)), so
// that FRI conjugate pairs land at adjacent positions 2j and 2j+1.
func (enc *RSEncoder) EncodeExt(p []field.Ext, fftOpt ...fft.Option) []field.Ext {
	_p := make([]field.Ext, enc.Domain.Cardinality)
	enc.EncodeExtInto(p, _p, fftOpt...)
	return _p
}

// EncodeExtInto is [RSEncoder.EncodeExt] writing the codeword into the
// caller-provided buffer out, whose length must be the codeword size. out must
// not alias p.
func (enc *RSEncoder) EncodeExtInto(p []field.Ext, out []field.Ext, fftOpt ...fft.Option) {
	n := len(p)

	N := enc.Domain.Cardinality
	if uint64(len(out)) != N {
		panic("fri: EncodeExtInto: output length must be the codeword size")
	}
	_p := out

	// Rate-two fast path; see EncodeInto for the layout argument.
	if enc.cosetSmallDomain != nil && n == enc.PlainTextSize {
		bitReverseCopy(_p[:n], p)
		copy(_p[n:], _p[:n])
		enc.smallDomain.FFTInverseExt6(_p[n:], fft.DIT, fftOpt...)
		enc.cosetSmallDomain.FFTExt6(_p[n:], fft.DIF, append(fftOpt[:len(fftOpt):len(fftOpt)], fft.OnCoset())...)
		return
	}

	copy(_p, p)
	clear(_p[n:]) // the slab is caller-provided and may be dirty

	enc.smallDomain.FFTInverseExt6(_p[:n], fft.DIF, fftOpt...)
	gutils.BitReverse(_p[:n])
	enc.Domain.FFTExt6(_p, fft.DIF, fftOpt...)
}

// InverseRate returns the inverse-rate of the code
func (enc *RSEncoder) InverseRate() int {
	return int(enc.Domain.Cardinality) / enc.PlainTextSize
}
