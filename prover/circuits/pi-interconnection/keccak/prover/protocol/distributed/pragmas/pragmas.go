// pragmas allows to define some pragma that are used by the distributed protocol compiler
package pragmas

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
)

type Pragma = string

const (
	// FullColumnPragma is used to mark that a column is a full column.
	// This will help the module discoverer to determine that the column is
	// actually full-density and not a regular column with a suboptimal representation.
	FullColumnPragma Pragma = "fullcolumn-pragma"

	// RightPadded is used to indicate that a column is right-padded
	RightPadded Pragma = "rightpadded-pragma"

	// LeftPadded is used to indicate that a column is left-padded
	LeftPadded Pragma = "leftpadded-pragma"

	None Pragma = "none-pragma"

	// PadWithZeroes is used to indicate that column should be padded with zeroes
	// only regardless of the default heuristic (which is to pad using the first
	// or the last value).
	PadWithZeroes Pragma = "pad-with-zeroes-pragma"

	// ModuleAdviceRef is used to attach a module name to a column. The module
	// name can be used to provide a module advice.
	ModuleAdviceRef Pragma = "module-advice-ref-pragma"
)

// MarkFullColumn marks a column as full-column.
func MarkFullColumn(col ifaces.Column) {
	nat := col.(column.Natural)
	nat.SetPragma(FullColumnPragma, true)
}

// IsFullColumn checks if a column is a full-column.
func IsFullColumn(col ifaces.Column) bool {
	nat := col.(column.Natural)
	_, ok := nat.GetPragma(FullColumnPragma)
	return ok
}

// IsRightPadded checks if a column is right-padded.
func IsRightPadded(col ifaces.Column) bool {
	nat := col.(column.Natural)
	_, ok := nat.GetPragma(RightPadded)
	return ok
}

// IsLeftPadded checks if a column is left-padded.
func IsLeftPadded(col ifaces.Column) bool {
	nat := col.(column.Natural)
	_, ok := nat.GetPragma(LeftPadded)
	return ok
}

// MarkRightPadded marks a column as right-padded.
func MarkRightPadded(col ifaces.Column) {
	nat := col.(column.Natural)
	nat.SetPragma(RightPadded, true)
}

// MarkLeftPadded marks a column as left-padded.
func MarkLeftPadded(col ifaces.Column) {
	nat := col.(column.Natural)
	nat.SetPragma(LeftPadded, true)
}

// AddModuleRef adds a module reference to a column. When a reference is added
// to a column it can be used to provide a module advice instead of using the
// column name. It is useful if the column name is not stable.
//
// If the module ref is specified for a column then the module discoverer will
// *require* an advice to provided for the reference.
func AddModuleRef(col ifaces.Column, moduleName string) {
	nat := col.(column.Natural)
	nat.SetPragma(ModuleAdviceRef, moduleName)
}

// TryGetModuleRef returns the module reference for a column.
func TryGetModuleRef(col ifaces.Column) (string, bool) {
	nat := col.(column.Natural)
	moduleName, ok := nat.GetPragma(ModuleAdviceRef)
	if ok {
		return moduleName.(string), true
	}
	return "", false
}

// IsZeroPadded checks if a column is zero-padded.
func IsZeroPadded(col ifaces.Column) bool {
	nat := col.(column.Natural)
	_, ok := nat.GetPragma(PadWithZeroes)
	return ok
}

// MarkZeroPadded marks a column as zero-padded.
func MarkZeroPadded(col ifaces.Column) {
	nat := col.(column.Natural)
	nat.SetPragma(PadWithZeroes, true)
}
