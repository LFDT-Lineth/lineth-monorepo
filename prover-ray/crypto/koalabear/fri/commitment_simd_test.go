package fri

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// TestHashSizedLeavesMatchesScalarReference cross-checks hashSizedLeaves'
// batched AVX-512 path against hashLeafScalar, the independent scalar
// reference, for every leaf. 40 rows gives 40 unpaired leaves (32 SIMD + 8
// scalar tail) and 20 paired leaves (16 SIMD + 4 scalar tail), so both modes
// exercise a full SIMD group whose digest the scalar tail loop never computes,
// plus the tail itself.
func TestHashSizedLeavesMatchesScalarReference(t *testing.T) {
	const rows = 40
	var ctr uint64
	t2 := tableOfSize(rows, &ctr)

	for _, paired := range []bool{false, true} {
		nbLeaves := rows
		if paired {
			nbLeaves = rows / 2
		}

		got := make([]field.Octuplet, nbLeaves)
		hashSizedLeaves(t2, paired, got)

		l := newLeafLayout(t2, paired)
		hasher := poseidon2.NewFixedLengthHasher()
		for j := 0; j < nbLeaves; j++ {
			want := l.hashLeafScalar(hasher, t2, j)
			if got[j] != want {
				t.Fatalf("paired=%v leaf %d: batched digest %v != scalar reference %v", paired, j, got[j], want)
			}
		}
	}
}

func hashSizedLeavesTestTable(rows, baseWidth, extWidth int) SizedTable {
	table := SizedTable{
		Base: make([][]field.Element, baseWidth),
		Ext:  make([][]field.Ext, extWidth),
	}
	for column := range table.Base {
		table.Base[column] = make([]field.Element, rows)
		for row := range table.Base[column] {
			table.Base[column][row].SetUint64(uint64(1 + column*rows + row))
		}
	}
	for column := range table.Ext {
		table.Ext[column] = make([]field.Ext, rows)
		for row := range table.Ext[column] {
			k := uint64(1000 + 6*(column*rows+row))
			table.Ext[column][row] = field.UintsToExt(k, k+1, k+2, k+3, k+4, k+5)
		}
	}
	return table
}

func TestHashSizedLeavesFixedLengthBoundaries(t *testing.T) {
	const rows = 40

	for _, baseWidth := range []int{1, 7, 8, 9, 15, 16, 17} {
		t.Run(fmt.Sprintf("unpaired/base=%d", baseWidth), func(t *testing.T) {
			table := hashSizedLeavesTestTable(rows, baseWidth, 0)
			got := make([]field.Octuplet, rows)
			hashSizedLeaves(table, false, got)
			l := newLeafLayout(table, false)
			h := poseidon2.NewFixedLengthHasher()
			for j, digest := range got {
				require.Equal(t, l.hashLeafScalar(h, table, j), digest, "leaf=%d", j)
			}
		})
	}

	for _, baseWidth := range []int{1, 4, 5, 8, 9} {
		t.Run(fmt.Sprintf("paired/base=%d", baseWidth), func(t *testing.T) {
			table := hashSizedLeavesTestTable(rows, baseWidth, 0)
			got := make([]field.Octuplet, rows/2)
			hashSizedLeaves(table, true, got)
			l := newLeafLayout(table, true)
			h := poseidon2.NewFixedLengthHasher()
			for j, digest := range got {
				require.Equal(t, l.hashLeafScalar(h, table, j), digest, "leaf=%d", j)
			}
		})
	}

	for _, testCase := range []struct {
		name                string
		baseWidth, extWidth int
	}{
		{name: "base=0/ext=1", extWidth: 1},
		{name: "base=3/ext=1", baseWidth: 3, extWidth: 1},
	} {
		for _, paired := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/paired=%t", testCase.name, paired), func(t *testing.T) {
				table := hashSizedLeavesTestTable(rows, testCase.baseWidth, testCase.extWidth)
				nbLeaves := rows
				if paired {
					nbLeaves /= 2
				}
				got := make([]field.Octuplet, nbLeaves)
				hashSizedLeaves(table, paired, got)
				l := newLeafLayout(table, paired)
				h := poseidon2.NewFixedLengthHasher()
				for j, digest := range got {
					require.Equal(t, l.hashLeafScalar(h, table, j), digest, "leaf=%d", j)
				}
			})
		}
	}
}

func TestHashSizedLeavesNineElementsUsesFirstBlockAsState(t *testing.T) {
	const rows = 40
	table := hashSizedLeavesTestTable(rows, 9, 0)
	got := make([]field.Octuplet, rows)
	hashSizedLeaves(table, false, got)

	values := make([]field.Element, 9)
	for i := range values {
		values[i] = table.Base[i][0]
	}
	var first, last field.Octuplet
	copy(first[:], values[:8])
	last[7] = values[8]
	expected := poseidon2.Compress(first, last)
	require.Equal(t, expected, got[0])

	l := newLeafLayout(table, false)
	require.Equal(t, expected, l.hashLeafScalar(poseidon2.NewFixedLengthHasher(), table, 0))
}
