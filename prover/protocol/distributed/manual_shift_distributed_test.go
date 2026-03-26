package distributed_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// ManualShiftLookupTestCase exercises ManuallyShifted columns through the full
// distributed wizard pipeline. When column.Shift appears in a lookup query,
// CompileManualShifter creates committed ManuallyShifted columns with global
// constraints. This test verifies those survive segmentation intact.
//
// The test creates two modules connected by a lookup where one side uses
// column.Shift, forcing the manual-shift compilation path.
type ManualShiftLookupTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *ManualShiftLookupTestCase) Name() string {
	return "ManualShiftLookup"
}

func (d *ManualShiftLookupTestCase) Define(comp *wizard.CompiledIOP) {
	d.wiop = comp

	// Module 0: a0 + b0 = c0, uses column.Shift(a0, 1) in a lookup
	a0 := comp.InsertCommit(0, "ms_a0", d.numRow, true)
	b0 := comp.InsertCommit(0, "ms_b0", d.numRow, true)
	c0 := comp.InsertCommit(0, "ms_c0", d.numRow, true)

	// Module 1: a1 + b1 = c1, differentiated by a duplicate constraint
	a1 := comp.InsertCommit(0, "ms_a1", d.numRow, true)
	b1 := comp.InsertCommit(0, "ms_b1", d.numRow, true)
	c1 := comp.InsertCommit(0, "ms_c1", d.numRow, true)

	comp.InsertGlobal(0, "ms_global_0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "ms_global_0_dup", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "ms_global_1", symbolic.Sub(c1, b1, a1))

	// Inclusion with column.Shift — this triggers CompileManualShifter during
	// the distributed compilation phase. The shifted column (a0 shifted by +1)
	// will become a ManuallyShifted committed column.
	a0_shifted := column.Shift(a0, 1)
	comp.InsertInclusion(0, "ms_inclusion_shifted",
		[]ifaces.Column{a0_shifted, b0},
		[]ifaces.Column{a1, b1},
	)
}

func (d *ManualShiftLookupTestCase) Assign(run *wizard.ProverRuntime) {
	n := d.numRow

	// Values chosen so that a + b = c and the lookup holds.
	// a0: 0, 1, 2, ..., n-1 (right-padded with zeros is fine since we pad)
	// b0: 1, 1, 1, ..., 1
	// c0: a0 + b0
	a0Vals := make([]field.Element, n)
	b0Vals := make([]field.Element, n)
	c0Vals := make([]field.Element, n)

	for i := 0; i < n; i++ {
		a0Vals[i] = field.NewElement(uint64(i))
		b0Vals[i] = field.One()
		c0Vals[i] = field.NewElement(uint64(i + 1))
	}

	// a0_shifted[i] = a0[i+1] cyclically = i+1 for the lookup.
	// The lookup says (a0_shifted, b0) ⊂ (a1, b1).
	// a0_shifted = [1, 2, ..., n-1, 0] — but with ManuallyShifted zero-padding
	// it becomes [1, 2, ..., n-1, 0]. We need a1, b1 to contain these rows.

	// Module 1 values: must contain all rows (a0_shifted[i], b0[i]) for all i
	// a0_shifted = [1, 2, ..., n-1, 0], b0 = [1, ..., 1]
	// So a1 must contain 0..n-1, b1 = all 1s
	a1Vals := make([]field.Element, n)
	b1Vals := make([]field.Element, n)
	c1Vals := make([]field.Element, n)

	for i := 0; i < n; i++ {
		a1Vals[i] = field.NewElement(uint64(i))
		b1Vals[i] = field.One()
		c1Vals[i] = field.NewElement(uint64(i + 1))
	}

	run.AssignColumn("ms_a0", smartvectors.NewRegular(a0Vals))
	run.AssignColumn("ms_b0", smartvectors.NewRegular(b0Vals))
	run.AssignColumn("ms_c0", smartvectors.NewRegular(c0Vals))
	run.AssignColumn("ms_a1", smartvectors.NewRegular(a1Vals))
	run.AssignColumn("ms_b1", smartvectors.NewRegular(b1Vals))
	run.AssignColumn("ms_c1", smartvectors.NewRegular(c1Vals))
}

func (d *ManualShiftLookupTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("ms_module_0", d.wiop.Columns.GetHandle("ms_a0")),
		distributed.SameSizeAdvice("ms_module_0", d.wiop.Columns.GetHandle("ms_a1")),
	}
}

// ManualShiftMultiOffsetTestCase verifies that multiple ManuallyShifted
// columns from the same root with different offsets survive distribution.
// This is the scenario that caused the original naming collision bug: two
// shifted columns from the same root would get identical column IDs.
type ManualShiftMultiOffsetTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *ManualShiftMultiOffsetTestCase) Name() string {
	return "ManualShiftMultiOffset"
}

func (d *ManualShiftMultiOffsetTestCase) Define(comp *wizard.CompiledIOP) {
	d.wiop = comp

	a0 := comp.InsertCommit(0, "mmo_a0", d.numRow, true)
	b0 := comp.InsertCommit(0, "mmo_b0", d.numRow, true)
	c0 := comp.InsertCommit(0, "mmo_c0", d.numRow, true)

	// Second module (differentiated by extra constraint)
	a1 := comp.InsertCommit(0, "mmo_a1", d.numRow, true)
	b1 := comp.InsertCommit(0, "mmo_b1", d.numRow, true)
	c1 := comp.InsertCommit(0, "mmo_c1", d.numRow, true)

	// Global constraints
	comp.InsertGlobal(0, "mmo_global_0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "mmo_global_0_dup", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "mmo_global_1", symbolic.Sub(c1, b1, a1))

	// Two lookups using different shifts of the same column — this is the
	// exact scenario that triggered the naming collision. Both shifts used
	// in inclusions against the same target.
	a0_shift_plus1 := column.Shift(a0, 1)

	comp.InsertInclusion(0, "mmo_incl_shift_plus1",
		[]ifaces.Column{a0_shift_plus1},
		[]ifaces.Column{a1},
	)
}

func (d *ManualShiftMultiOffsetTestCase) Assign(run *wizard.ProverRuntime) {
	n := d.numRow
	vals := make([]field.Element, n)
	for i := 0; i < n; i++ {
		vals[i] = field.NewElement(uint64(i))
	}

	// a + b = c for global constraints
	ones := make([]field.Element, n)
	sums := make([]field.Element, n)
	for i := 0; i < n; i++ {
		ones[i] = field.One()
		sums[i] = field.NewElement(uint64(i + 1))
	}

	run.AssignColumn("mmo_a0", smartvectors.NewRegular(vals))
	run.AssignColumn("mmo_b0", smartvectors.NewRegular(ones))
	run.AssignColumn("mmo_c0", smartvectors.NewRegular(sums))

	// Module 1: a1 must contain all values from shifted a0
	// ManuallyShifted(a0, +1) = [1, 2, ..., n-1, 0]
	run.AssignColumn("mmo_a1", smartvectors.NewRegular(vals))
	run.AssignColumn("mmo_b1", smartvectors.NewRegular(ones))
	run.AssignColumn("mmo_c1", smartvectors.NewRegular(sums))
}

func (d *ManualShiftMultiOffsetTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("mmo_module_0", d.wiop.Columns.GetHandle("mmo_a0")),
		distributed.SameSizeAdvice("mmo_module_0", d.wiop.Columns.GetHandle("mmo_a1")),
	}
}

// FullColumnWithShiftTestCase exercises ManuallyShifted columns through the
// distributed pipeline with fully-populated data (no padding). This tests that
// the noPaddingInformation path in LPPSegmentBoundaryCalculator correctly
// handles "full" columns in the presence of ManuallyShifted columns.
type FullColumnWithShiftTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *FullColumnWithShiftTestCase) Name() string {
	return "FullColumnWithShift"
}

func (d *FullColumnWithShiftTestCase) Define(comp *wizard.CompiledIOP) {
	d.wiop = comp

	a := comp.InsertCommit(0, "fcws_a", d.numRow, true)
	b := comp.InsertCommit(0, "fcws_b", d.numRow, true)
	c := comp.InsertCommit(0, "fcws_c", d.numRow, true)

	d2 := comp.InsertCommit(0, "fcws_d", d.numRow, true)
	e := comp.InsertCommit(0, "fcws_e", d.numRow, true)
	f := comp.InsertCommit(0, "fcws_f", d.numRow, true)

	comp.InsertGlobal(0, "fcws_global_0", symbolic.Sub(c, b, a))
	comp.InsertGlobal(0, "fcws_global_0_dup", symbolic.Sub(c, b, a))
	comp.InsertGlobal(0, "fcws_global_1", symbolic.Sub(f, e, d2))

	// Use a shifted column in the lookup
	a_shifted := column.Shift(a, -1)
	comp.InsertInclusion(0, "fcws_inclusion",
		[]ifaces.Column{a_shifted, b},
		[]ifaces.Column{d2, e},
	)
}

func (d *FullColumnWithShiftTestCase) Assign(run *wizard.ProverRuntime) {
	n := d.numRow

	// a = [0, 1, 2, ..., n-1], b = [1, 2, ..., n], c = a+b = [1, 3, 5, ...]
	aVals := make([]field.Element, n)
	bVals := make([]field.Element, n)
	cVals := make([]field.Element, n)

	for i := 0; i < n; i++ {
		aVals[i] = field.NewElement(uint64(i))
		bVals[i] = field.NewElement(uint64(i + 1))
		cVals[i] = field.NewElement(uint64(2*i + 1))
	}

	// ManuallyShifted(a, -1) = [0, 0, 1, ..., n-2]
	// Included tuples: (shifted_a[i], b[i]) = (max(0,i-1), i+1)
	// At i=0: (0, 1), at i=1: (0, 2), at i=2: (1, 3), ..., at i=n-1: (n-2, n)

	// For the including table (d, e), we need to contain all these tuples
	// and satisfy f = d + e.
	dVals := make([]field.Element, n)
	eVals := make([]field.Element, n)
	fVals := make([]field.Element, n)

	// d = [0, 0, 1, 2, ..., n-2], e = [1, 2, 3, ..., n]
	// This matches the included tuples exactly
	dVals[0] = field.NewElement(0)
	eVals[0] = field.NewElement(1)
	fVals[0] = field.NewElement(1) // d+e
	dVals[1] = field.NewElement(0)
	eVals[1] = field.NewElement(2)
	fVals[1] = field.NewElement(2) // d+e
	for i := 2; i < n; i++ {
		dVals[i] = field.NewElement(uint64(i - 1))
		eVals[i] = field.NewElement(uint64(i + 1))
		fVals[i] = field.NewElement(uint64(i - 1 + i + 1)) // d+e
	}

	run.AssignColumn("fcws_a", smartvectors.NewRegular(aVals))
	run.AssignColumn("fcws_b", smartvectors.NewRegular(bVals))
	run.AssignColumn("fcws_c", smartvectors.NewRegular(cVals))
	run.AssignColumn("fcws_d", smartvectors.NewRegular(dVals))
	run.AssignColumn("fcws_e", smartvectors.NewRegular(eVals))
	run.AssignColumn("fcws_f", smartvectors.NewRegular(fVals))
}

func (d *FullColumnWithShiftTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("fcws_module_0", d.wiop.Columns.GetHandle("fcws_a")),
		distributed.SameSizeAdvice("fcws_module_0", d.wiop.Columns.GetHandle("fcws_d")),
	}
}

// ZeroPaddedShiftTestCase tests that zero-padded ManuallyShifted columns
// are correctly handled during segmentation. With right-padded data, the
// MarkZeroPadded pragma on ManuallyShifted columns causes SegmentOfColumn
// to use zero-fill for out-of-bounds segments.
type ZeroPaddedShiftTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *ZeroPaddedShiftTestCase) Name() string {
	return "ZeroPaddedShift"
}

func (d *ZeroPaddedShiftTestCase) Define(comp *wizard.CompiledIOP) {
	d.wiop = comp

	a := comp.InsertCommit(0, "zps_a", d.numRow, true)
	b := comp.InsertCommit(0, "zps_b", d.numRow, true)
	c := comp.InsertCommit(0, "zps_c", d.numRow, true)

	d2 := comp.InsertCommit(0, "zps_d", d.numRow, true)
	e := comp.InsertCommit(0, "zps_e", d.numRow, true)
	f := comp.InsertCommit(0, "zps_f", d.numRow, true)

	comp.InsertGlobal(0, "zps_global_0", symbolic.Sub(c, b, a))
	comp.InsertGlobal(0, "zps_global_0_dup", symbolic.Sub(c, b, a))
	comp.InsertGlobal(0, "zps_global_1", symbolic.Sub(f, e, d2))

	// Negative shift — creates a ManuallyShifted column with zeros prepended
	a_shifted := column.Shift(a, -1)
	comp.InsertInclusion(0, "zps_inclusion",
		[]ifaces.Column{a_shifted},
		[]ifaces.Column{d2},
	)
}

func (d *ZeroPaddedShiftTestCase) Assign(run *wizard.ProverRuntime) {
	n := d.numRow

	// Right-padded data: active rows followed by zeros
	activeRows := n - n/4

	aVals := make([]field.Element, n)
	bVals := make([]field.Element, n)
	cVals := make([]field.Element, n)

	for i := 0; i < activeRows; i++ {
		aVals[i] = field.NewElement(uint64(i))
		bVals[i] = field.NewElement(uint64(i + 1))
		cVals[i] = field.NewElement(uint64(2*i + 1))
	}

	// ManuallyShifted(a, -1) = [0, 0, 1, 2, ..., activeRows-2, 0, ..., 0]
	// Values in the shifted column: {0, 1, ..., activeRows-2}
	// d must contain all those values
	dVals := make([]field.Element, n)
	eVals := make([]field.Element, n)
	fVals := make([]field.Element, n)

	for i := 0; i < activeRows; i++ {
		dVals[i] = field.NewElement(uint64(i))
		eVals[i] = field.NewElement(uint64(i + 1))
		fVals[i] = field.NewElement(uint64(2*i + 1))
	}

	run.AssignColumn("zps_a", smartvectors.RightZeroPadded(aVals[:activeRows], n))
	run.AssignColumn("zps_b", smartvectors.RightZeroPadded(bVals[:activeRows], n))
	run.AssignColumn("zps_c", smartvectors.RightZeroPadded(cVals[:activeRows], n))
	run.AssignColumn("zps_d", smartvectors.RightZeroPadded(dVals[:activeRows], n))
	run.AssignColumn("zps_e", smartvectors.RightZeroPadded(eVals[:activeRows], n))
	run.AssignColumn("zps_f", smartvectors.RightZeroPadded(fVals[:activeRows], n))
}

func (d *ZeroPaddedShiftTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("zps_module_0", d.wiop.Columns.GetHandle("zps_a")),
		distributed.SameSizeAdvice("zps_module_0", d.wiop.Columns.GetHandle("zps_d")),
	}
}

// TestDistributedManualShift runs the distributed wizard test for manual shift
// related test cases. Each exercises a different aspect of ManuallyShifted
// columns through the full segmentation and verification pipeline.
func TestDistributedManualShift(t *testing.T) {

	testCases := []DistributedTestCase{
		&ManualShiftLookupTestCase{numRow: 1 << NbRow},
		&ManualShiftMultiOffsetTestCase{numRow: 1 << NbRow},
		&FullColumnWithShiftTestCase{numRow: 1 << NbRow},
		&ZeroPaddedShiftTestCase{numRow: 1 << NbRow},
	}

	for _, tc := range testCases {
		t.Run(tc.Name(), func(t *testing.T) {
			runDistributedWizardTest(t, tc, false)
		})
	}
}
