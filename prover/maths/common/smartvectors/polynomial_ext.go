package smartvectors

import (
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// AddExt two vectors representing polynomials in coefficient form.
// a and b may have different sizes
func PolyAddExt(a, b SmartVector) SmartVector {

	small, large := a, b
	if a.Len() > b.Len() {
		small, large = large, small
	}

	res := make([]fext.Element, large.Len())
	large.WriteInSliceExt(res)
	for i := 0; i < small.Len(); i++ {
		x := small.GetExt(i)
		res[i].Add(&res[i], &x)
	}

	return NewRegularExt(res)
}

func PolySubExt(a, b SmartVector) SmartVector {

	maxLen := utils.Max(a.Len(), b.Len())
	res := make([]fext.Element, maxLen)
	a.WriteInSliceExt(res[:a.Len()])

	for i := 0; i < b.Len(); i++ {
		bi := b.GetExt(i)
		res[i].Sub(&res[i], &bi)
	}

	return NewRegularExt(res)
}

/*
Ruffini division
  - p polynomial in coefficient form
  - q field.Element, caracterizing the divisor X - q
  - quo quotient polynomial in coefficient form, result will be passed
    here. quo is truncated of its first entry in the process
  - expected to be at least as large as `p`

- rem, remainder also equals to P(r)

Supports &p == quo
*/
func RuffiniQuoRemExt(p SmartVector, q fext.Element) (quo SmartVector, rem fext.Element) {

	// The case where "p" is zero is assumed to be impossible as every type of
	// smart-vector strongly forbid dealing with zero length smart-vectors.
	if p.Len() == 0 {
		panic("Zero-length smart-vectors are forbidden")
	}

	// If p has length 1, then the general case algorithm does not work
	if p.Len() == 1 {
		quo = NewConstantExt(fext.Zero(), 1)
		rem = p.GetExt(0)
		return quo, rem
	}

	quo_ := make([]fext.Element, p.Len())

	// Pass the last coefficient
	quo_[p.Len()-1] = p.GetExt(p.Len() - 1)

	for i := p.Len() - 2; i >= 0; i-- {
		var c fext.Element
		c.Mul(&quo_[i+1], &q)
		pi := p.GetExt(i)
		quo_[i].Add(&c, &pi)
	}

	// As we employ custom allocation, we should not pass x[1:]
	rem = quo_[0]
	quo = NewRegularExt(quo_[1:])

	return quo, rem
}

// Evaluate a polynomial in Lagrange basis
func EvaluateFextPolyLagrange(v SmartVector, x fext.Element, oncoset ...bool) fext.Element {
	if con, ok := v.(*ConstantExt); ok {
		return con.Value
	}

	// Maybe there is an optim for windowed here
	res := make([]fext.Element, v.Len())
	v.WriteInSliceExt(res)

	if len(oncoset) > 0 && oncoset[0] {
		genFr := fft.GeneratorFullMultiplicativeGroup()
		genFr.Inverse(&genFr)
		x.MulByElement(&x, &genFr)
	}

	result, err := vortex.EvalFextPolyLagrange(res, x)
	if err != nil {
		panic(err)
	}

	return result
}

// Batch-evaluate polynomials in Lagrange basis
func BatchEvaluateFextPolyLagrange(vs []SmartVector, x fext.Element, oncoset ...bool) []fext.Element {
	results := make([]fext.Element, len(vs))

	// Pre-allocate with capacity to avoid multiple reallocations
	nonConstantPolys := make([][]fext.Element, 0, len(vs))
	nonConstantIndices := make([]int, 0, len(vs))
	totalConstant := 0

	// Use parallel processing for constant detection
	type workItem struct {
		index      int
		poly       []fext.Element
		isConstant bool
		value      fext.Element
	}

	workItems := make([]workItem, len(vs))

	parallel.Execute(len(vs), func(start, stop int) {
		for i := start; i < stop; i++ {
			if con, ok := vs[i].(*ConstantExt); ok {
				workItems[i] = workItem{i, nil, true, con.Value}
			} else {
				workItems[i] = workItem{i, vs[i].IntoRegVecSaveAllocExt(), false, fext.Element{}}
			}
		}
	})

	// Sequential collection (this part is fast)
	for _, item := range workItems {
		if item.isConstant {
			results[item.index] = item.value
			totalConstant++
		} else {
			nonConstantPolys = append(nonConstantPolys, item.poly)
			nonConstantIndices = append(nonConstantIndices, item.index)
		}
	}

	if totalConstant == len(vs) {
		return results
	}

	if len(nonConstantPolys) > 0 {
		nonConstantResults, err := vortex.BatchEvalFextPolyLagrange(nonConstantPolys, x, oncoset...)
		if err != nil {
			panic(err)
		}
		for j, result := range nonConstantResults {
			results[nonConstantIndices[j]] = result
		}
	}

	return results
}

// Evaluate a polynomial in coefficient basis
func EvalCoeffExt(v SmartVector, x fext.Element) fext.Element {
	// Maybe there is an optim for windowed here
	res := make([]fext.Element, v.Len())
	v.WriteInSliceExt(res)
	return vortex.EvalFextPolyHorner(res, x)
}

func EvalCoeffBivariateExt(v SmartVector, x fext.Element, numCoeffX int, y fext.Element) fext.Element {

	if v.Len()%numCoeffX != 0 {
		panic("size of v and nb coeff x are inconsistent")
	}

	// naive evaluation : we think it is not performance critical
	slice := make([]fext.Element, v.Len())
	v.WriteInSliceExt(slice)

	foldOnX := make([]fext.Element, len(slice)/numCoeffX)
	for i := 0; i < len(slice); i += numCoeffX {
		foldOnX[i/numCoeffX] = vortex.EvalFextPolyHorner(slice[i:i+numCoeffX], x)
	}

	return vortex.EvalFextPolyHorner(foldOnX, y)
}
