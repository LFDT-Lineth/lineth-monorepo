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
		for m := 0; m < n; m++ {
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
		for tt := 0; tt < half; tt++ {
			var y field.Element
			y.Exp(g2, big.NewInt(int64(bitReverseIdx(tt, kN-1))))
			want[tt] = polynomials.EvalCanonicalExt(qcoeffs, field.Lift(y))
		}

		got := foldLayerInternally(layer, nil, alpha, domain, invTwo)
		if len(got) != half {
			t.Fatalf("n=%d: fold returned %d values, want %d", n, len(got), half)
		}
		for tt := 0; tt < half; tt++ {
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
		for tt := 0; tt < half; tt++ {
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
// checkQueryExt including the alpha²-batched extra levels.
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

			p, err := NewParams(c.n, c.d, c.nq)
			if err != nil {
				t.Fatalf("NewParams: %v", err)
			}

			levels := []Level{newRandomLevel(prng, p, c.d)}
			for _, d := range c.extraDs {
				levels = append(levels, newRandomLevel(prng, p, d))
			}

			alphas := make([]field.Ext, p.numRounds)
			for i := range alphas {
				alphas[i] = field.PseudoRandExt(prng)
			}
			positions := make([]int, p.NumQueries)
			for i := range positions {
				positions[i] = int(prng.Uint64() % uint64(p.N))
			}

			prf := proverForTest(p, levels, alphas, positions)

			// Prove sorts levels by decreasing D; mirror that order for the verifier.
			levelRoots := make([]field.Octuplet, len(levels))
			levelDs := make([]int, len(levels))
			for i := range levels {
				levelRoots[i] = levels[i].Tree.Root()
				levelDs[i] = levels[i].D
			}

			if err := Verify(p, levelRoots, levelDs, prf, alphas, positions); err != nil {
				t.Fatalf("Verify (honest) failed: %v", err)
			}

			// Tampering an opened leaf must make verification fail.
			prf.FRIQueries[0][0].Leaf = field.PseudoRandOctuplet(prng)
			if err := Verify(p, levelRoots, levelDs, prf, alphas, positions); err == nil {
				t.Fatalf("Verify accepted a proof with a tampered leaf")
			}
		})
	}
}

// newRandomLevel builds a Level with a random evaluation vector of the size
// dictated by its degree d (N·d/D = N>>jl) and the matching binary Merkle tree.
func newRandomLevel(prng *rand.Rand, p Params, d int) Level {
	size := p.N * d / p.D
	evals := make([]field.Ext, size)
	for i := range evals {
		evals[i] = field.PseudoRandExt(prng)
	}
	return Level{D: d, Evals: evals, Tree: buildTreeExt(evals)}
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
	for j := 0; st.HasNext(); j++ {
		st.Fold(alphas[j])
	}

	return st.Open(openedPositions)
}
