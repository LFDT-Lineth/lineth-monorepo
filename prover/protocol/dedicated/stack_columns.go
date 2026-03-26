package dedicated

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// StackedColumn is a dedicated wizard computing a column by stacking other
// columns on top of each others.
// When source columns are padded with zeros, the Column field gives the
// stacked column with the padding included. Whereas UnpaddedColumn gives
// the stacking of the source columns without the padding. For example,
// if we have two source columns A and B, with A being [1, 2, 3, 0] and B
// being [4, 5, 6, 0], the stacked column will be [1, 2, 3, 0, 4, 5, 6, 0],
// while the unpadded column will be [1, 2, 3, 4, 5, 6, 0, 0]. Notice that
// stacking sometimes needs zero padding to make the stacked column size a power of two.
type StackedColumn struct {
	// Column is the built column
	Column column.Natural
	// Source is the list of columns to stack. All of them should have the
	// same number of rows.
	Source []ifaces.Column
	// UnpaddedColumn is the column that is built using the unpadded portions
	// of the source columns. This is useful for stacking the zero padded source
	// columns
	//
	// @alex: We store a pointer to Natural because having empty Natural causes
	// issues with the serializer. In principle, we could have fixed the
	// serializer but it's simpler to just use a pointer as it circumvent the
	// issue. However, we take dedicated care to ensure that the pointer is
	// never propagated and is always dereferenced when passed as ifaces.Column.
	UnpaddedColumn *column.Natural
	// ColumnFilter is the filter used for the projection query between
	// Column and UnpaddedColumn.
	//
	// @alex: We store a pointer to Natural because having empty Natural causes
	// issues with the serializer. In principle, we could have fixed the
	// serializer but it's simpler to just use a pointer as it circumvent the
	// issue. However, we take dedicated care to ensure that the pointer is
	// never propagated and is always dereferenced when passed as ifaces.Column.
	ColumnFilter *column.Natural
	// UnpaddedColumnFilter is the filter used for the projection query
	// between Column and UnpaddedColumn.
	//
	// @alex: We store a pointer to Natural because having empty Natural causes
	// issues with the serializer. In principle, we could have fixed the
	// serializer but it's simpler to just use a pointer as it circumvent the
	// issue. However, we take dedicated care to ensure that the pointer is
	// never propagated and is always dereferenced when passed as ifaces.Column.
	UnpaddedColumnFilter *column.Natural
	// UnpaddedSize is the size of the non zero portions of the source columns.
	UnpaddedSize int
	// AreSourceColsPadded indicates whether the source columns are padded or not.
	// We assume the padding size is the same for all source columns.
	AreSourceColsPadded bool
}

// Options for the StackedColumn
type StackColumnOp func(stkCol *StackedColumn)

// StackColumn defines and constrains a [StackedColumn] wizard element.
func StackColumn(comp *wizard.CompiledIOP, srcs []ifaces.Column, opts ...StackColumnOp) *StackedColumn {

	var (
		srcs_length = srcs[0].Size()
		// s is the identity permutation to be computed
		s = make([]smartvectors.SmartVector, 0, len(srcs))
		// count is the total number of elements in the stacked column
		count = 0
		name  = fmt.Sprintf("STACKED_COLUMN_%v_%v", comp.Columns.NumEntriesTotal(), comp.SelfRecursionCount)
		round = 0
		// Variables needed if the number of rows of the
		// stacked column is not a power of two.
		count_padded int
		srcs_padded  []ifaces.Column
		s_padded     []smartvectors.SmartVector
	)

	// Sanity check: all source columns should have the same size
	for i := 1; i < len(srcs); i++ {
		if srcs[i].Size() != srcs_length {
			utils.Panic("All source columns should have the same size, but got %v and %v", srcs_length, srcs[i].Size())
		}
	}

	for i := range srcs {
		round = max(round, srcs[i].Round())
		p := make([]field.Element, srcs_length)
		for j := range p {
			p[j].SetInt64(int64(count))
			count++
		}
		s = append(s, smartvectors.NewRegular(p))
	}

	if !utils.IsPowerOfTwo(count) {
		count_padded = utils.NextPowerOfTwo(count)
		padding_col := verifiercol.NewConstantCol(field.Zero(), srcs_length, "")
		srcs_padded = make([]ifaces.Column, 0, len(srcs)+(count_padded-count)/srcs_length)
		srcs_padded = append(srcs_padded, srcs...)
		for i := 0; i < (count_padded-count)/srcs_length; i++ {
			srcs_padded = append(srcs_padded, padding_col)
		}

		// Next we compute the padded identity permutation
		s_padded = make([]smartvectors.SmartVector, 0, len(s)+(count_padded-count)/srcs_length)
		s_padded = append(s_padded, s...)
		padding_count := 0
		for i := 0; i < (count_padded-count)/srcs_length; i++ {
			p := make([]field.Element, srcs_length)
			for j := range p {
				p[j].SetInt64(int64(count + padding_count))
				padding_count++
			}
			s_padded = append(s_padded, smartvectors.NewRegular(p))
		}
	} else {
		count_padded = count
		srcs_padded = srcs
		s_padded = s
	}

	col := comp.InsertCommit(round, ifaces.ColID(name), count_padded, true)

	comp.InsertFixedPermutation(
		round,
		ifaces.QueryID(name)+"PERMUTATION_CHECK",
		s_padded,
		[]ifaces.Column{col},
		srcs_padded,
	)

	stkCol := &StackedColumn{
		Column: col.(column.Natural),
		Source: srcs,
	}
	for _, op := range opts {
		op(stkCol)
	}

	// Handle the padding of the source columns
	if stkCol.AreSourceColsPadded {

		unpaddedColumn := comp.InsertCommit(
			round,
			ifaces.ColID(name+"_UNPADDED"),
			utils.NextPowerOfTwo(stkCol.UnpaddedSize*len(stkCol.Source)),
			true,
		).(column.Natural)
		stkCol.UnpaddedColumn = &unpaddedColumn

		columnFilter := comp.InsertCommit(
			round,
			ifaces.ColID(name+"_COLUMN_FILTER"),
			stkCol.Column.Size(),
			true,
		).(column.Natural)
		stkCol.ColumnFilter = &columnFilter

		unpaddedColumnFilter := comp.InsertCommit(
			round,
			ifaces.ColID(name+"_UNPADDED_COLUMN_FILTER"),
			stkCol.UnpaddedColumn.Size(),
			true,
		).(column.Natural)
		stkCol.UnpaddedColumnFilter = &unpaddedColumnFilter

		// Insert a projection query on the
		// stacked and unpadded stacked columns
		comp.InsertProjection(
			ifaces.QueryID(name)+"_PROJECTION",
			query.ProjectionInput{
				ColumnA: []ifaces.Column{stkCol.Column},
				ColumnB: []ifaces.Column{*stkCol.UnpaddedColumn},
				FilterA: *stkCol.ColumnFilter,
				FilterB: *stkCol.UnpaddedColumnFilter,
			},
		)
		// Insert a binarity constraint for ColumnFilter and
		// UnpaddedColumnFilter
		MustBeBinary(comp, *stkCol.ColumnFilter, round)
		MustBeBinary(comp, *stkCol.UnpaddedColumnFilter, round)
		return stkCol
	}
	return stkCol
}

// Assigns assigns the stack column
func (s *StackedColumn) Run(run *wizard.ProverRuntime) {
	var (
		column = make([]field.Element, 0, s.Column.Size())
	)
	// Variables needed only when IsPadded is true
	var (
		unpadded_column        []field.Element
		column_filter          []field.Element
		unpadded_column_filter []field.Element
		filterElemNonPadding   []field.Element
		filterElemPadding      []field.Element
	)

	if s.AreSourceColsPadded {
		// Only declare and use these variables if IsPadded is true
		filterElemNonPadding = make([]field.Element, 0, s.UnpaddedSize)
		filterElemPadding = make([]field.Element, 0, s.Source[0].Size()-s.UnpaddedSize)
		unpadded_column = make([]field.Element, 0, s.UnpaddedColumn.Size())
		column_filter = make([]field.Element, 0, s.ColumnFilter.Size())
		unpadded_column_filter = make([]field.Element, 0, s.UnpaddedColumnFilter.Size())

		for i := 0; i < s.UnpaddedSize; i++ {
			filterElemNonPadding = append(filterElemNonPadding, field.One())
		}
		for i := 0; i < s.Source[0].Size()-s.UnpaddedSize; i++ {
			filterElemPadding = append(filterElemPadding, field.Zero())
		}
	}

	// Assign the columns
	for i := range s.Source {
		source_sv := s.Source[i].GetColAssignment(run)
		// Handle both base field and extension smartvectors
		var source_assignment []field.Element
		if baseAssignment, err := source_sv.IntoRegVecSaveAllocBase(); err == nil {
			// This is a base field smartvector
			source_assignment = baseAssignment
		} else {
			// This is a field extension smartvector; use the first coordinate (B0.A0)
			source_assignment_ext := source_sv.IntoRegVecSaveAllocExt()
			source_assignment = make([]field.Element, len(source_assignment_ext))
			for j := range source_assignment_ext {
				source_assignment[j] = source_assignment_ext[j].B0.A0
			}
		}
		column = append(column, source_assignment...)
		if s.AreSourceColsPadded {
			if len(source_assignment) < s.UnpaddedSize {
				panic(fmt.Sprintf(
					"StackedColumn.Run: source column %d has length %d, which is less than UnpaddedSize %d",
					i, len(source_assignment), s.UnpaddedSize,
				))
			}
			source_assignment_unpadded := source_assignment[:s.UnpaddedSize]
			unpadded_column = append(unpadded_column, source_assignment_unpadded...)
			column_filter = append(column_filter, filterElemNonPadding...)
			column_filter = append(column_filter, filterElemPadding...)
			unpadded_column_filter = append(unpadded_column_filter, filterElemNonPadding...)
		}
	}
	run.AssignColumn(s.Column.ID, smartvectors.RightZeroPadded(column, s.Column.Size()))
	if s.AreSourceColsPadded {
		run.AssignColumn(s.UnpaddedColumn.ID, smartvectors.RightZeroPadded(unpadded_column, s.UnpaddedColumn.Size()))
		run.AssignColumn(s.ColumnFilter.ID, smartvectors.RightZeroPadded(column_filter, s.Column.Size()))
		run.AssignColumn(s.UnpaddedColumnFilter.ID, smartvectors.RightZeroPadded(unpadded_column_filter, s.UnpaddedColumnFilter.Size()))
	}
}

// Handles the padded source columns for the stacked column
func HandleSourcePaddedColumns(unpaddedSourceColSize int) StackColumnOp {
	return func(stkCol *StackedColumn) {
		// Sanity check: the source column size should be the next power of two
		// of the unpaddedSourceColSize
		if stkCol.Source[0].Size() != utils.NextPowerOfTwo(unpaddedSourceColSize) {
			utils.Panic("source size %v is not the next power of two of the unpaddedSourceColSize %v", stkCol.Source[0].Size(), unpaddedSourceColSize)
		}
		// If the unpaddedSourceColSize is already a power of two, we dont need anything special
		if utils.IsPowerOfTwo(unpaddedSourceColSize) {
			stkCol.AreSourceColsPadded = false
		} else {
			stkCol.UnpaddedSize = unpaddedSourceColSize
			stkCol.AreSourceColsPadded = true
		}
	}
}

// MustBeBinary constraints c to be binary
func MustBeBinary(comp *wizard.CompiledIOP, c ifaces.Column, round int) {
	comp.InsertGlobal(
		round,
		ifaces.QueryIDf("%v_IS_BINARY", c.GetColID()),
		sym.Mul(c, sym.Sub(c, 1)),
	)
}
