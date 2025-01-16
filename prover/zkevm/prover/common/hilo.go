package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// HiLoColumns represents a pair of column representing a sequence of bytes32
// element that do not fit in a single field element. The Hi columns stores the
// first 16 bytes of the column. And the Lo columns stores the last 16 bytes.
type HiLoColumns struct {
	Hi, Lo ifaces.Column
}

// NewHiLoColumns returns a new HiLoColumns with initialized and unconstrained
// columns.
func NewHiLoColumns(comp *wizard.CompiledIOP, size int, name string) HiLoColumns {
	return HiLoColumns{
		Hi: comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_HI", name),
			size,
		),
		Lo: comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_LO", name),
			size,
		),
	}
}

// HiLoAssignmentBuilder is a convenience structure storing the column builders
// relating to an HiLoColumns.
type HiLoAssignmentBuilder struct {
	Hi, Lo *VectorBuilder
}

// NewHiLoAssignmentBuilder returns a fresh [hiLoAssignmentBuilder]
func NewHiLoAssignmentBuilder(hiLo HiLoColumns) HiLoAssignmentBuilder {
	return HiLoAssignmentBuilder{
		Hi: NewVectorBuilder(hiLo.Hi),
		Lo: NewVectorBuilder(hiLo.Lo),
	}
}

// Push pushes a row representing `fb` onto `hl`
func (hl *HiLoAssignmentBuilder) Push(fb types.FullBytes32) {
	hl.Hi.PushHi(fb)
	hl.Lo.PushLo(fb)
}

// PushZeroes pushes a row representing 0 onto `hl`
func (hl *HiLoAssignmentBuilder) PushZeroes() {
	hl.Hi.PushZero()
	hl.Lo.PushZero()
}

// PadAssign pads `hl` with `fb` and assigns the resulting columns into `run`.
func (hl *HiLoAssignmentBuilder) PadAssign(run *wizard.ProverRuntime, fb types.FullBytes32) {
	var f field.Element
	f.SetBytes(fb[:16])
	hl.Hi.PadAndAssign(run, f)
	f.SetBytes(fb[16:])
	hl.Lo.PadAndAssign(run, f)
}
