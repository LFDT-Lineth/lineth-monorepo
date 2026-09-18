package logderivativesum_test

import (
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
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

// TestBuildZ_ShiftedFractionMustNotCancelEndpointRow pins the invariant that
// the running-sum recurrence cancels row 0 and nothing else, regardless of what
// shifts appear inside the caller's fraction expressions.
//
// The recurrence is zNum − (Z − Z<<−1)·zDen, and zNum/zDen embed the fraction
// numerators/denominators verbatim. Registering it with Module.NewVanishing
// derives the cancelled rows from *every* shift in the tree, so a numerator
// read as col.View().Shift(+1) cancels the last row on top of row 0.
//
// The last row is exactly the row opened as the Z endpoint and summed into the
// LogDerivativeSum Result. Cancelling it severs Z[n−1] from Z[n−2], letting a
// malicious prover choose the endpoint freely and pass the final-sum check for
// an arbitrary witness. This path is production-reachable: zkcdriver applies
// ColumnAccess.RelativeShift to lookup source/target views, so any corset
// lookup over a shifted register lands here.
//
// Only row 0 may ever be cancelled -- shifts on fraction columns are cyclic
// (tableElemAt wraps mod n), so the recurrence is well-defined on every row
// but the first.
func TestBuildZ_ShiftedFractionMustNotCancelEndpointRow(t *testing.T) {
	sys := wiop.NewSystemf("lds-shifted-fraction")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	num := mod.NewColumn(sys.Context.Childf("num"), r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	sys.NewLogDerivativeSum(
		sys.Context.Childf("ld"),
		// Numerator read with a +1 shift, mirroring what zkcdriver produces for
		// a lookup over a shifted register.
		[]wiop.Fraction{{Numerator: num.View().Shift(1), Denominator: one}},
	)

	logderivativesum.Compile(sys)

	cancellations := zRecurrenceCancellations(sys)
	require.NotEmpty(t, cancellations,
		"compile must register at least one z-recurrence Vanishing")

	for name, positions := range cancellations {
		assert.Equal(t, []int{0}, positions,
			"%s: z-recurrence must cancel row 0 only; cancelling a negative "+
				"position un-constrains the Z endpoint row and breaks soundness "+
				"of the log-derivative argument", name)
	}
}
