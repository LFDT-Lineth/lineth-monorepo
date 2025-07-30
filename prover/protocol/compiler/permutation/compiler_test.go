package permutation

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func TestPermutationPass(t *testing.T) {

	testCases := []struct {
		Define     func(*wizard.Builder)
		Prove      func(*wizard.ProverRuntime)
		Title      string
		ShouldPass bool
	}{
		{
			Define: func(builder *wizard.Builder) {
				a := builder.RegisterCommit("A", 16)
				b := builder.RegisterCommit("B", 16)
				builder.Permutation("PERM", []ifaces.Column{a}, []ifaces.Column{b})
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("B", smartvectors.ForTest(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0))
			},

			Title:      "single-column",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 16),
					builder.RegisterCommit("A2", 16),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 16),
					builder.RegisterCommit("B2", 16),
				}
				// the same permutation amon columns of a and b
				builder.Permutation("PERM", a, b)
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("A2", smartvectors.ForTest(10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 110, 111, 112, 113, 114, 115))
				run.AssignColumn("B1", smartvectors.ForTest(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0))
				run.AssignColumn("B2", smartvectors.ForTest(115, 114, 113, 112, 111, 110, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10))
			},

			Title:      "two-columnss",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 16),
					builder.RegisterCommit("A2", 16),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 16),
					builder.RegisterCommit("B2", 16),
				}
				// PERM1 does not have to be the same permutations as PERM2
				builder.Permutation("PERM1", a[:1], b[:1])
				builder.Permutation("PERM2", a[1:], b[1:])
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("A2", smartvectors.ForTest(11, 10, 12, 13, 14, 15, 16, 17, 18, 19, 110, 111, 112, 113, 114, 115))
				run.AssignColumn("B1", smartvectors.ForTest(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0))
				run.AssignColumn("B2", smartvectors.ForTest(115, 114, 113, 112, 111, 110, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10))
			},

			Title:      "two queries for one column",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 16),
					builder.RegisterCommit("A2", 16),
					builder.RegisterCommit("A3", 16),
					builder.RegisterCommit("A4", 16),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 16),
					builder.RegisterCommit("B2", 16),
					builder.RegisterCommit("B3", 16),
					builder.RegisterCommit("B4", 16),
				}
				builder.Permutation("PERM1", a[0:1], b[0:1])
				builder.Permutation("PERM2", a[1:2], b[1:2])
				builder.Permutation("PERM3", a[2:3], b[2:3])
				builder.Permutation("PERM4", a[3:4], b[3:4])
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("A2", smartvectors.ForTest(11, 10, 12, 13, 14, 15, 16, 17, 18, 19, 110, 111, 112, 113, 114, 115))
				run.AssignColumn("A3", smartvectors.ForTest(22, 20, 22, 23, 24, 25, 26, 27, 28, 29, 220, 222, 222, 223, 224, 225))
				run.AssignColumn("A4", smartvectors.ForTest(122, 120, 122, 123, 124, 125, 126, 127, 128, 129, 1220, 1222, 1222, 1223, 1224, 1225))
				run.AssignColumn("B1", smartvectors.ForTest(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0))
				run.AssignColumn("B2", smartvectors.ForTest(115, 114, 113, 112, 111, 110, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10))
				run.AssignColumn("B3", smartvectors.ForTest(225, 224, 223, 222, 222, 220, 29, 28, 27, 26, 25, 24, 23, 22, 22, 20))
				run.AssignColumn("B4", smartvectors.ForTest(1225, 1224, 1223, 1222, 1222, 1220, 129, 128, 127, 126, 125, 124, 123, 122, 122, 120))
			},

			Title:      "4 queries for one column, all on one column",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 8),
					builder.RegisterCommit("A2", 8),
					builder.RegisterCommit("A3", 8),
					builder.RegisterCommit("A4", 8),
					builder.RegisterCommit("A5", 8),
					builder.RegisterCommit("A6", 8),
					builder.RegisterCommit("A7", 8),
					builder.RegisterCommit("A8", 8),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 8),
					builder.RegisterCommit("B2", 8),
					builder.RegisterCommit("B3", 8),
					builder.RegisterCommit("B4", 8),
					builder.RegisterCommit("B5", 8),
					builder.RegisterCommit("B6", 8),
					builder.RegisterCommit("B7", 8),
					builder.RegisterCommit("B8", 8),
				}
				// the permutation between a[0] and b [0] is the same permutation as the one between a[1] and b[1]
				builder.Permutation("PERM1", a[0:2], b[0:2])
				builder.Permutation("PERM2", a[2:4], b[2:4])
				builder.Permutation("PERM3", a[4:6], b[4:6])
				builder.Permutation("PERM4", a[6:], b[6:])
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7))
				run.AssignColumn("A2", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))
				run.AssignColumn("A3", smartvectors.ForTest(2, 3, 4, 5, 6, 7, 8, 9))
				run.AssignColumn("A4", smartvectors.ForTest(3, 4, 5, 6, 7, 8, 9, 10))
				run.AssignColumn("A5", smartvectors.ForTest(10, 11, 12, 13, 14, 15, 16, 17))
				run.AssignColumn("A6", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18))
				run.AssignColumn("A7", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 19))
				run.AssignColumn("A8", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 110))

				run.AssignColumn("B1", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 0))
				run.AssignColumn("B2", smartvectors.ForTest(2, 3, 4, 5, 6, 7, 8, 1))
				run.AssignColumn("B3", smartvectors.ForTest(3, 4, 5, 6, 7, 8, 9, 2))
				run.AssignColumn("B4", smartvectors.ForTest(4, 5, 6, 7, 8, 9, 10, 3))
				run.AssignColumn("B5", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 10))
				run.AssignColumn("B6", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 11))
				run.AssignColumn("B7", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 12))
				run.AssignColumn("B8", smartvectors.ForTest(14, 15, 16, 17, 18, 19, 110, 13))
			},

			Title:      "4 queries for one column, all on two columns",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 4),
					builder.RegisterCommit("A2", 4),
					builder.RegisterCommit("A3", 4),
					builder.RegisterCommit("A4", 4),
					builder.RegisterCommit("A5", 8),
					builder.RegisterCommit("A6", 8),
					builder.RegisterCommit("A7", 8),
					builder.RegisterCommit("A8", 8),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 4),
					builder.RegisterCommit("B2", 4),
					builder.RegisterCommit("B3", 4),
					builder.RegisterCommit("B4", 4),
					builder.RegisterCommit("B5", 8),
					builder.RegisterCommit("B6", 8),
					builder.RegisterCommit("B7", 8),
					builder.RegisterCommit("B8", 8),
				}
				builder.Permutation("PERM1", a[0:2], b[0:2])
				builder.Permutation("PERM2", a[2:4], b[2:4])
				builder.Permutation("PERM3", a[4:6], b[4:6])
				builder.Permutation("PERM4", a[6:], b[6:])
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3))
				run.AssignColumn("A2", smartvectors.ForTest(1, 2, 3, 4))
				run.AssignColumn("A3", smartvectors.ForTest(2, 3, 4, 5))
				run.AssignColumn("A4", smartvectors.ForTest(3, 4, 5, 6))
				run.AssignColumn("A5", smartvectors.ForTest(10, 11, 12, 13, 14, 15, 16, 17))
				run.AssignColumn("A6", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18))
				run.AssignColumn("A7", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 19))
				run.AssignColumn("A8", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 110))

				run.AssignColumn("B1", smartvectors.ForTest(1, 2, 3, 0))
				run.AssignColumn("B2", smartvectors.ForTest(2, 3, 4, 1))
				run.AssignColumn("B3", smartvectors.ForTest(3, 4, 5, 2))
				run.AssignColumn("B4", smartvectors.ForTest(4, 5, 6, 3))
				run.AssignColumn("B5", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 10))
				run.AssignColumn("B6", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 11))
				run.AssignColumn("B7", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 12))
				run.AssignColumn("B8", smartvectors.ForTest(14, 15, 16, 17, 18, 19, 110, 13))
			},

			Title:      "4 queries for one column, all on two columns",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 4),
					builder.RegisterCommit("A2", 4),
					builder.RegisterCommit("A3", 4),
					builder.RegisterCommit("A4", 4),
					builder.RegisterCommit("A5", 8),
					builder.RegisterCommit("A6", 8),
					builder.RegisterCommit("A7", 8),
					builder.RegisterCommit("A8", 8),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 4),
					builder.RegisterCommit("B2", 4),
					builder.RegisterCommit("B3", 4),
					builder.RegisterCommit("B4", 4),
					builder.RegisterCommit("B5", 8),
					builder.RegisterCommit("B6", 8),
					builder.RegisterCommit("B7", 8),
					builder.RegisterCommit("B8", 8),
				}
				// each fragment has its own permutation, but the permutation is the same for the columns from the same fragment.
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM1", [][]ifaces.Column{a[0:2], a[2:4]}, [][]ifaces.Column{b[0:2], b[2:4]})
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM2", [][]ifaces.Column{a[4:6], a[6:8]}, [][]ifaces.Column{b[4:6], b[6:8]})
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3))
				run.AssignColumn("A2", smartvectors.ForTest(1, 2, 3, 4))
				run.AssignColumn("A3", smartvectors.ForTest(2, 3, 4, 5))
				run.AssignColumn("A4", smartvectors.ForTest(3, 4, 5, 6))
				run.AssignColumn("A5", smartvectors.ForTest(10, 11, 12, 13, 14, 15, 16, 17))
				run.AssignColumn("A6", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18))
				run.AssignColumn("A7", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 19))
				run.AssignColumn("A8", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 110))

				run.AssignColumn("B1", smartvectors.ForTest(1, 2, 3, 0))
				run.AssignColumn("B2", smartvectors.ForTest(2, 3, 4, 1))
				run.AssignColumn("B3", smartvectors.ForTest(2, 4, 3, 5))
				run.AssignColumn("B4", smartvectors.ForTest(3, 5, 4, 6))
				run.AssignColumn("B5", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 10))
				run.AssignColumn("B6", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 11))
				run.AssignColumn("B7", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 12))
				run.AssignColumn("B8", smartvectors.ForTest(14, 15, 16, 17, 18, 19, 110, 13))
			},

			Title:      "2 fragmented multi-column queries using different sizes",
			ShouldPass: true,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := [][]ifaces.Column{
					{builder.RegisterCommit("A1", 4)},
					{builder.RegisterCommit("A2", 4)},
					{builder.RegisterCommit("A3", 4)},
					{builder.RegisterCommit("A4", 4)},
				}
				b := [][]ifaces.Column{
					{builder.RegisterCommit("B1", 16)},
				}
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM1", a, b)
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3))
				run.AssignColumn("A2", smartvectors.ForTest(4, 5, 6, 7))
				run.AssignColumn("A3", smartvectors.ForTest(8, 9, 10, 11))
				run.AssignColumn("A4", smartvectors.ForTest(12, 13, 14, 15))

				run.AssignColumn("B1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
			},

			Title:      "dissymetric fractional query",
			ShouldPass: true,
		},

		// test cases that should Not Pass
		{
			Define: func(builder *wizard.Builder) {
				a := builder.RegisterCommit("A", 16)
				b := builder.RegisterCommit("B", 16)
				builder.Permutation("PERM", []ifaces.Column{a}, []ifaces.Column{b})
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A", smartvectors.ForTest(0, 2, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
				run.AssignColumn("B", smartvectors.ForTest(15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0))
			},

			Title:      "single-column, should not pass",
			ShouldPass: false,
		},
		{
			Define: func(builder *wizard.Builder) {
				a := []ifaces.Column{
					builder.RegisterCommit("A1", 4),
					builder.RegisterCommit("A2", 4),
					builder.RegisterCommit("A3", 4),
					builder.RegisterCommit("A4", 4),
					builder.RegisterCommit("A5", 8),
					builder.RegisterCommit("A6", 8),
					builder.RegisterCommit("A7", 8),
					builder.RegisterCommit("A8", 8),
				}
				b := []ifaces.Column{
					builder.RegisterCommit("B1", 4),
					builder.RegisterCommit("B2", 4),
					builder.RegisterCommit("B3", 4),
					builder.RegisterCommit("B4", 4),
					builder.RegisterCommit("B5", 8),
					builder.RegisterCommit("B6", 8),
					builder.RegisterCommit("B7", 8),
					builder.RegisterCommit("B8", 8),
				}
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM1", [][]ifaces.Column{a[0:2], a[2:4]}, [][]ifaces.Column{b[0:2], b[2:4]})
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM2", [][]ifaces.Column{a[4:6], a[6:8]}, [][]ifaces.Column{b[4:6], b[6:8]})
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(0, 1, 2, 3))
				run.AssignColumn("A2", smartvectors.ForTest(2, 2, 3, 4))
				run.AssignColumn("A3", smartvectors.ForTest(2, 3, 4, 5))
				run.AssignColumn("A4", smartvectors.ForTest(3, 4, 5, 6))
				run.AssignColumn("A5", smartvectors.ForTest(10, 11, 12, 13, 14, 15, 16, 17))
				run.AssignColumn("A6", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18))
				run.AssignColumn("A7", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 19))
				run.AssignColumn("A8", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 110))

				run.AssignColumn("B1", smartvectors.ForTest(1, 2, 3, 0))
				run.AssignColumn("B2", smartvectors.ForTest(2, 3, 4, 1))
				run.AssignColumn("B3", smartvectors.ForTest(3, 4, 5, 2))
				run.AssignColumn("B4", smartvectors.ForTest(4, 5, 6, 3))
				run.AssignColumn("B5", smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 10))
				run.AssignColumn("B6", smartvectors.ForTest(12, 13, 14, 15, 16, 17, 18, 11))
				run.AssignColumn("B7", smartvectors.ForTest(13, 14, 15, 16, 17, 18, 19, 12))
				run.AssignColumn("B8", smartvectors.ForTest(14, 15, 16, 17, 18, 19, 110, 13))
			},

			Title:      "2 fragmented multi-column queries using different sizes, should not pass",
			ShouldPass: false,
		},

		{
			Define: func(builder *wizard.Builder) {
				a := [][]ifaces.Column{
					{builder.RegisterCommit("A1", 4)},
					{builder.RegisterCommit("A2", 4)},
					{builder.RegisterCommit("A3", 4)},
					{builder.RegisterCommit("A4", 4)},
				}
				b := [][]ifaces.Column{
					{builder.RegisterCommit("B1", 16)},
				}
				builder.CompiledIOP.InsertFragmentedPermutation(0, "PERM1", a, b)
			},

			Prove: func(run *wizard.ProverRuntime) {
				run.AssignColumn("A1", smartvectors.ForTest(1, 1, 2, 3))
				run.AssignColumn("A2", smartvectors.ForTest(4, 5, 6, 7))
				run.AssignColumn("A3", smartvectors.ForTest(8, 9, 10, 11))
				run.AssignColumn("A4", smartvectors.ForTest(12, 13, 14, 15))

				run.AssignColumn("B1", smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15))
			},

			Title:      "dissymetric fractional query, should not pass",
			ShouldPass: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Title, func(t *testing.T) {
			comp := wizard.Compile(testCase.Define, CompileViaGrandProduct, dummy.Compile)
			proof := wizard.Prove(comp, testCase.Prove)
			if err := wizard.Verify(comp, proof); err != nil && testCase.ShouldPass {
				t.Fatalf("verifier did not pass: %v", err.Error())
			}
			if err := wizard.Verify(comp, proof); err == nil && !testCase.ShouldPass {
				t.Fatalf("verifier is passing for a false claim")
			}
		})
	}
}
