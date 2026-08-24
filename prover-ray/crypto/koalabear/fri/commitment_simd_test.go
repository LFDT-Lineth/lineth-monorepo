package fri

import (
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

	requireHashSizedLeavesMatchesScalar(t, t2, false)
	requireHashSizedLeavesMatchesScalar(t, t2, true)
}

func requireHashSizedLeavesMatchesScalar(t *testing.T, table SizedTable, paired bool) {
	t.Helper()
	nbLeaves := 0
	if len(table.Base) != 0 {
		nbLeaves = len(table.Base[0])
	} else if len(table.Ext) != 0 {
		nbLeaves = len(table.Ext[0])
	}
	if paired {
		nbLeaves /= 2
	}
	got := make([]field.Octuplet, nbLeaves)
	hashSizedLeaves(table, paired, got)

	l := newLeafLayout(table, paired)
	hasher := poseidon2.NewFixedLengthHasher()
	for j, digest := range got {
		require.Equal(t, l.hashLeafScalar(hasher, table, j), digest, "paired=%t leaf=%d", paired, j)
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

	for _, testCase := range []struct {
		name                string
		baseWidth, extWidth int
		paired              bool
	}{
		{name: "unpaired/base=1", baseWidth: 1},
		{name: "unpaired/base=7", baseWidth: 7},
		{name: "unpaired/base=8", baseWidth: 8},
		{name: "unpaired/base=9", baseWidth: 9},
		{name: "unpaired/base=15", baseWidth: 15},
		{name: "unpaired/base=16", baseWidth: 16},
		{name: "unpaired/base=17", baseWidth: 17},
		{name: "paired/base=1", baseWidth: 1, paired: true},
		{name: "paired/base=4", baseWidth: 4, paired: true},
		{name: "paired/base=5", baseWidth: 5, paired: true},
		{name: "paired/base=8", baseWidth: 8, paired: true},
		{name: "paired/base=9", baseWidth: 9, paired: true},
		{name: "unpaired/base=0/ext=1", extWidth: 1},
		{name: "paired/base=0/ext=1", extWidth: 1, paired: true},
		{name: "unpaired/base=3/ext=1", baseWidth: 3, extWidth: 1},
		{name: "paired/base=3/ext=1", baseWidth: 3, extWidth: 1, paired: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			table := hashSizedLeavesTestTable(rows, testCase.baseWidth, testCase.extWidth)
			requireHashSizedLeavesMatchesScalar(t, table, testCase.paired)
		})
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
