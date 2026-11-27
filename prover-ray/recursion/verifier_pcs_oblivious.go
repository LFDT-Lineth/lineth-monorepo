package recursion

import (
	"cmp"
	"fmt"
	"math/bits"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// The size-oblivious PCS opening (pcs.WithSizeOblivious, see
// pcs.ObliviousOpening), in the machine. The FRI schedule is fixed by the
// system; every committed module's log size ℓ is a cell (a dynamic module's
// size input, a constant for a sized one), and decides where the module
// enters the checks: one code path for both.
//
//  1. Transcript: as the static opening (see verifier_pcs.go).
//  2. Per module, once: its round j = MaxSizeLog2 − ℓ, its alphaDeep a (the
//     square of round j's fold challenge, read from a table), a^(Power) per
//     group, and per opening shift s the claim point z = ζ·ω_n^s (read from a
//     table at ℓ) and V_s = Σ a^(p(c))·y_(c,s).
//  3. Routing, once: the modules sorted by round, an input order π that
//     AssertCanonicalOrder checks (decreasing ℓ, then module order), and the
//     inputs s_j, the position in π of the first module of round j, checked
//     against the sorted rounds. Per round j ≥ 1, the running codeword's
//     factor: a_j^R if a module joins at round j, 1 otherwise.
//  4. Per query, the input trees: a group's pair is at height ℓ + rate − 1
//     of its batch's tree; its two rows are ordered by the query position's
//     bit j and hashed with the group's header, and the dynamic path
//     (MerklePathDynamic) chains the groups at their heights, in an input
//     order checked once per batch (AssertCanonicalOrder).
//  5. Per query and module, the module's DEEP terms at the query point x_j
//     and at its conjugate (x_j read from a table at j), and per round their
//     sums: prefix sums of the modules' terms in π order, read at s_j and
//     s_(j+1).
//  6. Per query, the running layers and the folds, as the static opening.
//
// Not supported, and rejected when the verifier is built: Merkle caps (module
// groups exclude them), the BN-packed Merkle hash and a final polynomial of
// more than one coefficient.

// obliviousPCS is the in-machine size-oblivious PCS opening of one system.
type obliviousPCS struct {
	pcsCommon
	op *pcs.ObliviousOpening

	mods    []*obModule // the committed modules, in module order
	modPos  map[int]int // module index → position in mods
	alpha2  []int       // per round, alphaDeep: the square of its fold challenge
	order   []int       // π_k: positions in mods, by increasing round
	orderIn []int       // the input indices of the π_k
	starts  []int       // s_0 … s_R: the position in π of the first module of round j
	coef    []int       // per round j ≥ 1, the running codeword's factor
	trees   []obTree    // per batch
	gInvs   []field.Element
	// siblings maps the input index of a dynamic path's sibling to its query,
	// batch and height: above its tree's actual height, a sibling is
	// anything (MerklePathDynamic does not read it).
	siblings map[int][3]int
}

// obModule is the query-independent part of a committed module.
type obModule struct {
	mod             *wiop.Module
	index           int // in sys.Modules
	log, round, a   int // ℓ, j and alphaDeep
	groups          []obGroup
	shifts          []int // the distinct opening shifts, increasing
	z, vz           []int // per shift
	groupsOfBatches map[[2]int]int
}

// obGroup is a group of a module: batch b's group g, and a^(its power).
type obGroup struct {
	batch, group, aPow int
}

// obTree is a batch's tree: its groups' heights and their input order.
type obTree struct {
	heights, order Table
}

func (v *Verifier) checkObliviousPCS(a *pcs.OpeningVerifierAction) error {
	op, err := a.Oblivious(v.sys)
	if err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	ob := &obliviousPCS{pcsCommon: newPCSCommon(v), op: op, modPos: map[int]int{}, siblings: map[int][3]int{}}
	if err := ob.supported(); err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	ob.claims(op.Manifests, op.Duplicates)
	zeta, err := ob.zeta(op.EvalPoints)
	if err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	ob.outOfDomain(zeta, op.LogCodewordSize)
	positions := ob.transcript(op.NumRounds, op.NumQueries, op.Batches, op.PrecomputedRoot)
	ob.modules(zeta)
	ob.routing()
	ob.treeOrders()
	var gInv field.Element
	gInv.Inverse(&op.Generators[0])
	ob.gInvs = make([]field.Element, op.LogCodewordSize)
	for i := range ob.gInvs {
		// bit i of the position weighs 2^(L−1−i) in its bit reversal
		g := gInv
		for range op.LogCodewordSize - 1 - i {
			g.Square(&g)
		}
		ob.gInvs[i] = g
	}
	for q, pos := range positions {
		ob.query(q, pos)
	}
	v.pcs = ob
	return nil
}

// supported rejects what the machine does not verify yet.
func (ob *obliviousPCS) supported() error {
	op := ob.op
	switch {
	case pcs.MerkleHash(ob.v.sys) != fri.MerkleHashKoalaBear:
		return fmt.Errorf("the Merkle hash %v is not supported", pcs.MerkleHash(ob.v.sys))
	case op.Params.MerkleCapHeight != 0:
		return fmt.Errorf("the Merkle caps are not supported")
	case op.LogFinalPolySize != 0:
		return fmt.Errorf("a final polynomial of 2^%d coefficients is not supported", op.LogFinalPolySize)
	case op.NumRounds != op.MaxSizeLog2 || op.MinSizeLog2 != 1 || op.MaxSizeLog2 > maxLogSize:
		return fmt.Errorf("a schedule of %d rounds, of sizes 2^%d to 2^%d", op.NumRounds, op.MinSizeLog2, op.MaxSizeLog2)
	case op.LogCodewordSize >= BitsWidth:
		return fmt.Errorf("a codeword of size 2^%d is too large", op.LogCodewordSize)
	}
	return nil
}

// modules computes the query-independent part of every committed module.
func (ob *obliviousPCS) modules(zeta int) {
	v, m, op := ob.v, ob.v.m, ob.op
	for _, a := range ob.alphas {
		ob.alpha2 = append(ob.alpha2, m.Mul(a, a))
	}
	alpha2 := m.Table(ob.alpha2)
	for k, mod := range v.sys.Modules {
		var groups []obGroup
		for b, gs := range op.Groups {
			for g := range gs {
				if gs[g].ModuleIndex == k {
					groups = append(groups, obGroup{batch: b, group: g})
				}
			}
		}
		if len(groups) == 0 {
			continue
		}
		om := &obModule{mod: mod, index: k, groups: groups, log: v.logSize(mod), groupsOfBatches: map[[2]int]int{}}
		// The alphaDeep table has an entry per fold, rounds 0 to R − 1 = MaxSizeLog2
		// − 1 (a final polynomial of one coefficient): reading it bounds ℓ to
		// [1, MaxSizeLog2], the sizes FRI folds (pcs.checkDynamicMinSizes) and
		// commits.
		om.round = m.Sub(v.scalarInt(op.MaxSizeLog2), om.log)
		om.a = m.Lookup(alpha2, om.round)
		shifts := map[int]bool{}
		for i := range om.groups {
			g := &om.groups[i]
			og := op.Groups[g.batch][g.group]
			g.aPow = ob.pow(om.a, og.Power)
			om.groupsOfBatches[[2]int{g.batch, g.group}] = i
			for _, col := range groupColumns(og) {
				for _, s := range col.Shifts {
					shifts[s] = true
				}
			}
		}
		for s := range shifts {
			om.shifts = append(om.shifts, s)
		}
		slices.Sort(om.shifts)
		for _, s := range om.shifts {
			w := ob.one
			if s != 0 {
				w = v.sizeConstAtLeast(mod, 0, rootKey(s), func(n int) field.Element { return root(n, s) })
			}
			om.z = append(om.z, m.Mul(zeta, w))
			vz := -1
			for _, g := range om.groups {
				cols := groupColumns(op.Groups[g.batch][g.group])
				top := topOpenedAt(cols, s)
				if top < 0 {
					continue
				}
				coeffs := make([]int, 0, top+1)
				for i := top; i >= 0; i-- {
					c := ob.zero
					if k := slices.Index(cols[i].Shifts, s); k >= 0 {
						c = v.cell(cols[i].Claims[k])
					}
					coeffs = append(coeffs, c)
				}
				term := m.Mul(g.aPow, m.Horner(om.a, coeffs))
				if vz < 0 {
					vz = term
				} else {
					vz = m.Add(vz, term)
				}
			}
			om.vz = append(om.vz, vz)
		}
		ob.modPos[k] = len(ob.mods)
		ob.mods = append(ob.mods, om)
	}
}

// groupColumns returns a group's columns by increasing power: its base
// columns, then its extension ones.
func groupColumns(g pcs.OpeningGroup) []pcs.OpenedColumn {
	return append(append([]pcs.OpenedColumn(nil), g.Base...), g.Ext...)
}

// topOpenedAt returns the index of the last column opened at shift s, -1 if
// none is.
func topOpenedAt(cols []pcs.OpenedColumn, s int) int {
	for i := len(cols) - 1; i >= 0; i-- {
		if slices.Contains(cols[i].Shifts, s) {
			return i
		}
	}
	return -1
}

// routing checks the modules' order by round and the rounds' starts in it,
// and computes the running codeword's factor per round.
func (ob *obliviousPCS) routing() {
	v, m, op := ob.v, ob.v.m, ob.op
	K, R := len(ob.mods), op.NumRounds
	logs, rounds := make([]int, K), make([]int, K)
	for k, om := range ob.mods {
		logs[k], rounds[k] = om.log, om.round
	}
	for k := range K {
		ob.orderIn = append(ob.orderIn, len(v.inputs))
		ob.order = append(ob.order, ob.inputFrom(KindScalar, func(in *pcsInput) Cell {
			return ScalarCell(field.NewElement(uint64(in.ob.modOrder[k])))
		}))
	}
	m.AssertCanonicalOrder(m.Table(ob.order), m.Table(logs), maxLogSize)

	// sorted[i] = the round of π_(i−1), with sentinels 0 and R around: the
	// rounds increase along π
	roundsT := m.Table(rounds)
	sorted := make([]int, 1, K+2)
	sorted[0] = ob.zero
	for _, p := range ob.order {
		sorted = append(sorted, m.Lookup(roundsT, p))
	}
	sorted = append(sorted, v.scalarInt(R))
	sortedT := m.Table(sorted)

	// s_j is the position of round j's first module: round(π_(s_j − 1)) < j
	// ≤ round(π_(s_j)), which only one position satisfies
	ob.starts = []int{ob.zero}
	b := bits.Len(uint(R))
	for j := 1; j < R; j++ {
		s := ob.inputFrom(KindScalar, func(in *pcsInput) Cell {
			return ScalarCell(field.NewElement(uint64(in.ob.starts[j])))
		})
		below := m.Sub(v.scalarInt(j-1), m.Lookup(sortedT, s))
		v.assertEq(below, m.Bits(below, b).Shr[0])
		above := m.Sub(m.Lookup(sortedT, m.Add(s, ob.one)), v.scalarInt(j))
		v.assertEq(above, m.Bits(above, b).Shr[0])
		ob.starts = append(ob.starts, s)
	}
	ob.starts = append(ob.starts, v.scalarInt(K))

	// a round with a module has a level: a^R·running + its DEEP terms
	nonzero := make([]int, 1, K+1)
	nonzero[0] = ob.zero
	for range K {
		nonzero = append(nonzero, ob.one)
	}
	nonzeroT := m.Table(nonzero)
	ob.coef = make([]int, R)
	for j := 1; j < R; j++ {
		has := m.Lookup(nonzeroT, m.Sub(ob.starts[j+1], ob.starts[j]))
		aR := ob.pow(ob.alpha2[j], op.RunningPower)
		ob.coef[j] = m.Add(ob.one, m.Mul(has, m.Sub(aR, ob.one)))
	}
}

// treeOrders checks, per batch, the input order of its groups in its tree.
func (ob *obliviousPCS) treeOrders() {
	v, m, op := ob.v, ob.v.m, ob.op
	for b, groups := range op.Groups {
		var heights, order []int
		for g, og := range groups {
			heights = append(heights, m.Add(ob.mods[ob.modPos[og.ModuleIndex]].log, v.scalarInt(op.RateLog-1)))
			order = append(order, ob.inputFrom(KindScalar, func(in *pcsInput) Cell {
				return ScalarCell(field.NewElement(uint64(in.ob.treeOrder[b][g])))
			}))
		}
		tr := obTree{heights: m.Table(heights), order: m.Table(order)}
		m.AssertCanonicalOrder(tr.order, tr.heights, op.LogCodewordSize-1)
		ob.trees = append(ob.trees, tr)
	}
}

// groupRow allocates the input cells of one side (0: the queried row, 1: its
// conjugate) of a group's pair.
func (ob *obliviousPCS) groupRow(q, b, g, side int) rowCells {
	og := ob.op.Groups[b][g]
	row := func(in *pcsInput) (fri.RowOpening, obGroupLoc) {
		loc := in.ob.groups[b][g]
		return in.proof.InputQueries[q][loc.tree].Leaves[loc.level][side], loc
	}
	var rc rowCells
	w := len(og.Base)
	for k := 0; k < w; k += CellWidth {
		rc.base = append(rc.base, ob.inputFrom(KindDigest, func(in *pcsInput) Cell {
			r, loc := row(in)
			var c Cell
			copy(c[:], r.Base[loc.base+k:loc.base+min(k+CellWidth, w)])
			return c
		}))
	}
	for k := range og.Ext {
		rc.ext = append(rc.ext, ob.inputFrom(KindE6, func(in *pcsInput) Cell {
			r, loc := row(in)
			return E6Cell(r.Ext[loc.ext+k])
		}))
	}
	return rc
}

// query checks one FRI query, at the position in the lane cell pos.
func (ob *obliviousPCS) query(q, pos int) {
	m, op := ob.v.m, ob.op
	L, R, H := op.LogCodewordSize, op.NumRounds, op.LogCodewordSize-1
	bitsQ := m.Bits(pos, L)

	// x_j, the query point at round j, and the position's bit j: whether
	// the queried row of round j is the odd one of its pair
	xInv := m.PowBits(bitsQ.Bit, ob.gInvs)
	xs := []int{m.Div(ob.one, xInv)}
	for j := 1; j < R; j++ {
		xs = append(xs, m.Mul(xs[j-1], xs[j-1]))
	}
	xT, bitT := m.Table(xs), m.Table(bitsQ.Bit[:R])
	x, bit := make([]int, len(ob.mods)), make([]int, len(ob.mods))
	for k, om := range ob.mods {
		x[k], bit[k] = m.Lookup(xT, om.round), m.Lookup(bitT, om.round)
	}

	// input trees: rows[module][its group] = the pair's two sides
	rows := make([][][2]rowCells, len(ob.mods))
	for k, om := range ob.mods {
		rows[k] = make([][2]rowCells, len(om.groups))
	}
	for b, groups := range op.Groups {
		leaves := make([]int, len(groups))
		for g, og := range groups {
			k := ob.modPos[og.ModuleIndex]
			pair := [2]rowCells{ob.groupRow(q, b, g, 0), ob.groupRow(q, b, g, 1)}
			rows[k][ob.mods[k].groupsOfBatches[[2]int{b, g}]] = pair
			even, odd := ob.selectRow(bit[k], pair[1], pair[0]), ob.selectRow(bit[k], pair[0], pair[1])
			leaves[g] = ob.hashLeaf(fri.SizedShape{BaseWidth: len(og.Base), ExtWidth: len(og.Ext)}, even, odd)
		}
		siblings := make([]int, H)
		for h := 1; h <= H; h++ {
			// Siblings[h − 1] is the sibling at height h; above the tree, any
			ob.siblings[len(ob.v.inputs)] = [3]int{q, b, h}
			siblings[h-1] = ob.inputFrom(KindDigest, func(in *pcsInput) Cell {
				o := in.proof.InputQueries[q][in.ob.treeOf[b]]
				if h-1 < len(o.Siblings) {
					return Cell(o.Siblings[h-1])
				}
				return Cell{}
			})
		}
		m.MerklePathDynamic(DynamicPath{
			Leaves: m.Table(leaves), Heights: ob.trees[b].heights, Order: ob.trees[b].order,
			Siblings: m.Table(siblings), Index: bitsQ.Shr[1], Root: ob.treeRoots[b],
		})
	}

	// the modules' DEEP terms, summed per round along π: deep[side][j]
	var deep [2][]int
	for side := range 2 {
		terms := make([]int, len(ob.mods))
		for k, om := range ob.mods {
			xs := x[k]
			if side == 1 {
				xs = m.Sub(ob.zero, xs)
			}
			terms[k] = ob.moduleValue(om, rows[k], side, xs)
		}
		termsT := m.Table(terms)
		prefix := make([]int, 1, len(ob.order)+1)
		prefix[0] = ob.zero
		for _, p := range ob.order {
			prefix = append(prefix, m.Add(prefix[len(prefix)-1], m.Lookup(termsT, p)))
		}
		prefixT := m.Table(prefix)
		for j := range R {
			deep[side] = append(deep[side], m.Sub(m.Lookup(prefixT, ob.starts[j+1]), m.Lookup(prefixT, ob.starts[j])))
		}
	}

	running := ob.runningLayers(q, bitsQ, R, L)
	rounds := make([]FoldRound, R)
	for j := range rounds {
		pair := [2]int{deep[0][j], deep[1][j]}
		if j > 0 {
			for side := range 2 {
				pair[side] = m.Add(m.Mul(ob.coef[j], running[j][side]), pair[side])
			}
		}
		target := ob.final
		if j+1 < R {
			target = running[j+1][0]
		}
		rounds[j] = FoldRound{P: pair[0], Q: pair[1], Alpha: ob.alphas[j], Target: target}
	}
	m.FoldChain(xInv, rounds)
}

// moduleValue returns a module's DEEP terms at a side of the query, at x:
//
//	Σ_s (Σ_g a^(p_g)·Σ_c a^(p(c) − p_g)·f_c(x) − V_s)/(x − z_s)
//
// the inner sums over the columns of group g opened at s (DeepHorner). The
// division binds its result only if x ≠ z_s, which outOfDomain ensures.
func (ob *obliviousPCS) moduleValue(om *obModule, rows [][2]rowCells, side, x int) int {
	m := ob.v.m
	res := -1
	for k, s := range om.shifts {
		h := -1
		for i, g := range om.groups {
			cols := groupColumns(ob.op.Groups[g.batch][g.group])
			top := topOpenedAt(cols, s)
			if top < 0 {
				continue
			}
			nb := len(ob.op.Groups[g.batch][g.group].Base)
			items := make([]deepItem, 0, top+1)
			for c := top; c >= 0; c-- {
				opened := slices.Contains(cols[c].Shifts, s)
				if c < nb {
					items = append(items, rows[i][side].item(c, false, opened))
				} else {
					items = append(items, rows[i][side].item(c-nb, true, opened))
				}
			}
			t := m.Mul(g.aPow, m.DeepHorner(om.a, deepChain(items)))
			if h < 0 {
				h = t
			} else {
				h = m.Add(h, t)
			}
		}
		term := m.Div(m.Sub(h, om.vz[k]), m.Sub(x, om.z[k]))
		if res < 0 {
			res = term
		} else {
			res = m.Add(res, term)
		}
	}
	return res
}

// obLayout is the layout of a proof's opening at its sizes: what the inputs
// read, and the order hints.
type obLayout struct {
	modOrder  []int // π: the positions of the modules by decreasing ℓ, then position
	starts    []int // per round j, the position in π of its first module
	treeOrder [][]int
	treeOf    []int // batch → its tree in the opening proof
	groups    [][]obGroupLoc
}

// obGroupLoc is where a group's rows are in its tree's opening: the level of
// its pair, and the offsets of its base and extension values in the rows.
type obGroupLoc struct {
	tree, level, base, ext int
}

// prepare computes the layout of an opening proof at the proof's sizes, and
// checks the proof's shape against it, as fri.PCS.Verify does before reading
// it. The sizes are checked by sizeInputs first.
func (ob *obliviousPCS) prepare(proof wiop.Proof) (*pcsInput, error) {
	op, p := ob.op, proof.PCSOpeningProof
	if p == nil {
		return nil, fmt.Errorf("no PCS opening proof")
	}
	logs := make([]int, len(ob.mods))
	for k, om := range ob.mods {
		var n int
		if om.mod.IsDynamic() {
			n = proof.DynamicSizes[om.index]
		} else {
			n = om.mod.Size()
		}
		logs[k] = bits.Len(uint(utils.NextPowerOfTwo(n) - 1))
		if logs[k] < op.MinSizeLog2 || logs[k] > op.MaxSizeLog2 {
			return nil, fmt.Errorf("module %q of size 2^%d, outside [2^%d, 2^%d]",
				om.mod.Context.Path(), logs[k], op.MinSizeLog2, op.MaxSizeLog2)
		}
	}
	lay := &obLayout{starts: make([]int, op.NumRounds)}
	lay.modOrder = canonical(logs)
	for j := range lay.starts {
		for _, l := range logs {
			if op.MaxSizeLog2-l < j {
				lay.starts[j]++
			}
		}
	}

	// the trees, by decreasing largest size, then batch (fri's
	// inputOpeningRoots), and the groups' rows in them
	type batchTree struct{ batch, top int }
	var order []batchTree
	for b, groups := range op.Groups {
		top := 0
		heights := make([]int, len(groups))
		for g, og := range groups {
			heights[g] = logs[ob.modPos[og.ModuleIndex]]
			top = max(top, heights[g])
		}
		lay.treeOrder = append(lay.treeOrder, canonical(heights))
		order = append(order, batchTree{b, top})
	}
	slices.SortStableFunc(order, func(x, y batchTree) int { return cmp.Compare(y.top, x.top) })
	lay.treeOf = make([]int, len(op.Groups))
	for t, bt := range order {
		lay.treeOf[bt.batch] = t
	}
	lay.groups = make([][]obGroupLoc, len(op.Groups))
	widths := make([]map[int]fri.SizedShape, len(op.Groups)) // per batch, size → widths
	for b, groups := range op.Groups {
		widths[b] = map[int]fri.SizedShape{}
		for _, og := range groups {
			l := logs[ob.modPos[og.ModuleIndex]]
			w := widths[b][l]
			lay.groups[b] = append(lay.groups[b], obGroupLoc{
				tree: lay.treeOf[b], level: l + op.RateLog - 1, base: w.BaseWidth, ext: w.ExtWidth,
			})
			w.BaseWidth += len(og.Base)
			w.ExtWidth += len(og.Ext)
			widths[b][l] = w
		}
	}

	if err := ob.checkShape(p, widths, lay); err != nil {
		return nil, err
	}
	return &pcsInput{proof: p, ob: lay}, nil
}

// canonical returns the indices of heights by decreasing height, equal ones
// by increasing index: the order AssertCanonicalOrder checks.
func canonical(heights []int) []int {
	res := make([]int, len(heights))
	for i := range res {
		res[i] = i
	}
	slices.SortStableFunc(res, func(a, b int) int { return cmp.Compare(heights[b], heights[a]) })
	return res
}

// checkShape checks an opening proof has the shape of the layout.
func (ob *obliviousPCS) checkShape(p *fri.OpeningProof, widths []map[int]fri.SizedShape, lay *obLayout) error {
	op := ob.op
	if err := checkFRIShape(p, op.NumRounds, op.NumQueries, len(op.Groups)); err != nil {
		return err
	}
	for q := range op.NumQueries {
		if len(p.InputQueries[q]) != len(op.Groups) {
			return fmt.Errorf("query %d: %d input tree openings, want %d", q, len(p.InputQueries[q]), len(op.Groups))
		}
		for b := range op.Groups {
			t := lay.treeOf[b]
			o := p.InputQueries[q][t]
			top := 0
			for l := range widths[b] {
				top = max(top, l)
			}
			height := top + op.RateLog
			if len(o.Leaves) != height || len(o.Siblings) != height-1 {
				return fmt.Errorf("query %d input tree %d: %d row levels and %d siblings, want %d and %d",
					q, t, len(o.Leaves), len(o.Siblings), height, height-1)
			}
			for level, pair := range o.Leaves {
				w, ok := widths[b][level-op.RateLog+1]
				if (pair != nil) != ok {
					return fmt.Errorf("query %d input tree %d: unexpected row pair at level %d", q, t, level)
				}
				if !ok {
					continue
				}
				for side, row := range pair {
					if len(row.Base) != w.BaseWidth || len(row.Ext) != w.ExtWidth {
						return fmt.Errorf("query %d input tree %d level %d: row %d has the wrong shape", q, t, level, side)
					}
				}
			}
		}
		if err := checkRunningShape(p, q, op.NumRounds, op.LogCodewordSize); err != nil {
			return fmt.Errorf("query %d: %w", q, err)
		}
	}
	return nil
}
