package horner

import (
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// CompileProjection compiles [query.Projection] queries
func CompileProjection(comp *wizard.CompiledIOP) {
	ProjectionToHorner(comp)
	CompileHorner(comp)
}
