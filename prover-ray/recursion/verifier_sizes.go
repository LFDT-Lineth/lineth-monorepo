package recursion

import (
	"fmt"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// maxLogSize is the log of the largest module size, wiop.ColumnSizeMaxSupported.
var maxLogSize = bits.TrailingZeros(uint(wiop.ColumnSizeMaxSupported))

// dynSize is a dynamic module's size in the machine: the input log ℓ and the
// size n = 2^ℓ, read from a table of the sizes the module may take. The
// lookup bounds ℓ: n is in [MinRuntimeSize, ColumnSizeMaxSupported], as
// System.Verify checks it.
type dynSize struct {
	mod     *wiop.Module
	log, n  int // the cells of ℓ and n
	minLog  int // the log of the module's MinRuntimeSize
	inputID int // the index of ℓ's input in Verifier.inputs
}

// allocSizes allocates the input of every dynamic module's size, in module
// order, and derives its size cell.
//
// With VerifierOptions.Heights, a sized module's size is data too: its log is
// the given cell, not an input, and it is not absorbed in the transcript (the
// prover of a sized module absorbs no size). Its lower bound is the module's
// MinRuntimeSize, from which its openings have one shape.
func (v *Verifier) allocSizes() {
	mins := v.sys.MinRuntimeSizes()
	sized := 0
	for k, mod := range v.sys.Modules {
		if !mod.IsDynamic() {
			if v.opts.Heights != nil {
				minLog := bits.TrailingZeros(uint(mod.MinRuntimeSize()))
				d := &dynSize{mod: mod, log: v.opts.Heights[sized], minLog: minLog, inputID: -1}
				d.n = v.sizeConstFrom(d, minLog, "size", func(n int) field.Element { return field.NewElement(uint64(n)) })
				v.data[mod] = d
			}
			sized++
			continue
		}
		minLog := bits.TrailingZeros(uint(mins[k]))
		d := &dynSize{mod: mod, log: v.m.InputScalar(), minLog: minLog, inputID: len(v.inputs)}
		v.inputs = append(v.inputs, verifierInput{module: mod, pub: -1})
		d.n = v.sizeConstFrom(d, minLog, "size", func(n int) field.Element { return field.NewElement(uint64(n)) })
		v.sizes[k] = d
		v.data[mod] = d
		v.sizeOrder = append(v.sizeOrder, k)
	}
}

// size returns the cell of a module's size: a constant for a sized module,
// the size cell of a dynamic one.
func (v *Verifier) size(mod *wiop.Module) int {
	if d := v.dyn(mod); d != nil {
		return d.n
	}
	return v.scalarInt(mod.Size())
}

// dyn returns the size of a module whose size is data (a dynamic module, or a
// sized one with VerifierOptions.Heights), nil for a module of constant size.
func (v *Verifier) dyn(mod *wiop.Module) *dynSize {
	if d, ok := v.data[mod]; ok {
		return d
	}
	if mod.IsDynamic() {
		panic(fmt.Sprintf("recursion: dynamic module %q has no size input", mod.Context.Path()))
	}
	return nil
}

// sizeConstAtLeast returns the cell of f(n), n the module's size, for a
// function f defined for the sizes 2^i, i ≥ lo, only: a constant for a sized
// module; for a dynamic one, the entry ℓ − lo of a table of f(2^i), which a
// smaller size makes fail, and the machine's execution with it. key names f:
// the tables are shared by the modules, and the entry is looked up once per
// module.
func (v *Verifier) sizeConstAtLeast(mod *wiop.Module, lo int, key string, f func(n int) field.Element) int {
	d := v.dyn(mod)
	if d == nil {
		n := mod.Size()
		if n < 1<<lo {
			panic(fmt.Sprintf("recursion: %s of module %q, of size %d below %d", key, mod.Context.Path(), n, 1<<lo))
		}
		return v.scalar(f(n))
	}
	return v.sizeConstFrom(d, lo, key, f)
}

// sizeConstFrom reads f(n) from the table of f(2^i), i in [lo, maxLogSize],
// at ℓ − lo. The table depends on (lo, key) only, and is shared by the
// modules; the cell is memoized per module. The size's own table bounds ℓ
// from below, so a table from lo below the module's minimum is only larger.
func (v *Verifier) sizeConstFrom(d *dynSize, lo int, key string, f func(n int) field.Element) int {
	if lo > maxLogSize {
		panic(fmt.Sprintf("recursion: module %q: no size from 2^%d", d.mod.Context.Path(), lo))
	}
	tk := sizeTableKey{lo, key}
	ck := sizeCellKey{d.mod, tk}
	if c, ok := v.sizeCells[ck]; ok {
		return c
	}
	t, ok := v.sizeTables[tk]
	if !ok {
		cells := make([]int, 0, maxLogSize+1-lo)
		for i := lo; i <= maxLogSize; i++ {
			cells = append(cells, v.scalar(f(1<<i)))
		}
		t = v.m.Table(cells)
		v.sizeTables[tk] = t
	}
	idx := d.log
	if lo > 0 {
		idx = v.m.Sub(idx, v.scalarInt(lo))
	}
	c := v.m.Lookup(t, idx)
	v.sizeCells[ck] = c
	return c
}

// sizeTableKey names a table of f(2^i), i from lo: f is named by key.
type sizeTableKey struct {
	lo  int
	key string
}

// sizeCellKey names the entry of a module's size in a table.
type sizeCellKey struct {
	mod   *wiop.Module
	table sizeTableKey
}

// powSize returns the cell of x^n, n the module's size: x^(2^ℓ) for a dynamic
// module, read from the table of x's 2^i-th powers (one per x, shared by the
// modules).
func (v *Verifier) powSize(x int, mod *wiop.Module, pows func(n int) int) int {
	d := v.dyn(mod)
	if d == nil {
		return pows(mod.Size())
	}
	t, ok := v.powTables[x]
	if !ok {
		cells := make([]int, maxLogSize+1)
		for i := range cells {
			cells[i] = pows(1 << i)
		}
		t = v.m.Table(cells)
		v.powTables[x] = t
	}
	return v.m.Lookup(t, d.log)
}

// absorbSizes absorbs the dynamic modules' sizes, as Runtime.AdvanceRound
// does at the end of every round from System.DynamicSizeRound on.
func (v *Verifier) absorbSizes() {
	for _, k := range v.sizeOrder {
		v.t.absorb(v.sizes[k].n, KindScalar)
	}
}

// sizeInputs returns the values of the size inputs: the logs of the proof's
// dynamic sizes. It rejects a proof that misses a size, carries one of
// another module, or one that is not a power of two. The machine checks the
// bounds.
func (v *Verifier) sizeInputs(sizes map[int]int, res []Cell) error {
	for k := range sizes {
		if _, ok := v.sizes[k]; !ok {
			return fmt.Errorf("recursion: the proof has a size for module %d, which is not dynamic", k)
		}
	}
	for _, k := range v.sizeOrder {
		d := v.sizes[k]
		n, ok := sizes[k]
		if !ok {
			return fmt.Errorf("recursion: the proof has no size for module %q", d.mod.Context.Path())
		}
		if n <= 0 || n&(n-1) != 0 {
			return fmt.Errorf("recursion: module %q has size %d, not a power of two", d.mod.Context.Path(), n)
		}
		res[d.inputID] = ScalarCell(field.NewElement(uint64(bits.TrailingZeros(uint(n)))))
	}
	return nil
}

// checkRowLimit checks that the modules' sizes sum below limit, as the row
// limits of lookups and permutations do (a module listed twice counts twice).
// It rejects a limit that even the smallest sizes reach. With dynamic
// modules, it checks that rem − Σ n is below 2^b, rem = limit − 1 − static
// (the sized modules' rows) and b its bit length: a sum above rem wraps
// around the modulus, above 2^b, as long as the sum cannot exceed the
// modulus, which it checks from the largest sizes.
func (v *Verifier) checkRowLimit(what string, mods []*wiop.Module, limit uint64) error {
	var static, minSum, maxSum uint64
	var dyn []int
	for _, mod := range mods {
		if d := v.dyn(mod); d != nil {
			dyn = append(dyn, d.n)
			minSum += 1 << d.minLog
			maxSum += wiop.ColumnSizeMaxSupported
			continue
		}
		static += uint64(mod.Size())
	}
	minSum += static
	maxSum += static
	if minSum >= limit {
		return fmt.Errorf("%s: at least %d rows, at or above the limit %d", what, minSum, limit)
	}
	if len(dyn) == 0 || maxSum < limit {
		return nil
	}
	// The dynamic sizes sum to at most rem = limit − 1 − static, which is at
	// least the smallest sizes' sum, 1 or more: b ≥ 1.
	rem := limit - 1 - static
	b := bits.Len64(rem)
	p := field.Modulus().Uint64()
	if b >= BitsWidth || maxSum >= p || maxSum-(limit-1) >= p-(1<<b) {
		return fmt.Errorf("%s: up to %d rows against the limit %d, which the machine cannot compare", what, maxSum, limit)
	}
	sum := dyn[0]
	for _, n := range dyn[1:] {
		sum = v.m.Add(sum, n)
	}
	d := v.m.Sub(v.scalar(field.NewElement(rem)), sum)
	v.assertEq(d, v.m.Bits(d, b).Shr[0])
	return nil
}
