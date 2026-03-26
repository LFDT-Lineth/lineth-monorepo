package ifaces

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// ColID is a [Column]'s unique string identifier.
//
// Ideally, the ColID should provide some context for the role that the
// identified column plays in the specified protocol. The proposed convention
// for the names is screaming-snake-case `ABDC_DEFR`. The convention is
// followed by the rest of the compiler.
type ColID string

// ColAssignment denotes a runtime (or static) assignment to a [Column]. In
// other words, a column symbolically represents a placeholder to an array of
// field elements and a ColAssignment denotes an array of field elements that
// corresponds to an assignment to a column.
type ColAssignment = smartvectors.SmartVector

// ColIDf is a convenience function to format ColIDs that provides an interface
// similar to [fmt.Printf]. It allows to more succinctly define the name of
// a column.
func ColIDf(s string, args ...interface{}) ColID {
	return ColID(fmt.Sprintf(s, args...))
}

// MarshalJSON implements [json.Marshaler] directly returning the name as a
// quoted string.
func (n *ColID) MarshalJSON() ([]byte, error) {
	var (
		nString = string(*n)
		nQuoted = strconv.Quote(nString)
	)
	return []byte(nQuoted), nil
}

// UnmarshalJSON implements [json.Unmarshaler] directly assigning the receiver's
// value from the unquoted string value of the bytes.
func (n *ColID) UnmarshalJSON(b []byte) error {
	var (
		nQuoted        = string(b)
		nUnquoted, err = strconv.Unquote(nQuoted)
	)

	if err != nil {
		return fmt.Errorf("could not unmarshal Name from unquoted string: %v : %w", nQuoted, err)
	}

	*n = ColID(nUnquoted)
	return nil
}

// Column symbolically represents a vector of field elements which may
// represent a committed vector, or a set of field elements that are part of
// proof of the protocol or even a part of the preprocessing of the protocol
// (i.e. a part of the protocol which can be computed offline). A column may
// refer directly to values that have been (or have to be) assigned in the
// Wizard or to derive vectors (which we also denote by abstract references
// sometime). Columns always have a static size, meaning that their size should
// be stated when declaring the column prior to compiling the Wizard. A current
// limitation is that all columns must have size equal to a power of two.
type Column interface {
	// This ensures that a column can be used as a symbolic variable within a
	// symbolic arithmetic expression.
	symbolic.Metadata
	// Round returns the ID of the prover-verifier interaction round at which
	// the column is assigned by the prover in the protocol. This corresponds
	// to the number of times the verifier sends a batch of random coins to the
	// prover before the column may be computed.
	Round() int
	// Size returns the size of the referenced column (i.e. the number of values
	// in the vector).
	Size() int
	// GetColID returns the column's unique string identifier. The [ColID]
	// typically provide context to what the column is used for and where it was
	// computed.
	GetColID() ColID
	// Returns true if the column is registered. This is trivial by design
	// (because [github.com/consensys/linea-monorepo/protocol/column.Natural] column objects are built by the function that registers
	// it). The goal of this function is to assert this fact. Precisely, it will
	// check if a corresponding entry in the store exists. If it does not, it
	// panics.
	MustExists()
	// GetColAssignment retrieves the assignment of the receiver column from a
	// [github.com/consensys/linea-monorepo/protocol/wizard.ProverRuntime]. It panics if the column has not been assigned yet. It is
	// the preferred way to extract the assignment of the column and should be
	// preferred over calling [Runtime.GetColumn] as the latter
	// will not accept columns that are not of the [column.Natural] type.
	GetColAssignment(run Runtime) ColAssignment
	// GetColAssignmentAt retrieves the assignment of a column at a particular
	// position. For instance, col.GetColAssignment(run, 0), returns the first
	// position of the assigned column.
	GetColAssignmentAt(run Runtime, pos int) field.Element
	GetColAssignmentAtBase(run Runtime, pos int) (field.Element, error)
	GetColAssignmentAtExt(run Runtime, pos int) fext.Element
	// GetColAssignmentGnark does the same as GetColAssignment but in a gnark
	// circuit. This will panic if the column is not yet assigned or if the
	// column is not visible by the verifier. For instance, it will panic if the
	// column is tagged as committed.
	GetColAssignmentGnark(run GnarkRuntime) []koalagnark.Element
	GetColAssignmentGnarkBase(run GnarkRuntime) ([]koalagnark.Element, error)
	GetColAssignmentGnarkExt(run GnarkRuntime) []koalagnark.Ext
	// GetColAssignmentGnarkAt recovers the assignment of the column at a
	// particular position. This will panic if the column is not yet assigned or if the
	// column is not visible by the verifier. For instance, it will panic if the
	// column is tagged as committed.
	GetColAssignmentGnarkAt(run GnarkRuntime, pos int) koalagnark.Element
	// IsComposite states whether a column is constructed by deriving one or
	// more columns. For instance [github.com/consensys/linea-monorepo/protocol/column.Natural] is not a composite column as
	// it directly refers to a set of values provided to the Wizard by the user
	// or by an intermediate compiler step. And [github.com/consensys/linea-monorepo/protocol/column.Shift] is a composite
	// column as it is derived from an underlying column (which may or may not
	// be a composite column itself)
	IsComposite() bool
	GetColAssignmentGnarkAtBase(run GnarkRuntime, pos int) (koalagnark.Element, error)
	GetColAssignmentGnarkAtExt(run GnarkRuntime, pos int) koalagnark.Ext
}

// ColumnAsVariable instantiates a [symbolic.Variable] from a column. The [symbolic.Variable]
// can be used to build arithmetic expressions involving the column and
// they can then be used to specify a [github.com/consensys/linea-monorepo/prover/protocol/query.GlobalConstraint] for instance.
//
// @alex: this is super verbose and cumbersome. It would be great if we could make
// this conversion implicit. An idea to improve this, would be to update the
// protocol/symbolic package API to provide functions of the form
// symbolic.Add(inputs ...any) where the type inference is delegated within the
// function.
func ColumnAsVariable(h Column) *symbolic.Expression {
	return symbolic.NewVariable(h)
}

// MustBeInRound asserts the round of registration of the commitment. It is
// useful to write defensive sanity-checks in the code.
func MustBeInRound(h Column, round int) {
	if round != h.Round() {
		utils.Panic("wrong round assertion for %v, expected %v but was %v", h.GetColID(), round, h.Round())
	}
}

// AssertNotComposite asserts that the handle is [column.Natural] or a verifier-col (see [github.com/consensys/linea-monorepo/protocol/column/verifiercol]). It is a convenience
// function to write sanity-checks as a defensive programming technique, and it
// is useful for writing a compiler.
func AssertNotComposite(h Column) {
	if h.IsComposite() {
		utils.Panic(
			"expected a natural handle, but got %v. You probably forgot to add"+
				"a `univariates.Naturalize` step the compilation process",
			reflect.TypeOf(h),
		)
	}
}

// AssertSameLength is a utility function comparing the Size of all the columns
// in `list` , panicking if the lengths are not all the same and returning the
// shared length otherwise.
func AssertSameLength(list ...Column) int {
	if len(list) == 0 {
		panic("passed an empty leaf")
	}

	res := list[0].Size()
	for i := range list {
		if list[i].Size() != res {
			utils.Panic("the column %v (size %v) does not have the same size as column 0 (size %v)", i, list[i].Size(), res)
		}
	}

	return res
}
