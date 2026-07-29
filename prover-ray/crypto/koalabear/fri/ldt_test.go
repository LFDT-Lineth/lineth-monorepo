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
	table[sizeLog2] = SizedTable{
		Base: [][]field.Element{make([]field.Element, size)},
		Ext:  [][]field.Ext{row},
	}
	committed := CommitterState{Tree: table.Merkleize(), EncodedTable: table}

	shifts := make(BatchShifts, sizeLog2+1)
	shifts[sizeLog2] = SizedShifts{Base: [][]int{{0}}, Ext: [][]int{{0}}}
	claimed := make(BatchClaimedValues, sizeLog2+1)
	claimed[sizeLog2] = SizedClaimedValues{
		Base: [][]field.Ext{{{}}},
		Ext:  [][]field.Ext{{{}}},
	}

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
