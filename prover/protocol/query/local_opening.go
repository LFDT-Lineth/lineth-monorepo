package query

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/google/uuid"
)

// Queries the opening of a handle at zero
type LocalOpening struct {
	Pol  ifaces.Column
	ID   ifaces.QueryID
	uuid uuid.UUID `serde:"omit"`
}

// Contains the result of a local opening
type LocalOpeningParams struct {
	BaseY  field.Element
	ExtY   fext.Element
	IsBase bool
}

// Updates a Fiat-Shamir state
func (lop LocalOpeningParams) UpdateFS(fs fiatshamir.FS) {
	if lop.IsBase {
		fs.Update(lop.BaseY)
	} else {
		// Change this for the actual extension!
		fs.UpdateExt(lop.ExtY)
	}
}

// Constructs a new local opening query
func NewLocalOpening(id ifaces.QueryID, pol ifaces.Column) LocalOpening {

	if len(pol.GetColID()) == 0 {
		utils.Panic("Assigned a polynomial name with an empty length")
	}

	return LocalOpening{ID: id, Pol: pol, uuid: uuid.New()}
}

// Name implements the [ifaces.Query] interface
func (r LocalOpening) Name() ifaces.QueryID {
	return r.ID
}

// IsBase returns if the column is a base-field column
func (r LocalOpening) IsBase() bool {
	return r.Pol.IsBase()
}

// Constructor for [LocalOpeningParams] when y is a base field element.
func NewLocalOpeningParams(y field.Element) LocalOpeningParams {
	return LocalOpeningParams{
		BaseY:  y,
		ExtY:   fext.Lift(y),
		IsBase: true,
	}
}

// Constructor for [LocalOpeningParams] when y is a base field element.
func NewLocalOpeningParamsExt(z fext.Element) LocalOpeningParams {
	return LocalOpeningParams{
		ExtY:   z,
		IsBase: false,
	}
}

func (lop LocalOpeningParams) ToGenericGroupElement() fext.GenericFieldElem {
	if lop.IsBase {
		return fext.NewGenFieldFromBase(lop.BaseY)
	} else {
		return fext.NewGenFieldFromExt(lop.ExtY)
	}
}

// Test that the polynomial evaluation holds
func (r LocalOpening) Check(run ifaces.Runtime) error {
	params := run.GetParams(r.ID).(LocalOpeningParams)
	if params.IsBase {
		actualY := r.Pol.GetColAssignmentAt(run, 0)
		if actualY != params.BaseY {
			return fmt.Errorf("expected P(x) = %s but got %s for %v", params.BaseY.String(), actualY.String(), r.Pol.GetColID())
		}
	} else {
		actualY := r.Pol.GetColAssignmentAtExt(run, 0)
		if actualY != params.ExtY {
			return fmt.Errorf("expected P(x) = %s but got %s for %v", params.ExtY.String(), actualY.String(), r.Pol.GetColID())
		}
	}

	return nil
}

// Test that the polynomial evaluation holds
func (r LocalOpening) CheckGnark(api frontend.API, run ifaces.GnarkRuntime) {
	params := run.GetParams(r.ID).(GnarkLocalOpeningParams)
	koalaAPI := koalagnark.NewAPI(api)
	if params.IsBase {
		actualY := r.Pol.GetColAssignmentGnarkAt(run, 0)
		koalaAPI.AssertIsEqual(params.BaseY, actualY)
	} else {
		actualY := r.Pol.GetColAssignmentGnarkAtExt(run, 0)
		koalaAPI.AssertIsEqualExt(params.ExtY, actualY)
	}
}

func (r LocalOpening) UUID() uuid.UUID {
	return r.uuid
}
