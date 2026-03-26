package common

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
)

// VectorBuilder is a convenience structure to assign columns by appending
// field elements on top of field elements.
type VectorBuilder struct {

	// column is the column being assigned by the current builder.
	column ifaces.Column

	// slice is the current stage of the field element
	slice []field.Element
}

// NewVectorBuilder initializes a new VectorBuilder with a new column.
func NewVectorBuilder(col ifaces.Column) *VectorBuilder {
	return &VectorBuilder{
		column: col,
		// The size is always divided by 16 because 99% of the time this
		// will be more than needed. It make sense to reduce it.
		slice: make([]field.Element, 0, col.Size()/16),
	}
}

// Resize resizes the slice up to `newLen`. Panics if the newLen is smaller
// than the current length.
func (vb *VectorBuilder) Resize(newLen int) {
	if len(vb.slice) > newLen {
		utils.Panic("the old length %v is larger than the newLen %v", len(vb.slice), newLen)
	}

	vb.slice = append(vb.slice, make([]field.Element, newLen-len(vb.slice))...)
}

// PushBoolean pushes one for bo=true and zero for bo=false.
func (vb *VectorBuilder) PushBoolean(bo bool) {
	if bo {
		vb.PushOne()
	} else {
		vb.PushZero()
	}
}

// PushZero pushes 0 onto `vb`
func (vb *VectorBuilder) PushZero() {
	vb.slice = append(vb.slice, field.Zero())
}

// PushOne pushes 1 onto `vb`
func (vb *VectorBuilder) PushOne() {
	vb.slice = append(vb.slice, field.One())
}

// PushField pushes `f` onto `vb`
func (vb *VectorBuilder) PushField(f field.Element) {
	vb.slice = append(vb.slice, f)
}

// PushInt pushes `x` onto `vb`.
func (vb *VectorBuilder) PushInt(x int) {
	f := field.NewElement(uint64(x))
	vb.PushField(f)
}

// PushHi pushes the 16 first bytes of `fb` onto `vb`.
func (vb *VectorBuilder) PushHi(fb types.FullBytes32) {
	var f field.Element
	f.SetBytes(fb[:16])
	vb.PushField(f)
}

// PushLo pushes the 16 last bytes of `fb` onto `vb“.
func (vb *VectorBuilder) PushLo(fb types.FullBytes32) {
	var f field.Element
	f.SetBytes(fb[16:])
	vb.PushField(f)
}

// PushBytes32 pushes a [types.Bytes32] as a single value onto `vb`. It panics
// if the value overflows a field element.
func (vb *VectorBuilder) PushBytes32(b32 types.Bytes32) {
	var f field.Element
	if err := f.SetBytesCanonical(b32[:]); err != nil {
		panic(err)
	}
	vb.PushField(f)
}

// PushBytes32 pushes a [types.Bytes32] as a single value onto `vb`. It panics
// if the value overflows a field element.
func (vb *VectorBuilder) PushBytes(b32 []byte) {
	var f field.Element
	if err := f.SetBytesCanonical(b32[:]); err != nil {
		panic(err)
	}
	vb.PushField(f)
}

// Pop removes the last pushed value of the column.
func (vb *VectorBuilder) Pop() {
	vb.slice = vb.slice[:len(vb.slice)-1]
}

// RepushLast pushes a value equal to the last pushed value of `vb`
func (vb *VectorBuilder) RepushLast() {
	if len(vb.slice) == 0 {
		panic("attempted to repush the last item of an empty builder")
	}
	last := vb.slice[len(vb.slice)-1]
	vb.PushField(last)
}

// PushInc repushes that last element incremented by 1
func (vb *VectorBuilder) PushInc() {
	var (
		one  = field.One()
		last = vb.slice[len(vb.slice)-1]
	)

	last.Add(&last, &one)
	vb.PushField(last)
}

// PushInc repushes that last element incremented by 1
func (vb *VectorBuilder) PushIncBy(by int) {

	if by < 0 {
		utils.Panic("by was negative (%v)", by)
	}

	var (
		byF  = field.NewElement(uint64(by))
		last = vb.slice[len(vb.slice)-1]
	)

	last.Add(&last, &byF)
	vb.PushField(last)
}

// PadAndAssign pads and assign the column built by `vb` using `v` as padding
// value and assigning into `run`.
func (vb *VectorBuilder) PadAndAssign(run *wizard.ProverRuntime, v ...field.Element) {

	if len(vb.slice) > vb.column.Size() {
		// We print the stack to help debugging
		exit.OnLimitOverflow(
			vb.column.Size(),
			len(vb.slice),
			fmt.Errorf("the slice size %v is larger than the column size %v", len(vb.slice), vb.column.Size()),
		)
	}

	paddingValue := field.Zero()
	if len(v) > 0 {
		paddingValue = v[0]
	}

	run.AssignColumn(
		vb.column.GetColID(),
		smartvectors.RightPadded(vb.slice, paddingValue, vb.column.Size()),
	)
}

// Height returns the total number of elements that have been pushed on this
// builder.
func (vb *VectorBuilder) Height() int {
	return len(vb.slice)
}

// Slice return the slice field of the VectorBuilder
func (vb *VectorBuilder) Slice() []field.Element {
	return vb.slice
}

// it pushes a slice of field Element
func (vb *VectorBuilder) PushSliceF(s []field.Element) {
	vb.slice = append(vb.slice, s...)
}

// it overwrites the last push
func (vb *VectorBuilder) OverWriteInt(n int) {
	vb.slice[len(vb.slice)-1] = field.NewElement(uint64(n))
}

// Last returns the last inserted value. Will panic if the vector is empty.
// Does not mutate the receiver.
func (vb *VectorBuilder) Last() field.Element {
	return vb.slice[len(vb.slice)-1]
}
