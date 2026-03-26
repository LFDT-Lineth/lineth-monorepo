package limbs

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// VectorBuilder is a helper to iteratively construct the assignment of a column
// by iteratively pushing values and then pad.
type VectorBuilder[E Endianness] struct {
	limbs  Limbs[E]
	slices [][]field.Element
}

// NewVectorBuilder creates a new vector builder for the provided limbs.
func NewVectorBuilder[E Endianness](l Limbs[E]) *VectorBuilder[E] {
	return &VectorBuilder[E]{
		limbs:  l,
		slices: make([][]field.Element, l.NumLimbs()),
	}
}

// Height returns the total number of rows that have been pushed
func (b *VectorBuilder[E]) Height() int {
	return len(b.slices[0])
}

// Push adds a new row to the builder. This does not check for the endianness.
func (b *VectorBuilder[E]) Push(row row[E]) {
	b.pushRaw(row.T)
}

// pushRaw pushes without checking the endianness
func (b *VectorBuilder[E]) pushRaw(row []field.Element) {
	if len(row) != b.limbs.NumLimbs() {
		utils.Panic("wrong number of columns %v != %v", len(row), b.limbs.NumLimbs())
	}
	for i := range row {
		b.slices[i] = append(b.slices[i], row[i])
	}
}

// PushRepeatBytes repeatedly push bytes to the builder
func (b *VectorBuilder[E]) PushRepeatBytes(x []byte, n int) {
	for i := 0; i < n; i++ {
		b.PushBytes(x)
	}
}

// PushBigInt pushes a new big.Int to the builder
func (b *VectorBuilder[E]) PushBigInt(x *big.Int) {
	row := bigIntToLimbs[E](x, b.limbs.BitSize())
	b.pushRaw(row)
}

// PushBytes pushes a new bytes to the builder
func (b *VectorBuilder[E]) PushBytes(x []byte) {
	row := bytesToLimbs[E](x)
	b.pushRaw(row)
}

// PushLeftPaddedBytes pushes a new left-zero-padded slice of byte to the
// builder. The function will panic if the number of bytes is greater than the
// limbs capacity.
func (b *VectorBuilder[E]) PushLeftPaddedBytes(x []byte) {
	capa := b.limbs.NumLimbs() * limbByteWidth
	if len(x) > capa {
		utils.Panic("wrong number of bytes %v > %v", len(x), b.limbs.NumLimbs())
	}
	nbZeroToAdd := capa - len(x)
	x = append(make([]byte, nbZeroToAdd), x...)
	b.PushBytes(x)
}

// PushBytes16 pushes a new bytes to the builder
func (b *VectorBuilder[E]) PushBytes16(x [16]byte) {
	b.PushBytes(x[:])
}

// PushZero pushes a new zero to the builder
func (b *VectorBuilder[E]) PushZero() {
	row := make([]field.Element, b.limbs.NumLimbs())
	b.pushRaw(row)
}

// PushSeqOfZeroes pushes a sequence of zeroes to the builder
func (b *VectorBuilder[E]) PushSeqOfZeroes(n int) {
	for i := 0; i < n; i++ {
		b.PushZero()
	}
}

// PushOne pushes a row storing the big integer one, respecting the endianness
// of the limbs.
func (b *VectorBuilder[E]) PushOne() {
	row := bigIntToLimbs[E](big.NewInt(1), b.limbs.BitSize())
	b.pushRaw(row)
}

// PushInt pushes a small integer to the builder. It will spread it into limbs
// if needed.
func (b *VectorBuilder[E]) PushInt(x int) {
	row := bigIntToLimbs[E](big.NewInt(int64(x)), b.limbs.BitSize())
	b.pushRaw(row)
}

// PeekAt returns the last pushed row in native form.
func (b *VectorBuilder[E]) PeekAt(r int) row[E] {
	rowF := make([]field.Element, len(b.slices))
	for i := range rowF {
		rowF[i] = b.slices[i][r]
	}
	return row[E]{T: rowF}
}

// PeekBigIntAt returns the last pushed row in big.Int form.
func (b *VectorBuilder[E]) PeekBigIntAt(r int) *big.Int {
	return limbToBigInt[E](b.PeekAt(r).T)
}

// PeekBytesAt returns the last pushed row in bytes form.
func (b *VectorBuilder[E]) PeekBytesAt(r int) []byte {
	return limbsToBytes[E](b.PeekAt(r).T)
}

// PeekLast returns the last pushed row in native form.
func (b *VectorBuilder[E]) PeekLast() row[E] {
	return b.PeekAt(len(b.slices[0]) - 1)
}

// RepushLast pushes a value equal to the last pushed value of `vb`
func (b *VectorBuilder[E]) RepushLast() {
	if len(b.slices[0]) == 0 {
		panic("attempted to repush the last item of an empty builder")
	}
	last := b.PeekAt(len(b.slices[0]) - 1)
	b.Push(last)
}

// PushInc repushes that last element incremented by 1
func (b *VectorBuilder[E]) PushInc() {
	last := b.PeekBigIntAt(len(b.slices[0]) - 1)
	last.Add(last, big.NewInt(1))
	b.PushBigInt(last)
}

// PadAndAssignZero assigns the content of the builder to the column.
func (b *VectorBuilder[E]) PadAndAssignZero(run *wizard.ProverRuntime) {
	for i := range b.slices {
		run.AssignColumn(
			b.limbs.C[i].GetColID(),
			smartvectors.RightZeroPadded(b.slices[i], b.limbs.C[i].Size()),
		)
	}
}

// PadLeftAndAssign zeroe-pads the vector to the left with zeroes and assigns it
// to the column.
func (b *VectorBuilder[E]) PadLeftAndAssignZero(run *wizard.ProverRuntime) {
	for i := range b.slices {
		run.AssignColumn(
			b.limbs.C[i].GetColID(),
			smartvectors.LeftZeroPadded(b.slices[i], b.limbs.C[i].Size()),
		)
	}
}
