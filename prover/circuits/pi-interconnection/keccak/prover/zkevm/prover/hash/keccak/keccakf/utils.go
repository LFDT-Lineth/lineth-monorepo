package keccakf

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Converts a U64 to a given base, the base should be given in field element
// form to save on expensive conversion.
func U64ToBaseX(x uint64, base *field.Element) field.Element {
	res := field.Zero()
	one := field.One()
	resIsZero := true

	for k := 64; k >= 0; k-- {
		// The test allows skipping useless field muls or testing
		// the entire field element.
		if !resIsZero {
			res.Mul(&res, base)
		}

		// Skips the field addition if the bit is zero
		bit := (x >> k) & 1
		if bit > 0 {
			res.Add(&res, &one)
			resIsZero = false
		}
	}

	return res
}

// BaseRecompose converts an array of field element representing small limbs
// in a given base and recomposes the full element represented.
func BaseRecompose(r []field.Element, base *field.Element) (res field.Element) {
	// .. using the Horner method
	s := field.Zero()
	for i := len(r) - 1; i >= 0; i-- {
		s.Mul(&s, base)
		s.Add(&s, &r[i])
	}
	return s
}

// It composes chunks and returns slices
func DecomposeSmall(r uint64, base int, nb int) (res []field.Element) {
	// It will essentially be used for chunk to slice decomposition
	res = make([]field.Element, 0, nb)
	base64 := uint64(base)
	curr := r
	for curr > 0 {
		limb := field.NewElement(curr % base64)
		res = append(res, limb)
		curr /= base64
	}

	if len(res) > nb {
		utils.Panic("expected %v limbs, but got %v", nb, len(res))
	}

	// Complete with zeroes
	for len(res) < nb {
		res = append(res, field.Zero())
	}

	return res
}

// Decompose a field element in slices
func DecomposeFrInSlice(f []field.Element, base int) (res [][]field.Element) {
	base *= base // pow 2
	base *= base // pow 4

	// Preallocate the result. The +1 is to isolate the MSB in case it is needed
	// when f is using 65 instead of 64 limbs.
	res = make([][]field.Element, numSlice+1)
	for k := range res {
		res[k] = make([]field.Element, len(f))
	}

	for r := 0; r < len(f); r++ {
		// Assumedly len(limbs) <= len(res). It will panic if this is not the
		// case. This would indicate that `f` uses more than 68 bits which is
		// not expected.
		limbs := DecomposeFr(f[r], base, numSlice+1)
		for k := range limbs {
			res[k][r] = limbs[k]
		}
	}

	return res
}

// Decompose a field into a given base. nb gives the number of chunks needed
func DecomposeFr(f field.Element, base int, nb int) (res []field.Element) {

	// Optimization : the computation is faster to perform if
	// f fits on a U64
	if f.IsUint64() {
		return DecomposeSmall(f.Uint64(), base, nb)
	}

	// Converts f to bigint
	var curr big.Int
	f.BigInt(&curr)

	// Also the base
	baseBig := big.NewInt(int64(base))
	zero := big.NewInt(0)
	var limbBig big.Int
	var limbF field.Element

	// Initialize the result
	res = make([]field.Element, 0, nb)

	for curr.Cmp(zero) > 0 {
		curr.DivMod(&curr, baseBig, &limbBig)
		limbF.SetBigInt(&limbBig)
		res = append(res, limbF)
	}

	if len(res) > nb {
		utils.Panic("expected %v limbs, but got %v", nb, len(res))
	}

	// Complete with zeroes
	for len(res) < nb {
		res = append(res, field.Zero())
	}

	return res
}

// Converts from (possibly dirty) base representation to a U64. Used for
// internal testing only.
func BaseXToU64(x field.Element, base *field.Element, optBitP0s ...int) (res uint64) {
	res = 0
	decomposedF := DecomposeFr(x, field.ToInt(base), 64)

	bitPos := 0
	if len(optBitP0s) > 0 {
		bitPos = optBitP0s[0]
	}

	for i, limb := range decomposedF {
		bit := (limb.Uint64() >> bitPos) & 1
		res |= bit << i
	}

	return res
}

// BaseRecomposeSliceExpr (de-)composes the slices in the given base and returns
// the corresponding expression
func BaseRecomposeSliceExpr(a []*symbolic.Expression, base int) *symbolic.Expression {

	// length assertion, there is should be 16 elements in the slice. We do not
	// enforces that at the type level because otherwise it requires the user
	// code to cast everything into a sized array which is annoying.
	if len(a) != numSlice {
		utils.Panic("expected length to be %v, got %v", numSlice, len(a))
	}

	x := symbolic.NewConstant(IntExp(uint64(base), numChunkBaseX))
	res := symbolic.NewConstant(0)
	for k := len(a) - 1; k >= 0; k-- {
		res = res.Mul(x).Add(a[k])
	}

	return res
}

// BaseRecomposeSliceExpr (de-)composes the slices in the given base and returns
// the corresponding expression. If prev is set to true, then we use shifted by
// -1 columns of a instead of directly the column of a.
func BaseRecomposeSliceHandles(a []ifaces.Column, base int, prev ...bool) *symbolic.Expression {

	// Deep copies a to avoid side-effects
	a_ := make([]ifaces.Column, len(a))
	copy(a_, a)

	// Optionally shift the columns
	if len(prev) > 0 && prev[0] {
		for i := range a_ {
			a_[i] = column.Shift(a_[i], -1)
		}
	}

	// Converts a into a sequence of variables
	vars := make([]*symbolic.Expression, 0, len(a_))
	for i := range a_ {
		vars = append(vars, ifaces.ColumnAsVariable(a_[i]))
	}

	// Delegates the call
	return BaseRecomposeSliceExpr(vars, base)
}

// BaseRecomposeSliceHandles (de-)composes the slices in the given base and
// returns the corresponding expression
func BaseRecomposeHandles(a []ifaces.Column, base int) *symbolic.Expression {
	x := symbolic.NewConstant(base)
	res := symbolic.NewConstant(0)
	for k := len(a) - 1; k >= 0; k-- {
		res = res.Mul(x).Add(ifaces.ColumnAsVariable(a[k]))
	}
	return res
}

// RotateSlice returns a slice containing the entries of arr rotated by n on the
// right.
func RotateRight[T any](t []T, n int) []T {
	res := make([]T, len(t))
	for i := range t {
		res[(i+n)%len(t)] = t[i]
	}
	return res
}

// Integer exponentiation, the recursive version
func IntExp(base uint64, exponent int) uint64 {

	if exponent == 0 {
		return 1
	}

	if exponent == 1 {
		return base
	}

	if exponent%2 == 0 {
		return IntExp(base*base, exponent/2)
	}

	// else exponent % 2 == 1 and exponent > 2
	return base * IntExp(base*base, (exponent-1)/2)
}

// Returns the number of rows required to prove `numKeccakf` calls to the
// permutation function. The result is padded to the next power of 2 in order to
// satisfy the requirements of the Wizard to have only powers of 2.
func numRows(numKeccakf int) int {
	return utils.NextPowerOfTwo(numKeccakf * numRounds)
}

// derive column names
func deriveName(mainName string, ids ...int) ifaces.ColID {
	idStr := []string{}
	for i := range ids {
		idStr = append(idStr, strconv.Itoa(ids[i]))
	}
	return ifaces.ColIDf("%v_%v_%v", "KECCAKF", mainName, strings.Join(idStr, "_"))
}
