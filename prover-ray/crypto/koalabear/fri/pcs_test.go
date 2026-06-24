package fri

import (
	"reflect"
	"strings"
	"testing"
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
