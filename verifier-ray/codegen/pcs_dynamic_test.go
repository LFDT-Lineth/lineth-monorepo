package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// DynamicModuleOrder must follow sys.Modules order (module-index order), matching
// prover-ray Runtime.AdvanceRound's dynamic-size absorption. If it followed
// verifier-registration order instead, a multi-dynamic-module protocol whose
// verifier actions are encountered in a different order than sys.Modules would
// absorb sizes in the wrong sequence and derive different Fiat-Shamir coins,
// rejecting honest proofs. (P1b regression guard.)
func TestDynamicModuleOrderFollowsSysModules(t *testing.T) {
	sys := wiop.NewSystemf("dynorder")
	r0 := sys.NewRound()
	// Create modules in a known order: dynA, static, dynB.
	dynA := sys.NewDynamicModule(sys.Context.Childf("dynA"), wiop.PaddingDirectionRight)
	staticMod := sys.NewSizedModule(sys.Context.Childf("static"), 4, wiop.PaddingDirectionNone)
	dynB := sys.NewDynamicModule(sys.Context.Childf("dynB"), wiop.PaddingDirectionRight)
	_ = staticMod
	_ = dynA.NewColumn(sys.Context.Childf("colA"), wiop.VisibilityOracle, r0)
	_ = dynB.NewColumn(sys.Context.Childf("colB"), wiop.VisibilityOracle, r0)

	order := DynamicModuleOrder(sys)
	if len(order) != 2 {
		t.Fatalf("DynamicModuleOrder len = %d, want 2", len(order))
	}
	// Must be sys.Modules order (dynA before dynB), skipping the static module.
	if order[0] != dynA || order[1] != dynB {
		t.Fatalf("DynamicModuleOrder = [%s %s], want [dynA dynB] in sys.Modules order",
			order[0].Context.Path(), order[1].Context.Path())
	}

	idx := DynamicModuleIndex(sys)
	if idx[dynA] != 0 || idx[dynB] != 1 {
		t.Fatalf("DynamicModuleIndex = {dynA:%d dynB:%d}, want {0,1}", idx[dynA], idx[dynB])
	}
}

// HasCommittedDynamicColumn must detect a dynamic module whose column is
// PCS-committed (VisibilityOracle). Such a column's FRI bundle size is a
// function of the proof's runtime size, so BuildPcsSystem cannot bake a single
// size-independent System and must reject it. (P1a regression guard.)
func TestHasCommittedDynamicColumn(t *testing.T) {
	// Case 1: dynamic module with a committed column → true.
	{
		sys := wiop.NewSystemf("dyncommitted")
		r0 := sys.NewRound()
		dyn := sys.NewDynamicModule(sys.Context.Childf("dyn"), wiop.PaddingDirectionRight)
		col := dyn.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		dyn.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()))
		pcscompiler.Compile(sys)
		if !HasCommittedDynamicColumn(sys) {
			t.Fatalf("HasCommittedDynamicColumn = false, want true for a committed dynamic column")
		}
	}

	// Case 2: only a static module with a committed column → false.
	{
		sys := wiop.NewSystemf("staticonly")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
		col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
		mod.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()))
		pcscompiler.Compile(sys)
		if HasCommittedDynamicColumn(sys) {
			t.Fatalf("HasCommittedDynamicColumn = true, want false for a static-only protocol")
		}
	}
}
