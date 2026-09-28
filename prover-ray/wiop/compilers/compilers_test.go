package compilers_test

import (
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// TestCompileFull_HooksAndPCS checks hook order, PCS state visibility, per-System
// option forwarding, and the honest/invalid proof boundary for a normal compile.
func TestCompileFull_HooksAndPCS(t *testing.T) {
	sc := wioptest.NewFibonacciVanishingScenario()
	var order []string
	checkGlobal := func(sys *wiop.System) error {
		if len(sys.LagrangeEvals) == 0 {
			return errors.New("global did not create evaluation claims")
		}
		order = append(order, "pre-1")
		return nil
	}
	checkPCS := func(sys *wiop.System) error {
		if pcs.FRINumQueries(sys) != 2 || len(sys.Rounds[len(sys.Rounds)-1].VerifierActions) == 0 {
			return errors.New("PCS did not register its opening")
		}
		order = append(order, "pcs-post")
		return nil
	}
	require.NoError(t, compilers.CompileFull(sc.Sys,
		compilers.WithMessageBusOption(messagebus.WithoutSharedRandomness()),
		compilers.WithPCSOption(pcs.WithFRINumQueries(2)),
		compilers.WithPreHook(compilers.PCS, checkGlobal),
		compilers.WithPreHook(compilers.PCS, func(*wiop.System) error {
			order = append(order, "pre-2")
			return nil
		}),
		compilers.WithPostHook(compilers.PCS, checkPCS),
	))
	require.Equal(t, []string{"pre-1", "pre-2", "pcs-post"}, order,
		"same-pass hooks run in registration order around the pass")
	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	require.Len(t, proof.PCSOpeningProof.InputQueries, 2, "PCS option must control opening size")
	require.NoError(t, sc.Sys.Verify(proof, pub), "honest witness verifies")

	invalid := wioptest.NewFibonacciVanishingScenario()
	require.NoError(t, compilers.CompileFull(invalid.Sys,
		compilers.WithPCSOption(pcs.WithFRINumQueries(2))))
	proof, pub = invalid.Sys.Prove(invalid.AssignInvalid)
	require.Error(t, invalid.Sys.Verify(proof, pub), "invalid witness must fail verification")
}

// TestCompileFull_HookErrors pins that any executed hook error names its pass
// and phase, stops the chain, and retains prior mutations.
func TestCompileFull_HookErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase string
		opt   func(compilers.Pass, func(*wiop.System) error) compilers.Option
	}{
		{name: "pre", phase: "pre-hook", opt: compilers.WithPreHook},
		{name: "post", phase: "post-hook", opt: compilers.WithPostHook},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := wioptest.NewFibonacciVanishingScenario()
			cause := errors.New("hook failed")
			var sawGlobal bool
			err := compilers.CompileFull(sc.Sys,
				compilers.WithPCSOption(pcs.WithFRINumQueries(2)),
				tc.opt(compilers.PCS, func(sys *wiop.System) error {
					sawGlobal = len(sys.LagrangeEvals) > 0
					return cause
				}),
			)
			require.ErrorIs(t, err, cause, "hook error must retain its cause")
			require.ErrorContains(t, err, "PCS "+tc.phase, "error must name pass and phase")
			require.True(t, sawGlobal, "earlier passes must still have produced evaluation claims")
			if tc.phase == "pre-hook" {
				require.Zero(t, pcs.FRINumQueries(sc.Sys), "PCS must not run after failing pre-hook")
			} else {
				require.Equal(t, 2, pcs.FRINumQueries(sc.Sys), "PCS mutations survive failing post-hook")
			}
		})
	}
}

// TestCompileFull_NoOpHooksAndNonPCSFailure covers hook invocation around a no-op
// pass and a failing non-PCS hook.
func TestCompileFull_NoOpHooksAndNonPCSFailure(t *testing.T) {
	sc := wioptest.NewFibonacciVanishingScenario()
	var order []string
	require.NoError(t, compilers.CompileFull(sc.Sys,
		compilers.WithoutPCS(),
		compilers.WithPreHook(compilers.MessageBus, func(*wiop.System) error {
			order = append(order, "pre")
			return nil
		}),
		compilers.WithPostHook(compilers.MessageBus, func(*wiop.System) error {
			order = append(order, "post")
			return nil
		}),
	))
	require.Equal(t, []string{"pre", "post"}, order, "hooks run even when messagebus has no entries")

	stopped := wioptest.NewFibonacciVanishingScenario()
	cause := errors.New("stop")
	err := compilers.CompileFull(stopped.Sys,
		compilers.WithPreHook(compilers.Global, func(*wiop.System) error { return cause }))
	require.ErrorIs(t, err, cause)
	require.ErrorContains(t, err, "Global pre-hook")
	require.Empty(t, stopped.Sys.LagrangeEvals, "the failing global pre-hook must stop before global")
	require.Len(t, stopped.Sys.Rounds, 1, "no pass before global needs a new round here")
}

// TestCompileFull_WithoutPCS checks that skipping PCS still runs all preceding
// reductions, leaves the witness uncommitted, and skips both PCS hooks.
func TestCompileFull_WithoutPCS(t *testing.T) {
	sc := wioptest.NewFibonacciVanishingScenario()
	var pcsHookCalls int
	require.NoError(t, compilers.CompileFull(sc.Sys,
		compilers.WithoutPCS(),
		compilers.WithPreHook(compilers.PCS, func(*wiop.System) error {
			pcsHookCalls++
			return nil
		}),
		compilers.WithPostHook(compilers.PCS, func(*wiop.System) error {
			pcsHookCalls++
			return nil
		}),
	))
	require.Zero(t, pcsHookCalls, "PCS hooks must not run when PCS is skipped")
	require.NotEmpty(t, sc.Sys.LagrangeEvals, "global reductions must be retained")
	require.Zero(t, pcs.FRINumQueries(sc.Sys), "no query metadata without PCS")

	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	require.Nil(t, proof.PCSOpeningProof, "no PCS opening without the pass")
	require.Empty(t, proof.Commitments, "columns must remain uncommitted")
	require.NoError(t, sc.Sys.Verify(proof, pub), "the in-the-clear scenario still verifies")
}

// TestCompileFull_MessageBusOption checks that a per-System messagebus option
// reaches the pass before it runs.
func TestCompileFull_MessageBusOption(t *testing.T) {
	sys := wiop.NewSystemf("composer-bus")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	send := mod.NewColumn(sys.Context.Childf("send"), r0)
	recv := mod.NewColumn(sys.Context.Childf("recv"), r0)
	busSend := sys.NewMessageBusSend(sys.Context.Childf("send-bus"), "shard", "h", wiop.NewTable(send.View()))
	busSend.SkipInShardCheck = true
	busRecv := sys.NewMessageBusReceive(sys.Context.Childf("recv-bus"), "shard", "h", wiop.NewTable(recv.View()))
	busRecv.SkipInShardCheck = true

	require.NoError(t, compilers.CompileFull(sys,
		compilers.WithMessageBusOption(messagebus.WithoutSharedRandomness()),
		compilers.WithPCSOption(pcs.WithFRINumQueries(2)),
	))
	require.False(t, messagebus.HasSharedRandomness(sys), "the opt-out option must reach messagebus")
}

// TestCompileFull_OptionValidation rejects an invalid pass, nil hook, and PCS
// option conflict before any pass mutates the System.
func TestCompileFull_OptionValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []compilers.Option
		want string
	}{
		{name: "invalid pass", opts: []compilers.Option{compilers.WithPreHook(compilers.PCS+1, func(*wiop.System) error { return nil })}, want: "invalid pass Pass(9)"},
		{name: "nil hook", opts: []compilers.Option{compilers.WithPostHook(compilers.Global, nil)}, want: "nil post hook for Global"},
		{name: "pcs conflict first", opts: []compilers.Option{compilers.WithoutPCS(), compilers.WithPCSOption(pcs.WithFRINumQueries(2))}, want: "conflicts with WithPCSOption"},
		{name: "pcs conflict last", opts: []compilers.Option{compilers.WithPCSOption(pcs.WithFRINumQueries(2)), compilers.WithoutPCS()}, want: "conflicts with WithPCSOption"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := wioptest.NewFibonacciVanishingScenario()
			rounds := len(sc.Sys.Rounds)
			err := compilers.CompileFull(sc.Sys, tc.opts...)
			require.ErrorContains(t, err, tc.want)
			require.Len(t, sc.Sys.Rounds, rounds, "option errors must not mutate the System")
			require.Zero(t, pcs.FRINumQueries(sc.Sys), "option errors must not record PCS metadata")
		})
	}
}
