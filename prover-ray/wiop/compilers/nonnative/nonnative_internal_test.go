package nonnative

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestCarryRowExtremesFitCarryRange checks that the carries of the most
// positive (left*right saturated) and most negative (modulus*quotient+result
// saturated) digit assignments fit the committed carry window: a too-narrow
// window would make honest proofs fail its range check.
func TestCarryRowExtremesFitCarryRange(t *testing.T) {
	for _, tc := range []struct{ limbBits, nbLimbs int }{
		{16, 1}, {16, 4}, {16, 8}, {16, 16}, {16, 32}, {8, 2}, {12, 24}, {27, 3},
	} {
		t.Run(fmt.Sprintf("limbBits=%d/limbs=%d", tc.limbBits, tc.nbLimbs), func(t *testing.T) {
			l := newLayout(&wiop.NonNative{
				NbBitsPerLimb: tc.limbBits,
				Left:          make([]*wiop.Column, tc.nbLimbs),
			})
			maxDigit := uint64(1)<<l.digitBits - 1
			bias := int64(l.carryBias())
			for _, saturated := range [][]int{{opLeft, opRight}, {opModulus, opQuotient, opResult}} {
				var digits [nbOperands][]uint64
				for op := range digits {
					digits[op] = make([]uint64, l.nbDigits)
				}
				for _, op := range saturated {
					for j := range digits[op] {
						digits[op][j] = maxDigit
					}
				}
				carries := make([]int64, max(2*l.nbDigits-2, 0))
				carryRow(digits, uint(l.digitBits), newCarryScratch(l.nbDigits), carries)
				for k, c := range carries {
					require.GreaterOrEqual(t, c, -bias, "carry %d below the window", k)
					require.Less(t, c, bias, "carry %d above the window", k)
				}
			}
		})
	}
}
