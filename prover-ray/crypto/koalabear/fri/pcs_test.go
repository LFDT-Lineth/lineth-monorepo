package fri

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
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
	if err != nil {
		t.Fatalf("canonicalLayout: %v", err)
	}

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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("layout mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
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
			if err == nil {
				t.Fatalf("canonicalLayout accepted invalid shifts")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestOpeningProofCarriesNoDeepQuotientRoots(t *testing.T) {
	if _, ok := reflect.TypeOf(OpeningProof{}).FieldByName("DeepQuotientRoots"); ok {
		t.Fatalf("OpeningProof must not carry DeepQuotientRoots")
	}
}

func TestProverStateOpenAlignsMultiSizeLevelLeaf(t *testing.T) {
	prng := rand.New(utils.NewRandSource(20260625))
	params, err := NewParams(16, 8, 1)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}

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

	topBranch := openLevelTreesAt([]*Tree{tree}, len(fullEvals), query)[0]
	if topBranch.Leaf != digestSizedRow(encoded[3], query) {
		t.Fatalf("top leaf opens row %d digest incorrectly", query)
	}
	if topBranch.Siblings[len(topBranch.Siblings)-1] != digestSizedRow(encoded[3], query^1) {
		t.Fatalf("top sibling does not open conjugate row %d", query^1)
	}

	levels := []Level{
		newRandomLevel(prng, params, params.D),
		{D: 4, Evals: levelEvals, Trees: []*Tree{tree, otherTree}},
	}
	alphas := []field.Ext{
		field.UintsToExt(41, 1, 0, 0, 0, 0),
		field.UintsToExt(43, 0, 1, 0, 0, 0),
		field.UintsToExt(47, 0, 0, 1, 0, 0),
	}
	proof := proverForTest(params, levels, alphas, []int{query})

	if len(proof.LevelQueries) != 1 {
		t.Fatalf("proof has %d level query sets, want 1", len(proof.LevelQueries))
	}
	opening := proof.LevelQueries[0][0]
	if len(opening) != 2 {
		t.Fatalf("level opening has %d branches, want 2", len(opening))
	}

	checkLevelBranch := func(name string, branch Branch, tree *Tree, encoded MultiSizeTable) {
		t.Helper()

		lifted := levelTreeLeafIndex(tree, len(levelEvals), base)
		root, err := branch.RecoverRoot(lifted)
		if err != nil {
			t.Fatalf("%s: RecoverRoot: %v", name, err)
		}
		if root != tree.Root() {
			t.Fatalf("%s: recovered root != tree root", name)
		}

		leaf, err := branchLeafAtLevel(branch, len(levelEvals))
		if err != nil {
			t.Fatalf("%s: branchLeafAtLevel: %v", name, err)
		}
		if leaf != digestSizedRow(encoded[2], base) {
			t.Fatalf("%s: aux leaf opens the wrong level row digest", name)
		}
		if leaf == digestSizedRow(encoded[2], query&7) {
			t.Fatalf("%s: aux leaf used the unshifted query index", name)
		}
	}
	checkLevelBranch("first tree", opening[0], tree, encoded)
	checkLevelBranch("second tree", opening[1], otherTree, otherEncoded)
}

type pcsOpenVerifyFixture struct {
	pcs   *PCS
	input VerifyInputs
	proof OpeningProof
}

type openInputs struct {
	Witnesses []Batch
	Committed []CommitterState
	Shifts    []BatchShifts
	Zetas     []field.Ext

	Challenges Challenges
}

func openForTest(t *testing.T, pcs *PCS, in openInputs) OpeningProof {
	t.Helper()

	pcs.Reset()
	defer pcs.Reset()
	claimed := make([]BatchClaimedValues, 0, len(in.Witnesses))
	for i := range in.Witnesses {
		batchClaims, err := pcs.Open(in.Witnesses[i], in.Committed[i], in.Zetas[i], in.Shifts[i])
		if err != nil {
			t.Fatalf("pcs.Open: %v", err)
		}
		claimed = append(claimed, batchClaims)
	}
	started, err := pcs.NewProverState(in.Challenges.AlphaDeep)
	if err != nil {
		t.Fatalf("pcs.NewProverState: %v", err)
	}
	if len(in.Challenges.FoldAlphas) < pcs.Params.numRounds {
		t.Fatalf("got %d FRI fold challenges, need %d", len(in.Challenges.FoldAlphas), pcs.Params.numRounds)
	}
	if len(in.Challenges.QueryPositions) < pcs.Params.NumQueries {
		t.Fatalf("got %d query positions, need %d", len(in.Challenges.QueryPositions), pcs.Params.NumQueries)
	}
	queryPositions := in.Challenges.QueryPositions[:pcs.Params.NumQueries]

	for round := range pcs.Params.numRounds {
		started.Fold(in.Challenges.FoldAlphas[round])
	}
	friProof := started.Open(queryPositions)
	rowOpenings := pcs.openedRows(queryPositions)

	return OpeningProof{
		ClaimedValues: claimed,
		RowOpenings:   rowOpenings,
		FRIProof:      friProof,
	}
}

func newPCSOpenVerifyFixture(t *testing.T) pcsOpenVerifyFixture {
	t.Helper()

	params, err := NewParams(8, 4, 1)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	encoders := makeEncoders(params.numRounds+1, 2)
	pcs, err := NewPCS(params, encoders)
	if err != nil {
		t.Fatalf("NewPCS: %v", err)
	}

	prng := rand.New(utils.NewRandSource(20260626))
	witness := make(Batch, 3)
	witness[2] = SizedTable{Ext: [][]field.Ext{
		field.VecPseudoRandExt(prng, 4),
		field.VecPseudoRandExt(prng, 4),
	}}
	witnesses := []Batch{witness}
	committed := []CommitterState{Commit(encoders, witness)}

	batchShifts := make(BatchShifts, 3)
	batchShifts[2] = SizedShifts{Ext: [][]int{{0}, {1}}}
	shifts := []BatchShifts{batchShifts}
	zetas := []field.Ext{field.UintsToExt(19, 2, 3, 5, 7, 11)}
	challenges := Challenges{
		AlphaDeep:      field.UintsToExt(23, 3, 5, 7, 11, 13),
		FoldAlphas:     []field.Ext{field.UintsToExt(29, 1, 0, 0, 0, 0), field.UintsToExt(31, 0, 1, 0, 0, 0)},
		QueryPositions: []int{3},
	}
	proof := openForTest(t, pcs, openInputs{
		Witnesses:  witnesses,
		Committed:  committed,
		Shifts:     shifts,
		Zetas:      zetas,
		Challenges: challenges,
	})

	return pcsOpenVerifyFixture{
		pcs: pcs,
		input: VerifyInputs{
			Roots:      []field.Octuplet{committed[0].Tree.Root()},
			Shapes:     shapesFromBatches(witnesses),
			Shifts:     shifts,
			Zetas:      zetas,
			Challenges: challenges,
		},
		proof: proof,
	}
}

func TestPCSOpenVerifyNormalFlow(t *testing.T) {
	fx := newPCSOpenVerifyFixture(t)
	if err := fx.pcs.Verify(fx.input, fx.proof); err != nil {
		t.Fatalf("pcs.Verify: %v", err)
	}
}

func TestPCSNewProverStateFoldsLikeReferenceVirtualLevels(t *testing.T) {
	params, err := NewParams(16, 8, 2)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}
	encoders := makeEncoders(params.numRounds+1, 2)
	pcs, err := NewPCS(params, encoders)
	if err != nil {
		t.Fatalf("NewPCS: %v", err)
	}

	prng := rand.New(utils.NewRandSource(20260627))
	witness := make(Batch, 4)
	witness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	witness[3] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 8)}}
	otherWitness := make(Batch, 4)
	otherWitness[2] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 4)}}
	otherWitness[3] = SizedTable{Ext: [][]field.Ext{field.VecPseudoRandExt(prng, 8)}}
	witnesses := []Batch{witness, otherWitness}
	committed := []CommitterState{Commit(encoders, witness), Commit(encoders, otherWitness)}

	batchShifts := make(BatchShifts, 4)
	batchShifts[2] = SizedShifts{Ext: [][]int{{1, 3}}}
	batchShifts[3] = SizedShifts{Ext: [][]int{{0, 2}}}
	otherBatchShifts := make(BatchShifts, 4)
	otherBatchShifts[2] = SizedShifts{Ext: [][]int{{0, 2}}}
	otherBatchShifts[3] = SizedShifts{Ext: [][]int{{1, 3}}}
	shifts := []BatchShifts{batchShifts, otherBatchShifts}

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	otherZeta := field.UintsToExt(41, 0, 1, 2, 3, 5)
	alphaDeepChallenge := field.UintsToExt(23, 3, 5, 7, 11, 13)
	firstClaims, err := pcs.Open(witness, committed[0], zeta, batchShifts)
	if err != nil {
		t.Fatalf("pcs.Open: %v", err)
	}
	otherClaims, err := pcs.Open(otherWitness, committed[1], otherZeta, otherBatchShifts)
	if err != nil {
		t.Fatalf("pcs.Open: %v", err)
	}
	claimed := []BatchClaimedValues{firstClaims, otherClaims}
	started, err := pcs.NewProverState(alphaDeepChallenge)
	if err != nil {
		t.Fatalf("pcs.NewProverState: %v", err)
	}
	if len(started.levels) != 2 {
		t.Fatalf("started with %d virtual levels, want 2", len(started.levels))
	}
	levelRoots, _ := levelVerifierInputs(started.levels)
	if len(levelRoots) != 2 || len(levelRoots[0]) != 2 || len(levelRoots[1]) != 2 {
		t.Fatalf("unexpected level root shape: %#v", levelRoots)
	}
	if levelRoots[0][0] != committed[0].Tree.Root() ||
		levelRoots[0][1] != committed[1].Tree.Root() ||
		levelRoots[1][0] != committed[0].Tree.Root() ||
		levelRoots[1][1] != committed[1].Tree.Root() {
		t.Fatalf("virtual levels should be backed by the committed batch tree")
	}

	claimPoint, err := pcs.shiftedPoint(3, 2, zeta)
	if err != nil {
		t.Fatalf("shiftedPoint: %v", err)
	}
	wantClaim := polynomials.EvalLagrange(
		field.VecFromExt(witness[3].Ext[0]),
		field.ElemFromExt(claimPoint),
	).AsExt()
	gotClaim := claimed[0][3].Ext[0][1]
	if !gotClaim.Equal(&wantClaim) {
		t.Fatalf("claimed value mismatch\ngot:  %s\nwant: %s", gotClaim.String(), wantClaim.String())
	}

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
	if !reflect.DeepEqual(gotProof.FRIRoots, referenceProof.FRIRoots) {
		t.Fatalf("FRI roots mismatch\ngot:  %#v\nwant: %#v", gotProof.FRIRoots, referenceProof.FRIRoots)
	}
	if !reflect.DeepEqual(gotProof.FinalPolyExt, referenceProof.FinalPolyExt) {
		t.Fatalf("final polynomial mismatch")
	}

	zetas := []field.Ext{zeta, otherZeta}
	oneShot := openForTest(t, pcs, openInputs{
		Witnesses: witnesses,
		Committed: committed,
		Shifts:    shifts,
		Zetas:     zetas,
		Challenges: Challenges{
			AlphaDeep:      alphaDeepChallenge,
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	})
	if !reflect.DeepEqual(oneShot.ClaimedValues, claimed) {
		t.Fatalf("one-shot claimed values differ from staged claimed values")
	}
	if !reflect.DeepEqual(oneShot.FRIProof.FRIRoots, referenceProof.FRIRoots) {
		t.Fatalf("one-shot FRI roots mismatch")
	}
	if !reflect.DeepEqual(oneShot.FRIProof.FinalPolyExt, referenceProof.FinalPolyExt) {
		t.Fatalf("one-shot final polynomial mismatch")
	}
	if len(oneShot.RowOpenings) != len(positions) {
		t.Fatalf("one-shot row openings have %d queries, want %d", len(oneShot.RowOpenings), len(positions))
	}
	roots := []field.Octuplet{committed[0].Tree.Root(), committed[1].Tree.Root()}
	if err := pcs.Verify(VerifyInputs{
		Roots:  roots,
		Shapes: shapesFromBatches(witnesses),
		Shifts: shifts,
		Zetas:  zetas,
		Challenges: Challenges{
			AlphaDeep:      alphaDeepChallenge,
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	}, oneShot); err != nil {
		t.Fatalf("pcs.Verify: %v", err)
	}

	layout, err := canonicalLayoutFromBatches(witnesses, shifts)
	if err != nil {
		t.Fatalf("canonicalLayoutFromBatches: %v", err)
	}
	orders := batchOrders(layout)
	queryIdx := 0
	top, topSibling, err := pcs.reconstructQueryPair(
		layout[0],
		orders[0],
		oneShot.RowOpenings[queryIdx],
		oneShot.FRIProof.FRIQueries[queryIdx][0],
		levelRoots[0],
		oneShot.ClaimedValues,
		zetas,
		alphaDeepChallenge,
		params.domainsLight[0],
		positions[queryIdx],
	)
	if err != nil {
		t.Fatalf("reconstruct top query values: %v", err)
	}
	if want := started.levels[0].Evals[positions[queryIdx]]; !top.Equal(&want) {
		t.Fatalf("top reconstructed value mismatch\ngot:  %s\nwant: %s", top.String(), want.String())
	}
	if want := started.levels[0].Evals[positions[queryIdx]^1]; !topSibling.Equal(&want) {
		t.Fatalf("top reconstructed sibling mismatch\ngot:  %s\nwant: %s", topSibling.String(), want.String())
	}

	auxRound, err := pcs.roundForSize(layout[1].SizeLog2)
	if err != nil {
		t.Fatalf("roundForSize: %v", err)
	}
	auxBase := positions[queryIdx] >> auxRound
	aux, err := pcs.reconstructQueryValue(
		layout[1],
		orders[1],
		oneShot.RowOpenings[queryIdx],
		oneShot.FRIProof.LevelQueries[0][queryIdx],
		levelRoots[1],
		oneShot.ClaimedValues,
		zetas,
		alphaDeepChallenge,
		params.domainsLight[auxRound],
		auxBase,
	)
	if err != nil {
		t.Fatalf("reconstruct aux query value: %v", err)
	}
	if want := started.levels[1].Evals[auxBase]; !aux.Equal(&want) {
		t.Fatalf("aux reconstructed value mismatch\ngot:  %s\nwant: %s", aux.String(), want.String())
	}
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
