package fri

import (
	"errors"

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
