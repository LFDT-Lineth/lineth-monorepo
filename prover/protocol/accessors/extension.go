package accessors

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

var _ ifaces.Accessor = &Extension{}

// Extension is a composite of accessors that represent an extension field,
// where each component represent a coordinate the represented element.
type Extension struct {
	Title  string
	Coords [4]ifaces.Accessor
}

func (e *Extension) Name() string {
	return e.Title
}

func (e *Extension) String() string {
	return e.Name()
}

func (e *Extension) AsVariable() *symbolic.Expression {
	return symbolic.NewVariable(e)
}

func (e *Extension) Round() int {
	return max(e.Coords[0].Round(), e.Coords[1].Round(), e.Coords[2].Round(), e.Coords[3].Round())
}

func (e *Extension) IsBase() bool {
	return false
}

func (e *Extension) GetValBase(run ifaces.Runtime) (field.Element, error) {
	return field.Zero(), fmt.Errorf("not a base element accessor")
}

func (e *Extension) GetVal(run ifaces.Runtime) field.Element {
	panic("should not be called as the result is an extension field")
}

func (e *Extension) GetValExt(run ifaces.Runtime) fext.Element {
	return fext.Element{
		B0: extensions.E2{
			A0: e.Coords[0].GetVal(run),
			A1: e.Coords[1].GetVal(run),
		},
		B1: extensions.E2{
			A0: e.Coords[2].GetVal(run),
			A1: e.Coords[3].GetVal(run),
		},
	}
}

func (e *Extension) GetFrontendVariable(api frontend.API, circ ifaces.GnarkRuntime) koalagnark.Element {
	panic("should not be called as the result is an extension field")
}

func (e *Extension) GetFrontendVariableBase(api frontend.API, circ ifaces.GnarkRuntime) (koalagnark.Element, error) {
	panic("should not be called as the result is an extension field")
}

func (e *Extension) GetFrontendVariableExt(api frontend.API, circ ifaces.GnarkRuntime) koalagnark.Ext {
	return koalagnark.Ext{
		B0: koalagnark.E2{
			A0: e.Coords[0].GetFrontendVariable(api, circ),
			A1: e.Coords[1].GetFrontendVariable(api, circ),
		},
		B1: koalagnark.E2{
			A0: e.Coords[2].GetFrontendVariable(api, circ),
			A1: e.Coords[3].GetFrontendVariable(api, circ),
		},
	}
}
