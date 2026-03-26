package verifiercol

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

type VerifierCol interface {
	ifaces.Column
	Split(comp *wizard.CompiledIOP, from, to int) ifaces.Column
}

// AssertIsPublicCol returns true if the column is public
// TODO @AlexandreBelling, this function seems at the wrong place in
// this package. We should consider rework the package organization
// to make it work in a cleaner manner.
func AssertIsPublicCol(comp *wizard.CompiledIOP, col ifaces.Column) {

	// Sanity-check, the columns should be accessible to the verifier
	grandpa := column.RootParents(col)

	// Careful, that the grandpa might be a verifier defined column
	// in this case, this is OK because this corresponds to a public
	// column but it will panic if we try to get its status so we need
	// to check first.
	if _, ok := grandpa.(VerifierCol); ok {
		return
	}

	// Else, we can deduce the public-hood of the column from looking
	// at its
	status := comp.Columns.Status(grandpa.GetColID())
	if !status.IsPublic() {
		utils.Panic(
			"commitment %v has root column %v of size %v but is not public (status %v)",
			col, grandpa.GetColID(), grandpa.Size(), status.String(),
		)
	}
}

// NewConcatTinyColumns creates a new ConcatTinyColumns. The columns must all
// have a length of "1"
func NewConcatTinyColumns(
	comp *wizard.CompiledIOP,
	paddedSize int,
	paddingVal field.Element,
	cols ...ifaces.Column,
) ifaces.Column {

	access := []ifaces.Accessor{}

	// Check the length of the columns
	for _, col := range cols {
		// sanity-check
		col.MustExists()

		// sanity check the publicity of the column
		AssertIsPublicCol(comp, col)

		if cc, isCC := col.(ConstCol); isCC {
			access = append(access, accessors.NewConstant(cc.F))
			continue
		}

		for pos := 0; pos < col.Size(); pos++ {
			access = append(access, accessors.NewFromPublicColumn(col, pos))
		}
	}

	// Then, the total length must not exceed the the PaddedSize
	if paddedSize < len(cols) {
		utils.Panic("the target length (=%v) is smaller than the given columns (=%v)", paddedSize, len(cols))
	}

	return NewFromAccessors(access, paddingVal, paddedSize)
}
