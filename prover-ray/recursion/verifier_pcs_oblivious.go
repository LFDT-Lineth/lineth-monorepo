package recursion

import (
	"fmt"
	"math/bits"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// PROTOTYPE (#4405). The PCS opening of a system compiled
// pcs.WithSizeOblivious, in the machine, with the modules' sizes as data: the
// log size ℓ of a module is a cell (a dynamic module's input, or a sized
// module's VerifierOptions.Heights cell; a constant otherwise), and the
// machine does the same work whatever its value.
//
// With D the log of the largest committable size, 2^r the blowup and
// L = D + r, a module of log size ℓ:
//
//   - is one group of every batch it has columns in: its row pair at a query
//     is hashed on its own, even row first, and enters the batch's tree at
//     height ℓ + r − 1 (MerklePathDynamic). The pair's parity is bit D − ℓ of
//     the query position;
//   - joins FRI at round j = D − ℓ, with a = α_j², at the query point
//     x_j = x^(2^j). Its value there is
//     T = Σ_z (Σ_c a^(p(c))·f_c(x_j) − Σ_c a^(p(c))·y_(c,z)) / (x_j − z),
//     over its columns c of every batch, z = ζ·ω_(2^ℓ)^shift.
//
// FRI always has D rounds. At round j, the pair folded is the running
// layer's (R, R'), or, if modules join at j, a^G·(R, R') + Σ (T, T') over
// them, G the number of committed columns. Which modules join at j is data:
// the modules' values are read in the order of their rounds (a permutation
// the prover gives, checked by AssertCanonicalOrder), summed up as prefix
// sums, and the sum of a round is the difference of the prefix sums at its
// bounds, the numbers of modules joining before j and before j + 1, which are
// computed once per proof.
//
// Not supported: Merkle caps, column elision, the BN-packed hash, a final
// polynomial of more than one coefficient.
type oblVerifier struct {
	v  *Verifier
	op *pcs.ObliviousOpening
	h  *pcsVerifier // for the helpers shared with the static opening

	D, r, L, H int // H = D + r − 1, the largest group height
	zero, one  int

	alphas    []int // fold challenges, one per round
	roots     []int // the running layers' roots, rounds 1 … D − 1
	final     int
	treeRoots []int // per batch
	mods      []*oblModule
	modOf     map[*wiop.Module]*oblModule
	batches   []oblBatch
	orderAll  []int // the modules by increasing round: input cells
	start     []int // start[j]: the number of modules joining before round j, j = 0 … D
	weight    []int // weight[j]: a_j^G if a module joins at round j, 1 otherwise
}

// oblModule is a module with committed columns.
type oblModule struct {
	mod    *wiop.Module
	log    int // the cell of ℓ
	round  int // D − ℓ
	height int // ℓ + r − 1
	a      int // its alphaDeep
	parts  []oblPart
	shifts []int // the shifts its columns are opened at
	z, w   []int // per shift: z, and W = Σ_parts a^power·V
}

// oblPart is a module's group in a batch, with a^power for the group's first
// power.
type oblPart struct {
	batch, group int
	a0           int
}

// oblBatch is a batch's tree: its groups' heights and canonical order.
type oblBatch struct {
	heights, order Table
	orderCells     []int
}

func (v *Verifier) checkPCSOblivious(a *pcs.OpeningVerifierAction) error {
	op, err := a.Oblivious(v.sys)
	if err != nil {
		return err
	}
	ov := &oblVerifier{
		v: v, op: op, modOf: map[*wiop.Module]*oblModule{},
		D: op.MaxSizeLog2, r: op.LogBlowup,
	}
	ov.L, ov.H = ov.D+ov.r, ov.D+ov.r-1
	ov.zero, ov.one = v.scalar(field.Zero()), v.scalar(field.One())
	ov.h = &pcsVerifier{v: v, zero: ov.zero, one: ov.one}
	switch {
	case ov.D < 2:
		return fmt.Errorf("PCS opening: %d FRI rounds, want at least 2", ov.D)
	case ov.L >= BitsWidth:
		return fmt.Errorf("PCS opening: a codeword of size 2^%d is too large", ov.L)
	case ov.r < 1:
		return fmt.Errorf("PCS opening: blowup 2^%d", ov.r)
	}
	v.obl = ov

	// claims: every manifest says Present, repeated openings agree
	for _, c := range op.Manifests {
		v.assertEq(v.cell(c), ov.zero)
	}
	for _, d := range op.Duplicates {
		v.assertEq(v.cell(d[0]), v.cell(d[1]))
	}
	zeta, err := ov.zeta()
	if err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	ov.outOfDomain(zeta)
	positions := ov.transcript()
	ov.modules(zeta)
	ov.trees()
	ov.rounds()
	for q, pos := range positions {
		ov.query(q, pos)
	}
	return nil
}

func (ov *oblVerifier) input(kind int, value func(*oblAssign) Cell) int {
	a := ov.v.m.input(kind)
	ov.v.inputs = append(ov.v.inputs, verifierInput{pub: -1, obl: value})
	return a
}

// zeta returns ζ, the LagrangeEvals' evaluation point, and checks they share
// it.
func (ov *oblVerifier) zeta() (int, error) {
	ev := &evaluator{v: ov.v, r: -1, memo: map[wiop.Expression]int{}}
	res := -1
	for _, p := range ov.op.EvalPoints {
		z, err := ev.eval(p)
		if err != nil {
			return 0, fmt.Errorf("evaluation point: %w", err)
		}
		if res < 0 {
			res = z
		} else if z != res {
			ov.v.assertEq(z, res)
		}
	}
	return res, nil
}

// outOfDomain checks ζ^(2^L) ≠ 1: no claim point is in a codeword domain.
func (ov *oblVerifier) outOfDomain(zeta int) {
	m := ov.v.m
	zN := zeta
	for range ov.L {
		zN = m.Mul(zN, zN)
	}
	m.Div(ov.one, m.Sub(zN, ov.one))
}

// transcript continues the inner transcript as the PCS's verifier does with a
// fixed FRI domain, and returns the query positions' cells.
func (ov *oblVerifier) transcript() []int {
	t, m := ov.v.t, ov.v.m
	for j := range ov.D {
		ov.alphas = append(ov.alphas, m.Lanes(t.squeeze(), 0, KindE6))
		if j == ov.D-1 {
			break
		}
		root := ov.input(KindDigest, func(a *oblAssign) Cell { return Cell(a.p.FRIProof.RoundRoots[j]) })
		ov.roots = append(ov.roots, root)
		t.absorb(root, KindDigest)
	}
	ov.final = ov.input(KindE6, func(a *oblAssign) Cell { return E6Cell(a.p.FRIProof.FinalPoly[0]) })
	t.absorb(ov.final, KindE6)
	positions := make([]int, 0, ov.op.NumQueries)
	var d int
	for k := range ov.op.NumQueries {
		if k%CellWidth == 0 {
			d = t.squeeze()
		}
		positions = append(positions, m.Lanes(d, k%CellWidth, KindScalar))
	}
	for _, b := range ov.op.Batches {
		switch {
		case !b.IsPrecomp:
			ov.treeRoots = append(ov.treeRoots, ov.v.commitments[b.Round.ID])
		case ov.v.opts.HasKey:
			ov.treeRoots = append(ov.treeRoots, ov.v.opts.Key)
		default:
			ov.treeRoots = append(ov.treeRoots, m.Const(Cell(ov.op.PrecomputedRoot)))
		}
	}
	return positions
}

// logCell returns the cell of a module's log size: data, or a constant.
func (ov *oblVerifier) logCell(mod *wiop.Module) int {
	if d := ov.v.dyn(mod); d != nil {
		return d.log
	}
	return ov.v.scalarInt(bits.TrailingZeros(uint(mod.Size())))
}

// modules computes, once per proof, what depends on a module's size but not
// on the query.
func (ov *oblVerifier) modules(zeta int) {
	v, m, op := ov.v, ov.v.m, ov.op
	alpha2 := make([]int, ov.D)
	for j, al := range ov.alphas {
		alpha2[j] = m.Mul(al, al)
	}
	alpha2Tbl := m.Table(alpha2)

	for _, mod := range v.sys.Modules {
		var parts []oblPart
		for b := range op.Groups {
			for gi, g := range op.Groups[b] {
				if g.Module == mod {
					parts = append(parts, oblPart{batch: b, group: gi})
				}
			}
		}
		if len(parts) == 0 {
			continue
		}
		om := &oblModule{mod: mod, log: ov.logCell(mod)}
		// The lookups at the round bound ℓ: 0 ≤ D − ℓ ≤ D − 1.
		om.round = m.Sub(v.scalarInt(ov.D), om.log)
		om.height = om.log
		if ov.r > 1 {
			om.height = m.Add(om.log, v.scalarInt(ov.r-1))
		}
		om.a = m.Lookup(alpha2Tbl, om.round)
		for i := range parts {
			g := op.Groups[parts[i].batch][parts[i].group]
			parts[i].a0 = ov.h.pow(om.a, g.Power)
			for _, col := range slices.Concat(g.Base, g.Ext) {
				for _, s := range col.Shifts {
					if !slices.Contains(om.shifts, s) {
						om.shifts = append(om.shifts, s)
					}
				}
			}
		}
		om.parts = parts
		slices.Sort(om.shifts)
		for _, s := range om.shifts {
			rot := v.sizeConstAtLeast(mod, 0, rootKey(s), func(n int) field.Element { return root(n, s) })
			om.z = append(om.z, m.Mul(zeta, rot))
			var a0s, vs []int
			for _, p := range parts {
				coeffs := ov.claims(op.Groups[p.batch][p.group], s)
				if coeffs == nil {
					continue
				}
				a0s, vs = append(a0s, p.a0), append(vs, m.Horner(om.a, coeffs))
			}
			sums := m.MulAddChain(ov.zero, a0s, vs)
			om.w = append(om.w, sums[len(sums)-1])
		}
		ov.modOf[mod] = om
		ov.mods = append(ov.mods, om)
	}
}

// groupColumn returns the column of local power lp of a group (its base
// columns, then its extension ones) and its row among its kind.
func groupColumn(g pcs.ObliviousGroup, lp int) (col pcs.ObliviousColumn, row int, ext bool) {
	if lp < len(g.Base) {
		return g.Base[lp], lp, false
	}
	return g.Ext[lp-len(g.Base)], lp - len(g.Base), true
}

// claims returns the claims of a group's columns at the shift s as the
// coefficients of V = Σ_c a^(lp(c))·y_(c,s), from the highest local power
// opened at s down to 0 (zero for a column not opened at s), nil if none is.
func (ov *oblVerifier) claims(g pcs.ObliviousGroup, s int) []int {
	var coeffs []int
	for lp := len(g.Base) + len(g.Ext) - 1; lp >= 0; lp-- {
		col, _, _ := groupColumn(g, lp)
		k := slices.Index(col.Shifts, s)
		switch {
		case k >= 0:
			coeffs = append(coeffs, ov.v.cell(col.Claims[k]))
		case coeffs != nil:
			coeffs = append(coeffs, ov.zero)
		}
	}
	return coeffs
}

// trees builds, per batch, the table of its groups' heights and their
// canonical order, given by the prover and checked, and the same for all the
// modules together: the order of their rounds.
func (ov *oblVerifier) trees() {
	m := ov.v.m
	order := func(n int, of func(*oblAssign) []int) []int {
		cells := make([]int, n)
		for k := range cells {
			cells[k] = ov.input(KindScalar, func(a *oblAssign) Cell {
				return ScalarCell(field.NewElement(uint64(of(a)[k])))
			})
		}
		return cells
	}
	for b, groups := range ov.op.Groups {
		heights := make([]int, len(groups))
		for gi, g := range groups {
			heights[gi] = ov.modOf[g.Module].height
		}
		ob := oblBatch{heights: m.Table(heights)}
		ob.orderCells = order(len(groups), func(a *oblAssign) []int { return a.orders[b] })
		ob.order = m.Table(ob.orderCells)
		m.AssertCanonicalOrder(ob.order, ob.heights, ov.H)
		ov.batches = append(ov.batches, ob)
	}
	heights := make([]int, len(ov.mods))
	for i, om := range ov.mods {
		heights[i] = om.height
	}
	ov.orderAll = order(len(ov.mods), func(a *oblAssign) []int { return a.orderAll })
	m.AssertCanonicalOrder(m.Table(ov.orderAll), m.Table(heights), ov.H)
}

// rounds computes, per round j, start[j], the number of modules joining
// before j, and the running layer's weight: a_j^G if a module joins at j, 1
// otherwise.
func (ov *oblVerifier) rounds() {
	v, m, M := ov.v, ov.v.m, len(ov.mods)
	ov.start = make([]int, ov.D+1)
	ov.start[0], ov.start[ov.D] = ov.zero, v.scalarInt(M)
	for j := 1; j < ov.D; j++ {
		// below[i] = 1 if i < j: a module of round i joins before j
		below := make([]int, ov.D)
		for i := range below {
			below[i] = ov.zero
			if i < j {
				below[i] = ov.one
			}
		}
		t := m.Table(below)
		sum := -1
		for _, om := range ov.mods {
			f := m.Lookup(t, om.round)
			if sum < 0 {
				sum = f
			} else {
				sum = m.Add(sum, f)
			}
		}
		ov.start[j] = sum
	}
	nonzero := make([]int, M+1)
	for i := range nonzero {
		nonzero[i] = ov.one
	}
	nonzero[0] = ov.zero
	nz := m.Table(nonzero)
	ov.weight = make([]int, ov.D)
	for j := range ov.weight {
		has := m.Lookup(nz, m.Sub(ov.start[j+1], ov.start[j]))
		aG := ov.h.pow(m.Mul(ov.alphas[j], ov.alphas[j]), ov.op.NumColumns)
		ov.weight[j] = m.Select(has, aG, ov.one)
	}
}

// rowInputs allocates the input cells of one side of a group's row pair at a
// query: its base values, eight per cell, and its extension values.
func (ov *oblVerifier) rowInputs(q, b, gi, side int, g pcs.ObliviousGroup) rowCells {
	var rc rowCells
	for k := 0; k < len(g.Base); k += CellWidth {
		rc.base = append(rc.base, ov.input(KindDigest, func(a *oblAssign) Cell {
			var c Cell
			copy(c[:], a.row(q, b, gi, side).Base[k:])
			return c
		}))
	}
	for k := range g.Ext {
		rc.ext = append(rc.ext, ov.input(KindE6, func(a *oblAssign) Cell {
			return E6Cell(a.row(q, b, gi, side).Ext[k])
		}))
	}
	return rc
}

// chain returns the DEEP Horner chain of a group's columns opened at the
// shift s, on the row rc: Σ_c a^(lp(c))·f_c, nil if none is opened at s.
func (ov *oblVerifier) chain(g pcs.ObliviousGroup, rc rowCells, s int) []DeepRow {
	var chain []DeepRow
	started := false
	for lp := len(g.Base) + len(g.Ext) - 1; lp >= 0; lp-- {
		col, row, ext := groupColumn(g, lp)
		opened := slices.Contains(col.Shifts, s)
		if !started && !opened {
			continue
		}
		started = true
		sel := uint8(0)
		if opened {
			sel = 1
		}
		if ext {
			chain = append(chain, DeepRow{Cell: rc.ext[row], Ext: true, Act: 1, Sel: sel})
			continue
		}
		cell, lane := rc.base[row/CellWidth], row%CellWidth
		// extend the previous row if this value is its cell's next lane down
		if n := len(chain); n > 0 && !chain[n-1].Ext && chain[n-1].Cell == cell &&
			chain[n-1].Act&(1<<(lane+1)) != 0 && chain[n-1].Act&(1<<(lane+1)-1) == 0 {
			chain[n-1].Act |= 1 << lane
			chain[n-1].Sel |= sel << lane
			continue
		}
		chain = append(chain, DeepRow{Cell: cell, Act: 1 << lane, Sel: sel << lane})
	}
	return chain
}

// query checks one FRI query, at the position in the lane cell pos.
func (ov *oblVerifier) query(q, pos int) {
	v, m, op := ov.v, ov.v.m, ov.op
	D, L, M := ov.D, ov.L, len(ov.mods)
	pb := m.Bits(pos, L)
	bitTbl := m.Table(pb.Bit[:D])

	// running layers: running[j] = (value at the query point, at its
	// conjugate), 1 ≤ j < D; zero at round 0. The path of layer 1 gives
	// g^(−bitrev(pos >> 1)), g the generator of the top domain: x⁻¹ up to
	// the sign that bit 0 of the position carries.
	gInv := pcs.FRIStaticParams(v.sys).FullDomainGenerator()
	gInv.Inverse(&gInv)
	running := make([][2]int, D)
	running[0] = [2]int{ov.zero, ov.zero}
	var xInv int
	for j := 1; j < D; j++ {
		self := ov.input(KindE6, func(a *oblAssign) Cell { return Cell(a.p.FRIProof.RunningQueries[q][j-1].Leaf) })
		height := L - j
		siblings := make([]int, height)
		for i := range height {
			kind := KindDigest
			if i == 0 {
				kind = KindE6
			}
			siblings[i] = ov.input(kind, func(a *oblAssign) Cell {
				return Cell(a.p.FRIProof.RunningQueries[q][j-1].Siblings[height-1-i])
			})
		}
		if j == 1 {
			y := m.Lanes(m.MerklePathX(self, siblings, pb.Shr[1], ov.roots[0], gInv), 0, KindScalar)
			xInv = m.Select(pb.Bit[0], m.Sub(ov.zero, y), y)
		} else {
			m.MerklePath(self, siblings, pb.Shr[j], ov.roots[j-1])
		}
		running[j] = [2]int{self, siblings[0]}
	}
	// x⁻¹ at every round
	xis := make([]int, D)
	xis[0] = xInv
	for j := 1; j < D; j++ {
		xis[j] = m.Mul(xis[j-1], xis[j-1])
	}
	xiTbl := m.Table(xis)

	// per module: the parity of its row pair, and its query point
	parity := make([]int, M)
	xs := make([][2]int, M)
	index := map[*oblModule]int{}
	for i, om := range ov.mods {
		index[om] = i
		parity[i] = m.Lookup(bitTbl, om.round)
		x := m.Div(ov.one, m.Lookup(xiTbl, om.round))
		xs[i] = [2]int{x, m.Sub(ov.zero, x)}
	}

	// input trees: rows[b][gi] = the group's pair
	rows := make([][][2]rowCells, len(op.Groups))
	for b, groups := range op.Groups {
		rows[b] = make([][2]rowCells, len(groups))
		leaves := make([]int, len(groups))
		for gi, g := range groups {
			pair := [2]rowCells{ov.rowInputs(q, b, gi, 0, g), ov.rowInputs(q, b, gi, 1, g)}
			rows[b][gi] = pair
			bit := parity[index[ov.modOf[g.Module]]]
			even, odd := ov.h.selectRow(bit, pair[1], pair[0]), ov.h.selectRow(bit, pair[0], pair[1])
			leaves[gi] = ov.h.hashLeaf(fri.SizedShape{BaseWidth: len(g.Base), ExtWidth: len(g.Ext)}, even, odd)
		}
		siblings := make([]int, ov.H)
		for h := range siblings {
			siblings[h] = ov.input(KindDigest, func(a *oblAssign) Cell { return a.sibling(q, b, h) })
			ov.v.inputs[len(ov.v.inputs)-1].free = func(a *oblAssign) bool {
				return h >= len(a.p.InputQueries[q][a.tree[b]].Siblings)
			}
		}
		m.MerklePathDynamic(DynamicPath{
			Leaves: m.Table(leaves), Heights: ov.batches[b].heights, Order: ov.batches[b].order,
			Siblings: m.Table(siblings), Index: pb.Shr[L-ov.H], Root: ov.treeRoots[b],
		})
	}

	// per module and side: its value at its round
	vals := [2][]int{make([]int, M), make([]int, M)}
	for i, om := range ov.mods {
		for side := range 2 {
			res := -1
			for k, s := range om.shifts {
				var a0s, hs []int
				for _, p := range om.parts {
					chain := ov.chain(op.Groups[p.batch][p.group], rows[p.batch][p.group][side], s)
					if chain == nil {
						continue
					}
					a0s, hs = append(a0s, p.a0), append(hs, m.DeepHorner(om.a, chain))
				}
				sums := m.MulAddChain(ov.zero, a0s, hs)
				term := m.Div(m.Sub(sums[len(sums)-1], om.w[k]), m.Sub(xs[i][side], om.z[k]))
				if res < 0 {
					res = term
				} else {
					res = m.Add(res, term)
				}
			}
			vals[side][i] = res
		}
	}

	// per round and side: the sum of the modules joining at the round
	var sums [2][]int
	for side := range 2 {
		t := m.Table(vals[side])
		prefix := make([]int, M+1)
		prefix[0] = ov.zero
		for k := range M {
			prefix[k+1] = m.Add(prefix[k], m.Lookup(t, ov.orderAll[k]))
		}
		pt := m.Table(prefix)
		at := make([]int, D+1)
		at[0], at[D] = prefix[0], prefix[M]
		for j := 1; j < D; j++ {
			at[j] = m.Lookup(pt, ov.start[j])
		}
		sums[side] = make([]int, D)
		for j := range D {
			sums[side][j] = m.Sub(at[j+1], at[j])
		}
	}

	rounds := make([]FoldRound, D)
	for j := range rounds {
		pair := [2]int{sums[0][j], sums[1][j]}
		if j > 0 {
			for side := range 2 {
				pair[side] = m.Add(m.Mul(ov.weight[j], running[j][side]), pair[side])
			}
		}
		target := ov.final
		if j+1 < D {
			target = running[j+1][0]
		}
		rounds[j] = FoldRound{P: pair[0], Q: pair[1], Alpha: ov.alphas[j], Target: target}
	}
	m.FoldChain(xInv, rounds)
}

// oblAssign lays a proof out for the inputs of the size-oblivious opening: it
// holds what depends on the proof's sizes.
type oblAssign struct {
	ov   *oblVerifier
	p    *fri.OpeningProof
	tree []int   // batch → its input tree in the proof
	glog [][]int // [batch][group] → log size
	// off[batch][group] is where the group's rows start in the row pair of
	// its size: its base values and its extension values.
	off      [][][2]int
	orders   [][]int
	orderAll []int
}

// row returns a side of a group's row pair at a query.
func (a *oblAssign) row(q, b, gi, side int) fri.RowOpening {
	g := a.ov.op.Groups[b][gi]
	pair := a.p.InputQueries[q][a.tree[b]].Leaves[a.ov.r+a.glog[b][gi]-1]
	row, off := pair[side], a.off[b][gi]
	return fri.RowOpening{Base: row.Base[off[0] : off[0]+len(g.Base)], Ext: row.Ext[off[1] : off[1]+len(g.Ext)]}
}

// sibling returns the sibling at height h + 1 of a batch's tree at a query,
// zero above the tree's height.
func (a *oblAssign) sibling(q, b, h int) Cell {
	s := a.p.InputQueries[q][a.tree[b]].Siblings
	if h >= len(s) {
		return Cell{}
	}
	return Cell(s[h])
}

// decreasingOrder returns the indices of logs by decreasing value, equal ones
// by increasing index.
func decreasingOrder(logs []int) []int {
	order := make([]int, len(logs))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int { return logs[j] - logs[i] })
	return order
}

// assign checks that an opening proof of sys has the shape the machine
// verifies for the sizes of sys and of the proof, and lays it out.
func (ov *oblVerifier) assign(sys *wiop.System, proof wiop.Proof) (*oblAssign, error) {
	p := proof.PCSOpeningProof
	if p == nil {
		return nil, fmt.Errorf("no PCS opening proof")
	}
	op, D, r, L := ov.op, ov.D, ov.r, ov.L
	// the log size of every module, by module index
	logOf := func(mod *wiop.Module) (int, error) {
		for k, vm := range ov.v.sys.Modules {
			if vm != mod {
				continue
			}
			var n int
			if vm.IsDynamic() {
				n = proof.DynamicSizes[k]
			} else {
				n = sys.Modules[k].Size()
			}
			if n < 2 || n&(n-1) != 0 || n > 1<<D {
				return 0, fmt.Errorf("module %q has size %d", sys.Modules[k].Context.Path(), n)
			}
			return bits.TrailingZeros(uint(n)), nil
		}
		return 0, fmt.Errorf("unknown module")
	}
	a := &oblAssign{ov: ov, p: p, tree: make([]int, len(op.Groups))}
	maxLog := make([]int, len(op.Groups))
	for b, groups := range op.Groups {
		logs := make([]int, len(groups))
		offs := make([][2]int, len(groups))
		next := map[int][2]int{}
		for gi, g := range groups {
			l, err := logOf(g.Module)
			if err != nil {
				return nil, err
			}
			logs[gi], offs[gi] = l, next[l]
			next[l] = [2]int{offs[gi][0] + len(g.Base), offs[gi][1] + len(g.Ext)}
			maxLog[b] = max(maxLog[b], l)
		}
		a.glog = append(a.glog, logs)
		a.off = append(a.off, offs)
		a.orders = append(a.orders, decreasingOrder(logs))
	}
	var all []int
	for _, om := range ov.mods {
		l, err := logOf(om.mod)
		if err != nil {
			return nil, err
		}
		all = append(all, l)
	}
	a.orderAll = decreasingOrder(all)
	// the input trees are in order of their largest size, then of batch
	for t, b := range decreasingOrder(maxLog) {
		a.tree[b] = t
	}

	f := p.FRIProof
	if len(f.RoundRoots) != D-1 || len(f.FinalPoly) != 1 {
		return nil, fmt.Errorf("%d round roots and a final polynomial of %d coefficients, want %d and 1",
			len(f.RoundRoots), len(f.FinalPoly), D-1)
	}
	if len(f.RunningQueries) != op.NumQueries || len(p.InputQueries) != op.NumQueries {
		return nil, fmt.Errorf("%d running and %d input queries, want %d",
			len(f.RunningQueries), len(p.InputQueries), op.NumQueries)
	}
	for q := range op.NumQueries {
		if len(f.RunningQueries[q]) != D-1 {
			return nil, fmt.Errorf("query %d: %d running layers, want %d", q, len(f.RunningQueries[q]), D-1)
		}
		for i, br := range f.RunningQueries[q] {
			if len(br.Siblings) != L-i-1 {
				return nil, fmt.Errorf("query %d round %d: %d siblings, want %d", q, i+1, len(br.Siblings), L-i-1)
			}
		}
		if len(p.InputQueries[q]) != len(op.Groups) {
			return nil, fmt.Errorf("query %d: %d input trees, want %d", q, len(p.InputQueries[q]), len(op.Groups))
		}
		for b, groups := range op.Groups {
			o := p.InputQueries[q][a.tree[b]]
			if len(o.Leaves) != r+maxLog[b] || len(o.Siblings) != r+maxLog[b]-1 {
				return nil, fmt.Errorf("query %d batch %d: %d row levels and %d siblings, want %d and %d",
					q, b, len(o.Leaves), len(o.Siblings), r+maxLog[b], r+maxLog[b]-1)
			}
			width := map[int][2]int{}
			for gi, g := range groups {
				w := width[a.glog[b][gi]]
				width[a.glog[b][gi]] = [2]int{w[0] + len(g.Base), w[1] + len(g.Ext)}
			}
			for l, w := range width {
				pair := o.Leaves[r+l-1]
				if pair == nil {
					return nil, fmt.Errorf("query %d batch %d: no row pair of size 2^%d", q, b, l)
				}
				for side := range 2 {
					if len(pair[side].Base) != w[0] || len(pair[side].Ext) != w[1] {
						return nil, fmt.Errorf("query %d batch %d size 2^%d: a row of the wrong shape", q, b, l)
					}
				}
			}
		}
	}
	return a, nil
}
