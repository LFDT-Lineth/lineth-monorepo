package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalLayout_Order(t *testing.T) {
	shapes := []Shape{
		{
			{},
			{},
			{BaseWidth: 1},
			{BaseWidth: 1, ExtWidth: 1},
		},
		{
			{},
			{},
			{ExtWidth: 1},
			{BaseWidth: 1},
		},
	}
	shifts := []BatchShifts{
		{
			{},
			{},
			{Base: [][]int{{0}}},
			{Base: [][]int{{2, 0}}, Ext: [][]int{{1}}},
		},
		{
			{},
			{},
			{Ext: [][]int{{2, 3}}},
			{Base: [][]int{{3}}},
		},
	}

	got, err := canonicalLayout(shapes, shifts)
	require.NoError(t, err)

	want := layout{
		{
			SizeLog2: 3,
			Entries: []deepEntry{
				{BatchIdx: 0, SizeLog2: 3, RowIdx: 0, AlphaPower: 0, Shifts: []int{2, 0}},
				{BatchIdx: 0, SizeLog2: 3, RowIdx: 0, IsExt: true, AlphaPower: 1, Shifts: []int{1}},
				{BatchIdx: 1, SizeLog2: 3, RowIdx: 0, AlphaPower: 2, Shifts: []int{3}},
			},
		},
		{
			SizeLog2: 2,
			Entries: []deepEntry{
				{BatchIdx: 0, SizeLog2: 2, RowIdx: 0, AlphaPower: 0, Shifts: []int{0}},
				{BatchIdx: 1, SizeLog2: 2, RowIdx: 0, IsExt: true, AlphaPower: 1, Shifts: []int{2, 3}},
			},
		},
	}
	assert.Equal(t, want, got)
}

func TestCanonicalLayout_RejectsShiftInvariants(t *testing.T) {
	shape := []Shape{{{}, {}, {BaseWidth: 1}}}

	tests := []struct {
		name    string
		shifts  []BatchShifts
		wantErr string
	}{
		{
			name:    "empty",
			shifts:  []BatchShifts{{{}, {}, {Base: [][]int{{}}}}},
			wantErr: "empty shift list",
		},
		{
			name:    "duplicate",
			shifts:  []BatchShifts{{{}, {}, {Base: [][]int{{2, 2}}}}},
			wantErr: "duplicate shift 2",
		},
		{
			name:    "aliasing",
			shifts:  []BatchShifts{{{}, {}, {Base: [][]int{{0, 4}}}}},
			wantErr: "shift 4 outside [0,4)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := canonicalLayout(shape, tc.shifts)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestAddOpeningZeta covers the shared-zeta invariant: zeta=0 is a valid
// out-of-domain point and is accepted (its zero value is not a sentinel), but
// a later opening carrying a different zeta is rejected.
func TestAddOpeningZeta(t *testing.T) {
	params, err := NewParams(8, 4, 1)
	require.NoError(t, err)
	pcs, err := NewPCS(params, makeEncoders(params.numRounds+1, 2))
	require.NoError(t, err)

	witness := make(Batch, 3)
	witness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(rand.New(utils.NewRandSource(1)), 4)}}
	committed := pcs.Commit(witness)
	shifts := make(BatchShifts, 3)
	shifts[2] = SizedShifts{Ext: [][]int{{0}}}
	claimed := make(BatchClaimedValues, 3)
	claimed[2] = SizedClaimedValues{Ext: [][]field.Ext{{{}}}}

	require.NoError(t, pcs.AddOpening(committed, field.Ext{}, shifts, claimed))
	require.ErrorContains(t,
		pcs.AddOpening(committed, field.UintsToExt(1, 0, 0, 0, 0, 0), shifts, claimed),
		"zeta mismatch")
}

func TestOpenInputBranchAlignsMultiSizeRows(t *testing.T) {
	prng := rand.New(utils.NewRandSource(20260625))
	params, err := NewParams(16, 8, 1)
	require.NoError(t, err)

	levelEncoder := NewEncoder(8, 4)
	fullEncoder := NewEncoder(16, 8)

	levelEvals := levelEncoder.EncodeExt(field.VecPseudoRandExt(prng, 4))
	fullEvals := fullEncoder.EncodeExt(field.VecPseudoRandExt(prng, 8))
	tree, encoded := multiSizeTreeForCodewords(levelEvals, fullEvals)

	otherLevelEvals := levelEncoder.EncodeExt(field.VecPseudoRandExt(prng, 4))
	otherFullEvals := fullEncoder.EncodeExt(field.VecPseudoRandExt(prng, 8))
	otherTree, otherEncoded := multiSizeTreeForCodewords(otherLevelEvals, otherFullEvals)

	const query = 11
	base := query >> 1

	topBranch := tree.OpenBranch(query)
	assert.Equal(t, digestSizedRow(encoded[3], query), topBranch.Leaf)
	assert.Equal(t, digestSizedRow(encoded[3], query^1), topBranch.Siblings[len(topBranch.Siblings)-1])

	checkInputBranch := func(name string, branch InputBranch, tree *Tree, encoded MultiSizeTable) {
		t.Helper()

		root, err := branch.RecoverRoot(query)
		require.NoError(t, err, name)
		assert.Equal(t, tree.Root(), root, name)

		leaf, err := branch.rowAtLevel(len(levelEvals))
		require.NoError(t, err, name)
		assert.Equal(t, digestSizedRow(encoded[2], base), hashRowOpening(leaf), name)
	}
	checkInputBranch("first tree", openInputBranch(params, CommitterState{Tree: tree, EncodedTable: encoded}, query), tree, encoded)
	checkInputBranch("second tree", openInputBranch(params, CommitterState{Tree: otherTree, EncodedTable: otherEncoded}, query), otherTree, otherEncoded)
}

type pcsOpenVerifyFixture struct {
	pcs       *PCS
	input     VerifyInputs
	proof     OpeningProof
	committed []CommitterState
}

type openInputs struct {
	Witnesses []Batch
	Committed []CommitterState
	Shifts    []BatchShifts
	Zeta      field.Ext

	Challenges Challenges
}

// openForTest runs the full prover-side opening flow and returns the proof
// together with the per-batch claimed values it computed. The PCS no longer
// derives the claims itself, so the caller computes them here (mirroring the
// outer protocol) and feeds them both into AddOpening and into VerifyInputs.
func openForTest(t *testing.T, pcs *PCS, in openInputs) (OpeningProof, []BatchClaimedValues) {
	t.Helper()

	pcs.Reset()
	defer pcs.Reset()
	batchClaims := make([]BatchClaimedValues, len(in.Witnesses))
	for i := range in.Witnesses {
		batchClaims[i] = claimedValuesForTest(t, pcs, in.Witnesses[i], in.Shifts[i], in.Zeta)
		err := pcs.AddOpening(in.Committed[i], in.Zeta, in.Shifts[i], batchClaims[i])
		require.NoError(t, err)
	}
	started, err := pcs.NewProverState(in.Challenges.AlphaDeep)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(in.Challenges.QueryPositions), pcs.Params.NumQueries)
	queryPositions := in.Challenges.QueryPositions[:pcs.Params.NumQueries]

	// Fold until the (possibly restricted) prover state is exhausted, so this
	// works whether or not pcs.Params was larger than the witness.
	for round := 0; started.HasNext(); round++ {
		started.Fold(in.Challenges.FoldAlphas[round])
	}
	return pcs.Open(started, queryPositions), batchClaims
}

// claimedValuesForTest evaluates every opened (size, row, shift) of a witness
// batch at zeta * omega_N^shift, producing the BatchClaimedValues the caller
// now supplies to AddOpening and echoes into VerifyInputs. In production the
// outer protocol performs this evaluation; the PCS itself no longer does.
func claimedValuesForTest(t *testing.T, pcs *PCS, witness Batch, shifts BatchShifts, zeta field.Ext) BatchClaimedValues {
	t.Helper()

	evalRow := func(poly field.Vec, sizeLog2 int, rowShifts []int) []field.Ext {
		values := make([]field.Ext, len(rowShifts))
		for i, shift := range rowShifts {
			point, err := pcs.shiftedPoint(sizeLog2, shift, zeta)
			require.NoError(t, err)
			values[i] = polynomials.EvalLagrange(poly, field.ElemFromExt(point)).AsExt()
		}
		return values
	}

	claimed := make(BatchClaimedValues, len(shifts))
	for sizeLog2, sizedShifts := range shifts {
		sizedWitness := witness[sizeLog2]
		sized := SizedClaimedValues{
			Base: make([][]field.Ext, len(sizedShifts.Base)),
			Ext:  make([][]field.Ext, len(sizedShifts.Ext)),
		}
		for rowIdx, rowShifts := range sizedShifts.Base {
			row := sizedWitness.Base[rowIdx]
			require.Len(t, row, 1<<sizeLog2)
			sized.Base[rowIdx] = evalRow(field.VecFromBase(row), sizeLog2, rowShifts)
		}
		for rowIdx, rowShifts := range sizedShifts.Ext {
			row := sizedWitness.Ext[rowIdx]
			require.Len(t, row, 1<<sizeLog2)
			sized.Ext[rowIdx] = evalRow(field.VecFromExt(row), sizeLog2, rowShifts)
		}
		claimed[sizeLog2] = sized
	}
	return claimed
}

func newPCSOpenVerifyFixture(t *testing.T) pcsOpenVerifyFixture {
	t.Helper()

	params, err := NewParams(8, 4, 1)
	require.NoError(t, err)
	encoders := makeEncoders(params.numRounds+1, 2)
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(20260626))
	witness := make(Batch, 3)
	witness[1] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 2)}}
	witness[2] = SizedTable{Ext: [][]field.Ext{
		field.VecPseudoRandExt(prng, 4),
		field.VecPseudoRandExt(prng, 4),
	}}
	witnesses := []Batch{witness}
	committed := []CommitterState{pcs.Commit(witness)}

	batchShifts := make(BatchShifts, 3)
	batchShifts[1] = SizedShifts{Ext: [][]int{{0}}}
	batchShifts[2] = SizedShifts{Ext: [][]int{{0}, {1}}}
	shifts := []BatchShifts{batchShifts}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		AlphaDeep:      field.UintsToExt(23, 3, 5, 7, 11, 13),
		FoldAlphas:     []field.Ext{field.UintsToExt(29, 1, 0, 0, 0, 0), field.UintsToExt(31, 0, 1, 0, 0, 0)},
		QueryPositions: []int{3},
	}
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zeta:       zeta,
		Challenges: challenges,
	})

	return pcsOpenVerifyFixture{
		pcs:       pcs,
		committed: committed,
		input: VerifyInputs{
			Roots:         []field.Octuplet{committed[0].Tree.Root()},
			Shapes:        shapesFromBatches(witnesses),
			Shifts:        shifts,
			ClaimedValues: claimed,
			Zeta:          zeta,
			Challenges:    challenges,
		},
		proof: proof,
	}
}

func TestPCSOpenVerifyNormalFlow(t *testing.T) {
	fx := newPCSOpenVerifyFixture(t)
	require.NoError(t, fx.pcs.Verify(fx.input, fx.proof))
}

// TestPCSStaticParamsLargerThanWitness exercises a static PCS whose Params are
// sized well above the witness: D=16 (numRounds=4) with only size-4 columns.
// The FRI schedule must restrict to the witness (2 folds), not fold 4 times.
func TestPCSStaticParamsLargerThanWitness(t *testing.T) {
	// D=16 static capacity, witness columns are size 4 (sizeLog2=2).
	params, err := NewParams(32, 16, 1)
	require.NoError(t, err)
	encoders := makeEncoders(params.numRounds+1, 2) // sizes 2^0..2^4
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(20260701))
	witness := make(Batch, 3)
	witness[2] = SizedTable{Ext: [][]field.Ext{
		field.VecPseudoRandExt(prng, 4),
		field.VecPseudoRandExt(prng, 4),
	}}
	witnesses := []Batch{witness}
	committed := []CommitterState{pcs.Commit(witness)}

	batchShifts := make(BatchShifts, 3)
	batchShifts[2] = SizedShifts{Ext: [][]int{{0}, {1}}}
	shifts := []BatchShifts{batchShifts}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	challenges := Challenges{
		AlphaDeep: field.UintsToExt(23, 3, 5, 7, 11, 13),
		FoldAlphas: []field.Ext{
			field.UintsToExt(29, 1, 0, 0, 0, 0),
			field.UintsToExt(31, 0, 1, 0, 0, 0),
		},
		QueryPositions: []int{3},
	}

	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zeta:       zeta,
		Challenges: challenges,
	})

	// The witness top is size 4 → exactly 2 folds → final poly of the inverse-rate
	// size (2), not the 4 folds Params.D=16 would dictate.
	require.Len(t, proof.FRIProof.FinalPolyExt, 2)

	require.NoError(t, pcs.Verify(VerifyInputs{
		Roots:         []field.Octuplet{committed[0].Tree.Root()},
		Shapes:        shapesFromBatches(witnesses),
		Shifts:        shifts,
		ClaimedValues: claimed,
		Zeta:          zeta,
		Challenges:    challenges,
	}, proof))
}

func TestPCSNewProverStateFoldsLikeReferenceVirtualLevels(t *testing.T) {
	params, err := NewParams(16, 8, 2)
	require.NoError(t, err)
	encoders := makeEncoders(params.numRounds+1, 2)
	pcs, err := NewPCS(params, encoders)
	require.NoError(t, err)

	prng := rand.New(utils.NewRandSource(20260627))
	witness := make(Batch, 4)
	witness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	witness[3] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 8)}}
	otherWitness := make(Batch, 4)
	otherWitness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	otherWitness[3] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 8)}}
	witnesses := []Batch{witness, otherWitness}
	committed := []CommitterState{pcs.Commit(witness), pcs.Commit(otherWitness)}

	batchShifts := make(BatchShifts, 4)
	batchShifts[2] = SizedShifts{Ext: [][]int{{1, 3}}}
	batchShifts[3] = SizedShifts{Ext: [][]int{{0, 2}}}
	otherBatchShifts := make(BatchShifts, 4)
	otherBatchShifts[2] = SizedShifts{Ext: [][]int{{0, 2}}}
	otherBatchShifts[3] = SizedShifts{Ext: [][]int{{1, 3}}}
	shifts := []BatchShifts{batchShifts, otherBatchShifts}

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	alphaDeepChallenge := field.UintsToExt(23, 3, 5, 7, 11, 13)
	firstClaims := claimedValuesForTest(t, pcs, witness, batchShifts, zeta)
	otherClaims := claimedValuesForTest(t, pcs, otherWitness, otherBatchShifts, zeta)
	err = pcs.AddOpening(committed[0], zeta, batchShifts, firstClaims)
	require.NoError(t, err)
	err = pcs.AddOpening(committed[1], zeta, otherBatchShifts, otherClaims)
	require.NoError(t, err)
	started, err := pcs.NewProverState(alphaDeepChallenge)
	require.NoError(t, err)
	require.Len(t, started.levels, 2)
	levelRoots, _ := verifierInputsForLevels(started.levels)
	require.Len(t, levelRoots, 2)
	require.Len(t, levelRoots[0], 2)
	require.Len(t, levelRoots[1], 2)
	assert.Equal(t, committed[0].Tree.Root(), levelRoots[0][0])
	assert.Equal(t, committed[1].Tree.Root(), levelRoots[0][1])
	assert.Equal(t, committed[0].Tree.Root(), levelRoots[1][0])
	assert.Equal(t, committed[1].Tree.Root(), levelRoots[1][1])

	claimPoint, err := pcs.shiftedPoint(3, 2, zeta)
	require.NoError(t, err)
	wantClaim := polynomials.EvalLagrange(
		field.VecFromExt(witness[3].Ext[0]),
		field.ElemFromExt(claimPoint),
	).AsExt()
	gotClaim := firstClaims[3].Ext[0][1]
	assert.Equal(t, wantClaim, gotClaim)

	referenceLevels := make([]Level, len(started.levels))
	for i, level := range started.levels {
		referenceLevels[i] = Level{
			D:     level.D,
			Evals: append([]field.Ext(nil), level.Evals...),
			Trees: []*Tree{buildTreeExt(level.Evals)},
		}
	}

	foldAlphas := []field.Ext{
		field.UintsToExt(29, 1, 0, 0, 0, 0),
		field.UintsToExt(31, 0, 1, 0, 0, 0),
		field.UintsToExt(37, 0, 0, 1, 0, 0),
	}
	positions := []int{3, 11}
	referenceProof := proverForTest(params, referenceLevels, foldAlphas, positions)

	for round := range params.numRounds {
		started.Fold(foldAlphas[round])
	}
	gotProof := started.Open(positions)
	assert.Equal(t, referenceProof.FRIRoots, gotProof.FRIRoots)
	assert.Equal(t, referenceProof.FinalPolyExt, gotProof.FinalPolyExt)

	oneShot, oneShotClaims := openForTest(t, pcs, openInputs{
		Witnesses: witnesses,
		Committed: committed,
		Shifts:    shifts,
		Zeta:      zeta,
		Challenges: Challenges{
			AlphaDeep:      alphaDeepChallenge,
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	})
	roots := []field.Octuplet{committed[0].Tree.Root(), committed[1].Tree.Root()}
	require.NoError(t, pcs.Verify(VerifyInputs{
		Roots:         roots,
		Shapes:        shapesFromBatches(witnesses),
		Shifts:        shifts,
		ClaimedValues: oneShotClaims,
		Zeta:          zeta,
		Challenges: Challenges{
			AlphaDeep:      alphaDeepChallenge,
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	}, oneShot))
}

func multiSizeTreeForCodewords(levelEvals, fullEvals []field.Ext) (*Tree, MultiSizeTable) {
	table := make(MultiSizeTable, 4)
	table[2] = SizedTable{Ext: [][]field.Ext{levelEvals}}
	table[3] = SizedTable{Ext: [][]field.Ext{fullEvals}}
	return table.Merkleize(), table
}

func digestSizedRow(table SizedTable, row int) field.Octuplet {
	return hashRowOpening(openEncodedRow(table, row))
}
