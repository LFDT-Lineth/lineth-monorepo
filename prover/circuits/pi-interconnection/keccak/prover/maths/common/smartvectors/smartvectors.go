package smartvectors

import (
	"errors"
	"fmt"
	"iter"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/exit"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

const conversionError = "smartvector holds field extensions, but a base element was requested"

// SmartVector is an abstraction over vectors of field elements that can be
// optimized for structured vectors. For instance, if we have a vector of
// repeated elements we can use smartvectors.NewConstant(x, n) to represent it.
// This way instead of using n * sizeof(field.Element) memory it will only store
// the element once. Additionally, every operation performed on it will be
// sped up with dedicated algorithms.
//
// There are a few precautions to take when implementing or using smart-vectors
//   - constructing a zero-length smart-vector should be considered illegal. The
//     reason for such a restriction is tha t
//   - although the smart-vectors are not immutable, the user should refrain
//     mutating them after they are created as this may have unintended side
//     effects that are hard to track.
type SmartVector interface {
	// Len returns the length of the SmartVector
	Len() int
	// Get returns an entry of the SmartVector at particular position
	GetBase(int) (field.Element, error)
	Get(int) field.Element
	GetExt(int) fext.Element
	// SubVector returns a subvector of the [SmartVector]. It mirrors slice[Start:Stop]
	SubVector(int, int) SmartVector
	// RotateRight cyclically rotates the SmartVector
	RotateRight(int) SmartVector
	// WriteInSlice writes the SmartVector into a slice. The slice must be just
	// as large as [Len] otherwise the function will panic
	WriteInSlice([]field.Element)
	WriteInSliceExt([]fext.Element)
	// Pretty returns a prettified version of the vector, useful for debugging.
	Pretty() string
	// DeepCopy returns a deep-copy of the SmartVector which can be freely
	// mutated without affecting the
	DeepCopy() SmartVector
	// IntoRegVecSaveAlloc converts a smart-vector into a normal vec. The
	// implementation minimizes then number of copies
	IntoRegVecSaveAlloc() []field.Element
	IntoRegVecSaveAllocBase() ([]field.Element, error)
	IntoRegVecSaveAllocExt() []fext.Element

	// IterateCompact returns an iterator over the elements of the smartvectors,
	// the iterator will iterate depending on the type of smartvectors:
	//
	//   - Regular, Regular, Rotated: will iterate over the elements of the
	//     vector as it would be expected by a normal iterator.
	//   - Constant: the iterator will only return a single value.
	//   - PaddedCircularWindow (left-padded): the iterator will first return
	//     the filling value and then the elements of the windows.
	//   - PaddedCircularWindow (right-padded): the iterator will first return
	//     the elements of the window and then the filling value
	//   - PaddedCircularWindow (bi-directionally-padded): the iterator will return
	//     (1) one element for the padding value, (2) the elements of the window
	//     and (3) one element for the padding value again
	IterateCompact() iter.Seq[field.Element]

	// IterateSkipPadding iterates over the non-padding area or a smart-vector
	//
	// - Constant: returns an empty iterator
	// - PaddedCircularWindow: returns an iterator over the windows
	// - Regular and others: returns an iterator over whole vector
	IterateSkipPadding() iter.Seq[field.Element]

	// GetPtr returns a pointer to the element at position n. Please do not
	// mutate the result as this can have unpredictable side-effects.
	GetPtr(int) *field.Element
}

// AllocateRegular returns a newly allocated smart-vector
func AllocateRegular(n int) SmartVector {
	return NewRegular(make([]field.Element, n))
}

// Copy into a smart-vector, will panic if into is not a regular
// Mainly used as a sugar for refactoring
func Copy(into *SmartVector, x SmartVector) {
	*into = x.DeepCopy()
}

// Rand creates a vector with random entries. Used for testing. Should not be
// used to generate secrets. Not reproducible.
func Rand(n int) SmartVector {
	v := vector.Rand(n)
	return NewRegular(v)
}

// Rand creates a vector with random entries. Used for testing. Should not be
// used to generate secrets. Takes a math.Rand as input for reproducibility
// math
func PseudoRand(rng *rand.Rand, n int) SmartVector {
	return NewRegular(vector.PseudoRand(rng, n))
}

// ForTest returns a witness from a explicit litteral assignement
func ForTest(xs ...int) SmartVector {
	return NewRegular(vector.ForTest(xs...))
}

// IntoRegVec converts a smart-vector into a normal vec. The resulting vector
// is always reallocated and can be safely mutated without side-effects
// on s.
func IntoRegVec(s SmartVector) []field.Element {
	res := make([]field.Element, s.Len())
	s.WriteInSlice(res)
	return res
}

func IntoRegVecExt(s SmartVector) []fext.Element {
	res := make([]fext.Element, s.Len())
	s.WriteInSliceExt(res)
	return res
}

// IntoGnarkAssignment converts a smart-vector into a gnark assignment
func IntoGnarkAssignment(sv SmartVector) []frontend.Variable {
	res := make([]frontend.Variable, sv.Len())
	_, err := sv.GetBase(0)
	if err == nil {
		for i := range res {
			elem, _ := sv.GetBase(i)
			res[i] = elem
		}
	} else {
		for i := range res {
			elem := sv.GetExt(i)
			res[i] = elem
		}
	}
	return res
}

// LeftPadded creates a new padded vector (padded on the left)
func LeftPadded(v []field.Element, padding field.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		exit.OnLimitOverflow(
			targetLen,
			len(v),
			fmt.Errorf("unpadded vector (length=%v) must be smaller than the target length (%v)", len(v), targetLen),
		)
	}

	if len(v) == targetLen {
		return NewRegular(v)
	}

	if len(v) == 0 {
		return NewConstant(padding, targetLen)
	}

	return NewPaddedCircularWindow(v, padding, targetLen-len(v), targetLen)
}

// RightPadded creates a new vector (padded on the right)
func RightPadded(v []field.Element, padding field.Element, targetLen int) SmartVector {

	if len(v) > targetLen {
		exit.OnLimitOverflow(
			targetLen,
			len(v),
			fmt.Errorf("unpadded vector (length=%v) must be smaller than the target length (%v)", len(v), targetLen),
		)
	}

	if len(v) == targetLen {
		return NewRegular(v)
	}

	if len(v) == 0 {
		return NewConstant(padding, targetLen)
	}

	return NewPaddedCircularWindow(v, padding, 0, targetLen)
}

// RightZeroPadded creates a new vector (padded on the right)
func RightZeroPadded(v []field.Element, targetLen int) SmartVector {
	return RightPadded(v, field.Zero(), targetLen)
}

// LeftZeroPadded creates a new vector (padded on the left)
func LeftZeroPadded(v []field.Element, targetLen int) SmartVector {
	return LeftPadded(v, field.Zero(), targetLen)
}

// Density returns the density of a smart-vector. By density we mean the size
// of the concrete underlying vectors. This can be used as a proxi for the
// memory required to store the smart-vector.
func Density(v SmartVector) int {
	switch w := v.(type) {
	case *Constant:
		return 0
	case *PaddedCircularWindow:
		return len(w.Window_)
	case *Regular:
		return len(*w)
	case *Rotated:
		return len(w.v.Regular)
	case *Pooled:
		return len(w.Regular)
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// PaddingOrientationOf returns an integer indicating the orientation of the
// padding of a column. '0' indicates an unresolved orientation. '1' indicates
// that the columns if right-padded and '-1' indicates that it is left-padded.
//
// The function returns an error if the vector is not a padded-circular window.
func PaddingOrientationOf(v SmartVector) (int, error) {

	switch w := v.(type) {
	case *PaddedCircularWindow:
		if w.Offset_ == 0 {
			return 1, nil
		}
		if w.Offset_+len(w.Window_) == w.TotLen_ {
			return -1, nil
		}
	default:
		return 0, errors.New("vector is not a padded-circular window")
	}

	return 0, nil
}

// Window returns the effective window of the vector,
// if the vector is Padded with zeroes it return the window.
// Namely, the part without zero pads.
func Window(v SmartVector) []field.Element {
	res, err := WindowBase(v)
	if err != nil {
		panic(conversionError)
	}
	return res
}

func WindowBase(v SmartVector) ([]field.Element, error) {
	switch w := v.(type) {
	case *Constant:
		return []field.Element{}, nil
	case *PaddedCircularWindow:
		return w.Window_, nil
	case *Regular:
		return *w, nil
	case *Rotated:
		return w.IntoRegVecSaveAllocBase()
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func WindowExt(v SmartVector) []fext.Element {
	switch w := v.(type) {
	case *Constant:
		return []fext.Element{}
	case *PaddedCircularWindow:
		temp := make([]fext.Element, len(w.Window_))
		for i := 0; i < len(w.Window_); i++ {
			elem := w.Window_[i]
			temp[i].SetFromBase(&elem)
		}
		return temp
	case *Regular:
		temp := make([]fext.Element, len(*w))
		for i := 0; i < len(*w); i++ {
			elem, _ := w.GetBase(i)
			temp[i].SetFromBase(&elem)
		}
		return temp
	case *Rotated:
		return w.IntoRegVecSaveAllocExt()
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// PaddingVal returns either the constant value of the smart-vector
// if it is a constant or the padding value of the padded circular window
// smart-vector. Otherwise, it returns zero. The function also returns
// a flag indicating if the value has padding.
func PaddingVal(v SmartVector) (val field.Element, hasPadding bool) {
	switch w := v.(type) {
	case *Constant:
		return w.Value, true
	case *PaddedCircularWindow:
		return w.PaddingVal_, true
	default:
		return field.Element{}, false
	}
}

// TryReduceSizeRight detects if the input smart-vector can be reduced to a constant
// smart-vector. It will only apply over the following types: [Regular].
func TryReduceSizeRight(v SmartVector) (new SmartVector, totalSaving int) {

	switch w := v.(type) {
	case *Constant, *Rotated, *Pooled, *PaddedCircularWindow:
		return v, 0
	case *Regular:

		if res, ok := tryIntoConstant(*w); ok {
			return res, len(*w)
		}

		if res, ok := tryIntoRightPadded(*w); ok {
			return res, len(*w) - len(res.Window_)
		}

		return v, 0

	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// TryReduceSizeLeft detects if the input smart-vector can be reduced to a
// left-padded smart-vector. It will only apply over the following types:
// [Regular].
func TryReduceSizeLeft(v SmartVector) (new SmartVector, totalSaving int) {

	switch w := v.(type) {
	case *Constant, *Rotated, *Pooled, *PaddedCircularWindow:
		return v, 0
	case *Regular:

		if res, ok := tryIntoConstant(*w); ok {
			return res, len(*w)
		}

		if res, ok := tryIntoLeftPadded(*w); ok {
			return res, len(*w) - len(res.Window_)
		}

		return v, 0

	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// tryIntoConstant attemps to rewrite the smart-vector into a constant smart-vector.
func tryIntoConstant(w Regular) (*Constant, bool) {

	// to detect if a regular vector can be reduced to a constant, we need to
	// check if all the values are equals. That's an expensive, so we instead
	// by comparing values that would be likely to be unequal if it was not a
	// constant. Also, we need to rule out the case where len(*w) because it
	// is irrelevant to reducing the size.
	if len(w) <= 1 {
		return nil, false
	}

	if w[0] != w[1] {
		return nil, false
	}

	if w[0] != w[len(w)-1] {
		return nil, false
	}

	if w[0] != w[len(w)/2] {
		return nil, false
	}

	// This is expensive check where we check all the values in the vector
	// to see if they are all equal. This is not the most efficient way to
	// detect if a vector is a constant but the only reliable one.
	for i := range w {
		if w[i] != w[0] {
			return nil, false
		}
	}

	return NewConstant(w[0], len(w)), true
}

// tryIntoRightPadded scans the smartvector and attempts to rewrite it into a
// a more space-efficient right padded circular windows.
func tryIntoRightPadded(v Regular) (*PaddedCircularWindow, bool) {

	var (
		bestPos = len(v) - 1
		last    = v[len(v)-1]
	)

	for i := len(v) - 2; i >= 0; i-- {
		if v[i] != last {
			break
		}
		bestPos = i
	}

	if bestPos == len(v)-1 {
		return nil, false
	}

	if bestPos == 0 {
		utils.Panic("passed a constant vector to tryIntoRightPadded, it should have been handled by tryIntoConstant")
	}

	return RightPadded(v[:bestPos], last, len(v)).(*PaddedCircularWindow), true
}

// tryIntoLeftPadded scans the smartvector and attempts to rewrite it into a
// a more space-efficient left padded circular windows.
func tryIntoLeftPadded(v Regular) (*PaddedCircularWindow, bool) {

	var (
		bestPos = 0
		first   = v[0]
	)

	for i := 1; i < len(v); i++ {
		if v[i] != first {
			break
		}
		bestPos = i
	}

	if bestPos == 0 {
		return nil, false
	}

	if bestPos == len(v)-1 {
		utils.Panic("passed a constant vector to tryIntoLeftPadded, it should have been handled by tryIntoConstant")
	}

	return LeftPadded(v[bestPos+1:], first, len(v)).(*PaddedCircularWindow), true
}

// FromCompactWithShape creates a new smart-vector with the same shape as the
// sameAs smart-vector and the values provided in the compact slice.
//
//   - If sameAs is left-padded, then the new smart-vector will be left-padded
//     also, it will take the first value of compact as the padding value and the
//     following one as the "plain" values. The function asserts that the window
//     of len(sameAs.window) == len(compact) - 1.
//   - If sameAs is right-padded, then the new smart-vector will be right-padded
//     also, it will take the last value of compact as the padding value and the
//     previous one as the "plain" values. The function asserts that the window
//     of len(sameAs.window) == len(compact) - 1.
//   - If sameAs is constant, then the new smart-vector will be constant as well
//     and the function asserts that compact has the length 1.
//   - If sameAs is regular, then the new smart-vector will be regular as well
//     and have all its values from compact.
//
// In other situations, the function will panic.
func FromCompactWithShape(v SmartVector, compact []field.Element) SmartVector {
	switch w := v.(type) {
	case *Constant:
		return NewConstant(compact[0], v.Len())
	case *PaddedCircularWindow:
		// right-padded
		if w.Offset_ == 0 {

			if len(w.Window_) != len(compact)-1 {
				panic("unexpected shape for padded circular window")
			}

			last := compact[len(compact)-1]
			window := compact[:len(compact)-1]
			return RightPadded(window, last, w.Len())
		}

		// left-padded
		if w.Offset_+len(w.Window_) == w.TotLen_ {

			if len(w.Window_) != len(compact)-1 {
				panic("unexpected shape for padded circular window")
			}

			first := compact[0]
			window := compact[1:]
			return LeftPadded(window, first, w.Len())
		}

		panic("unexpected shape for padded circular window")
	case *Regular:

		if len(*w) != len(compact) {
			panic("unexpected shape for regular vector")
		}

		return NewRegular(compact)
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

// CoWindowRange scans the windows range of all the provided smartvectors
// and returns the largest covering one. If one input is nil, then it is discarded
// if all inputs are constants or nil, then the range is (0, 0). If one of the
// inputs is a padded circular window with wrap-around, then the range will be
// (0, totalSize).
//
// The function assumes that all provided smartvectors have the same length.
func CoWindowRange(sv ...SmartVector) (start, stop int) {

	foundAny := false

	for _, s := range sv {

		if s == nil {
			continue
		}

		switch s := s.(type) {
		default:
			return 0, s.Len()
		case *Constant:
			continue
		case *PaddedCircularWindow:

			if s.Offset()+len(s.Window()) > s.Len() {
				return 0, s.Len()
			}

			if !foundAny {
				start = s.Offset()
				stop = s.Offset() + len(s.Window())
				continue
			}

			start = min(start, s.Offset())
			stop = max(stop, s.Offset()+len(s.Window()))
			foundAny = true
		}
	}

	return start, stop
}

// CoCompactRange is as [CoWindowRange] but returns the compact range,
// meaning the same as [CoWindowRange] but adding either an extra
// position at the beginning or at the end to cover the padding value.
//
// This is compatible with [FromCompactWithShape] to conduct space and
// time efficient operations over smart-vectors.
func CoCompactRange(sv ...SmartVector) (start, stop int) {

	start, stop = CoWindowRange(sv...)

	if start > 0 {
		start--
	}

	if stop < sv[0].Len() {
		stop++
	}

	return start, stop
}

// FromCompactWithRange is as [FromCompactWithShape] but takes explicit
// start and stop positions for the range. The function expects that
// len(compact) = stop - start otherwise it will panic.
func FromCompactWithRange(compact []field.Element, start, stop, fullLen int) SmartVector {

	if stop-start > len(compact)+2 {
		utils.Panic("inconsistent compact range, stop-start != len(compact), stop=%v, start=%v, len(compact)=%v", stop, start, len(compact))
	}

	if stop-start > fullLen {
		utils.Panic("inconsistent compact range, stop-start > fullLen, stop=%v, start=%v, fullLen=%v", stop, start, fullLen)
	}

	if stop-start == 1 {
		return NewConstant(compact[0], fullLen)
	}

	if start == 0 && stop == fullLen {
		return NewRegular(compact)
	}

	if start == 0 {
		return RightPadded(compact[:len(compact)-1], compact[len(compact)-1], fullLen)
	}

	if stop == fullLen {
		return LeftPadded(compact[1:], compact[0], fullLen)
	}

	// At this point, we are dealing with a padded circular window whoses
	// window is in the middle of the vector. That means the first element
	// and the last element are the padding value, but this also means that
	// that they should be identical.
	if compact[0] != compact[len(compact)-1] {
		panic("inconsistent compact range")
	}

	return NewPaddedCircularWindow(compact[1:len(compact)-1], compact[0], start+1, fullLen)

}
