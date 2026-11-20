package wiop_test

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// testFRINumQueries keeps the FRI part of the test circuits small. Soundness
// of the tests does not rest on FRI: the tampering they exercise is caught by
// Merkle authentication and the algebraic identities, not by query sampling.
const testFRINumQueries = 2

// testPCSOptions compiles a test system at the small test query count. Column
// elision stays off (the default): the circuit supports only all-Present
// manifests.
func testPCSOptions() []pcs.Option {
	return []pcs.Option{pcs.WithFRINumQueries(testFRINumQueries)}
}

type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

// nonConstVec returns a column whose interpolant is not constant.
//
// This matters for every test that exercises the PCS. FRI verifies the deep
// quotient (f(x) − claim)/(x − zeta). When f is constant, the claimed
// evaluation equals that constant at *every* zeta and the encoded codeword is
// that constant at every position, so the numerator vanishes identically. The
// quotient is then zero whatever zeta and the fold challenges are, zeros fold
// to zeros, and no constraint in checkFolds depends on the transcript any
// more — leaving the circuit's Fiat-Shamir derivation completely untested
// while the test still reports success.
func nonConstVec(n int) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	for i := range elems {
		elems[i].SetUint64(uint64(i*i + 1))
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// newPCSOnlySystem builds a minimal PCS-compiled system: one committed
// column opened at a coin through a LagrangeEval.
func newPCSOnlySystem() (*wiop.System, *wiop.Column, *wiop.LagrangeEval) {
	sys := wiop.NewSystemf("gnark-pcs")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, testPCSOptions()...)
	return sys, col, le
}

// compiledVerifier is the verifier circuit of a system compiled once over
// BN254, so that many witnesses can be solved against it without paying
// for compilation again (it dominates the cost of the emulated circuit).
type compiledVerifier struct {
	sys     *wiop.System
	circ    *wiop.VerifierCircuit
	modulus *big.Int
	ccs     interface {
		Solve(witness.Witness, ...solver.Option) (any, error)
		GetNbConstraints() int
	}
}

// compileVerifierCircuit compiles the verifier circuit of sys over BN254. The
// circuit is allocated from the System alone: no proof is needed.
func compileVerifierCircuit(t *testing.T, sys *wiop.System) *compiledVerifier {
	t.Helper()
	modulus := ecc.BN254.ScalarField()
	cv := &compiledVerifier{sys: sys, circ: wiop.AllocateVerifierCircuit(sys), modulus: modulus}
	var err error
	cv.ccs, err = frontend.Compile(modulus, scs.NewBuilder, cv.circ)
	require.NoError(t, err, "verifier circuit must compile")
	return cv
}

// solve assigns (proof, pub) to the compiled circuit and returns the error from
// solving it.
func (cv *compiledVerifier) solve(t *testing.T, proof wiop.Proof, pub wiop.PublicInput) error {
	t.Helper()
	assignment := cv.circ.AssignVerifierCircuit(proof, pub)
	w, err := frontend.NewWitness(assignment, cv.modulus)
	require.NoError(t, err, "assignment must produce a witness")
	_, err = cv.ccs.Solve(w)
	return err
}

func TestVerifierCircuit_PCSOnly(t *testing.T) {
	sys, col, le := newPCSOnlySystem()
	// Must be non-constant, or the deep quotient vanishes and the circuit's
	// Fiat-Shamir derivation stops being constrained by anything. See
	// [nonConstVec].
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(8)) })
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

	cv := compileVerifierCircuit(t, sys)
	t.Run("honest", func(t *testing.T) {
		require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
		t.Logf("constraints (emulated over BN254): %d", cv.ccs.GetNbConstraints())
	})
	t.Run("tampered-claim", func(t *testing.T) {
		bad := cloneProof(proof)
		bad.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemFromExt(field.Uint64ToExt(7))
		require.Error(t, sys.Verify(bad, pub), "tampered claim must fail natively")
		require.Error(t, cv.solve(t, bad, pub), "tampered claim must not satisfy the circuit")
	})
	t.Run("tampered-commitment", func(t *testing.T) {
		bad := cloneProof(proof)
		root := bad.Commitments[0]
		one := field.One()
		root[0].Add(&root[0], &one)
		bad.Commitments[0] = root
		require.Error(t, cv.solve(t, bad, pub), "tampered commitment must not satisfy the circuit")
	})
}

// TestVerifierCircuit_PCSOnly_RSBlowup checks that the circuit replays the
// opening at the System's blowup: query positions are drawn from the blowup's
// codeword domain, so a circuit assuming another rate rejects honest proofs.
func TestVerifierCircuit_PCSOnly_RSBlowup(t *testing.T) {
	for _, blowup := range []int{4, 16} {
		t.Run(fmt.Sprintf("blowup=%d", blowup), func(t *testing.T) {
			sys := wiop.NewSystemf("gnark-pcs-blowup")
			r0 := sys.NewRound()
			r1 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
			col := mod.NewColumn(sys.Context.Childf("col"), r0)
			zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
			le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
			r1.RegisterAction(&selfAssignLagrange{le: le})
			pcs.Compile(sys, append(testPCSOptions(), pcs.WithRSBlowup(blowup))...)

			proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(8)) })
			require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")
			cv := compileVerifierCircuit(t, sys)
			require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
		})
	}
}

// newPCSOnlyCapSystem is a [newPCSOnlySystem] over a 16-row column, so its
// Merkle trees are tall enough (input tree of height 5, running layers of
// height 4, 3 and 2) for distinct cap heights to clamp differently.
func newPCSOnlyCapSystem(capOpt pcs.Option) (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("gnark-pcs-cap")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 16, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, append(testPCSOptions(), capOpt)...)
	return sys, col
}

// TestVerifierCircuit_PCSOnly_MerkleCap checks that the circuit verifies
// honest proofs at every cap height the System can be compiled with: no cap,
// the query-count optimum, a height below every tree's height-1, and one
// clamped to height-1 on every tree.
func TestVerifierCircuit_PCSOnly_MerkleCap(t *testing.T) {
	for _, tc := range []struct {
		name string
		opt  pcs.Option
		want int
	}{
		{name: "none", opt: pcs.WithMerkleCapHeight(0), want: 0},
		{name: "optimal", opt: pcs.WithOptimalMerkleCap(), want: 1},
		{name: "height=3", opt: pcs.WithMerkleCapHeight(3), want: 3},
		{name: "height=24", opt: pcs.WithMerkleCapHeight(24), want: 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sys, col := newPCSOnlyCapSystem(tc.opt)
			require.Equal(t, tc.want, pcs.MerkleCapHeight(sys), "System must record its cap height")

			proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(16)) })
			require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")
			cv := compileVerifierCircuit(t, sys)
			require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
		})
	}
}

// TestVerifierCircuit_PCSOnly_MerkleCapMismatch checks that the circuit takes
// the cap height from the System, not from the proof: a proof made at another
// cap height does not have the circuit's geometry and must be refused.
func TestVerifierCircuit_PCSOnly_MerkleCapMismatch(t *testing.T) {
	verifierSys, _ := newPCSOnlyCapSystem(pcs.WithMerkleCapHeight(0))
	proverSys, col := newPCSOnlyCapSystem(pcs.WithMerkleCapHeight(3))
	proof, pub := proverSys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, nonConstVec(16)) })
	require.NoError(t, proverSys.Verify(proof, pub), "honest proof must verify at its own cap height")
	require.Error(t, verifierSys.Verify(proof, pub), "a capped proof must fail natively without the cap")

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("circuit definition panicked: %v", r)
			}
		}()
		modulus := ecc.BN254.ScalarField()
		circ := wiop.AllocateVerifierCircuit(verifierSys)
		ccs, err := frontend.Compile(modulus, scs.NewBuilder, circ)
		if err != nil {
			return err
		}
		assignment := circ.AssignVerifierCircuit(proof, pub)
		w, err := frontend.NewWitness(assignment, modulus)
		if err != nil {
			return err
		}
		_, err = ccs.Solve(w)
		return err
	}()
	require.ErrorContains(t, err, "the PCS opening proof does not have the circuit's geometry",
		"a capped proof must not satisfy the uncapped System's circuit")
}

// TestVerifierCircuit_PCSModuleGroups checks the circuit verifier on a system
// compiled with module groups: modules A and B share a size and have
// interleaved columns, so their rows are reordered and committed as two
// groups, either below a larger module or as the largest size of the round,
// which pair leaves let them split.
func TestVerifierCircuit_PCSModuleGroups(t *testing.T) {
	for _, sizes := range [][2]int{{8, 32}, {32, 8}} {
		verifyModuleGroupsCircuit(t, sizes[0], sizes[1])
	}
}

// TestVerifierCircuit_PCSFixedFRIDomain checks the circuit verifier on a
// system compiled with module groups and a fixed FRI domain, whose top rounds
// fold the zero codeword.
func TestVerifierCircuit_PCSFixedFRIDomain(t *testing.T) {
	verifyModuleGroupsCircuit(t, 8, 32, pcs.WithFixedFRIDomain())
}

func verifyModuleGroupsCircuit(t *testing.T, sizeAB, sizeBig int, opts ...pcs.Option) {
	t.Helper()
	sys := wiop.NewSystemf("gnark-pcs-groups")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	a := sys.NewSizedModule(sys.Context.Childf("A"), sizeAB, wiop.PaddingDirectionNone)
	b := sys.NewSizedModule(sys.Context.Childf("B"), sizeAB, wiop.PaddingDirectionNone)
	big := sys.NewSizedModule(sys.Context.Childf("Big"), sizeBig, wiop.PaddingDirectionNone)
	cols := []*wiop.Column{
		b.NewColumn(sys.Context.Childf("b0"), r0),
		a.NewColumn(sys.Context.Childf("a0"), r0),
		b.NewColumn(sys.Context.Childf("b1"), r0),
		big.NewColumn(sys.Context.Childf("big"), r0),
	}
	views := make([]*wiop.ColumnView, len(cols))
	for i, col := range cols {
		views[i] = col.View()
	}
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), views, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, append(append(testPCSOptions(), pcs.WithModuleGroups()), opts...)...)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		for _, col := range cols[:3] {
			rt.AssignColumn(col, nonConstVec(sizeAB))
		}
		rt.AssignColumn(cols[3], nonConstVec(sizeBig))
	})
	require.NoError(t, sys.Verify(proof, pub), "honest proof must verify natively")

	cv := compileVerifierCircuit(t, sys)
	require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
	bad := cloneProof(proof)
	bad.Cells[le.EvaluationClaims[1].Context.ID] = field.ElemFromExt(field.Uint64ToExt(7))
	require.Error(t, cv.solve(t, bad, pub), "tampered claim must not satisfy the circuit")
}

func TestVerifierCircuit_Vanishing(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		if hasDynamicModule(sc.Sys) {
			continue
		}
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			pcs.Compile(sc.Sys, testPCSOptions()...)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			// An invalid witness yields a proof of the same shape that the
			// native verifier rejects; the circuit must reject it too.
			invalid := build()
			global.Compile(invalid.Sys)
			pcs.Compile(invalid.Sys, testPCSOptions()...)
			badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
			require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")

			cv := compileVerifierCircuit(t, sc.Sys)
			require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
			t.Logf("constraints: %d", cv.ccs.GetNbConstraints())
			require.Error(t, cv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")
		})
	}
}

// TestVerifierCircuit_Vanishing_TamperedProof starts from an honest proof and
// corrupts one value at a time: every cell, then every round commitment. Each
// corrupted proof must be rejected both natively and by the circuit.
func TestVerifierCircuit_Vanishing_TamperedProof(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		if hasDynamicModule(sc.Sys) {
			continue
		}
		t.Run(sc.Name, func(t *testing.T) {
			global.Compile(sc.Sys)
			pcs.Compile(sc.Sys, testPCSOptions()...)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

			one := field.One()
			cellIDs := slices.Sorted(maps.Keys(proof.Cells))
			rounds := slices.Sorted(maps.Keys(proof.Commitments))
			require.NotEmpty(t, cellIDs, "scenario must expose at least one cell")
			require.NotEmpty(t, rounds, "scenario must expose at least one commitment")

			tamperCell := func(id wiop.ObjectID) wiop.Proof {
				bad := cloneProof(proof)
				bad.Cells[id] = bad.Cells[id].Add(field.ElemFromBase(one))
				return bad
			}
			tamperCommitment := func(round int) wiop.Proof {
				bad := cloneProof(proof)
				root := bad.Commitments[round]
				root[0].Add(&root[0], &one)
				bad.Commitments[round] = root
				return bad
			}

			cv := compileVerifierCircuit(t, sc.Sys)
			for _, id := range cellIDs {
				bad := tamperCell(id)
				require.Errorf(t, sc.Sys.Verify(bad, pub), "tampered cell %d must fail natively", id)
				require.Errorf(t, cv.solve(t, bad, pub), "tampered cell %d must not satisfy the circuit", id)
			}
			for _, round := range rounds {
				require.Errorf(t, cv.solve(t, tamperCommitment(round), pub), "tampered commitment of round %d must not satisfy the circuit", round)
			}
		})
	}
}

// unsupportedAction is a verifier action without an in-circuit counterpart.
type unsupportedAction struct{}

func (unsupportedAction) Check(*wiop.Runtime) error { return nil }

func TestAllocateVerifierCircuit_RejectsUnsupportedAction(t *testing.T) {
	sys, _, _ := newPCSOnlySystem()
	sys.Rounds[1].RegisterVerifierAction(unsupportedAction{})

	require.Panics(t, func() { wiop.AllocateVerifierCircuit(sys) },
		"a verifier action without CheckGnark must be rejected instead of dropped")
}

func TestAssignVerifierCircuit_RejectsShapeMismatch(t *testing.T) {
	sys, col, le := newPCSOnlySystem()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, wioptest.ConstVec(8, 3)) })
	circ := wiop.AllocateVerifierCircuit(sys)

	bad := cloneProof(proof)
	// The claim is declared as an extension element; retagging it as base
	// changes the transcript layout and must be refused.
	bad.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemFromBase(field.NewElement(3))
	require.PanicsWithValue(t,
		`wiop: VerifierCircuit: cell "gnark-pcs/le/claim[0]" does not carry its declared field tag (extension=true)`,
		func() { circ.AssignVerifierCircuit(bad, pub) },
		"a proof whose cell field tags differ from the declarations must be rejected")
	require.ErrorContains(t, sys.Verify(bad, pub), "does not carry its declared field tag",
		"the native verifier must refuse the retagged proof too")
}

func hasDynamicModule(sys *wiop.System) bool {
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			return true
		}
	}
	return false
}

func cloneProof(p wiop.Proof) wiop.Proof {
	res := wiop.Proof{
		Cells:           make(map[wiop.ObjectID]field.Gen, len(p.Cells)),
		DynamicSizes:    make(map[int]int, len(p.DynamicSizes)),
		Commitments:     make(map[int]field.Octuplet, len(p.Commitments)),
		PCSOpeningProof: p.PCSOpeningProof,
	}
	for k, v := range p.Cells {
		res.Cells[k] = v
	}
	for k, v := range p.DynamicSizes {
		res.DynamicSizes[k] = v
	}
	for k, v := range p.Commitments {
		res.Commitments[k] = v
	}
	return res
}

// TestVerifierCircuit_FixedDynamicSizes covers the terminal-layer recipe for
// dynamic modules: pin every dynamic module to a fixed size before compiling
// ([wiop.System.FixDynamicSizes]), which makes the proof shape, and so the
// circuit, independent of the trace. Each dynamic vanishing scenario is fixed at
// its natural size, where the honest proof must be accepted natively and
// in-circuit and an invalid witness rejected by both.
//
// At twice the natural size the data no longer fills the module and padding
// rows appear. Padding is only sound for constraints that hold on padding rows,
// which is already what a dynamic module requires whenever its trace length is
// not a power of two. Fixing the size does not change that requirement:
//   - DynamicQuadratic (col² = col, zero padding) is padding-safe and must pass
//     end to end;
//   - DynamicFibonacci and LeftPadDynamic exactly fill size 8 and their
//     recurrences break at the first padding row, so the honest witness must be
//     rejected natively.
func TestVerifierCircuit_FixedDynamicSizes(t *testing.T) {
	paddingSafe := map[string]bool{"DynamicQuadratic": true}
	ran := 0
	for _, build := range wioptest.VanishingScenarios() {
		probe := build()
		if !hasDynamicModule(probe.Sys) {
			continue
		}
		// Prove the dynamic system once to learn the sizes its data needs.
		global.Compile(probe.Sys)
		pcs.Compile(probe.Sys, testPCSOptions()...)
		probeProof, _ := probe.Sys.Prove(probe.AssignHonest)
		natural := probeProof.DynamicSizes
		require.NotEmpty(t, natural, "%s: a dynamic proof must carry its sizes", probe.Name)

		for _, factor := range []int{1, 2} {
			sizes := make(map[int]int, len(natural))
			for k, v := range natural {
				sizes[k] = v * factor
			}
			fixed := func() *wioptest.VanishingScenario {
				sc := build()
				require.NoError(t, sc.Sys.FixDynamicSizes(sizes))
				require.False(t, hasDynamicModule(sc.Sys), "every dynamic module must be fixed")
				global.Compile(sc.Sys)
				pcs.Compile(sc.Sys, testPCSOptions()...)
				return sc
			}
			padded := factor > 1
			t.Run(fmt.Sprintf("%s/x%d", probe.Name, factor), func(t *testing.T) {
				ran++
				sc := fixed()
				proof, pub := sc.Sys.Prove(sc.AssignHonest)
				require.Empty(t, proof.DynamicSizes, "a fixed system carries no dynamic sizes")
				if padded && !paddingSafe[probe.Name] {
					require.ErrorContains(t, sc.Sys.Verify(proof, pub), "global quotient check failed",
						"constraints that fail on padding rows must reject the padded witness")
					return
				}
				require.NoError(t, sc.Sys.Verify(proof, pub), "honest proof must verify natively")

				invalid := fixed()
				badProof, badPub := invalid.Sys.Prove(invalid.AssignInvalid)
				require.Error(t, invalid.Sys.Verify(badProof, badPub), "invalid witness must fail natively")

				cv := compileVerifierCircuit(t, sc.Sys)
				require.NoError(t, cv.solve(t, proof, pub), "honest proof must satisfy the circuit")
				t.Logf("sizes %v: %d constraints", sizes, cv.ccs.GetNbConstraints())
				require.Error(t, cv.solve(t, badProof, badPub), "invalid witness must not satisfy the circuit")
			})
		}
	}
	require.Equal(t, 6, ran, "expected three dynamic scenarios at two sizes each")
}

// TestVerifierCircuit_CompiledBeforeAnyProof is the point of allocating from
// the System: the circuit is compiled right after the System, before anything
// is proven, and then accepts honest proofs of different witnesses. The
// constraint system is a function of the System alone, so allocating twice
// yields the same one.
func TestVerifierCircuit_CompiledBeforeAnyProof(t *testing.T) {
	sys, col, _ := newPCSOnlySystem()
	cv := compileVerifierCircuit(t, sys)
	again := compileVerifierCircuit(t, sys)
	require.Equal(t, cv.ccs.GetNbConstraints(), again.ccs.GetNbConstraints(),
		"the constraint system must not depend on the allocation")

	for _, offset := range []uint64{1, 1000} {
		elems := make([]field.Element, 8)
		for i := range elems {
			elems[i].SetUint64(uint64(i*i) + offset)
		}
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(elems)})
		})
		require.NoError(t, sys.Verify(proof, pub), "offset %d: honest proof must verify natively", offset)
		require.NoError(t, cv.solve(t, proof, pub), "offset %d: honest proof must satisfy the precompiled circuit", offset)
	}
}

// TestAllocateVerifierCircuit_RejectsColumnElision checks that a system whose
// batch shapes the prover chooses per proof is refused at allocation.
func TestAllocateVerifierCircuit_RejectsColumnElision(t *testing.T) {
	sys := wiop.NewSystemf("gnark-pcs-elision")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	pcs.Compile(sys, append(testPCSOptions(), pcs.WithColumnElision())...)

	require.PanicsWithValue(t,
		"pcs: OpeningProofShape: all-Present manifests only: compile without WithColumnElision",
		func() { wiop.AllocateVerifierCircuit(sys) })
}
