package verifiercol

import (
	"errors"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// RepeatedAccessor is a [VerifierCol] implementation that is built by
// repeated Size times the same accessor. It will not work if the provided
// accessor require "api" to be evaluated in a gnark circuit.
type RepeatedAccessor struct {
	Accessor ifaces.Accessor
	ColSize  int
}

func (f RepeatedAccessor) IsBase() bool {
	return f.Accessor.IsBase()
}

func (f RepeatedAccessor) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {
	if !f.IsBase() {
		return field.Element{}, errors.New("not base")
	}

	return f.Accessor.GetVal(run), nil
}

func (f RepeatedAccessor) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {
	if f.IsBase() {
		v, err := f.Accessor.GetValBase(run)
		if err != nil {
			panic(err)
		}
		return fext.Lift(v)
	}

	return f.Accessor.GetValExt(run)
}

func (f RepeatedAccessor) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	if !f.Accessor.IsBase() {
		// Note that the slice is not a copy of the frontend variable, but rather a slice of the same object.
		return nil, errors.New("accessor is not not base")
	}

	res := make([]koalagnark.Element, f.Size())
	x := f.Accessor.GetFrontendVariable(nil, run)
	for i := range res {
		res[i] = x
	}
	return res, nil
}

func (f RepeatedAccessor) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	res := make([]koalagnark.Ext, f.Size())
	x := f.Accessor.GetFrontendVariableExt(nil, run)
	for i := range res {
		res[i] = x
	}
	return res
}

func (f RepeatedAccessor) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	if f.Accessor.IsBase() {
		return f.Accessor.GetFrontendVariable(nil, run), nil
	}
	return koalagnark.Element{}, errors.New("accessor is not base")
}

func (f RepeatedAccessor) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	return f.Accessor.GetFrontendVariableExt(nil, run)
}

// NewRepeatedAccessor instantiates a [RepeatedAccessor] column from an
// [ifaces.Accessor] and a size.
func NewRepeatedAccessor(accessor ifaces.Accessor, size int) ifaces.Column {
	return RepeatedAccessor{Accessor: accessor, ColSize: size}
}

// Round returns the round ID of the column and implements the [ifaces.Column]
// interface.
func (f RepeatedAccessor) Round() int {
	return f.Accessor.Round()
}

// GetColID returns the column ID
func (f RepeatedAccessor) GetColID() ifaces.ColID {
	return ifaces.ColIDf("REPEATED_%v_SIZE=%v", f.Accessor.Name(), f.ColSize)
}

// MustExists implements the [ifaces.Column] interface and always returns true.
func (f RepeatedAccessor) MustExists() {}

// Size returns the size of the colum and implements the [ifaces.Column]
// interface.
func (f RepeatedAccessor) Size() int {
	return f.ColSize
}

// Split implements the [VerifierCol] interface
func (f RepeatedAccessor) Split(_ *wizard.CompiledIOP, from, to int) ifaces.Column {
	return NewRepeatedAccessor(f.Accessor, to-from)
}

// GetColAssignment returns the assignment of the current column
func (f RepeatedAccessor) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	return smartvectors.NewConstant(f.Accessor.GetVal(run), f.Size())
}

// GetColAssignmentGnark returns a gnark assignment of the current column
func (f RepeatedAccessor) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {
	res := make([]koalagnark.Element, f.Size())
	x := f.Accessor.GetFrontendVariable(nil, run)
	for i := range res {
		res[i] = x
	}
	return res
}

// GetColAssignmentAt returns a particular position of the column
func (f RepeatedAccessor) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	return f.Accessor.GetVal(run)
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (f RepeatedAccessor) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	return f.Accessor.GetFrontendVariable(nil, run)
}

// IsComposite implements the [ifaces.Column] interface
func (f RepeatedAccessor) IsComposite() bool {
	return false
}

// String implements the [symbolic.Metadata] interface
func (f RepeatedAccessor) String() string {
	return f.Accessor.Name()
}
