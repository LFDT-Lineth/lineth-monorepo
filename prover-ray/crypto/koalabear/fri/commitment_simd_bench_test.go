package fri

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

func BenchmarkHashSizedLeaves(b *testing.B) {
	const rows = 1 << 16

	for _, baseWidth := range []int{1, 8, 9, 16, 17} {
		b.Run(fmt.Sprintf("unpaired/base=%d", baseWidth), func(b *testing.B) {
			table := benchmarkHashSizedLeavesTable(rows, baseWidth)
			out := make([]field.Octuplet, rows)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				hashSizedLeaves(table, false, out)
			}
		})
	}

	for _, baseWidth := range []int{1, 4, 5, 8, 9} {
		b.Run(fmt.Sprintf("paired/base=%d", baseWidth), func(b *testing.B) {
			table := benchmarkHashSizedLeavesTable(rows, baseWidth)
			out := make([]field.Octuplet, rows/2)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				hashSizedLeaves(table, true, out)
			}
		})
	}
}

func benchmarkHashSizedLeavesTable(rows, baseWidth int) SizedTable {
	base := make([][]field.Element, baseWidth)
	for column := range base {
		base[column] = make([]field.Element, rows)
		for row := range base[column] {
			base[column][row].SetUint64(uint64(1 + column*rows + row))
		}
	}
	return SizedTable{Base: base}
}
