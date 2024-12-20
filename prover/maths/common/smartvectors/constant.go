package smartvectors

import (
	"fmt"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// A constant vector is a vector obtained by repeated "length" time the same value
type Constant struct {
	val    field.Element
	length int
}

// Construct a new "Constant" smart-vector
func NewConstant(val field.Element, length int) *Constant {
	if length <= 0 {
		utils.Panic("zero or negative length are not allowed")
	}
	return &Constant{val: val, length: length}
}

// Return the length of the smart-vector
func (c *Constant) Len() int { return c.length }

// Returns an entry of the constant
func (c *Constant) GetBase(int) (field.Element, error) { return c.val, nil }

func (c *Constant) GetExt(int) fext.Element { return *new(fext.Element).SetFromBase(&c.val) }

func (r *Constant) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// Returns a subvector
func (c *Constant) SubVector(start, stop int) SmartVector {
	if start > stop {
		utils.Panic("negative length are not allowed")
	}
	if start == stop {
		utils.Panic("zero length are not allowed")
	}
	assertCorrectBound(start, c.length)
	// The +1 is because we accept if "Stop = length"
	assertCorrectBound(stop, c.length+1)
	return NewConstant(c.val, stop-start)
}

// Returns a rotated version of the slice
func (c *Constant) RotateRight(int) SmartVector {
	return NewConstant(c.val, c.length)
}

// Write the constant vector in a slice
func (c *Constant) WriteInSlice(s []field.Element) {
	assertHasLength(len(s), c.Len())
	for i := range s {
		s[i] = c.val
	}
}

func (c *Constant) WriteInSliceExt(s []fext.Element) {
	for i := 0; i < len(s); i++ {
		s[i].SetFromBase(&c.val)
	}
}

func (c *Constant) Val() field.Element {
	return c.val
}

func (c *Constant) Pretty() string {
	return fmt.Sprintf("Constant[%v;%v]", c.val.String(), c.length)
}

func (c *Constant) DeepCopy() SmartVector {
	return NewConstant(c.val, c.length)
}

func (c *Constant) IntoRegVecSaveAlloc() []field.Element {
	res, err := c.IntoRegVecSaveAllocBase()
	if err != nil {
		panic(conversionError)
	}
	return res
}

// Temporary function for code transition
func (c *Constant) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return IntoRegVec(c), nil
}

func (c *Constant) IntoRegVecSaveAllocExt() []fext.Element {
	temp := IntoRegVec(c)
	res := make([]fext.Element, len(temp))
	for i := 0; i < len(temp); i++ {
		elem := temp[i]
		res[i].SetFromBase(&elem)
	}
	return res
}
