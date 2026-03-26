package verifiercol

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// compile check to enforce the struct to belong to the corresponding interface
var _ VerifierCol = ExpandedProofOrVerifyingKeyColWithZero{}

type ExpandedProofOrVerifyingKeyColWithZero struct {
	Col       ifaces.Column
	Expansion int
}

// Round returns the round ID of the column and implements the [ifaces.Column]
// interface.
func (ex ExpandedProofOrVerifyingKeyColWithZero) Round() int {
	return ex.Col.Round()
}

// GetColID returns the column ID
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColID() ifaces.ColID {
	return ifaces.ColIDf("EXPANDED_%v_%v_WITH_ZERO", ex.Col.GetColID(), ex.Expansion)
}

// MustExists implements the [ifaces.Column] interface and always returns true.
func (ex ExpandedProofOrVerifyingKeyColWithZero) MustExists() {
	ex.Col.MustExists()
}

// Size returns the size of the colum and implements the [ifaces.Column]
// interface.
func (ex ExpandedProofOrVerifyingKeyColWithZero) Size() int {
	return ex.Col.Size() * ex.Expansion
}

// IsBase returns the base status of the column and implements the [ifaces.Column]
func (ex ExpandedProofOrVerifyingKeyColWithZero) IsBase() bool {
	return ex.Col.IsBase()
}

// GetColAssignment returns the assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignment(run ifaces.Runtime) ifaces.ColAssignment {
	assi := ex.Col.GetColAssignment(run)
	if smartvectors.IsBase(assi) {
		values := make([][]field.Element, ex.Expansion)
		values[0] = smartvectors.IntoRegVec(assi)
		// zeros denote slice of zeros of length of the verifier column
		zeros := make([]field.Element, ex.Col.Size())
		// we start at 1 because the first slice is the actual verifier column
		// the other slices are zeros
		for j := 1; j < ex.Expansion; j++ {
			values[j] = zeros
		}
		res := vector.Interleave(values...)
		return smartvectors.NewRegular(res)
	}

	// extension case
	assiExt := smartvectors.IntoRegVecExt(assi)
	// prepare slices of extension elements
	valuesExt := make([][]fext.Element, ex.Expansion)
	valuesExt[0] = assiExt
	zerosExt := make([]fext.Element, ex.Col.Size())
	for j := 1; j < ex.Expansion; j++ {
		valuesExt[j] = zerosExt
	}
	// interleave into a single slice
	newSize := ex.Expansion * ex.Col.Size()
	resExt := make([]fext.Element, newSize)
	for i := 0; i < ex.Expansion; i++ {
		for j := 0; j < ex.Col.Size(); j++ {
			resExt[i+j*ex.Expansion] = valuesExt[i][j]
		}
	}
	return smartvectors.NewRegularExt(resExt)
}

// GetColAssignmentGnark returns a gnark assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnark(run ifaces.GnarkRuntime) []koalagnark.Element {
	assi := ex.Col.GetColAssignmentGnark(run)
	res := make([]koalagnark.Element, ex.Size())
	for i := 0; i < len(assi); i++ {
		res[i*ex.Expansion] = assi[i]
		for j := 1; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = koalagnark.NewElement(0)
		}
	}
	return res
}

// GetColAssignmentGnarkBase returns a gnark assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkBase(run ifaces.GnarkRuntime) ([]koalagnark.Element, error) {
	assi, err := ex.Col.GetColAssignmentGnarkBase(run)
	if err != nil {
		return nil, err
	}
	res := make([]koalagnark.Element, ex.Size())
	for i := 0; i < len(assi); i++ {
		res[i*ex.Expansion] = assi[i]
		for j := 1; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = koalagnark.NewElement(0)
		}
	}
	return res, nil
}

// GetColAssignmentGnarkExt returns a gnark assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkExt(run ifaces.GnarkRuntime) []koalagnark.Ext {
	assi := ex.Col.GetColAssignmentGnarkExt(run)
	res := make([]koalagnark.Ext, ex.Size())
	zeroExt := koalagnark.NewExt(fext.Zero())
	for i := 0; i < len(assi); i++ {
		res[i*ex.Expansion] = assi[i]
		for j := 1; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = zeroExt
		}
	}
	return res
}

// GetColAssignmentGnarkExt returns a gnark assignment of the current column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkExtAsPtr(run ifaces.GnarkRuntime) []*koalagnark.Ext {
	assi := ex.Col.GetColAssignmentGnarkExt(run)
	res := make([]*koalagnark.Ext, ex.Size())
	zeroExt := koalagnark.NewExt(fext.Zero())
	for i := 0; i < len(assi); i++ {
		res[i*ex.Expansion] = &assi[i]
		for j := 1; j < ex.Expansion; j++ {
			res[j+i*ex.Expansion] = &zeroExt
		}
	}
	return res
}

// GetColAssignmentAt returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentAt(run ifaces.Runtime, pos int) field.Element {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentAt(run, pos/ex.Expansion)
	}
	return field.Zero()
}

// GetColAssignmentAtBase returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentAtBase(run ifaces.Runtime, pos int) (field.Element, error) {
	if pos%ex.Expansion == 0 {
		r, err := ex.Col.GetColAssignmentAtBase(run, pos/ex.Expansion)
		if err != nil {
			return field.Zero(), err
		}
		return r, nil
	}
	return field.Zero(), nil
}

// GetColAssignmentAtExt returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentAtExt(run ifaces.Runtime, pos int) fext.Element {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentAtExt(run, pos/ex.Expansion)
	}
	return fext.Zero()
}

// GetColAssignmentGnarkAt returns a particular position of the column in a gnark circuit
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkAt(run ifaces.GnarkRuntime, pos int) koalagnark.Element {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentGnarkAt(run, pos/ex.Expansion)
	}
	return koalagnark.NewElement(0)
}

// GetColAssignmentAtBase returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkAtBase(run ifaces.GnarkRuntime, pos int) (koalagnark.Element, error) {
	if pos%ex.Expansion == 0 {
		r, err := ex.Col.GetColAssignmentGnarkAtBase(run, pos/ex.Expansion)
		if err != nil {
			return koalagnark.NewElement(0), err
		}
		return r, nil
	}
	return koalagnark.NewElement(0), nil
}

// GetColAssignmentAtExt returns a particular position of the column
func (ex ExpandedProofOrVerifyingKeyColWithZero) GetColAssignmentGnarkAtExt(run ifaces.GnarkRuntime, pos int) koalagnark.Ext {
	if pos%ex.Expansion == 0 {
		return ex.Col.GetColAssignmentGnarkAtExt(run, pos/ex.Expansion)
	}
	return koalagnark.NewExt(fext.Zero())
}

// IsComposite implements the [ifaces.Column] interface
func (ex ExpandedProofOrVerifyingKeyColWithZero) IsComposite() bool {
	return ex.Col.IsComposite()
}

// String implements the [symbolic.Metadata] interface
func (ex ExpandedProofOrVerifyingKeyColWithZero) String() string {
	return string(ex.GetColID())
}

// Split implements the [VerifierCol] interface,
// it is no-op for proofcols
func (ex ExpandedProofOrVerifyingKeyColWithZero) Split(_ *wizard.CompiledIOP, from, to int) ifaces.Column {
	return ex.Col
}
