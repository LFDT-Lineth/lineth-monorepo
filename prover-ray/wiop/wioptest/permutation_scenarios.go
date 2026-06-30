package wioptest

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// PermutationScenario is a fixture for testing the grandproduct compiler
// (permutation → grand product → Z columns) end-to-end through the full
// pipeline.
//
// Sys holds the pre-compilation system; AssignHonest assigns a witness where A
// and B are equal as multisets, AssignInvalid one where they are not. The
// grandproduct compiler allocates the β/α coin round and the Result round
// itself, so each builder only declares the witness round r0.
type PermutationScenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Sys is the pre-compilation System; each factory call returns an
	// independent Sys.
	Sys *wiop.System
	// AssignHonest assigns oracle columns with a valid permutation witness.
	// After Compile + Prove + Verify, must accept.
	AssignHonest func(rt *wiop.Runtime)
	// AssignInvalid assigns oracle columns that are not a permutation. After
	// Compile + Prove + Verify, must reject.
	AssignInvalid func(rt *wiop.Runtime)
}

// PermutationScenarios returns factory functions for the built-in permutation
// scenarios. The returned Sys is always pre-compilation.
func PermutationScenarios() []func() *PermutationScenario {
	return []func() *PermutationScenario{
		NewPermutationSingleColumnScenario,
		NewPermutationMultiColumnScenario,
		NewPermutationMultiFragmentScenario,
	}
}

// NewPermutationSingleColumnScenario: a single-column permutation. B is a
// reordering of A in the honest case; the invalid case changes one B value so
// the multisets differ.
func NewPermutationSingleColumnScenario() *PermutationScenario {
	sys := wiop.NewSystemf("perm-single")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	return &PermutationScenario{
		Name: "SingleColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colB, makeVec(30, 10, 40, 20))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colB, makeVec(30, 10, 40, 99))
		},
	}
}

// NewPermutationMultiColumnScenario: a two-column permutation. Rows (pairs)
// are permuted together in the honest case.
func NewPermutationMultiColumnScenario() *PermutationScenario {
	sys := wiop.NewSystemf("perm-multi")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	a0 := modA.NewColumn(sys.Context.Childf("A0"), wiop.VisibilityOracle, r0)
	a1 := modA.NewColumn(sys.Context.Childf("A1"), wiop.VisibilityOracle, r0)
	b0 := modB.NewColumn(sys.Context.Childf("B0"), wiop.VisibilityOracle, r0)
	b1 := modB.NewColumn(sys.Context.Childf("B1"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(a0.View(), a1.View())},
		[]wiop.Table{wiop.NewTable(b0.View(), b1.View())},
	)

	return &PermutationScenario{
		Name: "MultiColumn",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			// rows: (1,5) (2,6) (3,7) (4,8) permuted to (3,7)(1,5)(4,8)(2,6).
			rt.AssignColumn(a0, makeVec(1, 2, 3, 4))
			rt.AssignColumn(a1, makeVec(5, 6, 7, 8))
			rt.AssignColumn(b0, makeVec(3, 1, 4, 2))
			rt.AssignColumn(b1, makeVec(7, 5, 8, 6))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			// Swap one B column so a row pairing breaks: (3,5) is not an A row.
			rt.AssignColumn(a0, makeVec(1, 2, 3, 4))
			rt.AssignColumn(a1, makeVec(5, 6, 7, 8))
			rt.AssignColumn(b0, makeVec(3, 1, 4, 2))
			rt.AssignColumn(b1, makeVec(5, 7, 8, 6))
		},
	}
}

// NewPermutationMultiFragmentScenario: A is split across two fragments (two
// modules of different sizes) whose combined multiset equals B's single
// fragment.
func NewPermutationMultiFragmentScenario() *PermutationScenario {
	sys := wiop.NewSystemf("perm-frag")
	r0 := sys.NewRound()
	modA1 := sys.NewSizedModule(sys.Context.Childf("modA1"), 2, wiop.PaddingDirectionNone)
	modA2 := sys.NewSizedModule(sys.Context.Childf("modA2"), 2, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	a1 := modA1.NewColumn(sys.Context.Childf("A1"), wiop.VisibilityOracle, r0)
	a2 := modA2.NewColumn(sys.Context.Childf("A2"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(a1.View()), wiop.NewTable(a2.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	return &PermutationScenario{
		Name: "MultiFragment",
		Sys:  sys,
		AssignHonest: func(rt *wiop.Runtime) {
			rt.AssignColumn(a1, makeVec(10, 20))
			rt.AssignColumn(a2, makeVec(30, 40))
			rt.AssignColumn(colB, makeVec(40, 10, 30, 20))
		},
		AssignInvalid: func(rt *wiop.Runtime) {
			rt.AssignColumn(a1, makeVec(10, 20))
			rt.AssignColumn(a2, makeVec(30, 40))
			rt.AssignColumn(colB, makeVec(40, 10, 30, 99))
		},
	}
}
