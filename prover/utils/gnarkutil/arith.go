package gnarkutil

import (
	"errors"
	"math/big"
	"math/bits"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

func RepeatedVariable(x koalagnark.Element, n int) []koalagnark.Element {
	res := make([]koalagnark.Element, n)
	for i := range res {
		res[i] = x
	}
	return res
}
func RepeatedVariableExt(x koalagnark.Ext, n int) []koalagnark.Ext {
	res := make([]koalagnark.Ext, n)
	for i := range res {
		res[i] = x
	}
	return res
}

// Exp in gnark circuit, using the fast exponentiation
// Optimized for power-of-two exponents (only repeated squaring needed)
func ExpExt(api frontend.API, x koalagnark.Ext, n int) koalagnark.Ext {

	koalaAPI := koalagnark.NewAPI(api)

	if n < 0 {
		x = koalaAPI.InverseExt(x)
		n = -n
	}

	if n == 0 {
		return koalaAPI.OneExt()
	}

	if n == 1 {
		return x
	}

	// Fast path: if n is a power of two, use only repeated squaring
	// This is common in FFT-related computations where n = 2^k
	if bits.OnesCount(uint(n)) == 1 {
		// n = 2^k, so we just need k squarings
		res := x
		for n > 1 {
			res = koalaAPI.SquareExt(res)
			n >>= 1
		}
		return res
	}

	// General case: square-and-multiply
	acc := x
	res := koalaAPI.OneExt()

	// right-to-left
	for n != 0 {
		if n&1 == 1 {
			res = koalaAPI.MulExt(res, acc)
		}
		acc = koalaAPI.SquareExt(acc)
		n >>= 1
	}
	return res
}

// Exp in gnark circuit, using the fast exponentiation
// Optimized for power-of-two exponents (only repeated squaring needed)
func Exp(api frontend.API, x koalagnark.Element, n int) koalagnark.Element {

	koalaAPI := koalagnark.NewAPI(api)

	if n < 0 {
		x = koalaAPI.Inverse(x)
		n = -n
	}

	if n == 0 {
		return koalagnark.NewElement(1)
	}

	if n == 1 {
		return x
	}

	// Fast path: if n is a power of two, use only repeated squaring
	if bits.OnesCount(uint(n)) == 1 {
		res := x
		for n > 1 {
			res = koalaAPI.Mul(res, res)
			n >>= 1
		}
		return res
	}

	// General case: square-and-multiply
	acc := x
	res := koalagnark.NewElement(1)

	// right-to-left
	for n != 0 {
		if n&1 == 1 {
			res = koalaAPI.Mul(res, acc)
		}
		acc = koalaAPI.Mul(acc, acc)
		n >>= 1
	}
	return res
}

// ExpVariableExponent exponentiates x by n in a gnark circuit. Where n is not fixed.
// n is limited to n bits (max)
func ExpVariableExponent(api frontend.API, x koalagnark.Element, exp frontend.Variable, expNumBits int) koalagnark.Element {

	koalaAPI := koalagnark.NewAPI(api)

	expBits := api.ToBinary(exp, expNumBits)
	res := koalagnark.NewElement(1)

	for i := len(expBits) - 1; i >= 0; i-- {
		if i != len(expBits)-1 {
			res = koalaAPI.Mul(res, res)
		}
		tmp := koalaAPI.Mul(res, x)
		res = koalaAPI.Select(expBits[i], tmp, res)
	}

	return res
}

// DivBy31 returns q, r such that v = 31 q + r, and 0 ≤ r < 31
// side effect: ensures 0 ≤ v[i] < 2ᵇⁱᵗˢ⁺².
func DivBy31(api frontend.API, v frontend.Variable, bits int) (q, r frontend.Variable, err error) {
	_q, _r, err := DivManyBy31(api, []frontend.Variable{v}, bits)
	if err != nil {
		return nil, nil, err
	}
	return _q[0], _r[0], nil
}

// DivManyBy31 returns q, r for each v such that v = 31 q + r, and 0 ≤ r < 31
// side effect: ensures 0 ≤ v[i] < 2ᵇⁱᵗˢ⁺² for all i
func DivManyBy31(api frontend.API, v []frontend.Variable, bits int) (q, r []frontend.Variable, err error) {
	qNbBits := bits - 4

	if hintOut, err := api.Compiler().NewHint(divBy31Hint, 2*len(v), v...); err != nil {
		return nil, nil, err
	} else {
		q, r = hintOut[:len(v)], hintOut[len(v):]
	}

	rChecker := rangecheck.New(api)

	for i := range v { // TODO See if lookups or api.AssertIsLte would be more efficient
		rChecker.Check(r[i], 5)
		api.AssertIsDifferent(r[i], 31)
		rChecker.Check(q[i], qNbBits)
		api.AssertIsEqual(v[i], api.Add(api.Mul(q[i], 31), r[i])) // 31 × q < 2ᵇⁱᵗˢ⁻⁴ 2⁵ ⇒ v < 2ᵇⁱᵗˢ⁺¹ + 31 < 2ᵇⁱᵗˢ⁺²
	}
	return q, r, nil
}

// outs: [quotients], [remainders]
func divBy31Hint(_ *big.Int, ins []*big.Int, outs []*big.Int) error {
	if len(outs) != 2*len(ins) {
		return errors.New("expected output layout: [quotients][remainders]")
	}

	q := outs[:len(ins)]
	r := outs[len(ins):]
	for i := range ins {
		v := ins[i].Uint64()
		q[i].SetUint64(v / 31)
		r[i].SetUint64(v % 31)
	}

	return nil
}
