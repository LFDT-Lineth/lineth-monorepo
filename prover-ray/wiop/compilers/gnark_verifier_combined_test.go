package compilers_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// combinedWitness assigns every column of [newCombinedSystem]. Each field is
// one feature's witness, so an invalid variant changes a single feature.
type combinedWitness struct {
	fib, rc, local []uint64
	table, looked  []uint64
	permA, permB   []uint64
	sent, received []uint64
	twinA, twinB   []uint64
}

func honestCombinedWitness() combinedWitness {
	return combinedWitness{
		fib:      []uint64{1, 1, 2, 3, 5, 8, 13, 21},
		rc:       []uint64{0, 7, 1, 6, 2, 5, 3, 4},
		local:    []uint64{0, 9, 9, 9},
		table:    []uint64{10, 20, 30, 40},
		looked:   []uint64{10, 20, 10, 30},
		permA:    []uint64{10, 20, 30, 40},
		permB:    []uint64{30, 10, 40, 20},
		sent:     []uint64{10, 20, 30, 40},
		received: []uint64{40, 10, 30, 20},
		twinA:    []uint64{3, 1, 4, 1},
		twinB:    []uint64{3, 1, 4, 1},
	}
}

// newCombinedSystem declares, in round 0, a vanishing, a local constraint, a
// range check, a lookup, a permutation and a message bus, so that every
// compiler pass contributes to the same PCS opening. The compilers put their
// own columns in rounds 1 and 2, so rounds 3 and 4 each commit only one twin
// column, constrained equal: their honest batches are identical, so they have
// equal roots, which exercises root dedup being off end to end.
// It returns the system and its round-0 assignment; the twins are assigned by
// prover actions of their own rounds.
func newCombinedSystem(w combinedWitness) (*wiop.System, func(*wiop.Runtime)) {
	sys := wiop.NewSystemf("gnark-combined")
	r0 := sys.NewRound()
	sys.NewRound()
	sys.NewRound()
	r3 := sys.NewRound()
	r4 := sys.NewRound()
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

	modPA := sys.NewSizedModule(ctx("modPA"), 4, wiop.PaddingDirectionNone)
	modPB := sys.NewSizedModule(ctx("modPB"), 4, wiop.PaddingDirectionNone)
	permA := modPA.NewColumn(ctx("PA"), r0)
	permB := modPB.NewColumn(ctx("PB"), r0)
	sys.NewPermutation(ctx("perm"),
		[]wiop.Table{wiop.NewTable(permA.View())},
		[]wiop.Table{wiop.NewTable(permB.View())})

	modBA := sys.NewSizedModule(ctx("modBA"), 4, wiop.PaddingDirectionNone)
	modBB := sys.NewSizedModule(ctx("modBB"), 4, wiop.PaddingDirectionNone)
	sent := modBA.NewColumn(ctx("sent"), r0)
	received := modBB.NewColumn(ctx("received"), r0)
	sys.NewMessageBusSend(ctx("send"), "shard", "ping", wiop.NewTable(sent.View()))
	sys.NewMessageBusReceive(ctx("recv"), "shard", "ping", wiop.NewTable(received.View()))

	modTw := sys.NewSizedModule(ctx("modTwin"), 4, wiop.PaddingDirectionNone)
	twinA := modTw.NewColumn(ctx("twinA"), r3)
	twinB := modTw.NewColumn(ctx("twinB"), r4)
	modTw.NewVanishing(ctx("twins-equal"), wiop.Sub(twinA.View(), twinB.View()))
	r3.RegisterAction(proverHook(func(rt *wiop.Runtime) { rt.AssignColumn(twinA, vecOf(w.twinA...)) }))
	r4.RegisterAction(proverHook(func(rt *wiop.Runtime) { rt.AssignColumn(twinB, vecOf(w.twinB...)) }))

	return sys, func(rt *wiop.Runtime) {
		rt.AssignColumn(fib, vecOf(w.fib...))
		rt.AssignColumn(rc, vecOf(w.rc...))
		rt.AssignColumn(local, vecOf(w.local...))
		rt.AssignColumn(table, vecOf(w.table...))
		rt.AssignColumn(looked, vecOf(w.looked...))
		rt.AssignColumn(permA, vecOf(w.permA...))
		rt.AssignColumn(permB, vecOf(w.permB...))
		rt.AssignColumn(sent, vecOf(w.sent...))
		rt.AssignColumn(received, vecOf(w.received...))
	}
}

// TestGnarkVerifier_Combined runs one system using every compiler through the
// full pipeline and the verifier circuit: the honest proof passes, and an
// invalid witness for each feature is rejected natively and in the circuit.
// The prover refuses a lookup or range-check witness that does not hold, so
// those are only covered by their own scenario tests.
func TestGnarkVerifier_Combined(t *testing.T) {
	sys, assign := newCombinedSystem(honestCombinedWitness())
	compileGnarkPipeline(t, sys, noSharedRandomness)
	proof, pub := sys.Prove(assign)
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")
	require.Equal(t, proof.Commitments[3], proof.Commitments[4], "the twin rounds must commit equal roots")

	gv := compileGnarkVerifier(t, sys, proof, pub)
	require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
	t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

	invalid := map[string]func(*combinedWitness){
		"vanishing":   func(w *combinedWitness) { w.fib[7] = 22 },
		"local":       func(w *combinedWitness) { w.local[0] = 7 },
		"permutation": func(w *combinedWitness) { w.permB[0] = 31 },
		"message bus": func(w *combinedWitness) { w.received[3] = 21 },
		"twins":       func(w *combinedWitness) { w.twinB[0] = 5 },
	}
	for name, corrupt := range invalid {
		t.Run(name, func(t *testing.T) {
			w := honestCombinedWitness()
			corrupt(&w)
			badSys, badAssign := newCombinedSystem(w)
			compileGnarkPipeline(t, badSys, noSharedRandomness)
			badProof, badPub := badSys.Prove(badAssign)
			require.Error(t, badSys.Verify(badProof, badPub), "invalid witness must fail natively")
			require.Error(t, gv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")
		})
	}
}
