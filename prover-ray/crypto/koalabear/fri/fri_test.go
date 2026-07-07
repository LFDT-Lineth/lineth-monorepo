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
// Q(Y) = P_e(Y) + alpha·P_o(Y), and that a non-nil auxiliary vector adds
// alpha²·aux at the matching output position. The expected codeword is built by
// an independent canonical evaluation, so this also pins down the bit-reversed
// twiddle alignment.
func TestFoldLayerInternally(t *testing.T) {

	prng := rand.New(utils.NewRandSource(1))

	var two, invTwo field.Element
	two.SetUint64(2)
	invTwo.Inverse(&two)

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

		got := foldLayerInternally(layer, nil, alpha, domain, invTwo)
		if len(got) != half {
			t.Fatalf("n=%d: fold returned %d values, want %d", n, len(got), half)
		}
		for tt := range half {
			if !got[tt].Equal(&want[tt]) {
				t.Fatalf("n=%d: fold[%d] = %s, want %s", n, tt, got[tt].String(), want[tt].String())
			}
		}

		// aux: the fold mixes alpha²·aux[t] into output position t
		aux := make([]field.Ext, half)
		for i := range aux {
			aux[i] = field.PseudoRandExt(prng)
		}
		var alpha2 field.Ext
		alpha2.Square(&alpha)

		gotAux := foldLayerInternally(layer, aux, alpha, domain, invTwo)
		for tt := range half {
			var wantAux, term field.Ext
			term.Mul(&aux[tt], &alpha2)
			wantAux.Add(&want[tt], &term)
			if !gotAux[tt].Equal(&wantAux) {
				t.Fatalf("n=%d: fold+aux[%d] mismatch", n, tt)
			}
		}
	}
}

// TestProveVerify is the end-to-end check: an honest proof verifies across a few
// (N, D, levels) configurations, and tampering with an opened leaf is rejected.
// It exercises the full ProverState (Fold/Open), the query opening, and
// checkQueryExt including the alpha²-batched extra levels, going through
// pcs.Verify (the sole FRI entry point) via the ldtFixture compiler: each
// level is a PCS column whose reconstructed DEEP quotient is an arbitrary
// target codeword, independent of degree.
func TestProveVerify(t *testing.T) {

	type cfg struct {
		name     string
		n, d, nq int
		extraDs  []int
	}
	cfgs := []cfg{
		{"single-level", 8, 4, 3, nil},
		{"one-extra", 16, 8, 4, []int{2}},
		{"two-extra", 16, 8, 4, []int{4, 2}},
	}

	prng := rand.New(utils.NewRandSource(99))

	for _, c := range cfgs {
		t.Run(c.name, func(t *testing.T) {

			fx := newLDTFixture(t, c.n, c.d, c.nq)
			fx.addLevel(t, utils.Log2Ceil(c.d), field.VecPseudoRandExt(prng, c.n))
			for _, d := range c.extraDs {
				fx.addLevel(t, utils.Log2Ceil(d), field.VecPseudoRandExt(prng, c.n*d/c.d))
			}

			alphaDeep := field.PseudoRandExt(prng)
			foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds)
			for i := range foldAlphas {
				foldAlphas[i] = field.PseudoRandExt(prng)
			}
			positions := make([]int, fx.pcs.Params.NumQueries)
			for i := range positions {
				positions[i] = int(prng.Uint64() % uint64(c.n))
			}

			proof := fx.open(t, alphaDeep, foldAlphas, positions)

			if err := fx.verify(alphaDeep, foldAlphas, positions, proof); err != nil {
				t.Fatalf("Verify (honest) failed: %v", err)
			}

			// Tampering an opened leaf must make verification fail.
			proof.FRIProof.FRIQueries[0][0][0].Leaf = field.PseudoRandOctuplet(prng)
			if err := fx.verify(alphaDeep, foldAlphas, positions, proof); err == nil {
				t.Fatalf("Verify accepted a proof with a tampered leaf")
			}
		})
	}
}

// TestProverStateOpenLoopsOverLevelTrees checks that a level backed by
// multiple trees (here, two PCS batches sharing a size) is opened and
// authenticated independently per tree.
func TestProverStateOpenLoopsOverLevelTrees(t *testing.T) {

	prng := rand.New(utils.NewRandSource(20260624))

	fx := newLDTFixture(t, 16, 8, 2)
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 16)) // top level, D=8: two trees
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 16))
	fx.addLevel(t, 1, field.VecPseudoRandExt(prng, 4)) // extra level, D=2: two trees
	fx.addLevel(t, 1, field.VecPseudoRandExt(prng, 4))

	alphaDeep := field.PseudoRandExt(prng)
	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds)
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	positions := []int{3, 11}

	proof := fx.open(t, alphaDeep, foldAlphas, positions)
	require.NoError(t, fx.verify(alphaDeep, foldAlphas, positions, proof))

	require.Len(t, proof.FRIProof.FRIQueries[0][0], 2)
	require.Len(t, proof.FRIProof.LevelQueries, 1)
	require.Len(t, proof.FRIProof.LevelQueries[0][0], 2)

	for i, branch := range proof.FRIProof.FRIQueries[0][0] {
		root, err := branch.RecoverRoot(positions[0])
		require.NoError(t, err)
		assert.Equal(t, fx.roots[i], root)
	}

	base := positions[0] >> utils.Log2Ceil(8/2) // p.D/extraD = 8/2
	for i, branch := range proof.FRIProof.LevelQueries[0][0] {
		root, err := branch.RecoverRoot(base)
		require.NoError(t, err)
		assert.Equal(t, fx.roots[2+i], root)
	}

	proof.FRIProof.FRIQueries[0][0][1].Leaf = field.PseudoRandOctuplet(prng)
	require.Error(t, fx.verify(alphaDeep, foldAlphas, positions, proof))
}

// newRandomLevel builds a Level with a random evaluation vector of the size
// dictated by its degree d (N·d/D = N>>jl) and the matching binary Merkle tree.
func newRandomLevel(prng *rand.Rand, p Params, d int) Level {
	size := p.N * d / p.D
	evals := make([]field.Ext, size)
	for i := range evals {
		evals[i] = field.PseudoRandExt(prng)
	}
	return Level{D: d, Evals: evals, Trees: []*Tree{buildTreeExt(evals)}}
}

func verifierInputsForLevels(levels []Level) ([]QueryLayerRoots, []int) {
	levelRoots := make([]QueryLayerRoots, len(levels))
	levelDs := make([]int, len(levels))
	for i := range levels {
		levelRoots[i] = make(QueryLayerRoots, len(levels[i].Trees))
		for j, tree := range levels[i].Trees {
			levelRoots[i][j] = tree.Root()
		}
		levelDs[i] = levels[i].D
	}
	return levelRoots, levelDs
}

// proverForTest runs multi-degree FRI (commit + query phase) and returns a Proof
// together with the query positions. levels[0].D must equal p.D and every Level
// must contain one evaluation vector on exactly one rail. levels is sorted
// in-place in decreasing order of D.
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
	if len(alphas) < p.numRounds {
		utils.Panic("fri: Prove: need %d folding challenges, got %d", p.numRounds, len(alphas))
	}

	// Drive the state machine: feed one folding challenge per round, then open.
	for j := range p.numRounds {
		st.Fold(alphas[j])
	}

	return st.Open(openedPositions)
}
