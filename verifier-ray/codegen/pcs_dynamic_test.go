package codegen

import (
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
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

// (The former TestHasCommittedDynamicColumn was removed: committed dynamic
// columns are now SUPPORTED via runtime layout reconstruction, so BuildPcsSystem
// no longer rejects them and the HasCommittedDynamicColumn guard was deleted.)

// BuildPcsSystem must REJECT a dynamic column opened at two shift offsets that
// alias mod its runtime size. prover-ray dedups such openings (mod the size) to
// one, but the size-independent ColumnDesc schedule keeps both, so the verifier
// would expect an extra (unauthenticated) claim and double-count the DEEP
// quotient. Storing raw offsets is only sound when they don't alias — the guard
// enforces that.
func TestBuildPcsSystemRejectsAliasingDynamicShifts(t *testing.T) {
	sys := wiop.NewSystemf("dyn-alias")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), wiop.VisibilityOracle, r0)
	// Open the column at offsets 1 and 9: at the proving size 8 these alias
	// (1 == 9 mod 8), so prover-ray produces one opening but the raw schedule two.
	mod.NewVanishing(
		sys.Context.Childf("alias"),
		wiop.Sub(col.View().Shift(1), col.View().Shift(9)),
	)

	global.Compile(sys)
	pcscompiler.Compile(sys)

	// Prove at size 8 (where offsets 1 and 9 alias).
	vals := make([]field.Element, 8)
	for i := range vals {
		vals[i].SetUint64(uint64(i + 1))
	}
	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vals)})
	for _, action := range rt.CurrentRound().ProverActions {
		action.Run(rt)
	}
	for rt.CurrentRound().ID < len(rt.System.Rounds)-1 {
		rt.AdvanceRound()
		for _, action := range rt.CurrentRound().ProverActions {
			action.Run(rt)
		}
	}

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}
	_, err = BuildPcsSystem(sys, rt, routing)
	if err == nil {
		t.Fatalf("BuildPcsSystem accepted aliasing dynamic-column shifts; want an error")
	}
	if !strings.Contains(err.Error(), "alias") {
		t.Fatalf("BuildPcsSystem error = %q, want an aliasing rejection", err.Error())
	}
}
