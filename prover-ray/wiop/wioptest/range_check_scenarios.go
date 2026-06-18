package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// NewRangeCheckBasicScenario: a single RangeCheck with bound 8 over a
// size-8 module, witness covering every value in [0, 8).
func NewRangeCheckBasicScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-basic")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 8)

	return &RangeCheckCompilerScenario{
		Name: "Basic",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 2, 3, 4, 5, 6, 7))
		},
	}
}

// NewRangeCheckSharedBoundScenario: two RangeChecks with the same bound
// share a single precomputed range column.
func NewRangeCheckSharedBoundScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-shared")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rcA"), colA, 4)
	mod.NewRangeCheck(sys.Context.Childf("rcB"), colB, 4)

	return &RangeCheckCompilerScenario{
		Name: "SharedBound",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 2, 3))
			rt.AssignColumn(colB, makeVec(3, 0, 2, 1))
		},
	}
}

// NewRangeCheckDistinctBoundsScenario: two RangeChecks with different
// bounds, exercising two distinct precomputed range modules.
func NewRangeCheckDistinctBoundsScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-distinct")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rcA"), colA, 4)
	mod.NewRangeCheck(sys.Context.Childf("rcB"), colB, 8)

	return &RangeCheckCompilerScenario{
		Name: "DistinctBounds",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 2, 3))
			rt.AssignColumn(colB, makeVec(5, 6, 7, 4))
		},
	}
}

// NewRangeCheckBoundIsPowerOfTwoScenario: bound exactly matches a power of
// two so NextPowerOfTwo is a no-op and the precomputed range column is
// fully populated.
func NewRangeCheckBoundIsPowerOfTwoScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-pow2")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 16)

	return &RangeCheckCompilerScenario{
		Name: "BoundIsPowerOfTwo",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 8, 15, 1))
		},
	}
}

// NewRangeCheckBoundIsOneScenario: bound 1 → only value 0 is permitted.
// The precomputed range module is size 1.
func NewRangeCheckBoundIsOneScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-bound1")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 1)

	return &RangeCheckCompilerScenario{
		Name: "BoundIsOne",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 0, 0, 0))
		},
	}
}

// NewRangeCheckMultiModuleScenario: two RangeChecks, one on each of two
// different witness modules, both targeting the same bound. The rangecheck
// pass must still share a single range column across both queries even
// though the checked columns live on distinct modules.
func NewRangeCheckMultiModuleScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-cross-mod")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	modA.NewRangeCheck(sys.Context.Childf("rcA"), colA, 4)
	modB.NewRangeCheck(sys.Context.Childf("rcB"), colB, 4)

	return &RangeCheckCompilerScenario{
		Name: "MultiModule",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(0, 1, 2, 3))
			rt.AssignColumn(colB, makeVec(3, 2, 1, 0))
		},
	}
}

// NewRangeCheckLargeBoundScenario: a bound that requires a sizeable
// range column (bound = 128 → range module of size 128). Stresses the
// downstream lookup pipeline at non-trivial scale.
func NewRangeCheckLargeBoundScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-large")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 128)

	return &RangeCheckCompilerScenario{
		Name: "LargeBound",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 1, 63, 127, 50, 100, 7, 99))
		},
	}
}

// NewRangeCheckNonPowerOfTwoBoundScenario: a bound that is NOT a power of
// two (5) — the range-column module is sized to NextPowerOfTwo(5) = 8, with
// only the first 5 slots populated. The remaining slots hold the padding
// value (zero), which is still in [0, 5). All checked column values must
// match the populated portion.
func NewRangeCheckNonPowerOfTwoBoundScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-np2")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 5)

	return &RangeCheckCompilerScenario{
		Name: "NonPowerOfTwoBound",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 4, 2, 1)) // all ∈ [0, 5)
		},
	}
}

// NewRangeCheckAllZerosScenario: a checked column that is uniformly zero.
// Honest M concentrates the entire lookup-table multiplicity on a single
// range-column row.
func NewRangeCheckAllZerosScenario() *RangeCheckCompilerScenario {
	sys := wiop.NewSystemf("rc-zeros")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 8)

	return &RangeCheckCompilerScenario{
		Name: "AllZeros",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			rt.AssignColumn(col, makeVec(0, 0, 0, 0))
		},
	}
}

// --- Soundness scenarios -----------------------------------------------------

// NewRangeCheckValueAtBoundScenario: the witness column contains a value
// equal to the bound B (B is exclusive: only [0, B) is permitted).
func NewRangeCheckValueAtBoundScenario() *RangeCheckSoundnessScenario {
	sys := wiop.NewSystemf("rc-at-bound")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 4)

	return &RangeCheckSoundnessScenario{
		Name: "ValueAtBound",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			// 4 is out of range [0, 4); the downstream M-assignment panics.
			rt.AssignColumn(col, makeVec(0, 1, 2, 4, 0, 0, 0, 0))
		},
	}
}

// NewRangeCheckValueAboveBoundScenario: the witness column contains a
// value strictly greater than the bound.
func NewRangeCheckValueAboveBoundScenario() *RangeCheckSoundnessScenario {
	sys := wiop.NewSystemf("rc-above")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewRangeCheck(sys.Context.Childf("rc"), col, 4)

	return &RangeCheckSoundnessScenario{
		Name: "ValueAboveBound",
		Sys:  sys,
		AssignWitness: func(rt *wiop.Runtime) {
			// 7 >> 4; the downstream M-assignment panics.
			rt.AssignColumn(col, makeVec(0, 1, 2, 7, 0, 0, 0, 0))
		},
	}
}
