// Package nonnative compiles every unreduced [wiop.NonNative] query into
// vanishing polynomial identities, range checks and limb recompositions.
//
// For each query, every operand limb is first split into digits of s bits,
// where s divides the query's NbBitsPerLimb (see [layout]); when s equals
// NbBitsPerLimb the limbs are their own digits. Digit j of an operand is the
// coefficient of X^j of a polynomial, and the compiler asserts, coefficient by
// coefficient, the polynomial identity
//
//	left(X)*right(X) - modulus(X)*quotient(X) - result(X) - carry(X)*(2^s - X) = 0
//
// where carry is a per-query column sequence computed by the compiler from the
// base-2^s schoolbook multiplication carries. Coefficient k reads
//
//	lr_k - mq_k - res_k - (2^s*c_k - c_{k-1}) = 0
//
// with lr_k and mq_k the k-th coefficients of left*right and modulus*quotient.
//
// # Soundness of the multiplication
//
// Each coefficient identity holds modulo the KoalaBear modulus p. The integer
// relation Left*Right = Quotient*Modulus + Result follows if no coefficient
// can reach a non-zero multiple of p: every coefficient is then zero over the
// integers, and evaluating the identity at X = 2^s yields the relation. The
// compiler enforces it by
//
//   - range-checking every digit into [0, 2^s) and, when limbs are split,
//     constraining each limb to recompose from its digits (which also bounds
//     every limb to [0, 2^NbBitsPerLimb));
//   - range-checking every carry into a signed window around zero, committed
//     with a public bias;
//   - choosing s as the widest divisor of NbBitsPerLimb for which the
//     worst-case coefficient over all range-checked assignments stays below p.
//
// Bounding the carries alone is not enough: products of two 16-bit limbs
// already exceed p ~ 2^31, and with 16-bit digits a prover can claim
// Result + p (p has small 16-bit limbs and then only shifts a small carry).
// 16-bit limbs are therefore checked with 8-bit digits.
//
// The resulting [wiop.Vanishing] and [wiop.RangeCheck] queries are left for the
// downstream passes to reduce; this package does not itself assert them
// against the transcript beyond registering them.
package nonnative

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Operand indices, in the order returned by [operands].
const (
	opLeft = iota
	opRight
	opModulus
	opResult
	opQuotient
	nbOperands
)

var operandNames = [nbOperands]string{"left", "right", "modulus", "result", "quotient"}

// operands returns the limb columns of q indexed by the op* constants.
func operands(q *wiop.NonNative) [nbOperands][]*wiop.Column {
	return [nbOperands][]*wiop.Column{q.Left, q.Right, q.Modulus, q.Result, q.Quotient}
}

// Compile reduces every unreduced [wiop.NonNative] query in sys to
// [wiop.Vanishing] identities, plus the range checks and limb recompositions
// that make them sound.
//
// It is a no-op if sys contains no eligible queries.
func Compile(sys *wiop.System) {
	var queries []*wiop.NonNative
	for _, m := range sys.Modules {
		for _, g := range m.NonNatives {
			if g.IsReduced() {
				continue
			}
			queries = append(queries, g)
		}
	}

	for i, q := range queries {
		c := newCompiledQuery(q, q.Context().Childf("q%d", i))
		q.InputRound.RegisterAction(&proverAction{c: c})
		q.MarkAsReduced()
	}
}

// layout is the digit and carry representation used to check one query.
type layout struct {
	// digitBits is the digit width s; it divides the query's NbBitsPerLimb.
	digitBits int
	// digitsPerLimb is NbBitsPerLimb / digitBits.
	digitsPerLimb int
	// nbDigits is the number of digits of each operand.
	nbDigits int
	// carryBits is the width of the committed carries: a carry c is committed
	// as c + 2^(carryBits-1) and range-checked into [0, 2^carryBits).
	carryBits int
}

// newLayout returns the widest-digit layout for q whose identity is sound
// (see [layout.maxCoefficient]).
//
// Panics if no divisor of NbBitsPerLimb yields a sound layout, which requires
// operands of roughly 2^29 digits.
func newLayout(q *wiop.NonNative) layout {
	b := q.NbBitsPerLimb
	for k := 1; k <= b; k++ {
		if b%k != 0 {
			continue
		}
		l := layout{
			digitBits:     b / k,
			digitsPerLimb: k,
			nbDigits:      len(q.Left) * k,
		}
		// Honest carries lie in [-(n*P+1), n*P] (see [carryRow]), which fits
		// [-2^h, 2^h) for the smallest h with 2^h >= n*P+1.
		maxHonestCarry := uint64(l.nbDigits)*(uint64(1)<<l.digitBits-1) + 1
		l.carryBits = bits.Len64(maxHonestCarry-1) + 1
		if l.maxCoefficient().Cmp(field.Modulus()) < 0 {
			return l
		}
	}
	panic(fmt.Sprintf(
		"nonnative: no sound digit layout for %d limbs of %d bits (query %s)",
		len(q.Left), b, q.Context().Path(),
	))
}

// carryBias is the public offset added to every committed carry.
func (l layout) carryBias() uint64 { return uint64(1) << (l.carryBits - 1) }

// maxCoefficient bounds the absolute value of every integer coefficient of
//
//	left(X)*right(X) - modulus(X)*quotient(X) - result(X) - carry(X)*(2^s - X)
//
// over all assignments passing the range checks: digits in [0, P] with
// P = 2^s - 1 and carries in [-beta, beta-1] with beta = 2^(carryBits-1). With
// n digits per operand, a coefficient sums at most n digit products. It is
// largest either with every product of left*right at P^2, carry c_k = -beta
// and c_{k-1} = beta-1, or with every product of modulus*quotient at P^2, the
// result digit at P, c_k = beta-1 and c_{k-1} = -beta.
//
// The identity is sound when this bound is below the field modulus p: a
// coefficient that vanishes modulo p is then zero over the integers, so the
// difference polynomial is zero and evaluating it at 2^s yields the integer
// relation. The same bound holds for every coefficient identity on its own.
func (l layout) maxCoefficient() *big.Int {
	var (
		n     = new(big.Int).SetInt64(int64(l.nbDigits))
		radix = new(big.Int).Lsh(big.NewInt(1), uint(l.digitBits))
		p     = new(big.Int).Sub(radix, big.NewInt(1))
		beta  = new(big.Int).SetUint64(l.carryBias())
		betaM = new(big.Int).Sub(beta, big.NewInt(1))
	)
	products := new(big.Int).Mul(n, new(big.Int).Mul(p, p))

	// products + radix*beta + (beta-1)
	positive := new(big.Int).Mul(radix, beta)
	positive.Add(positive, products).Add(positive, betaM)

	// products + P + radix*(beta-1) + beta
	negative := new(big.Int).Mul(radix, betaM)
	negative.Add(negative, products).Add(negative, p).Add(negative, beta)

	if positive.Cmp(negative) > 0 {
		return positive
	}
	return negative
}

// compiledQuery holds the columns the compiler allocates for one query.
type compiledQuery struct {
	query *wiop.NonNative
	layout
	// digits[op] holds the little-endian base-2^digitBits digits of operand op.
	// Limb i is recomposed from digits[op][i*digitsPerLimb : (i+1)*digitsPerLimb].
	// When digitsPerLimb is 1, digits[op] are the operand's limb columns.
	digits [nbOperands][]*wiop.Column
	// carry holds the biased carries, one per coefficient of the product
	// polynomial except the leading one (whose carry is zero).
	carry []*wiop.Column
}

// newCompiledQuery allocates the digit and carry columns of q and registers
// their range checks and constraints.
func newCompiledQuery(q *wiop.NonNative, ctx *wiop.ContextFrame) *compiledQuery {
	m := q.Module
	c := &compiledQuery{query: q, layout: newLayout(q)}

	limbs := operands(q)
	for op := range nbOperands {
		c.digits[op] = c.newDigits(ctx.Childf("%s", operandNames[op]), limbs[op])
	}

	carryCtx := ctx.Childf("carry")
	c.carry = make([]*wiop.Column, max(2*c.nbDigits-2, 0))
	for k := range c.carry {
		c.carry[k] = m.NewColumn(carryCtx.Childf("limb-%d", k), q.InputRound)
		m.NewRangeCheck(carryCtx.Childf("range-%d", k), c.carry[k], 1<<c.carryBits)
	}
	for k, coeff := range c.identityCoefficients() {
		m.NewVanishing(ctx.Childf("identity-%d", k), coeff)
	}
	return c
}

// newDigits returns the range-checked digit columns of an operand with the
// given limbs. When a limb holds several digits, it allocates the digits and
// constrains every limb to recompose from them; otherwise the limbs are the
// digits.
func (c *compiledQuery) newDigits(ctx *wiop.ContextFrame, limbs []*wiop.Column) []*wiop.Column {
	m := c.query.Module
	digitBound := 1 << c.digitBits
	digits := limbs
	if c.digitsPerLimb > 1 {
		digits = make([]*wiop.Column, c.nbDigits)
		for j := range digits {
			digits[j] = m.NewColumn(ctx.Childf("digit-%d", j), c.query.InputRound)
		}
		for i, limb := range limbs {
			m.NewVanishing(
				ctx.Childf("recompose-%d", i),
				wiop.Sub(limb.View(), c.limbFromDigits(digits, i)),
			)
		}
	}
	for j, d := range digits {
		m.NewRangeCheck(ctx.Childf("range-%d", j), d, digitBound)
	}
	return digits
}

// limbFromDigits recomposes limb i from its digits.
func (c *compiledQuery) limbFromDigits(digits []*wiop.Column, i int) wiop.Expression {
	limbDigits := digits[i*c.digitsPerLimb : (i+1)*c.digitsPerLimb]
	return polyEval(views(limbDigits), constant(uint64(1)<<c.digitBits))
}

// identityCoefficients returns, for every coefficient k of the product
// polynomial,
//
//	lr_k - mq_k - res_k - (2^s*c_k - c_{k-1})
//
// where lr_k and mq_k are the k-th coefficients of left*right and
// modulus*quotient over the digit columns and c_k is the committed carry minus
// the bias, with c_{-1} and the leading carry zero.
func (c *compiledQuery) identityCoefficients() []wiop.Expression {
	n := c.nbDigits
	bias := constant(c.carryBias())
	radix := constant(uint64(1) << c.digitBits)
	carry := func(k int) wiop.Expression { return wiop.Sub(c.carry[k].View(), bias) }
	coeffProduct := func(a, b []*wiop.Column, k int) wiop.Expression {
		var terms []wiop.Expression
		for i := max(0, k-n+1); i <= min(k, n-1); i++ {
			terms = append(terms, wiop.Mul(a[i].View(), b[k-i].View()))
		}
		return wiop.Sum(terms...)
	}

	out := make([]wiop.Expression, 2*n-1)
	for k := range out {
		e := wiop.Sub(
			coeffProduct(c.digits[opLeft], c.digits[opRight], k),
			coeffProduct(c.digits[opModulus], c.digits[opQuotient], k),
		)
		if k < n {
			e = wiop.Sub(e, c.digits[opResult][k].View())
		}
		if k < len(c.carry) {
			e = wiop.Sub(e, wiop.Mul(radix, carry(k)))
		}
		if k > 0 {
			e = wiop.Add(e, carry(k-1))
		}
		out[k] = e
	}
	return out
}

// polyEval builds the coefficient-basis polynomial Σ_i coeffs[i] * x^i as a
// symbolic expression using Horner's rule.
func polyEval(coeffs []wiop.Expression, x wiop.Expression) wiop.Expression {
	res := coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		res = wiop.Add(wiop.Mul(res, x), coeffs[i])
	}
	return res
}

func views(cols []*wiop.Column) []wiop.Expression {
	out := make([]wiop.Expression, len(cols))
	for i, col := range cols {
		out[i] = col.View()
	}
	return out
}

func constant(v uint64) *wiop.Constant {
	var e field.Element
	e.SetUint64(v)
	return wiop.NewConstantField(e)
}

// proverAction assigns a query's digit and carry columns from the runtime
// assignments of its operand limbs.
type proverAction struct {
	c *compiledQuery
}

func (a *proverAction) Run(rt *wiop.Runtime) {
	c := a.c
	m := c.query.Module
	nbRows := m.RuntimeSize(rt)
	nbLimbs := len(c.query.Result)
	digitMask := uint64(1)<<c.digitBits - 1
	bias := int64(c.carryBias())
	split := c.digitsPerLimb > 1

	limbs := operands(c.query)
	var limbVecs [nbOperands][]*wiop.ConcreteVector
	var digitVals [nbOperands][][]field.Element
	for op := range nbOperands {
		limbVecs[op] = make([]*wiop.ConcreteVector, len(limbs[op]))
		for i, col := range limbs[op] {
			limbVecs[op][i] = rt.GetColumnAssignment(col)
		}
		if split {
			digitVals[op] = newColumnValues(c.nbDigits, nbRows)
		}
	}
	carryVals := newColumnValues(len(c.carry), nbRows)

	parallel.Execute(nbRows, func(start, stop int) {
		var rowLimbs, rowDigits [nbOperands][]uint64
		for op := range nbOperands {
			rowLimbs[op] = make([]uint64, nbLimbs)
			rowDigits[op] = make([]uint64, c.nbDigits)
		}
		scratch := newCarryScratch(c.nbDigits)
		carries := make([]int64, len(c.carry))

		for row := start; row < stop; row++ {
			for op := range nbOperands {
				for i, v := range limbVecs[op] {
					limb := v.ElementAtN(m.Padding, nbRows, row).AsBase()
					rowLimbs[op][i] = limb.Uint64()
					splitDigits(rowLimbs[op][i], uint(c.digitBits), digitMask, rowDigits[op][i*c.digitsPerLimb:(i+1)*c.digitsPerLimb])
				}
				if split {
					for j, d := range rowDigits[op] {
						digitVals[op][j][row].SetUint64(d)
					}
				}
			}
			carryRow(rowDigits, uint(c.digitBits), scratch, carries)
			for k, carry := range carries {
				carryVals[k][row].SetUint64(uint64(carry + bias))
			}
		}
	})

	if split {
		for op := range nbOperands {
			assignColumns(rt, c.digits[op], digitVals[op])
		}
	}
	assignColumns(rt, c.carry, carryVals)
}

// splitDigits writes the little-endian digits of limb into out.
func splitDigits(limb uint64, digitBits uint, digitMask uint64, out []uint64) {
	for d := range out {
		out[d] = (limb >> (uint(d) * digitBits)) & digitMask
	}
}

func newColumnValues(nbCols, nbRows int) [][]field.Element {
	out := make([][]field.Element, nbCols)
	for i := range out {
		out[i] = make([]field.Element, nbRows)
	}
	return out
}

func assignColumns(rt *wiop.Runtime, cols []*wiop.Column, vals [][]field.Element) {
	for i, col := range cols {
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vals[i])})
	}
}

// carryScratch holds the per-coefficient sums of left*right and
// modulus*quotient+result, reused across rows.
type carryScratch struct {
	lhs, rhs []uint64
}

func newCarryScratch(nbDigits int) carryScratch {
	n := max(2*nbDigits-1, 0)
	return carryScratch{lhs: make([]uint64, n), rhs: make([]uint64, n)}
}

// carryRow computes into carries the signed base-2^digitBits carries of one
// row of the multiplication identity: carry k is
//
//	floor((carry_{k-1} + lr_k - mq_k - res_k) / 2^digitBits)
//
// where lr_k and mq_k are the k-th coefficients of the digit products
// left*right and modulus*quotient. For digits in [0, P], P = 2^digitBits - 1,
// and n digits per operand, every coefficient lies in [-(n*P^2+P), n*P^2], so
// by induction every carry lies in [-(n*P+1), n*P]. A sound [layout] keeps the
// coefficients below the field modulus, so neither the uint64 sums nor the
// int64 carries overflow.
func carryRow(digits [nbOperands][]uint64, digitBits uint, s carryScratch, carries []int64) {
	clear(s.lhs)
	clear(s.rhs)
	for i, l := range digits[opLeft] {
		for j, r := range digits[opRight] {
			s.lhs[i+j] += l * r
		}
	}
	for i, m := range digits[opModulus] {
		for j, q := range digits[opQuotient] {
			s.rhs[i+j] += m * q
		}
	}
	for i, r := range digits[opResult] {
		s.rhs[i] += r
	}
	var carry int64
	for k := range carries {
		carry += int64(s.lhs[k]) - int64(s.rhs[k])
		carry >>= digitBits
		carries[k] = carry
	}
}
