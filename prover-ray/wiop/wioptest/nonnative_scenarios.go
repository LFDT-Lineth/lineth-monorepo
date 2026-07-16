package wioptest

import (
	"crypto/rand"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// nonNativeBitsPerLimb is the limb width used by every NonNative fixture in
// this file.
const nonNativeBitsPerLimb = 16

// NonNativeScenario is a fixture for testing the nonnative compiler pass
// (package wiop/compilers/nonnative) in combination with the downstream
// localvanishing/global pipeline that discharges the [wiop.Vanishing]
// identity it produces.
//
// A test typically calls
//
//	nonnative.Compile(sc.Sys)
//	// then the standard localvanishing/global pipeline
type NonNativeScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignHonest assigns the operand limb columns with a witness satisfying
	// Left*Right = Quotient*Modulus + Result on every row.
	AssignHonest func(rt *wiop.Runtime)
	// AssignInvalid assigns operand limb columns that violate the relation on
	// at least one row.
	AssignInvalid func(rt *wiop.Runtime)
}

// NonNativeScenarios returns factory functions for the built-in NonNative
// compiler-integration fixtures: one per representative input width, plus a
// handful of exceptional-input fixtures (zero/maximal/undersized modulus,
// unreduced operands).
func NonNativeScenarios() []func() *NonNativeScenario {
	return []func() *NonNativeScenario{
		NewNonNativeU64Scenario,
		NewNonNativeU128Scenario,
		NewNonNativeU256Scenario,
		NewNonNativeModulusZeroScenario,
		NewNonNativeModulusMaxScenario,
		NewNonNativeModulusSmallScenario,
		NewNonNativeUnreducedOperandsScenario,
		NewNonNativeNonUniqueDecompositionScenario,
	}
}

// NewNonNativeU64Scenario returns a NonNative fixture with 64-bit operands
// (4 limbs of 16 bits).
func NewNonNativeU64Scenario() *NonNativeScenario {
	return newNonNativeRandomScenario("NonNativeU64", 64)
}

// NewNonNativeU128Scenario returns a NonNative fixture with 128-bit operands
// (8 limbs of 16 bits).
func NewNonNativeU128Scenario() *NonNativeScenario {
	return newNonNativeRandomScenario("NonNativeU128", 128)
}

// NewNonNativeU256Scenario returns a NonNative fixture with 256-bit operands
// (16 limbs of 16 bits).
func NewNonNativeU256Scenario() *NonNativeScenario {
	return newNonNativeRandomScenario("NonNativeU256", 256)
}

// NewNonNativeModulusZeroScenario returns a NonNative fixture where Modulus
// is zero on every row. Since Quotient*Modulus is then always zero
// regardless of Quotient, the relation degenerates to Left*Right = Result.
// Every operand spans at least two limbs (>= 2^16). Row 0 additionally
// assigns a non-zero Quotient (Quotient is zero on the other rows) to
// demonstrate that, when Modulus is zero, Quotient can take any value.
func NewNonNativeModulusZeroScenario() *NonNativeScenario {
	lefts := []*big.Int{big.NewInt(100003), big.NewInt(100019), big.NewInt(100043), big.NewInt(100057)}
	rights := []*big.Int{big.NewInt(100007), big.NewInt(100021), big.NewInt(100049), big.NewInt(100069)}
	moduli := []*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)}
	quotientOverride := []*big.Int{big.NewInt(42), nil, nil, nil}
	return newNonNativeScenario("NonNativeModulusZero", 64, lefts, rights, moduli, quotientOverride)
}

// NewNonNativeModulusMaxScenario returns a NonNative fixture where Modulus
// equals the maximum value representable by the allotted limbs (2^64 - 1,
// i.e. every limb saturated to 0xFFFF), exercising the compiler's carry
// arithmetic at the top of the representable range. Left and Right are
// random in [0, Modulus), as in [newNonNativeRandomScenario].
func NewNonNativeModulusMaxScenario() *NonNativeScenario {
	const (
		nbBits = 64
		nbRows = 4
	)
	maxVal := new(big.Int).Lsh(big.NewInt(1), uint(nbBits))
	maxVal.Sub(maxVal, big.NewInt(1))

	lefts := make([]*big.Int, nbRows)
	rights := make([]*big.Int, nbRows)
	moduli := make([]*big.Int, nbRows)
	for i := range nbRows {
		var err error
		moduli[i] = maxVal
		lefts[i], err = rand.Int(rand.Reader, maxVal)
		if err != nil {
			panic(err)
		}
		rights[i], err = rand.Int(rand.Reader, maxVal)
		if err != nil {
			panic(err)
		}
	}
	return newNonNativeScenario("NonNativeModulusMax", nbBits, lefts, rights, moduli, nil)
}

// NewNonNativeModulusSmallScenario returns a NonNative fixture where Modulus
// (3) is far smaller than the maximum value the allotted limbs could hold,
// so most of Modulus's limbs (and, by extension, Result's limbs) are zero
// even though Left is random over the entire representable range and so
// typically spans every limb. Right is kept random in [0, Modulus), which
// guarantees the derived Quotient (= floor(Left*Right/Modulus)) never
// exceeds Left, since Right < Modulus.
func NewNonNativeModulusSmallScenario() *NonNativeScenario {
	const (
		nbBits = 64
		nbRows = 4
	)
	modulus := big.NewInt(3)
	bound := new(big.Int).Lsh(big.NewInt(1), uint(nbBits))

	lefts := make([]*big.Int, nbRows)
	rights := make([]*big.Int, nbRows)
	moduli := make([]*big.Int, nbRows)
	for i := range nbRows {
		var err error
		moduli[i] = modulus
		lefts[i], err = rand.Int(rand.Reader, bound)
		if err != nil {
			panic(err)
		}
		rights[i], err = rand.Int(rand.Reader, modulus)
		if err != nil {
			panic(err)
		}
	}
	return newNonNativeScenario("NonNativeModulusSmall", nbBits, lefts, rights, moduli, nil)
}

// NewNonNativeUnreducedOperandsScenario returns a NonNative fixture where
// Left and/or Right are not reduced modulo Modulus: row 0 has an
// out-of-range Left, row 1 an out-of-range Right, row 2 both, and row 3
// neither (a fully-reduced control row for contrast). [NonNative] places no
// requirement that operands be pre-reduced, only that Result is. Modulus and
// every unreduced operand span at least two limbs (>= 2^16).
func NewNonNativeUnreducedOperandsScenario() *NonNativeScenario {
	modulus := big.NewInt(70000)
	lefts := []*big.Int{big.NewInt(100003), big.NewInt(9), big.NewInt(150001), big.NewInt(69999)}
	rights := []*big.Int{big.NewInt(5), big.NewInt(100007), big.NewInt(130003), big.NewInt(3)}
	moduli := []*big.Int{modulus, modulus, modulus, modulus}
	return newNonNativeScenario("NonNativeUnreducedOperands", 64, lefts, rights, moduli, nil)
}

// NewNonNativeNonUniqueDecompositionScenario documents a known limitation of
// the compiled [wiop.NonNative] protocol: it enforces only the algebraic
// identity Left*Right = Quotient*Modulus + Result, not that Result is the
// unique, canonical remainder (0 <= Result < Modulus). Given any valid
// decomposition, the pair (Quotient-1, Result+Modulus) satisfies the exact same
// identity:
//
//	(Quotient-1)*Modulus + (Result+Modulus) = Quotient*Modulus + Result
//
// — and is therefore accepted by the compiled proof just as readily as the
// canonical pair, even though [wiop.NonNative.Check] (a non-cryptographic,
// convenience check only) does reject it as "not reduced".
//
// This is fine for uses that only care about the algebraic relation; it only
// matters when a caller needs Result to be canonical, e.g. when parsing or
// hashing it downstream. Such callers must add their own range check binding
// Result < Modulus — the nonnative compiler does not provide one.
//
// Two limbs, 16 bits/limb:
//
//   - left = 99991, right = 99989, modulus = 100000
//   - canonical: quotient = 99980, result = 99
//     (99991*99989 = 99980*100000 + 99)
//   - alternate: quotient = 99979, result = 100099 = 99+100000
//     (99991*99989 = 99979*100000 + 100099 too, but 100099 >= modulus)
func NewNonNativeNonUniqueDecompositionScenario() *NonNativeScenario {
	lefts := []*big.Int{big.NewInt(99991), big.NewInt(99991)}
	rights := []*big.Int{big.NewInt(99989), big.NewInt(99989)}
	moduli := []*big.Int{big.NewInt(100000), big.NewInt(100000)}
	quotientOverride := []*big.Int{big.NewInt(99980), big.NewInt(99979)}
	return newNonNativeScenario("NonNativeNonUniqueDecomposition", 64, lefts, rights, moduli, quotientOverride)
}

// newNonNativeRandomScenario builds a NonNative fixture for a given operand
// width using random moduli/operands. The module has 4 rows; modulus[i] is
// uniform random in (0, 2^nbBits), left[i] and right[i] are uniform random in
// [0, modulus[i]).
func newNonNativeRandomScenario(name string, nbBits int) *NonNativeScenario {
	const nbRows = 4
	var err error

	bound := new(big.Int).Lsh(big.NewInt(1), uint(nbBits))
	bound.Sub(bound, big.NewInt(1))

	lefts := make([]*big.Int, nbRows)
	rights := make([]*big.Int, nbRows)
	moduli := make([]*big.Int, nbRows)
	for i := range nbRows {
		moduli[i], err = rand.Int(rand.Reader, bound)
		if err != nil {
			panic(err)
		}
		lefts[i], err = rand.Int(rand.Reader, moduli[i])
		if err != nil {
			panic(err)
		}
		rights[i], err = rand.Int(rand.Reader, moduli[i])
		if err != nil {
			panic(err)
		}
	}

	return newNonNativeScenario(name, nbBits, lefts, rights, moduli, nil)
}

// newNonNativeScenario builds a NonNative fixture for a given operand width
// from explicit per-row Left/Right/Modulus values. Quotient and Result are
// derived: Quotient, Result = Left*Right / Modulus, Left*Right % Modulus, or,
// when Modulus is zero, Quotient = 0, Result = Left*Right (Quotient*Modulus
// is then zero regardless of Quotient, so the relation degenerates to
// Left*Right = Result; every row's Left*Right must therefore fit within
// nbBits bits in that case).
//
// quotientOverride, if non-nil, replaces the derived Quotient for any row
// whose entry is itself non-nil; Result is then recomputed as
// Left*Right - Quotient*Modulus so the relation still holds for that row.
// This is only meaningful for demonstration purposes when Modulus is zero,
// where any Quotient value keeps Quotient*Modulus at zero. Pass nil to derive
// every row's Quotient normally.
//
// AssignInvalid perturbs the last row's Result by ±1, violating the relation
// on exactly that row while keeping it within [0, Modulus) (or non-negative,
// when Modulus is zero).
func newNonNativeScenario(name string, nbBits int, lefts, rights, moduli,
	quotientOverride []*big.Int) *NonNativeScenario {
	nbRows := len(lefts)
	nbLimbs := (nbBits + nonNativeBitsPerLimb - 1) / nonNativeBitsPerLimb

	quotients := make([]*big.Int, nbRows)
	results := make([]*big.Int, nbRows)
	for i := range nbRows {
		product := new(big.Int).Mul(lefts[i], rights[i])
		switch {
		case quotientOverride != nil && quotientOverride[i] != nil:
			quotients[i] = new(big.Int).Set(quotientOverride[i])
			results[i] = new(big.Int).Sub(product, new(big.Int).Mul(quotients[i], moduli[i]))
		case moduli[i].Sign() == 0:
			quotients[i] = big.NewInt(0)
			results[i] = product
		default:
			quotients[i], results[i] = new(big.Int).QuoRem(product, moduli[i], new(big.Int))
		}
	}

	sys := wiop.NewSystemf("%s", name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), nbRows, wiop.PaddingDirectionNone)

	newLimbCols := func(label string) []*wiop.Column {
		cols := make([]*wiop.Column, nbLimbs)
		for i := range cols {
			cols[i] = mod.NewColumn(sys.Context.Childf("%s-limb-%d", label, i), wiop.VisibilityOracle, r0)
		}
		return cols
	}
	left := newLimbCols("left")
	right := newLimbCols("right")
	mods := newLimbCols("modulus")
	res := newLimbCols("result")
	quo := newLimbCols("quotient")

	mod.NewNonNative(sys.Context.Childf("nn"), nonNativeBitsPerLimb, left, right, mods, res, quo)

	// tamperRow < 0 assigns the honest results; otherwise result[tamperRow]
	// is perturbed by ±1 (staying non-negative) to violate the relation on
	// exactly that row.
	assign := func(rt *wiop.Runtime, tamperRow int) {
		assignLimbColumns(rt, left, lefts, nbLimbs)
		assignLimbColumns(rt, right, rights, nbLimbs)
		assignLimbColumns(rt, mods, moduli, nbLimbs)
		assignLimbColumns(rt, quo, quotients, nbLimbs)

		resultsToAssign := results
		if tamperRow >= 0 {
			resultsToAssign = make([]*big.Int, nbRows)
			copy(resultsToAssign, results)
			tampered := new(big.Int).Set(results[tamperRow])
			if tampered.Sign() == 0 {
				tampered.Add(tampered, big.NewInt(1))
			} else {
				tampered.Sub(tampered, big.NewInt(1))
			}
			resultsToAssign[tamperRow] = tampered
		}
		assignLimbColumns(rt, res, resultsToAssign, nbLimbs)
	}

	return &NonNativeScenario{
		Name: name,
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			assign(rt, -1)
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			assign(rt, nbRows-1)
		},
	}
}

// splitLimbs decomposes a into nbLimbs little-endian limbs of
// nonNativeBitsPerLimb bits each.
func splitLimbs(a *big.Int, nbLimbs int) []field.Element {
	els := make([]field.Element, nbLimbs)
	mask := new(big.Int).Lsh(big.NewInt(1), uint(nonNativeBitsPerLimb))
	mask.Sub(mask, big.NewInt(1))
	rem := new(big.Int).Set(a)
	for i := range nbLimbs {
		var limb big.Int
		limb.And(rem, mask)
		els[i].SetBigInt(&limb)
		rem.Rsh(rem, uint(nonNativeBitsPerLimb))
	}
	return els
}

// assignLimbColumns assigns cols (one column per little-endian limb) with the
// per-row decomposition of vals.
func assignLimbColumns(rt *wiop.Runtime, cols []*wiop.Column, vals []*big.Int, nbLimbs int) {
	limbRows := make([][]field.Element, nbLimbs)
	for i := range limbRows {
		limbRows[i] = make([]field.Element, len(vals))
	}
	for row, v := range vals {
		limbs := splitLimbs(v, nbLimbs)
		for i, l := range limbs {
			limbRows[i][row] = l
		}
	}
	for i, col := range cols {
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(limbRows[i])})
	}
}
