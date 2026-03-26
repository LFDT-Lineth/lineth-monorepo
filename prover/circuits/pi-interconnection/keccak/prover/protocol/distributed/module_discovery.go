package distributed

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// ModuleName is a name used to identify an horizontal module
type ModuleName string

var (
	// NoModuleFound is a default module name that is returned whenever
	// the discoverer gives conflicting modules names for an item or
	// a column.
	NoModuleFound ModuleName = ModuleName("no module found")
	// AnyModule is a default module name that is returned to indicate
	// the provided item is not assigned any particular module. For
	// instance [verifiercol.FromConst] or an [accessors.FromConst].
	// Unlike [NoModuleFound], it does not indicate an error but more
	// a wildcard.
	AnyModule ModuleName = ModuleName("any module")
)

// paddingInformation is an indication of how a column is padded
type paddingInformation int

const (
	constantPaddingInformation = iota + 1
	leftPaddingInformation
	rightPaddingInformation
	noPaddingInformation
)

// ExpressionIsInModule is a helper function that returns the module of a [symbolic.Expression]
// using the informations of the discoverer. The resolution of the module name
// occurs as follows:
//   - If the expression is a constant, the module is [AnyModule].
//   - By default, the expression of type [AnyModule].
//   - If the expression contains operands that are not [AnyModule]
//     but have the same type. Then the expression takes their type.
//   - If the expression contains variables that are from different modules,
//     (excluding [AnyModule]), the function returns [NoModuleFound].
func ModuleOfExpr(disc *StandardModuleDiscoverer, expr *symbolic.Expression) ModuleName {
	metadata := expr.BoardListVariableMetadata()
	return ModuleOfList(disc, metadata...)
}

// NewSizeOfExpr looks for the metadata in the expressions and resolves the new
// size of the columns in the expression. The function returns 0 if the expression
// does not have any size-resolvable item.
func NewSizeOfExpr(disc *StandardModuleDiscoverer, expr *symbolic.Expression) int {
	metadata := expr.BoardListVariableMetadata()
	newSize := 0

	for _, m := range metadata {
		if c, ok := m.(ifaces.Column); ok {

			cSize := NewSizeOfColumn(disc, c)

			if cSize == 0 {
				continue
			}

			if newSize == 0 {
				newSize = cSize
				continue
			}

			if cSize != newSize {
				utils.Panic("inconsistenct size: col=%v has-size=%v but expected=%v", c.GetColID(), cSize, newSize)
			}
		}
	}
	return newSize
}

// ModuleOfColumn returns the module associated with the provided column.
// The provided column can be of any type unlike what [*StandardModuleDiscoverer.ModuleOf]
// requires.
func ModuleOfColumn(disc *StandardModuleDiscoverer, col ifaces.Column) ModuleName {

	switch c := col.(type) {

	case column.Natural:
		return disc.ModuleOf(c)

	case column.Shifted:
		return ModuleOfColumn(disc, c.Parent)

	case verifiercol.ConstCol:
		return AnyModule

	case verifiercol.FromAccessors:
		return ModuleOfList(disc, c.Accessors...)

	default:
		utils.Panic("unexpected type of column: %T", col)
	}

	return ""
}

// NewSizeOfColumn returns the new size of the provided column. If the
// column size is not resolvable (but expected), the function returns
// zero.
//
// The function panics if the type of the column is unexpected: all
// the [verifiercol.VerifierCol] except for [verifiercol.ConstCol].
func NewSizeOfColumn(disc *StandardModuleDiscoverer, col ifaces.Column) int {

	switch c := col.(type) {
	case column.Natural:
		if c.Status() == column.Precomputed || c.Status() == column.VerifyingKey {
			return c.Size()
		}
		return disc.NewSizeOf(c)
	case column.Shifted:
		return NewSizeOfColumn(disc, c.Parent)
	case verifiercol.ConstCol:
		return 0
	default:
		utils.Panic("expected type of column: %T", col)
	}

	return -1
}

// ModuleOfAccessor returns the module associated with acc
func ModuleOfAccessor(disc *StandardModuleDiscoverer, acc ifaces.Accessor) ModuleName {

	switch a := acc.(type) {
	case *accessors.FromConstAccessor:
		return AnyModule
	case *accessors.FromExprAccessor:
		return ModuleOfExpr(disc, a.Expr)
	case *accessors.FromCoinAccessor:
		return AnyModule
	case *accessors.FromPublicColumn:
		return ModuleOfColumn(disc, a.Col)
	case *accessors.FromLocalOpeningYAccessor:
		return ModuleOfColumn(disc, a.Q.Pol)
	default:
		utils.Panic("expected type of accessor: %T", acc)
	}

	return ""
}

// ModuleOfList returns the module associated with the provided list of
// items. Items can be either [ifaces.Column], [ifaces.Accessor] or [symbolic.Expression].
func ModuleOfList[T any](disc *StandardModuleDiscoverer, items ...T) ModuleName {

	res := AnyModule

	for _, item_ := range items {

		var m ModuleName

		switch item := any(item_).(type) {
		case ifaces.Accessor:
			m = ModuleOfAccessor(disc, item)
		case ifaces.Column:
			m = ModuleOfColumn(disc, item)
		case *symbolic.Expression:
			m = ModuleOfExpr(disc, item)
		case coin.Info:
			m = AnyModule
		case variables.X, variables.PeriodicSample:
			m = AnyModule
		default:
			utils.Panic("unexpected type %T", item)
		}

		if m == NoModuleFound {
			return NoModuleFound
		}

		if m == AnyModule {
			continue
		}

		if res == AnyModule {
			res = m
		}

		if res != m {
			return NoModuleFound
		}
	}

	return res
}

// NewSizeOfList returns the new size of the provided list of items.
// The function asserts that all provided items have the same new size
// without which the
func NewSizeOfList[T any](disc *StandardModuleDiscoverer, items ...T) int {

	res := 0

	for _, item_ := range items {

		sizeOfItem := 0

		switch item := any(item_).(type) {
		case ifaces.Column:
			sizeOfItem = NewSizeOfColumn(disc, item)
		case *symbolic.Expression:
			sizeOfItem = NewSizeOfExpr(disc, item)
		default:
			utils.Panic("unexpected type %T", item)
		}

		if res == 0 {
			res = sizeOfItem
		}

		if res != sizeOfItem {
			utils.Panic("inconsistent sizes %v != %v", res, sizeOfItem)
		}
	}

	return res
}

// MustBeResolved checks that a module name is neither [AnyModule] or [NoModuleFound]
// and throws a panic if it is not.
func (m ModuleName) MustBeResolved() {
	if m == AnyModule {
		utils.Panic("could not resolve module: AnyModule")
	}

	if m == NoModuleFound {
		utils.Panic("could not resolve module: NoModuleFound")
	}
}
