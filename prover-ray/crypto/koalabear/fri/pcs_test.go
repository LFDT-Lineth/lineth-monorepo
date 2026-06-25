package fri

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
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
	domain := fft.NewDomain(8)
	light := domainLight{cardinality: domain.Cardinality, generator: domain.Generator}

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
	alphaDeep := field.UintsToExt(19, 2, 3, 5, 7, 11)

	columns := make([]quotientColumn, len(polys))
	expectedPoly := make([]field.Ext, 0)
	var alphaPower field.Ext
	alphaPower.SetOne()

	for i, poly := range polys {
		columns[i].Evals = encodeTestPoly(poly, domain)
		columns[i].Claims = make([]quotientClaim, len(claimPoints[i]))

		for j, point := range claimPoints[i] {
			value := polynomials.EvalCanonicalExt(poly, point)
			claim := quotientClaim{Point: point, Value: value}
			columns[i].Claims[j] = claim

			quotient := quotientPolyForClaim(poly, claim)
			addScaledPoly(&expectedPoly, quotient, alphaPower)
		}

		alphaPower.Mul(&alphaPower, &alphaDeep)
	}

	level, err := reconstructLevel(quotientLevelInput{
		D:       4,
		Domain:  light,
		Columns: columns,
	}, alphaDeep)
	if err != nil {
		t.Fatalf("reconstructLevel: %v", err)
	}
	if level.D != 4 {
		t.Fatalf("level.D = %d, want 4", level.D)
	}
	if len(level.Evals) != int(domain.Cardinality) {
		t.Fatalf("level has %d evals, want %d", len(level.Evals), domain.Cardinality)
	}

	for pos, got := range level.Evals {
		x := testBitReversedDomainPoint(domain, pos)
		want := polynomials.EvalCanonicalExt(expectedPoly, x)
		if !got.Equal(&want) {
			t.Fatalf("eval[%d] mismatch\ngot:  %s\nwant: %s", pos, got.String(), want.String())
		}
	}
}

func TestReconstructLevelRejectsDomainClaimPoint(t *testing.T) {
	domain := fft.NewDomain(8)
	light := domainLight{cardinality: domain.Cardinality, generator: domain.Generator}
	claimPoint := testBitReversedDomainPoint(domain, 3)

	_, err := reconstructLevel(quotientLevelInput{
		D:      4,
		Domain: light,
		Columns: []quotientColumn{
			{
				Evals: make([]field.Ext, domain.Cardinality),
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
	domain := fft.NewDomain(8)
	level, err := reconstructLevel(quotientLevelInput{
		D:      4,
		Domain: domainLight{cardinality: domain.Cardinality, generator: domain.Generator},
		Columns: []quotientColumn{
			{Evals: make([]field.Ext, domain.Cardinality)},
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

func encodeTestPoly(poly []field.Ext, domain *fft.Domain) []field.Ext {
	evals := make([]field.Ext, domain.Cardinality)
	logSize := utils.Log2Ceil(int(domain.Cardinality))
	for pos := range evals {
		var x field.Element
		x.Exp(domain.Generator, big.NewInt(int64(bitReverseIdx(pos, logSize))))
		evals[pos] = polynomials.EvalCanonicalExt(poly, field.Lift(x))
	}
	return evals
}

func testBitReversedDomainPoint(domain *fft.Domain, pos int) field.Ext {
	var x field.Element
	x.Exp(domain.Generator, big.NewInt(int64(bitReverseIdx(pos, utils.Log2Ceil(int(domain.Cardinality))))))
	return field.Lift(x)
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
