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

		got := foldLayerInternally(layer, alpha, domain, invTwo)
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

// TestCheckFolds is a direct unit test of the pure fold-check: a
// self-consistent set of resolved values (built by hand, no PCS or proof
// involved) must verify, and breaking any single link in the recurrence --
// an intermediate round, the aux mix-in, or the final target -- must be
// rejected.
func TestCheckFolds(t *testing.T) {
	p, err := NewParams(8, 4, 1)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(1))

	fold := func(self, sib, alpha field.Ext, domain domainLight, base int) field.Ext {
		var xInv field.Element
		x := domainPoint(domain, base)
		xInv.Inverse(&x)

		var sum, diff, out field.Ext
		sum.Add(&self, &sib)
		sum.MulByElement(&sum, &p.invTwo)
		diff.Sub(&self, &sib)
		diff.MulByElement(&diff, &p.invTwo)
		diff.MulByElement(&diff, &xInv)
		diff.Mul(&diff, &alpha)
		out.Add(&sum, &diff)
		return out
	}

	const s = 3
	self0, sib0 := field.PseudoRandExt(prng), field.PseudoRandExt(prng)
	alpha0, alpha1 := field.PseudoRandExt(prng), field.PseudoRandExt(prng)
	aux1Self, aux1Sib := field.PseudoRandExt(prng), field.PseudoRandExt(prng)

	// self1 is round 0's fold output (no level introduces at round 0).
	self1 := fold(self0, sib0, alpha0, p.domainsLight[0], s)

	// The level introduced at round 1 (aux1Self, aux1Sib) already has the
	// round-0 running codeword folded into it (see reconstructQueryValueAt),
	// so checkFolds takes it as-is, in place of Rounds[1].
	final := fold(aux1Self, aux1Sib, alpha1, p.domainsLight[1], s>>1)

	newResolved := func() []resolvedQuery {
		return []resolvedQuery{{
			Rounds: []inputPair{{Self: self0, Sibling: sib0}, {Self: self1}},
			Aux:    map[int]inputPair{1: {Self: aux1Self, Sibling: aux1Sib}},
			Final:  final,
		}}
	}
	foldAlphas := []field.Ext{alpha0, alpha1}
	positions := []int{s}

	require.NoError(t, checkFolds(p, newResolved(), foldAlphas, positions))

	one := field.Lift(field.One())

	t.Run("broken intermediate round", func(t *testing.T) {
		resolved := newResolved()
		resolved[0].Rounds[1].Self.Add(&resolved[0].Rounds[1].Self, &one)
		require.ErrorContains(t, checkFolds(p, resolved, foldAlphas, positions), "folded value mismatch")
	})

	t.Run("broken aux", func(t *testing.T) {
		resolved := newResolved()
		auxPair := resolved[0].Aux[1]
		auxPair.Self.Add(&auxPair.Self, &one)
		resolved[0].Aux[1] = auxPair
		require.ErrorContains(t, checkFolds(p, resolved, foldAlphas, positions), "does not match FinalPoly")
	})

	t.Run("broken final", func(t *testing.T) {
		resolved := newResolved()
		resolved[0].Final.Add(&resolved[0].Final, &one)
		require.ErrorContains(t, checkFolds(p, resolved, foldAlphas, positions), "does not match FinalPoly")
	})
}

// TestProveVerify is the end-to-end check: an honest proof verifies across a few
// (N, D, levels) configurations, and tampering with an opened leaf is rejected.
// It exercises the full ProverState (Fold/Open), the query opening, and
// checkFolds including the alpha²-batched extra levels, going through
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

			foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds)
			for i := range foldAlphas {
				foldAlphas[i] = field.PseudoRandExt(prng)
			}
			positions := make([]int, fx.pcs.Params.NumQueries)
			for i := range positions {
				positions[i] = int(prng.Uint64() % uint64(c.n))
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

	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds)
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

	base := positions[0] >> utils.Log2Ceil(8/2) // p.D/extraD = 8/2
	for i, branch := range inputQuery[2:] {
		root, err := branch.RecoverRoot(base)
		require.NoError(t, err)
		assert.Equal(t, fx.roots[2+i], root)
	}

	branch := proof.InputQueries[0][1]
	branch.Leaves[len(branch.Leaves)-1][0].Ext[0] = field.PseudoRandExt(prng)
	require.Error(t, fx.verify(foldAlphas, positions, proof))
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
