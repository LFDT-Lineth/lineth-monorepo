package codegen

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// newBooleanPcsSystem builds a minimal PCS-compiled system: one size-4 static
// module with a single boolean column, no dynamic modules, no lookups. Small
// enough to prove/verify quickly while still exercising the full FRI/PCS
// opening machinery EncodeProof needs to serialize.
func newBooleanPcsSystem(t *testing.T) (*wiop.System, *wiop.Column) {
	t.Helper()
	sys := wiop.NewSystemf("proofcodec-test")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	mod.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(wiop.Mul(col.View(), col.View()), col.View()))
	global.Compile(sys)
	pcscompiler.Compile(sys)
	return sys, col
}

func TestEncodeProofRoundTripsAnHonestProof(t *testing.T) {
	sys, col := newBooleanPcsSystem(t)

	assign := func(rt *wiop.Runtime) {
		vals := make([]field.Element, 4)
		vals[0].SetUint64(0)
		vals[1].SetUint64(1)
		vals[2].SetUint64(1)
		vals[3].SetUint64(0)
		rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(vals)})
	}

	// Drive a manual runtime (rather than sys.Prove) so we retain the *Runtime
	// EncodeProof needs for BuildPcsSystem's canonical entry_claims, mirroring
	// pcs_zig_test.go's own pattern; then build the (Proof, PublicInput) pair
	// the same way sys.Prove would, via a second pass, so both the runtime and
	// the proof reflect the identical honest witness.
	rt := wiop.NewRuntime(sys)
	assign(rt)
	for _, action := range rt.CurrentRound().ProverActions {
		action.Run(rt)
	}
	for rt.CurrentRound().ID < len(rt.System.Rounds)-1 {
		rt.AdvanceRound()
		for _, action := range rt.CurrentRound().ProverActions {
			action.Run(rt)
		}
	}

	proof, pub := sys.Prove(assign, wiop.ProveOptions{CheckUnreducedQueries: true})
	if err := sys.Verify(proof, pub); err != nil {
		t.Fatalf("sys.Verify() error = %v, want honest proof to verify", err)
	}

	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}

	b, err := EncodeProof(sys, rt, routing, proof, pub)
	if err != nil {
		t.Fatalf("EncodeProof() error = %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("EncodeProof() returned empty bytes")
	}

	// Encoding must be deterministic: re-encoding the same proof yields
	// byte-identical output.
	b2, err := EncodeProof(sys, rt, routing, proof, pub)
	if err != nil {
		t.Fatalf("EncodeProof() second call error = %v", err)
	}
	if len(b) != len(b2) {
		t.Fatalf("EncodeProof() not deterministic: lengths %d vs %d", len(b), len(b2))
	}
	for i := range b {
		if b[i] != b2[i] {
			t.Fatalf("EncodeProof() not deterministic: byte %d differs (%d vs %d)", i, b[i], b2[i])
		}
	}
}

func TestEncodeProofRejectsSystemWithoutPcs(t *testing.T) {
	sys := wiop.NewSystemf("no-pcs")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {}, wiop.ProveOptions{})
	routing, err := BuildCoinRouting(sys)
	if err != nil {
		t.Fatalf("BuildCoinRouting() error = %v", err)
	}

	rt := wiop.NewRuntime(sys)
	if _, err := EncodeProof(sys, rt, routing, proof, pub); err == nil {
		t.Fatalf("EncodeProof() error = nil, want an error for a non-PCS-compiled system")
	}
}
