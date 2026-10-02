package pcs

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// selfAssignLagrange is a prover action that fills a LagrangeEval's claim cells
// from the committed column assignments. In the real pipeline the global pass
// owns this; here the test supplies it directly.
type selfAssignLagrange struct{ le *wiop.LagrangeEval }

func (a *selfAssignLagrange) Run(rt *wiop.Runtime) { a.le.SelfAssign(rt) }

func baseVec(n int, val uint64) *wiop.ConcreteVector {
	elems := make([]field.Element, n)
	var e field.Element
	e.SetUint64(val)
	for i := range elems {
		elems[i] = e
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}
}

// newPCSTestSystem builds the smallest protocol the PCS pass can compile: a
// size-4 oracle column committed in round 0, evaluated at a verifier coin in
// round 1 via a LagrangeEval. The claim cell is self-assigned by a round-1
// prover action so sys.Prove drives the whole flow.
func newPCSTestSystem() (*wiop.System, *wiop.Column, *wiop.LagrangeEval) {
	sys := wiop.NewSystemf("pcs-it")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	return sys, col, le
}

// TestCompileEndToEnd checks that an honest witness passes through the full
// commit → open → verify flow that Compile wires up.
func TestCompileEndToEnd(t *testing.T) {
	sys, col, _ := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// The committed column must not survive as raw data in the proof.
	require.NotNil(t, proof.PCSOpeningProof, "proof must carry the FRI opening proof")
	require.NotEmpty(t, proof.Commitments, "proof must carry the round commitments")

	require.NoError(t, sys.Verify(proof, pub), "honest witness must verify")
}

// TestCompileRejectsWrongClaim checks that tampering with a claimed evaluation
// (the value the FRI opening is meant to bind) is rejected.
func TestCompileRejectsWrongClaim(t *testing.T) {
	sys, col, le := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// The true evaluation of the constant-3 column is 3; claim 0 instead.
	proof.Cells[le.EvaluationClaims[0].Context.ID] = field.ElemZero()

	require.Error(t, sys.Verify(proof, pub), "a tampered evaluation claim must be rejected")
}

// TestCompileDynamicModule checks the full flow when the committed column lives
// in a dynamic module, whose size is only known at prove time. The FRI
// parameters are built from the runtime size on both the prover and verifier.
func TestCompileDynamicModule(t *testing.T) {
	sys := wiop.NewSystemf("pcs-dyn")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})

	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(8, 5)) // size fixed to 8 at prove time
	})
	require.Equal(t, 8, proof.DynamicSizes[0], "dynamic size must travel in the proof")
	require.NoError(t, sys.Verify(proof, pub), "honest dynamic-module witness must verify")
}

// TestCompileDynamicModulePaddingLeft checks the full commit→open→verify flow
// for a dynamic PaddingDirectionLeft module whose column assignment is shorter
// than the allocated domain size, so that actual padding rows are present.
//
// PaddingDirectionLeft places padding at the beginning of the domain and data at
// the end: [pad…pad | data]. Before the fix, writeDownVectorBase/Ext ignored the
// padding direction and always produced [data | pad…pad], committing a different
// polynomial than the one evalLagrangePadded evaluates. The DEEP-quotient
// numerator then had unintended poles, so FRI folding failed with "final layer
// has nonzero coefficient, not low-degree enough."
func TestCompileDynamicModulePaddingLeft(t *testing.T) {
	sys := wiop.NewSystemf("pcs-left")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionLeft)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})

	Compile(sys)

	// 5 non-constant elements → domain rounds to 8, leaving 3 padding rows.
	// PaddingDirectionLeft commits [0,0,0,1,2,3,4,5]; the old code committed
	// [1,2,3,4,5,0,0,0] — a distinct degree-7 polynomial — causing FRI failure.
	elems := make([]field.Element, 5)
	for i := range elems {
		elems[i].SetUint64(uint64(i + 1))
	}
	data := &wiop.ConcreteVector{Plain: field.VecFromBase(elems)}

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, data)
	})

	require.NoError(t, sys.Verify(proof, pub), "left-padded column must verify")
}

// TestCompileRejectsTamperedCommitment checks that corrupting a transported
// round commitment is rejected.
func TestCompileRejectsTamperedCommitment(t *testing.T) {
	sys, col, _ := newPCSTestSystem()
	Compile(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(col, baseVec(4, 3))
	})

	// Round 0 owns the only committed batch; flip a byte of its root.
	root := proof.Commitments[0]
	one := field.One()
	root[0].Add(&root[0], &one)
	proof.Commitments[0] = root

	require.Error(t, sys.Verify(proof, pub), "a tampered commitment must be rejected")
}

// TestCompile_PerSystemFRINumQueries checks that each compiled System opens
// exactly its own configured query count, independently of other Systems
// compiled and proved concurrently in the same process.
func TestCompile_PerSystemFRINumQueries(t *testing.T) {
	cases := []struct {
		name         string
		opts         []Option
		want         int
		expectedFail bool
	}{
		{name: "default", want: defaultFRINumQueries},
		{name: "two", opts: []Option{WithFRINumQueries(2)}, want: 2},
		{name: "four", opts: []Option{WithFRINumQueries(4)}, want: 4},
		{name: "overwrite", opts: []Option{WithFRINumQueries(4), WithFRINumQueries(2)}, expectedFail: true},
	}
	const repetitions = 3
	for rep := range repetitions {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/rep%d", tc.name, rep), func(t *testing.T) {
				t.Parallel()
				sys, col, _ := newPCSTestSystem()
				if tc.expectedFail {
					numRounds := len(sys.Rounds)
					require.Panics(t, func() { Compile(sys, tc.opts...) },
						"overwriting an already set query count must be rejected")
					require.Len(t, sys.Rounds, numRounds, "rejected options must not add an opening round")
					require.False(t, sys.Rounds[0].HasCommitment, "rejected options must not commit columns")
					require.Zero(t, FRINumQueries(sys), "rejected options must not record a query count")
					return
				}
				Compile(sys, tc.opts...)
				require.Equal(t, tc.want, FRINumQueries(sys), "compiled System must record its query count")

				proof, pub := sys.Prove(func(rt *wiop.Runtime) {
					rt.AssignColumn(col, baseVec(4, 3))
				})
				require.NotNil(t, proof.PCSOpeningProof, "proof must carry the FRI opening proof")
				require.Len(t, proof.PCSOpeningProof.InputQueries, tc.want,
					"one input query per configured FRI query")
				require.Len(t, proof.PCSOpeningProof.FRIProof.RunningQueries, tc.want,
					"one running query per configured FRI query")
				require.NoError(t, sys.Verify(proof, pub), "honest witness must verify")
			})
		}
	}
}

// TestCompile_RejectsNonPositiveFRINumQueries checks that an invalid query count
// is rejected before Compile touches the System.
func TestCompile_RejectsNonPositiveFRINumQueries(t *testing.T) {
	for _, n := range []int{0, -1} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			sys, _, _ := newPCSTestSystem()
			numRounds := len(sys.Rounds)
			numActions := len(sys.Rounds[0].ProverActions)

			require.Panics(t, func() { Compile(sys, WithFRINumQueries(n)) },
				"a non-positive query count must be rejected")

			require.Len(t, sys.Rounds, numRounds, "no opening round may be added")
			require.False(t, sys.Rounds[0].HasCommitment, "no round may be flagged as committed")
			require.Len(t, sys.Rounds[0].ProverActions, numActions, "no commit action may be registered")
			require.Zero(t, FRINumQueries(sys), "no query count may be recorded")
		})
	}
}

// TestCompile_NoCommittedColumnsRecordsNoQueries checks that the no-op path
// leaves no opening round and reports no FRI queries.
func TestCompile_NoCommittedColumnsRecordsNoQueries(t *testing.T) {
	sys := wiop.NewSystemf("pcs-empty")
	sys.NewRound()

	Compile(sys, WithFRINumQueries(2))

	require.Len(t, sys.Rounds, 1, "no opening round without committed columns")
	require.Zero(t, FRINumQueries(sys), "no FRI opening means no query count")
}

// TestCompile_PerSystemRSBlowup checks that each compiled System commits and
// opens at its own configured blowup, independently of Systems at other
// blowups compiled and proved concurrently in the same process.
func TestCompile_PerSystemRSBlowup(t *testing.T) {
	cases := []struct {
		name        string
		opts        []Option
		want        int
		wantMaxSize uint8
	}{
		{name: "default", want: defaultRSBlowup, wantMaxSize: 22},
		{name: "four", opts: []Option{WithRSBlowup(4)}, want: 4, wantMaxSize: 22},
		{name: "eight", opts: []Option{WithRSBlowup(8)}, want: 8, wantMaxSize: 21},
		{name: "sixteen", opts: []Option{WithRSBlowup(16)}, want: 16, wantMaxSize: 20},
	}
	const repetitions = 3
	for rep := range repetitions {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/rep%d", tc.name, rep), func(t *testing.T) {
				t.Parallel()
				sys, col, _ := newPCSTestSystem()
				Compile(sys, append([]Option{WithFRINumQueries(2)}, tc.opts...)...)

				require.Equal(t, tc.want, RSBlowup(sys), "compiled System must record its blowup")
				require.Equal(t, tc.wantMaxSize, FRIMaxCommittableSizeLog2(sys),
					"row limit must shrink once blowup·2^22 exceeds the field's 2-adicity")
				params := FRIStaticParams(sys)
				require.Equal(t, tc.want, 1<<(params.LogCodewordSize-params.LogPlainTextSize),
					"FRI envelope must be built at the System's blowup")

				proof, pub := sys.Prove(func(rt *wiop.Runtime) {
					rt.AssignColumn(col, baseVec(4, 3))
				})
				require.NoError(t, sys.Verify(proof, pub), "honest witness must verify")
			})
		}
	}
}

// TestCompile_RSBlowupBindsProof checks that the blowup is part of what a proof
// commits to: the same witness commits differently at two blowups, and a proof
// produced at one blowup is rejected by an otherwise identical System at another.
func TestCompile_RSBlowupBindsProof(t *testing.T) {
	prove := func(blowup int) (*wiop.System, wiop.Proof, wiop.PublicInput) {
		sys, col, _ := newPCSTestSystem()
		Compile(sys, WithFRINumQueries(2), WithRSBlowup(blowup))
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(col, baseVec(4, 3))
		})
		return sys, proof, pub
	}
	_, proof2, pub2 := prove(2)
	sys4, proof4, _ := prove(4)

	require.NotEqual(t, proof2.Commitments[0], proof4.Commitments[0],
		"the same witness must commit to different codewords at different blowups")
	require.Error(t, sys4.Verify(proof2, pub2),
		"a blowup-2 proof must not verify against a blowup-4 System")
}

// TestWithRSBlowup_RejectsInvalid checks that blowups that are not powers of
// two in [2, maxRSBlowup] are rejected when the option is built.
func TestWithRSBlowup_RejectsInvalid(t *testing.T) {
	for _, blowup := range []int{-2, 0, 1, 3, 6, 2 * maxRSBlowup} {
		t.Run(fmt.Sprint(blowup), func(t *testing.T) {
			require.Panics(t, func() { WithRSBlowup(blowup) },
				"blowup %d must be rejected", blowup)
		})
	}
}

// TestCompile_RejectsRepeatedRSBlowup checks that setting the blowup twice is
// rejected before Compile touches the System.
func TestCompile_RejectsRepeatedRSBlowup(t *testing.T) {
	sys, _, _ := newPCSTestSystem()
	numRounds := len(sys.Rounds)

	require.Panics(t, func() { Compile(sys, WithRSBlowup(4), WithRSBlowup(8)) },
		"overwriting an already set blowup must be rejected")

	require.Len(t, sys.Rounds, numRounds, "rejected options must not add an opening round")
	require.False(t, sys.Rounds[0].HasCommitment, "rejected options must not commit columns")
	require.Zero(t, RSBlowup(sys), "rejected options must not record a blowup")
}

// tinyRowLimitBlowup leaves a row limit of 2^1 on KoalaBear (2^(24-23)), so
// row-limit violations are testable with 4-row columns.
const tinyRowLimitBlowup = 1 << (field.MaxOrderRoot - 1)

// TestCompile_RSBlowupRejectsOversizedStaticColumn checks that a static column
// above the blowup's row limit is rejected before Compile touches the System.
func TestCompile_RSBlowupRejectsOversizedStaticColumn(t *testing.T) {
	sys, _, _ := newPCSTestSystem() // 4-row static column
	numRounds := len(sys.Rounds)

	require.Panics(t, func() { Compile(sys, WithRSBlowup(tinyRowLimitBlowup)) },
		"a 4-row column must be rejected when the row limit is 2")

	require.Len(t, sys.Rounds, numRounds, "no opening round may be added")
	require.False(t, sys.Rounds[0].HasCommitment, "no round may be flagged as committed")
	require.Zero(t, RSBlowup(sys), "no blowup may be recorded")
}

// newDynamicPCSTestSystem is [newPCSTestSystem] with the committed column in a
// dynamic module, compiled at tinyRowLimitBlowup.
func newDynamicPCSTestSystem() (*wiop.System, *wiop.Column) {
	sys := wiop.NewSystemf("pcs-dyn-rowlimit")
	r0 := sys.NewRound()
	r1 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("col"), r0)
	zeta := r1.NewCoinField(sys.Context.Childf("zeta"))
	le := sys.NewLagrangeEval(sys.Context.Childf("le"), []*wiop.ColumnView{col.View()}, zeta)
	r1.RegisterAction(&selfAssignLagrange{le: le})
	Compile(sys, WithFRINumQueries(2), WithRSBlowup(tinyRowLimitBlowup))
	return sys, col
}

// TestCompile_RSBlowupRejectsOversizedDynamicColumn checks that the prover
// refuses to commit a dynamic column above the blowup's row limit.
func TestCompile_RSBlowupRejectsOversizedDynamicColumn(t *testing.T) {
	sys, col := newDynamicPCSTestSystem()

	require.PanicsWithValue(t,
		"pcs: round 0 commits a column of size 2^2, above the 2^1 committable at RS blowup 8388608",
		func() {
			sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, baseVec(4, 3)) })
		}, "a 4-row dynamic column must be rejected when the row limit is 2")
}

// TestVerify_Regression_ForgedDynamicSizeAboveRowLimit checks that a proof
// claiming a dynamic size above the blowup's row limit — but within
// [wiop.ColumnSizeMaxSupported], so System.Verify's own cap lets it through — is
// rejected with an error rather than a panic while deriving FRI domains.
func TestVerify_Regression_ForgedDynamicSizeAboveRowLimit(t *testing.T) {
	sys, col := newDynamicPCSTestSystem()
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(col, baseVec(2, 3)) })
	require.NoError(t, sys.Verify(proof, pub), "honest 2-row witness must verify")

	proof.DynamicSizes[0] = 4
	var err error
	require.NotPanics(t, func() { err = sys.Verify(proof, pub) }, "a forged size must not panic the verifier")
	require.ErrorContains(t, err, "exceeds the 2^1 committable", "a forged size above the row limit must be rejected")
}
