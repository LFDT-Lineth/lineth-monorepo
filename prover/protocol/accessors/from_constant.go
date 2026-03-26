package accessors

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

var _ ifaces.Accessor = &FromConstAccessor{}

// FromConstAccessor implements [ifaces.Accessor]. It symbolizes a constant
// value and be used as input to functions that expects an [ifaces.Accessor] but
// where the caller is only interested in passing a constant value.
type FromConstAccessor struct {
	// F is the constant served by the accessor
	Base       field.Element
	Ext        fext.Element
	IsBaseFlag bool
}

// NewConstant returns an [ifaces.Accessor] object representing a constant value.
func NewConstant(f field.Element) ifaces.Accessor {
	return &FromConstAccessor{
		Base:       f,
		Ext:        fext.Lift(f),
		IsBaseFlag: true,
	}
}

// NewConstant returns an [ifaces.Accessor] object representing a constant value.
func NewConstantExt(f fext.Element) ifaces.Accessor {
	return &FromConstAccessor{
		Base:       field.Zero(),
		Ext:        f,
		IsBaseFlag: false,
	}
}

// Name implements [ifaces.Accessor]
func (c *FromConstAccessor) Name() string {
	if c.IsBaseFlag {
		return fmt.Sprintf("CONST_ACCESSOR_%v", c.Base.String())
	} else {
		return fmt.Sprintf("CONST_ACCESSOR_%v", c.Ext.String())
	}
}

// String implements [github.com/consensys/linea-monorepo/prover/symbolic.Metadata]
func (c *FromConstAccessor) String() string {
	return c.Name()
}

// GetVal implements [ifaces.Accessor]
func (c *FromConstAccessor) GetVal(run ifaces.Runtime) field.Element {
	return c.Base
}

func (c *FromConstAccessor) GetValBase(run ifaces.Runtime) (field.Element, error) {
	if c.IsBaseFlag {
		return c.Base, nil
	} else {
		panic("Requested a base field element from an accessor defined over field extensions.")
	}
}

func (c *FromConstAccessor) GetValExt(run ifaces.Runtime) fext.Element {
	return c.Ext
}

func (c *FromConstAccessor) GetFrontendVariable(_ frontend.API, _ ifaces.GnarkRuntime) koalagnark.Element {
	if c.IsBaseFlag {
		return koalagnark.NewElementFromKoala(c.Base)
	} else {
		panic("Requested a base field element from an accessor defined over field extensions.")
	}
}

func (c *FromConstAccessor) GetFrontendVariableBase(_ frontend.API, _ ifaces.GnarkRuntime) (koalagnark.Element, error) {
	if c.IsBaseFlag {
		return koalagnark.NewElementFromKoala(c.Base), nil
	} else {
		panic("Requested a base field element from an accessor defined over field extensions.")
	}
}

func (c *FromConstAccessor) GetFrontendVariableExt(_ frontend.API, _ ifaces.GnarkRuntime) koalagnark.Ext {
	e := koalagnark.NewExt(c.Ext)
	return e
}

// AsVariable implements the [ifaces.Accessor] interface
func (c *FromConstAccessor) AsVariable() *symbolic.Expression {
	if c.IsBaseFlag {
		return symbolic.NewConstant(c.Base)
	} else {
		return symbolic.NewConstant(c.Ext)
	}
}

// Round implements the [ifaces.Accessor] interface
func (c *FromConstAccessor) Round() int {
	return 0
}

func (c *FromConstAccessor) IsBase() bool {
	return c.IsBaseFlag
}
