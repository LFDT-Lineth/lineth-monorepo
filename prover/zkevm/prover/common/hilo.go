package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// HiLoColumns represents a pair of column representing a sequence of bytes32
// element that do not fit in a single field element. The Hi columns stores the
// first 16 bytes of the column. And the Lo columns stores the last 16 bytes.
type HiLoColumns struct {
	Hi, Lo [NbLimbU128]ifaces.Column
}

// NewHiLoColumns returns a new HiLoColumns with initialized and unconstrained
// columns.
func NewHiLoColumns(comp *wizard.CompiledIOP, size int, name string) HiLoColumns {
	res := HiLoColumns{}
	for i := range NbLimbU128 {
		res.Hi[i] = comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_HI_%v", name, i),
			size,
			true,
		)

		res.Lo[i] = comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_LO_%v", name, i),
			size,
			true,
		)
	}

	return res
}

// HiLoAssignmentBuilder is a convenience structure storing the column builders
// relating to an HiLoColumns.
type HiLoAssignmentBuilder struct {
	Hi, Lo [NbLimbU128]*VectorBuilder
}

// NewHiLoAssignmentBuilder returns a fresh [hiLoAssignmentBuilder]
func NewHiLoAssignmentBuilder(hiLo HiLoColumns) HiLoAssignmentBuilder {
	res := HiLoAssignmentBuilder{}

	for i := range NbLimbU128 {
		res.Hi[i] = NewVectorBuilder(hiLo.Hi[i])
		res.Lo[i] = NewVectorBuilder(hiLo.Lo[i])
	}

	return res
}

// Push pushes a row representing `fb` onto `hl`
func (hl *HiLoAssignmentBuilder) Push(fb [NbLimbU256][]byte) {
	for i := range NbLimbU128 {
		hiBytes := LeftPadToFrBytes(fb[i])
		hl.Hi[i].PushBytes(hiBytes)
		loBytes := LeftPadToFrBytes(fb[NbLimbU128+i])
		hl.Lo[i].PushBytes(loBytes)
	}
}

// PushZeroes pushes a row representing 0 onto `hl`
func (hl *HiLoAssignmentBuilder) PushZeroes() {
	for i := range NbLimbU128 {
		hl.Hi[i].PushZero()
		hl.Lo[i].PushZero()
	}
}

// PadAssign pads `hl` with `fb` and assigns the resulting columns into `run`.
func (hl *HiLoAssignmentBuilder) PadAssign(run *wizard.ProverRuntime, fb [NbLimbU256][]byte) {
	for i := range NbLimbU128 {
		var f field.Element
		f.SetBytes(fb[i])
		hl.Hi[i].PadAndAssign(run, f)
		f.SetBytes(fb[NbLimbU128+i])
		hl.Lo[+i].PadAndAssign(run, f)
	}
}
