package vortex

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	wfft "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/arena"
)

// rsEncode encodes a vector `v` and returns the corresponding the Reed-Solomon
// codeword. The input vector is interpreted as a polynomial in Lagrange basis
// over a domain of n-roots of unity Omega_n and returns its representation in
// the Lagrange basis Omega_{n * blow-up} where blow-up corresponds to the
// inverse-rate of the code. The code is systematic as the original vector is
// interleaved within the encoded vector.
func (p *Params) rsEncode(v smartvectors.SmartVector, vArena ...*arena.VectorArena) smartvectors.SmartVector {

	// Short path, v is a constant vector. It's encoding is also a constant vector
	// with the same value.
	if cons, ok := v.(*smartvectors.Constant); ok {
		return smartvectors.NewConstant(cons.Val(), p.NumEncodedCols())
	}
	var expandedCoeffs []field.Element
	if len(vArena) > 0 {
		expandedCoeffs = arena.Get[field.Element](vArena[0], p.NumEncodedCols())
	} else {
		expandedCoeffs = make([]field.Element, p.NumEncodedCols())
	}

	// copy the input
	v.WriteInSlice(expandedCoeffs[:v.Len()])

	const rho = 2
	if rho != p.BlowUpFactor {
		smallDomain := p.Domains[0]
		largeDomain := p.Domains[1]

		smallDomain.FFTInverse(expandedCoeffs[:v.Len()], fft.DIF, fft.WithNbTasks(1))
		fft.BitReverse(expandedCoeffs[:v.Len()])

		largeDomain.FFT(expandedCoeffs, fft.DIF, fft.WithNbTasks(1))
		fft.BitReverse(expandedCoeffs)

		return smartvectors.NewRegular(expandedCoeffs)
	}

	// fast path; we avoid the bit reverse operations and work on the smaller domain.
	inputCoeffs := field.Vector(expandedCoeffs[:p.NbColumns])

	p.Domains[0].FFTInverse(inputCoeffs, fft.DIF, fft.WithNbTasks(1))
	inputCoeffs.Mul(inputCoeffs, p.CosetTableBitReverse)

	p.Domains[0].FFT(inputCoeffs, fft.DIT, fft.WithNbTasks(1))
	for j := p.NbColumns - 1; j >= 0; j-- {
		expandedCoeffs[rho*j+1] = expandedCoeffs[j]
		expandedCoeffs[rho*j] = v.Get(j)
	}

	return smartvectors.NewRegular(expandedCoeffs)
}

// IsCodeword returns nil iff the argument `v` is a correct codeword and an
// error is returned otherwise.
func (p *Params) isCodeword(v smartvectors.SmartVector) error {
	coeffs := smartvectors.FFTInverse(v, wfft.DIT, true, 0, 0, nil)
	for i := p.NbColumns; i < p.NumEncodedCols(); i++ {
		c := coeffs.Get(i)
		if !c.IsZero() {
			return fmt.Errorf("not a reed-solomon codeword")
		}
	}

	return nil
}
