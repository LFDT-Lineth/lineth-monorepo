package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
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
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	one := wiop.NewConstantVector(mod, field.NewFromString("1"))
	sys.NewLogDerivativeSum(sys.Context.Childf("ld"), []wiop.Fraction{
		{Numerator: col.View(), Denominator: one},
	})
	logderivativesum.Compile(sys)
	global.Compile(sys) // appends quotient+eval rounds so no cell lands in the last round
	return sys, col
}

func TestBuildLogDerivSystemExtractsQuery(t *testing.T) {
	sys, _ := newSingleFractionLDS(t)

	ld, err := BuildLogDerivSystem(sys)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(ld.Queries))
	}
	q := ld.Queries[0]
	if len(q.Entries) != 1 {
		t.Fatalf("a single fraction packs into one Z column, got %d entries", len(q.Entries))
	}
	if want := (ModuleSize{StaticSize: 4}); q.Entries[0].Size != want {
		t.Fatalf("the Z column's module is static of size 4: got size %+v, want %+v", q.Entries[0].Size, want)
	}
	if q.ResultIsZero {
		t.Fatalf("a plain LogDerivativeSum query must not be marked result-is-zero")
	}
}

func TestWriteLogDerivSystemZigRendersQuery(t *testing.T) {
	sys, _ := newSingleFractionLDS(t)
	ld, err := BuildLogDerivSystem(sys)
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
		"system_0_logderiv_query_0_entries = [_]logderivativesum.Entry{",
		".size = .{ .static = 4 }",
		"system_0_logderiv_queries = [_]logderivativesum.Query{",
		".result_ref = .{",
		".result_is_zero = false",
		"const system_0_logderiv = logderivativesum.System{ .queries = &system_0_logderiv_queries };",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, got)
		}
	}
	// No concrete field values should be emitted for averages or result.
	if strings.Contains(got, "[_][6]u32{") {
		t.Fatalf("generated Zig must not emit concrete [6]u32 arrays:\n%s", got)
	}
}

func TestBuildLogDerivSystemMarksLookupResultZero(t *testing.T) {
	// A single-column inclusion lookup reduces to a LogDerivativeSum whose
	// Result must be zero; the extracted query must carry ResultIsZero.
	sys := wiop.NewSystemf("lk-codegen")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
	colT := modT.NewColumn(sys.Context.Childf("T"), r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)

	lookuptologderivsum.Compile(sys)
	logderivativesum.Compile(sys)
	global.Compile(sys)

	ld, err := BuildLogDerivSystem(sys)
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

// TestBuildLogDerivSystemDynamicModuleSize: the column total is n·C with n a
// dynamic module's runtime size, so the entry must reference the
// proof-supplied size by the module's dynamic index, not a static size. Two
// dynamic modules are declared and only the second one is in the sum, so the
// expected index is the literal 1: a mix-up in the dynamic indices fails.
func TestBuildLogDerivSystemDynamicModuleSize(t *testing.T) {
	sys := wiop.NewSystemf("ld-codegen-dyn")
	r0 := sys.NewRound()
	sys.NewRound()
	static := sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	unused := sys.NewDynamicModule(sys.Context.Childf("dyn0"), wiop.PaddingDirectionRight)
	unused.NewColumn(sys.Context.Childf("u"), r0)
	dyn := sys.NewDynamicModule(sys.Context.Childf("dyn1"), wiop.PaddingDirectionRight)
	colS := static.NewColumn(sys.Context.Childf("s"), r0)
	colD := dyn.NewColumn(sys.Context.Childf("d"), r0)
	sys.NewLogDerivativeSum(sys.Context.Childf("ld"), []wiop.Fraction{
		{Numerator: colS.View(), Denominator: wiop.NewConstantVector(static, field.NewFromString("1"))},
		{Numerator: colD.View(), Denominator: wiop.NewConstantVector(dyn, field.NewFromString("1"))},
	})
	logderivativesum.Compile(sys)
	global.Compile(sys)

	ld, err := BuildLogDerivSystem(sys)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 1 || len(ld.Queries[0].Entries) != 2 {
		t.Fatalf("want one query with one Z column per module, got %+v", ld.Queries)
	}
	want := ModuleSize{Dynamic: true, DynamicIndex: 1}
	if got := ld.Queries[0].Entries[1].Size; got != want {
		t.Fatalf("the dynamic module's Z column must use its proof-supplied size: got %+v, want %+v", got, want)
	}
}

func TestBuildLogDerivSystemNoQueries(t *testing.T) {
	sys := wiop.NewSystemf("ld-none")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	ld, err := BuildLogDerivSystem(sys)
	if err != nil {
		t.Fatalf("BuildLogDerivSystem() error = %v", err)
	}
	if len(ld.Queries) != 0 {
		t.Fatalf("a system without LogDerivativeSum queries must yield no queries, got %d", len(ld.Queries))
	}
}
