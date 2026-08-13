package codegen

import (
	"reflect"
	"testing"
)

// pcsEntryOrder must reproduce the Zig verifier's own runtime reconstruction
// order (src/query/pcs.zig's `reconstruct`): size DESC, batch ASC, base rows
// then ext rows, declaration order within each bucket.
func TestPcsEntryOrder(t *testing.T) {
	// Declaration order 0..4, deliberately out of bucket order.
	pcs := PcsSystem{
		NumBatches:  2,
		MaxSizeLog2: 3,
		Columns: []PcsColumnDesc{
			{BatchIdx: 1, IsExt: false}, // 0
			{BatchIdx: 0, IsExt: false}, // 1
			{BatchIdx: 0, IsExt: true},  // 2
			{BatchIdx: 1, IsExt: false}, // 3
			{BatchIdx: 0, IsExt: false}, // 4
		},
	}
	// Resolved sizes for this proof, aligned with the declaration above.
	colSizeLog2 := []int{2, 3, 3, 3, 2}

	got := pcsEntryOrder(pcs, colSizeLog2)
	want := []int{1, 2, 3, 4, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pcsEntryOrder() = %v, want %v", got, want)
	}
}
