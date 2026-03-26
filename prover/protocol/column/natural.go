package column

import (
	"fmt"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"

	"github.com/consensys/linea-monorepo/prover/utils"
)

var _ ifaces.Column = &Natural{}

// Natural represents a [ifaces.Column] that has been directly declared in the
// corresponding [github.com/consensys/linea-monorepo/prover/protocol/wizard.CompiledIOP] or [github.com/consensys/linea-monorepo/prover/protocol/wizard.Builder]
// object. The struct should not be constructed directly and should be
// constructed from the [github.com/consensys/linea-monorepo/prover/protocol/wizard.CompiledIOP]
type Natural struct {
	// The ID of the column
	ID ifaces.ColID
	// position contains the indexes of the column in the store.
	position columnPosition
	// store points to the Store where the column is registered. It is accessed
	// to fetch static informations about the column such as its size or its
	// status.
	store *Store
}

// Size returns the size of the column, as required by the [ifaces.Column]
// interface.
func (n Natural) Size() int {
	return n.store.GetSize(n.ID)
}

// GetColID implements the [ifaces.Column] interface and returns the string
// identifier of the column.
func (n Natural) GetColID() ifaces.ColID {
	return n.ID
}

// MustExists validates the construction of the natural handle and implements
// the [ifaces.Column] interface.
func (n Natural) MustExists() {
	// store pointer is set
	if n.store == nil {
		utils.Panic("no entry for store in %v", n.GetColID())
	}

	s := n.store

	// check the positions matches
	storedPos := s.indicesByNames.MustGet(n.ID)
	if n.position != storedPos {
		utils.Panic("mismatched position has %v, but stored was %v", n.position, storedPos)
	}
}

// Round retuns the round of definition of the column. See [ifaces.Column] as
// method implements the interface.
func (n Natural) Round() int {
	return n.position.round
}

// IsComposite implements [ifaces.Column], by definition, it is not a
// composite thus it shall always return true
func (n Natural) IsComposite() bool { return false }

// GetColAssignment implements [ifaces.Column]
func (n Natural) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	return run.GetColumn(n.ID)
}

// GetColAssignmentAt implements [ifaces.Column]
func (n Natural) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	return run.GetColumnAt(n.ID, pos)
}

func (n Natural) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {
	return run.GetColumnAtBase(n.ID, pos)
}

func (n Natural) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {
	return run.GetColumnAtExt(n.ID, pos)
}

// GetColAssignmentGnark implements [ifaces.Column]
func (n Natural) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {
	return run.GetColumn(n.ID)
}

func (n Natural) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	if !n.store.info(n.ID).IsBase {
		return []koalagnark.Element{}, fmt.Errorf("requested base elements but column defined over field extensions")
	}
	return run.GetColumn(n.ID), nil
}

func (n Natural) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	return run.GetColumnExt(n.ID)
}

// GetColAssignmentGnarkAt implements [ifaces.Column]
func (n Natural) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	return run.GetColumnAt(n.ID, utils.PositiveMod(pos, n.Size()))
}

func (n Natural) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	if !n.store.info(n.ID).IsBase {
		return koalagnark.NewElement(0), fmt.Errorf("requested base elements but column defined over field extensions")
	}
	return run.GetColumnAt(n.ID, utils.PositiveMod(pos, n.Size())), nil
}

func (n Natural) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	return run.GetColumnAtExt(n.ID, utils.PositiveMod(pos, n.Size()))
}

// String returns the ID of the column as a string and implements [ifaces.Column]
// and [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (n Natural) String() string {
	return string(n.GetColID()) + "_" + strconv.Itoa(n.Round()) + "_" + strconv.Itoa(n.Size())
}

// Status returns the status of the column. It is only implemented for Natural
// and not by the other composite columns.
func (n Natural) Status() Status {
	return n.store.Status(n.ID)
}

func (n Natural) IsBase() bool {
	return n.store.info(n.ID).IsBase
}

// SetPragma sets the pragma for a given column name.
func (n Natural) SetPragma(pragma string, data any) {
	n.store.SetPragma(n.ID, pragma, data)
}

// GetPragma returns the pragma for a given column name.
func (n Natural) GetPragma(pragma string) (any, bool) {
	return n.store.GetPragma(n.ID, pragma)
}

// GetStoreUnsafe returns the internal store pointer of the column. It is unsafe to
// use.
func (n Natural) GetStoreUnsafe() *Store {
	return n.store
}
