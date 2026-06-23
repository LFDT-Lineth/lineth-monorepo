package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
// Round structure produced by [messagebus.Compile] + [logderivativesum.Compile]:
//
//   - Round 0: user-witness columns (selectors, value columns, multiplicities).
//   - Round 1: shared (α, β) coins; no prover actions.
//   - Round 2: LogDerivativeSum Result cells + Z columns; one prover action
//     per LogDerivativeSum query that assigns Z and the result.
func drive(rt *wiop.Runtime) {
	rt.AdvanceRound() // → coin round, samples α and β
	rt.AdvanceRound() // → result round
	runRound(rt)      // assigns Z columns + result cells
}

// TestCompile_Balanced is the canonical completeness case: one handle, one
// Send entry, one Receive entry, multiplicities all 1, every sent row
// matches a received row. The shard's residual on the handle is zero and
// the verifier must accept.
func TestCompile_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "ping",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "ping",
			wiop.NewTable(colB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
		rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_TamperedMultiplicity exercises the soundness path: with a
// receiver multiplicity that no longer counts the senders correctly, the
// shard's residual on the handle is non-zero and the verifier must reject.
func TestCompile_TamperedMultiplicity(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "ping",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "ping",
			wiop.NewTable(colB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
		rt.AssignColumn(mulB, makeVec(2, 1, 1, 1)) // wrong: row 0 is counted twice

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"verifier must reject a multiplicity that miscounts the senders")
	})
}

// TestCompile_TamperedValueFailsInShardCheck exercises the in-shard sum
// rejection path through a tampered VALUE column (rather than a tampered
// multiplicity, which TestCompile_TamperedMultiplicity already covers). The
// send and receive multisets disagree on a single row's value, so the
// folded denominators differ and the per-handle sum lands at a non-zero
// element of the extension field.
func TestCompile_TamperedValueFailsInShardCheck(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "ping",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "ping",
			wiop.NewTable(colB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(10, 21, 30, 40)) // wrong: row 1 holds 21, not 20
		rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"verifier must reject when a receive row's value does not appear in the send multiset")
	})
}

// TestCompile_TamperedFilterFailsInShardCheck exercises the in-shard sum
// rejection path through an ASYMMETRIC selector. The send side filters out
// a row that the receive side still claims, so the multiset on each side
// has a different cardinality and the per-handle sum is non-zero.
func TestCompile_TamperedFilterFailsInShardCheck(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "ping",
			wiop.NewFilteredTable(selA.View(), colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "ping",
			wiop.NewTable(colB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(selA, makeVec(1, 0, 1, 1)) // sender filters out row 1 (value 20)
		rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
		rt.AssignColumn(mulB, makeVec(1, 1, 1, 1)) // receiver still claims all four

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"verifier must reject when the send-side selector drops a row the receive side still claims")
	})
}

// TestCompile_MultipleSendersOneReceiver verifies that several Send entries
// on the same handle can balance against a single Receive entry if
// multiplicities tally them correctly. Sender 1 emits values [10, 20];
// sender 2 also emits [10, 20]; the receiver holds [10, 20] with
// multiplicity [2, 2]. All three entries live on the same shard.
func TestCompile_MultipleSendersOneReceiver(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modS2 := sys.NewSizedModule(sys.Context.Childf("modS2"), 2, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 2, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
		colS2 := modS2.NewColumn(sys.Context.Childf("S2"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)
		mulR := modR.NewColumn(sys.Context.Childf("mR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-S1"),
			"shard", "bus",
			wiop.NewTable(colS1.View()),
		)
		sys.NewMessageBusSend(
			sys.Context.Childf("send-S2"),
			"shard", "bus",
			wiop.NewTable(colS2.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-R"),
			"shard", "bus",
			wiop.NewTable(colR.View()),
			mulR.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS1, makeVec(10, 20))
		rt.AssignColumn(colS2, makeVec(10, 20))
		rt.AssignColumn(colR, makeVec(10, 20))
		rt.AssignColumn(mulR, makeVec(2, 2))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_MultiColumnTuples covers width > 1 tables: senders push
// (key, value) pairs; receivers consume them with matching multiplicity. The
// compiler must allocate an α coin (in addition to β) and fold each tuple
// via Horner before hashing.
func TestCompile_MultiColumnTuples(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		keyA := modA.NewColumn(sys.Context.Childf("kA"), wiop.VisibilityOracle, r0)
		valA := modA.NewColumn(sys.Context.Childf("vA"), wiop.VisibilityOracle, r0)
		keyB := modB.NewColumn(sys.Context.Childf("kB"), wiop.VisibilityOracle, r0)
		valB := modB.NewColumn(sys.Context.Childf("vB"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "kv",
			wiop.NewTable(keyA.View(), valA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "kv",
			wiop.NewTable(keyB.View(), valB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(keyA, makeVec(1, 2, 3, 4))
		rt.AssignColumn(valA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(keyB, makeVec(1, 2, 3, 4))
		rt.AssignColumn(valB, makeVec(10, 20, 30, 40))
		rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_FilteredSelectors covers Tables with a selector column on
// either side: only rows where the selector is 1 contribute to the
// accumulator.
func TestCompile_FilteredSelectors(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		selB := modB.NewColumn(sys.Context.Childf("selB"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"),
			"shard", "filtered",
			wiop.NewFilteredTable(selA.View(), colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"),
			"shard", "filtered",
			wiop.NewFilteredTable(selB.View(), colB.View()),
			mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		// Active sends: A[0]=10, A[2]=20 (selA = [1,0,1,0]).
		// Active receives at same values, multiplicities counted by selB-respecting
		// filter (selB = [1,1,0,0], mulB = [1,1,0,0]) → receives 10 and 20 once each.
		rt.AssignColumn(colA, makeVec(10, 99, 20, 99))
		rt.AssignColumn(selA, makeVec(1, 0, 1, 0))
		rt.AssignColumn(colB, makeVec(10, 20, 77, 77))
		rt.AssignColumn(selB, makeVec(1, 1, 0, 0))
		rt.AssignColumn(mulB, makeVec(1, 1, 0, 0))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_TwoHandlesIndependent verifies that two unrelated handles in
// the same system are checked independently: they share the global (α, β)
// coins but each handle still gets its own LogDerivativeSum cells and its
// own verifier action, so tampering one handle cannot mask the other.
func TestCompile_TwoHandlesIndependent(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 2, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
		modC := sys.NewSizedModule(sys.Context.Childf("modC"), 2, wiop.PaddingDirectionNone)
		modD := sys.NewSizedModule(sys.Context.Childf("modD"), 2, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)
		colC := modC.NewColumn(sys.Context.Childf("C"), wiop.VisibilityOracle, r0)
		colD := modD.NewColumn(sys.Context.Childf("D"), wiop.VisibilityOracle, r0)
		mulD := modD.NewColumn(sys.Context.Childf("mD"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "alpha",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "alpha",
			wiop.NewTable(colB.View()), mulB.View(),
		)
		sys.NewMessageBusSend(
			sys.Context.Childf("send-C"), "shard", "beta",
			wiop.NewTable(colC.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-D"), "shard", "beta",
			wiop.NewTable(colD.View()), mulD.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(7, 8))
		rt.AssignColumn(colB, makeVec(7, 8))
		rt.AssignColumn(mulB, makeVec(1, 1))
		rt.AssignColumn(colC, makeVec(100, 200))
		rt.AssignColumn(colD, makeVec(100, 200))
		rt.AssignColumn(mulD, makeVec(1, 1))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_ReceiveWithoutMultiplicity covers the "implicit-1 multiplicity"
// path on the Receive side. Pass nil and the compiler must treat it as the
// constant 1.
func TestCompile_ReceiveWithoutMultiplicity(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 2, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 2, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "impl-one",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "impl-one",
			wiop.NewTable(colB.View()),
			nil, // explicit nil → constant-1 multiplicity
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(5, 6))
		rt.AssignColumn(colB, makeVec(5, 6))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt))
	})
}

// TestCompile_NoMessageBusesIsNoOp asserts that running the compiler against
// a system with no message-bus queries is a no-op: no rounds, coins, or
// verifier actions are appended.
func TestCompile_NoMessageBusesIsNoOp(t *testing.T) {
	sys := wiop.NewSystemf("mb-empty")
	r0 := sys.NewRound()
	_ = r0

	roundsBefore := len(sys.Rounds)
	messagebus.Compile(sys)
	assert.Len(t, sys.Rounds, roundsBefore,
		"Compile must not append rounds when there are no MessageBus queries")
}

// TestCompile_DynamicModule_Balanced is the completeness counterpart of
// [TestCompile_Balanced] for dynamic modules: the participating modules'
// sizes are not declared statically but established at runtime by the first
// column assignment. The same compiled System is then re-driven across two
// different runtime sizes to confirm size-agnostic compilation.
func TestCompile_DynamicModule_Balanced(t *testing.T) {
	sys := wiop.NewSystemf("mb-dyn-balanced")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"),
		"shard", "ping",
		wiop.NewTable(colA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"),
		"shard", "ping",
		wiop.NewTable(colB.View()),
		mulB.View(),
	)

	messagebus.Compile(sys)
	logderivativesum.Compile(sys)

	cases := []struct {
		name string
		vals []uint64
	}{
		{"size-4", []uint64{10, 20, 30, 40}},
		{"size-8", []uint64{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := wiop.NewRuntime(sys)
			rt.AssignColumn(colA, makeVec(tc.vals...))
			rt.AssignColumn(colB, makeVec(tc.vals...))
			ones := make([]uint64, len(tc.vals))
			for i := range ones {
				ones[i] = 1
			}
			rt.AssignColumn(mulB, makeVec(ones...))

			drive(&rt)
			require.NoError(t, checkAllVerifierActions(&rt))
		})
	}
}

// TestCompile_DynamicModule_TamperedFails is the soundness counterpart on a
// dynamic module: a multiplicity that miscounts the senders must be rejected
// regardless of the runtime size.
func TestCompile_DynamicModule_TamperedFails(t *testing.T) {

	sys := wiop.NewSystemf("mb-dyn-tampered")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"),
		"shard", "ping",
		wiop.NewTable(colA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"),
		"shard", "ping",
		wiop.NewTable(colB.View()),
		mulB.View(),
	)

	messagebus.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
	rt.AssignColumn(mulB, makeVec(2, 1, 1, 1)) // wrong: row 0 counted twice

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"verifier must reject a multiplicity that miscounts senders on a dynamic module")
}

// TestCompile_DynamicModule_TamperedValueFailsInShardCheck is the
// dynamic-module counterpart of [TestCompile_TamperedValueFailsInShardCheck]:
// the send and receive multisets disagree on a single row's value (colB has
// 21 where colA has 20), so the per-handle sum lands at a non-zero element
// of the extension field and the in-shard verifier action must reject —
// regardless of the runtime-determined row count.
func TestCompile_DynamicModule_TamperedValueFailsInShardCheck(t *testing.T) {
	sys := wiop.NewSystemf("mb-dyn-tampered-value")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"),
		"shard", "ping",
		wiop.NewTable(colA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"),
		"shard", "ping",
		wiop.NewTable(colB.View()),
		mulB.View(),
	)

	messagebus.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(colB, makeVec(10, 21, 30, 40)) // wrong: row 1 holds 21, not 20
	rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"verifier must reject a tampered receive value on a dynamic-module bus")
}

// TestCompile_DynamicModule_TamperedFilterFailsInShardCheck is the
// dynamic-module counterpart of [TestCompile_TamperedFilterFailsInShardCheck]:
// the send side filters out a row the receive side still claims, so the
// multisets have different cardinalities and the in-shard verifier action
// must reject. Exercises the selector path through the dynamic-module fold.
func TestCompile_DynamicModule_TamperedFilterFailsInShardCheck(t *testing.T) {
	sys := wiop.NewSystemf("mb-dyn-tampered-filter")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	selA := modA.NewColumn(sys.Context.Childf("selA"), wiop.VisibilityOracle, r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
	mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"),
		"shard", "ping",
		wiop.NewFilteredTable(selA.View(), colA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"),
		"shard", "ping",
		wiop.NewTable(colB.View()),
		mulB.View(),
	)

	messagebus.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(selA, makeVec(1, 0, 1, 1)) // sender filters out row 1 (value 20)
	rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
	rt.AssignColumn(mulB, makeVec(1, 1, 1, 1)) // receiver still claims all four

	drive(&rt)
	assert.Error(t, checkAllVerifierActions(&rt),
		"verifier must reject an asymmetric send-side selector on a dynamic-module bus")
}

// TestCompile_DynamicModule_MultiColumnTuples covers the width-2 α-fold over
// dynamic modules: (key, value) tuples sent and received with matching
// multiplicities. Exercises both the Horner fold and the runtime-determined
// row count simultaneously.
func TestCompile_DynamicModule_MultiColumnTuples(t *testing.T) {

	sys := wiop.NewSystemf("mb-dyn-tuples")
	r0 := sys.NewRound()
	setupMessageBusHook(sys)

	modA := sys.NewDynamicModule(sys.Context.Childf("modA"), wiop.PaddingDirectionRight)
	modB := sys.NewDynamicModule(sys.Context.Childf("modB"), wiop.PaddingDirectionRight)
	keyA := modA.NewColumn(sys.Context.Childf("kA"), wiop.VisibilityOracle, r0)
	valA := modA.NewColumn(sys.Context.Childf("vA"), wiop.VisibilityOracle, r0)
	keyB := modB.NewColumn(sys.Context.Childf("kB"), wiop.VisibilityOracle, r0)
	valB := modB.NewColumn(sys.Context.Childf("vB"), wiop.VisibilityOracle, r0)
	mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-A"),
		"shard", "kv",
		wiop.NewTable(keyA.View(), valA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-B"),
		"shard", "kv",
		wiop.NewTable(keyB.View(), valB.View()),
		mulB.View(),
	)

	messagebus.Compile(sys)
	logderivativesum.Compile(sys)

	rt := wiop.NewRuntime(sys)
	rt.AssignColumn(keyA, makeVec(1, 2, 3, 4))
	rt.AssignColumn(valA, makeVec(10, 20, 30, 40))
	rt.AssignColumn(keyB, makeVec(1, 2, 3, 4))
	rt.AssignColumn(valB, makeVec(10, 20, 30, 40))
	rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

	drive(&rt)
	require.NoError(t, checkAllVerifierActions(&rt))
}

// TestCompile_WidthMismatchPanics asserts that the compiler rejects two
// participants on the same handle with different column widths — the
// alpha-fold is only meaningful when every participant has the same width.
func TestCompile_WidthMismatchPanics(t *testing.T) {
	sys := wiop.NewSystemf("mb-width-mismatch")
	r0 := sys.NewRound()

	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 2, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colA2 := mod.NewColumn(sys.Context.Childf("A2"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-1col"), "shard", "h",
		wiop.NewTable(colA.View()),
	)
	sys.NewMessageBusReceive(
		sys.Context.Childf("recv-2col"), "shard", "h",
		wiop.NewTable(colB.View(), colA2.View()),
		nil,
	)

	assert.Panics(t, func() { messagebus.Compile(sys) })
}

// TestCompile_MixedOriginShardPanics asserts that Compile rejects a system
// containing unreduced MessageBus entries from more than one shard.
// [messagebus.Compile] is a per-shard operation; mixing OriginShard values
// in a single invocation is a misuse the compiler catches at the top of
// the pass so the error surfaces before any artefacts are emitted.
func TestCompile_MixedOriginShardPanics(t *testing.T) {
	sys := wiop.NewSystemf("mb-mixed-shards")
	r0 := sys.NewRound()

	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 2, wiop.PaddingDirectionNone)
	colA := mod.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
	colB := mod.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)

	sys.NewMessageBusSend(
		sys.Context.Childf("send-shardA"), "shardA", "h",
		wiop.NewTable(colA.View()),
	)
	sys.NewMessageBusSend(
		sys.Context.Childf("send-shardB"), "shardB", "h",
		wiop.NewTable(colB.View()),
	)

	assert.Panics(t, func() { messagebus.Compile(sys) },
		"Compile must panic when MessageBus entries straddle different OriginShard values")
}

// TestCheckHandleSumInShard_ExpectedNonZero exercises the Expected field on
// [messagebus.CheckHandleSumInShard]. The compile-time path always sets
// Expected to zero (single-shard semantics); the non-zero branch is intended
// for the cross-shard layer that constructs this action itself. This test
// fills that coverage gap by:
//
//  1. Building a balanced single-handle pipeline so the LDS Result cell —
//     this shard's residual on the handle — is guaranteed to be zero.
//  2. Suppressing Compile's auto-registered action via
//     [wiop.System.MessageBusSkipInShardCheck].
//  3. Constructing CheckHandleSumInShard directly with two Expected values:
//     zero (matches the actual residual) and one (does not), and asserting
//     Check accepts/rejects accordingly.
//
// The error message is also spot-checked to confirm it mentions the handle
// name and the "expected" framing, since the cross-shard caller will rely
// on those for diagnostics.
func TestCheckHandleSumInShard_ExpectedNonZero(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		// Own the in-shard check ourselves so Compile doesn't pre-register one.
		// This flag is orthogonal to the coin-source variation (hook vs natural
		// FS) — both subtests exercise the manual CheckHandleSumInShard path.
		sys.MessageBusSkipInShardCheck = true

		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), wiop.VisibilityOracle, r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), wiop.VisibilityOracle, r0)
		mulB := modB.NewColumn(sys.Context.Childf("mB"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "ping",
			wiop.NewTable(colA.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "ping",
			wiop.NewTable(colB.View()), mulB.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colB, makeVec(10, 20, 30, 40))
		rt.AssignColumn(mulB, makeVec(1, 1, 1, 1))

		drive(&rt)

		// The single LDS query is this shard's residual on the handle.
		require.Len(t, sys.LogDerivativeSums, 1, "single-shard Compile must emit exactly one LDS per handle")
		residual := sys.LogDerivativeSums[0].Result

		// Happy path: Expected matches the actual residual (zero for a balanced bus).
		happy := &messagebus.CheckHandleSumInShard{
			Handle:   "ping",
			Cell:     residual,
			Path:     "test/ping/expected-zero",
			Expected: field.ElemZero(),
		}
		require.NoError(t, happy.Check(rt),
			"Check must accept when Expected matches the actual residual")

		// Sad path: Expected differs from the actual residual, so Check must reject.
		sad := &messagebus.CheckHandleSumInShard{
			Handle:   "ping",
			Cell:     residual,
			Path:     "test/ping/expected-one",
			Expected: field.ElemOne(),
		}
		err := sad.Check(rt)
		require.Error(t, err,
			"Check must reject when Expected differs from the actual residual")
		assert.Contains(t, err.Error(), `handle "ping"`,
			"error must name the handle for diagnostics")
		assert.Contains(t, err.Error(), "expected",
			"error must include the expected-value context for diagnostics")
	})
}

// TestNewMessageBusReceive_WrongMultiplicityModulePanics asserts that the
// constructor rejects a vector-valued multiplicity bound to a different
// module than the receiving table.
func TestNewMessageBusReceive_WrongMultiplicityModulePanics(t *testing.T) {
	sys := wiop.NewSystemf("mb-bad-mul-mod")
	r0 := sys.NewRound()
	_ = r0
	mod := sys.NewSizedModule(sys.Context.Childf("m"), 2, wiop.PaddingDirectionNone)
	foreign := sys.NewSizedModule(sys.Context.Childf("foreign"), 2, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("c"), wiop.VisibilityOracle, r0)

	assert.Panics(t, func() {
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-bad-mod"), "shard", "h",
			wiop.NewTable(col.View()),
			wiop.NewConstantVector(foreign, field.NewFromString("1")),
		)
	})
}
