package lookuptologderivsum_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/wioptest"
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
			proof := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof),
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
			sc.AssignWitness(&rt)

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
			runRound(&rt)     // assigns Z and the LDS result

			err := checkAllVerifierActions(&rt)
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
		a.Run(*rt)
	}
}

// checkAllVerifierActions evaluates every verifier action across every round
// of the runtime. Returns the first non-nil error or nil if everything
// passes.
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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
		runRound(&rt) // round 0 — M assignment task
	}, "M assignment must panic when an active A row has no match in B")
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	assert.Panics(t, func() { runRound(&rt) },
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	assert.Panics(t, func() { runRound(&rt) },
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
}

// ---- Idempotence and edge cases ----

func TestCompile_NoInclusions(t *testing.T) {
	sys := wiop.NewSystemf("ll-empty")
	sys.NewRound()
	lookuptologderivsum.Compile(sys) // must not panic
	assert.Empty(t, sys.LogDerivativeSums,
		"compile without inclusion queries must register no LogDerivativeSum")
}

func TestCompile_MultiFragmentBPanics(t *testing.T) {
	sys := wiop.NewSystemf("ll-multi-frag-B")
	r0 := sys.NewRound()
	modT1 := sys.NewSizedModule(sys.Context.Childf("modT1"), 4, wiop.PaddingDirectionNone)
	modT2 := sys.NewSizedModule(sys.Context.Childf("modT2"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 2, wiop.PaddingDirectionNone)
	colT1 := modT1.NewColumn(sys.Context.Childf("T1"), wiop.VisibilityOracle, r0)
	colT2 := modT2.NewColumn(sys.Context.Childf("T2"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)

	sys.NewInclusion(sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{
			wiop.NewTable(colT1.View()),
			wiop.NewTable(colT2.View()),
		})

	assert.Panics(t, func() { lookuptologderivsum.Compile(sys) },
		"multi-fragment lookup tables are out of scope for this MVP")
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
	runRound(&rt)     // assigns Z and the LogDerivativeSum result

	err := checkAllVerifierActions(&rt)
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
	runRound(&rt)

	err := checkAllVerifierActions(&rt)
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

	assert.Panics(t, func() { runRound(&rt) },
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
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 3, wiop.PaddingDirectionNone)

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
	// S references only the three selected rows.
	rt.AssignColumn(tx, makeVec(1, 99, 2, 3))
	rt.AssignColumn(ty, makeVec(10, 99, 20, 30))
	rt.AssignColumn(filterT, makeVec(1, 0, 1, 1))
	rt.AssignColumn(sx, makeVec(1, 2, 3))
	rt.AssignColumn(sy, makeVec(10, 20, 30))

	driveProtocol(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
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

	assert.Panics(t, func() { runRound(&rt) },
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
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 3, wiop.PaddingDirectionNone)

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
	// First two S tuples are valid; row 2 = (7, 70) is not in T (neither
	// selected nor masked) so no B-row hash can match it.
	rt.AssignColumn(sx, makeVec(1, 2, 7))
	rt.AssignColumn(sy, makeVec(10, 20, 70))

	assert.Panics(t, func() { runRound(&rt) },
		"multi-column lookup with B-filter must reject an S tuple absent from T")
}
