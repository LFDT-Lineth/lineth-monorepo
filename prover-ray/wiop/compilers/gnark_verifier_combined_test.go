package compilers_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// vecOf returns a base-field column assignment holding vals.
func vecOf(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// combinedWitness assigns every column of [newCombinedSystem]. Each field is
// one feature's witness, so an invalid variant changes a single feature.
type combinedWitness struct {
	fib, rc, local []uint64
	table, looked  []uint64
}

func honestCombinedWitness() combinedWitness {
	return combinedWitness{
		fib:    []uint64{1, 1, 2, 3, 5, 8, 13, 21},
		rc:     []uint64{0, 7, 1, 6, 2, 5, 3, 4},
		local:  []uint64{0, 9, 9, 9},
		table:  []uint64{10, 20, 30, 40},
		looked: []uint64{10, 20, 10, 30},
	}
}

// newCombinedSystem declares, in round 0, a vanishing, a local constraint, a
// range check and a lookup, so that every pass the verifier circuit supports
// contributes to the same PCS opening. It returns the system and its round-0
// assignment.
func newCombinedSystem(w combinedWitness) (*wiop.System, func(*wiop.Runtime)) {
	sys := wiop.NewSystemf("gnark-combined")
	r0 := sys.NewRound()
	ctx := sys.Context.Childf

	mod8 := sys.NewSizedModule(ctx("mod8"), 8, wiop.PaddingDirectionNone)
	fib := mod8.NewColumn(ctx("fib"), r0)
	mod8.NewVanishing(ctx("fib-rec"), wiop.Sub(wiop.Sub(fib.View(), fib.View().Shift(-1)), fib.View().Shift(-2)))
	rc := mod8.NewColumn(ctx("rc"), r0)
	mod8.NewRangeCheck(ctx("rc-8"), rc, 8)

	modL := sys.NewSizedModule(ctx("modL"), 4, wiop.PaddingDirectionNone)
	local := modL.NewColumn(ctx("local"), r0)
	modL.NewLocalConstraint(ctx("local-row0"), local.View(), 0)

	modT := sys.NewSizedModule(ctx("modT"), 4, wiop.PaddingDirectionNone)
	modS := sys.NewSizedModule(ctx("modS"), 4, wiop.PaddingDirectionNone)
	table := modT.NewColumn(ctx("T"), r0)
	looked := modS.NewColumn(ctx("S"), r0)
	sys.NewInclusion(ctx("inc"),
		[]wiop.Table{wiop.NewTable(looked.View())},
		[]wiop.Table{wiop.NewTable(table.View())})

	return sys, func(rt *wiop.Runtime) {
		rt.AssignColumn(fib, vecOf(w.fib...))
		rt.AssignColumn(rc, vecOf(w.rc...))
		rt.AssignColumn(local, vecOf(w.local...))
		rt.AssignColumn(table, vecOf(w.table...))
		rt.AssignColumn(looked, vecOf(w.looked...))
	}
}

// TestGnarkVerifier_Combined runs one system mixing vanishings, local
// constraints, range checks and lookups through the full pipeline and the
// verifier circuit: the honest proof passes, and an invalid witness for each
// feature the prover accepts is rejected natively and in the circuit. The
// prover refuses a lookup or range-check witness that does not hold, so those
// are only covered by their own scenario tests.
func TestGnarkVerifier_Combined(t *testing.T) {
	sys, assign := newCombinedSystem(honestCombinedWitness())
	compileGnarkPipeline(t, sys)
	proof, pub := sys.Prove(assign)
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

	gv := compileGnarkVerifier(t, sys)
	require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
	t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

	invalid := map[string]func(*combinedWitness){
		"vanishing": func(w *combinedWitness) { w.fib[7] = 22 },
		"local":     func(w *combinedWitness) { w.local[0] = 7 },
	}
	for name, corrupt := range invalid {
		t.Run(name, func(t *testing.T) {
			w := honestCombinedWitness()
			corrupt(&w)
			badSys, badAssign := newCombinedSystem(w)
			compileGnarkPipeline(t, badSys)
			badProof, badPub := badSys.Prove(badAssign)
			require.ErrorContains(t, badSys.Verify(badProof, badPub), "wiop/compilers: global quotient check failed for module", "invalid witness must fail natively")
			gv.requireRejectedBy(t, badProof, badPub, "global.(*Verifier).CheckGnark")
		})
	}
}
