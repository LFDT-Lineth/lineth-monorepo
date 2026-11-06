package vecprog

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// opLengths are block lengths around and across the 16-lane width of the
// vector kernels, none of them but the full block a multiple of 16.
var opLengths = []int{1, 7, 15, 17, 33, 100, 255, BlockSize}

var allOps = []Op{Add, Sub, Mul, Div, Double, Square, Negate, Inverse}

// randBase draws n base elements, one in four of them zero.
func randBase(rng *rand.Rand, n int) []field.Element {
	v := field.VecPseudoRandBase(rng, n)
	for i := range v {
		if rng.IntN(4) == 0 {
			v[i] = field.Element{}
		}
	}
	return v
}

// randExt draws n extension elements, one in four of them zero.
func randExt(rng *rand.Rand, n int) []field.Ext {
	v := field.VecPseudoRandExt(rng, n)
	for i := range v {
		if rng.IntN(4) == 0 {
			v[i] = field.Ext{}
		}
	}
	return v
}

// operandKind is the shape of an operand over a block.
type operandKind int

const (
	okBaseScalar operandKind = iota // a random base scalar
	okBaseZero                      // the base scalar zero
	okBaseVec                       // a base vector
	okExtScalar                     // a random extension scalar
	okExtZero                       // the extension scalar zero
	okExtVec                        // an extension vector
)

func (k operandKind) String() string {
	return [...]string{"baseScalar", "baseZero", "baseVec", "extScalar", "extZero", "extVec"}[k]
}

func (k operandKind) isVector() bool { return k == okBaseVec || k == okExtVec }

// baseOperandKinds are the kinds of a base-valued operand, allOperandKinds
// those of any operand.
var (
	baseOperandKinds = []operandKind{okBaseScalar, okBaseZero, okBaseVec}
	allOperandKinds  = []operandKind{okBaseScalar, okBaseZero, okBaseVec, okExtScalar, okExtZero, okExtVec}
)

func newOperand(rng *rand.Rand, kind operandKind, L int) operand {
	switch kind {
	case okBaseScalar:
		return operand{scalar: true, isBase: true, s: field.Lift(field.VecPseudoRandBase(rng, 1)[0])}
	case okBaseZero:
		return operand{scalar: true, isBase: true}
	case okBaseVec:
		return operand{isBase: true, base: randBase(rng, L)}
	case okExtScalar:
		return operand{scalar: true, s: field.VecPseudoRandExt(rng, 1)[0]}
	case okExtZero:
		return operand{scalar: true}
	}
	return operand{ext: randExt(rng, L)}
}

// Every vector path of baseApply must match the element-by-element
// evaluation, for every operator and every combination of operand kinds a
// base-valued node can have (an operation over scalars only is a scalar
// node, never applied over a block).
func TestBaseApplyMatchesGeneric(t *testing.T) {
	rng := rand.New(rand.NewPCG(31, 32))
	w := (&Program{}).newWorker()
	for _, o := range allOps {
		for _, ka := range baseOperandKinds {
			for _, kb := range baseOperandKinds {
				if o.Arity() == 1 && kb != okBaseVec {
					continue // b is unused: run each unary case once
				}
				if !ka.isVector() && (o.Arity() == 1 || !kb.isVector()) {
					continue
				}
				for _, L := range opLengths {
					t.Run(fmt.Sprintf("op=%d/a=%v/b=%v/L=%d", o, ka, kb, L), func(t *testing.T) {
						a := newOperand(rng, ka, L)
						var b operand
						if o.Arity() > 1 {
							b = newOperand(rng, kb, L)
						}
						got, want := make([]field.Element, L), make([]field.Element, L)
						w.baseApply(o, o.Arity(), &a, &b, got)
						applyBaseGeneric(o, o.Arity(), &a, &b, want)
						require.Equal(t, want, got)
					})
				}
			}
		}
	}
}

// Every vector path of extApply must match the element-by-element
// evaluation, for every operator over every combination of operand kinds
// with a vector (an operation over scalars only is a scalar node). The
// combinations of an extension-valued node have an extension operand;
// base-only ones are run too, as extApply lifts them.
func TestExtApplyMatchesGeneric(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
	w := (&Program{}).newWorker()
	for _, o := range allOps {
		for _, ka := range allOperandKinds {
			for _, kb := range allOperandKinds {
				if o.Arity() == 1 && kb != okExtVec {
					continue // b is unused: run each unary case once
				}
				if !ka.isVector() && (o.Arity() == 1 || !kb.isVector()) {
					continue
				}
				for _, L := range opLengths {
					t.Run(fmt.Sprintf("op=%d/a=%v/b=%v/L=%d", o, ka, kb, L), func(t *testing.T) {
						a := newOperand(rng, ka, L)
						var b operand
						if o.Arity() > 1 {
							b = newOperand(rng, kb, L)
						}
						got, want := make([]field.Ext, L), make([]field.Ext, L)
						w.extApply(o, o.Arity(), &a, &b, got)
						applyExtGeneric(o, o.Arity(), &a, &b, want)
						require.Equal(t, want, got)
					})
				}
			}
		}
	}
}

// BenchmarkExtApply times extApply against the element-by-element
// evaluation over one block, for extension Add and Sub with a scalar or a
// base-field first operand.
func BenchmarkExtApply(b *testing.B) {
	rng := rand.New(rand.NewPCG(3, 4))
	w := (&Program{}).newWorker()
	op := func(k operandKind) operand { return newOperand(rng, k, BlockSize) }
	cases := []struct {
		name string
		o    Op
		a, b operand
	}{
		{"Add/extVec-extScalar", Add, op(okExtVec), op(okExtScalar)},
		{"Add/extScalar-extVec", Add, op(okExtScalar), op(okExtVec)},
		{"Add/extVec-baseScalar", Add, op(okExtVec), op(okBaseScalar)},
		{"Add/baseVec-extVec", Add, op(okBaseVec), op(okExtVec)},
		{"Sub/extVec-extScalar", Sub, op(okExtVec), op(okExtScalar)},
		{"Sub/extScalar-extVec", Sub, op(okExtScalar), op(okExtVec)},
		{"Sub/baseScalar-extVec", Sub, op(okBaseScalar), op(okExtVec)},
		{"Sub/baseVec-extVec", Sub, op(okBaseVec), op(okExtVec)},
		{"Add/extVec-extVec", Add, op(okExtVec), op(okExtVec)},
	}
	dst := make([]field.Ext, BlockSize)
	for _, c := range cases {
		b.Run(c.name+"/generic", func(b *testing.B) {
			for b.Loop() {
				applyExtGeneric(c.o, c.o.Arity(), &c.a, &c.b, dst)
			}
		})
		b.Run(c.name+"/vector", func(b *testing.B) {
			for b.Loop() {
				w.extApply(c.o, c.o.Arity(), &c.a, &c.b, dst)
			}
		})
	}
}

// BenchmarkBaseApply times baseApply against the element-by-element
// evaluation over one block, for the operand kinds of the quotient.
func BenchmarkBaseApply(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	w := (&Program{}).newWorker()
	vec := func() operand { return newOperand(rng, okBaseVec, BlockSize) }
	scal := func() operand { return newOperand(rng, okBaseScalar, BlockSize) }
	cases := []struct {
		name string
		o    Op
		a, b operand
	}{
		{"Add/vector-scalar", Add, vec(), scal()},
		{"Sub/vector-scalar", Sub, vec(), scal()},
		{"Sub/scalar-vector", Sub, scal(), vec()},
		{"Negate", Negate, vec(), operand{}},
		{"Double", Double, vec(), operand{}},
		{"Square", Square, vec(), operand{}},
	}
	dst := make([]field.Element, BlockSize)
	for _, c := range cases {
		b.Run(c.name+"/generic", func(b *testing.B) {
			for b.Loop() {
				applyBaseGeneric(c.o, c.o.Arity(), &c.a, &c.b, dst)
			}
		})
		b.Run(c.name+"/vector", func(b *testing.B) {
			for b.Loop() {
				w.baseApply(c.o, c.o.Arity(), &c.a, &c.b, dst)
			}
		})
	}
}
