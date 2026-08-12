package pcs

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Size-1 (FRI boundary-round) column forgery
// =============================================================================
//
// A committed column of plaintext size 1 enters FRI at the *boundary* round
// (round == numRounds(), since logFinalPolySize is 0 in production). Such a
// level is never folded; the verifier's only test of it is `Self == Sibling` in
// fri.checkFolds. Writing the committed 2-element codeword's unique degree-<2
// interpolant as v(X) = E + X*O, that test is equivalent to
//
//	E + zeta*O == y,     i.e.  fold_zeta(v) == y,
//
// which pins v only to degree < 2. A size-1 Reed-Solomon codeword must have
// degree < 1, i.e. be the constant (w, w). So the PCS is NOT binding for size-1
// columns: a prover may commit (w+b, w-b), representing v(X) = w + b*X, and the
// FRI layer accepts it so long as the claimed opening is v(zeta).
//
// These tests answer the follow-on question: does anything DOWNSTREAM of the
// PCS catch that forgery?
//
// The forgery is mounted where a real prover would have to mount it, respecting
// the protocol's own ordering:
//
//	hook A, at the committed column's round: overwrite the committed codeword
//	        and re-Merkleize. This necessarily runs BEFORE zeta is drawn, just
//	        as in a real proof -- which is exactly why the attacker cannot steer
//	        the opened value.
//	hook B, at the LagrangeEval's round (zeta now known): overwrite the claimed
//	        evaluation cell to v(zeta), the only value the boundary check will
//	        accept for the forged codeword. The cell is overwritten before
//	        AdvanceRound absorbs it, so the prover and verifier transcripts stay
//	        in sync and every challenge is re-derived consistently.

// newMixedSizeSystem builds a system with a size-8 module and a size-1 module,
// both committed in round 0, and returns the size-1 column as the forgery
// target.
//
// The size mix is what puts that column on the boundary round: the FRI schedule
// is restricted to the largest opened size (2^3), giving numRounds == 3, so the
// size-1 level is introduced at round 3 == numRounds and is never folded. A
// system whose columns are ALL size 1 would instead restrict to numRounds == 0
// and take a different (also defective, but distinct) code path.
func newMixedSizeSystem() (sys *wiop.System, target *wiop.Column, assign func(*wiop.Runtime)) {
	sys = wiop.NewSystemf("mixed")
	r0 := sys.NewRound()

	big := sys.NewSizedModule(sys.Context.Childf("big"), 8, wiop.PaddingDirectionNone)
	bcol := big.NewColumn(sys.Context.Childf("bcol"), wiop.VisibilityOracle, r0)
	// Fibonacci: A[i] - A[i-1] - A[i-2] == 0.
	big.NewVanishing(
		sys.Context.Childf("fib"),
		wiop.Sub(wiop.Sub(bcol.View(), bcol.View().Shift(-1)), bcol.View().Shift(-2)),
	)

	smallMod := sys.NewSizedModule(sys.Context.Childf("small"), 1, wiop.PaddingDirectionNone)
	target = smallMod.NewColumn(sys.Context.Childf("scol"), wiop.VisibilityOracle, r0)
	// scol == 17: a constraint on the single row of a size-1 module.
	smallMod.NewVanishing(
		sys.Context.Childf("is17"),
		wiop.Sub(target.View(), wiop.NewConstantVector(smallMod, field.NewFromString("17"))),
	)

	assign = func(rt *wiop.Runtime) {
		rt.AssignColumn(bcol, makeVecForTest(1, 1, 2, 3, 5, 8, 13, 21))
		rt.AssignColumn(target, makeVecForTest(17))
	}
	return sys, target, assign
}

func makeVecForTest(values ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(values))
	for i, v := range values {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// forgeryPlan is the mutable state shared between the two prover hooks.
type forgeryPlan struct {
	// b is the slope of the forged interpolant v(X) = w + b*X. b == 0 means "no
	// forgery": v is the honest constant and everything must still verify.
	b field.Element
	// target is the size-1 column to forge.
	target *wiop.Column
	// honest is the target's true row value w, recovered from its codeword.
	honest field.Element
	// forgedClaim is v(zeta), written into the claim cells by hook B.
	forgedClaim field.Ext
	// firedA / firedB record that each hook actually found its target, so a
	// silently-skipped forgery can never masquerade as a passing test.
	firedA, firedB bool
}

// proverHook adapts a closure to [wiop.ProverAction].
type proverHook struct{ run func(rt *wiop.Runtime) }

func (h *proverHook) Run(rt *wiop.Runtime) { h.run(rt) }

// recordingVerifier wraps a verifier action, records its verdict under label,
// and always reports success. Swallowing the error is what lets every action run
// on a single replay: wiop.Verify short-circuits on the first failure, so
// without this the global check (registered on an earlier round) would mask
// whether the PCS check accepted or rejected the forged column -- which is the
// entire question under test.
type recordingVerifier struct {
	inner   wiop.VerifierAction
	label   string
	verdict map[string]error
	seen    map[string]bool
}

func (r *recordingVerifier) Check(rt *wiop.Runtime) error {
	err := r.inner.Check(rt)
	r.seen[r.label] = true
	// Keep the first error seen for a label: several actions may share a type.
	if prev, ok := r.verdict[r.label]; !ok || prev == nil {
		r.verdict[r.label] = err
	}
	return nil
}

func instrumentVerifiers(sys *wiop.System) (verdict map[string]error, seen map[string]bool) {
	verdict = make(map[string]error)
	seen = make(map[string]bool)
	for _, r := range sys.Rounds {
		for i, va := range r.VerifierActions {
			r.VerifierActions[i] = &recordingVerifier{
				inner: va, label: fmt.Sprintf("%T", va), verdict: verdict, seen: seen,
			}
		}
	}
	return verdict, seen
}

func compileForForgeryTest(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	Compile(sys)
}

// installForgeryHooks registers hook A on every committed interactive round and
// hook B on every LagrangeEval round. Must run AFTER Compile so the hooks are
// appended behind the PCS commit action and the global prover action.
func installForgeryHooks(sys *wiop.System, plan *forgeryPlan) {
	for _, batch := range CommittedBatches(sys) {
		if batch.IsPrecomp {
			// Committed at compile time, before any runtime exists, so it
			// cannot be reached from a prover hook.
			continue
		}
		round := batch.Round
		round.RegisterAction(&proverHook{run: func(rt *wiop.Runtime) {
			forgeCommittedCodeword(rt, round, plan)
		}})
	}

	seenRounds := make(map[*wiop.Round]bool)
	for _, eval := range sys.LagrangeEvals {
		round := eval.Round()
		if seenRounds[round] {
			continue
		}
		seenRounds[round] = true
		round.RegisterAction(&proverHook{run: func(rt *wiop.Runtime) {
			forgeClaimCells(rt, sys, plan)
		}})
	}
}

// forgeCommittedCodeword (hook A) rewrites the target column's committed
// codeword from the honest constant (w, w) to (w+b, w-b) -- the evaluations of
// v(X) = w + b*X over the size-2 codeword domain {1, -1} -- then re-Merkleizes
// the batch and republishes the root so Fiat-Shamir absorbs the forged value.
func forgeCommittedCodeword(rt *wiop.Runtime, round *wiop.Round, plan *forgeryPlan) {
	if plan.firedA {
		return
	}
	raw, ok := rt.GetState(committedStateKey(round.ID))
	if !ok {
		return
	}
	st := raw.(*fri.CommitterState)

	layout, _ := GetLayout(round, rt)
	loc, ok := layout[plan.target.Context.ID]
	if !ok {
		return // target lives in a different round
	}
	if loc.SizeID != 0 || loc.IsExt {
		panic(fmt.Sprintf("target column is not a size-1 base column: sizeID=%d isExt=%v",
			loc.SizeID, loc.IsExt))
	}

	codeword := st.EncodedTable[0].Base[loc.Position]
	if len(codeword) != 2 {
		panic(fmt.Sprintf("expected a 2-element size-1 codeword, got %d", len(codeword)))
	}
	// An honest size-1 codeword is constant; assert it so the test fails loudly
	// if the encoding assumption ever drifts.
	if !codeword[0].Equal(&codeword[1]) {
		panic("size-1 codeword is not constant before forgery")
	}
	plan.honest = codeword[0]

	var c0, c1 field.Element
	c0.Add(&plan.honest, &plan.b)
	c1.Sub(&plan.honest, &plan.b)
	codeword[0], codeword[1] = c0, c1

	st.Tree = st.EncodedTable.Merkleize()
	rt.Commitments[round.ID] = st.Tree.Root()
	plan.firedA = true
}

// forgeClaimCells (hook B) overwrites every LagrangeEval claim cell that opens
// the forged column, replacing the honest w with v(zeta) = w + b*zeta -- the
// unique value the FRI boundary check accepts for the forged codeword.
func forgeClaimCells(rt *wiop.Runtime, sys *wiop.System, plan *forgeryPlan) {
	if !plan.firedA {
		return
	}
	for _, eval := range sys.LagrangeEvals {
		zeta := eval.EvaluationPoint.EvaluateSingle(rt).Value.AsExt()

		var forged field.Ext
		forged.MulByElement(&zeta, &plan.b)
		honestExt := field.Lift(plan.honest)
		forged.Add(&forged, &honestExt)

		for k, colView := range eval.Polynomials {
			if colView.Column != plan.target {
				continue
			}
			plan.forgedClaim = forged
			claim := eval.EvaluationClaims[k]
			// Force lazy resolution before overriding: claim cells are assigned
			// on demand by LagrangeEval.SelfAssign.
			rt.GetCellValue(claim)
			rt.OverrideCell(claim, field.ElemFromExt(forged))
			plan.firedB = true
		}
	}
}

// runForgery compiles the mixed-size system, mounts the forgery with slope b,
// proves, and verifies with every verifier action instrumented.
func runForgery(t *testing.T, b field.Element) (*forgeryPlan, map[string]error, map[string]bool) {
	t.Helper()

	SetFRINumQueriesForTest(4)
	t.Cleanup(func() { SetFRINumQueriesForTest(229) })

	sys, target, assign := newMixedSizeSystem()
	compileForForgeryTest(sys)

	plan := &forgeryPlan{b: b, target: target}
	installForgeryHooks(sys, plan)
	verdict, seen := instrumentVerifiers(sys)

	proof, pub := sys.Prove(assign)
	require.NoError(t, sys.Verify(proof, pub),
		"instrumented Verify swallows action errors; a hard error here means the replay itself broke")

	return plan, verdict, seen
}

const (
	pcsAction    = "*pcs.OpeningVerifierAction"
	globalAction = "*global.Verifier"
)

// TestSizeOneForgery_Control is the completeness baseline: with slope b == 0 the
// "forgery" is the identity, so the hooks exercise the same re-Merkleize and
// cell-override paths without changing any value. Every verifier action must
// pass. Without this, a harness bug could make the real test's rejection
// meaningless.
func TestSizeOneForgery_Control(t *testing.T) {
	plan, verdict, seen := runForgery(t, field.Zero())

	require.True(t, plan.firedA, "hook A must have found the size-1 column")
	require.True(t, plan.firedB, "hook B must have found the matching claim cell")
	require.True(t, seen[pcsAction], "the PCS verifier action must have run")
	require.True(t, seen[globalAction], "the global verifier action must have run")

	for label, err := range verdict {
		require.NoError(t, err, "unforged proof must satisfy every verifier action (%s)", label)
	}
}

// TestSizeOneForgery_NotCaughtByPCS_CaughtByGlobal is the experiment.
//
// With b != 0 the committed size-1 column is v(X) = w + b*X, which is NOT a
// valid size-1 Reed-Solomon codeword, and its claimed opening is v(zeta) rather
// than the true row value w. The two assertions pin down exactly who notices:
//
//   - the PCS/FRI verifier ACCEPTS -- the boundary-round check only requires
//     degree < 2, so the forged column is indistinguishable from an honest one
//     at that layer. This is the soundness gap, reproduced end-to-end through
//     the real wiop pipeline rather than against the FRI package in isolation.
//   - the global constraint verifier REJECTS -- the claimed value v(zeta) does
//     not satisfy the quotient identity that the honest w satisfies.
//
// So the bug is real and invisible to the commitment scheme, but it is caught
// downstream. Note WHY the attacker gains nothing: hook A must run before zeta
// exists, so b is fixed blind and the opened value v(zeta) = w + b*zeta is then
// pinned by a challenge the prover cannot predict.
func TestSizeOneForgery_NotCaughtByPCS_CaughtByGlobal(t *testing.T) {
	plan, verdict, seen := runForgery(t, field.NewElement(7))

	require.True(t, plan.firedA, "hook A must have forged the size-1 column")
	require.True(t, plan.firedB, "hook B must have forged the matching claim cell")

	honestExt := field.Lift(plan.honest)
	require.False(t, plan.forgedClaim.Equal(&honestExt),
		"the forgery must actually change the opened value")

	require.True(t, seen[pcsAction], "the PCS verifier action must have run")
	require.NoError(t, verdict[pcsAction],
		"SOUNDNESS GAP: the FRI/PCS layer accepted a non-constant size-1 column")

	require.True(t, seen[globalAction], "the global verifier action must have run")
	require.Error(t, verdict[globalAction],
		"the global constraint check is the only thing between this forgery and a forged proof")

	t.Logf("PCS verdict:    accepted (binding failure confirmed end-to-end)")
	t.Logf("global verdict: %v", verdict[globalAction])
}

// TestSizeOneForgery_LabelsExist guards the two type-name constants above: a
// rename would otherwise turn the assertions into vacuous lookups of a missing
// map key (nil error, seen == false).
func TestSizeOneForgery_LabelsExist(t *testing.T) {
	_, _, seen := runForgery(t, field.Zero())
	for _, label := range []string{pcsAction, globalAction} {
		require.True(t, seen[label], "verifier action %q not found; the constant is stale", label)
	}
}
