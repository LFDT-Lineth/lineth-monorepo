package fri_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	fiatshamir "github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fiatshamirrefactor"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/merkle"
)

func freshTS() *fiatshamir.Transcript {
	hasher := hash.NewPoseidon2SpongeHasher()
	return fiatshamir.NewTranscript(&hasher)
}

func randomPoly(n int) []koalabear.Element {
	elems := make([]koalabear.Element, n)
	for i := range elems {
		_, _ = elems[i].SetRandom()
	}
	return elems
}

func randomExtPoly(n int) []ext.E6 {
	elems := make([]ext.E6, n)
	for i := range elems {
		elems[i].MustSetRandom()
	}
	return elems
}

// buildLevelTree builds the paired-leaf Merkle tree expected by FRI for a
// single-poly level (helper around p.BuildLevelTree).
func buildLevelTree(t *testing.T, p fri.Params, layer []koalabear.Element) *merkle.Tree {
	t.Helper()
	tree, err := p.BuildLevelTree(layer)
	if err != nil {
		t.Fatalf("BuildLevelTree: %v", err)
	}
	return tree
}

func buildLevelTreeExt(t *testing.T, p fri.Params, layer []ext.E6) *merkle.Tree {
	t.Helper()
	tree, err := p.BuildLevelTreeExt(layer)
	if err != nil {
		t.Fatalf("BuildLevelTreeExt: %v", err)
	}
	return tree
}

func testParams(t *testing.T, N, D, queries int) fri.Params {
	t.Helper()
	p, err := fri.NewParams(N, D, queries, commitment.DefaultLeafHasher, commitment.DefaultNodeHasher)
	if err != nil {
		t.Fatalf("NewParams(%d,%d,%d): %v", N, D, queries, err)
	}
	return p
}

// TestProveVerify runs prove+verify for several (N, D, Q) parameter sets.
func TestProveVerify(t *testing.T) {
	cases := []struct{ N, D, Q int }{
		{16, 2, 1},
		{16, 4, 2},
		{64, 4, 4},
		{64, 8, 3},
		{256, 16, 5},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("N%d_D%d_Q%d", tc.N, tc.D, tc.Q), func(t *testing.T) {
			p := testParams(t, tc.N, tc.D, tc.Q)

			poly := randomPoly(tc.D)
			evals, err := p.Encode(poly)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			tree := buildLevelTree(t, p, evals)
			tsP := freshTS()
			prf, _, err := fri.Prove(p, []fri.Level{{
				D:     p.D,
				Evals: fri.LevelEvals{Base: evals},
				Tree:  tree,
			}}, tsP)
			if err != nil {
				t.Fatalf("Prove: %v", err)
			}

			tsV := freshTS()
			if err := fri.Verify(p, []hash.Digest{tree.Root()}, []int{p.D}, prf, tsV); err != nil {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
}

func TestProveVerifyExtRail(t *testing.T) {
	p := testParams(t, 64, 4, 4)

	poly := randomExtPoly(p.D)
	evals, err := p.EncodeExt(poly)
	if err != nil {
		t.Fatalf("EncodeExt: %v", err)
	}

	tree := buildLevelTreeExt(t, p, evals)
	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{{
		D:     p.D,
		Evals: fri.LevelEvals{Ext: evals},
		Tree:  tree,
	}}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	tsV := freshTS()
	if err := fri.Verify(p, []hash.Digest{tree.Root()}, []int{p.D}, prf, tsV); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestProveVerifyExtRailWithExtraLevel(t *testing.T) {
	p := testParams(t, 64, 16, 4)
	pSmall := testParams(t, 16, 4, 4)

	poly0 := randomExtPoly(p.D)
	evals0, err := p.EncodeExt(poly0)
	if err != nil {
		t.Fatalf("EncodeExt level 0: %v", err)
	}
	poly1 := randomExtPoly(pSmall.D)
	evals1, err := pSmall.EncodeExt(poly1)
	if err != nil {
		t.Fatalf("EncodeExt extra level: %v", err)
	}

	tree0 := buildLevelTreeExt(t, p, evals0)
	tree1 := buildLevelTreeExt(t, p, evals1)

	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{
		{
			D:     p.D,
			Evals: fri.LevelEvals{Ext: evals0},
			Tree:  tree0,
		},
		{
			D:     pSmall.D,
			Evals: fri.LevelEvals{Ext: evals1},
			Tree:  tree1,
		},
	}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}
	if len(prf.LevelQueries) != 1 {
		t.Fatalf("LevelQueries length = %d, want 1", len(prf.LevelQueries))
	}

	tsV := freshTS()
	if err := fri.Verify(p, []hash.Digest{tree0.Root(), tree1.Root()}, []int{p.D, pSmall.D}, prf, tsV); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// TestVerifyRejectsWrongRoot ensures Verify fails when root0 doesn't match the proof.
func TestVerifyRejectsWrongRoot(t *testing.T) {
	p := testParams(t, 64, 4, 4)
	evals, _ := p.Encode(randomPoly(p.D))

	tree := buildLevelTree(t, p, evals)
	tsP := freshTS()
	prf, _, _ := fri.Prove(p, []fri.Level{{
		D:     p.D,
		Evals: fri.LevelEvals{Base: evals},
		Tree:  tree,
	}}, tsP)

	var badRoot hash.Digest
	for i := range badRoot {
		_, _ = badRoot[i].SetRandom()
	}

	tsV := freshTS()
	if err := fri.Verify(p, []hash.Digest{badRoot}, []int{p.D}, prf, tsV); err == nil {
		t.Fatal("Verify accepted a proof with a wrong root0")
	}
}

// TestVerifyRejectsFlippedLeaf corrupts one leaf in a QueryLayer and expects rejection.
func TestVerifyRejectsFlippedLeaf(t *testing.T) {
	p := testParams(t, 64, 4, 4)
	evals, _ := p.Encode(randomPoly(p.D))
	tree := buildLevelTree(t, p, evals)

	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{{
		D:     p.D,
		Evals: fri.LevelEvals{Base: evals},
		Tree:  tree,
	}}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// Flip the first leaf of the first query, first layer.
	_, _ = prf.FRIQueries[0].Layers[0].LeafPBase.SetRandom()

	tsV := freshTS()
	if err := fri.Verify(p, []hash.Digest{tree.Root()}, []int{p.D}, prf, tsV); err == nil {
		t.Fatal("Verify accepted a proof with a corrupted leaf")
	}
}

func TestVerifyRejectsFlippedExtLeaf(t *testing.T) {
	p := testParams(t, 64, 4, 4)
	evals, _ := p.EncodeExt(randomExtPoly(p.D))
	tree := buildLevelTreeExt(t, p, evals)

	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{{
		D:     p.D,
		Evals: fri.LevelEvals{Ext: evals},
		Tree:  tree,
	}}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	prf.FRIQueries[0].Layers[0].LeafPExt.MustSetRandom()

	tsV := freshTS()
	if err := fri.Verify(p, []hash.Digest{tree.Root()}, []int{p.D}, prf, tsV); err == nil {
		t.Fatal("Verify accepted a proof with a corrupted ext leaf")
	}
}

func TestVerifyRejectsExtRunningProofWithWrongLeafIndex(t *testing.T) {
	p := testParams(t, 64, 4, 4)
	evals, _ := p.EncodeExt(randomExtPoly(p.D))
	tree := buildLevelTreeExt(t, p, evals)

	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{{
		D:     p.D,
		Evals: fri.LevelEvals{Ext: evals},
		Tree:  tree,
	}}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	prf.FRIQueries[0].Layers[0].Path.LeafIdx += p.N / 2

	tsV := freshTS()
	err = fri.Verify(p, []hash.Digest{tree.Root()}, []int{p.D}, prf, tsV)
	requireLeafIndexError(t, err)
}

func TestVerifyRejectsLevelProofWithWrongLeafIndex(t *testing.T) {
	p := testParams(t, 64, 16, 4)
	pSmall := testParams(t, 16, 4, 4)

	evals0, _ := p.Encode(randomPoly(p.D))
	evals1, _ := pSmall.Encode(randomPoly(pSmall.D))
	tree0 := buildLevelTree(t, p, evals0)
	tree1 := buildLevelTree(t, p, evals1)

	tsP := freshTS()
	prf, _, err := fri.Prove(p, []fri.Level{
		{
			D:     p.D,
			Evals: fri.LevelEvals{Base: evals0},
			Tree:  tree0,
		},
		{
			D:     pSmall.D,
			Evals: fri.LevelEvals{Base: evals1},
			Tree:  tree1,
		},
	}, tsP)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	prf.LevelQueries[0][0].Path.LeafIdx += len(evals1) / 2

	tsV := freshTS()
	err = fri.Verify(p, []hash.Digest{tree0.Root(), tree1.Root()}, []int{p.D, pSmall.D}, prf, tsV)
	requireLeafIndexError(t, err)
}

func requireLeafIndexError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Verify accepted a proof with a wrong Merkle proof leaf index")
	}
	if !strings.Contains(err.Error(), "leaf index") && !strings.Contains(err.Error(), "opened leaf") {
		t.Fatalf("Verify failed for a different reason: %v", err)
	}
}
