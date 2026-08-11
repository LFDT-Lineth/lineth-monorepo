package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// newSingleLookup builds a size-4 inclusion S ⊆ T (one A fragment, one B
// fragment, both static) and runs the lookup-reduction compiler, registering
// exactly one RowLimitVerifierAction.
func newSingleLookup(t *testing.T) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("rl-codegen")
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
	return sys
}

func TestBuildRowLimitSystemExtractsCheck(t *testing.T) {
	sys := newSingleLookup(t)

	rl, err := BuildRowLimitSystem(sys)
	if err != nil {
		t.Fatalf("BuildRowLimitSystem() error = %v", err)
	}
	if len(rl.Checks) != 1 {
		t.Fatalf("expected exactly one check, got %d", len(rl.Checks))
	}
	check := rl.Checks[0]
	if len(check.IncludedModules) != 1 || check.IncludedModules[0].Dynamic || check.IncludedModules[0].StaticSize != 4 {
		t.Fatalf("expected one static included module of size 4, got %+v", check.IncludedModules)
	}
	if len(check.IncludingsModules) != 1 || check.IncludingsModules[0].Dynamic || check.IncludingsModules[0].StaticSize != 4 {
		t.Fatalf("expected one static includings module of size 4, got %+v", check.IncludingsModules)
	}
	if check.Limit != wiop.MaxLookupRows {
		t.Fatalf("expected limit %d, got %d", wiop.MaxLookupRows, check.Limit)
	}
}

func TestBuildRowLimitSystemNoLookups(t *testing.T) {
	sys := wiop.NewSystemf("rl-empty")
	sys.NewRound()

	rl, err := BuildRowLimitSystem(sys)
	if err != nil {
		t.Fatalf("BuildRowLimitSystem() error = %v", err)
	}
	if len(rl.Checks) != 0 {
		t.Fatalf("expected no checks for a system with no lookups, got %d", len(rl.Checks))
	}
}

func TestBuildRowLimitSystemDynamicModule(t *testing.T) {
	sys := wiop.NewSystemf("rl-dyn")
	r0 := sys.NewRound()
	modT := sys.NewSizedModule(sys.Context.Childf("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewDynamicModule(sys.Context.Childf("modS"), wiop.PaddingDirectionRight)
	colT := modT.NewColumn(sys.Context.Childf("T"), wiop.VisibilityOracle, r0)
	colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
	sys.NewInclusion(
		sys.Context.Childf("inc"),
		[]wiop.Table{wiop.NewTable(colS.View())},
		[]wiop.Table{wiop.NewTable(colT.View())},
	)
	lookuptologderivsum.Compile(sys)

	rl, err := BuildRowLimitSystem(sys)
	if err != nil {
		t.Fatalf("BuildRowLimitSystem() error = %v", err)
	}
	if len(rl.Checks) != 1 {
		t.Fatalf("expected exactly one check, got %d", len(rl.Checks))
	}
	check := rl.Checks[0]
	if len(check.IncludedModules) != 1 || !check.IncludedModules[0].Dynamic || check.IncludedModules[0].DynamicIndex != 0 {
		t.Fatalf("expected one dynamic included module at index 0, got %+v", check.IncludedModules)
	}
}

func TestWriteRowLimitSystemZigRendersCheck(t *testing.T) {
	sys := newSingleLookup(t)
	rl, err := BuildRowLimitSystem(sys)
	if err != nil {
		t.Fatalf("BuildRowLimitSystem() error = %v", err)
	}

	var out bytes.Buffer
	if err := WriteRowLimitSystemZig(&out, 0, rl); err != nil {
		t.Fatalf("WriteRowLimitSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"const rowlimit = @import",
		"system_0_rowlimit_check_0_included_modules = [_]rowlimit.ModuleSize{",
		".{ .static = 4 },",
		"system_0_rowlimit_checks = [_]rowlimit.Check{",
		".limit = 1073741824",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected generated Zig to contain %q, got:\n%s", want, got)
		}
	}
}
