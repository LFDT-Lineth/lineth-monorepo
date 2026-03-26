package smartvectors

import (
	"fmt"
	"iter"
	"slices"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// It's a slice - zero padded up to a certain length - and rotated
type PaddedCircularWindow struct {
	Window_     []field.Element
	PaddingVal_ field.Element
	// Totlen is the length of the represented vector
	TotLen_, Offset_ int
}

// Create a new padded circular window vector
func NewPaddedCircularWindow(window []field.Element, paddingVal field.Element, offset, totLen int) *PaddedCircularWindow {
	// The window should not be larger than the total length
	if len(window) > totLen {
		utils.Panic("The window size is too large %v because totlen is %v", len(window), totLen)
	}

	if len(window) == totLen {
		utils.Panic("Forbidden : the window should not take the full length")
	}

	if len(window) == 0 {
		utils.Panic("Forbidden : empty window")
	}

	// Normalize the offset to be in range [0:totlen)
	offset = utils.PositiveMod(offset, totLen)
	return &PaddedCircularWindow{
		Window_:     window,
		PaddingVal_: paddingVal,
		Offset_:     offset,
		TotLen_:     totLen,
	}
}

// Returns the length of the vector
func (p *PaddedCircularWindow) Len() int {
	return p.TotLen_
}

// Offset returns the offset of the PCW
func (p *PaddedCircularWindow) Offset() int {
	return p.Offset_
}

// Windows returns the length of the window of the PCQ
func (p *PaddedCircularWindow) Window() []field.Element {
	return p.Window_
}

// PaddingVal returns the value used for padding the window
func (p *PaddedCircularWindow) PaddingVal() field.Element {
	return p.PaddingVal_
}

// Returns a queries position
func (p *PaddedCircularWindow) GetBase(n int) (field.Element, error) {
	// Check if the queried index is in the window
	posFromWindowsPoV := utils.PositiveMod(n-p.Offset_, p.TotLen_)
	if posFromWindowsPoV < len(p.Window_) {
		return p.Window_[posFromWindowsPoV], nil
	}
	// Else, return the padding value
	return p.PaddingVal_, nil
}

func (p *PaddedCircularWindow) GetExt(n int) fext.Element {
	elem, _ := p.GetBase(n)
	return *new(fext.Element).SetFromBase(&elem)
}

func (r *PaddedCircularWindow) Get(n int) field.Element {
	res, err := r.GetBase(n)
	if err != nil {
		panic(err)
	}
	return res
}

// Extract a subvector from p[Start:Stop), the subvector cannot "roll-over".
// i.e, we enforce that Start < Stop
func (p *PaddedCircularWindow) SubVector(start, stop int) SmartVector {
	// negative Start value is not allowed
	if start < 0 {
		panic("negative Start value is not allowed")
	}
	// Sanity checks for all subvectors
	assertCorrectBound(start, p.TotLen_)
	// The +1 is because we accept if "Stop = length"
	assertCorrectBound(stop, p.TotLen_+1)

	if start > stop {
		panic("rollover are forbidden")
	}

	if start == stop {
		panic("zero length subvector is forbidden")
	}

	/*
		This function has a high-combinatoric complexity and in order to reason about
		each case, we represent them as follows:
			[a, b) is the interval of the subvector. We can assume that [a, b) does not
			roll-over the vector.
			[c,d) is the interval of the window of `p`. It can roll-over.

		We use 'a' as the origin for other coordinates and reduce
		the ongoing combinatoric when listing all the cases. Let b_ = b-a

			0xxxxxxxxxxxxxxxxxxxxxb                      N
			|                     |                      |
		1)	|                     |   c------------d     |
		2)	|-------------------------d     c------------| (including b == d)
		2*)	c-------------------------d                  | (including b == d)
		3)	|       c-------d     |                      |
		4)	|       c------------------------d           | (including b == d)
		5)	|-----d         c----------------------------|
		6)	|-------d             |           c----------| (including c == b)
		7)	d                     |           c----------| (including c == b)


		For consistency, with the above picture, we rename as the offset coordinates
	*/

	n := p.Len()
	b := stop - start
	c := normalize(p.Interval().Start(), start, n)
	d := normalize(p.Interval().Stop(), start, n)

	// Case 1 : return a constant vector
	if b <= c && c < d {
		return NewConstant(p.PaddingVal_, b)
	}

	// Case 2 : return a regular vector
	if b <= d && d < c {
		reg := Regular(p.Window_[n-c : n-c+b])
		return &reg
	}

	// Case 2* : same as 2 but c == 0
	if b <= d && c == 0 {
		reg := Regular(p.Window_[:b])
		return &reg
	}

	// Case 3 : the window is fully contained in the subvector
	if c < d && d <= b {
		return NewPaddedCircularWindow(p.Window_, p.PaddingVal_, c, b)
	}

	// Case 4 : left-ended
	if c < b && c <= d {
		return NewPaddedCircularWindow(p.Window_[:b-c], p.PaddingVal_, c, b)
	}

	// Case 5 : the window is double ended (we skip some element in the center of the window)
	if d < c && c < b {
		left := p.Window_[:b-c]
		right := p.Window_[n-c:]

		// The deep-copy of left ensures that we do not append
		// on the same concrete slice.
		w := append(vector.DeepCopy(left), right...)
		return NewPaddedCircularWindow(w, p.PaddingVal_, c, b)
	}

	// Case 6 : right-ended
	if 0 < d && d < b && b <= c {
		return NewPaddedCircularWindow(p.Window_[n-c:], p.PaddingVal_, 0, b)
	}

	// Case 7 : d == 0 and c is out
	if d == 0 && b <= c {
		return NewConstant(p.PaddingVal_, b)
	}

	panic(fmt.Sprintf("unsupported case : b %v, c %v, d %v", b, c, d))

}

// Rotate the vector
func (p *PaddedCircularWindow) RotateRight(offset int) SmartVector {
	return NewPaddedCircularWindow(vector.DeepCopy(p.Window_), p.PaddingVal_, p.Offset_+offset, p.TotLen_)
}

func (p *PaddedCircularWindow) WriteInSlice(buff []field.Element) {
	assertHasLength(len(buff), p.TotLen_)

	for i := range p.Window_ {
		pos := utils.PositiveMod(i+p.Offset_, p.TotLen_)
		buff[pos] = p.Window_[i]
	}

	for i := len(p.Window_); i < p.TotLen_; i++ {
		pos := utils.PositiveMod(i+p.Offset_, p.TotLen_)
		buff[pos] = p.PaddingVal_
	}
}

func (p *PaddedCircularWindow) WriteInSliceExt(buff []fext.Element) {
	temp := make([]field.Element, len(buff))
	p.WriteInSlice(temp)
	for i := 0; i < len(buff); i++ {
		elem := temp[i]
		buff[i].SetFromBase(&elem)
	}

}

func (p *PaddedCircularWindow) Pretty() string {
	return fmt.Sprintf("Windowed[totlen=%v offset=%v, paddingVal=%v, window=%v]", p.TotLen_, p.Offset_, p.PaddingVal_.String(), vector.Prettify(p.Window_))
}

// IterateCompact returns an iterator over the elements of the PaddedCircularWindow
// in a "compact" way. It can behave in 3 different ways:
//   - (left-padded): the iterator will first return one element for the padding value
//     and then the elements of the window.
//   - (right-padded): the iterator will first return the elements of the window
//     and then one element for the padding value.
//   - (others): the iterator will not try to be smart and will return the elements
func (p *PaddedCircularWindow) IterateCompact() iter.Seq[field.Element] {

	if p.Offset_ > 0 && p.Offset_+len(p.Window_) != p.TotLen_ {
		all := p.IntoRegVecSaveAlloc()
		return slices.Values(all)
	}

	its := []iter.Seq[field.Element]{}

	if p.Offset_ > 0 {
		its = append(its, slices.Values([]field.Element{p.PaddingVal_}))
	}

	its = append(its, slices.Values(p.Window_))

	if p.Offset_+len(p.Window_) < p.TotLen_ {
		its = append(its, slices.Values([]field.Element{p.PaddingVal_}))
	}

	return utils.ChainIterators(its...)
}

// IterateSkipPadding returns an iterator over the windows of the PaddedCircularWindow
func (p *PaddedCircularWindow) IterateSkipPadding() iter.Seq[field.Element] {
	return slices.Values(p.Window_)
}

func (p *PaddedCircularWindow) Interval() CircularInterval {
	return IvalWithStartLen(p.Offset_, len(p.Window_), p.TotLen_)
}

// normalize converts the (circle) coordinator x to another coordinate by changing
// the origin point on the discret circle. mod denotes the number of points in
// the circle.
func normalize(x, newRef, mod int) int {
	return utils.PositiveMod(x-newRef, mod)
}

// processWindowedOnly applies the operator `op` to all the smartvectors
// contained in `svecs` with `coeffs` that have the type [PaddedCircularWindow]
//
// The function does so by attempting to fit result on the smallest possible
// window.
//
// In case, this is not possible. The function will "give up" and convert all
// the paddedCircularWindow into Regulars and pretend it did not find any.
//
// The function returns the partial result of the operation and the number of
// padded circular windows SmartVector that it found.
func processWindowedOnly(op operator, svecs []SmartVector, coeffs_ []int) (res SmartVector, numMatches int) {

	// First we compute the union windows.
	length := svecs[0].Len()
	windows := []PaddedCircularWindow{}
	intervals := []CircularInterval{}
	coeffs := []int{}

	// Gather all the windows into a slice
	for i, svec := range svecs {
		if pcw, ok := svec.(*PaddedCircularWindow); ok {
			windows = append(windows, *pcw)
			intervals = append(intervals, pcw.Interval())
			coeffs = append(coeffs, coeffs_[i]) // collect the coeffs related to each window
			// Sanity-check : all vectors must have the same length
			assertHasLength(svec.Len(), length)
			numMatches++
		}
	}

	if numMatches == 0 {
		return nil, numMatches
	}

	// has the dimension of the cover with garbage values in it
	smallestCover := SmallestCoverInterval(intervals)

	// Edge-case: in case the smallest-cover of the pcw found in svecs is the
	// full-circle the code below will not work as it assumes that is possible
	if smallestCover.IsFullCircle() {
		for i, svec := range svecs {
			if _, ok := svec.(*PaddedCircularWindow); ok {
				temp, _ := svec.IntoRegVecSaveAllocBase()
				svecs[i] = NewRegular(temp)
			}
		}
		return nil, 0
	}

	// Sanity-check : normally all offset are normalized, this should ensure that Start
	// is positive. This is critical here because if some of the offset are not normalized
	// then we may end up with a union windows that does not make sense.
	if smallestCover.Start() < 0 {
		utils.Panic("All offset should be normalized, but Start is %v", smallestCover.Start())
	}

	// Ensures we do not reuse an input vector here to limit the risk of overwriting one
	// of the input. This can happen if there is only a single window or if one windows
	// covers all the other.
	unionWindow := make([]field.Element, smallestCover.IntervalLen)
	var paddedTerm field.Element
	offset := smallestCover.Start()

	/*
		Now we actually compute the linear combinations for all offsets
	*/

	isFirst := true
	for i, pcw := range windows {
		interval := intervals[i]

		// Find the intersection with the larger window
		start_ := normalize(interval.Start(), offset, length)
		stop_ := normalize(interval.Stop(), offset, length)
		if stop_ == 0 {
			stop_ = length
		}

		// For the first match, we can save the operations by copying instead of
		// multiplying / adding
		if isFirst {
			isFirst = false
			op.vecIntoTerm(unionWindow[start_:stop_], pcw.Window_, coeffs[i])
			// #nosec G601 -- Deliberate pass by reference. (We trust the pointed object is not mutated)
			op.constIntoTerm(&paddedTerm, &pcw.PaddingVal_, coeffs[i])
			vector.Fill(unionWindow[:start_], paddedTerm)
			vector.Fill(unionWindow[stop_:], paddedTerm)
			continue
		}

		// sanity-check : Start and Stop are consistent with the size of pcw
		if stop_-start_ != len(pcw.Window_) {
			utils.Panic(
				"sanity-check failed. The renormalized coordinates (Start=%v, Stop=%v) are inconsistent with pcw : (len=%v)",
				start_, stop_, len(pcw.Window_),
			)
		}

		op.vecIntoVec(unionWindow[start_:stop_], pcw.Window_, coeffs[i])

		// Update the padded term
		// #nosec G601 -- Deliberate pass by reference. (We trust the pointed object is not mutated)
		op.constIntoConst(&paddedTerm, &pcw.PaddingVal_, coeffs[i])

		// Complete the left and the right-side of the window (i.e) the part
		// of unionWindow that does not overlap with pcw.window.
		// #nosec G601 -- Deliberate pass by reference. (We trust the pointed object is not mutated)
		op.constIntoVec(unionWindow[:start_], &pcw.PaddingVal_, coeffs[i])
		// #nosec G601 -- Deliberate pass by reference. (We trust the pointed object is not mutated)
		op.constIntoVec(unionWindow[stop_:], &pcw.PaddingVal_, coeffs[i])
	}

	if smallestCover.IsFullCircle() {
		return NewRegular(unionWindow), numMatches
	}

	return NewPaddedCircularWindow(unionWindow, paddedTerm, offset, length), numMatches
}

func (w *PaddedCircularWindow) DeepCopy() SmartVector {
	window := vector.DeepCopy(w.Window_)
	return NewPaddedCircularWindow(window, w.PaddingVal_, w.Offset_, w.TotLen_)
}

// Converts a smart-vector into a normal vec. The implementation minimizes
// then number of copies.
func (w *PaddedCircularWindow) IntoRegVecSaveAlloc() []field.Element {
	res, err := w.IntoRegVecSaveAllocBase()
	if err != nil {
		panic(conversionError)
	}
	return res

}

func (w *PaddedCircularWindow) IntoRegVecSaveAllocBase() ([]field.Element, error) {
	return IntoRegVec(w), nil
}

func (w *PaddedCircularWindow) IntoRegVecSaveAllocExt() []fext.Element {
	temp, _ := w.IntoRegVecSaveAllocBase()
	res := make([]fext.Element, len(temp))
	for i := 0; i < len(temp); i++ {
		elem := temp[i]
		res[i].SetFromBase(&elem)
	}
	return res
}

func (w *PaddedCircularWindow) GetPtr(n int) *field.Element {

	// This normalizes the position of n with respect to the start of the
	// window.
	n = n - w.Offset_
	if n < 0 {
		n += w.TotLen_
	}

	if n < len(w.Window_) {
		return &w.Window_[n]
	}

	return &w.PaddingVal_
}
