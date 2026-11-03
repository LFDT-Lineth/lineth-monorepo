package global

import (
	"fmt"
	"sync"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/internal/vecprog"
)

// The quotient program of a bucket depends on the module size, through the
// shifts of its column views, but on nothing else that changes between
// proofs: the coset tables, cancellations, cells and coins of a proof enter it
// as [vecprog.Bindings]. It is therefore built once per bucket and module
// size -- once for a static module, once per runtime size for a dynamic one
// -- and every later proof only binds its data to it.
//
// Concurrency: expression nodes are never mutated after construction, and a
// compiled program is never written after [vecprog.Builder.Compile]. A
// program is built (under its cache's lock) before any run reads it, and a
// run's workers read only the program and their run's bindings, writing only
// their own chunk of the aggregate. So programs may be shared by concurrent
// runs.

// programCache holds the quotient programs of one bucket by module size.
type programCache struct {
	mu  sync.Mutex
	byN map[int]*bucketProgram
}

func newProgramCache() *programCache {
	return &programCache{byN: make(map[int]*bucketProgram)}
}

// program returns the quotient program of bkt at module size n, building it
// on first use.
func (q *moduleQuotient) program(bkt *proverBucket, n int) *bucketProgram {
	c := bkt.programs
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.byN[n]
	if !ok {
		p = newBucketProgram(bktVanishings(bkt), q.mergeCoin, n, bkt.ratio, q.maxRatio)
		c.byN[n] = p
	}
	return p
}

// tableKind is what a base-field slot of a quotient program holds.
type tableKind uint8

const (
	tableColumn       tableKind = iota // a column's coset evaluations
	tableSelector                      // a Lagrange selector's coset evaluations
	tableCancellation                  // a vanishing's cancellation coset
)

// baseTable identifies a base-field slot; vanishing is only used to fetch a
// cancellation, whose identity is the set of cancelled positions.
type baseTable struct {
	kind      tableKind
	col       wiop.ObjectID
	position  int
	cancelled string
	vanishing int
}

// bucketProgram is the compiled quotient of one bucket at one module size,
// and the layout of the [vecprog.Bindings] it is run with: Base[i] is
// base[i], Ext[i] the coset evaluations of the column extCols[i], and
// Params[i] the value of params[i], the merge coin first.
type bucketProgram struct {
	prog    *vecprog.Program
	base    []baseTable
	extCols []wiop.ObjectID
	params  []wiop.Expression // *wiop.CoinField or *wiop.Cell
}

// newBucketProgram lowers the vanishings of a bucket of the given ratio into
// one program over the bucket's cosets of a module of n rows, whose tables
// hold maxRatio cosets: Σ_j coin^j · P_j · C_j, P_j the j-th vanishing and
// C_j its cancellation.
func newBucketProgram(vanishings []*wiop.Vanishing, mergeCoin *wiop.CoinField, n, ratio, maxRatio int) *bucketProgram {
	bp := &bucketProgram{params: []wiop.Expression{mergeCoin}}
	var (
		b         = vecprog.NewBuilder()
		baseSlots = map[baseTable]int{}
		extSlots  = map[wiop.ObjectID]int{}
		params    = map[wiop.Expression]int{mergeCoin: 0}
	)
	baseSlot := func(t baseTable) int {
		key := t
		key.vanishing = 0
		if s, ok := baseSlots[key]; ok {
			return s
		}
		baseSlots[key] = len(bp.base)
		bp.base = append(bp.base, t)
		return len(bp.base) - 1
	}
	param := func(e wiop.Expression) int {
		if s, ok := params[e]; ok {
			return s
		}
		params[e] = len(bp.params)
		bp.params = append(bp.params, e)
		return len(bp.params) - 1
	}
	leaf := func(expr wiop.Expression) int {
		switch e := expr.(type) {
		case *wiop.ColumnView:
			// A shift s reads the point x_t·ω_n^s, in the same small coset
			// as x_t (see [cosetShifts]).
			offset := ((e.ShiftingOffset % n) + n) % n
			id := e.Column.Context.ID
			if !e.Column.IsExtension {
				return b.BaseSlot(baseSlot(baseTable{kind: tableColumn, col: id}), offset)
			}
			s, ok := extSlots[id]
			if !ok {
				s = len(bp.extCols)
				extSlots[id] = s
				bp.extCols = append(bp.extCols, id)
			}
			return b.ExtSlot(s, offset)
		case *wiop.LagrangeSelector:
			return b.BaseSlot(baseSlot(baseTable{kind: tableSelector, position: e.Position}), 0)
		case *wiop.Cell:
			return b.Param(param(e), !e.IsExtension())
		case *wiop.CoinField:
			return b.Param(param(e), false)
		}
		panic(fmt.Sprintf("wiop/compilers: unsupported expression type %T in the quotient", expr))
	}

	lowering := wiop.NewExpressionLowering(nil, b, leaf)
	coin := b.Param(0, false)
	coinPow := b.Scalar(field.Lift(field.One()), true)
	for j, v := range vanishings {
		cancel := -1
		if len(v.CancelledPositions) > 0 {
			cancel = baseSlot(baseTable{
				kind: tableCancellation, cancelled: fmt.Sprint(v.CancelledPositions), vanishing: j,
			})
		}
		b.AccumulateBy(lowering.Lower(v.Expression), cancel, coinPow)
		coinPow = b.Op(vecprog.Mul, coinPow, coin)
	}
	bp.prog = b.Compile(n, ratio, maxRatio/ratio)
	return bp
}

// bind returns the bindings of one run: the module's coset tables, the
// cancellation of the j-th vanishing as cancellation(j), and the values of
// the cells and coins in rt.
func (bp *bucketProgram) bind(
	rt *wiop.Runtime,
	cosetEvals map[wiop.ObjectID][]field.Element,
	cosetEvalsExt map[wiop.ObjectID][]field.Ext,
	selectorCosets map[int][]field.Element,
	cancellation func(j int) []field.Element,
) *vecprog.Bindings {
	bind := &vecprog.Bindings{
		Base:   make([][]field.Element, len(bp.base)),
		Ext:    make([][]field.Ext, len(bp.extCols)),
		Params: make([]field.Ext, len(bp.params)),
	}
	for i, t := range bp.base {
		switch t.kind {
		case tableColumn:
			bind.Base[i] = cosetEvals[t.col]
		case tableSelector:
			bind.Base[i] = selectorCosets[t.position]
		case tableCancellation:
			bind.Base[i] = cancellation(t.vanishing)
		}
	}
	for i, id := range bp.extCols {
		bind.Ext[i] = cosetEvalsExt[id]
	}
	for i, e := range bp.params {
		switch e := e.(type) {
		case *wiop.CoinField:
			bind.Params[i] = rt.GetCoinValue(e).AsExt()
		case *wiop.Cell:
			v := rt.GetCellValue(e)
			switch {
			case e.IsExtension():
				bind.Params[i] = v.AsExt()
			case v.IsBase():
				bind.Params[i] = field.Lift(v.AsBase())
			default:
				panic(fmt.Sprintf("wiop: cell %q declared as base but holds an extension-field value", e.Context.Path()))
			}
		}
	}
	return bind
}

// BucketStats are the node counts of the quotient of one bucket.
type BucketStats struct {
	Module     string
	Ratio      int
	Vanishings int
	// TreeNodes counts the nodes of the vanishings walked as trees, leaves
	// included, saturating at the uint64 maximum: what a lowering that
	// re-descends into shared subexpressions visits.
	TreeNodes uint64
	// DistinctNodes counts the distinct nodes of the vanishings, by pointer.
	DistinctNodes int
	// ProgramLeaves, ProgramSteps and ProgramScalars count, in the compiled
	// program, the table leaves read, the nodes computed at every point, and
	// the scalars computed once per proof.
	ProgramLeaves, ProgramSteps, ProgramScalars int
}

// Stats returns the node counts of every bucket of a, its program compiled
// at the size of a static module and at dynamicSize for a dynamic one. The
// programs are compiled for the occasion, not cached.
func (a *QuotientProverAction) Stats(dynamicSize int) []BucketStats {
	var out []BucketStats
	for _, q := range a.modules {
		n := dynamicSize
		if !q.m.IsDynamic() {
			n = q.m.Size()
		}
		for i := range q.buckets {
			bkt := &q.buckets[i]
			vanishings := bktVanishings(bkt)
			st := BucketStats{Module: q.m.Context.Path(), Ratio: bkt.ratio, Vanishings: len(vanishings)}

			treeSize := map[wiop.Expression]uint64{}
			var walk func(e wiop.Expression) uint64
			walk = func(e wiop.Expression) uint64 {
				if s, ok := treeSize[e]; ok {
					return s
				}
				s := uint64(1)
				if op, ok := e.(*wiop.ArithmeticOperation); ok {
					for _, o := range op.Operands {
						s = saturatingAdd(s, walk(o))
					}
				}
				treeSize[e] = s
				return s
			}
			for _, v := range vanishings {
				st.TreeNodes = saturatingAdd(st.TreeNodes, walk(v.Expression))
			}
			st.DistinctNodes = len(treeSize)

			ps := newBucketProgram(vanishings, q.mergeCoin, n, bkt.ratio, q.maxRatio).prog.Stats()
			st.ProgramLeaves, st.ProgramSteps, st.ProgramScalars = ps.Leaves, ps.Steps, ps.Scalars
			out = append(out, st)
		}
	}
	return out
}

func saturatingAdd(a, b uint64) uint64 {
	if a+b < a {
		return ^uint64(0)
	}
	return a + b
}
