package fri

import (
	"errors"
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// MultiSizeTable is represents the witness in a FRI multisize batch commitment.
// It consists of a list of (possibly empty) tables corresponding to a power of
// size.
//
// We expect Tables[i].Size() = K * 2^i || 0 where K is some constant
// depending on the situation. If the MultiSizeTable represents the plaintext
// witness, then K=1 but if the MultiSizeTable represents the encoded witness
// the factor K corresponds to the blowup factor of the code. The last entry may
// not be empty.
type MultiSizeTable []SizedTable

// SizedTable represents a list of encoded rows of the same size. For all i and
// j, k we have the invariant that len(Base[i]) == len(Base[j]) and
// len(Base[i]) == len(Ext[k]) whenever this is defined. Their length is also a
// power of 2.
type SizedTable struct {
	Base [][]field.Element
	Ext  [][]field.Ext
	// Groups optionally splits the rows into groups, each committed with its
	// own leaf hash (see [MultiSizeTable.Merkleize]). Nil means a single group
	// holding every row.
	Groups []GroupShape
}

// GroupShape is one group of a size: it takes the next BaseWidth base rows and
// the next ExtWidth extension rows of that size, in order. The groups of a size
// cover all of its rows, and every group holds at least one row.
type GroupShape struct {
	BaseWidth int
	ExtWidth  int
}

// sizeGroups returns the groups of a size of the given widths: groups itself,
// or one group of every row when groups is nil.
func sizeGroups(baseWidth, extWidth int, groups []GroupShape) []GroupShape {
	if groups == nil {
		return []GroupShape{{BaseWidth: baseWidth, ExtWidth: extWidth}}
	}
	return groups
}

// checkGroups reports whether groups splits a size of the given widths: every
// group is nonempty and together they hold exactly its rows. Nil is the single
// group and always valid.
func checkGroups(baseWidth, extWidth int, groups []GroupShape) error {
	if groups == nil {
		return nil
	}
	var base, ext int
	for i, g := range groups {
		if g.BaseWidth < 0 || g.ExtWidth < 0 || g.BaseWidth+g.ExtWidth == 0 {
			return fmt.Errorf("group %d has widths (%d, %d), want a nonempty group", i, g.BaseWidth, g.ExtWidth)
		}
		base += g.BaseWidth
		ext += g.ExtWidth
	}
	if base != baseWidth || ext != extWidth {
		return fmt.Errorf("groups hold (%d, %d) rows, want (%d, %d)", base, ext, baseWidth, extWidth)
	}
	return nil
}

// grouped reports whether the table declares groups at any size. A grouped
// table is committed group by group at every size, the largest one included
// (see [MultiSizeTable.Merkleize]).
func (table MultiSizeTable) grouped() bool {
	for i := range table {
		if table[i].Groups != nil {
			return true
		}
	}
	return false
}

// groupViews returns one view of table per group, sharing its rows.
func (table *SizedTable) groupViews() []SizedTable {
	groups := sizeGroups(len(table.Base), len(table.Ext), table.Groups)
	views := make([]SizedTable, len(groups))
	var base, ext int
	for i, g := range groups {
		views[i] = SizedTable{
			Base: table.Base[base : base+g.BaseWidth],
			Ext:  table.Ext[ext : ext+g.ExtWidth],
		}
		base += g.BaseWidth
		ext += g.ExtWidth
	}
	return views
}

// checkWellFormedness checks if the multi-size table has valid dimensions. It
// returns the value of K (corresponding to the size of the first sub-table).
func (table MultiSizeTable) checkWellFormedness() (k int, err error) {

	k = -1 // sentinel value

	for i := range table {

		size := table[i].Size() // -1 if table[i] is empty
		if size < 0 && i == len(table)-1 {
			return 0, errors.New("last entry is empty")
		}
		if err := checkGroups(len(table[i].Base), len(table[i].Ext), table[i].Groups); err != nil {
			return 0, fmt.Errorf("size %d: %w", i, err)
		}

		if size < 0 {
			continue
		}

		kOther := size / (1 << i)
		if k < 0 {
			k = kOther
		}

		if k != kOther {
			return 0, errors.New("inconsistent size")
		}
	}

	return k, nil
}

// NumRows counts the total number of rows: len(Base) + len(Ext)
func (table *SizedTable) NumRows() int {
	return len(table.Base) + len(table.Ext)
}

// Size returns the size of the table. Returns -1 as a sentinel value.
func (table *SizedTable) Size() int {

	size := -1

	for _, base := range table.Base {
		if size < 0 {
			size = len(base)
		}

		if size != len(base) {
			panic("inconsistent sizes: base")
		}
	}

	for _, ext := range table.Ext {
		if size < 0 {
			size = len(ext)
		}

		if size != len(ext) {
			panic("inconsistent sizes: ext")
		}
	}

	if size < 0 {
		return size
	}

	if !utils.IsPowerOfTwo(size) {
		panic("the size is not a power of 2")
	}

	return size
}
