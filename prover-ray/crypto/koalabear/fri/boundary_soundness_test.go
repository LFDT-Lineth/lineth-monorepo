package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// buildExploit assembles a two-level opening at a NONZERO zeta:
//
//	level A (round 0, size 2^3): the constant polynomial `a`, claimed y = a.
//	          Its DEEP quotient is identically zero, so the whole running
//	          codeword folds to zero and FinalPoly = [0]. This level exists
//	          only because buildProvePlan requires a level at round 0; it
//	          contributes nothing and isolates the boundary check.
//	level B (round numRounds() == 3, size 2^0 == 1): the boundary level. Its
//	          codeword has 2 entries (blowup 2). An HONEST size-1 column is a
//	          constant, so its codeword must be (c, c). We commit (c0, c1)
//	          with c0 != c1 -- i.e. the degree-1 polynomial v(X) = a1 + b1*X,
//	          which is NOT in the size-1 Reed-Solomon code.
//
// boundaryClaim is the value claimed for level B at zeta.
func buildExploit(t *testing.T, c0, c1 field.Ext, boundaryClaim field.Ext, zeta field.Ext) *ldtFixture {
	t.Helper()

	fx := newLDTFixture(t, 4, 3, 4) // logN=4, logD=3, 4 queries -> numRounds=3
	require.Equal(t, uint8(3), fx.pcs.Params.numRounds())

	// ---- level A: constant polynomial at size 8, quotient identically zero.
	constVal := field.PseudoRandExt(rand.New(utils.NewRandSource(99)))
	lagrange := make([]field.Ext, 8)
	for i := range lagrange {
		lagrange[i] = constVal
	}
	codeword := fx.pcs.Encoders[3].EncodeExt(lagrange)
	require.Len(t, codeword, 16)

	tableA := make(MultiSizeTable, 4)
	tableA[3] = SizedTable{Ext: [][]field.Ext{codeword}}
	committedA := CommitterState{Tree: tableA.Merkleize(), EncodedTable: tableA}
	shiftsA := make(BatchShifts, 4)
	shiftsA[3] = SizedShifts{Ext: [][]int{{0}}}
	claimsA := make(BatchClaimedValues, 4)
	claimsA[3] = SizedClaimedValues{Ext: [][]field.Ext{{constVal}}}
	require.NoError(t, fx.pcs.AddOpening(committedA, zeta, shiftsA, claimsA))
	fx.roots = append(fx.roots, committedA.Tree.Root())
	fx.shapes = append(fx.shapes, committedA.EncodedTable.Shape())
	fx.shifts = append(fx.shifts, shiftsA)
	fx.claims = append(fx.claims, claimsA)

	// ---- level B: the boundary level, codeword (c0, c1).
	tableB := make(MultiSizeTable, 1)
	tableB[0] = SizedTable{Ext: [][]field.Ext{{c0, c1}}}
	committedB := CommitterState{Tree: tableB.Merkleize(), EncodedTable: tableB}
	shiftsB := make(BatchShifts, 1)
	shiftsB[0] = SizedShifts{Ext: [][]int{{0}}}
	claimsB := make(BatchClaimedValues, 1)
	claimsB[0] = SizedClaimedValues{Ext: [][]field.Ext{{boundaryClaim}}}
	require.NoError(t, fx.pcs.AddOpening(committedB, zeta, shiftsB, claimsB))
	fx.roots = append(fx.roots, committedB.Tree.Root())
	fx.shapes = append(fx.shapes, committedB.EncodedTable.Shape())
	fx.shifts = append(fx.shifts, shiftsB)
	fx.claims = append(fx.claims, claimsB)

	return fx
}

func runExploit(t *testing.T, fx *ldtFixture, zeta field.Ext) error {
	t.Helper()
	prng := rand.New(utils.NewRandSource(4242))
	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds())
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	positions := []int{0, 3, 9, 14} // hits both halves, so both levelPos 0 and 1
	proof := fx.open(t, foldAlphas, positions)
	// NOTE: ldtFixture.verify leaves VerifyInputs.Zeta at its zero value, so it
	// only ever verifies at zeta=0. Pass the real zeta here.
	return fx.pcs.Verify(VerifyInputs{
		Roots:         fx.roots,
		Shapes:        fx.shapes,
		Shifts:        fx.shifts,
		ClaimedValues: fx.claims,
		Zeta:          zeta,
		Challenges: Challenges{
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	}, proof)
}

// TestBoundaryLevelAcceptsNonConstantColumn demonstrates that the boundary-round
// check (checkFolds' `Self == Sibling`) enforces degree < 2 on the committed
// column, not degree < 1 as the size-1 Reed-Solomon code requires.
func TestBoundaryLevelAcceptsNonConstantColumn(t *testing.T) {
	prng := rand.New(utils.NewRandSource(7))
	zeta := field.PseudoRandExt(prng) // nonzero, out of domain (generic ext elt)

	// v(X) = a1 + b1*X with b1 != 0: NOT a valid size-1 codeword.
	a1 := field.PseudoRandExt(prng)
	b1 := field.PseudoRandExt(prng)

	// codeword positions over the size-2 domain {1, -1}: c0 = v(1), c1 = v(-1).
	var c0, c1 field.Ext
	c0.Add(&a1, &b1)
	c1.Sub(&a1, &b1)

	// the value the boundary check will accept: y = v(zeta) = a1 + b1*zeta.
	var y, bz field.Ext
	bz.Mul(&b1, &zeta)
	y.Add(&a1, &bz)

	t.Run("non-constant column with y=v(zeta) is ACCEPTED", func(t *testing.T) {
		err := runExploit(t, buildExploit(t, c0, c1, y, zeta), zeta)
		require.NoError(t, err, "SOUNDNESS: a degree-1 column passed as a size-1 (constant) column")
	})

	t.Run("control: same column, claim y=a1 (the constant term) is rejected", func(t *testing.T) {
		err := runExploit(t, buildExploit(t, c0, c1, a1, zeta), zeta)
		require.Error(t, err)
		t.Logf("rejected as expected: %v", err)
	})

	t.Run("control: honest constant column (c0==c1) with y=c0 is accepted", func(t *testing.T) {
		err := runExploit(t, buildExploit(t, a1, a1, a1, zeta), zeta)
		require.NoError(t, err)
	})
}
