package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// NewNonNativeScenario returns a scenario for a [wiop.NonNative] query,
// exercising Check via limb composition only (no compiler pass and no
// challenge coin involved).
//
// Two limbs, 16 bits/limb (values up to 2^32-1), single row. Every operand
// except Result is >= 2^16, so the relation spans both limbs:
//
//   - left = 99991 = (34455, 1), right = 99989 = (34453, 1)
//
//   - modulus = 100000 = (34464, 1)
//
//   - quotient = 99980 = (34444, 1), result = 99 = (99, 0)
//
//   - left*right = 9998000099 = quotient*modulus + result
//     (99991*99989 = 99980*100000 + 99)
//
//   - Invalid: result set to 100 (99991*99989 != 99980*100000 + 100).
func NewNonNativeScenario() *Scenario {
	sys := wiop.NewSystemf("nn-sc")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)

	newLimbs := func(label string) []*wiop.Column {
		return []*wiop.Column{
			mod.NewColumn(sys.Context.Childf("%s-limb-0", label), r0),
			mod.NewColumn(sys.Context.Childf("%s-limb-1", label), r0),
		}
	}
	left := newLimbs("left")
	right := newLimbs("right")
	modulus := newLimbs("modulus")
	result := newLimbs("result")
	quotient := newLimbs("quotient")

	q := mod.NewNonNative(sys.Context.Childf("nn"), 16, left, right, modulus, result, quotient)

	assignLimbs := func(rt *wiop.Runtime, cols []*wiop.Column, limb0, limb1 uint64) {
		rt.AssignColumn(cols[0], makeVec(limb0))
		rt.AssignColumn(cols[1], makeVec(limb1))
	}

	return &Scenario{
		Name:  "NonNative",
		Sys:   sys,
		Query: q,
		RunHonest: func(rt *wiop.Runtime) {
			assignLimbs(rt, left, 34455, 1)     // 99991
			assignLimbs(rt, right, 34453, 1)    // 99989
			assignLimbs(rt, modulus, 34464, 1)  // 100000
			assignLimbs(rt, result, 99, 0)      // 99
			assignLimbs(rt, quotient, 34444, 1) // 99980
		},
		RunInvalid: func(rt *wiop.Runtime) {
			assignLimbs(rt, left, 34455, 1)     // 99991
			assignLimbs(rt, right, 34453, 1)    // 99989
			assignLimbs(rt, modulus, 34464, 1)  // 100000
			assignLimbs(rt, result, 100, 0)     // wrong: 99991*99989 != 99980*100000+100
			assignLimbs(rt, quotient, 34444, 1) // 99980
		},
	}
}
