package vortex_koalabear

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/require"
)

func TestNoSisTransversalHashMatchesReference(t *testing.T) {
	testCases := []struct {
		name    string
		numRows int
		numCols int
	}{
		{name: "rows_387_cols_1024", numRows: 387, numCols: 1024},
		{name: "rows_160_cols_1024", numRows: 160, numCols: 1024},
		{name: "rows_8_cols_1024", numRows: 8, numCols: 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rows := makeNoSisRows(tc.numRows, tc.numCols)
			params := NewParams(2, tc.numCols/2, tc.numRows, 6, 16)
			got := params.noSisTransversalHash(rows)
			want := referenceNoSisTransversalHash(rows)
			require.Equal(t, want, got)
		})
	}
}

func BenchmarkNoSisTransversalHash(b *testing.B) {
	testCases := []struct {
		name    string
		numRows int
		numCols int
	}{
		{name: "rows_387_cols_16384", numRows: 387, numCols: 1 << 14},
		{name: "rows_160_cols_16384", numRows: 160, numCols: 1 << 14},
		{name: "rows_8_cols_16384", numRows: 8, numCols: 1 << 14},
	}

	for _, tc := range testCases {
		rows := makeNoSisRows(tc.numRows, tc.numCols)
		params := NewParams(2, tc.numCols/2, tc.numRows, 6, 16)

		b.Run(fmt.Sprintf("optimized_%s", tc.name), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = params.noSisTransversalHash(rows)
			}
		})

		b.Run(fmt.Sprintf("reference_%s", tc.name), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = referenceNoSisTransversalHash(rows)
			}
		})
	}
}

func BenchmarkVortexHashPathsByRows(b *testing.B) {
	const numCols = 1 << 13

	// Covers the small, around-threshold, and large row counts seen in limitless.
	rowCounts := []int{
		4, 8, 12, 28,
		64, 80, 96, 104, 112, 123, 128, 136, 140, 159, 160,
		175, 193, 204, 208, 212, 226, 228, 257, 264, 270, 271,
		304, 312, 320, 352, 384, 387, 388, 396, 404, 410, 411,
		436, 440, 448, 454, 480, 492, 512, 576, 640, 768, 832,
		896, 960, 1024, 1152, 1280, 1504, 2208,
	}

	for _, numRows := range rowCounts {
		rows := makeNoSisRows(numRows, numCols)
		params := NewParams(2, numCols, numRows, 6, 16)
		encodedRows := params.EncodeRows(rows)
		paddedNoSisRows := ((numRows + 7) / 8) * 8
		sisChunks := (numRows + 31) / 32

		b.Run(fmt.Sprintf("rows_%d", numRows), func(b *testing.B) {
			b.Run("no_sis_hash_only", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(paddedNoSisRows), "padded_rows")
				b.ReportMetric(float64(sisChunks), "sis_chunks")
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = params.noSisTransversalHash(encodedRows)
				}
			})

			b.Run("sis_hash_only", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(paddedNoSisRows), "padded_rows")
				b.ReportMetric(float64(sisChunks), "sis_chunks")
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = params.sisTransversalHash(encodedRows)
				}
			})
		})
	}
}

func referenceNoSisTransversalHash(v []smartvectors.SmartVector) []field.Octuplet {
	nbRows := len(v)
	nbCols := v[0].Len()
	res := make([]field.Octuplet, nbCols)
	curCol := make([]field.Element, nbRows)
	h := poseidon2_koalabear.NewMDHasher()
	for col := 0; col < nbCols; col++ {
		for row := 0; row < nbRows; row++ {
			curCol[row] = v[row].Get(col)
		}
		h.WriteElements(curCol...)
		res[col] = h.SumElement()
		h.Reset()
	}
	return res
}

func makeNoSisRows(numRows, numCols int) []smartvectors.SmartVector {
	rows := make([]smartvectors.SmartVector, numRows)
	for row := 0; row < numRows; row++ {
		switch {
		case row%17 == 0:
			rows[row] = smartvectors.NewConstant(field.NewElement(uint64(row+1)), numCols)
		case row%11 == 0:
			windowLen := numCols / 4
			window := make([]field.Element, windowLen)
			for i := range window {
				window[i] = field.NewElement(uint64((row+1)*(i+3) + 7))
			}
			rows[row] = smartvectors.RightPadded(window, field.NewElement(uint64(row+5)), numCols)
		default:
			vec := make([]field.Element, numCols)
			for i := range vec {
				vec[i] = field.NewElement(uint64((row+1)*(i+1) + 13))
			}
			rows[row] = smartvectors.NewRegular(vec)
		}
	}
	return rows
}
