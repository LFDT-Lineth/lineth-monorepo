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
	// Left*Right = Quotient*Modulus + Result, 0 <= Result < Modulus on every
	// active row.
	AssignHonest func(rt *wiop.Runtime)
	// AssignInvalid assigns operand limb columns that violate the relation on
	// at least one active row.
	AssignInvalid func(rt *wiop.Runtime)
}

// NonNativeScenarios returns factory functions for the built-in NonNative
// compiler-integration fixtures: one per representative input width, plus a
// handful of exceptional-input fixtures (maximal/undersized modulus,
// unreduced operands, unreduced result, inactive rows).
func NonNativeScenarios() []func() *NonNativeScenario {
	return []func() *NonNativeScenario{
		NewNonNativeU64Scenario,
		NewNonNativeU128Scenario,
		NewNonNativeU256Scenario,
		NewNonNativeSelectorScenario,
		NewNonNativeModulusMaxScenario,
		NewNonNativeModulusSmallScenario,
		NewNonNativeUnreducedOperandsScenario,
		NewNonNativeUnreducedResultScenario,
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

// NewNonNativeSelectorScenario returns a NonNative fixture with a selector
// column, whose inactive rows hold witnesses that no active row could:
//
//   - row 1: Modulus = 0, Left*Right = Result, Quotient = 42;
//   - row 2: Result = 2^70, whose top 16-bit limb is out of range, and the
//     relation does not hold.
//
// Rows 0 and 3 are active and honest. AssignInvalid activates row 1, where a
// zero Modulus leaves no Result < Modulus.
func NewNonNativeSelectorScenario() *NonNativeScenario {
	garbage := new(big.Int).Lsh(big.NewInt(1), 70)
	return newNonNativeFixture(nonNativeFixture{
		name:             "NonNativeSelector",
		nbBits:           64,
		lefts:            []*big.Int{big.NewInt(99991), big.NewInt(100019), big.NewInt(5), big.NewInt(69999)},
		rights:           []*big.Int{big.NewInt(99989), big.NewInt(100021), big.NewInt(7), big.NewInt(3)},
		moduli:           []*big.Int{big.NewInt(100000), big.NewInt(0), big.NewInt(3), big.NewInt(70000)},
		quotientOverride: []*big.Int{nil, big.NewInt(42), nil, nil},
		selector:         []uint64{1, 0, 0, 1},
		tamperHonest: func(v *nonNativeRows) {
			v.quotients[2], v.results[2] = big.NewInt(0), garbage
		},
		tamperInvalid: func(v *nonNativeRows) {
			v.selector[1] = 1
		},
	})
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

// NewNonNativeUnreducedResultScenario returns a NonNative fixture whose
// invalid witness is a non-canonical decomposition: given a valid
// decomposition, the pair (Quotient-1, Result+Modulus) satisfies the same
// identity
//
//	(Quotient-1)*Modulus + (Result+Modulus) = Quotient*Modulus + Result
//
// but Result+Modulus >= Modulus, which the compiled query must reject.
//
// Two limbs, 16 bits/limb:
//
//   - left = 99991, right = 99989, modulus = 100000
//   - honest: quotient = 99980, result = 99
//     (99991*99989 = 99980*100000 + 99)
//   - invalid on row 1: quotient = 99979, result = 100099 = 99+100000
func NewNonNativeUnreducedResultScenario() *NonNativeScenario {
	lefts := []*big.Int{big.NewInt(99991), big.NewInt(99991)}
	rights := []*big.Int{big.NewInt(99989), big.NewInt(99989)}
	moduli := []*big.Int{big.NewInt(100000), big.NewInt(100000)}
	return newNonNativeFixture(nonNativeFixture{
		name:   "NonNativeUnreducedResult",
		nbBits: 64,
		lefts:  lefts,
		rights: rights,
		moduli: moduli,
		tamperInvalid: func(v *nonNativeRows) {
			v.quotients[1].Sub(v.quotients[1], big.NewInt(1))
			v.results[1].Add(v.results[1], v.moduli[1])
		},
	})
}

// newNonNativeRandomScenario builds a NonNative fixture for a given operand
// width using random moduli/operands. The module has 4 rows; modulus[i] is
// uniform random in (0, 2^nbBits), left[i] and right[i] are uniform random in
// [0, modulus[i]).
func newNonNativeRandomScenario(name string, nbBits int) *NonNativeScenario {
	const nbRows = 4
	var err error

	bound := new(big.Int).Lsh(big.NewInt(1), uint(nbBits))

	lefts := make([]*big.Int, nbRows)
	rights := make([]*big.Int, nbRows)
	moduli := make([]*big.Int, nbRows)
	for i := range nbRows {
		// Ensure modulus is non-zero; crypto/rand.Int rejects max <= 0.
		for {
			moduli[i], err = rand.Int(rand.Reader, bound)
			if err != nil {
				panic(err)
			}
			if moduli[i].Sign() != 0 {
				break
			}
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
// from explicit per-row Left/Right/Modulus values, with every row active.
// Quotient and Result are derived by [decomposeRows].
//
// AssignInvalid perturbs the last row's Result by ±1, violating the relation
// on exactly that row.
func newNonNativeScenario(name string, nbBits int, lefts, rights, moduli,
	quotientOverride []*big.Int) *NonNativeScenario {
	return newNonNativeFixture(nonNativeFixture{
		name:             name,
		nbBits:           nbBits,
		lefts:            lefts,
		rights:           rights,
		moduli:           moduli,
		quotientOverride: quotientOverride,
	})
}

// nonNativeFixture describes the rows of a NonNative fixture.
type nonNativeFixture struct {
	name                  string
	nbBits                int
	lefts, rights, moduli []*big.Int
	// quotientOverride is passed to [decomposeRows].
	quotientOverride []*big.Int
	// selector, if non-nil, gives the query a selector column with these
	// per-row values.
	selector []uint64
	// tamperHonest, if non-nil, edits the derived honest rows.
	tamperHonest func(v *nonNativeRows)
	// tamperInvalid, if non-nil, edits a copy of the honest rows into the
	// invalid witness. By default the last row's Result is perturbed by ±1.
	tamperInvalid func(v *nonNativeRows)
}

// nonNativeRows holds the per-row operand values of a NonNative witness.
type nonNativeRows struct {
	lefts, rights, moduli, quotients, results []*big.Int
	selector                                  []uint64
}

func (v *nonNativeRows) clone() *nonNativeRows {
	cloneInts := func(xs []*big.Int) []*big.Int {
		out := make([]*big.Int, len(xs))
		for i, x := range xs {
			out[i] = new(big.Int).Set(x)
		}
		return out
	}
	return &nonNativeRows{
		lefts:     cloneInts(v.lefts),
		rights:    cloneInts(v.rights),
		moduli:    cloneInts(v.moduli),
		quotients: cloneInts(v.quotients),
		results:   cloneInts(v.results),
		selector:  append([]uint64(nil), v.selector...),
	}
}

// perturbLastResult moves the last row's Result by ±1, staying non-negative.
func perturbLastResult(v *nonNativeRows) {
	last := v.results[len(v.results)-1]
	if last.Sign() == 0 {
		last.Add(last, big.NewInt(1))
	} else {
		last.Sub(last, big.NewInt(1))
	}
}

// newNonNativeFixture builds the [NonNativeScenario] described by f.
func newNonNativeFixture(f nonNativeFixture) *NonNativeScenario {
	nbRows := len(f.lefts)
	nbLimbs := (f.nbBits + nonNativeBitsPerLimb - 1) / nonNativeBitsPerLimb

	sys := wiop.NewSystemf("%s", f.name)
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), nbRows, wiop.PaddingDirectionNone)

	newLimbCols := func(label string) []*wiop.Column {
		cols := make([]*wiop.Column, nbLimbs)
		for i := range cols {
			cols[i] = mod.NewColumn(sys.Context.Childf("%s-limb-%d", label, i), r0)
		}
		return cols
	}
	left := newLimbCols("left")
	right := newLimbCols("right")
	mods := newLimbCols("modulus")
	res := newLimbCols("result")
	quo := newLimbCols("quotient")
	var selector *wiop.Column
	if f.selector != nil {
		selector = mod.NewColumn(sys.Context.Childf("selector"), r0)
	}

	mod.NewNonNative(sys.Context.Childf("nn"), nonNativeBitsPerLimb, left, right, mods, res, quo, selector)

	assign := func(rt *wiop.Runtime, v *nonNativeRows) {
		assignLimbColumns(rt, left, v.lefts, nbLimbs)
		assignLimbColumns(rt, right, v.rights, nbLimbs)
		assignLimbColumns(rt, mods, v.moduli, nbLimbs)
		assignLimbColumns(rt, quo, v.quotients, nbLimbs)
		assignLimbColumns(rt, res, v.results, nbLimbs)
		if selector != nil {
			sel := make([]field.Element, nbRows)
			for row, b := range v.selector {
				sel[row].SetUint64(b)
			}
			rt.AssignColumn(selector, &wiop.ConcreteVector{Plain: field.VecFromBase(sel)})
		}
	}

	honest := &nonNativeRows{lefts: f.lefts, rights: f.rights, moduli: f.moduli, selector: f.selector}
	honest.quotients, honest.results = decomposeRows(f.lefts, f.rights, f.moduli, f.quotientOverride)
	if f.tamperHonest != nil {
		f.tamperHonest(honest)
	}
	invalid := honest.clone()
	if f.tamperInvalid != nil {
		f.tamperInvalid(invalid)
	} else {
		perturbLastResult(invalid)
	}

	return &NonNativeScenario{
		Name:          f.name,
		Sys:           sys,
		AssignHonest:  func(rt *wiop.Runtime) { assign(rt, honest) },
		AssignInvalid: func(rt *wiop.Runtime) { assign(rt, invalid) },
	}
}

// decomposeRows derives, for every row, Quotient, Result = Left*Right / Modulus,
// Left*Right % Modulus, or, when Modulus is zero, Quotient = 0,
// Result = Left*Right (Quotient*Modulus is then zero regardless of Quotient,
// so the relation degenerates to Left*Right = Result; every row's Left*Right
// must therefore fit within the operand width in that case).
//
// quotientOverride, if non-nil, replaces the derived Quotient for any row
// whose entry is itself non-nil; Result is then recomputed as
// Left*Right - Quotient*Modulus so the relation still holds for that row.
// Pass nil to derive every row's Quotient normally.
func decomposeRows(lefts, rights, moduli, quotientOverride []*big.Int) (quotients, results []*big.Int) {
	quotients = make([]*big.Int, len(lefts))
	results = make([]*big.Int, len(lefts))
	for i := range lefts {
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
	return quotients, results
}

// splitLimbs decomposes a into nbLimbs little-endian limbs of
// nonNativeBitsPerLimb bits each. The top limb takes every remaining bit, so a
// value of nbLimbs*nonNativeBitsPerLimb bits or more gets an out-of-range top
// limb.
func splitLimbs(a *big.Int, nbLimbs int) []field.Element {
	els := make([]field.Element, nbLimbs)
	mask := new(big.Int).Lsh(big.NewInt(1), uint(nonNativeBitsPerLimb))
	mask.Sub(mask, big.NewInt(1))
	rem := new(big.Int).Set(a)
	for i := range nbLimbs - 1 {
		var limb big.Int
		limb.And(rem, mask)
		els[i].SetBigInt(&limb)
		rem.Rsh(rem, uint(nonNativeBitsPerLimb))
	}
	els[nbLimbs-1].SetBigInt(rem)
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
