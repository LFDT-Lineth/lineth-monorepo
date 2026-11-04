package compilers_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
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

// compileGnarkPipeline is [compilers.CompileFull] with root deduplication
// disabled, which the in-circuit PCS verifier requires. Extra options are
// appended, e.g. to opt a round-0 bus out of shared randomness.
func compileGnarkPipeline(t *testing.T, sys *wiop.System, extra ...compilers.Option) {
	t.Helper()
	opts := append(wioptest.TestingCompileOptions(), compilers.ConfigurePCS(pcs.WithoutRootDedup()))
	require.NoError(t, compilers.CompileFull(sys, append(opts, extra...)...))
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
			compileGnarkPipeline(t, sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			compileGnarkPipeline(t, invalid.Sys)
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
			compileGnarkPipeline(t, sc.Sys)
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
			compileGnarkPipeline(t, invalid.Sys)
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
			compileGnarkPipeline(t, sc.Sys)
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
			compileGnarkPipeline(t, invalid.Sys)
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

// newMessageBusSystem declares one handle with a send on module A and a
// receive on module B. The bus balances exactly when B is a reordering of A.
func newMessageBusSystem() (*wiop.System, *wiop.Column, *wiop.Column) {
	sys := wiop.NewSystemf("gnark-mb")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewMessageBusSend(sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))
	return sys, colA, colB
}

// noSharedRandomness opts the round-0 bus of [newMessageBusSystem] out of
// shared randomness, which needs every bus column on the coin round.
var noSharedRandomness = compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness())

// TestGnarkVerifier_MessageBus checks the in-circuit in-shard product check.
// In the invalid proof the receive side holds a value that was never sent:
// the prover derives the handle's product honestly from that witness, so the
// proof is internally consistent and only the product-equals-one check can
// reject it.
func TestGnarkVerifier_MessageBus(t *testing.T) {
	sys, colA, colB := newMessageBusSystem()
	compileGnarkPipeline(t, sys, noSharedRandomness)
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(colA, vecOf(10, 20, 30, 40))
		rt.AssignColumn(colB, vecOf(40, 10, 30, 20))
	})
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

	gv := compileGnarkVerifier(t, sys, proof, pub)
	require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
	t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

	invalid, badA, badB := newMessageBusSystem()
	compileGnarkPipeline(t, invalid, noSharedRandomness)
	badProof, badPub := invalid.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(badA, vecOf(10, 20, 30, 40))
		rt.AssignColumn(badB, vecOf(40, 10, 30, 21)) // 21 was never sent
	})
	require.Error(t, invalid.Verify(badProof, badPub), "unbalanced bus must fail natively")
	require.Error(t, gv.solve(t, badProof, badPub), "unbalanced bus must not satisfy the circuit")
}

// vecOf returns a base-field column assignment holding vals.
func vecOf(vals ...uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, len(vals))
	for i, v := range vals {
		elems[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
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

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")

			invalid := build()
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")
			require.Error(t, gv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")

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

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			invalid := build()
			compileGnarkPipeline(t, invalid.Sys)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")
			require.Error(t, gv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")

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

			gv := compileGnarkVerifier(t, sc.Sys, proof, pub)
			require.NoError(t, gv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", gv.ccs.GetNbConstraints())

			if i == 0 {
				requireTamperingRejected(t, gv, proof, pub)
			}
		})
	}
}
