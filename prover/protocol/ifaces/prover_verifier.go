package ifaces

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
)

// Runtime is implemented by the [github.com/consensys/linea-monorepo/protocol/wizard.ProverRuntime] and
// [wizard.VerifierRuntime]. The interface exists to prevent circular
// dependencies internally.
type Runtime interface {
	// GetColumn returns the assignment of a registered column.
	GetColumn(ColID) ColAssignment
	// GetColumnAt returns a particular position of an assigned column.
	GetColumnAt(ColID, int) field.Element
	GetColumnAtBase(ColID, int) (field.Element, error)
	GetColumnAtExt(ColID, int) fext.Element
	// GetRandomCoinFieldExt returns the value of a random challenge coin
	GetRandomCoinFieldExt(name coin.Name) fext.Element
	// GetRandomCoinIntegerVec returns the value of a coin.IntegerVec coin
	GetRandomCoinIntegerVec(name coin.Name) []int
	// GetParams returns the runtime parameters of a query
	GetParams(id QueryID) QueryParams
}

// Interface implemented by the [github.com/consensys/linea-monorepo/protocol/wizard.WizardVerifierCircuit]. The interface
// exists to prevent circular dependencies internally.
type GnarkRuntime interface {
	// GetColumn is as [Runtime.GetColumn] but in a gnark circuit
	GetColumn(ColID) []koalagnark.Element
	GetColumnBase(ColID) ([]koalagnark.Element, error)
	GetColumnExt(ColID) []koalagnark.Ext
	// GetColumnAt is as [Runtime.GetColumnAt] but in a gnark circuit
	GetColumnAt(ColID, int) koalagnark.Element
	GetColumnAtBase(ColID, int) (koalagnark.Element, error)
	GetColumnAtExt(ColID, int) koalagnark.Ext
	// GetRandomCoinField is as [Runtime.GetRandomCoinField] but in a gnark circuit
	GetRandomCoinFieldExt(name coin.Name) koalagnark.Ext
	// GetRandomCoinIntegerVec is as [Runtime.GetRandomCoinIntegerVec] but in a gnark circuit
	GetRandomCoinIntegerVec(name coin.Name) []koalagnark.Element
	// GetParams is as [Runtime.GetParams] but in a gnark circuit
	GetParams(id QueryID) GnarkQueryParams
}
