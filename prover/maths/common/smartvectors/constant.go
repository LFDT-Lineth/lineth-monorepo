package smartvectors

import (
	"fmt"
	"iter"
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// A constant vector is a vector obtained by repeated "length" time the same value
type Constant struct {
	Value  field.Element
	Length int
}

// Construct a new "Constant" smart-vector
func NewConstant(val field.Element, length int) *Constant {
	if length <= 0 {
		utils.Panic("zero or negative length are not allowed")
	}
	return &Constant{Value: val, Length: length}
}

// Return the length of the smart-vector
func (c *Constant) Len() int { return c.Length }

// Returns an entry of the constant
func (c *Constant) GetBase(int) (field.Element, error) { return c.Value, nil }

func (c *Constant) GetExt(int) fext.Element {
	var res fext.Element
	fext.SetFromBase(&res, &c.Value)
	return res
}
func (r *Constant) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

func (r *Constant) GetPtr(n int) *field.Element {
	return &r.Value
}

// Returns a subvector
func (c *Constant) SubVector(start, stop int) SmartVector {
	if start > stop {
		utils.Panic("negative length are not allowed")
	}
	if start == stop {
		utils.Panic("zero length are not allowed")
	}
	assertCorrectBound(start, c.Length)
	// The +1 is because we accept if "Stop = length"
	assertCorrectBound(stop, c.Length+1)
	return NewConstant(c.Value, stop-start)
}

// Returns a rotated version of the slice
func (c *Constant) RotateRight(int) SmartVector {
	return NewConstant(c.Value, c.Length)
}

// Write the constant vector in a slice
func (c *Constant) WriteInSlice(s []field.Element) {
	assertHasLength(len(s), c.Len())
	for i := range s {
		s[i] = c.Value
	}
}

func (c *Constant) WriteInSliceExt(s []fext.Element) {
	for i := 0; i < len(s); i++ {
		fext.SetFromBase(&s[i], &c.Value)
	}
}

func (c *Constant) Val() field.Element {
	return c.Value
}

func (c *Constant) Pretty() string {
	return fmt.Sprintf("Constant[%v;%v]", c.Value.String(), c.Length)
}

func (c *Constant) DeepCopy() SmartVector {
	return NewConstant(c.Value, c.Length)
}

func (c *Constant) IntoRegVecSaveAlloc() []field.Element {
	res, err := c.IntoRegVecSaveAllocBase()
	if err != nil {
		panic(errConversion)
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
		fext.SetFromBase(&res[i], &elem)
	}
	return res
}

// IterateCompact returns an iterator returning a single time the constant
// value.
func (c *Constant) IterateCompact() iter.Seq[field.Element] {
	return slices.Values([]field.Element{c.Value})
}

// IterateSkipPadding returns an empty iterator as the whole content of a
// [Constant] is padding.
func (c *Constant) IterateSkipPadding() iter.Seq[field.Element] {
	return slices.Values([]field.Element{})
}
