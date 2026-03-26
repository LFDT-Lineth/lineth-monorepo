package query

import (
	"errors"
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

// Inclusion describes an inclusion query (a.k.a. a lookup constraint). The
// query can feature conditional “included" tables and conditional “including"
// tables. The query can additionally feature an fragmented table meaning that
// the including “table" to consider is the union of two tables.
type Inclusion struct {

	// Included represents the table over which the constraint applies. The
	// columns must be a non-zero collection of columns of the same size.
	Included []ifaces.Column
	// Including represents the reference table of the inclusion constraint. It
	// stores all the values that the “including" table is required to store.
	// The table must be a non-zero collection of columns of the same size.
	//
	// Including can also represent a fragmented table. In that case, the double
	// slice is indexed as [fragment][column]. In the non-fragmented case, the
	// slice is as if there is only a single fragment
	Including [][]ifaces.Column
	// ID stores the identifier string of the query
	ID ifaces.QueryID
	// IncludedFilter is (allegedly) a binary-assigned column specifying
	// with a one whether the corresponding row of the “included" is subjected
	// to the constraint and with a 0 whether the row is disregarded.
	IncludedFilter ifaces.Column
	// IncludingFilter is (allegedly) a binary-assigned column specifying
	// with a one whether a row of the including “table" is allowed and with a
	// 0 whether the corresponding row is forbidden.
	//
	// The slices is indexed per number of fragment, in the non-fragmented case,
	// consider there is only a single segment.
	IncludingFilter []ifaces.Column

	uuid uuid.UUID `serde:"omit"`
}

// NewInclusion constructs an inclusion. Will panic if it is mal-formed
func NewInclusion(
	id ifaces.QueryID,
	included []ifaces.Column,
	including [][]ifaces.Column,
	includedFilter ifaces.Column,
	includingFilter []ifaces.Column,
) Inclusion {

	if len(included) == 0 {
		utils.Panic("the included table has no columns")
	}

	if len(including) == 0 {
		utils.Panic("no including fragments were provided")
	}

	// This works because we already tested that both sides have the same number of columns
	nCol := len(included)

	// Check the existence of the handles
	for i := 0; i < nCol; i++ {
		included[i].MustExists()
	}

	for frag := range including {

		for i := 0; i < nCol; i++ {
			including[frag][i].MustExists()
		}

		if len(including[frag]) != nCol {
			utils.Panic(
				"Table(T)[fragment=%v] and lookups(S) don't have the same number of columns %v %v",
				frag, len(including[frag]), len(included))
		}

		// All columns of including must have the same MaxSize
		if _, err := utils.AllReturnEqual(ifaces.Column.Size, including[frag]); err != nil {
			utils.Panic(
				"The fragment %v of the including table is malformed, all columns must have the same length: %v",
				frag, err.Error(),
			)
		}

		// Checks on filters, and the including filter size
		if includingFilter != nil {
			includingFilter[frag].MustExists() //check the existence of the filter

			if includingFilter[frag].Size() != including[frag][0].Size() {
				utils.Panic(
					"the fragment of the including fragment #%v does not have the same size (%v) as the table fragment it is refering to (%v)",
					frag, includingFilter[frag].Size(), including[frag][0].Size(),
				)
			}
		}
	}

	// Same thing for included
	if _, err := utils.AllReturnEqual(ifaces.Column.Size, included); err != nil {
		utils.Panic("The included table is malformed, all columns must have the same length: %v", err.Error())
	}

	// Checks on filters, and the included filter size
	if includedFilter != nil {
		includedFilter.MustExists() //check the existence of the filter

		if includedFilter.Size() != included[0].Size() {
			utils.Panic(
				"the included filter (size=%v) does not have the same size as the table it is refering to (size=%v)",
				includedFilter.Size(), included[0].Size(),
			)
		}
	}

	return Inclusion{
		Included:        included,
		Including:       including,
		ID:              id,
		IncludedFilter:  includedFilter,
		IncludingFilter: includingFilter,
		uuid:            uuid.New(),
	}
}

// Name implements the [ifaces.Query] interface
func (r Inclusion) Name() ifaces.QueryID {
	return r.ID
}

// IsFilteredOnIncluding returns true if the table is filtered on the included
// side of the table.
func (r Inclusion) IsFilteredOnIncluding() bool {
	return r.IncludingFilter != nil
}

// IsFilteredOnIncluded returns true if the table is filtered on the including
// side of the table
func (r Inclusion) IsFilteredOnIncluded() bool {
	return r.IncludedFilter != nil
}

// Check implements the [ifaces.Query] interface
func (r Inclusion) Check(run ifaces.Runtime) error {

	including := make([][]ifaces.ColAssignment, len(r.Including))
	included := make([]ifaces.ColAssignment, len(r.Included))

	hash := func(v *fext.Element) uint32 {
		h := v.B0.A0.Uint64()
		h = (h * 31) ^ v.B0.A1.Uint64()
		h = (h * 31) ^ v.B1.A0.Uint64()
		h = (h * 31) ^ v.B1.A1.Uint64()
		return uint32(h)
	}

	// Populate the `including`
	for frag := range r.Including {
		including[frag] = make([]smartvectors.SmartVector, len(r.Including[frag]))
		for i, pol := range r.Including[frag] {
			including[frag][i] = pol.GetColAssignment(run)
		}
	}

	// Populate the included
	for i, pol := range r.Included {
		included[i] = pol.GetColAssignment(run)
	}

	// Populate Filters
	var (
		filterIncluding []smartvectors.SmartVector
		filterIncluded  smartvectors.SmartVector
	)

	if r.IsFilteredOnIncluding() {
		filterIncluding = make([]smartvectors.SmartVector, len(r.IncludingFilter))
		for frag := range r.IncludingFilter {
			filterIncluding[frag] = r.IncludingFilter[frag].GetColAssignment(run)
		}
	}

	if r.IsFilteredOnIncluded() {
		filterIncluded = r.IncludedFilter.GetColAssignment(run)
	}

	/*
		Sample a random element alpha, usefull for multivalued inclusion checks
		It allows to reference multiple number through a linear combination
	*/
	var alpha fext.Element
	_, err := alpha.SetRandom()
	if err != nil {
		// Cannot happen unless the entropy was exhausted
		panic(err)
	}

	// Gather the elements of including in a set. Randomly combining the columns
	// so that the rows can be summed up by a single field element, easier to
	// look up in the map.
	numInclusion := 0
	for frag := range r.Including {
		numInclusion += r.Including[frag][0].Size()
	}
	inclusionSet := make(map[uint32]struct{}, numInclusion)
	for frag := range r.Including {
		for row := 0; row < r.Including[frag][0].Size(); row++ {
			if !r.IsFilteredOnIncluding() || filterIncluding[frag].Get(row) == field.One() {
				rand := rowLinComb(alpha, row, including[frag])
				inclusionSet[hash(&rand)] = struct{}{}
			}
		}
	}

	var errLU error

	// Effectively run the check on the included table
	for row := 0; row < r.Included[0].Size(); row++ {
		if r.IsFilteredOnIncluded() && filterIncluded.Get(row) == field.Zero() {
			continue
		}

		rand := rowLinComb(alpha, row, included)
		if _, ok := inclusionSet[hash(&rand)]; !ok {
			notFoundRow := []string{}
			for c := range included {
				x := included[c].Get(row)
				notFoundRow = append(notFoundRow, fmt.Sprintf("%v=%v", r.Included[c].GetColID(), x.Text(16)))
			}

			errLU = errors.Join(errLU, fmt.Errorf("row %v was not found in the `including` table : %v", row, notFoundRow))
		}
	}

	return errLU
}

// GnarkCheck implements the [ifaces.Query] interface. It will panic in this
// construction because we do not have a good way to check the query within a
// circuit
func (i Inclusion) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	panic("UNSUPPORTED : can't check an inclusion query directly into the circuit")
}

// GetShiftedRelatedColumns returns the list of the [HornerParts.Selectors]
// found in the query. This is used to check if the query is compatible with
// Wizard distribution.
//
// Note: the fact that this method is implemented makes [Inclusion] satisfy
// an anonymous interface that is matched to detect queries that are
// incompatible with wizard distribution. So we should not rename or remove
// this implementation without doing the corresponding changes in the
// distributed package. Otherwise, this will silence the checks that we are
// doing.
func (i Inclusion) GetShiftedRelatedColumns() []ifaces.Column {

	res := []ifaces.Column{}

	if i.IncludedFilter != nil && i.IncludedFilter.IsComposite() {
		res = append(res, i.IncludedFilter)
	}

	for _, included := range i.Included {
		if included.IsComposite() {
			res = append(res, included)
		}
	}

	for frag := range i.Including {

		if i.IncludingFilter != nil && i.IncludingFilter[frag] != nil && i.IncludingFilter[frag].IsComposite() {
			res = append(res, i.IncludingFilter[frag])
		}

		for _, col := range i.Including[frag] {
			if col.IsComposite() {
				res = append(res, col)
			}
		}
	}

	return res
}

func (i Inclusion) UUID() uuid.UUID {
	return i.uuid
}
