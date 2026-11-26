package compilers_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

// gnarkVerifier is the verifier circuit of a fully compiled system, compiled
// once over BN254 so several witnesses can be solved against it.
type gnarkVerifier struct {
	sys *wiop.System
	// circ is the unassigned circuit assignments are made from; the one
	// compiled is a separate allocation, which frontend.Compile writes into.
	circ *wiop.VerifierCircuit
	ccs  interface {
		Solve(witness.Witness, ...solver.Option) (any, error)
		GetNbConstraints() int
	}
}

func compileGnarkVerifier(t *testing.T, sys *wiop.System) *gnarkVerifier {
	t.Helper()
	gv := &gnarkVerifier{sys: sys, circ: wiop.AllocateVerifierCircuit(sys)}
	var err error
	gv.ccs, err = frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder,
		wiop.AllocateVerifierCircuit(sys))
	require.NoError(t, err, "verifier circuit must compile")
	return gv
}

func (gv *gnarkVerifier) solve(t *testing.T, proof wiop.Proof, pub wiop.PublicInput) error {
	t.Helper()
	assignment := gv.circ.AssignVerifierCircuit(proof, pub)
	w, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	require.NoError(t, err, "assignment must produce a witness")
	_, err = gv.ccs.Solve(w)
	return err
}

// requireRejectedBy checks that proof does not satisfy the compiled circuit,
// and that the failing assertion is raised inside check, a function name as it
// appears in a stack trace (e.g. "logderivativesum.(*VerifierAction).CheckGnark").
// The compiled solver only reports a constraint index, so the failing check is
// located by re-running the circuit in gnark's test engine, whose error carries
// the stack of the failing assertion.
func (gv *gnarkVerifier) requireRejectedBy(t *testing.T, proof wiop.Proof, pub wiop.PublicInput, check string) {
	t.Helper()
	require.Error(t, gv.solve(t, proof, pub), "proof must not satisfy the compiled circuit")
	circuit := wiop.AllocateVerifierCircuit(gv.sys)
	assignment := gv.circ.AssignVerifierCircuit(proof, pub)
	err := test.IsSolved(circuit, assignment, ecc.BN254.ScalarField())
	require.Error(t, err, "proof must not satisfy the circuit in the test engine")
	require.Contains(t, err.Error(), check, "proof must be rejected by %s", check)
}

// compileGnarkPipeline runs [compilers.CompileFull] with the testing options.
func compileGnarkPipeline(t *testing.T, sys *wiop.System) {
	t.Helper()
	require.NoError(t, compilers.CompileFull(sys, wioptest.TestingCompileOptions()...))
}

// withCell returns a copy of proof whose cell id is shifted by one.
func withCell(proof wiop.Proof, id wiop.ObjectID) wiop.Proof {
	cells := maps.Clone(proof.Cells)
	cells[id] = cells[id].Add(field.ElemFromBase(field.One()))
	bad := proof
	bad.Cells = cells
	return bad
}

// proverHook adapts a function to a [wiop.ProverAction].
type proverHook func(*wiop.Runtime)

func (h proverHook) Run(rt *wiop.Runtime) { h(rt) }

// TestGnarkVerifier_LogDerivativeSumScenarios checks the in-circuit final-sum
// identity. The invalid proof is produced by a prover that assigns a wrong
// extension-valued Result before the query self-assigns it: the proof is internally consistent
// (the transcript absorbs the wrong value), so only the final-sum check can
// reject it.
func TestGnarkVerifier_LogDerivativeSumScenarios(t *testing.T) {
	for _, build := range wioptest.LogDerivativeSumCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			require.NotEmpty(t, invalid.Sys.LogDerivativeSums, "scenario must declare a LogDerivativeSum")
			// Unlike sc.TamperResult, keep the cell an extension element: the
			// circuit's witness layout is fixed by the honest proof's field tags.
			res := invalid.Sys.LogDerivativeSums[0].Result
			res.Round().RegisterAction(proverHook(func(rt *wiop.Runtime) {
				rt.AssignCell(res, field.ElemFromExt(field.Uint64ToExt(123456)))
			}))
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignWitness)
			require.ErrorContains(t, invalid.Sys.Verify(badProof, badPub), "wiop/compilers/logderivativesum: final-sum check failed for query", "wrong Result must fail natively")
			gv.requireRejectedBy(t, badProof, badPub, "logderivativesum.(*VerifierAction).CheckGnark")
		})
	}
}

// TestGnarkVerifier_LookupScenarios checks the in-circuit lookup identity.
// The invalid proof bumps one entry of a multiplicity column M after the
// prover assigned it: every later value (Z columns, Result) is then derived
// honestly from the bad M, so the proof is internally consistent and only the
// Result-is-zero check can reject it.
func TestGnarkVerifier_LookupScenarios(t *testing.T) {
	for _, build := range wioptest.LookupScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			before := snapshotModuleColumns(invalid.Sys)
			lookuptologderivsum.Compile(invalid.Sys)
			mCols := newBaseColumns(invalid.Sys, before)
			require.NotEmpty(t, mCols, "lookuptologderivsum must allocate a multiplicity column")
			wioptest.Mutator{Column: mCols[0], Row: 0, Tweak: wioptest.AddOne}.Compile(invalid.Sys)
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignWitness)
			require.ErrorContains(t, invalid.Sys.Verify(badProof, badPub), "wiop/compilers/lookuptologderivsum: aggregated lookup result for query", "wrong multiplicity must fail natively")
			gv.requireRejectedBy(t, badProof, badPub, "lookuptologderivsum.(*ResultIsZeroVerifierAction).CheckGnark")
		})
	}
}

// isDynamic reports whether sys has a dynamic module, which the verifier
// circuit does not support.
func isDynamic(sys *wiop.System) bool {
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			return true
		}
	}
	return false
}

// requireTamperingRejected corrupts the honest proof one value at a time,
// every cell then every round commitment, and checks the circuit rejects each
// corrupted proof.
func requireTamperingRejected(t *testing.T, gv *gnarkVerifier, proof wiop.Proof, pub wiop.PublicInput) {
	t.Helper()
	ids := slices.Sorted(maps.Keys(proof.Cells))
	require.NotEmpty(t, ids, "proof must carry at least one cell")
	for _, id := range ids {
		require.Errorf(t, gv.solve(t, withCell(proof, id), pub), "tampered cell %d must not satisfy the circuit", id)
	}
	one := field.One()
	for _, round := range slices.Sorted(maps.Keys(proof.Commitments)) {
		bad := proof
		bad.Commitments = maps.Clone(proof.Commitments)
		root := bad.Commitments[round]
		root[0].Add(&root[0], &one)
		bad.Commitments[round] = root
		require.Errorf(t, gv.solve(t, bad, pub), "tampered commitment of round %d must not satisfy the circuit", round)
	}
}

// TestGnarkVerifier_LocalVanishingScenarios checks the circuit on every
// static local-vanishing fixture: honest proofs pass, an invalid witness is
// rejected, and so is every single-value corruption of the first fixture's
// proof.
func TestGnarkVerifier_LocalVanishingScenarios(t *testing.T) {
	tampered := false
	for _, build := range wioptest.LocalVanishingScenarios() {
		sc := build()
		if isDynamic(sc.Sys) {
			continue
		}
		withTampering := !tampered
		tampered = true
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")

			invalid := build()
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.ErrorContains(t, invalid.Sys.Verify(badProof, badPub), "wiop/compilers: global quotient check failed for module", "invalid witness must fail natively")
			gv.requireRejectedBy(t, badProof, badPub, "global.(*Verifier).CheckGnark")

			if withTampering {
				requireTamperingRejected(t, gv, proof, pub)
			}
		})
	}
}

// TestGnarkVerifier_NonNativeScenarios is the non-native counterpart of
// [TestGnarkVerifier_LocalVanishingScenarios].
func TestGnarkVerifier_NonNativeScenarios(t *testing.T) {
	for i, build := range wioptest.NonNativeScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.ErrorContains(t, invalid.Sys.Verify(badProof, badPub), "wiop/compilers: global quotient check failed for module", "invalid witness must fail natively")
			gv.requireRejectedBy(t, badProof, badPub, "global.(*Verifier).CheckGnark")

			if i == 0 {
				requireTamperingRejected(t, gv, proof, pub)
			}
		})
	}
}

// TestGnarkVerifier_RangeCheckScenarios checks the circuit on every
// range-check fixture. An out-of-range witness cannot be proven at all (the
// multiplicity assignment panics), so soundness is exercised by corrupting the
// first fixture's honest proof one value at a time.
func TestGnarkVerifier_RangeCheckScenarios(t *testing.T) {
	for i, build := range wioptest.RangeCheckCompilerScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			if i == 0 {
				requireTamperingRejected(t, gv, proof, pub)
			}
		})
	}
}
