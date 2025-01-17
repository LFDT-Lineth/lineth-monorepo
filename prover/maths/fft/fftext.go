package fft

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"math/bits"

	"github.com/consensys/linea-monorepo/prover/maths/field"
)

func (domain *Domain) FFTExt(a []fext.Element, decimation Decimation, opts ...Option) {

	opt := fftOptions(opts...)

	// find the stage where we should stop spawning go routines in our recursive calls
	// (ie when we have as many go routines running as we have available CPUs)
	maxSplits := bits.TrailingZeros64(ecc.NextPowerOfTwo(uint64(opt.nbTasks)))
	if opt.nbTasks == 1 {
		maxSplits = -1
	}

	// if coset != 0, scale by coset table
	if opt.coset {
		scale := func(cosetTable []field.Element) {
			for i := 0; i < len(a); i++ {
				// scale the first coordinate
				a[i].A0.Mul(&a[i].A0, &cosetTable[i])
				// scale the second coordinate
				a[i].A1.Mul(&a[i].A1, &cosetTable[i])
			}
		}
		if decimation == DIT {
			scale(domain.CosetTableReversed)

		} else {
			scale(domain.CosetTable)
		}
	}

	switch decimation {
	case DIF:
		difFFTExt(a, domain.Twiddles, 0, maxSplits, nil, opt.nbTasks)
	case DIT:
		ditFFTExt(a, domain.Twiddles, 0, maxSplits, nil, opt.nbTasks)
	default:
		panic("not implemented")
	}
}

// FFTInverseExt computes (recursively) the inverse discrete Fourier transform of a and stores the result in a
// if decimation == DIT (decimation in time), the input must be in bit-reversed order
// if decimation == DIF (decimation in frequency), the output will be in bit-reversed order
// coset sets the shift of the fft (0 = no shift, standard fft)
// len(a) must be a power of 2, and w must be a len(a)th root of unity in field F.
func (domain *Domain) FFTInverseExt(a []fext.Element, decimation Decimation, opts ...Option) {

	opt := fftOptions(opts...)

	// find the stage where we should stop spawning go routines in our recursive calls
	// (ie when we have as many go routines running as we have available CPUs)
	maxSplits := bits.TrailingZeros64(ecc.NextPowerOfTwo(uint64(opt.nbTasks)))
	if opt.nbTasks == 1 {
		maxSplits = -1
	}

	switch decimation {
	case DIF:
		difFFTExt(a, domain.TwiddlesInv, 0, maxSplits, nil, opt.nbTasks)
	case DIT:
		ditFFTExt(a, domain.TwiddlesInv, 0, maxSplits, nil, opt.nbTasks)
	default:
		panic("not implemented")
	}

	// scale by CardinalityInv
	if !opt.coset {
		for i := 0; i < len(a); i++ {
			// process first coordinate
			a[i].A0.Mul(&a[i].A0, &domain.CardinalityInv)
			// process second coordinate
			a[i].A1.Mul(&a[i].A1, &domain.CardinalityInv)
		}
		return
	}

	scale := func(cosetTable []field.Element) {
		for i := 0; i < len(a); i++ {
			// process first coordinate
			a[i].A0.Mul(&a[i].A0, &cosetTable[i]).
				Mul(&a[i].A0, &domain.CardinalityInv)
			// process second coordinate
			a[i].A1.Mul(&a[i].A1, &cosetTable[i]).
				Mul(&a[i].A1, &domain.CardinalityInv)
		}
	}
	if decimation == DIT {
		scale(domain.CosetTableInv)
		return
	}

	// decimation == DIF
	scale(domain.CosetTableInvReversed)

}

func difFFTExt(a []fext.Element, twiddles [][]field.Element, stage int, maxSplits int, chDone chan struct{}, nbTasks int) {
	if chDone != nil {
		defer close(chDone)
	}

	n := len(a)
	if n == 1 {
		return
	} else if n == 256 {
		kerDIFNP_256Ext(a, twiddles, stage)
		return
	}

	m := n >> 1

	parallelButterfly := (m > butterflyThreshold) && (stage < maxSplits)

	// i == 0
	if parallelButterfly {
		parallel.Execute(m, func(start, end int) {
			innerDIFWithTwiddlesExt(a, twiddles[stage], start, end, m)
		}, nbTasks/(1<<(stage)))
	} else {
		innerDIFWithTwiddlesExt(a, twiddles[stage], 0, m, m)
	}

	if m == 1 {
		return
	}

	nextStage := stage + 1
	if stage < maxSplits {
		chDone := make(chan struct{}, 1)
		go difFFTExt(a[m:n], twiddles, nextStage, maxSplits, chDone, nbTasks)
		difFFTExt(a[0:m], twiddles, nextStage, maxSplits, nil, nbTasks)
		<-chDone
	} else {
		difFFTExt(a[0:m], twiddles, nextStage, maxSplits, nil, nbTasks)
		difFFTExt(a[m:n], twiddles, nextStage, maxSplits, nil, nbTasks)
	}
}

func ditFFTExt(a []fext.Element, twiddles [][]field.Element, stage int, maxSplits int, chDone chan struct{}, nbTasks int) {
	if chDone != nil {
		defer close(chDone)
	}

	n := len(a)
	if n == 1 {
		return
	} else if n == 256 {
		kerDITNP_256Ext(a, twiddles, stage)
		return
	}

	m := n >> 1

	parallelButterfly := (m > butterflyThreshold) && (stage < maxSplits)

	nextStage := stage + 1

	if stage < maxSplits {
		// that's the only time we fire go routines
		chDone := make(chan struct{}, 1)
		go ditFFTExt(a[m:n], twiddles, nextStage, maxSplits, chDone, nbTasks)
		ditFFTExt(a[0:m], twiddles, nextStage, maxSplits, nil, nbTasks)
		<-chDone
	} else {
		ditFFTExt(a[0:m], twiddles, nextStage, maxSplits, nil, nbTasks)
		ditFFTExt(a[m:n], twiddles, nextStage, maxSplits, nil, nbTasks)
	}

	if parallelButterfly {
		parallel.Execute(m, func(start, end int) {
			innerDITWithTwiddlesExt(a, twiddles[stage], start, end, m)
		}, nbTasks/(1<<(stage)))
	} else {
		innerDITWithTwiddlesExt(a, twiddles[stage], 0, m, m)
	}
}

func innerDIFWithTwiddlesExt(a []fext.Element, twiddles []field.Element, start, end, m int) {
	if start == 0 {
		fext.Butterfly(&a[0], &a[m])
		start++
	}
	for i := start; i < end; i++ {
		fext.Butterfly(&a[i], &a[i+m])
		// process the first coordinate
		a[i+m].A0.Mul(&a[i+m].A0, &twiddles[i])
		// process the second coordinate
		a[i+m].A1.Mul(&a[i+m].A1, &twiddles[i])
	}
}

func innerDITWithTwiddlesExt(a []fext.Element, twiddles []field.Element, start, end, m int) {
	if start == 0 {
		fext.Butterfly(&a[0], &a[m])
		start++
	}
	for i := start; i < end; i++ {
		// process the first coordinate
		a[i+m].A0.Mul(&a[i+m].A0, &twiddles[i])
		// process the second coordinate
		a[i+m].A1.Mul(&a[i+m].A1, &twiddles[i])
		fext.Butterfly(&a[i], &a[i+m])
	}
}

func kerDIFNP_256Ext(a []fext.Element, twiddles [][]field.Element, stage int) {
	// code unrolled & generated by internal/generator/fft/template/fft.go.tmpl

	innerDIFWithTwiddlesExt(a[:256], twiddles[stage+0], 0, 128, 128)
	for offset := 0; offset < 256; offset += 128 {
		innerDIFWithTwiddlesExt(a[offset:offset+128], twiddles[stage+1], 0, 64, 64)
	}
	for offset := 0; offset < 256; offset += 64 {
		innerDIFWithTwiddlesExt(a[offset:offset+64], twiddles[stage+2], 0, 32, 32)
	}
	for offset := 0; offset < 256; offset += 32 {
		innerDIFWithTwiddlesExt(a[offset:offset+32], twiddles[stage+3], 0, 16, 16)
	}
	for offset := 0; offset < 256; offset += 16 {
		innerDIFWithTwiddlesExt(a[offset:offset+16], twiddles[stage+4], 0, 8, 8)
	}
	for offset := 0; offset < 256; offset += 8 {
		innerDIFWithTwiddlesExt(a[offset:offset+8], twiddles[stage+5], 0, 4, 4)
	}
	for offset := 0; offset < 256; offset += 4 {
		innerDIFWithTwiddlesExt(a[offset:offset+4], twiddles[stage+6], 0, 2, 2)
	}
	for offset := 0; offset < 256; offset += 2 {
		fext.Butterfly(&a[offset], &a[offset+1])
	}
}

func kerDITNP_256Ext(a []fext.Element, twiddles [][]field.Element, stage int) {
	// code unrolled & generated by internal/generator/fft/template/fft.go.tmpl

	for offset := 0; offset < 256; offset += 2 {
		fext.Butterfly(&a[offset], &a[offset+1])
	}
	for offset := 0; offset < 256; offset += 4 {
		innerDITWithTwiddlesExt(a[offset:offset+4], twiddles[stage+6], 0, 2, 2)
	}
	for offset := 0; offset < 256; offset += 8 {
		innerDITWithTwiddlesExt(a[offset:offset+8], twiddles[stage+5], 0, 4, 4)
	}
	for offset := 0; offset < 256; offset += 16 {
		innerDITWithTwiddlesExt(a[offset:offset+16], twiddles[stage+4], 0, 8, 8)
	}
	for offset := 0; offset < 256; offset += 32 {
		innerDITWithTwiddlesExt(a[offset:offset+32], twiddles[stage+3], 0, 16, 16)
	}
	for offset := 0; offset < 256; offset += 64 {
		innerDITWithTwiddlesExt(a[offset:offset+64], twiddles[stage+2], 0, 32, 32)
	}
	for offset := 0; offset < 256; offset += 128 {
		innerDITWithTwiddlesExt(a[offset:offset+128], twiddles[stage+1], 0, 64, 64)
	}
	innerDITWithTwiddlesExt(a[:256], twiddles[stage+0], 0, 128, 128)
}
