package byte32cmp

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/stretchr/testify/require"
)

const max16 = 0xFFFF

type testCase struct {
	name        string
	aVals       [][]int
	bVals       [][]int
	maskVals    []int
	result      [][]int
	expectError bool
	isAddition  bool
}

func TestAddColToLimbs(t *testing.T) {
	tests := []testCase{
		{
			name: "with_carry",
			aVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, max16, 0},
				{0, max16, max16, 0},
				{max16, max16, max16, 0},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{1, 1, 1, 0},
			},
			maskVals:   []int{1, 1, 1, 0},
			isAddition: true,
		},
		// Edge cases (all single‐row, 4 limbs each)
		{
			name: "zero_plus_zero",
			aVals: [][]int{
				{0}, {0}, {0}, {0},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {0},
			},
			isAddition: true,
		},
		{
			name: "max_lsb_plus_one",
			aVals: [][]int{
				{0}, {0}, {0}, {max16},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {1},
			},
			isAddition: true,
		},
		{
			name: "cascade_carry",
			aVals: [][]int{
				{0}, {max16}, {max16}, {max16},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {1},
			},
			isAddition: true,
		},
		{
			name: "max_16bit_addition",
			aVals: [][]int{
				{0}, {0}, {0}, {0x8000},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {0x8000},
			},
			isAddition: true,
		},
		{
			name: "partial_carry",
			aVals: [][]int{
				{0}, {0}, {max16}, {0x8000},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {0x8000},
			},
			isAddition: true,
		},
		// multiple rows
		{
			name: "multi_row_4",
			aVals: [][]int{
				{0, 0x1234, 0, 0},
				{0, 0x5678, 0x5678, 0},
				{0, 0x9ABC, 0x9ABC, 0},
				{0, 0xDEF0, 0xDEF0, 1},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 1, 1, max16},
			},
			isAddition: true,
		},
		{
			name: "multi_row_2",
			aVals: [][]int{
				{0, 0x1234, 0, 0},
				{0, 0x5678, 0x5678, 1},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 1, 1, max16},
			},
			isAddition: true,
		},
		{
			name: "multi_row_8",
			aVals: [][]int{
				{0, 0x1234, 0, 0},
				{0, 0x5678, 0x5678, 0},
				{0, 0x9ABC, 0x9ABC, 0},
				{0, 0xDEF0, 0xDEF0, 0},
				{0, 0x1111, 0x1111, 0},
				{0, 0x2222, 0x2222, 0},
				{0, 0x3333, 0x3333, 0},
				{0, 0x4444, 0x4444, 1},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 1, 1, max16},
			},
			isAddition: true,
		},
		// single limb
		{
			name: "single_limb",
			aVals: [][]int{
				{100, 0xFFFE, 0, 0x7FFF},
			},
			bVals: [][]int{
				{50, 1, 0, 0x7FFF},
			},
			isAddition: true,
		},
		// overflow cases
		{
			name:        "overflow_not_allowed",
			aVals:       [][]int{{max16}},
			bVals:       [][]int{{1}},
			expectError: true,
			isAddition:  true,
		},
		{
			name:        "overflow_multilimb_not_allowed",
			aVals:       [][]int{{max16}, {max16}, {max16}, {max16}},
			bVals:       [][]int{{1}, {0}, {0}, {0}},
			expectError: true,
			isAddition:  true,
		},
		// with precomputed result
		{
			name: "with_precomputed_result",
			aVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, max16, 0},
				{0, max16, max16, 0},
				{max16, max16, max16, 0},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{1, 1, 1, 0},
			},
			maskVals: []int{1, 1, 1, 0},
			result: [][]int{
				{0, 0, 1, 0},
				{0, 1, 0, 0},
				{1, 0, 0, 0},
				{0, 0, 0, 0},
			},
			isAddition: true,
		},
		// multi-limb b operands
		{
			name: "multi_limb_b_operand",
			aVals: [][]int{
				{0}, {0}, {0}, {0x4000},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {0x4000},
			},
			isAddition: true,
		},
		{
			name: "multi_limb_b_with_carry",
			aVals: [][]int{
				{0}, {0}, {0x8000}, {0x8000},
			},
			bVals: [][]int{
				{0}, {0}, {0x8000}, {0x8000},
			},
			isAddition: true,
		},
		{
			name: "different_limb_values",
			aVals: [][]int{
				{0x1111}, {0x2222}, {0x3333}, {0x4444},
			},
			bVals: [][]int{
				{0x5555}, {0x6666}, {0x7777}, {0x8888},
			},
			isAddition: true,
		},
		{
			name: "multi_row_different_b_values",
			aVals: [][]int{
				{0x1000, 0x2000, 0x3000, 0x0},
				{0x4000, 0x5000, 0x6000, 0x0},
				{0x7000, 0x8000, 0x9000, 0x0},
				{0xA000, 0xB000, 0xC000, 0x0},
			},
			bVals: [][]int{
				{0x0100, 0x0200, 0x0300, 0x0},
				{0x0400, 0x0500, 0x0600, 0x0},
				{0x0700, 0x0800, 0x0900, 0x0},
				{0x0A00, 0x0B00, 0x0C00, 0x0},
			},
			isAddition: true,
		},
		{
			name: "sub_with_carry",
			aVals: [][]int{
				{0}, {0}, {1}, {1},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {2},
			},
			isAddition: false,
		},
		{
			name: "sub_with_carry_cascade",
			aVals: [][]int{
				{1}, {0}, {0}, {0},
			},
			bVals: [][]int{
				{0}, {0}, {0}, {1},
			},
			isAddition: false,
		},
		{
			name:        "sub_overflow_not_allowed",
			aVals:       [][]int{{0}},
			bVals:       [][]int{{1}},
			expectError: true,
			isAddition:  false,
		},
		{
			name:        "sub_overflow_not_allowed",
			aVals:       [][]int{{5, 5, 5, 5}},
			bVals:       [][]int{{9, 9, 9, 9}},
			expectError: true,
			isAddition:  false,
		},
		{
			name: "sub_with_precomputed_result",
			aVals: [][]int{
				{0, 0, 1, 0},
				{0, 1, 0, 0},
				{1, 0, 0, 0},
				{0, 0, 0, 0},
			},
			bVals: [][]int{
				{0, 0, 0, 0},
				{0, 0, max16, 0},
				{0, max16, max16, 0},
				{max16, max16, max16, 0},
			},
			maskVals: []int{1, 1, 1, 0},
			result: [][]int{
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{0, 0, 0, 0},
				{1, 1, 1, 0},
			},
			isAddition: false,
		},
		{
			name: "sub_multi_limb_b_with_carry",
			aVals: [][]int{
				{32769}, {32768}, {32768}, {32768},
			},
			bVals: [][]int{
				{32768}, {32769}, {32769}, {32769},
			},
			isAddition: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testAddColToLimbs(t, tc)
		})
	}
}

func testAddColToLimbs(t *testing.T, tc testCase) {
	var pa wizard.ProverAction

	define := func(builder *wizard.Builder) {
		comp := builder.CompiledIOP

		numRows := len(tc.aVals[0])

		aLimbs := LimbColumns{
			Limbs:       make([]ifaces.Column, len(tc.aVals)),
			LimbBitSize: 16,
			IsBigEndian: true,
		}

		bLimbs := LimbColumns{
			Limbs:       make([]ifaces.Column, len(tc.aVals)),
			LimbBitSize: 16,
			IsBigEndian: true,
		}

		for i := range tc.aVals {
			aLimbs.Limbs[i] = comp.InsertCommit(0, ifaces.ColIDf("A%d", i), numRows, true)
			bLimbs.Limbs[i] = comp.InsertCommit(0, ifaces.ColIDf("B%d", i), numRows, true)
		}

		var maskCol ifaces.Column
		if tc.maskVals != nil {
			maskCol = comp.InsertCommit(0, "MASK", numRows, true)
		} else {
			maskCol = comp.InsertPrecomputed("MASK", smartvectors.NewConstant(field.One(), numRows))
		}

		var result LimbColumns
		if tc.result != nil {
			result = LimbColumns{
				Limbs:       make([]ifaces.Column, len(tc.result)),
				LimbBitSize: 16,
				IsBigEndian: true,
			}

			for i := range tc.aVals {
				result.Limbs[i] = comp.InsertCommit(0, ifaces.ColIDf("R%d", i), numRows, true)
			}
		}

		_, pa = NewMultiLimbAdd(comp, &MultiLimbAddIn{
			Name:   tc.name,
			ALimbs: aLimbs,
			BLimbs: bLimbs,
			Mask:   symbolic.NewVariable(maskCol),
			Result: result,
		}, tc.isAddition)
	}

	prover := func(run *wizard.ProverRuntime) {
		for i := range tc.aVals {
			run.AssignColumn(ifaces.ColIDf("A%d", i), smartvectors.ForTest(tc.aVals[i]...))
			run.AssignColumn(ifaces.ColIDf("B%d", i), smartvectors.ForTest(tc.bVals[i]...))
		}

		if tc.maskVals != nil {
			run.AssignColumn("MASK", smartvectors.ForTest(tc.maskVals...))
		}

		if tc.result != nil {
			for i, vals := range tc.result {
				run.AssignColumn(ifaces.ColIDf("R%d", i), smartvectors.ForTest(vals...))
			}
		}

		pa.Run(run)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)

	if tc.expectError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}
}
