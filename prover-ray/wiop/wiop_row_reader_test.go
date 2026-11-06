package wiop_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// referenceRows returns the n rows of the assignment a, padded towards pd and
// cyclically shifted by shift, in the extension field, one row at a time:
// row i is padded row p = (i+shift) mod n, which holds data when it falls in
// [start, start+len) with start = n−len for Left padding and 0 otherwise.
func referenceRows(a *wiop.ConcreteVector, pd wiop.PaddingDirection, n, shift int) []field.Ext {
	start := 0
	if pd == wiop.PaddingDirectionLeft {
		start = n - a.Plain.Len()
	}
	out := make([]field.Ext, n)
	for i := range out {
		p := ((i+shift)%n + n) % n
		switch {
		case p < start || p >= start+a.Plain.Len():
			out[i] = field.Lift(a.Padding)
		case a.Plain.IsBase():
			out[i] = field.Lift(a.Plain.AsBase()[p-start])
		default:
			out[i] = a.Plain.AsExt()[p-start]
		}
	}
	return out
}

func randBase(rng *rand.Rand, n int) []field.Element {
	v := make([]field.Element, n)
	for i := range v {
		v[i].SetUint64(rng.Uint64())
	}
	return v
}

func randExt(rng *rand.Rand, n int) []field.Ext {
	v := make([]field.Ext, n)
	for i := range v {
		v[i].B0.A0.SetUint64(rng.Uint64())
		v[i].B1.A1.SetUint64(rng.Uint64())
		v[i].B2.A0.SetUint64(rng.Uint64())
	}
	return v
}

// checkReader compares every block [lo, hi) the reader returns, in both
// fields when the data is base, with want.
func checkReader(t *testing.T, r wiop.RowReader, want []field.Ext, msg string) {
	t.Helper()
	n := len(want)
	require.Equal(t, n, r.Len(), msg)
	ext := make([]field.Ext, n+3)
	base := make([]field.Element, n+3)
	for lo := 0; lo <= n; lo++ {
		for hi := lo; hi <= n; hi++ {
			got := r.ReadExt(lo, hi, ext)
			require.Len(t, got, hi-lo, msg)
			require.Equal(t, want[lo:hi], got, "%s: ReadExt [%d, %d)", msg, lo, hi)
			if !r.IsBase() {
				continue
			}
			gotBase := r.ReadBase(lo, hi, base)
			require.Len(t, gotBase, hi-lo, msg)
			for i, x := range gotBase {
				require.Equal(t, want[lo+i], field.Lift(x), "%s: ReadBase [%d, %d) row %d", msg, lo, hi, lo+i)
			}
		}
	}
}

// evaluatedAsExt returns the rows of EvaluateVector's result, lifted.
func evaluatedAsExt(v wiop.ConcreteVector) []field.Ext {
	if !v.Plain.IsBase() {
		return v.Plain.AsExt()
	}
	out := make([]field.Ext, v.Plain.Len())
	for i, x := range v.Plain.AsBase() {
		out[i] = field.Lift(x)
	}
	return out
}

// columnKind is a column's field paired with its assignment's.
type columnKind int

const (
	baseColumn        columnKind = iota // base column, base data
	extColumn                           // extension column, extension data
	extColumnBaseData                   // extension column, base data
)

var paddings = []wiop.PaddingDirection{wiop.PaddingDirectionNone, wiop.PaddingDirectionLeft, wiop.PaddingDirectionRight}

// TestRowReaderMatchesEvaluateVector reads random base, extension and
// base-assigned extension columns of every data length, through views of
// positive, negative and larger-than-n shifts, over every row range (ranges
// crossing the wrap point and the padding boundary included), and checks the
// reader, EvaluateVector and EvaluateVectorAsExt against a row-by-row
// reference.
func TestRowReaderMatchesEvaluateVector(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for _, n := range []int{1, 2, 4, 8} {
		shifts := []int{0, 1, -1, n - 1, -(n - 1), n, -n, n + 1, -n - 1, 2*n + 3, -3*n - 2}
		for _, pd := range paddings {
			for dataLen := 1; dataLen <= n; dataLen++ {
				for _, kind := range []columnKind{baseColumn, extColumn, extColumnBaseData} {
					sys := wiop.NewSystemf("s")
					r0 := sys.NewRound()
					mod := sys.NewSizedModule(sys.Context.Childf("m"), n, pd)
					var col *wiop.Column
					a := &wiop.ConcreteVector{}
					a.Padding.SetUint64(rng.Uint64())
					switch kind {
					case baseColumn:
						col = mod.NewColumn(sys.Context.Childf("c"), r0)
						a.Plain = field.VecFromBase(randBase(rng, dataLen))
					case extColumn:
						col = mod.NewExtensionColumn(sys.Context.Childf("c"), r0)
						a.Plain = field.VecFromExt(randExt(rng, dataLen))
					default:
						col = mod.NewExtensionColumn(sys.Context.Childf("c"), r0)
						a.Plain = field.VecFromBase(randBase(rng, dataLen))
					}
					rt := wiop.NewRuntime(sys)
					rt.AssignColumn(col, a)
					for _, s := range shifts {
						msg := fmt.Sprintf("n=%d pd=%v len=%d kind=%d shift=%d", n, pd, dataLen, kind, s)
						cv := col.View().Shift(s)
						want := referenceRows(a, pd, n, s)
						checkReader(t, cv.Rows(rt), want, msg)
						require.Equal(t, want, evaluatedAsExt(cv.EvaluateVector(rt)), msg)
						for _, m := range []int{n / 2, n, n + 3} {
							got := cv.EvaluateVectorAsExt(rt, m)
							require.Len(t, got, m, msg)
							for i := range m {
								w := field.Lift(a.Padding)
								if i < n {
									w = want[i]
								}
								require.Equal(t, w, got[i], "%s: EvaluateVectorAsExt(%d) row %d", msg, m, i)
							}
						}
					}
				}
			}
		}
	}
}

// TestRowReaderDynamicModule reads views of a dynamic module, sized per
// runtime from its assignments.
func TestRowReaderDynamicModule(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	for _, pd := range []wiop.PaddingDirection{wiop.PaddingDirectionLeft, wiop.PaddingDirectionRight} {
		for _, dataLen := range []int{3, 5, 8} {
			sys := wiop.NewSystemf("s")
			r0 := sys.NewRound()
			mod := sys.NewDynamicModule(sys.Context.Childf("m"), pd)
			col := mod.NewColumn(sys.Context.Childf("c"), r0)
			short := mod.NewColumn(sys.Context.Childf("short"), r0)
			rt := wiop.NewRuntime(sys)
			a := &wiop.ConcreteVector{Plain: field.VecFromBase(randBase(rng, dataLen))}
			b := &wiop.ConcreteVector{Plain: field.VecFromBase(randBase(rng, 2))}
			b.Padding.SetUint64(rng.Uint64())
			rt.AssignColumn(col, a)
			rt.AssignColumn(short, b)
			n := mod.RuntimeSize(rt)
			for _, s := range []int{0, 1, -2, n + 1} {
				msg := fmt.Sprintf("dynamic pd=%v len=%d shift=%d", pd, dataLen, s)
				checkReader(t, col.View().Shift(s).Rows(rt), referenceRows(a, pd, n, s), msg)
				checkReader(t, short.View().Shift(s).Rows(rt), referenceRows(b, pd, n, s), msg)
				require.Equal(t, referenceRows(b, pd, n, s), evaluatedAsExt(short.View().Shift(s).EvaluateVector(rt)), msg)
			}
		}
	}
}

// TestNewRowReaderOverfull covers assignments longer than n, which only
// [wiop.NewRowReader] can lay out: Left padding keeps the last n values, None
// and Right the first n.
func TestNewRowReaderOverfull(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 9))
	for _, pd := range paddings {
		for _, n := range []int{1, 3, 4} {
			for _, ext := range []bool{false, true} {
				a := &wiop.ConcreteVector{}
				a.Padding.SetUint64(rng.Uint64())
				if ext {
					a.Plain = field.VecFromExt(randExt(rng, n+2))
				} else {
					a.Plain = field.VecFromBase(randBase(rng, n+2))
				}
				for _, s := range []int{0, 1, -1, n + 2} {
					msg := fmt.Sprintf("overfull pd=%v n=%d ext=%v shift=%d", pd, n, ext, s)
					checkReader(t, wiop.NewRowReader(a, pd, n, s), referenceRows(a, pd, n, s), msg)
				}
			}
		}
	}
}

func TestRowReaderPanics(t *testing.T) {
	a := &wiop.ConcreteVector{Plain: field.VecFromExt(make([]field.Ext, 4))}
	r := wiop.NewRowReader(a, wiop.PaddingDirectionNone, 4, 1)
	require.Panics(t, func() { r.ReadBase(0, 1, make([]field.Element, 1)) }, "base read of ext data")
	require.Panics(t, func() { r.ReadExt(-1, 1, make([]field.Ext, 2)) })
	require.Panics(t, func() { r.ReadExt(2, 1, make([]field.Ext, 2)) })
	require.Panics(t, func() { r.ReadExt(0, 5, make([]field.Ext, 5)) })
	require.Panics(t, func() { r.ReadExt(0, 3, make([]field.Ext, 2)) }, "short dst")
	require.Empty(t, r.ReadExt(4, 4, nil))
}
