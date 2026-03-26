package utilities

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/require"
	"testing"
)

type testCase struct {
	name        string
	aVals       []uint64
	b           uint64
	expected    []uint64
	expectPanic bool
}

func TestMultiLimbAdd16Bit(t *testing.T) {
	tests := []testCase{
		{
			name:        "empty_slice",
			aVals:       []uint64{},
			b:           1,
			expectPanic: true,
		},
		{
			name:     "single_limb_no_carry",
			aVals:    []uint64{100},
			b:        50,
			expected: []uint64{150},
		},
		{
			name:     "single_limb_with_carry_no_overflow",
			aVals:    []uint64{0xFFFE},
			b:        1,
			expected: []uint64{0xFFFF},
		},
		{
			name:        "single_limb_overflow",
			aVals:       []uint64{0xFFFF},
			b:           1,
			expectPanic: true,
		},
		{
			name:     "two_limbs_no_carry",
			aVals:    []uint64{1, 2},
			b:        3,
			expected: []uint64{1, 5},
		},
		{
			name:     "two_limbs_with_carry",
			aVals:    []uint64{1, 0xFFFF},
			b:        1,
			expected: []uint64{2, 0},
		},
		{
			name:     "cascade_carry",
			aVals:    []uint64{0, 0xFFFF, 0xFFFF},
			b:        1,
			expected: []uint64{1, 0, 0},
		},
		{
			name:        "overflow_multi_limb",
			aVals:       []uint64{0xFFFF, 0xFFFF},
			b:           1,
			expectPanic: true,
		},
		{
			name:        "initial_limb_exceeds_mask",
			aVals:       []uint64{0, 0x10000},
			b:           0,
			expectPanic: true,
		},
		{
			name:        "single_limb_large_b_overflow",
			aVals:       []uint64{0},
			b:           0x1_0000,
			expectPanic: true,
		},
		{
			name:     "two_limbs_large_b_no_overflow",
			aVals:    []uint64{1, 2},
			b:        0x1_0001,
			expected: []uint64{2, 3},
		},
		{
			name:        "two_limbs_large_b_overflow",
			aVals:       []uint64{0, 0},
			b:           1 << 32,
			expectPanic: true,
		},
		{
			name:     "three_limbs_large_b_no_overflow",
			aVals:    []uint64{1, 2, 3},
			b:        0x1_0001_0002,
			expected: []uint64{2, 3, 5},
		},
		{
			name:        "three_limbs_large_b_overflow",
			aVals:       []uint64{0xFFFF, 0xFFFF, 0xFFFF},
			b:           1 << 48,
			expectPanic: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limbs := make([]field.Element, len(tc.aVals))
			for i, v := range tc.aVals {
				limbs[i].SetUint64(v)
			}

			if tc.expectPanic {
				require.Panics(t, func() {
					Multi16bitLimbAdd(limbs, tc.b)
				})
				return
			}

			var res []field.Element
			require.NotPanics(t, func() {
				res = Multi16bitLimbAdd(limbs, tc.b)
			})

			for i, want := range tc.expected {
				got := res[i].Uint64()
				require.Equal(t, want, got, "limb %d mismatch", i)
			}
		})
	}
}
