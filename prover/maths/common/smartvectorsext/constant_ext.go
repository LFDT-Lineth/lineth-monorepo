package smartvectorsext

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// A constant vector is a vector obtained by repeated "length" time the same value
type ConstantExt struct {
	val    fext.Element
	length int
}

// Construct a new "Constant" smart-vector
func NewConstantExt(val fext.Element, length int) *ConstantExt {
	if length <= 0 {
		utils.Panic("zero or negative length are not allowed")
	}
	return &ConstantExt{val: val, length: length}
}

// Return the length of the smart-vector
func (c *ConstantExt) Len() int { return c.length }

// Returns an entry of the constant
func (c *ConstantExt) GetBase(int) (field.Element, error) {
	return field.Zero(), fmt.Errorf(conversionError)
}

func (c *ConstantExt) GetExt(int) fext.Element { return c.val }

func (r *ConstantExt) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// Returns a subvector
func (c *ConstantExt) SubVector(start, stop int) smartvectors.SmartVector {
	if start > stop {
		utils.Panic("negative length are not allowed")
	}
	if start == stop {
		utils.Panic("zero length are not allowed")
	}
	assertCorrectBound(start, c.length)
	// The +1 is because we accept if "stop = length"
	assertCorrectBound(stop, c.length+1)
	return NewConstantExt(c.val, stop-start)
}

// Returns a rotated version of the slice
func (c *ConstantExt) RotateRight(int) smartvectors.SmartVector {
	return NewConstantExt(c.val, c.length)
}

// Write the constant vector in a slice
func (c *ConstantExt) WriteInSlice(s []field.Element) {
	panic(conversionError)
}

func (c *ConstantExt) WriteInSliceExt(s []fext.Element) {
	for i := 0; i < len(s); i++ {
		s[i].Set(&c.val)
	}
}

func (c *ConstantExt) Val() fext.Element {
	return c.val
}

func (c *ConstantExt) Pretty() string {
	return fmt.Sprintf("Constant[%v;%v]", c.val.String(), c.length)
}

func (c *ConstantExt) DeepCopy() smartvectors.SmartVector {
	return NewConstantExt(c.val, c.length)
}

func (c *ConstantExt) IntoRegVecSaveAlloc() []field.Element {
	panic(conversionError)
}

func (c *ConstantExt) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return nil, fmt.Errorf(conversionError)
}

func (c *ConstantExt) IntoRegVecSaveAllocExt() []fext.Element {
	res := smartvectors.IntoRegVecExt(c)
	return res
}
