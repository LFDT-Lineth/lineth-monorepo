package symbolic

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

// simplifyLinComb simplifies a linear combination by expanding nested LinCombs,
// grouping terms with the same hash, and accumulating constants.
func simplifyLinComb(items []*Expression, coeffs []int) ([]*Expression, []int) {
	if len(items) != len(coeffs) {
		panic("unmatching lengths")
	}

	// Accumulator for the constant term
	constantAcc := fext.GenericFieldZero()

	// We use a map for larger sets of items to ensure O(N) complexity
	// For small sets, we use a slice and linear scan
	const linearThreshold = 16

	var (
		newItems  []*Expression
		newCoeffs []int
	)

	// Map for fast lookup when we have many items
	var itemMap map[esHash]int // maps ESHash to index in newItems

	// Helper to add a term
	addTerm := func(item *Expression, coeff int) {
		if coeff == 0 {
			return
		}

		// Handle Constant
		if c, ok := item.Operator.(Constant); ok {
			var t fext.GenericFieldElem
			t.SetInt64(int64(coeff))
			t.Mul(&c.Val)
			constantAcc.Add(&t)
			return
		}
		if c, ok := item.Operator.(*Constant); ok {
			var t fext.GenericFieldElem
			t.SetInt64(int64(coeff))
			t.Mul(&c.Val)
			constantAcc.Add(&t)
			return
		}

		// Handle Variable/Other
		// Check if we should switch to map
		if itemMap == nil && len(newItems) >= linearThreshold {
			itemMap = make(map[esHash]int, len(newItems))
			for i, it := range newItems {
				itemMap[it.ESHash] = i
			}
		}

		if itemMap != nil {
			if idx, found := itemMap[item.ESHash]; found {
				newCoeffs[idx] += coeff
			} else {
				idx = len(newItems)
				newItems = append(newItems, item)
				newCoeffs = append(newCoeffs, coeff)
				itemMap[item.ESHash] = idx
			}
		} else {
			// Linear scan
			found := false
			for i := range newItems {
				if newItems[i].ESHash == item.ESHash {
					newCoeffs[i] += coeff
					found = true
					break
				}
			}
			if !found {
				newItems = append(newItems, item)
				newCoeffs = append(newCoeffs, coeff)
			}
		}
	}

	// Process items
	for i, item := range items {
		coeff := coeffs[i]
		if coeff == 0 {
			continue
		}

		var (
			cLinC   *LinComb
			cIsLinC bool
		)

		switch o := item.Operator.(type) {
		case LinComb:
			cIsLinC = true
			cLinC = &o
		case *LinComb:
			cIsLinC = true
			cLinC = o
		}

		if cIsLinC {
			for k, grandChild := range item.Children {
				addTerm(grandChild, coeff*cLinC.Coeffs[k])
			}
		} else {
			addTerm(item, coeff)
		}
	}

	// Add constant term if non-zero
	if !constantAcc.IsZero() {
		cExpr := NewConstant(constantAcc)
		newItems = append(newItems, cExpr)
		newCoeffs = append(newCoeffs, 1)
	}

	// Filter out zero coefficients
	n := 0
	for i, c := range newCoeffs {
		if c != 0 {
			newItems[n] = newItems[i]
			newCoeffs[n] = c
			n++
		}
	}
	newItems = newItems[:n]
	newCoeffs = newCoeffs[:n]

	return newItems, newCoeffs
}

// simplifyProduct simplifies a product by expanding nested Products,
// grouping terms with the same hash, and accumulating constants.
func simplifyProduct(items []*Expression, exponents []int) ([]*Expression, []int) {
	if len(items) != len(exponents) {
		panic("unmatching lengths")
	}

	// Accumulator for the constant term
	constantAcc := fext.GenericFieldOne()

	const linearThreshold = 16

	var (
		newItems     []*Expression
		newExponents []int
	)

	var itemMap map[esHash]int

	addTerm := func(item *Expression, exponent int) {
		if exponent == 0 {
			return
		}

		// Handle Constant
		if c, ok := item.Operator.(Constant); ok {
			var t fext.GenericFieldElem
			t.Exp(&c.Val, big.NewInt(int64(exponent)))
			constantAcc.Mul(&t)
			return
		}
		if c, ok := item.Operator.(*Constant); ok {
			var t fext.GenericFieldElem
			t.Exp(&c.Val, big.NewInt(int64(exponent)))
			constantAcc.Mul(&t)
			return
		}

		if itemMap == nil && len(newItems) >= linearThreshold {
			itemMap = make(map[esHash]int, len(newItems))
			for i, it := range newItems {
				itemMap[it.ESHash] = i
			}
		}

		if itemMap != nil {
			if idx, found := itemMap[item.ESHash]; found {
				newExponents[idx] += exponent
			} else {
				idx = len(newItems)
				newItems = append(newItems, item)
				newExponents = append(newExponents, exponent)
				itemMap[item.ESHash] = idx
			}
		} else {
			found := false
			for i := range newItems {
				if newItems[i].ESHash == item.ESHash {
					newExponents[i] += exponent
					found = true
					break
				}
			}
			if !found {
				newItems = append(newItems, item)
				newExponents = append(newExponents, exponent)
			}
		}
	}

	for i, item := range items {
		exponent := exponents[i]
		if exponent == 0 {
			continue
		}

		if exponent < 0 {
			panic("negative exponents are not allowed")
		}

		if item.ESHash.IsZero() {
			return []*Expression{NewConstant(0)}, []int{1}
		}

		var (
			cProd   *Product
			cIsProd bool
		)

		switch o := item.Operator.(type) {
		case Product:
			cIsProd = true
			cProd = &o
		case *Product:
			cIsProd = true
			cProd = o
		}

		if cIsProd {
			for k, grandChild := range item.Children {
				addTerm(grandChild, exponent*cProd.Exponents[k])
			}
		} else {
			addTerm(item, exponent)
		}
	}

	if !constantAcc.IsOne() {
		cExpr := NewConstant(constantAcc)
		newItems = append(newItems, cExpr)
		newExponents = append(newExponents, 1)
	}

	n := 0
	for i, e := range newExponents {
		if e != 0 {
			newItems[n] = newItems[i]
			newExponents[n] = e
			n++
		}
	}
	newItems = newItems[:n]
	newExponents = newExponents[:n]

	return newItems, newExponents
}
