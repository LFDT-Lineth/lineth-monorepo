package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/consensys/linea-monorepo/prover-ray/wiop/compilers/global"
)

func TestBuildVanishingSystemStaticModuleSize(t *testing.T) {
	sys := wiop.NewSystemf("static")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	mod.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()))

	global.Compile(sys)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	got, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildVanishingSystem() error = %v", err)
	}
	if len(got.Modules) != 1 {
		t.Fatalf("module count = %d, want 1", len(got.Modules))
	}
	if got.Modules[0].Size.Dynamic {
		t.Fatalf("static module exported as dynamic")
	}
	if got.Modules[0].Size.StaticSize != 8 {
		t.Fatalf("static size = %d, want 8", got.Modules[0].Size.StaticSize)
	}
}

func TestBuildVanishingSystemDynamicIndicesAreCompact(t *testing.T) {
	sys := wiop.NewSystemf("dynamic")
	r0 := sys.NewRound()
	staticMod := sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	dynA := sys.NewDynamicModule(sys.Context.Childf("dynA"), wiop.PaddingDirectionRight)
	dynB := sys.NewDynamicModule(sys.Context.Childf("dynB"), wiop.PaddingDirectionRight)
	staticCol := staticMod.NewColumn(sys.Context.Childf("staticCol"), wiop.VisibilityOracle, r0)
	colA := dynA.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	colB := dynB.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)
	staticMod.NewVanishing(sys.Context.Childf("staticBool"), wiop.Sub(wiop.Mul(staticCol.View(), staticCol.View()), staticCol.View()))
	dynA.NewVanishing(sys.Context.Childf("aBool"), wiop.Sub(wiop.Mul(colA.View(), colA.View()), colA.View()))
	dynB.NewVanishing(sys.Context.Childf("bBool"), wiop.Sub(wiop.Mul(colB.View(), colB.View()), colB.View()))

	global.Compile(sys)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	got, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildVanishingSystem() error = %v", err)
	}
	if got.DynamicModuleCount != 2 {
		t.Fatalf("dynamic module count = %d, want 2", got.DynamicModuleCount)
	}
	var indices []int
	for _, module := range got.Modules {
		if module.Size.Dynamic {
			indices = append(indices, module.Size.DynamicIndex)
		}
	}
	if len(indices) != 2 || indices[0] != 0 || indices[1] != 1 {
		t.Fatalf("dynamic indices = %v, want [0 1]", indices)
	}
}

func TestWriteVanishingScenariosZigEmitsSizeModes(t *testing.T) {
	sys := wiop.NewSystemf("mixed")
	r0 := sys.NewRound()
	staticMod := sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	dynMod := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
	staticCol := staticMod.NewColumn(sys.Context.Childf("staticCol"), wiop.VisibilityOracle, r0)
	dynCol := dynMod.NewColumn(sys.Context.Childf("dynCol"), wiop.VisibilityOracle, r0)
	staticMod.NewVanishing(sys.Context.Childf("staticBool"), wiop.Sub(wiop.Mul(staticCol.View(), staticCol.View()), staticCol.View()))
	dynMod.NewVanishing(sys.Context.Childf("dynBool"), wiop.Sub(wiop.Mul(dynCol.View(), dynCol.View()), dynCol.View()))

	global.Compile(sys)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	vanishingSystem, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildVanishingSystem() error = %v", err)
	}
	var out bytes.Buffer
	if err := WriteVanishingScenariosZig(&out, []NamedVanishingSystem{{Name: "mixed", System: vanishingSystem}}); err != nil {
		t.Fatalf("WriteVanishingScenariosZig() error = %v", err)
	}
	zig := out.String()
	for _, want := range []string{".{ .static = 4 }", ".{ .dynamic = 0 }"} {
		if !strings.Contains(zig, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, zig)
		}
	}
}

func TestBuildVanishingSystemSupportsCellAndCoinLeaves(t *testing.T) {
	sys := wiop.NewSystemf("cell-coin")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	cell := r0.NewCell(sys.Context.Childf("cell"), false)
	coin := r1.NewCoinField(sys.Context.Childf("coin"))
	mod.NewVanishing(sys.Context.Childf("coinScaled"), wiop.Mul(coin, wiop.Sub(col.View(), cell)))

	global.Compile(sys)
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	vanishingSystem, err := BuildVanishingSystem(sys, routing)
	if err != nil {
		t.Fatalf("BuildVanishingSystem() error = %v", err)
	}

	var sawCell, sawCoin bool
	for _, expr := range vanishingSystem.Modules[0].Expressions {
		sawCell = sawCell || expr.Kind == ExprCellValue
		sawCoin = sawCoin || expr.Kind == ExprCoinValue
	}
	if !sawCell || !sawCoin {
		t.Fatalf("cell/coin leaves exported: cell=%v coin=%v", sawCell, sawCoin)
	}

	var out bytes.Buffer
	if err := WriteVanishingScenariosZig(&out, []NamedVanishingSystem{{Name: "cell-coin", System: vanishingSystem}}); err != nil {
		t.Fatalf("WriteVanishingScenariosZig() error = %v", err)
	}
	zig := out.String()
	for _, want := range []string{".cell_value", ".coin_value"} {
		if !strings.Contains(zig, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, zig)
		}
	}
}

func TestExprNodeLiteralRendersZeroConstant(t *testing.T) {
	got := exprNodeLiteral(ExprNode{Kind: ExprConstant})
	want := ".{ .constant = field.Element.init(0) },"
	if got != want {
		t.Fatalf("exprNodeLiteral(zero constant) = %q, want %q", got, want)
	}
}
