package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// newSingleFractionLDS builds a size-4 module with a single LogDerivativeSum
// query Σ col[i]/1, compiled to one Z column. It returns the system and the
// oracle column so callers can assign witnesses before running the prover.
func newSingleFractionLDS(t *testing.T) (*wiop.System, *wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("ld-codegen")
	r0 := sys.NewRound()
	sys.NewRound() // result round, following the column round
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	sys.NewLogDerivativeSum(sys.Context.Childf("ld"), []wiop.Fraction{
		{Numerator: col.View(), Denominator: one},
	})
	logderivativesum.Compile(sys)
	return sys, col
}

// runTestProver creates a runtime, assigns the given oracle columns, and
// advances through all rounds running every prover action.
func runTestProver(sys *wiop.System, assignments map[*wiop.Column][]uint64) wiop.Runtime {
	rt := wiop.NewRuntime(sys)
	for col, vals := range assignments {
		elems := make([]field.Element, len(vals))
		for i, v := range vals {
			elems[i].SetUint64(v)
		}
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})
	}
	for _, action := range rt.CurrentRound().ProverActions {
		action.Run(rt)
	}
	for rt.CurrentRound().ID < len(rt.System.Rounds)-1 {
		rt.AdvanceRound()
		for _, action := range rt.CurrentRound().ProverActions {
			action.Run(rt)
		}
	}
	return rt
}

func TestBuildLogDerivSystemExtractsQuery(t *testing.T) {
	sys, col := newSingleFractionLDS(t)
	rt := runTestProver(sys, map[*wiop.Column][]uint64{col: {1, 2, 3, 4}})

	ld, err := BuildLogDerivSystem(sys, rt)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(ld.Queries))
	}
	q := ld.Queries[0]
	if len(q.ZFinals) != 1 {
		t.Fatalf("a single fraction packs into one Z column, got %d z-finals", len(q.ZFinals))
	}
	if q.ResultIsZero {
		t.Fatalf("a plain LogDerivativeSum query must not be marked result-is-zero")
	}
}

func TestWriteLogDerivSystemZigRendersQuery(t *testing.T) {
	sys, col := newSingleFractionLDS(t)
	rt := runTestProver(sys, map[*wiop.Column][]uint64{col: {1, 2, 3, 4}})
	ld, err := BuildLogDerivSystem(sys, rt)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}

	var out bytes.Buffer
	if err := WriteLogDerivSystemZig(&out, 0, ld); err != nil {
		t.Fatalf("WriteLogDerivSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"const logderivativesum = @import",
		"system_0_logderiv_query_0_zfinals = [_][6]u32{",
		"system_0_logderiv_queries = [_]logderivativesum.Query{",
		".result_is_zero = false",
		"const system_0_logderiv = logderivativesum.System{ .queries = &system_0_logderiv_queries };",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, got)
		}
	}
}

func TestBuildLogDerivSystemMarksLookupResultZero(t *testing.T) {
	// A single-column inclusion lookup reduces to a LogDerivativeSum whose
	// Result must be zero; the extracted query must carry ResultIsZero.
	sys := wiop.NewSystemf("lk-codegen")
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

	// Assign S ⊆ T: all values from S appear in T.
	rt := runTestProver(sys, map[*wiop.Column][]uint64{
		colT: {1, 2, 3, 4},
		colS: {1, 1, 2, 3},
	})

	ld, err := BuildLogDerivSystem(sys, rt)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 1 {
		t.Fatalf("the lookup reduces to exactly one LogDerivativeSum query, got %d", len(ld.Queries))
	}
	if !ld.Queries[0].ResultIsZero {
		t.Fatalf("a lookup-reduced query must be marked result-is-zero")
	}
}

func TestBuildLogDerivSystemNoQueries(t *testing.T) {
	sys := wiop.NewSystemf("ld-none")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	rt := runTestProver(sys, nil)

	ld, err := BuildLogDerivSystem(sys, rt)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 0 {
		t.Fatalf("a system without LogDerivativeSum queries must yield no queries, got %d", len(ld.Queries))
	}
}
