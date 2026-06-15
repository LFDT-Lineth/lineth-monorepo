package logderivativesum_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompile_WioptestCompleteness exercises every
// [wioptest.LogDerivativeSumCompilerScenarios] fixture: an honest assignment
// must drive the prover actions to completion and the verifier actions must
// then accept.
func TestCompile_WioptestCompleteness(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			logderivativesum.Compile(sc.Sys)
			proof := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof),
				"compiled verifier must accept an honest witness")
		})
	}
}

// TestCompile_WioptestSoundness exercises every
// [wioptest.LogDerivativeSumCompilerScenarios] fixture's invalid path. The
// witness is assigned honestly; the test then advances to the result round
// and corrupts the Result cell BEFORE the prover action runs (the prover
// skips re-assigning a cell that already holds a value). This isolates the
// verifier's claim-vs-running-sum identity from the per-bucket recurrence
// check.
func TestCompile_WioptestSoundness(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			logderivativesum.Compile(sc.Sys)
			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(&rt)
			rt.AdvanceRound()
			sc.TamperResult(&rt)
			runRound(&rt)
			assert.Error(t, checkAllVerifierActions(&rt),
				"compiled verifier must reject an invalid witness")
		})
	}
}

// TestCompile_WioptestSoundness_TamperZ runs every wioptest scenario with a
// constant-17 Z column instead of the honest prover output, and pins each
// Result cell to zero. The final-sum verifier action then rejects the witness
// because the sum of the (constant-17) Z[n-1] endpoints cannot equal zero.
//
// We bypass [proverAction.Run] entirely (no runRound), since the runtime
// rejects re-assigning a column. Instead we set up the post-prover state
// manually: Z gets the bogus value and each LogDerivativeSum's Result cell is
// pinned to zero.
func TestCompile_WioptestSoundness_TamperZ(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			// Snapshot per-module column lists so we can identify Z columns
			// (added by the compiler) by diffing after compile.
			beforeByMod := make(map[*wiop.Module]map[*wiop.Column]struct{})
			for _, m := range sc.Sys.Modules {
				cols := make(map[*wiop.Column]struct{}, len(m.Columns))
				for _, c := range m.Columns {
					cols[c] = struct{}{}
				}
				beforeByMod[m] = cols
			}

			logderivativesum.Compile(sc.Sys)

			var zCols []*wiop.Column
			for _, m := range sc.Sys.Modules {
				before := beforeByMod[m]
				for _, c := range m.Columns {
					if _, existed := before[c]; existed {
						continue
					}
					if c.IsExtension {
						zCols = append(zCols, c)
					}
				}
			}
			if len(zCols) == 0 {
				t.Skip("scenario emits no Z columns — nothing to tamper")
			}

			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(&rt)
			rt.AdvanceRound()

			// Pre-assign every Z column with a constant non-zero extension value.
			for _, z := range zCols {
				n := z.Module.RuntimeSize(rt)
				vals := make([]field.Ext, n)
				for i := range vals {
					vals[i] = field.Lift(field.NewFromString("17"))
				}
				rt.AssignColumn(z, &wiop.ConcreteVector{Plain: field.VecFromExt(vals)})
			}

			// Pin each LogDerivativeSum's Result cell to zero so the
			// claim-vs-running-sum check fires (the sum of constant-17
			// Z[n-1] across entries cannot equal zero).
			for _, ld := range sc.Sys.LogDerivativeSums {
				rt.AssignCell(ld.Result, field.ElemFromExt(field.Ext{}))
			}

			assert.Error(t, checkAllVerifierActions(&rt),
				"verifier must reject a corrupted Z column")
		})
	}
}

// ---- Helpers ----

func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// genFromUint64 builds a base-field-valued field.Gen from a uint64 literal.
func genFromUint64(v uint64) field.Gen {
	var x field.Element
	x.SetUint64(v)
	return field.ElemFromBase(x)
}

// requireGenEqual asserts that two field.Gen values represent the same field
// element. The base/extension wrapper flag is ignored, since the compiler may
// choose to store the result in extension form even when the value is in the
// base subfield.
func requireGenEqual(t *testing.T, want, got field.Gen, msg string) {
	t.Helper()
	diff := want.Sub(got)
	if !diff.IsZero() {
		t.Fatalf("%s: want=%v got=%v", msg, want, got)
	}
}

func runRound(rt *wiop.Runtime) {
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(*rt)
	}
}

func checkAllVerifierActions(rt *wiop.Runtime) error {
	for _, r := range rt.System.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(*rt); err != nil {
				return err
			}
		}
	}
	return nil
}

func findZColumn(t *testing.T, m *wiop.Module, existing []*wiop.Column) *wiop.Column {
	t.Helper()
	known := make(map[*wiop.Column]struct{}, len(existing))
	for _, c := range existing {
		known[c] = struct{}{}
	}
	for i := len(m.Columns) - 1; i >= 0; i-- {
		if _, ok := known[m.Columns[i]]; !ok {
			return m.Columns[i]
		}
	}
	t.Fatalf("no Z column found on module %s", m.Context.Path())
	return nil
}

// newSimpleSum builds a system with a single fraction Num/1 over a column
// committed in round 0 and a filter committed in round 0 as well. The query
// result cell lives in round 1.
func newSimpleFilteredSum(t *testing.T, n int) (
	sys *wiop.System,
	num *wiop.Column,
	filter *wiop.Column,
	ld *wiop.LogDerivativeSum,
) {
	t.Helper()
	sys = wiop.NewSystemf("ld2-test")
	r0 := sys.NewRound()
	sys.NewRound() // hosts ld.Result
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), n, wiop.PaddingDirectionNone)
	num = mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	filter = mod.NewColumn(sys.Context.Childf("filter"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld = sys.NewLogDerivativeSum(
		sys.Context.Childf("ld2"),
		[]wiop.Fraction{{
			Filter:      filter.View(),
			Numerator:   num.View(),
			Denominator: one,
		}},
	)
	return
}

// ---- Structural tests ----

// countScalarVanishings returns the number of scalar (non-multi-valued)
// vanishings on m — i.e. the endpoint openings produced by
// [wiop.ColumnPosition.Open], as opposed to the multi-valued Z recurrence.
func countScalarVanishings(m *wiop.Module) int {
	n := 0
	for _, v := range m.Vanishings {
		if !v.Expression.IsMultiValued() {
			n++
		}
	}
	return n
}

func TestCompile_AddsZColumnAndVanishing(t *testing.T) {
	sys, _, _, _ := newSimpleFilteredSum(t, 8)
	mod := sys.Modules[0]
	colsBefore := len(mod.Columns)
	vansBefore := len(mod.Vanishings)

	logderivativesum.Compile(sys)

	assert.Len(t, mod.Columns, colsBefore+1,
		"compile must add exactly one Z column for a single fraction")
	require.Len(t, mod.Vanishings, vansBefore+3,
		"compile must add one recurrence plus the two endpoint-opening vanishings")
	assert.Equal(t, 2, countScalarVanishings(mod),
		"the Z[0] and Z[n-1] openings are scalar vanishings")
	assert.True(t, sys.LogDerivativeSums[0].IsReduced(),
		"the LogDerivativeSum query must be marked reduced after compile")
}

func TestCompile_SkipsRecurrenceForSizeOne(t *testing.T) {
	sys, _, _, _ := newSimpleFilteredSum(t, 1)
	mod := sys.Modules[0]

	logderivativesum.Compile(sys)

	require.Len(t, mod.Vanishings, 2,
		"a size-1 module needs no recurrence; only the two endpoint openings remain")
	assert.Equal(t, 2, countScalarVanishings(mod),
		"Z[0] and Z[n-1] coincide but each opening is still a scalar vanishing")
}

func TestCompile_Idempotent(t *testing.T) {
	sys, _, _, _ := newSimpleFilteredSum(t, 8)
	logderivativesum.Compile(sys)

	mod := sys.Modules[0]
	colsAfterFirst := len(mod.Columns)
	vansAfterFirst := len(mod.Vanishings)

	logderivativesum.Compile(sys)

	assert.Len(t, mod.Columns, colsAfterFirst,
		"second compile must not add new Z columns")
	assert.Len(t, mod.Vanishings, vansAfterFirst,
		"second compile must not add new vanishings (recurrence or openings)")
}

func TestCompile_NoQueries(t *testing.T) {
	sys := wiop.NewSystemf("ld2-empty")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	logderivativesum.Compile(sys) // must not panic

	for _, m := range sys.Modules {
		assert.Empty(t, m.Vanishings)
	}
}

// TestCompile_PacksFractions verifies that 4 filtered fractions on the same
// module are packed into ⌈4/3⌉ = 2 Z columns.
func TestCompile_PacksFractions(t *testing.T) {
	sys := wiop.NewSystemf("ld2-pack")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))

	fractions := make([]wiop.Fraction, 4)
	for i := range fractions {
		c := mod.NewColumn(sys.Context.Childf("c%d", i), wiop.VisibilityOracle, r0)
		fractions[i] = wiop.Fraction{Numerator: c.View(), Denominator: one}
	}
	sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), fractions)

	colsBefore := len(mod.Columns)
	logderivativesum.Compile(sys)

	assert.Len(t, mod.Columns, colsBefore+2,
		"4 fractions must be packed into ⌈4/3⌉ = 2 Z columns")
	assert.Len(t, mod.Vanishings, 6,
		"two Z columns: each has its own recurrence vanishing plus two endpoint openings")
	assert.Equal(t, 4, countScalarVanishings(mod),
		"each Z column contributes two scalar endpoint openings (Z[0] and Z[n-1])")
}

// ---- Completeness tests ----

// TestCompile_Completeness_NoFilter asserts that a fraction with no filter
// behaves exactly like the non-filtered LogDerivativeSum.
func TestCompile_Completeness_NoFilter(t *testing.T) {
	sys := wiop.NewSystemf("ld2-no-filter")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Numerator: col.View(), Denominator: one}, // Filter is nil
	})

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(2, 2, 2, 2))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt), "the original query must be consistent with the compiled artefacts")
}

// TestCompile_Completeness_AllOnes verifies that an all-ones filter yields
// the same sum as having no filter at all.
func TestCompile_Completeness_AllOnes(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// values [3, 5, 7, 9], filter all ones → sum = 24.
	rt.AssignColumn(num, makeVec(3, 5, 7, 9))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(24), rt.GetCellValue(ld.Result),
		"the compiled sum must match num·1 summed over all rows")
}

// TestCompile_Completeness_AllZeros verifies that an all-zero filter yields a
// zero sum and that Z is uniformly zero (the constant prefix sum).
func TestCompile_Completeness_AllZeros(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(3, 5, 7, 9))
	rt.AssignColumn(filter, makeVec(0, 0, 0, 0))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(0), rt.GetCellValue(ld.Result),
		"an all-zero filter must zero out the sum")
}

// TestCompile_Completeness_PartialFilter verifies that the filter masks
// individual rows: only rows with filter[i] = 1 contribute.
func TestCompile_Completeness_PartialFilter(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// values [3, 5, 7, 9], filter [1, 0, 1, 0] → sum = 3 + 7 = 10.
	rt.AssignColumn(num, makeVec(3, 5, 7, 9))
	rt.AssignColumn(filter, makeVec(1, 0, 1, 0))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(10), rt.GetCellValue(ld.Result),
		"only filtered-in rows must contribute")
}

// TestCompile_Completeness_FilterMasksZeroDenominator demonstrates the main
// reason for filter support: a row with a zero denominator is OK as long as
// the filter masks it. Without the filter, this configuration would panic in
// the prover.
func TestCompile_Completeness_FilterMasksZeroDenominator(t *testing.T) {
	sys := wiop.NewSystemf("ld2-mask-zero-den")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	num := mod.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	den := mod.NewColumn(sys.Context.Childf("den"), wiop.VisibilityOracle, r0)
	filter := mod.NewColumn(sys.Context.Childf("filter"), wiop.VisibilityOracle, r0)

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Filter: filter.View(), Numerator: num.View(), Denominator: den.View()},
	})

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// num [4, 99, 8, 99], den [2, 0, 4, 0], filter [1, 0, 1, 0]
	// → sum = 4/2 + 8/4 = 2 + 2 = 4. The 99/0 rows are masked.
	rt.AssignColumn(num, makeVec(4, 99, 8, 99))
	rt.AssignColumn(den, makeVec(2, 0, 4, 0))
	rt.AssignColumn(filter, makeVec(1, 0, 1, 0))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(4), rt.GetCellValue(ld.Result),
		"masked rows with zero denominator must not contribute")
}

// TestCompile_Completeness_PackedMixedFilters exercises packing where
// fractions have heterogeneous filter configurations (some nil, some present).
func TestCompile_Completeness_PackedMixedFilters(t *testing.T) {
	sys := wiop.NewSystemf("ld2-mixed")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	num1 := mod.NewColumn(sys.Context.Childf("num1"), wiop.VisibilityOracle, r0)
	num2 := mod.NewColumn(sys.Context.Childf("num2"), wiop.VisibilityOracle, r0)
	num3 := mod.NewColumn(sys.Context.Childf("num3"), wiop.VisibilityOracle, r0)
	den := mod.NewColumn(sys.Context.Childf("den"), wiop.VisibilityOracle, r0)
	filter2 := mod.NewColumn(sys.Context.Childf("filter2"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Numerator: num1.View(), Denominator: one},                         // no filter, den = 1
		{Filter: filter2.View(), Numerator: num2.View(), Denominator: one}, // filtered, den = 1
		{Numerator: num3.View(), Denominator: den.View()},                  // no filter, vector den
	})

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num1, makeVec(1, 2, 3, 4)) // sum = 10
	rt.AssignColumn(num2, makeVec(10, 20, 30, 40))
	rt.AssignColumn(filter2, makeVec(1, 0, 1, 0)) // contributes 10 + 30 = 40
	rt.AssignColumn(num3, makeVec(6, 6, 8, 8))
	rt.AssignColumn(den, makeVec(2, 2, 2, 2)) // sum = 3+3+4+4 = 14
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(64), rt.GetCellValue(ld.Result),
		"packed mixed-filter sum must aggregate to 10 + 40 + 14")
}

// TestCompile_Completeness_BucketsByModule verifies that fractions on
// distinct modules each get their own Z column on the right module.
func TestCompile_Completeness_BucketsByModule(t *testing.T) {
	sys := wiop.NewSystemf("ld2-multi-mod")
	r0 := sys.NewRound()
	sys.NewRound()

	mA := sys.NewSizedModule(sys.Context.Childf("mA"), 4, wiop.PaddingDirectionNone)
	mB := sys.NewSizedModule(sys.Context.Childf("mB"), 4, wiop.PaddingDirectionNone)
	cA := mA.NewColumn(sys.Context.Childf("cA"), wiop.VisibilityOracle, r0)
	cB := mB.NewColumn(sys.Context.Childf("cB"), wiop.VisibilityOracle, r0)
	fB := mB.NewColumn(sys.Context.Childf("fB"), wiop.VisibilityOracle, r0)
	oneA := wiop.NewConstantVector(mA, field.NewFromString("1"))
	oneB := wiop.NewConstantVector(mB, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Numerator: cA.View(), Denominator: oneA},
		{Filter: fB.View(), Numerator: cB.View(), Denominator: oneB},
	})

	logderivativesum.Compile(sys)
	// Each module gets one Z column → one recurrence plus two endpoint openings.
	assert.Len(t, mA.Vanishings, 3)
	assert.Len(t, mB.Vanishings, 3)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(cA, makeVec(1, 2, 3, 4)) // sum_A = 10
	rt.AssignColumn(cB, makeVec(5, 6, 7, 8))
	rt.AssignColumn(fB, makeVec(1, 1, 0, 0)) // sum_B = 5 + 6 = 11
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt))

	requireGenEqual(t, genFromUint64(21), rt.GetCellValue(ld.Result),
		"per-module Z values must aggregate to 10 + 11 = 21")
}

// ---- Soundness tests ----

// TestCompile_Soundness_WrongResult asserts that the verifier action rejects
// a manipulated Result cell.
func TestCompile_Soundness_WrongResult(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()
	rt.AssignCell(ld.Result, field.ElemFromBase(field.NewFromString("99")))
	runRound(&rt)

	assert.Error(t, checkAllVerifierActions(&rt),
		"a corrupted Result cell must be detected by the verifier action")
}

// TestCompile_Soundness_WrongZ asserts the recurrence vanishing rejects a Z
// assignment that does not satisfy the running-sum relation.
func TestCompile_Soundness_WrongZ(t *testing.T) {
	sys, num, filter, _ := newSimpleFilteredSum(t, 4)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	// honest Z would be [2, 4, 6, 8]; constant Z violates Z[i] − Z[i−1] = 2.
	bogus := []field.Ext{
		field.Lift(field.NewFromString("1")),
		field.Lift(field.NewFromString("1")),
		field.Lift(field.NewFromString("1")),
		field.Lift(field.NewFromString("1")),
	}
	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(bogus)})

	require.Len(t, mod.Vanishings, 3,
		"one recurrence, the row-0 local constraint, and the endpoint opening")
	rec := mod.Vanishings[0] // the recurrence is registered before the boundary constraints
	require.True(t, rec.Expression.IsMultiValued(), "Vanishings[0] must be the recurrence")
	assert.Error(t, rec.Check(rt),
		"recurrence vanishing must reject a Z column that violates the relation")
}

// TestCompile_Soundness_WrongInitialZ asserts the row-0 local constraint
// rejects a Z column whose row 0 does not satisfy Z[0]·zDen[0] = zNum[0],
// even though it satisfies the recurrence. This boundary used to be checked by
// the verifier action; it is now pinned in-circuit by a local constraint, so
// the verifier action (final-sum only) no longer sees it.
func TestCompile_Soundness_WrongInitialZ(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	// Z that satisfies the recurrence (constant difference of 2) but with a
	// shifted base: Z[0] = 5 instead of 2.
	shifted := []field.Ext{
		field.Lift(field.NewFromString("5")),
		field.Lift(field.NewFromString("7")),
		field.Lift(field.NewFromString("9")),
		field.Lift(field.NewFromString("11")),
	}
	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(shifted)})

	// The endpoint opening is lazy: it resolves to the (malicious) Z column
	// value when read, so no explicit assignment is needed here.
	rt.AssignCell(ld.Result, field.ElemFromExt(shifted[3]))

	// The recurrence (multi-valued) accepts the constant-step Z, and the
	// final-sum verifier action only compares consistent endpoints to Result —
	// neither catches the shifted base.
	rec := mod.Vanishings[0]
	require.True(t, rec.Expression.IsMultiValued(), "Vanishings[0] must be the recurrence")
	require.NoError(t, rec.Check(rt), "recurrence must accept a constant-step Z")
	require.NoError(t, checkAllVerifierActions(&rt),
		"the final-sum verifier action only checks endpoints against Result")

	// The row-0 local constraint must reject the shifted base.
	rejected := false
	for _, v := range mod.Vanishings {
		if v.Expression.IsMultiValued() {
			continue
		}
		if err := v.Check(rt); err != nil {
			rejected = true
		}
	}
	assert.True(t, rejected,
		"the row-0 local constraint must reject a Z whose row-0 value disagrees with zNum[0]/zDen[0]")
}

// ---- Construction-time validation ----

func TestNewLogDerivativeSum_NilCtxPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	r := sys.Rounds[0]
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	frac := wiop.Fraction{Numerator: col.View(), Denominator: one}
	assert.Panics(t, func() { sys.NewLogDerivativeSum(nil, []wiop.Fraction{frac}) })
}

func TestNewLogDerivativeSum_EmptyFractionsPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), nil)
	})
}

func TestNewLogDerivativeSum_NilNumeratorPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	frac := wiop.Fraction{Numerator: nil, Denominator: col.View()}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestNewLogDerivativeSum_NilDenominatorPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)
	frac := wiop.Fraction{Numerator: col.View(), Denominator: nil}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

func TestNewLogDerivativeSum_FilterModuleMismatchPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	r0 := sys.NewRound()
	sys.NewRound()
	mod1 := sys.NewSizedModule(sys.Context.Childf("m1"), 4, wiop.PaddingDirectionNone)
	mod2 := sys.NewSizedModule(sys.Context.Childf("m2"), 4, wiop.PaddingDirectionNone)
	num := mod1.NewColumn(sys.Context.Childf("num"), wiop.VisibilityOracle, r0)
	flt := mod2.NewColumn(sys.Context.Childf("flt"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod1, field.NewFromString("1"))
	frac := wiop.Fraction{Filter: flt.View(), Numerator: num.View(), Denominator: one}
	assert.Panics(t, func() {
		sys.NewLogDerivativeSum(sys.Context.Childf("q"), []wiop.Fraction{frac})
	})
}

// TestCompile_ConditionalLookupShape models a tiny conditional-lookup pattern:
// we emit S-side fractions Filter_S/(γ + S) and a T-side fraction
// −M/(γ + T), and check that the aggregate is zero when the multiplicities
// are correct. γ is a fixed constant rather than a coin (sufficient for a
// soundness sanity check here, since we choose denominators to be non-zero).
func TestCompile_ConditionalLookupShape(t *testing.T) {
	sys := wiop.NewSystemf("ld2-cond-lookup")
	r0 := sys.NewRound()
	sys.NewRound()

	// The "table" T = [10, 20] and the "checked" S = [10, 10, 20, 99].
	// filterS = [1, 1, 1, 0] gates out the bogus 99. Multiplicities M = [2, 1].
	//
	// The aggregate is
	//     (1/(γ+10) + 1/(γ+10) + 1/(γ+20) + 0) − (2/(γ+10) + 1/(γ+20)) = 0.
	mS := sys.NewSizedModule(sys.Context.Childf("mS"), 4, wiop.PaddingDirectionNone)
	mT := sys.NewSizedModule(sys.Context.Childf("mT"), 2, wiop.PaddingDirectionNone)

	colS := mS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := mS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)
	colT := mT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colM := mT.NewColumn(sys.Context.Childf("M"), wiop.VisibilityOracle, r0)

	gammaS := wiop.NewConstantVector(mS, field.NewFromString("7"))
	gammaT := wiop.NewConstantVector(mT, field.NewFromString("7"))

	// (1/(γ+S)) and (−M/(γ+T))
	denS := wiop.Add(gammaS, colS.View())
	denT := wiop.Add(gammaT, colT.View())
	negM := wiop.Negate(colM.View())
	oneS := wiop.NewConstantVector(mS, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Filter: filterS.View(), Numerator: oneS, Denominator: denS},
		{Numerator: negM, Denominator: denT},
	})

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colS, makeVec(10, 10, 20, 99))
	rt.AssignColumn(filterS, makeVec(1, 1, 1, 0))
	rt.AssignColumn(colT, makeVec(10, 20))
	rt.AssignColumn(colM, makeVec(2, 1))
	rt.AdvanceRound()
	runRound(&rt)

	require.NoError(t, checkAllVerifierActions(&rt))
	require.NoError(t, ld.Check(rt),
		"conditional-lookup style sum must reduce to zero when multiplicities are correct")

	requireGenEqual(t, genFromUint64(0), rt.GetCellValue(ld.Result),
		"S minus T·M (with filtered S) must aggregate to zero on a satisfied lookup")
}
