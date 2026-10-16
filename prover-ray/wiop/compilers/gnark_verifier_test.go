package compilers_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// gnarkVerifier is the verifier circuit of a fully compiled system, compiled
// once over BN254 so several witnesses can be solved against it.
type gnarkVerifier struct {
	sys         *wiop.System
	template    wiop.Proof
	templatePub wiop.PublicInput
	ccs         interface {
		Solve(witness.Witness, ...solver.Option) (any, error)
		GetNbConstraints() int
	}
}

func compileGnarkVerifier(t *testing.T, sys *wiop.System, template wiop.Proof, pub wiop.PublicInput) *gnarkVerifier {
	t.Helper()
	gv := &gnarkVerifier{sys: sys, template: template, templatePub: pub}
	var err error
	gv.ccs, err = frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder,
		wiop.AllocateVerifierCircuit(sys, template, pub))
	require.NoError(t, err, "verifier circuit must compile")
	return gv
}

func (gv *gnarkVerifier) solve(t *testing.T, proof wiop.Proof, pub wiop.PublicInput) error {
	t.Helper()
	assignment := wiop.AllocateVerifierCircuit(gv.sys, gv.template, gv.templatePub).
		AssignVerifierCircuit(proof, pub)
	w, err := frontend.NewWitness(assignment, ecc.BN254.ScalarField())
	require.NoError(t, err, "assignment must produce a witness")
	_, err = gv.ccs.Solve(w)
	return err
}

// compileGnarkPipeline is [compileFullPipeline] with column elision and root
// deduplication disabled, which the in-circuit PCS verifier requires.
func compileGnarkPipeline(sys *wiop.System) {
	compilePipelineBeforePCS(sys)
	pcs.Compile(sys, pcs.CompileOptions{DisableColumnElision: true, DisableRootDedup: true})
}

// withCell returns a copy of proof whose cell id is shifted by one.
func withCell(proof wiop.Proof, id wiop.ObjectID) wiop.Proof {
	cells := make(map[wiop.ObjectID]field.Gen, len(proof.Cells))
	for k, v := range proof.Cells {
		cells[k] = v
	}
	cells[id] = cells[id].Add(field.ElemFromBase(field.One()))
	bad := proof
	bad.Cells = cells
	return bad
}

// grandProductResults returns the Result cells of every GrandProduct the
// grandproduct pass registered a verifier action for.
func grandProductResults(sys *wiop.System) []*wiop.Cell {
	var res []*wiop.Cell
	for _, r := range sys.Rounds {
		for _, va := range r.VerifierActions {
			if a, ok := va.(*grandproduct.FinalProductCheck); ok {
				res = append(res, a.GrandProduct.Result)
			}
		}
	}
	return res
}

// TestGnarkVerifier_PermutationScenarios checks that the verifier circuit of
// a permutation-compiled system accepts exactly the proofs the native verifier
// accepts: honest proofs pass, while a non-permutation witness or a tampered
// grand-product Result are rejected.
func TestGnarkVerifier_PermutationScenarios(t *testing.T) {
	for _, build := range wioptest.PermutationScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileGnarkPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			compileGnarkPipeline(invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")
			require.Error(t, gv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")

			results := grandProductResults(sc.Sys)
			require.NotEmpty(t, results, "grandproduct must register a final-product check")
			for _, res := range results {
				bad := withCell(proof, res.Context.ID)
				require.Error(t, sc.Sys.Verify(bad, pub), "tampered Result must fail natively")
				require.Error(t, gv.solve(t, bad, pub), "tampered Result must not satisfy the circuit")
			}
		})
	}
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
			compileGnarkPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
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
			compileGnarkPipeline(invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignWitness)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "wrong Result must fail natively")
			require.Error(t, gv.solve(t, badProof, badPub), "wrong Result must not satisfy the circuit")
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
			compileGnarkPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignWitness)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			before := snapshotModuleColumns(invalid.Sys)
			lookuptologderivsum.Compile(invalid.Sys)
			mCols := newBaseColumns(invalid.Sys, before)
			require.NotEmpty(t, mCols, "lookuptologderivsum must allocate a multiplicity column")
			wioptest.Mutator{Column: mCols[0], Row: 0, Tweak: wioptest.AddOne}.Compile(invalid.Sys)
			compileGnarkPipeline(invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignWitness)
			if invalid.Sys.Verify(badProof, badPub) == nil {
				// The bumped row is masked by the table's filter, so the
				// tampered M is still a valid multiplicity assignment.
				t.Skip("tampered M row does not contribute to the lookup sum")
			}
			require.Error(t, gv.solve(t, badProof, badPub), "wrong multiplicity must not satisfy the circuit")
		})
	}
}
