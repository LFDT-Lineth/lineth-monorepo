package fri

import (
	"math/big"
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bitReverseIdx returns the nbits-wide bit-reversal of i (matching the slice
// permutation gnark's BitReverse applies).
func bitReverseIdx(i, nbits int) int {
	if nbits == 0 {
		return 0
	}
	return int(bits.Reverse64(uint64(i)) >> (64 - nbits))
}

// TestFoldLayerInternally checks that folding a bit-reversed codeword of P with
// challenge alpha yields the bit-reversed codeword of the folded polynomial
// Q(Y) = P_e(Y) + alpha·P_o(Y). The expected codeword is built by an
// independent canonical evaluation, so this also pins down the bit-reversed
// twiddle alignment.
func TestFoldLayerInternally(t *testing.T) {

	prng := rand.New(utils.NewRandSource(1))

	for _, n := range []int{4, 8, 16} {

		var (
			kN     = utils.Log2Ceil(n)
			half   = n / 2
			domain = fft.NewDomain(uint64(n))
			g      = domain.Generator
			alpha  = field.PseudoRandExt(prng)
		)

		// random degree-(n-1) polynomial in canonical (coefficient) form
		coeffs := make([]field.Ext, n)
		for i := range coeffs {
			coeffs[i] = field.PseudoRandExt(prng)
		}

		// bit-reversed codeword: layer[m] = P(g^{bitReverse(m)})
		layer := make([]field.Ext, n)
		for m := range n {
			var x field.Element
			x.Exp(g, big.NewInt(int64(bitReverseIdx(m, kN))))
			layer[m] = polynomials.EvalCanonicalExt(coeffs, field.Lift(x))
		}

		// Q(Y) = P_e(Y) + alpha·P_o(Y): q_i = c_{2i} + alpha·c_{2i+1}
		qcoeffs := make([]field.Ext, half)
		for i := range qcoeffs {
			var odd field.Ext
			odd.Mul(&coeffs[2*i+1], &alpha)
			qcoeffs[i].Add(&coeffs[2*i], &odd)
		}

		// expected: bit-reversed codeword of Q over the half domain (generator g²)
		var g2 field.Element
		g2.Square(&g)
		want := make([]field.Ext, half)
		for tt := range half {
			var y field.Element
			y.Exp(g2, big.NewInt(int64(bitReverseIdx(tt, kN-1))))
			want[tt] = polynomials.EvalCanonicalExt(qcoeffs, field.Lift(y))
		}

		got := foldLayerInternally(layer, alpha, domain)
		if len(got) != half {
			t.Fatalf("n=%d: fold returned %d values, want %d", n, len(got), half)
		}
		for tt := range half {
			if !got[tt].Equal(&want[tt]) {
				t.Fatalf("n=%d: fold[%d] = %s, want %s", n, tt, got[tt].String(), want[tt].String())
			}
		}
	}
}

// TestCheckOpeningProofShapeRejectsWrongFinalPolyLength targets the low-degree
// bound: FinalPoly holds the folded polynomial's coefficients directly, so
// its length alone is the degree bound (see pcs.Verify's final-codeword FFT
// expansion). With the default logFinalPolySize=0 that length must be
// exactly 1, so a 2-entry FinalPoly must be rejected before any query is
// checked.
func TestCheckOpeningProofShapeRejectsWrongFinalPolyLength(t *testing.T) {
	p, err := NewParams(3, 2, 1)
	require.NoError(t, err)

	prf := Proof{
		RoundRoots:     make([]field.Octuplet, p.numRounds()-1),
		RunningQueries: make([]RunningQuery, p.NumQueries),
		FinalPoly:      []field.Ext{{}, field.Lift(field.One())},
	}
	for k := range prf.RunningQueries {
		prf.RunningQueries[k] = make(RunningQuery, p.numRounds()-1)
	}
	foldAlphas := make([]field.Ext, p.numRounds())
	positions := []int{0}

	require.ErrorContains(t, checkOpeningProofShape(p, prf, foldAlphas, positions), "FinalPoly has 2 entries, want 1")
}

// TestProveVerify is the end-to-end check: an honest proof verifies across a few
// (N, D, levels) configurations, and tampering with an opened leaf is rejected.
// It exercises the full ProverState (Fold/Open), the query opening, and
// checkFolds including the alpha²-batched extra levels, going through
// pcs.Verify (the sole FRI entry point) via the ldtFixture compiler: each
// level is a PCS column whose reconstructed DEEP quotient is a genuinely
// low-degree target codeword.
func TestProveVerify(t *testing.T) {

	type cfg struct {
		name       string
		logN, logD uint8
		nq         uint
		extraDs    []int
	}
	cfgs := []cfg{
		{"single-level", 3, 2, 3, nil},
		{"one-extra", 4, 3, 4, []int{2}},
		{"two-extra", 4, 3, 4, []int{4, 2}},
		// A D=1 extra level (a constant polynomial) is introduced at round
		// jl == numRounds(), at the boundary of the fold schedule. Regression
		// test for the off-by-one that previously rejected this as
		// "intro round numRounds, must be < numRounds" in buildProvePlan and
		// pcs.Verify.
		{"extra-D1-at-final-round", 4, 3, 4, []int{2, 1}},
		// The top-level polynomial itself has D=1 (a constant), so numRounds
		// == 0: HasNext() is false from the start, no fold ever runs, and
		// layer 0 IS the final layer. Regression test for two bugs this
		// exposed: (1) NewProverState never populated FinalPoly when no Fold
		// call happens; (2) pcs.Verify indexed Rounds[0] into a zero-length
		// slice (allocated with size numRounds==0), panicking.
		{"top-level-D1-zero-rounds", 1, 0, 2, nil},
	}

	prng := rand.New(utils.NewRandSource(99))

	for _, c := range cfgs {
		t.Run(c.name, func(t *testing.T) {

			fx := newLDTFixture(t, c.logN, c.logD, c.nq)
			fx.addLevel(t, c.logD, field.VecPseudoRandExt(prng, 1<<int(c.logD)))
			for _, d := range c.extraDs {
				fx.addLevel(t, uint8(utils.Log2Ceil(d)), field.VecPseudoRandExt(prng, d))
			}

			foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds())
			for i := range foldAlphas {
				foldAlphas[i] = field.PseudoRandExt(prng)
			}
			positions := make([]int, fx.pcs.Params.NumQueries)
			for i := range positions {
				positions[i] = int(prng.Uint64() % (1 << uint64(c.logN)))
			}

			proof := fx.open(t, foldAlphas, positions)

			if err := fx.verify(foldAlphas, positions, proof); err != nil {
				t.Fatalf("Verify (honest) failed: %v", err)
			}

			// Tampering an opened input leaf must make verification fail.
			branch := proof.InputQueries[0][0]
			branch.Leaves[len(branch.Leaves)-1][0].Ext[0] = field.PseudoRandExt(prng)
			if err := fx.verify(foldAlphas, positions, proof); err == nil {
				t.Fatalf("Verify accepted a proof with a tampered leaf")
			}
		})
	}
}

// TestBoundaryRoundLevelClaimVerified is a regression test for the soundness gap
// where a D=1 aux level at the boundary round (jl == numRounds()) had its
// evaluation claim silently ignored: checkFolds skipped rq.Aux[numRounds()],
// and alphaDeep was zero so reconstructQueryValueAt discarded all but the first
// entry's DEEP quotient. A tampered Ext claimed value must be rejected.
func TestBoundaryRoundLevelClaimVerified(t *testing.T) {
	prng := rand.New(utils.NewRandSource(2025))

	// logN=4, logD=3 → numRounds=3. The D=1 extra level has logCodewordLen=1,
	// so jl = logN - logCodewordLen = 4 - 1 = 3 = numRounds().
	fx := newLDTFixture(t, 4, 3, 2)
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 8))
	fx.addLevel(t, 0, field.VecPseudoRandExt(prng, 1))

	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds())
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	positions := make([]int, fx.pcs.Params.NumQueries)
	for i := range positions {
		positions[i] = int(prng.Uint64() % (1 << 4))
	}

	proof := fx.open(t, foldAlphas, positions)
	require.NoError(t, fx.verify(foldAlphas, positions, proof), "honest proof must verify")

	// Tamper: flip the D=1 boundary level's Ext claimed evaluation from 0 to 1.
	// The prover committed to R(X) = X*target(X) evaluated at zeta=0, so the
	// honest claim is R(0) = 0; a false claim must be rejected.
	fx.claims[1][0].Ext[0][0] = field.Lift(field.One())
	require.Error(t, fx.verify(foldAlphas, positions, proof),
		"tampered Ext claim for D=1 boundary-round level must be rejected")
}

// TestProveVerifyWithFinalPolyDegree checks that stopping FRI before a single
// constant (logFinalPolySize > 0) still verifies honestly and still rejects a
// tampered final coefficient.
func TestProveVerifyWithFinalPolyDegree(t *testing.T) {
	prng := rand.New(utils.NewRandSource(7))

	fx := newLDTFixture(t, 4, 3, 4, LogFinalPolySize(1))
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 8))

	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds())
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	positions := []int{1, 5, 9, 13}

	proof := fx.open(t, foldAlphas, positions)
	require.Len(t, proof.FRIProof.FinalPoly, 2)
	require.NoError(t, fx.verify(foldAlphas, positions, proof))

	one := field.Lift(field.One())
	proof.FRIProof.FinalPoly[1].Add(&proof.FRIProof.FinalPoly[1], &one)
	require.Error(t, fx.verify(foldAlphas, positions, proof))

	// Same nonconstant final poly, but the sole opening is below the static
	// plaintext size, so the proof runs through the restrictTo path (which the
	// case above, opening at the full size, does not).
	fxR := newLDTFixture(t, 4, 3, 4, LogFinalPolySize(1))
	fxR.addLevel(t, 2, field.VecPseudoRandExt(prng, 4))

	foldAlphasR := make([]field.Ext, fxR.pcs.Params.numRounds())
	for i := range foldAlphasR {
		foldAlphasR[i] = field.PseudoRandExt(prng)
	}
	positionsR := []int{1, 3, 5, 7}

	proofR := fxR.open(t, foldAlphasR, positionsR)
	require.Len(t, proofR.FRIProof.FinalPoly, 2)
	require.NoError(t, fxR.verify(foldAlphasR, positionsR, proofR))
}

// TestProverStateOpenLoopsOverLevelTrees checks that a level backed by
// multiple trees (here, two PCS batches sharing a size) is opened and
// authenticated independently per tree.
func TestProverStateOpenLoopsOverLevelTrees(t *testing.T) {

	prng := rand.New(utils.NewRandSource(20260624))

	fx := newLDTFixture(t, 4, 3, 2)
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 8)) // top level, D=8: two trees
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 8))
	fx.addLevel(t, 1, field.VecPseudoRandExt(prng, 2)) // extra level, D=2: two trees
	fx.addLevel(t, 1, field.VecPseudoRandExt(prng, 2))

	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds())
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	positions := []int{3, 11}

	proof := fx.open(t, foldAlphas, positions)
	require.NoError(t, fx.verify(foldAlphas, positions, proof))

	inputQuery := proof.InputQueries[0]
	require.Len(t, inputQuery, 4)

	for i, branch := range inputQuery[:2] {
		root, err := branch.RecoverRoot(positions[0])
		require.NoError(t, err)
		assert.Equal(t, fx.roots[i], root)
	}

	base := positions[0] >> utils.Log2Ceil(8/2) // p.LogPlainTextSize - extraLevelLog2 = 3-1
	for i, branch := range inputQuery[2:] {
		root, err := branch.RecoverRoot(base)
		require.NoError(t, err)
		assert.Equal(t, fx.roots[2+i], root)
	}

	branch := proof.InputQueries[0][1]
	branch.Leaves[len(branch.Leaves)-1][0].Ext[0] = field.PseudoRandExt(prng)
	require.Error(t, fx.verify(foldAlphas, positions, proof))
}

func verifierInputsForLevels(levels []Level) []QueryLayerRoots {
	levelRoots := make([]QueryLayerRoots, len(levels))
	for i := range levels {
		levelRoots[i] = make(QueryLayerRoots, len(levels[i].Trees))
		for j, tree := range levels[i].Trees {
			levelRoots[i][j] = tree.Root()
		}
	}
	return levelRoots
}

// proverForTest runs multi-degree FRI (commit + query phase) and returns a Proof
// together with the query positions. levels[0]'s codeword length must equal
// 2^p.LogCodewordSize and every Level must contain one evaluation vector on
// exactly one rail.
//
// This helper is test-only and INSECURE: it takes the folding challenges
// (alphas) and the query positions (openedPositions) as explicit inputs instead
// of deriving them from the commitments via Fiat-Shamir. A real, non-interactive
// prover must squeeze every challenge and query position out of a transcript
// that has already absorbed the corresponding Merkle roots, so that the prover
// cannot choose them after the fact. Letting the caller supply them directly
// breaks soundness — a malicious prover could pick alphas and positions that
// make a low-degree-test failure go unnoticed — which is exactly why this lives
// in the test file: tests need deterministic, externally-controlled challenges
// to pin down behaviour, but no production code path should ever build a proof
// this way.
func proverForTest(p Params, levels []Level, alphas []field.Ext, openedPositions []int) Proof {

	st, err := NewProverState(p, levels)
	if err != nil {
		utils.Panic("could not build prover state: %v", err)
	}
	if len(alphas) < int(p.numRounds()) {
		utils.Panic("fri: Prove: need %d folding challenges, got %d", p.numRounds(), len(alphas))
	}

	// Drive the state machine: feed one folding challenge per round, then open.
	for j := range p.numRounds() {
		st.Fold(alphas[j])
	}

	return st.Open(openedPositions)
}
