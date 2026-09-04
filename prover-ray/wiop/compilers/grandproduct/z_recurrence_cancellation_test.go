package grandproduct_test

import (
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zRecurrenceCancellations returns, for every Vanishing whose context name
// contains "z-recurrence", the constraint name and its CancelledPositions.
func zRecurrenceCancellations(sys *wiop.System) map[string][]int {
	out := make(map[string][]int)
	for _, m := range sys.Modules {
		for _, v := range m.Vanishings {
			name := v.Context().String()
			if strings.Contains(name, "z-recurrence") {
				out[name] = v.CancelledPositions
			}
		}
	}
	return out
}

// TestBuildZ_ShiftedFactorMustNotCancelEndpointRow pins the invariant that the
// running-product recurrence cancels row 0 and nothing else, regardless of what
// shifts appear inside the caller's factor expressions.
//
// The recurrence is Z·zDen − Z<<−1·zNum, and zNum/zDen embed the permutation's
// factor expressions verbatim. Registering it with Module.NewVanishing derives
// the cancelled rows from *every* shift in the tree, so a factor column read as
// colA.View().Shift(+1) cancels the last row on top of row 0.
//
// The last row is exactly the row opened as ZFinal and fed to
// FinalProductCheck. Cancelling it severs Z[n−1] from Z[n−2]: a malicious
// prover picks the endpoint freely so the product reads as one, and
// CheckResultIsOne accepts an arbitrary non-permuted multiset. Only row 0 may
// ever be cancelled here -- factor-column shifts are cyclic (tableElemAt wraps
// mod n), so the recurrence is well-defined on every row but the first.
func TestBuildZ_ShiftedFactorMustNotCancelEndpointRow(t *testing.T) {
	sys := wiop.NewSystemf("gp-shifted-factor")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View().Shift(1))}, // A side read with a +1 shift
		[]wiop.Table{wiop.NewTable(colB.View())},
	)

	grandproduct.Compile(sys)

	cancellations := zRecurrenceCancellations(sys)
	require.NotEmpty(t, cancellations,
		"compile must register at least one z-recurrence Vanishing")

	for name, positions := range cancellations {
		assert.Equal(t, []int{0}, positions,
			"%s: z-recurrence must cancel row 0 only; cancelling a negative "+
				"position un-constrains the ZFinal endpoint row and breaks "+
				"soundness of the permutation argument", name)
	}
}
