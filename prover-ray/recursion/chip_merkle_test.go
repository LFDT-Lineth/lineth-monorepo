package recursion

import (
	"math/bits"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

const merkleDepth = 4 // 16 leaves

// merkleTree builds a mixed-height fri.Tree: a bottom table of 16 rows, and
// tables of 8 and 2 rows, whose digests are injected at the levels of 4 nodes
// and at the root.
func merkleTree(t *testing.T) *fri.Tree {
	t.Helper()
	rnd := rand.New(rand.NewPCG(4, 5))
	sized := func(rows, base, ext int) fri.SizedTable {
		var st fri.SizedTable
		for range base {
			col := make([]field.Element, rows)
			for i := range col {
				col[i].SetUint64(rnd.Uint64())
			}
			st.Base = append(st.Base, col)
		}
		for range ext {
			col := make([]field.Ext, rows)
			for i := range col {
				col[i] = randE6(rnd)
			}
			st.Ext = append(st.Ext, col)
		}
		return st
	}
	table := fri.MultiSizeTable{
		{}, sized(2, 1, 1), {}, sized(8, 2, 0), sized(1<<merkleDepth, 3, 1),
	}
	return table.Merkleize()
}

// branchPath returns a branch's siblings, leaf level first, and its auxiliary
// digests as injections (the digest of a node of depth k is injected at the
// level of 2^k nodes), as cells.
func branchPath(br fri.Branch) (sibs []Cell, injWidths []int, injs []Cell) {
	d := len(br.Siblings)
	for k := range d {
		i := d - 1 - k // fri.Branch lists them from the top
		sibs = append(sibs, Cell(br.Siblings[i]))
		if aux := br.AuxSiblings[i]; aux != nil {
			injWidths = append(injWidths, 1<<(d-k-1))
			injs = append(injs, Cell(*aux))
		}
	}
	return sibs, injWidths, injs
}

func bitrev(i, width int) int {
	return int(bits.Reverse(uint(i)) >> (bits.UintSize - width))
}

// merkleProgram opens every leaf of the tree with MerklePathX, its leaf,
// siblings, injections and index as inputs, the root a constant, and asserts
// [x⁻¹, b0] equal to Go's. It returns the program and its inputs.
func merkleProgram(t *testing.T, tree *fri.Tree, gInv field.Element) (*Machine, []Cell) {
	t.Helper()
	m := &Machine{}
	root := m.Const(Cell(tree.Root()))
	var in []Cell
	input := func(kind int, v Cell) int {
		in = append(in, v)
		return m.input(kind)
	}
	for i := range 1 << merkleDepth {
		br := tree.OpenBranch(i)
		require.NoError(t, br.AuthenticateToCap(i, []field.Octuplet{tree.Root()}), "leaf %d", i)
		sibCells, widths, injCells := branchPath(br)
		leaf := input(KindDigest, Cell(br.Leaf))
		sibs := make([]int, len(sibCells))
		for k, s := range sibCells {
			sibs[k] = input(KindDigest, s)
		}
		injs := make([]Injection, len(injCells))
		for k, c := range injCells {
			injs[k] = Injection{Width: widths[k], Leaf: input(KindDigest, c)}
		}
		idx := input(KindScalar, ScalarCell(field.NewElement(uint64(i))))
		xb := m.MerklePathX(leaf, sibs, idx, root, gInv, injs...)
		var want Cell
		want[0].ExpInt64(gInv, int64(bitrev(i, merkleDepth)))
		want[1].SetUint64(uint64(i & 1))
		m.AssertEqE6(xb, m.Const(want))
	}
	return m, in
}

var merkleGInv = field.NewElement(123456789)

// TestMerklePath: every leaf of a mixed-height fri.Tree (its branches checked
// by fri itself) opens to the root in the machine, with [x⁻¹, b0], and
// proves.
func TestMerklePath(t *testing.T) {
	tree := merkleTree(t)
	m, in := merkleProgram(t, tree, merkleGInv)
	for i, p := range m.paths {
		// every path crosses a node of the 4-node level and the root, which
		// both have an auxiliary digest
		require.Len(t, p.steps, merkleDepth+2, "path %d: 4 levels and 2 injections", i)
	}
	p, err := m.Compile()
	require.NoError(t, err)
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r))
}

// TestMerklePathRejects: openings executed with a wrong sibling, injection,
// index or leaf compute another root, which the last row's read of the root
// rejects; an index beyond 2^depth (the same low bits, so the same root) is
// rejected by the index's bound; and a wrong x⁻¹ accumulator by its
// recurrence.
func TestMerklePathRejects(t *testing.T) {
	tree := merkleTree(t)
	m, in := merkleProgram(t, tree, merkleGInv)
	p, err := m.Compile()
	require.NoError(t, err)
	// inputs per path: leaf, 4 siblings, 2 injections, index; path 5's
	const perPath, path = 1 + merkleDepth + 2 + 1, 5
	leaf := path * perPath
	sib, inj, idx := leaf+1, leaf+1+merkleDepth, leaf+perPath-1
	for name, edit := range map[string]func(in []Cell){
		"sibling":   func(in []Cell) { in[sib+2][3].SetUint64(9) },
		"injection": func(in []Cell) { in[inj+1][0].SetUint64(9) },
		"leaf":      func(in []Cell) { in[leaf][0].SetUint64(9) },
		"index":     func(in []Cell) { in[idx] = ScalarCell(field.NewElement(6)) },
		// 5 + 16: the same low bits, so the same root and [x⁻¹, b0]
		"index beyond the depth": func(in []Cell) { in[idx] = ScalarCell(field.NewElement(path + 16)) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := append([]Cell(nil), in...)
			edit(bad)
			r, err := p.Execute(bad)
			require.NoError(t, err)
			require.Error(t, proveAndVerify(p, r), "a wrong opening accepted")
		})
	}
	t.Run("accumulator", func(t *testing.T) {
		r, err := p.Execute(in)
		require.NoError(t, err)
		r.Witness(p, merkleMod, merkleX)[path*(merkleDepth+2)+2].SetUint64(5)
		require.Error(t, proveAndVerify(p, r), "a wrong x⁻¹ accumulator accepted")
	})
}
