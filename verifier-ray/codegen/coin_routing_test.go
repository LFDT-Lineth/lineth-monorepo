package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
)

// The spec carries prover-ray's Module.MinRuntimeSize for each dynamic module,
// in module_sizes order (sys.Modules order, sized modules skipped): the
// smallest power of two N > 2·max|shift| over the module's openings.
func TestBuildCoinRoutingDynamicModuleMinSizes(t *testing.T) {
	sys := wiop.NewSystemf("dynamic-min-sizes")
	r0 := sys.NewRound()

	fib := sys.NewDynamicModule(sys.Context.Childf("fib"), wiop.PaddingDirectionRight)
	f := fib.NewColumn(sys.Context.Childf("f"), r0)
	fib.NewVanishing(sys.Context.Childf("fib"),
		wiop.Sub(wiop.Sub(f.View(), f.View().Shift(-1)), f.View().Shift(-2)))

	sized := sys.NewSizedModule(sys.Context.Childf("sized"), 8, wiop.PaddingDirectionRight)
	s := sized.NewColumn(sys.Context.Childf("s"), r0)
	sized.NewVanishing(sys.Context.Childf("s-shift"), wiop.Sub(s.View(), s.View().Shift(3)))

	flat := sys.NewDynamicModule(sys.Context.Childf("flat"), wiop.PaddingDirectionRight)
	a := flat.NewColumn(sys.Context.Childf("a"), r0)
	b := flat.NewColumn(sys.Context.Childf("b"), r0)
	flat.NewVanishing(sys.Context.Childf("eq"), wiop.Sub(a.View(), b.View()))

	global.Compile(sys)

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	if routing.DynamicModuleCount != 2 {
		t.Fatalf("DynamicModuleCount = %d, want 2", routing.DynamicModuleCount)
	}
	// fib opens at shifts 0, -1, -2: N > 4, so 8. flat opens at shift 0 only.
	want := []int{8, 1}
	if len(routing.DynamicModuleMinSizes) != len(want) {
		t.Fatalf("DynamicModuleMinSizes = %v, want %v", routing.DynamicModuleMinSizes, want)
	}
	for i := range want {
		if routing.DynamicModuleMinSizes[i] != want[i] {
			t.Fatalf("DynamicModuleMinSizes = %v, want %v", routing.DynamicModuleMinSizes, want)
		}
	}

	var buf bytes.Buffer
	if err := WriteSpecZig(&buf, routing); err != nil {
		t.Fatalf("WriteSpecZig() error = %v", err)
	}
	if out := buf.String(); !strings.Contains(out, ".dynamic_module_min_sizes = &[_]usize{ 8, 1 },") {
		t.Fatalf("generated spec missing dynamic_module_min_sizes:\n%s", out)
	}
}
