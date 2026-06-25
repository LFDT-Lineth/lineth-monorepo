package fri

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
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
			{Ext: [][]int{{4, 5}}},
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
				{BatchIdx: 1, SizeLog2: 2, RowIdx: 0, IsExt: true, AlphaPower: 1, Shifts: []int{4, 5}},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("layout mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestCanonicalLayout_RejectsShiftInvariants(t *testing.T) {
	shape := []Shape{{{BaseWidth: 1}}}

	tests := []struct {
		name    string
		shifts  []BatchShifts
		wantErr string
	}{
		{
			name:    "empty",
			shifts:  []BatchShifts{{{Base: [][]int{{}}}}},
			wantErr: "empty shift list",
		},
		{
			name:    "duplicate",
			shifts:  []BatchShifts{{{Base: [][]int{{2, 2}}}}},
			wantErr: "duplicate shift 2",
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

func TestReconstructLevelMatchesDirectQuotientPolynomial(t *testing.T) {
	encoder := NewEncoder(8, 4)
	light := domainLight{cardinality: encoder.Domain.Cardinality, generator: encoder.Domain.Generator}

	polys := [][]field.Ext{
		{
			field.UintsToExt(1, 0, 2, 0, 0, 0),
			field.UintsToExt(3, 1, 0, 0, 0, 0),
			field.UintsToExt(5, 0, 0, 1, 0, 0),
			field.UintsToExt(7, 0, 0, 0, 1, 0),
		},
		{
			field.UintsToExt(2, 0, 0, 0, 0, 1),
			field.UintsToExt(4, 0, 1, 0, 0, 0),
			field.UintsToExt(6, 0, 0, 1, 0, 0),
		},
	}
	claimPoints := [][]field.Ext{
		{
			field.UintsToExt(11, 1, 0, 0, 0, 0),
			field.UintsToExt(13, 0, 1, 0, 0, 0),
		},
		{
			field.UintsToExt(17, 0, 0, 1, 0, 0),
			field.UintsToExt(11, 1, 0, 0, 0, 0),
		},
	}
	alphaDeepChallenge := field.UintsToExt(19, 2, 3, 5, 7, 11)

	columns := make([]quotientColumn, len(polys))
	expectedPoly := make([]field.Ext, 0)
	var alphaDeepPower field.Ext
	alphaDeepPower.SetOne()

	for i, poly := range polys {
		columns[i].AlphaPower = i
		columns[i].Evals = encodeCanonicalTestPoly(poly, &encoder)
		columns[i].Claims = make([]quotientClaim, len(claimPoints[i]))

		for j, point := range claimPoints[i] {
			value := polynomials.EvalCanonicalExt(poly, point)
			claim := quotientClaim{Point: point, Value: value}
			columns[i].Claims[j] = claim

			quotient := quotientPolyForClaim(poly, claim)
			addScaledPoly(&expectedPoly, quotient, alphaDeepPower)
		}

		alphaDeepPower.Mul(&alphaDeepPower, &alphaDeepChallenge)
	}
	columns[0], columns[1] = columns[1], columns[0]

	level, err := reconstructLevel(quotientLevelInput{
		D:       4,
		Domain:  light,
		Columns: columns,
	}, alphaDeepChallenge)
	if err != nil {
		t.Fatalf("reconstructLevel: %v", err)
	}
	if level.D != 4 {
		t.Fatalf("level.D = %d, want 4", level.D)
	}
	if len(level.Evals) != int(encoder.Domain.Cardinality) {
		t.Fatalf("level has %d evals, want %d", len(level.Evals), encoder.Domain.Cardinality)
	}

	expectedCodeword := encodeCanonicalTestPoly(expectedPoly, &encoder)
	for pos, got := range level.Evals {
		want := expectedCodeword[pos]
		if !got.Equal(&want) {
			t.Fatalf("eval[%d] mismatch\ngot:  %s\nwant: %s", pos, got.String(), want.String())
		}
	}
}

func TestReconstructLevelRejectsDomainClaimPoint(t *testing.T) {
	encoder := NewEncoder(8, 4)
	light := domainLight{cardinality: encoder.Domain.Cardinality, generator: encoder.Domain.Generator}
	claimPoint := domainPointExt(light, 3)

	_, err := reconstructLevel(quotientLevelInput{
		D:      4,
		Domain: light,
		Columns: []quotientColumn{
			{
				Evals: make([]field.Ext, encoder.Domain.Cardinality),
				Claims: []quotientClaim{
					{Point: claimPoint, Value: field.Uint64ToExt(42)},
				},
			},
		},
	}, field.Uint64ToExt(7))
	if err == nil {
		t.Fatalf("reconstructLevel accepted a claim point on the domain")
	}
	if !strings.Contains(err.Error(), "lands on domain") {
		t.Fatalf("error %q does not mention domain collision", err.Error())
	}
}

func TestReconstructLevelHandlesNoClaims(t *testing.T) {
	encoder := NewEncoder(8, 4)
	level, err := reconstructLevel(quotientLevelInput{
		D:      4,
		Domain: domainLight{cardinality: encoder.Domain.Cardinality, generator: encoder.Domain.Generator},
		Columns: []quotientColumn{
			{Evals: make([]field.Ext, encoder.Domain.Cardinality)},
		},
	}, field.Uint64ToExt(7))
	if err != nil {
		t.Fatalf("reconstructLevel: %v", err)
	}
	for pos := range level.Evals {
		if !level.Evals[pos].IsZero() {
			t.Fatalf("eval[%d] = %s, want zero", pos, level.Evals[pos].String())
		}
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
	fullDomain := domainLight{cardinality: fullEncoder.Domain.Cardinality, generator: fullEncoder.Domain.Generator}
	levelCoeffs := []field.Ext{
		field.UintsToExt(3, 1, 0, 0, 0, 0),
		field.UintsToExt(5, 0, 1, 0, 0, 0),
		field.UintsToExt(7, 0, 0, 1, 0, 0),
	}
	fullCoeffs := []field.Ext{
		field.UintsToExt(11, 0, 0, 0, 1, 0),
		field.UintsToExt(13, 0, 0, 0, 0, 1),
		field.UintsToExt(17, 1, 1, 0, 0, 0),
		field.UintsToExt(19, 0, 1, 1, 0, 0),
	}
	otherLevelCoeffs := []field.Ext{
		field.UintsToExt(23, 1, 0, 1, 0, 0),
		field.UintsToExt(29, 0, 1, 0, 1, 0),
	}
	otherFullCoeffs := []field.Ext{
		field.UintsToExt(31, 0, 0, 1, 0, 1),
		field.UintsToExt(37, 1, 0, 0, 1, 0),
	}

	levelEvals := encodeCanonicalTestPoly(levelCoeffs, &levelEncoder)
	fullEvals := encodeCanonicalTestPoly(fullCoeffs, &fullEncoder)
	tree, encoded := multiSizeTreeForCodewords(levelEvals, fullEvals)

	otherLevelEvals := encodeCanonicalTestPoly(otherLevelCoeffs, &levelEncoder)
	otherFullEvals := encodeCanonicalTestPoly(otherFullCoeffs, &fullEncoder)
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
	x := domainPointExt(fullDomain, query)
	var minusX field.Ext
	minusX.Neg(&x)
	wantSelf := polynomials.EvalCanonicalExt(fullCoeffs, x)
	wantSibling := polynomials.EvalCanonicalExt(fullCoeffs, minusX)
	if !fullEvals[query].Equal(&wantSelf) {
		t.Fatalf("full eval at query does not match f_i(x)")
	}
	if !fullEvals[query^1].Equal(&wantSibling) {
		t.Fatalf("full eval at query^1 does not match f_i(-x)")
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

	topCoeffs := []field.Ext{
		field.UintsToExt(2, 1, 0, 0, 0, 0),
		field.UintsToExt(3, 0, 1, 0, 0, 0),
		field.UintsToExt(5, 0, 0, 1, 0, 0),
		field.UintsToExt(7, 0, 0, 0, 1, 0),
	}
	auxCoeffs := []field.Ext{
		field.UintsToExt(11, 0, 0, 0, 0, 1),
		field.UintsToExt(13, 1, 1, 0, 0, 0),
		field.UintsToExt(17, 0, 1, 1, 0, 0),
	}

	witness := make(Batch, 4)
	witness[2] = SizedTable{Ext: [][]field.Ext{canonicalToLagrangeTestPoly(auxCoeffs, 4)}}
	witness[3] = SizedTable{Ext: [][]field.Ext{canonicalToLagrangeTestPoly(topCoeffs, 8)}}
	witnesses := []Batch{witness}
	committed := []CommitterState{Commit(encoders, witness)}

	batchShifts := make(BatchShifts, 4)
	batchShifts[2] = SizedShifts{Ext: [][]int{{1, 3}}}
	batchShifts[3] = SizedShifts{Ext: [][]int{{0, 2}}}
	shifts := []BatchShifts{batchShifts}

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	alphaDeepChallenge := field.UintsToExt(23, 3, 5, 7, 11, 13)
	started, err := pcs.NewProverState(ProverStateInputs{
		Witnesses: witnesses,
		Committed: committed,
		Shifts:    shifts,
		Zeta:      zeta,
		AlphaDeep: alphaDeepChallenge,
	})
	if err != nil {
		t.Fatalf("pcs.NewProverState: %v", err)
	}
	if len(started.Levels) != 2 {
		t.Fatalf("started with %d virtual levels, want 2", len(started.Levels))
	}
	if len(started.LevelRoots) != 2 || len(started.LevelRoots[0]) != 1 || len(started.LevelRoots[1]) != 1 {
		t.Fatalf("unexpected level root shape: %#v", started.LevelRoots)
	}
	if started.LevelRoots[0][0] != committed[0].Tree.Root() || started.LevelRoots[1][0] != committed[0].Tree.Root() {
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
	gotClaim := started.ClaimedValues[0][3].Ext[0][1]
	if !gotClaim.Equal(&wantClaim) {
		t.Fatalf("claimed value mismatch\ngot:  %s\nwant: %s", gotClaim.String(), wantClaim.String())
	}

	referenceLevels := make([]Level, len(started.Levels))
	for i, level := range started.Levels {
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

	for round := 0; started.State.HasNext(); round++ {
		started.State.Fold(foldAlphas[round])
	}
	gotProof := started.State.Open(positions)
	if !reflect.DeepEqual(gotProof.FRIRoots, referenceProof.FRIRoots) {
		t.Fatalf("FRI roots mismatch\ngot:  %#v\nwant: %#v", gotProof.FRIRoots, referenceProof.FRIRoots)
	}
	if !reflect.DeepEqual(gotProof.FinalPolyExt, referenceProof.FinalPolyExt) {
		t.Fatalf("final polynomial mismatch")
	}

	oneShot, err := pcs.Open(OpenInputs{
		Witnesses: witnesses,
		Committed: committed,
		Shifts:    shifts,
		Challenges: Challenges{
			Zeta:           zeta,
			AlphaDeep:      alphaDeepChallenge,
			FoldAlphas:     foldAlphas,
			QueryPositions: positions,
		},
	})
	if err != nil {
		t.Fatalf("pcs.Open: %v", err)
	}
	if !reflect.DeepEqual(oneShot.ClaimedValues, started.ClaimedValues) {
		t.Fatalf("one-shot claimed values differ from staged claimed values")
	}
	if !reflect.DeepEqual(oneShot.FRIProof.FRIRoots, referenceProof.FRIRoots) {
		t.Fatalf("one-shot FRI roots mismatch")
	}
	if !reflect.DeepEqual(oneShot.FRIProof.FinalPolyExt, referenceProof.FinalPolyExt) {
		t.Fatalf("one-shot final polynomial mismatch")
	}
}

func quotientPolyForClaim(poly []field.Ext, claim quotientClaim) []field.Ext {
	adjusted := make([]field.Ext, len(poly))
	copy(adjusted, poly)
	adjusted[0].Sub(&adjusted[0], &claim.Value)

	quotient := make([]field.Ext, len(adjusted)-1)
	quotient[len(quotient)-1] = adjusted[len(adjusted)-1]
	for i := len(quotient) - 2; i >= 0; i-- {
		var term field.Ext
		term.Mul(&claim.Point, &quotient[i+1])
		quotient[i].Add(&adjusted[i+1], &term)
	}
	return quotient
}

func encodeCanonicalTestPoly(poly []field.Ext, encoder *RSEncoder) []field.Ext {
	return encoder.EncodeExt(canonicalToLagrangeTestPoly(poly, encoder.PlainTextSize))
}

func canonicalToLagrangeTestPoly(poly []field.Ext, size int) []field.Ext {
	lagrange := make([]field.Ext, size)
	copy(lagrange, poly)
	domain := fft.NewDomain(uint64(size))
	domain.FFTExt6(lagrange, fft.DIF)
	gutils.BitReverse(lagrange)
	return lagrange
}

func addScaledPoly(accum *[]field.Ext, poly []field.Ext, scale field.Ext) {
	for len(*accum) < len(poly) {
		*accum = append(*accum, field.Ext{})
	}
	for i := range poly {
		var term field.Ext
		term.Mul(&poly[i], &scale)
		(*accum)[i].Add(&(*accum)[i], &term)
	}
}

func multiSizeTreeForCodewords(levelEvals, fullEvals []field.Ext) (*Tree, MultiSizeTable) {
	table := make(MultiSizeTable, 4)
	table[2] = SizedTable{Ext: [][]field.Ext{levelEvals}}
	table[3] = SizedTable{Ext: [][]field.Ext{fullEvals}}
	return table.Merkleize(), table
}

func digestSizedRow(table SizedTable, row int) field.Octuplet {
	hasher := poseidon2.NewMDHasher()
	for _, base := range table.Base {
		hasher.WriteElements(base[row])
	}
	for _, ext := range table.Ext {
		value := ext[row]
		hasher.WriteElements(
			value.B0.A0, value.B0.A1,
			value.B1.A0, value.B1.A1,
			value.B2.A0, value.B2.A1,
		)
	}
	return hasher.SumDigest()
}
