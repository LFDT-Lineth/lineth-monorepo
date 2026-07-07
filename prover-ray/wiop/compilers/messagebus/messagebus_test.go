package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// makeVec builds a base-field ConcreteVector from uint64 literals.
func makeVec(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// runRound executes every prover action registered on the runtime's current
// round.
func runRound(rt *wiop.Runtime) {
	for _, a := range rt.CurrentRound().ProverActions {
		a.Run(*rt)
	}
}

// checkAllVerifierActions evaluates every verifier action across every round
// of the runtime and returns the first non-nil error.
func checkAllVerifierActions(rt *wiop.Runtime) error {
	for _, r := range rt.System.Rounds {
		for _, va := range r.VerifierActions {
			if err := va.Check(*rt); err != nil {
				return err
			}
		}
	}
	return nil
}

// fixedSeedHook is a test [wiop.ProverAction] that overrides the runtime's
// Fiat–Shamir state with a precomputed seed before any coin in the
// containing round is sampled. It is the test stand-in for the cross-shard
// layer's SetInitialFSHash-equivalent in production: the cross-shard layer
// would read the seed from a shared-randomness column; this test version
// carries the seed inline.
type fixedSeedHook struct {
	seed field.Octuplet
}

func (h *fixedSeedHook) Run(rt wiop.Runtime) {
	rt.SetFSState(h.seed)
}

// testMessageBusSeed is the seed every messagebus test uses on the
// with-hook code path. The concrete value is unimportant — any
// deterministic non-zero octuplet works.
var testMessageBusSeed = field.NewOctupletFromStrings(
	[8]string{"1", "2", "3", "5", "8", "13", "21", "34"},
)

// setupMessageBusHook pre-allocates the round that [messagebus.Compile]
// will claim for α and β (immediately after the witness round) and
// registers a [fixedSeedHook] on it. When Compile later runs,
// ensureRoundAfter discovers the pre-allocated tail round and reuses it,
// so α and β get declared on the same round the hook lives on. Subsequent
// [wiop.Runtime.AdvanceRound] fires the hook before sampling, which means
// α and β derive deterministically from testMessageBusSeed instead of
// from the witness columns absorbed on the round before.
//
// This mirrors the wiring a sharded protocol uses in production — the
// only difference is the seed source: a hard-coded octuplet here, a
// shared-randomness column in the real cross-shard layer.
func setupMessageBusHook(sys *wiop.System) *wiop.Round {
	coinRound := sys.NewRound()
	coinRound.RegisterPreSamplingHook(&fixedSeedHook{seed: testMessageBusSeed})
	return coinRound
}

// runWithAndWithoutHook runs body as two subtests covering both coin
// pathways for any messagebus pipeline: once with the natural Fiat–Shamir
// transcript driving α/β (no hook registered) and once with
// [setupMessageBusHook] pre-attached so α/β derive from testMessageBusSeed
// instead. body receives a fresh [wiop.System] (named after the running
// subtest), the witness round r0 already allocated, and is expected to
// declare modules/columns/queries on r0 and drive the proof.
func runWithAndWithoutHook(t *testing.T, body func(t *testing.T, sys *wiop.System, r0 *wiop.Round)) {
	t.Helper()
	for _, tc := range []struct {
		name     string
		withHook bool
	}{
		{"natural-fs", false},
		{"with-presampling-hook", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sys := wiop.NewSystemf("%s", t.Name())
			r0 := sys.NewRound()
			if tc.withHook {
				setupMessageBusHook(sys)
			}
			body(t, sys, r0)
		})
	}
}

// drive runs the canonical "assign-witness → advance to coin round → advance
// to result round → run prover" loop for a message-bus pipeline. After this
// returns, every prover action has executed and the verifier actions are
// ready to be checked.
//
// Round structure produced by [messagebus.Compile] + [grandproduct.Compile]:
//
//   - Round 0: user-witness columns (selectors, value columns).
//   - Round 1: shared (α, β) coins; no prover actions.
//   - Round 2: GrandProduct Result cells + Z columns; one prover action
//     per GrandProduct query that assigns Z and the result.
func drive(rt *wiop.Runtime) {
	rt.AdvanceRound() // → coin round, samples α and β
	rt.AdvanceRound() // → result round
	runRound(rt)      // assigns Z columns + result cells
}
