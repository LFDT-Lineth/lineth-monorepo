package fastpolyext

import (
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext/gnarkfext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext/gnarkutilext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Evaluate a polynomial in lagrange basis on a gnark circuit
func InterpolateGnark(api gnarkfext.API, poly []gnarkfext.Variable, x gnarkfext.Variable) gnarkfext.Variable {

	if !utils.IsPowerOfTwo(len(poly)) {
		utils.Panic("only support powers of two but poly has length %v", len(poly))
	}

	// When the poly is of length 1 it means it is a constant polynomial and its
	// evaluation is trivial.
	if len(poly) == 1 {
		return poly[0]
	}

	n := len(poly)
	domain := fft.NewDomain(n)
	one := gnarkfext.One()

	// Test that x is not a root of unity. In the other case, we would
	// have to divide by zero. In practice this constraint is not necessary
	// (because the division constraint would be non-satisfiable anyway)
	// But doing an explicit check clarifies the need.
	xN := gnarkutilext.Exp(api, x, n)
	api.AssertIsDifferent(xN, gnarkfext.One())

	// Compute the term-wise summand of the interpolation formula.
	// This will allow the gnark solver to process the expensive
	// inverses in parallel.
	terms := make([]gnarkfext.Variable, n)
	skip := make([]bool, n)

	// omegaMinI carries the domain's inverse root of unity generator raised to
	// the power I in the following loop. It is initialized with omega**0 = 1.
	// omegaI := frontend.Variable(1)
	omegaI := gnarkfext.One()
	//wrappedGeneratorInv := fext.Element{domain.GeneratorInv, 0}

	for i := 0; i < n; i++ {

		if i > 0 {
			omegaI = api.MulByBase(omegaI, domain.GeneratorInv)
		}

		// If the current term is the constant zero, we continue without generating
		// constraints. As a result, 'terms' may contain nil elements. Therefore,
		// we will need to remove them later
		c1, isC1 := api.Inner.Compiler().ConstantValue(poly[i].A0)
		c2, isC2 := api.Inner.Compiler().ConstantValue(poly[i].A1)
		if isC1 && c1.IsInt64() && c1.Int64() == 0 && isC2 && c2.IsInt64() && c2.Int64() == 0 {
			skip[i] = true
			continue
		}

		xOmegaN := api.Mul(x, omegaI)
		terms[i] = api.Sub(xOmegaN, gnarkfext.One())
		// No point doing a batch inverse in a circuit
		terms[i] = api.Inverse(terms[i])
		terms[i] = api.Mul(terms[i], poly[i])
	}

	nonNilTerms := make([]gnarkfext.Variable, 0, len(terms))
	for i := range terms {
		if skip[i] {
			continue
		}

		nonNilTerms = append(nonNilTerms, terms[i])
	}

	// Then sum all the terms
	var res gnarkfext.Variable

	switch {
	case len(nonNilTerms) == 0:
		res = gnarkfext.NewZero()
	case len(nonNilTerms) == 1:
		res = nonNilTerms[0]
	case len(nonNilTerms) == 2:
		res = api.Add(nonNilTerms[0], nonNilTerms[1])
	default:
		res = api.Add(nonNilTerms[0], nonNilTerms[1], nonNilTerms[2:]...)
	}

	/*
		Then multiply the res by a factor \frac{g^{1 - n}X^n -g}{n}
	*/
	factor := xN
	factor = api.Sub(factor, one)
	factor = api.MulByBase(factor, domain.CardinalityInv)
	res = api.Mul(res, factor)

	return res

}
