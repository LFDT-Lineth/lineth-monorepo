package main

import (
	"fmt"
	"strings"

	"github.com/consensys/linea-monorepo/prover/utils"
)

func partialFFT(domainSize, numField, mask int64) string {

	gen := initializePartialFFTCodeGen(domainSize, numField, mask)

	gen.header()
	gen.indent()

	var (
		numStages int = utils.Log2Ceil(int(domainSize))
		numSplits int = 1
		splitSize int = int(domainSize)
	)

	for level := 0; level < numStages; level++ {
		for s := 0; s < numSplits; s++ {
			for k := 0; k < splitSize/2; k++ {
				gen.twiddleMulLine(s*splitSize+splitSize/2+k, numSplits-1+s)
			}
		}

		for s := 0; s < numSplits; s++ {
			for k := 0; k < splitSize/2; k++ {
				gen.butterFlyLine(s*splitSize+k, s*splitSize+splitSize/2+k)
			}
		}

		splitSize /= 2
		numSplits *= 2
	}

	gen.desindent()
	gen.tail()
	return gen.Builder.String()
}

func initializePartialFFTCodeGen(domainSize, numField, mask int64) PartialFFTCodeGen {
	res := PartialFFTCodeGen{
		DomainSize: int(domainSize),
		NumField:   int(numField),
		Mask:       int(mask),
		IsZero:     make([]bool, domainSize),
		Builder:    &strings.Builder{},
		NumIndent:  0,
	}

	for i := range res.IsZero {
		var (
			fieldSize = domainSize / numField
			bit       = i / int(fieldSize)
			isZero    = ((mask >> bit) & 1) == 0
		)

		res.IsZero[i] = isZero
	}

	return res
}

type PartialFFTCodeGen struct {
	DomainSize int
	NumField   int
	Mask       int
	Builder    *strings.Builder
	NumIndent  int
	IsZero     []bool
}

func (p *PartialFFTCodeGen) header() {
	writeIndent(p.Builder, p.NumIndent)
	line := fmt.Sprintf("func partialFFT_%v(a, twiddles []field.Element) {\n", p.Mask)
	p.Builder.WriteString(line)
}

func (p *PartialFFTCodeGen) tail() {
	writeIndent(p.Builder, p.NumIndent)
	p.Builder.WriteString("}\n")
}

func (p *PartialFFTCodeGen) butterFlyLine(i, j int) {
	allZeroes := p.IsZero[i] && p.IsZero[j]
	if allZeroes {
		return
	}

	p.IsZero[i] = false
	p.IsZero[j] = false

	writeIndent(p.Builder, p.NumIndent)

	line := fmt.Sprintf("field.Butterfly(&a[%v], &a[%v])\n", i, j)
	if _, err := p.Builder.WriteString(line); err != nil {
		panic(err)
	}
}

func (p *PartialFFTCodeGen) twiddleMulLine(i, twidPos int) {
	if p.IsZero[i] {
		return
	}

	writeIndent(p.Builder, p.NumIndent)

	line := fmt.Sprintf("a[%v].Mul(&a[%v], &twiddles[%v])\n", i, i, twidPos)
	if _, err := p.Builder.WriteString(line); err != nil {
		panic(err)
	}
}

func (p *PartialFFTCodeGen) desindent() {
	p.NumIndent--
}

func (p *PartialFFTCodeGen) indent() {
	p.NumIndent++
}

func writeIndent(w *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		w.WriteString("\t")
	}
}
