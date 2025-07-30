package common

import (
	"strings"

	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// It commits to the columns of round zero, from the same module.
func CreateColFn(comp *wizard.CompiledIOP, rootName string, size int, withPragmas pragmas.Pragma) func(name string, args ...interface{}) ifaces.Column {

	return func(name string, args ...interface{}) ifaces.Column {
		s := []string{rootName, name}
		v := strings.Join(s, "_")
		col := comp.InsertCommit(0, ifaces.ColIDf(v, args...), size)

		switch withPragmas {
		case pragmas.None:
		case pragmas.LeftPadded:
			pragmas.MarkLeftPadded(col)
		case pragmas.RightPadded:
			pragmas.MarkRightPadded(col)
		case pragmas.FullColumnPragma:
			pragmas.MarkFullColumn(col)
		}

		return col
	}

}
