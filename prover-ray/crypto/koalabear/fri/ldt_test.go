package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// ldtFixture commits R(X)=X*target(X) at zeta=0, so the DEEP quotient is
// target. target is genuinely low-degree: addLevel RS-encodes the given
// plaintext-sized coefficients up to the level's full codeword domain, so
// FinalPoly legitimately comes out constant after an honest fold.
type ldtFixture struct {
	pcs *PCS

	roots  []field.Octuplet
	shapes []Shape
	shifts []BatchShifts
	claims []BatchClaimedValues
}

func newLDTFixture(t *testing.T, logN, logD uint8, numQueries uint, opts ...Option) *ldtFixture {
	t.Helper()

	params, err := NewParams(logN, logD, numQueries, opts...)
	require.NoError(t, err)
	encoders := makeEncoders(int(params.LogPlainTextSize)+1, 1<<int(logN-logD))
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	return &ldtFixture{pcs: pcs}
}

func (fx *ldtFixture) addLevel(t *testing.T, sizeLog2 uint8, coeffs []field.Ext) {
	t.Helper()
	fx.addLevelWith(t, sizeLog2, coeffs, true)
}

// addLevelWith is addLevel, committing the zero base column next to R only
// when withBase is set.
func (fx *ldtFixture) addLevelWith(t *testing.T, sizeLog2 uint8, coeffs []field.Ext, withBase bool) {
	t.Helper()

	require.Len(t, coeffs, 1<<sizeLog2)
	target := fx.pcs.Encoders[sizeLog2].EncodeExt(coeffs)

	domain := fx.pcs.Params.domainsLight[fx.pcs.Params.LogPlainTextSize-sizeLog2]
	size, err := reconstructDomainSize(domain)
	require.NoError(t, err)
	require.Len(t, target, size)

	row := make([]field.Ext, size)
	for i := range row {
		x := domainPointExt(domain, i)
		row[i].Mul(&target[i], &x)
	}

	table := make(MultiSizeTable, sizeLog2+1)
	table[sizeLog2] = SizedTable{Ext: [][]field.Ext{row}}
	shifts := make(BatchShifts, sizeLog2+1)
	shifts[sizeLog2] = SizedShifts{Ext: [][]int{{0}}}
	claimed := make(BatchClaimedValues, sizeLog2+1)
	claimed[sizeLog2] = SizedClaimedValues{Ext: [][]field.Ext{{{}}}}
	if withBase {
		table[sizeLog2].Base = [][]field.Element{make([]field.Element, size)}
		shifts[sizeLog2].Base = [][]int{{0}}
		claimed[sizeLog2].Base = [][]field.Ext{{{}}}
	}
	committed := CommitterState{Tree: table.Merkleize(), EncodedTable: table}

	require.NoError(t, fx.pcs.AddOpening(committed, field.Ext{}, shifts, claimed))

	fx.roots = append(fx.roots, committed.Tree.Root())
	fx.shapes = append(fx.shapes, committed.EncodedTable.Shape())
	fx.shifts = append(fx.shifts, shifts)
	fx.claims = append(fx.claims, claimed)
}

func (fx *ldtFixture) open(t *testing.T, foldAlphas []field.Ext, positions []int) OpeningProof {
	t.Helper()

	started, err := fx.pcs.NewProverState()
	require.NoError(t, err)
	for round := 0; started.HasNext(); round++ {
		started.Fold(foldAlphas[round])
	}
	return fx.pcs.Open(started, positions)
}

func (fx *ldtFixture) verify(foldAlphas []field.Ext, positions []int, proof OpeningProof) error {
	return fx.pcs.Verify(VerifyInputs{
		Roots:         fx.roots,
		Shapes:        fx.shapes,
		Shifts:        fx.shifts,
		ClaimedValues: fx.claims,
		Challenges: Challenges{
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	}, proof)
}
