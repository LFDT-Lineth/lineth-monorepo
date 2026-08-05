package lookuptologderivsum_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompile_WioptestCompleteness runs every honest-witness scenario from
// [wioptest.LookupScenarios] through the full lookuptologderivsum →
// logderivativesum pipeline. The verifier must accept.
func TestCompile_WioptestCompleteness(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			lookuptologderivsum.Compile(sc.Sys)
			logderivativesum.Compile(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub),
				"compiled verifier must accept an honest witness")
		})
	}
}

// TestCompile_WioptestSoundnessPanics runs every prover-side soundness
// scenario from [wioptest.LookupSoundnessScenarios]. Each one is engineered
// so that the M-assignment prover action (round 0) panics; we assert that
// behaviour via assert.Panics.
func TestCompile_WioptestSoundnessPanics(t *testing.T) {
	for _, build := range wioptest.LookupSoundnessScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			lookuptologderivsum.Compile(sc.Sys)
			logderivativesum.Compile(sc.Sys)
			assert.Panics(t, func() { sc.Sys.Prove(sc.AssignWitness) },
				"M-assignment prover task must panic on this invalid witness")
		})
	}
}

// TestCompile_WioptestSoundness_TamperM runs every honest wioptest lookup
// scenario but overwrites the multiplicity column(s) M with zeros before
// the M-assignment prover task can run. The aggregated LogDerivativeSum
// then equals the sum of A-side fractions only (B-side cancels to zero),
// which is non-zero with overwhelming probability over γ. The
// resultIsZeroVerifierAction must reject.
//
// Lookup soundness is therefore double-covered: prover-side panics for
// outright violations (no matching B row, etc.) AND verifier-side
// rejection when a malicious prover skips the M-assignment task. The
// EmptySelected scenario is skipped because its A-side contributes zero
// to the aggregate even with M=0, so the verifier cannot distinguish.
func TestCompile_WioptestSoundness_TamperM(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			if sc.Name == "EmptySelected" {
				t.Skip("A-side contributes zero with all-zero filter; M=0 is consistent")
			}

			// Snapshot per-module columns to identify the M column(s)
			// (added by the lookuptologderivsum pass as new non-extension
			// columns on each lookup-table module).
			beforeByMod := make(map[*wiop.Module]map[*wiop.Column]struct{})
			for _, m := range sc.Sys.Modules {
				cols := make(map[*wiop.Column]struct{}, len(m.Columns))
				for _, c := range m.Columns {
					cols[c] = struct{}{}
				}
				beforeByMod[m] = cols
			}

			lookuptologderivsum.Compile(sc.Sys)

			var mCols []*wiop.Column
			for _, m := range sc.Sys.Modules {
				before := beforeByMod[m]
				for _, c := range m.Columns {
					if _, existed := before[c]; existed {
						continue
					}
					if !c.IsExtension {
						mCols = append(mCols, c)
					}
				}
			}
			require.NotEmpty(t, mCols,
				"lookuptologderivsum must allocate at least one M column")

			logderivativesum.Compile(sc.Sys)

			rt := wiop.NewRuntime(sc.Sys)
			sc.AssignWitness(rt)

			// Pre-assign every M with zeros. The mAssignmentTask doesn't
			// guard against re-assignment, so we must skip the round-0
			// prover loop entirely (otherwise the task panics on M's
			// pre-existing assignment). The two AdvanceRound calls below
			// move us straight to the result round; runRound there triggers
			// only the LDS prover task, which sees the bogus M and computes
			// a non-zero aggregated sum.
			for _, mCol := range mCols {
				n := mCol.Module.RuntimeSize(rt)
				zeros := make([]field.Element, n)
				rt.AssignColumn(mCol, &wiop.ConcreteVector{Plain: field.VecFromBase(zeros)})
			}
			rt.AdvanceRound() // → coin round (samples α/γ)
			rt.AdvanceRound() // → result round
			runRound(rt)      // assigns Z and the LDS result

			err := checkAllVerifierActions(rt)
			assert.ErrorContains(t, err, "must be zero",
				"verifier must reject a tampered M (aggregated sum != 0)")
		})
	}
}

// ---- Helpers ----

// makeVec builds a base-field ConcreteVector from uint64 literals. Length is
// inferred from the variadic arguments.
func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// runRound executes every prover action registered on the runtime's current
// round.
func runRound(rt *wiop.Runtime) {
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(rt)
	}
}

// checkAllVerifierActions evaluates every verifier action across every round
// of the runtime. Returns the first non-nil error or nil if everything
// passes.
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

// driveProtocol mimics the canonical "assign-witness → run → advance" loop
// for our two-stage compiled lookup. After this returns, every prover action
// has run and the verifier actions are ready to be checked.
//
// Round structure assumed:
//   - Round 0: user-witness columns (already assigned by the caller) plus the
//     M column. The M-assignment prover action runs here, before any coin is
//     sampled, so M cannot be chosen as a function of γ.
//   - Round 1: α and γ coins; no prover actions.
//   - Round 2: LogDerivativeSum result + Z columns; one prover action
//     assigns Z and the result cell.
func driveProtocol(rt *wiop.Runtime) {
	runRound(rt)      // round 0: assigns M
	rt.AdvanceRound() // → round 1, samples α/γ
	rt.AdvanceRound() // → round 2
	runRound(rt)      // assigns Z and the LogDerivativeSum result
}

// ---- Single-column, no filters ----

func TestCompile_SingleColumn_NoFilters(t *testing.T) {
	sys := wiop.NewSystemf("ll-simple")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())}, // included (A)
		[]wiop.Table{wiop.NewTable(colT.View())}, // including (B)
	)

	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// T = [10, 20, 30, 40], S = [10, 20, 10, 30] — every S value appears in T.
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS, makeVec(10, 20, 10, 30))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

func TestCompile_SingleColumn_NoMatchPanics(t *testing.T) {
	sys := wiop.NewSystemf("ll-nomatch")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS, makeVec(10, 99, 10, 30)) // 99 is not in T

	assert.Panics(t, func() {
		runRound(rt) // round 0 — M assignment task
	}, "M assignment must panic when an active A row has no match in B")
}

// ---- Row limit (compile-time static check) ----
//
// These tests exceed wiop.MaxLookupRows (2^30) for real by declaring a static
// module of that size. No 2^30 vector is ever materialised: a static module's
// size is metadata, and the compile-time row-limit check
// ([wiop.TableRelationQuery.PrecheckRowLimit], invoked from Compile) reads
// only that size. Because the compile-time bound is a conservative upper bound
// (dynamic modules are counted as their 2^22 max, and static ones as their
// exact size), an over-limit static lookup is caught during Compile itself —
// before any witness exists — so these assert on Compile panicking. The runtime
// prover/verifier variants are exercised directly in query_table_relation_test.go.

// buildOverLimitSystem builds (but does not compile) a single-column inclusion
// S ⊆ T whose A side (aSize) and B side (bSize) are declared with the given
// static module sizes.
func buildOverLimitSystem(t *testing.T, aSize, bSize int) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("ll-limit")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), bSize, wiop.PaddingDirectionRight)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), aSize, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	return sys
}

// TestCompile_RowLimit_CompilePanics_ASide compiles a lookup whose A side has
// 2^30 rows. The compile-time static row-limit check must panic before any
// witness is assigned.
func TestCompile_RowLimit_CompilePanics_ASide(t *testing.T) {
	sys := buildOverLimitSystem(t, 1<<30, 2) // A side = 2^30 rows (>= bound); B side tiny.
	assert.Panics(t, func() { lookuptologderivsum.Compile(sys) },
		"Compile must panic when a lookup A side reaches the row limit")
}

// TestCompile_RowLimit_CompilePanics_BSide compiles a lookup whose B side has
// 2^30 rows. The compile-time check must panic on the B side independently of
// the A side.
func TestCompile_RowLimit_CompilePanics_BSide(t *testing.T) {
	sys := buildOverLimitSystem(t, 2, 1<<30) // B side = 2^30 rows (>= bound); A side tiny.
	assert.Panics(t, func() { lookuptologderivsum.Compile(sys) },
		"Compile must panic when a lookup B side reaches the row limit")
}

// TestCompile_RowLimit_GroupingSplitsIntoSubgroups builds three lookups that
// share the same including (B) table, each with a 2^29-row A side.
//
// Sharing a lookup table no longer tightens the per-lookup budget: instead of
// dividing MaxLookupRows = 2^30 by the lookup count, the compiler bin-packs the
// lookups into subgroups whose static A-side total stays strictly below the
// budget. Two 2^29-row sides already sum to exactly 2^30 (the budget is
// exclusive), so each lookup lands in its own subgroup — three subgroups, three
// multiplicity columns M on the shared table module — and Compile must NOT
// panic. This is the direct counterpart to the removed "grouping tightens the
// bound" behaviour.
func TestCompile_RowLimit_GroupingSplitsIntoSubgroups(t *testing.T) {
	sys := wiop.NewSystemf("ll-limit-group")
	r0 := sys.NewRound()
	// One shared B table (same colT pointer ⇒ same canonicalIncludingKey ⇒ all
	// queries land in the same bucket, to be split into subgroups).
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	bTable := wiop.NewTable(colT.View())

	const nLookups = 3
	for i := 0; i < nLookups; i++ {
		modS := sys.NewSizedModule(sys.Context.Childf("modS%d", i), 1<<29, wiop.PaddingDirectionRight)
		colS := modS.NewColumn(sys.Context.Childf("S%d", i), wiop.VisibilityOracle, r0)
		sys.NewInclusion(
			sys.Context.Childf("inc%d", i),
			[]wiop.Table{wiop.NewTable(colS.View())},
			[]wiop.Table{bTable},
		)
	}

	colsBefore := len(modT.Columns)
	assert.NotPanics(t, func() { lookuptologderivsum.Compile(sys) },
		"bin-packing must open a fresh subgroup per over-budget-when-combined lookup instead of panicking")
	// Each 2^29-row lookup occupies its own subgroup (two would reach 2^30), so
	// the shared table module gains exactly one M column per lookup.
	assert.Len(t, modT.Columns, colsBefore+nLookups,
		"each subgroup must allocate its own multiplicity column M on the shared table")
	assert.Len(t, sys.LogDerivativeSums, 1,
		"every subgroup's fractions must still fold into a single aggregated LogDerivativeSum")
}

// TestCompile_RowLimit_MultipleFragmentsCombine builds a single inclusion whose
// A side has two fragments, each 2^29 rows. Neither fragment reaches the
// MaxLookupRows = 2^30 budget on its own, but the check sums the fragment
// heights per side (2^29 + 2^29 = 2^30), so their combined height reaches the
// limit and Compile must panic. This exercises the per-side summation across
// multiple columns, as opposed to a single over-sized column.
func TestCompile_RowLimit_MultipleFragmentsCombine(t *testing.T) {
	sys := wiop.NewSystemf("ll-limit-multifrag")
	r0 := sys.NewRound()

	// Tiny B (including) table.
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)

	// Two A (included) fragments, each 2^29 rows and individually under budget.
	modS0 := sys.NewSizedModule(sys.Context.Childf("modS0"), 1<<29, wiop.PaddingDirectionRight)
	colS0 := modS0.NewColumn(sys.Context.Childf("S0"), wiop.VisibilityOracle, r0)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 1<<29, wiop.PaddingDirectionRight)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS0.View()), wiop.NewTable(colS1.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	assert.Panics(t, func() { lookuptologderivsum.Compile(sys) },
		"two 2^29-row A fragments must sum to the 2^30 budget and make Compile reject the lookup")
}

// ---- Filter on the included side (A) ----

func TestCompile_FilterOnIncluded(t *testing.T) {
	sys := wiop.NewSystemf("ll-filterA")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// T = [10, 20]; S = [10, 99, 20, 99] — the 99s are masked by filterS.
	rt.AssignColumn(colT, makeVec(10, 20))
	rt.AssignColumn(colS, makeVec(10, 99, 20, 99))
	rt.AssignColumn(filterS, makeVec(1, 0, 1, 0))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// ---- Filter on the including side (B) ----

func TestCompile_FilterOnIncluding(t *testing.T) {
	sys := wiop.NewSystemf("ll-filterT")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// T = [10, 999, 20, 999] with filterT = [1, 0, 1, 0] — only 10 and 20 are
	// valid table entries. S references 10 and 20.
	rt.AssignColumn(colT, makeVec(10, 999, 20, 999))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 0))
	rt.AssignColumn(colS, makeVec(10, 20, 10, 20))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// TestCompile_FilterOnIncluding_FilteredTRowCantMatch verifies that an A row
// whose value matches a *masked-out* B row is rejected at M-assignment time.
// The IsFilteredOnIncluding trick prepends the filter to B (so the masked B
// rows hash differently from any A row whose head is 1), so the M-assignment
// task should report no match.
func TestCompile_FilterOnIncluding_FilteredTRowCantMatch(t *testing.T) {
	sys := wiop.NewSystemf("ll-filterT-mask")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// T = [10, 99, 20, 30]; filterT = [1, 0, 1, 1] — 99 is masked out.
	// S = [99] tries to match the masked-out row.
	rt.AssignColumn(colT, makeVec(10, 99, 20, 30))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	rt.AssignColumn(colS, makeVec(99))

	assert.Panics(t, func() { runRound(rt) },
		"matching a filtered-out B row must be rejected by M assignment")
}

// ---- Filters on both sides ----

func TestCompile_DoubleConditional(t *testing.T) {
	sys := wiop.NewSystemf("ll-double")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 999, 20, 999))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 0))
	rt.AssignColumn(colS, makeVec(10, 0, 20, 7))
	rt.AssignColumn(filterS, makeVec(1, 0, 1, 0))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// ---- Multi-column lookup (uses α coin) ----

func TestCompile_MultiColumn(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-col")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Two-column table: rows (1,10), (2,20), (3,30), (4,40).
	// S takes (2,20) twice and (3,30), (1,10).
	rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
	rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
	rt.AssignColumn(sx, makeVec(2, 2, 3, 1))
	rt.AssignColumn(sy, makeVec(20, 20, 30, 10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// TestCompile_MultiColumn_PartialMatchFails sanity-checks the multi-column
// path: a witness whose tuple matches column-wise but not pair-wise (e.g.
// (1,20) is not a row of T) must be rejected at M-assignment time.
func TestCompile_MultiColumn_PartialMatchFails(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-col-bad")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)
	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
	rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
	// (1, 20) — never a row of T.
	rt.AssignColumn(sx, makeVec(1))
	rt.AssignColumn(sy, makeVec(20))

	assert.Panics(t, func() { runRound(rt) },
		"multi-column lookup must reject a tuple that does not appear in T")
}

// ---- Multiple queries sharing the same B ----

func TestCompile_MultipleQueriesSameTable(t *testing.T) {
	sys := wiop.NewSystemf("ll-shared-T")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 4, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)

	tabT := wiop.NewTable(colT.View())
	sys.NewInclusion(sys.Context.Childf("inc1"),
		[]wiop.Table{wiop.NewTable(colS1.View())}, []wiop.Table{tabT})
	sys.NewInclusion(sys.Context.Childf("inc2"),
		[]wiop.Table{wiop.NewTable(colS2.View())}, []wiop.Table{tabT})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	// Exactly one M column should have been added to modT — both queries
	// share the same lookup table and therefore the same multiplicity column.
	assert.Len(t, modT.Columns, colsBefore+1,
		"a single shared lookup table must yield exactly one M column")
	for _, q := range sys.TableRelations {
		assert.True(t, q.IsReduced(),
			"every consumed inclusion query must be marked reduced")
	}
	// Exactly one LogDerivativeSum query is registered, regardless of how
	// many inclusion queries were merged.
	assert.Len(t, sys.LogDerivativeSums, 1,
		"every inclusion query must be folded into a single aggregated LogDerivativeSum")

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS1, makeVec(10, 20, 10, 30))
	rt.AssignColumn(colS2, makeVec(40, 30))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

func TestCompile_MultipleQueriesDistinctTables(t *testing.T) {
	sys := wiop.NewSystemf("ll-distinct-T")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc1"),
		[]wiop.Table{wiop.NewTable(colS1.View())},
		[]wiop.Table{wiop.NewTable(colT1.View())})
	sys.NewInclusion(sys.Context.Childf("inc2"),
		[]wiop.Table{wiop.NewTable(colS2.View())},
		[]wiop.Table{wiop.NewTable(colT2.View())})

	colsT1Before := len(modT1.Columns)
	colsT2Before := len(modT2.Columns)
	lookuptologderivsum.Compile(sys)
	assert.Len(t, modT1.Columns, colsT1Before+1,
		"modT1 must carry exactly one new M column")
	assert.Len(t, modT2.Columns, colsT2Before+1,
		"modT2 must carry exactly one new M column")
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT1, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colT2, makeVec(100, 200))
	rt.AssignColumn(colS1, makeVec(10, 30))
	rt.AssignColumn(colS2, makeVec(100, 200))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// ---- Idempotence and edge cases ----

func TestCompile_NoInclusions(t *testing.T) {
	sys := wiop.NewSystemf("ll-empty")
	sys.NewRound()
	lookuptologderivsum.Compile(sys) // must not panic
	assert.Empty(t, sys.LogDerivativeSums,
		"compile without inclusion queries must register no LogDerivativeSum")
}

// TestCompile_MultiFragmentB checks that a lookup whose target is the union of
// two fragments (living on different modules) compiles, allocates one M column
// per fragment, and proves/verifies when every S value appears in the union.
func TestCompile_MultiFragmentB(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-frag-B")
	r0 := sys.NewRound()
	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
		})

	colsT1Before := len(modT1.Columns)
	colsT2Before := len(modT2.Columns)
	lookuptologderivsum.Compile(sys)
	assert.Len(t, modT1.Columns, colsT1Before+1,
		"modT1 must carry exactly one new M column")
	assert.Len(t, modT2.Columns, colsT2Before+1,
		"modT2 must carry exactly one new M column")
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT1, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colT2, makeVec(100, 200))
	// Every S value is present in the union T1 ∪ T2; values are drawn from
	// both fragments to exercise per-fragment M assignment.
	rt.AssignColumn(colS, makeVec(10, 30, 100, 200))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// TestCompile_MultiFragmentBothSides checks a single inclusion whose A side and
// B side are BOTH unions of two fragments living on different modules. This
// crosses multi-fragment S (several included tables) with multi-fragment B (the
// per-fragment mValues matrix): one M column is allocated per B fragment, and
// every value drawn by either S fragment must resolve to some row of the T1 ∪ T2
// union. The values are spread across both fragments on each side so neither the
// A-side iteration nor the per-fragment M assignment is exercised on a single
// fragment alone.
func TestCompile_MultiFragmentBothSides(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-frag-both")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 4, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{
			wiop.NewTable(colS1.View()),
			wiop.NewTable(colS2.View()),
		},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
		})

	colsT1Before := len(modT1.Columns)
	colsT2Before := len(modT2.Columns)
	lookuptologderivsum.Compile(sys)
	assert.Len(t, modT1.Columns, colsT1Before+1,
		"modT1 must carry exactly one new M column")
	assert.Len(t, modT2.Columns, colsT2Before+1,
		"modT2 must carry exactly one new M column")
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Union T = T1 ∪ T2 = {10,20,30,40} ∪ {100,200}.
	rt.AssignColumn(colT1, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colT2, makeVec(100, 200))
	// Each S fragment draws from both B fragments; every value is in the union.
	// Honest M: modT1 → [1,1,1,1], modT2 → [1,1].
	rt.AssignColumn(colS1, makeVec(10, 30, 100, 20))
	rt.AssignColumn(colS2, makeVec(200, 40))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// ---- Soundness: verifier rejects an incorrect multiplicity column ----

// TestCompile_VerifierFailsOnZeroM exercises the resultIsZeroVerifierAction by
// bypassing the M-assignment prover task and pinning M to all zeros instead.
// Every selected A row contributes 1/(γ + RLC(S_j)) to the LogDerivativeSum
// while the B side contributes nothing, so the aggregated result is non-zero
// with overwhelming probability over γ and the verifier must reject.
func TestCompile_VerifierFailsOnZeroM(t *testing.T) {
	sys := wiop.NewSystemf("ll-zero-M")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	require.Len(t, modT.Columns, colsBefore+1,
		"lookuptologderivsum.Compile must add exactly one M column to modT")
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	// Honest witness: every S row appears in T (correct M would be [2,1,1,0]).
	rt.AssignColumn(colS, makeVec(10, 20, 10, 30))
	// Cheat: assign M directly with the wrong value, skipping the prover task.
	rt.AssignColumn(mCol, makeVec(0, 0, 0, 0))

	rt.AdvanceRound() // → coin round, samples α/γ
	rt.AdvanceRound() // → result round
	runRound(rt)      // assigns Z and the LogDerivativeSum result

	err := checkAllVerifierActions(rt)
	assert.ErrorContains(t, err, "must be zero",
		"verifier must reject when M is left at zero despite active A rows")
}

// TestCompile_VerifierFailsOnInflatedM is a sharper variant of the previous
// test: M differs from the honest count by a single increment on one row.
// The aggregated result is then exactly the extra fraction emitted on the
// B side, which is non-zero with overwhelming probability. This pins down
// that the verifier does not merely catch grossly-wrong M but any deviation
// from the honest multiplicity.
func TestCompile_VerifierFailsOnInflatedM(t *testing.T) {
	sys := wiop.NewSystemf("ll-inflated-M")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS, makeVec(10, 20, 10, 30))
	// Honest M would be [2,1,1,0]; we inflate row 3 to claim T[3]=40 was
	// looked up once even though no S row references it.
	rt.AssignColumn(mCol, makeVec(2, 1, 1, 1))

	rt.AdvanceRound()
	rt.AdvanceRound()
	runRound(rt)

	err := checkAllVerifierActions(rt)
	assert.ErrorContains(t, err, "must be zero",
		"verifier must reject any deviation from the honest multiplicity, however small")
}

// ---- Non-binary filter rejection ----

// TestCompile_NonBinaryIncludedFilterPanics covers the guard inside the
// M-assignment task that rejects A-side selectors carrying values other than
// 0 or 1. The reduction treats the filter as a 0/1 mask (M is incremented by
// one per active row), so any other value would silently break the
// honest-prover identity. The task aborts early instead.
func TestCompile_NonBinaryIncludedFilterPanics(t *testing.T) {
	sys := wiop.NewSystemf("ll-nonbin-filter")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	filterS := modS.NewColumn(sys.Context.Childf("filterS"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewFilteredTable(filterS.View(), colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Every S value is in T, so the only failure mode is the non-binary
	// filter entry on row 1.
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
	rt.AssignColumn(filterS, makeVec(1, 7, 1, 1))

	assert.Panics(t, func() { runRound(rt) },
		"non-binary included filter must be rejected by M assignment")
}

// ---- Multi-column with B-side filter (prepend trick × α-RLC) ----

// TestCompile_MultiColumn_FilterOnIncluding covers the cross between the
// IsFilteredOnIncluding prepend trick and the α-RLC needed for multi-column
// lookups. With width-2 columns plus a prepended selector, the effective
// row width is 3, so α is sampled and both the prepend and the RLC must
// agree between prover (rowHash) and verifier (symbolic LogDerivativeSum)
// for the identity to close.
func TestCompile_MultiColumn_FilterOnIncluding(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-filterT")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), tx.View(), ty.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Table rows: (1,10) selected, (99,99) masked, (2,20) selected, (3,30) selected.
	// S references only the three selected rows, with (1,10) drawn twice.
	rt.AssignColumn(tx, makeVec(1, 99, 2, 3))
	rt.AssignColumn(ty, makeVec(10, 99, 20, 30))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	rt.AssignColumn(sx, makeVec(1, 2, 3, 1))
	rt.AssignColumn(sy, makeVec(10, 20, 30, 10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// TestCompile_MultiColumn_FilterOnIncluding_MaskedRowFails pairs the
// happy-path test above with the soundness case: an S tuple that matches a
// *masked-out* B tuple column-wise must be rejected. The B selector
// prepended into the hash makes the masked B row's hash carry a 0 head,
// while every A row carries a 1 head — so the M-assignment task reports
// no match instead of silently incrementing M on the filtered-out row.
func TestCompile_MultiColumn_FilterOnIncluding_MaskedRowFails(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-filterT-mask")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 1, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), tx.View(), ty.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(tx, makeVec(1, 99, 2, 3))
	rt.AssignColumn(ty, makeVec(10, 99, 20, 30))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	// S = (99, 99) tries to match the masked-out B tuple.
	rt.AssignColumn(sx, makeVec(99))
	rt.AssignColumn(sy, makeVec(99))

	assert.Panics(t, func() { runRound(rt) },
		"masked-out multi-column B row must not be reachable from an active A row")
}

// TestCompile_MultiColumn_FilterOnIncluding_InvalidColumnsFails is the third
// counterpart to TestCompile_MultiColumn_FilterOnIncluding: where the
// masked-row variant exercises the prepend trick by trying to hit a
// filtered-out B row, this variant exercises baseline correctness — the S
// tuple simply does not appear in T at all (not even as a masked row).
// M-assignment must reject; if it did not, the lookup would silently
// validate witnesses outside the table.
func TestCompile_MultiColumn_FilterOnIncluding_InvalidColumnsFails(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-filterT-invalid")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), tx.View(), ty.View())},
	)
	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Same table as the happy path: selected rows are (1,10), (2,20), (3,30);
	// (99,99) is masked.
	rt.AssignColumn(tx, makeVec(1, 99, 2, 3))
	rt.AssignColumn(ty, makeVec(10, 99, 20, 30))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	// All S tuples but one are valid; row 2 = (7, 70) is not in T (neither
	// selected nor masked) so no B-row hash can match it.
	rt.AssignColumn(sx, makeVec(1, 2, 7, 3))
	rt.AssignColumn(sy, makeVec(10, 20, 70, 30))

	assert.Panics(t, func() { runRound(rt) },
		"multi-column lookup with B-filter must reject an S tuple absent from T")
}

// ---- Canonical column ordering (query deduplication) ----

// TestCompile_ScrambledColumnOrder_SharesGroup builds two inclusion queries
// against the same two-column table T, the second one listing the columns in
// the opposite order (and its A columns permuted conjointly). The compiler
// must canonicalize the column order before bucketing so both queries share
// one group — a single M column on the table module and a single α — and the
// end-to-end protocol must still accept an honest witness.
func TestCompile_ScrambledColumnOrder_SharesGroup(t *testing.T) {
	sys := wiop.NewSystemf("ll-scrambled")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modSa := sys.NewSizedModule(sys.Context.Childf("modSa"), 4, wiop.PaddingDirectionNone)
	modSb := sys.NewSizedModule(sys.Context.Childf("modSb"), 4, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sax := modSa.NewColumn(sys.Context.Childf("Sax"), wiop.VisibilityOracle, r0)
	say := modSa.NewColumn(sys.Context.Childf("Say"), wiop.VisibilityOracle, r0)
	sbx := modSb.NewColumn(sys.Context.Childf("Sbx"), wiop.VisibilityOracle, r0)
	sby := modSb.NewColumn(sys.Context.Childf("Sby"), wiop.VisibilityOracle, r0)

	// Query 1: (Sax, Say) ⊆ (Tx, Ty).
	sys.NewInclusion(
		sys.Context.Childf("incA"),
		[]wiop.Table{wiop.NewTable(sax.View(), say.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)
	// Query 2: same table, columns scrambled — (Sby, Sbx) ⊆ (Ty, Tx).
	sys.NewInclusion(
		sys.Context.Childf("incB"),
		[]wiop.Table{wiop.NewTable(sby.View(), sbx.View())},
		[]wiop.Table{wiop.NewTable(ty.View(), tx.View())},
	)

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	assert.Len(t, modT.Columns, colsBefore+1,
		"queries over the same table with scrambled column order must share one M column")

	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// T pairs: (1,10), (2,20), (3,30), (4,40).
	rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
	rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
	// Query 1 pairs: (1,10), (2,20), (1,10), (3,30) — all in T.
	rt.AssignColumn(sax, makeVec(1, 2, 1, 3))
	rt.AssignColumn(say, makeVec(10, 20, 10, 30))
	// Query 2 pairs (as (x,y)): (2,20), (4,40), (2,20), (2,20) — all in T.
	rt.AssignColumn(sbx, makeVec(2, 4, 2, 2))
	rt.AssignColumn(sby, makeVec(20, 40, 20, 20))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))
}

// TestCompile_ScrambledColumnOrder_StillSound scrambles the column order the
// same way but assigns query 2 an S tuple whose components are swapped
// relative to T's pairing — (10, 1) instead of (1, 10). If canonicalization
// mis-paired the A columns against the reordered B columns this witness would
// pass; instead M-assignment must reject it.
func TestCompile_ScrambledColumnOrder_StillSound(t *testing.T) {
	sys := wiop.NewSystemf("ll-scrambled-sound")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modSa := sys.NewSizedModule(sys.Context.Childf("modSa"), 2, wiop.PaddingDirectionNone)
	modSb := sys.NewSizedModule(sys.Context.Childf("modSb"), 2, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sax := modSa.NewColumn(sys.Context.Childf("Sax"), wiop.VisibilityOracle, r0)
	say := modSa.NewColumn(sys.Context.Childf("Say"), wiop.VisibilityOracle, r0)
	sbx := modSb.NewColumn(sys.Context.Childf("Sbx"), wiop.VisibilityOracle, r0)
	sby := modSb.NewColumn(sys.Context.Childf("Sby"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(
		sys.Context.Childf("incA"),
		[]wiop.Table{wiop.NewTable(sax.View(), say.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())},
	)
	sys.NewInclusion(
		sys.Context.Childf("incB"),
		[]wiop.Table{wiop.NewTable(sby.View(), sbx.View())},
		[]wiop.Table{wiop.NewTable(ty.View(), tx.View())},
	)

	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(tx, makeVec(1, 2, 3, 4))
	rt.AssignColumn(ty, makeVec(10, 20, 30, 40))
	rt.AssignColumn(sax, makeVec(1, 2))
	rt.AssignColumn(say, makeVec(10, 20))
	// (x,y) = (10,1): the components of a valid pair, swapped. Not in T.
	rt.AssignColumn(sbx, makeVec(10, 2))
	rt.AssignColumn(sby, makeVec(1, 20))

	assert.Panics(t, func() { runRound(rt) },
		"a swapped-component S tuple must still be rejected after canonicalization")
}

// ---- Fragment counts beyond two ----
//
// The multi-fragment tests above all use exactly two fragments, which cannot
// distinguish "one M per fragment" from "one M per module" nor exercise the
// three-way tie-break in the hash join. The tests below vary the fragment
// count on each side independently: the two counts are unrelated, since
// len(includings) counts B fragments of the union lookup table while
// len(included) counts A fragments (one per included table).

// readM returns the multiplicity column's assignment as uint64s. It fails the
// test if the column was never assigned or came back as an extension vector,
// neither of which the M-assignment task should ever produce.
func readM(t *testing.T, rt *wiop.Runtime, mCol *wiop.Column) []uint64 {
	t.Helper()
	require.True(t, rt.HasColumnAssignment(mCol),
		"M column %v must be assigned by the M-assignment task", mCol.Context.Path())
	plain := rt.GetColumnAssignment(mCol).Plain
	require.True(t, plain.IsBase(), "M must be a base-field column")
	base := plain.AsBase()
	out := make([]uint64, len(base))
	for i := range base {
		out[i] = base[i].Uint64()
	}
	return out
}

// TestCompile_ThreeFragmentB_SingleFragmentA uses a three-fragment lookup table
// of three *different* heights against a single included table. Each fragment
// must receive its own M column, sized to that fragment's module, and the
// per-fragment multiplicities must be counted independently.
func TestCompile_ThreeFragmentB_SingleFragmentA(t *testing.T) {
	sys := wiop.NewSystemf("ll-three-frag-B")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modT3 := sys.NewSizedModule(sys.Context.Childf("modT3"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 8, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colT3 := modT3.NewColumn(sys.Context.Childf("T3"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
			wiop.NewTable(colT3.View()),
		})

	t1Before, t2Before, t3Before := len(modT1.Columns), len(modT2.Columns), len(modT3.Columns)
	lookuptologderivsum.Compile(sys)
	require.Len(t, modT1.Columns, t1Before+1, "modT1 must carry exactly one new M column")
	require.Len(t, modT2.Columns, t2Before+1, "modT2 must carry exactly one new M column")
	require.Len(t, modT3.Columns, t3Before+1, "modT3 must carry exactly one new M column")
	m1, m2, m3 := modT1.Columns[t1Before], modT2.Columns[t2Before], modT3.Columns[t3Before]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Union T = {10,20,30,40} ∪ {100,200} ∪ {1000,2000,3000,4000}; all distinct.
	rt.AssignColumn(colT1, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colT2, makeVec(100, 200))
	rt.AssignColumn(colT3, makeVec(1000, 2000, 3000, 4000))
	// S draws from all three fragments, with 10 twice and 3000 twice.
	rt.AssignColumn(colS, makeVec(10, 10, 200, 3000, 3000, 30, 4000, 100))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	// Every entry is unique across the union, so the counts are unambiguous —
	// no tie-break is involved here.
	assert.Equal(t, []uint64{2, 0, 1, 0}, readM(t, rt, m1), "M for fragment 0")
	assert.Equal(t, []uint64{1, 1}, readM(t, rt, m2), "M for fragment 1")
	assert.Equal(t, []uint64{0, 0, 2, 1}, readM(t, rt, m3), "M for fragment 2")
}

// TestCompile_AsymmetricFragmentCounts pairs a three-fragment B side with a
// two-fragment A side, so len(includings) != len(included). The M-assignment
// task indexes mValues by B fragment and iterates A fragments separately; a
// confusion between the two indices would go unnoticed whenever the counts
// happen to be equal, as they are in every pre-existing multi-fragment test.
func TestCompile_AsymmetricFragmentCounts(t *testing.T) {
	sys := wiop.NewSystemf("ll-asym-frag")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 2, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 4, wiop.PaddingDirectionNone)
	modT3 := sys.NewSizedModule(sys.Context.Childf("modT3"), 2, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 4, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colT3 := modT3.NewColumn(sys.Context.Childf("T3"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{
			wiop.NewTable(colS1.View()),
			wiop.NewTable(colS2.View()),
		},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
			wiop.NewTable(colT3.View()),
		})

	t1Before, t2Before, t3Before := len(modT1.Columns), len(modT2.Columns), len(modT3.Columns)
	lookuptologderivsum.Compile(sys)
	require.Len(t, modT1.Columns, t1Before+1)
	require.Len(t, modT2.Columns, t2Before+1)
	require.Len(t, modT3.Columns, t3Before+1)
	m1, m2, m3 := modT1.Columns[t1Before], modT2.Columns[t2Before], modT3.Columns[t3Before]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT1, makeVec(10, 20))
	rt.AssignColumn(colT2, makeVec(30, 40, 50, 80))
	rt.AssignColumn(colT3, makeVec(60, 70))
	// Both A fragments draw from several B fragments each.
	rt.AssignColumn(colS1, makeVec(10, 40, 70, 50))
	rt.AssignColumn(colS2, makeVec(30, 10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	assert.Equal(t, []uint64{2, 0}, readM(t, rt, m1), "M for fragment 0")
	assert.Equal(t, []uint64{1, 1, 1, 0}, readM(t, rt, m2), "M for fragment 1")
	assert.Equal(t, []uint64{0, 1}, readM(t, rt, m3), "M for fragment 2")
}

// TestCompile_SingleFragmentB_ThreeFragmentA is the mirror image: one B
// fragment but three A fragments. The number of M columns tracks the B
// fragment count only, so exactly one M must be allocated no matter how many
// included tables feed into it.
func TestCompile_SingleFragmentB_ThreeFragmentA(t *testing.T) {
	sys := wiop.NewSystemf("ll-one-B-three-A")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
	modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 4, wiop.PaddingDirectionNone)
	modS3 := sys.NewSizedModule(sys.Context.Childf("modS3"), 1, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
	colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
	colS3 := modS3.NewColumn(sys.Context.Childf("S3"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{
			wiop.NewTable(colS1.View()),
			wiop.NewTable(colS2.View()),
			wiop.NewTable(colS3.View()),
		},
		[]wiop.Table{wiop.NewTable(colT.View())})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	require.Len(t, modT.Columns, colsBefore+1,
		"the M column count tracks B fragments, not A fragments")
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colT, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colS1, makeVec(10, 20))
	rt.AssignColumn(colS2, makeVec(30, 10, 30, 40))
	rt.AssignColumn(colS3, makeVec(10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	// 10 appears three times across the three A fragments, 20 once, 30 twice,
	// 40 once.
	assert.Equal(t, []uint64{3, 1, 2, 1}, readM(t, rt, mCol))
}

// ---- Repeated entries in the lookup table ----
//
// A value may legitimately occur at several rows of the lookup table, within
// one fragment or across fragments. Those rows produce LogDerivativeSum terms
// with the *identical* denominator γ + RLC(v), so their numerators simply add:
// any split of the total count across the duplicate rows satisfies the
// identity, and the verifier accepts all of them. The prover nevertheless
// picks one deterministically — the latest occurrence by (fragment, row) — and
// charges the whole count there, leaving the other copies at zero. The tests
// below assert that documented convention exactly, so a change to the
// tie-break shows up as a test failure rather than as a silent difference in
// prover output; they also check the verifier still accepts, which is the part
// that actually matters for soundness.

// TestCompile_DuplicateTEntriesWithinFragment has one fragment holding the
// same value at two rows. The whole multiplicity lands on the higher row.
func TestCompile_DuplicateTEntriesWithinFragment(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-within-frag")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// 10 occurs at rows 0 and 1; 30 occurs at rows 2 and 3.
	rt.AssignColumn(colT, makeVec(10, 10, 30, 30))
	rt.AssignColumn(colS, makeVec(10, 30, 10, 10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	// 10 was looked up three times and 30 once; each count is charged to the
	// later of the two duplicate rows, so rows 0 and 2 stay at zero.
	assert.Equal(t, []uint64{0, 3, 0, 1}, readM(t, rt, mCol))
}

// TestCompile_DuplicateTEntriesAcrossFragments places the same value in two
// different fragments. The tie-break compares fragment index first, so the
// count goes to the higher-indexed fragment — even though its row index is
// lower than the other copy's.
func TestCompile_DuplicateTEntriesAcrossFragments(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-across-frag")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
		})

	t1Before, t2Before := len(modT1.Columns), len(modT2.Columns)
	lookuptologderivsum.Compile(sys)
	m1, m2 := modT1.Columns[t1Before], modT2.Columns[t2Before]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// 10 lives at (frag 0, row 3) and at (frag 1, row 0).
	rt.AssignColumn(colT1, makeVec(20, 30, 40, 10))
	rt.AssignColumn(colT2, makeVec(10, 50))
	rt.AssignColumn(colS, makeVec(10, 10, 10, 50))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	// Fragment index dominates the tie-break: all three lookups of 10 are
	// charged to fragment 1 row 0, and fragment 0 row 3 stays at zero.
	assert.Equal(t, []uint64{0, 0, 0, 0}, readM(t, rt, m1), "M for fragment 0")
	assert.Equal(t, []uint64{3, 1}, readM(t, rt, m2), "M for fragment 1")
}

// TestCompile_DuplicateTEntriesAcrossThreeFragments repeats one value in all
// three fragments at once. Only the last fragment's copy carries the count,
// which distinguishes a genuine max over the fragment index from a pairwise
// comparison that could settle on a middle fragment.
func TestCompile_DuplicateTEntriesAcrossThreeFragments(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-three-frag")
	r0 := sys.NewRound()

	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 2, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 2, wiop.PaddingDirectionNone)
	modT3 := sys.NewSizedModule(sys.Context.Childf("modT3"), 2, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)

	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colT3 := modT3.NewColumn(sys.Context.Childf("T3"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
			wiop.NewTable(colT3.View()),
		})

	t1Before, t2Before, t3Before := len(modT1.Columns), len(modT2.Columns), len(modT3.Columns)
	lookuptologderivsum.Compile(sys)
	m1, m2, m3 := modT1.Columns[t1Before], modT2.Columns[t2Before], modT3.Columns[t3Before]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// 7 appears at row 0 of every fragment; the second row differs per fragment.
	rt.AssignColumn(colT1, makeVec(7, 11))
	rt.AssignColumn(colT2, makeVec(7, 12))
	rt.AssignColumn(colT3, makeVec(7, 13))
	rt.AssignColumn(colS, makeVec(7, 7))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	assert.Equal(t, []uint64{0, 0}, readM(t, rt, m1), "M for fragment 0")
	assert.Equal(t, []uint64{0, 0}, readM(t, rt, m2), "M for fragment 1")
	assert.Equal(t, []uint64{2, 0}, readM(t, rt, m3), "M for fragment 2")
}

// TestCompile_DuplicateTEntries_Unreferenced covers duplicate table rows that
// no A row ever looks up. Both copies must stay at zero; a stray increment
// would make the aggregated sum non-zero and the verifier reject.
func TestCompile_DuplicateTEntries_Unreferenced(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-unref")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// 99 is duplicated at rows 1 and 2 but never looked up.
	rt.AssignColumn(colT, makeVec(10, 99, 99, 20))
	rt.AssignColumn(colS, makeVec(10, 20))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	assert.Equal(t, []uint64{1, 0, 0, 1}, readM(t, rt, mCol))
}

// TestCompile_DuplicateTEntries_MultiColumn repeats a whole *tuple* rather than
// a single value, so the duplicate is only visible after the RLC collapse. The
// same tie-break applies to the collapsed hash.
func TestCompile_DuplicateTEntries_MultiColumn(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-multicol")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)

	tx := modT.NewColumn(sys.Context.Childf("Tx"), wiop.VisibilityOracle, r0)
	ty := modT.NewColumn(sys.Context.Childf("Ty"), wiop.VisibilityOracle, r0)
	sx := modS.NewColumn(sys.Context.Childf("Sx"), wiop.VisibilityOracle, r0)
	sy := modS.NewColumn(sys.Context.Childf("Sy"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(sx.View(), sy.View())},
		[]wiop.Table{wiop.NewTable(tx.View(), ty.View())})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// Tuples: (1,10), (2,20), (1,10), (3,30) — (1,10) is duplicated at rows
	// 0 and 2. Note rows 0 and 1 share no *component*, so a per-column
	// comparison would not see the duplicate; only the collapsed tuple does.
	rt.AssignColumn(tx, makeVec(1, 2, 1, 3))
	rt.AssignColumn(ty, makeVec(10, 20, 10, 30))
	// S tuples: (1,10), (1,10), (3,30), (2,20).
	rt.AssignColumn(sx, makeVec(1, 1, 3, 2))
	rt.AssignColumn(sy, makeVec(10, 10, 30, 20))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	assert.Equal(t, []uint64{0, 1, 2, 1}, readM(t, rt, mCol))
}

// TestCompile_DuplicateTEntries_FilteredCopy is the discriminating case for the
// interaction between duplicates and the IsFilteredOnIncluding trick: the same
// value sits at two rows and the *later* one is masked out. Because the
// selector is prepended into the hashed value, the masked row collapses to a
// different value and never enters the duplicate group at all — so the count
// must land on the earlier, active row. A tie-break applied before the
// selector was folded in would instead pick the masked row, leaving the active
// row at zero and the aggregated sum non-zero.
func TestCompile_DuplicateTEntries_FilteredCopy(t *testing.T) {
	sys := wiop.NewSystemf("ll-dup-filtered-copy")
	r0 := sys.NewRound()

	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)

	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	filterT := modT.NewColumn(sys.Context.Childf("filterT"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewFilteredTable(filterT.View(), colT.View())})

	colsBefore := len(modT.Columns)
	lookuptologderivsum.Compile(sys)
	mCol := modT.Columns[colsBefore]
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	// 10 occurs at rows 0 and 1, but row 1 is masked out by filterT.
	rt.AssignColumn(colT, makeVec(10, 10, 30, 40))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	rt.AssignColumn(colS, makeVec(10, 10))

	driveProtocol(rt)
	require.NoError(t, checkAllVerifierActions(rt))

	assert.Equal(t, []uint64{2, 0, 0, 0}, readM(t, rt, mCol),
		"the count must go to the active copy, not the later masked one")
}
