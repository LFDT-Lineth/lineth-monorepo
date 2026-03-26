package smartvectors

import (
	"fmt"
	"iter"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// A constant vector is a vector obtained by repeated "length" time the same value
type ConstantExt struct {
	Value  fext.Element
	Length int
}

// Construct a new "Constant" smart-vector
func NewConstantExt(val fext.Element, length int) *ConstantExt {
	if length <= 0 {
		utils.Panic("zero or negative length are not allowed")
	}
	return &ConstantExt{Value: val, Length: length}
}

// Return the length of the smart-vector
func (c *ConstantExt) Len() int { return c.Length }

// Returns an entry of the constant
func (c *ConstantExt) GetBase(int) (field.Element, error) {
	return field.Zero(), errConversion
}

func (c *ConstantExt) GetExt(int) fext.Element { return c.Value }

func (r *ConstantExt) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// Returns a subvector
func (c *ConstantExt) SubVector(start, stop int) SmartVector {
	if start > stop {
		utils.Panic("negative length are not allowed")
	}
	if start == stop {
		utils.Panic("zero length are not allowed")
	}
	assertCorrectBound(start, c.Length)
	// The +1 is because we accept if "stop = length"
	assertCorrectBound(stop, c.Length+1)
	return NewConstantExt(c.Value, stop-start)
}

// Returns a rotated version of the slice
func (c *ConstantExt) RotateRight(int) SmartVector {
	return NewConstantExt(c.Value, c.Length)
}

// Write the constant vector in a slice
func (c *ConstantExt) WriteInSlice(s []field.Element) {
	panic(errConversion)
}

func (c *ConstantExt) WriteInSliceExt(s []fext.Element) {
	for i := 0; i < len(s); i++ {
		s[i].Set(&c.Value)
	}
}

func (c *ConstantExt) Val() fext.Element {
	return c.Value
}

func (c *ConstantExt) Pretty() string {
	return fmt.Sprintf("Constant[%v;%v]", c.Value.String(), c.Length)
}

func (c *ConstantExt) DeepCopy() SmartVector {
	return NewConstantExt(c.Value, c.Length)
}

func (c *ConstantExt) IntoRegVecSaveAlloc() []field.Element {
	panic(errConversion)
}

func (c *ConstantExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, errConversion
}

func (c *ConstantExt) IntoRegVecSaveAllocExt() []fext.Element {
	res := IntoRegVecExt(c)
	return res
}

func (c *ConstantExt) IterateCompact() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *ConstantExt) IterateSkipPadding() iter.Seq[field.Element] {
	panic("not available for extensions")
}

func (c *ConstantExt) GetPtr(n int) *field.Element {
	panic("not available for extensions")
}
