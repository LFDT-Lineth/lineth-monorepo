package dedicated_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManuallyShiftZeroPadding verifies that Assign produces zero-padded
// results, not cyclic rotations. This is critical for the distributed system's
// segment boundary detection: cyclic rotation places non-zero values at
// boundaries, corrupting the padding analysis.
func TestManuallyShiftZeroPadding(t *testing.T) {

	testCases := []struct {
		name     string
		data     []int
		offset   int
		expected []int // expected assignment values
	}{
		{
			name:     "negative offset zero-pads at front",
			data:     []int{10, 20, 30, 40},
			offset:   -1,
			expected: []int{0, 10, 20, 30}, // prepend zero, drop last
		},
		{
			name:     "negative offset -2 zero-pads two at front",
			data:     []int{10, 20, 30, 40},
			offset:   -2,
			expected: []int{0, 0, 10, 20}, // prepend two zeros, drop last two
		},
		{
			name:     "positive offset zero-pads at end",
			data:     []int{10, 20, 30, 40},
			offset:   1,
			expected: []int{20, 30, 40, 0}, // drop first, append zero
		},
		{
			name:     "positive offset +2 zero-pads two at end",
			data:     []int{10, 20, 30, 40},
			offset:   2,
			expected: []int{30, 40, 0, 0}, // drop first two, append two zeros
		},
		{
			name:     "zero offset is identity",
			data:     []int{10, 20, 30, 40},
			offset:   0,
			expected: []int{10, 20, 30, 40},
		},
		{
			name:     "negative offset with all nonzero data",
			data:     []int{1, 2, 3, 4, 5, 6, 7, 8},
			offset:   -1,
			expected: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			name:     "positive offset with all nonzero data",
			data:     []int{1, 2, 3, 4, 5, 6, 7, 8},
			offset:   1,
			expected: []int{2, 3, 4, 5, 6, 7, 8, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ms *dedicated.ManuallyShifted
			var captured smartvectors.SmartVector

			define := func(b *wizard.Builder) {
				root := b.CompiledIOP.InsertCommit(0, "ROOT", len(tc.data), true)
				ms = dedicated.ManuallyShift(b.CompiledIOP, root, tc.offset, "TEST_COL")
			}

			comp := wizard.Compile(define)

			wizard.Prove(comp, func(run *wizard.ProverRuntime) {
				run.AssignColumn("ROOT", smartvectors.ForTest(tc.data...))
				ms.Assign(run)
				captured = ms.Natural.GetColAssignment(run)
			})

			// Verify the assigned values match expected zero-padded result
			require.NotNil(t, captured, "column assignment should have been captured")
			for i, exp := range tc.expected {
				got := captured.Get(i)
				var expField field.Element
				expField.SetInt64(int64(exp))
				assert.Equalf(t, expField.String(), got.String(),
					"mismatch at position %d: expected %d, got %s", i, exp, got.String())
			}
		})
	}
}

// TestManuallyShiftNamingCollision verifies that ManuallyShift produces unique
// column IDs when the same base column is shifted by different offsets.
// Previously, only the name was used (without offset), causing collisions.
func TestManuallyShiftNamingCollision(t *testing.T) {

	var ms1, ms2, ms3 *dedicated.ManuallyShifted

	define := func(b *wizard.Builder) {
		root := b.CompiledIOP.InsertCommit(0, "ROOT", 16, true)
		// Same name prefix, different offsets — must produce different column IDs
		ms1 = dedicated.ManuallyShift(b.CompiledIOP, root, -1, "SHIFTED_ROOT")
		ms2 = dedicated.ManuallyShift(b.CompiledIOP, root, 1, "SHIFTED_ROOT")
		ms3 = dedicated.ManuallyShift(b.CompiledIOP, root, -2, "SHIFTED_ROOT")
	}

	comp := wizard.Compile(define)

	// All three should have distinct column IDs
	id1 := ms1.Natural.ID
	id2 := ms2.Natural.ID
	id3 := ms3.Natural.ID

	assert.NotEqual(t, id1, id2, "offset -1 and +1 should have different column IDs")
	assert.NotEqual(t, id1, id3, "offset -1 and -2 should have different column IDs")
	assert.NotEqual(t, id2, id3, "offset +1 and -2 should have different column IDs")

	// Verify all columns exist in the compiled IOP
	assert.True(t, comp.Columns.Exists(id1), "column %v should exist", id1)
	assert.True(t, comp.Columns.Exists(id2), "column %v should exist", id2)
	assert.True(t, comp.Columns.Exists(id3), "column %v should exist", id3)
}

// TestManuallyShiftDummyVerifierPasses verifies that the ManuallyShift
// constraint (Natural - Shift(root, offset) = 0) passes the dummy verifier
// with the zero-padding assignment. The global constraint check skips boundary
// rows automatically.
func TestManuallyShiftDummyVerifierPasses(t *testing.T) {

	testCases := []struct {
		name   string
		size   int
		offset int
	}{
		{"size4_offset-1", 4, -1},
		{"size4_offset+1", 4, 1},
		{"size4_offset-2", 4, -2},
		{"size4_offset+2", 4, 2},
		{"size16_offset-1", 16, -1},
		{"size16_offset+1", 16, 1},
		{"size16_offset-3", 16, -3},
		{"size16_offset+3", 16, 3},
		{"size64_offset-1", 64, -1},
		{"size64_offset+1", 64, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ms *dedicated.ManuallyShifted

			// Build test data with all nonzero values to stress boundaries
			data := make([]int, tc.size)
			for i := range data {
				data[i] = i + 1
			}

			define := func(b *wizard.Builder) {
				root := b.CompiledIOP.InsertCommit(0, "ROOT", tc.size, true)
				ms = dedicated.ManuallyShift(b.CompiledIOP, root, tc.offset, "")
			}

			comp := wizard.Compile(define, dummy.Compile)

			proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
				run.AssignColumn("ROOT", smartvectors.ForTest(data...))
				ms.Assign(run)
			})

			err := wizard.Verify(comp, proof)
			require.NoError(t, err, "dummy verifier should pass for offset=%d size=%d", tc.offset, tc.size)
		})
	}
}

// TestManuallyShiftMultipleOffsetsOnSameRoot verifies that creating multiple
// ManuallyShifted columns from the same root with different offsets all pass
// the dummy verifier. This is the scenario that caused the original naming
// collision bug.
func TestManuallyShiftMultipleOffsetsOnSameRoot(t *testing.T) {

	var shifts []*dedicated.ManuallyShifted

	data := make([]int, 16)
	for i := range data {
		data[i] = i + 100
	}

	define := func(b *wizard.Builder) {
		root := b.CompiledIOP.InsertCommit(0, "ROOT", 16, true)

		offsets := []int{-1, 1, -2, 2, -3, 3}
		for _, off := range offsets {
			ms := dedicated.ManuallyShift(b.CompiledIOP, root, off, "MULTI_SHIFT")
			shifts = append(shifts, ms)
		}
	}

	comp := wizard.Compile(define, dummy.Compile)

	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
		run.AssignColumn("ROOT", smartvectors.ForTest(data...))
		for _, ms := range shifts {
			ms.Assign(run)
		}
	})

	err := wizard.Verify(comp, proof)
	require.NoError(t, err, "dummy verifier should pass for multiple offsets on same root")
}

// TestManuallyShiftAlreadyExistsDedup verifies that ManuallyShift handles
// duplicate name collision gracefully by appending a unique suffix.
func TestManuallyShiftAlreadyExistsDedup(t *testing.T) {

	var ms1, ms2 *dedicated.ManuallyShifted

	define := func(b *wizard.Builder) {
		root := b.CompiledIOP.InsertCommit(0, "ROOT", 8, true)
		// Same name, same offset — the second call should detect collision and dedup
		ms1 = dedicated.ManuallyShift(b.CompiledIOP, root, -1, "SAME_NAME")
		ms2 = dedicated.ManuallyShift(b.CompiledIOP, root, -1, "SAME_NAME")
	}

	comp := wizard.Compile(define, dummy.Compile)

	assert.NotEqual(t, ms1.Natural.ID, ms2.Natural.ID,
		"two ManuallyShift calls with the same name+offset should produce different IDs via dedup")

	assert.True(t, comp.Columns.Exists(ms1.Natural.ID))
	assert.True(t, comp.Columns.Exists(ms2.Natural.ID))

	// Both should pass the dummy verifier
	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
		run.AssignColumn("ROOT", smartvectors.ForTest(1, 2, 3, 4, 5, 6, 7, 8))
		ms1.Assign(run)
		ms2.Assign(run)
	})

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)
}

// TestManuallyShiftLargeOffset tests edge cases where offset is close to
// the column size.
func TestManuallyShiftLargeOffset(t *testing.T) {

	testCases := []struct {
		name   string
		size   int
		offset int
	}{
		{"offset_near_size_negative", 8, -7},
		{"offset_near_size_positive", 8, 7},
		{"offset_half_size_negative", 16, -8},
		{"offset_half_size_positive", 16, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ms *dedicated.ManuallyShifted

			data := make([]int, tc.size)
			for i := range data {
				data[i] = i + 1
			}

			define := func(b *wizard.Builder) {
				root := b.CompiledIOP.InsertCommit(0, "ROOT", tc.size, true)
				ms = dedicated.ManuallyShift(b.CompiledIOP, root, tc.offset, "")
			}

			comp := wizard.Compile(define, dummy.Compile)

			proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
				run.AssignColumn("ROOT", smartvectors.ForTest(data...))
				ms.Assign(run)
			})

			err := wizard.Verify(comp, proof)
			require.NoError(t, err)
		})
	}
}
