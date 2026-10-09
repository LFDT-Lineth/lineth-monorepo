package logderivativesum_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
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
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
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
			sc.AssignWitness(rt)
			rt.AdvanceRound()
			sc.TamperResult(rt)
			runRound(rt)
			assert.Error(t, checkAllVerifierActions(rt),
				"compiled verifier must reject an invalid witness")
		})
	}
}

// TestCompile_WioptestSoundness_TamperZ runs every wioptest scenario with a
// constant-17 Z column and an arbitrary average cell C = 1234567 instead of
// the honest prover output, and pins each Result cell to Σ n·C over its Z
// columns. The final-sum verifier action then accepts, so the recurrence must
// reject the witness: with a constant Z it reads C·zDen = zNum on every row,
// which holds only if every row's sum of fractions is C.
//
// We bypass [proverAction.Run] entirely (no runRound), since the runtime
// rejects re-assigning a column. Instead we set up the post-prover state
// manually.
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
			sc.AssignWitness(rt)
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

			// Average cells C and Result cells Σ n·C: the final-sum identity
			// holds, so only the recurrence can catch the tampered Z.
			c := field.Lift(field.NewFromString("1234567"))
			for _, r := range sc.Sys.Rounds {
				for _, va := range r.VerifierActions {
					a, ok := va.(*logderivativesum.VerifierAction)
					if !ok {
						continue
					}
					var result field.Ext
					for _, e := range a.Entries {
						rt.AssignCell(e.Average, field.ElemFromExt(c))
						var n field.Element
						n.SetUint64(uint64(e.Module().RuntimeSize(rt)))
						var nc field.Ext
						nc.MulByElement(&c, &n)
						result.Add(&result, &nc)
					}
					rt.AssignCell(a.LogDerivativeSum.Result, field.ElemFromExt(result))
				}
			}

			require.NoError(t, checkAllVerifierActions(rt),
				"the final-sum identity holds for Result = Σ n·C")
			assert.Error(t, checkAllRecurrences(sc.Sys, rt),
				"the recurrence must reject a constant Z with an arbitrary average")
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
		a.Run(rt)
	}
}

func checkAllVerifierActions(rt *wiop.Runtime) error {
	for _, r := range rt.System.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(rt); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkAllRecurrences checks every multi-valued vanishing of sys: after
// Compile, the Z recurrences (and any vanishing of the scenario itself).
func checkAllRecurrences(sys *wiop.System, rt *wiop.Runtime) error {
	for _, m := range sys.Modules {
		for _, v := range m.Vanishings {
			if !v.Expression.IsMultiValued() {
				continue
			}
			if err := v.Check(rt); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifierEntries returns the Z entries of every compiled LogDerivativeSum
// of sys, read from the registered verifier actions.
func verifierEntries(t *testing.T, sys *wiop.System) []logderivativesum.ZEntry {
	t.Helper()
	var res []logderivativesum.ZEntry
	for _, r := range sys.Rounds {
		for _, va := range r.VerifierActions {
			if a, ok := va.(*logderivativesum.VerifierAction); ok {
				res = append(res, a.Entries...)
			}
		}
	}
	require.NotEmpty(t, res, "Compile must register a logderivativesum verifier action")
	return res
}

// averageCell returns the average cell of the only Z column of sys.
func averageCell(t *testing.T, sys *wiop.System) *wiop.Cell {
	t.Helper()
	entries := verifierEntries(t, sys)
	require.Len(t, entries, 1, "the system must have a single Z column")
	return entries[0].Average
}

// extVec returns the extension vector of the given small integers (possibly
// negative).
func extVec(vals ...int64) []field.Ext {
	res := make([]field.Ext, len(vals))
	for i, v := range vals {
		var x field.Element
		x.SetInt64(v)
		res[i] = field.Lift(x)
	}
	return res
}

// extGen returns the extension field.Gen of a small integer.
func extGen(v int64) field.Gen { return field.ElemFromExt(extVec(v)[0]) }

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
	num = mod.NewColumn(sys.Context.Childf("num"), r0)
	filter = mod.NewColumn(sys.Context.Childf("filter"), r0)
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
// vanishings on m, such as local constraints and endpoint openings. The
// cyclic recurrence needs none.
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
	sys, _, _, ld := newSimpleFilteredSum(t, 8)
	mod := sys.Modules[0]
	colsBefore := len(mod.Columns)
	vansBefore := len(mod.Vanishings)
	cellsBefore := len(ld.Result.Round().Cells)

	logderivativesum.Compile(sys)

	assert.Len(t, mod.Columns, colsBefore+1,
		"compile must add exactly one Z column for a single fraction")
	require.Len(t, mod.Vanishings, vansBefore+1,
		"compile must add the cyclic recurrence and no boundary constraint")
	assert.Equal(t, 0, countScalarVanishings(mod),
		"the cyclic recurrence needs no local constraint or endpoint opening")
	assert.Empty(t, mod.Vanishings[vansBefore].CancelledPositions,
		"the recurrence must hold on every row, row 0 included")
	assert.Len(t, ld.Result.Round().Cells, cellsBefore+1,
		"compile must add the Z column's average cell to the result round")
	assert.True(t, sys.LogDerivativeSums[0].IsReduced(),
		"the LogDerivativeSum query must be marked reduced after compile")
}

// TestCompile_RecurrenceForSizeOne: on a one-row module the recurrence is
// still registered, since it is what binds the average cell (C·zDen = zNum),
// and an honest witness verifies.
func TestCompile_RecurrenceForSizeOne(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 1)
	mod := sys.Modules[0]

	logderivativesum.Compile(sys)

	require.Len(t, mod.Vanishings, 1,
		"a size-1 module keeps the recurrence, which binds the average cell")

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(7))
	rt.AssignColumn(filter, makeVec(1))
	rt.AdvanceRound()
	runRound(rt)

	require.NoError(t, mod.Vanishings[0].Check(rt), "honest single-row recurrence must hold")
	require.NoError(t, checkAllVerifierActions(rt), "honest single-row witness must verify")
	requireGenEqual(t, genFromUint64(7), rt.GetCellValue(ld.Result), "single-row total must be 7")
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
		c := mod.NewColumn(sys.Context.Childf("c%d", i), r0)
		fractions[i] = wiop.Fraction{Numerator: c.View(), Denominator: one}
	}
	sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), fractions)

	colsBefore := len(mod.Columns)
	logderivativesum.Compile(sys)

	assert.Len(t, mod.Columns, colsBefore+2,
		"4 fractions must be packed into ⌈4/3⌉ = 2 Z columns")
	assert.Len(t, mod.Vanishings, 2,
		"two Z columns: each has its own cyclic recurrence vanishing")
	assert.Equal(t, 0, countScalarVanishings(mod),
		"cyclic Z columns contribute no scalar boundary constraint")
}

// ---- Completeness tests ----

// TestCompile_Completeness_NoFilter asserts that a fraction with no filter
// behaves exactly like the non-filtered LogDerivativeSum.
func TestCompile_Completeness_NoFilter(t *testing.T) {
	sys := wiop.NewSystemf("ld2-no-filter")
	r0 := sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Numerator: col.View(), Denominator: one}, // Filter is nil
	})

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, makeVec(2, 2, 2, 2))
	rt.AdvanceRound()
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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

	num := mod.NewColumn(sys.Context.Childf("num"), r0)
	den := mod.NewColumn(sys.Context.Childf("den"), r0)
	filter := mod.NewColumn(sys.Context.Childf("filter"), r0)

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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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

	num1 := mod.NewColumn(sys.Context.Childf("num1"), r0)
	num2 := mod.NewColumn(sys.Context.Childf("num2"), r0)
	num3 := mod.NewColumn(sys.Context.Childf("num3"), r0)
	den := mod.NewColumn(sys.Context.Childf("den"), r0)
	filter2 := mod.NewColumn(sys.Context.Childf("filter2"), r0)
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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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
	cA := mA.NewColumn(sys.Context.Childf("cA"), r0)
	cB := mB.NewColumn(sys.Context.Childf("cB"), r0)
	fB := mB.NewColumn(sys.Context.Childf("fB"), r0)
	oneA := wiop.NewConstantVector(mA, field.NewFromString("1"))
	oneB := wiop.NewConstantVector(mB, field.NewFromString("1"))

	ld := sys.NewLogDerivativeSum(sys.Context.Childf("ld2"), []wiop.Fraction{
		{Numerator: cA.View(), Denominator: oneA},
		{Filter: fB.View(), Numerator: cB.View(), Denominator: oneB},
	})

	logderivativesum.Compile(sys)
	// Each module gets one Z column → one cyclic recurrence.
	assert.Len(t, mA.Vanishings, 1)
	assert.Len(t, mB.Vanishings, 1)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(cA, makeVec(1, 2, 3, 4)) // sum_A = 10
	rt.AssignColumn(cB, makeVec(5, 6, 7, 8))
	rt.AssignColumn(fB, makeVec(1, 1, 0, 0)) // sum_B = 5 + 6 = 11
	rt.AdvanceRound()
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
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
	runRound(rt)

	assert.Error(t, checkAllVerifierActions(rt),
		"a corrupted Result cell must be detected by the verifier action")
}

// TestCompile_Soundness_WrongZ asserts the recurrence vanishing rejects a Z
// assignment that does not satisfy the running-sum relation, with the honest
// average cell.
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

	// v = [2, 2, 2, 2] and C = 2, so the honest Z is constant; a jump at row
	// 1 violates Z[1] − Z[0] = v[1] − C = 0.
	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(extVec(0, 5, 5, 5))})
	rt.AssignCell(averageCell(t, sys), extGen(2))

	require.Len(t, mod.Vanishings, 1, "the cyclic recurrence is the only constraint")
	assert.Error(t, mod.Vanishings[0].Check(rt),
		"recurrence vanishing must reject a Z column that violates the relation")
}

// TestCompile_Soundness_WrongAverage is the soundness core of the cyclic
// recurrence: an average cell C' that is not the column's total over n (here
// 3 instead of 2) can be made consistent with Result (n·C' = 12), but then no
// Z satisfies the recurrence, since its rows sum to Σ v − n·C' ≠ 0 while the
// left side telescopes to zero. The best attempt, Z[i] = Σ_{k≤i}(v[k] − C'),
// fails at row 0, where the cycle does not close.
func TestCompile_Soundness_WrongAverage(t *testing.T) {
	sys, num, filter, ld := newSimpleFilteredSum(t, 4)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(extVec(-1, -2, -3, -4))})
	rt.AssignCell(averageCell(t, sys), extGen(3))
	rt.AssignCell(ld.Result, extGen(12))

	require.NoError(t, checkAllVerifierActions(rt),
		"the final-sum identity holds for Result = n·C'")
	assert.Error(t, mod.Vanishings[0].Check(rt),
		"the recurrence must reject an average cell that is not the column's total over n")
}

// TestCompile_ZShiftedByConstant documents that the cyclic Z is only defined
// up to a constant: the recurrence reads differences of Z, and nothing else
// reads Z. Shifting the honest Z is therefore accepted, and harmless, since
// the total is carried by the average cell.
func TestCompile_ZShiftedByConstant(t *testing.T) {
	sys, num, filter, _ := newSimpleFilteredSum(t, 4)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(1, 2, 3, 6))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	// Total 12, C = 3: the honest Z is [−2, −3, −3, 0]; shift it by 5.
	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(extVec(3, 2, 2, 5))})
	rt.AssignCell(averageCell(t, sys), extGen(3))
	rt.AssignCell(sys.LogDerivativeSums[0].Result, extGen(12))

	require.NoError(t, mod.Vanishings[0].Check(rt), "a constant shift of Z satisfies the recurrence")
	require.NoError(t, checkAllVerifierActions(rt), "and the final-sum identity")
}

// ---- Dynamic-module coverage ----

// newDynamicFilteredSum is the dynamic-module counterpart of
// [newSimpleFilteredSum]: it builds the same single-fraction Num/1 LDS query
// but over a [wiop.NewDynamicModule] whose domain size is fixed at runtime
// by the first column assignment instead of at construction time. The
// helper does NOT supply a size — every [TestCompile_DynamicModule_*]
// caller is expected to pick one per [wiop.Runtime] via the witness vector
// it assigns.
func newDynamicFilteredSum(t *testing.T) (
	sys *wiop.System,
	num *wiop.Column,
	filter *wiop.Column,
	ld *wiop.LogDerivativeSum,
) {
	t.Helper()
	sys = wiop.NewSystemf("ld2-dyn-test")
	r0 := sys.NewRound()
	sys.NewRound() // hosts ld.Result
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	num = mod.NewColumn(sys.Context.Childf("num"), r0)
	filter = mod.NewColumn(sys.Context.Childf("filter"), r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	ld = sys.NewLogDerivativeSum(
		sys.Context.Childf("ld2-dyn"),
		[]wiop.Fraction{{
			Filter:      filter.View(),
			Numerator:   num.View(),
			Denominator: one,
		}},
	)
	return
}

// TestCompile_DynamicModule_Completeness exercises the completeness path for
// dynamic-module LDS: the same compiled System is re-driven against two
// different runtime sizes, and an honest witness must verify both times.
// This pins down the end-to-end behaviour of [zCol.At(-1)] +
// [Module.RuntimeSize] driving the endpoint LocalOpening to the correct row.
func TestCompile_DynamicModule_Completeness(t *testing.T) {
	sys, num, filter, _ := newDynamicFilteredSum(t)
	logderivativesum.Compile(sys)

	for _, tc := range []struct {
		name string
		n    int
	}{
		{"size-4", 4},
		{"size-8", 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nums := make([]uint64, tc.n)
			fil := make([]uint64, tc.n)
			for i := range nums {
				nums[i] = 2
				fil[i] = 1
			}

			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(num, makeVec(nums...))
			rt.AssignColumn(filter, makeVec(fil...))
			rt.AdvanceRound()
			runRound(rt)

			require.NoError(t, checkAllVerifierActions(rt),
				"honest dynamic-module assignment must verify (RuntimeSize=%d)", tc.n)
		})
	}
}

// TestCompile_DynamicModule_RecurrenceCatchesWrongZ is the soundness
// counterpart focused on the recurrence Vanishing. The honest Z for
// num=[2,2,2,2], filter=[1,1,1,1] is [2,4,6,8]; we instead pin Z to a
// constant column and assert the Vanishing rejects it. This proves the
// unconditional Vanishing emission for dynamic modules ([m.IsDynamic() ||
// m.Size() > 1]) is actually constraining at runtime — without it, a
// tampered Z would pass the recurrence and only the LDS verifier action's
// initial-condition check could catch the error.
func TestCompile_DynamicModule_RecurrenceCatchesWrongZ(t *testing.T) {
	sys, num, filter, _ := newDynamicFilteredSum(t)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	require.Len(t, mod.Vanishings, 1,
		"dynamic module must emit the cyclic recurrence and no boundary constraint")

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	// The honest Z is constant (v = C = 2); a jump at row 1 breaks it.
	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(extVec(0, 5, 5, 5))})
	rt.AssignCell(averageCell(t, sys), extGen(2))

	assert.Error(t, mod.Vanishings[0].Check(rt),
		"recurrence Vanishing must reject a wrong Z on a dynamic module")
}

// TestCompile_DynamicModule_Soundness_WrongResult is the dynamic-module
// counterpart of [TestCompile_Soundness_WrongResult]: a tampered Result
// cell must be caught by the LDS verifier action's claim-vs-running-sum
// identity, even when the underlying column lives on a dynamic module.
func TestCompile_DynamicModule_Soundness_WrongResult(t *testing.T) {
	sys, num, filter, ld := newDynamicFilteredSum(t)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()
	// Pre-assigning Result before the prover action runs short-circuits the
	// prover-side AssignCell (HasCellAssignment is true), so the bogus value
	// survives into the verifier action.
	rt.AssignCell(ld.Result, field.ElemFromBase(field.NewFromString("99")))
	runRound(rt)

	assert.Error(t, checkAllVerifierActions(rt),
		"a corrupted Result cell on a dynamic-module LDS must be detected by the verifier action")
}

// TestCompile_DynamicModule_Soundness_WrongAverage is the dynamic-module
// counterpart of [TestCompile_Soundness_WrongAverage]: n is the runtime size,
// and an average cell that is not the column's total over n is rejected by the
// recurrence, though consistent with Result.
func TestCompile_DynamicModule_Soundness_WrongAverage(t *testing.T) {
	sys, num, filter, ld := newDynamicFilteredSum(t)
	mod := sys.Modules[0]
	witnessColumns := append([]*wiop.Column{}, mod.Columns...)
	logderivativesum.Compile(sys)
	zCol := findZColumn(t, mod, witnessColumns)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(2, 2, 2, 2))
	rt.AssignColumn(filter, makeVec(1, 1, 1, 1))
	rt.AdvanceRound()

	rt.AssignColumn(zCol, &wiop.ConcreteVector{Plain: field.VecFromExt(extVec(-1, -2, -3, -4))})
	rt.AssignCell(averageCell(t, sys), extGen(3))
	rt.AssignCell(ld.Result, extGen(12))

	require.NoError(t, checkAllVerifierActions(rt),
		"the final-sum identity holds for Result = n·C' with the runtime n")
	assert.Error(t, mod.Vanishings[0].Check(rt),
		"the recurrence must reject a wrong average cell on a dynamic module")
}

// TestCompile_DynamicModule_SizeOne: a dynamic module whose runtime size
// turns out to be 1. The cyclic recurrence then reads C·zDen = zNum on the
// single row (Z's −1 shift wraps onto itself), and both the recurrence and
// the LDS verifier action must accept an honest single-row witness.
func TestCompile_DynamicModule_SizeOne(t *testing.T) {
	sys, num, filter, _ := newDynamicFilteredSum(t)
	mod := sys.Modules[0]
	logderivativesum.Compile(sys)

	require.Len(t, mod.Vanishings, 1,
		"dynamic module emits the cyclic recurrence, whatever its runtime size")

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(num, makeVec(7))
	rt.AssignColumn(filter, makeVec(1))
	rt.AdvanceRound()
	runRound(rt)

	require.NoError(t, mod.Vanishings[0].Check(rt),
		"honest recurrence must hold when RuntimeSize == 1")
	require.NoError(t, checkAllVerifierActions(rt),
		"honest single-row dynamic-module assignment must verify end-to-end")
}

// ---- Construction-time validation ----

func TestNewLogDerivativeSum_NilCtxPanic(t *testing.T) {
	sys := wiop.NewSystemf("s")
	sys.NewRound()
	sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	r := sys.Rounds[0]
	col := mod.NewColumn(sys.Context.Childf("c"), r)
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
	col := mod.NewColumn(sys.Context.Childf("c"), r0)
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
	col := mod.NewColumn(sys.Context.Childf("c"), r0)
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
	num := mod1.NewColumn(sys.Context.Childf("num"), r0)
	flt := mod2.NewColumn(sys.Context.Childf("flt"), r0)
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

	colS := mS.NewColumn(sys.Context.Childf("S"), r0)
	filterS := mS.NewColumn(sys.Context.Childf("filterS"), r0)
	colT := mT.NewColumn(sys.Context.Childf("T"), r0)
	colM := mT.NewColumn(sys.Context.Childf("M"), r0)

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
	runRound(rt)

	require.NoError(t, checkAllVerifierActions(rt))
	require.NoError(t, ld.Check(rt),
		"conditional-lookup style sum must reduce to zero when multiplicities are correct")

	requireGenEqual(t, genFromUint64(0), rt.GetCellValue(ld.Result),
		"S minus T·M (with filtered S) must aggregate to zero on a satisfied lookup")
}
