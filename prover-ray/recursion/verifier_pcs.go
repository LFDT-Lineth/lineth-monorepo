package recursion

import (
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// leafDomainTag is fri's Merkle leaf domain separator ("Lfri_lf1"), the first
// element every leaf hash absorbs, before the row's widths.
const leafDomainTag uint64 = 0x4c66_7269_5f6c_6631

// The PCS opening (pcs.OpeningVerifierAction, fri.PCS.Verify), in the
// machine. Its structure is static (pcs.StaticOpening, fri.VerifierPlan),
// so the machine checks one proof shape:
//
//  1. Transcript: a fold challenge per FRI round, each intermediate layer's
//     root absorbed after its challenge; then the final polynomial, and the
//     query positions, eight per challenge (one per lane, reduced to the
//     domain's bits).
//  2. Per query, the input trees: every opened row pair is an input, hashed
//     as fri's leaves are (MDHash of the leaf header and the row), and the
//     path from the largest size's row to the batch's root is checked, the
//     smaller sizes' pairs injected at their levels (MerklePath). The path of
//     a tree of the largest size gives the query point's inverse x⁻¹
//     (MerklePathX).
//  3. Per query, the running layers: each one's pair, and its path to the
//     layer's root.
//  4. Per query and level, the level's value at the query point and its
//     conjugate: a^R·running + Σ_z (Σ_c a^(p(c))·f_c(x) − V_z)/(x − z), the
//     inner sums DEEP Horner chains over the opened rows and V_z computed
//     once per proof (see DeepHorner).
//  5. Per query, the folds (FoldChain), from the top level to the final
//     polynomial, a constant. A level joining at the boundary round, after
//     the last fold, is not folded: its value at the query point and at its
//     conjugate must be equal.
//
// Not supported, and rejected when the verifier is built: Merkle caps,
// module groups (grouped commitments), column elision, a fixed FRI domain,
// the BN-packed Merkle hash, a final polynomial of more than one coefficient,
// and dynamic modules.

// pcsVerifier is the in-machine PCS opening of one system, and the layout of
// its proof inputs, which Inputs checks a proof against.
type pcsVerifier struct {
	v    *Verifier
	op   *pcs.StaticOpening
	plan fri.VerifierPlan

	zero, one int
	alphas    []int // fold challenges, one per round
	roots     []int // the running layers' roots, rounds 1 … NumRounds − 1
	final     int   // the final polynomial, a constant
	treeRoots []int // the input trees' roots
	levels    []pcsLevel
	xTree     int // a tree of the largest size, whose path gives x⁻¹
}

// pcsLevel is the query-independent part of a level: its alphaDeep a, a^R,
// and per claim point z (a shift of the level's columns), z and V_z.
type pcsLevel struct {
	a, aR  int
	shifts []int // the distinct shifts, increasing
	z, vz  []int // per shift
	// order lists the level's entries by decreasing power; top[k] is the
	// index in order of the first entry opened at shifts[k].
	order []int
	top   []int
}

// rowCells are the input cells of one opened row: its base values, eight per
// cell (the last one partial), and its extension values.
type rowCells struct {
	base, ext []int
}

func (v *Verifier) checkPCS(a *pcs.OpeningVerifierAction) error {
	op, err := a.Static(v.sys)
	if err != nil {
		return err
	}
	pv := &pcsVerifier{v: v, op: op, plan: op.Plan}
	if err := pv.supported(); err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	pv.zero, pv.one = v.scalar(field.Zero()), v.scalar(field.One())
	pv.claims()
	zeta, err := pv.zeta()
	if err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	positions := pv.transcript()
	if err := pv.levelConstants(zeta); err != nil {
		return fmt.Errorf("PCS opening: %w", err)
	}
	for q, pos := range positions {
		pv.query(q, pos)
	}
	v.pcs = pv
	return nil
}

// supported rejects what the machine does not verify yet.
func (pv *pcsVerifier) supported() error {
	sys, plan := pv.v.sys, pv.plan
	switch {
	case pcs.MerkleHash(sys) != fri.MerkleHashKoalaBear:
		return fmt.Errorf("the Merkle hash %v is not supported", pcs.MerkleHash(sys))
	case pcs.FRIFixedDomain(sys):
		return fmt.Errorf("a fixed FRI domain is not supported")
	case plan.Params.MerkleCapHeight != 0:
		return fmt.Errorf("the Merkle caps are not supported")
	case plan.NumRounds < 1:
		return fmt.Errorf("an opening without FRI fold is not supported")
	case plan.LogFinalPolySize != 0:
		return fmt.Errorf("a final polynomial of 2^%d coefficients is not supported", plan.LogFinalPolySize)
	case plan.LogCodewordSize >= BitsWidth:
		return fmt.Errorf("a codeword of size 2^%d is too large", plan.LogCodewordSize)
	case pv.op.QueryRange != 1<<plan.LogCodewordSize:
		return fmt.Errorf("query positions below %d, for a codeword of size 2^%d",
			pv.op.QueryRange, plan.LogCodewordSize)
	case len(plan.Levels) == 0 || plan.Levels[0].Round != 0:
		return fmt.Errorf("no level at the first FRI round")
	}
	pv.xTree = -1
	for t, tree := range plan.Trees {
		if tree.Grouped || tree.CapDepth != 0 {
			return fmt.Errorf("input tree %d: grouped or capped trees are not supported", t)
		}
		if pv.xTree < 0 && tree.Height == plan.LogCodewordSize {
			pv.xTree = t
		}
	}
	if pv.xTree < 0 {
		return fmt.Errorf("no input tree of the largest size")
	}
	for i, l := range plan.Levels {
		// ungrouped powers: the level's columns are numbered 0 … C − 1
		powers := make([]bool, len(l.Entries))
		for _, e := range l.Entries {
			if e.Power < 0 || e.Power >= len(powers) || powers[e.Power] {
				return fmt.Errorf("level %d: column powers are not 0 … %d", i, len(powers)-1)
			}
			powers[e.Power] = true
		}
	}
	return nil
}

// claims checks what RecoverBatchClaims checks: every manifest says Present
// (no elision), and repeated openings of a column at a shift agree.
func (pv *pcsVerifier) claims() {
	v := pv.v
	for _, c := range pv.op.Manifests {
		v.assertEq(v.cell(c), pv.zero)
	}
	for _, d := range pv.op.Duplicates {
		v.assertEq(v.cell(d[0]), v.cell(d[1]))
	}
}

// zeta returns ζ, the LagrangeEvals' evaluation point, and checks they share
// it.
func (pv *pcsVerifier) zeta() (int, error) {
	ev := &evaluator{v: pv.v, r: -1, memo: map[wiop.Expression]int{}, pows: map[int]int{}, anns: map[int]int{}}
	res := -1
	for _, p := range pv.op.EvalPoints {
		z, err := ev.eval(p)
		if err != nil {
			return 0, fmt.Errorf("evaluation point: %w", err)
		}
		if res < 0 {
			res = z
		} else if z != res {
			pv.v.assertEq(z, res)
		}
	}
	return res, nil
}

// input allocates a proof input of the opening, whose value value reads from
// the opening proof (checked against the plan's shape by Inputs first).
func (pv *pcsVerifier) input(kind int, value func(*fri.OpeningProof) Cell) int {
	a := pv.v.m.input(kind)
	pv.v.inputs = append(pv.v.inputs, verifierInput{pub: -1, pcs: value})
	return a
}

// transcript continues the inner system's transcript as compiled.verify
// does, and returns the query positions' cells (lanes of challenges, not yet
// reduced).
func (pv *pcsVerifier) transcript() []int {
	t, m, plan := pv.v.t, pv.v.m, pv.plan
	for j := range plan.NumRounds {
		pv.alphas = append(pv.alphas, m.Lanes(t.squeeze(), 0, KindE6))
		if j == plan.NumRounds-1 {
			break
		}
		root := pv.input(KindDigest, func(p *fri.OpeningProof) Cell { return Cell(p.FRIProof.RoundRoots[j]) })
		pv.roots = append(pv.roots, root)
		t.absorb(root, KindDigest)
	}
	pv.final = pv.input(KindE6, func(p *fri.OpeningProof) Cell { return E6Cell(p.FRIProof.FinalPoly[0]) })
	t.absorb(pv.final, KindE6)
	// RandomManyIntegers: one challenge per 8 positions, a position per lane
	positions := make([]int, 0, pv.op.NumQueries)
	var d int
	for k := range pv.op.NumQueries {
		if k%CellWidth == 0 {
			d = t.squeeze()
		}
		positions = append(positions, m.Lanes(d, k%CellWidth, KindScalar))
	}
	for _, b := range pv.op.Batches {
		if b.IsPrecomp {
			pv.treeRoots = append(pv.treeRoots, m.Const(Cell(pv.op.PrecomputedRoot)))
		} else {
			pv.treeRoots = append(pv.treeRoots, pv.v.commitments[b.Round.ID])
		}
	}
	return positions
}

// levelConstants computes, per level, its alphaDeep a, a^R, and per claim
// point z = ζ·ω^shift, V_z = Σ_c a^(p(c))·y_(c,z).
func (pv *pcsVerifier) levelConstants(zeta int) error {
	m, plan := pv.v.m, pv.plan
	for _, l := range plan.Levels {
		var lv pcsLevel
		if l.Round < plan.NumRounds {
			lv.a = m.Mul(pv.alphas[l.Round], pv.alphas[l.Round])
		} else {
			lv.a = pv.alphas[plan.NumRounds-1]
		}
		lv.aR = pv.pow(lv.a, l.RunningPower)
		lv.order = make([]int, len(l.Entries))
		for i := range lv.order {
			lv.order[i] = i
		}
		slices.SortFunc(lv.order, func(i, j int) int { return l.Entries[j].Power - l.Entries[i].Power })
		rot := map[int]field.Element{}
		for _, e := range l.Entries {
			for k, s := range e.Shifts {
				if r, ok := rot[s]; ok && r != e.Rotations[k] {
					return fmt.Errorf("level of size 2^%d: two rotations for shift %d", l.SizeLog2, s)
				}
				rot[s] = e.Rotations[k]
			}
		}
		for s := range rot {
			lv.shifts = append(lv.shifts, s)
		}
		slices.Sort(lv.shifts)
		for _, s := range lv.shifts {
			// coefficients by decreasing power, from the highest opened at s
			var coeffs []int
			top := -1
			for i, idx := range lv.order {
				e := l.Entries[idx]
				k := slices.Index(e.Shifts, s)
				if top < 0 && k < 0 {
					continue
				}
				if top < 0 {
					top = i
				}
				if k < 0 {
					coeffs = append(coeffs, pv.zero)
					continue
				}
				coeffs = append(coeffs, pv.v.cell(pv.claim(l.SizeLog2, e, k)))
			}
			lv.top = append(lv.top, top)
			lv.z = append(lv.z, m.Mul(zeta, pv.v.scalar(rot[s])))
			lv.vz = append(lv.vz, m.Horner(lv.a, coeffs))
		}
		pv.levels = append(pv.levels, lv)
	}
	return nil
}

// claim returns the claim cell of an entry of a level of size sizeLog2 at
// its k-th shift.
func (pv *pcsVerifier) claim(sizeLog2 int, e fri.PlanEntry, k int) *wiop.Cell {
	sized := pv.op.Claims[e.Batch][sizeLog2]
	if e.IsExt {
		return sized.Ext[e.Row][k]
	}
	return sized.Base[e.Row][k]
}

// pow returns a^n, by squarings.
func (pv *pcsVerifier) pow(a, n int) int {
	res, started := pv.one, false
	for i := 62; i >= 0; i-- {
		if started {
			res = pv.v.m.Mul(res, res)
		}
		if n>>i&1 == 1 {
			if started {
				res = pv.v.m.Mul(res, a)
			} else {
				res, started = a, true
			}
		}
	}
	return res
}

// treeLevel returns the index in an input tree opening's Leaves of the pair
// of size sizeLog2: the bottom level for the tree's largest size, the
// auxiliary level rate + size − 1 for a smaller one.
func (pv *pcsVerifier) treeLevel(tree fri.PlanTree, sizeLog2 int) int {
	if sizeLog2 == len(tree.Shape)-1 {
		return tree.Height - 1
	}
	return pv.plan.RateLog + sizeLog2 - 1
}

// treeSizes returns the sizes a tree's opening carries pairs of, the largest
// first: the sizes of the batch with columns.
func treeSizes(tree fri.PlanTree) []int {
	var res []int
	for s := len(tree.Shape) - 1; s >= 0; s-- {
		if tree.Shape[s].BaseWidth+tree.Shape[s].ExtWidth > 0 {
			res = append(res, s)
		}
	}
	return res
}

// rowInputs allocates the input cells of one side (0: the queried row, 1: its
// conjugate) of a tree opening's pair at a level.
func (pv *pcsVerifier) rowInputs(q, t, level, side int, shape fri.SizedShape) rowCells {
	var rc rowCells
	for k := 0; k < shape.BaseWidth; k += CellWidth {
		rc.base = append(rc.base, pv.input(KindDigest, func(p *fri.OpeningProof) Cell {
			var c Cell
			copy(c[:], p.InputQueries[q][t].Leaves[level][side].Base[k:])
			return c
		}))
	}
	for k := range shape.ExtWidth {
		rc.ext = append(rc.ext, pv.input(KindE6, func(p *fri.OpeningProof) Cell {
			return E6Cell(p.InputQueries[q][t].Leaves[level][side].Ext[k])
		}))
	}
	return rc
}

// leafItems returns the items fri hashes a leaf of rows of the given shape
// from (its header, then every row's base values and extension values), and
// their kinds.
func (pv *pcsVerifier) leafItems(shape fri.SizedShape, rows ...rowCells) (items, kinds []int) {
	var hdr Cell
	hdr[0].SetUint64(leafDomainTag)
	hdr[1].SetUint64(uint64(shape.BaseWidth))
	hdr[2].SetUint64(uint64(shape.ExtWidth))
	items, kinds = []int{pv.v.m.Const(hdr)}, []int{3}
	for _, r := range rows {
		for k, c := range r.base {
			items = append(items, c)
			kinds = append(kinds, min(CellWidth, shape.BaseWidth-k*CellWidth))
		}
		for _, c := range r.ext {
			items = append(items, c)
			kinds = append(kinds, KindE6)
		}
	}
	return items, kinds
}

func (pv *pcsVerifier) hashLeaf(shape fri.SizedShape, rows ...rowCells) int {
	items, kinds := pv.leafItems(shape, rows...)
	return pv.v.m.MDHash(pv.v.m.Pack(items, kinds))
}

// query checks one FRI query, at the position in the lane cell pos.
func (pv *pcsVerifier) query(q, pos int) {
	m, plan := pv.v.m, pv.plan
	bits := m.Bits(pos, plan.LogCodewordSize)
	top := plan.LogCodewordSize - plan.RateLog

	// input trees: rows[batch][size] = the pair's two sides
	rows := make([]map[int][2]rowCells, len(pv.op.Batches))
	var xInv int
	for t, tree := range plan.Trees {
		rows[tree.Batch] = map[int][2]rowCells{}
		sizes := treeSizes(tree)
		for _, s := range sizes {
			level := pv.treeLevel(tree, s)
			rows[tree.Batch][s] = [2]rowCells{
				pv.rowInputs(q, t, level, 0, tree.Shape[s]),
				pv.rowInputs(q, t, level, 1, tree.Shape[s]),
			}
		}
		bottom := sizes[0]
		pair := rows[tree.Batch][bottom]
		leaf := pv.hashLeaf(tree.Shape[bottom], pair[0])
		siblings := []int{pv.hashLeaf(tree.Shape[bottom], pair[1])}
		for lvl := tree.Height - 2; lvl >= 0; lvl-- {
			siblings = append(siblings, pv.input(KindDigest, func(p *fri.OpeningProof) Cell {
				return Cell(p.InputQueries[q][t].Siblings[lvl])
			}))
		}
		// the smaller sizes' pairs, hashed even row first, injected
		var injections []Injection
		for _, s := range sizes[1:] {
			pair := rows[tree.Batch][s]
			bit := bits.Bit[top-s]
			even, odd := pv.selectRow(bit, pair[1], pair[0]), pv.selectRow(bit, pair[0], pair[1])
			injections = append(injections, Injection{
				Width:  1 << pv.treeLevel(tree, s),
				Digest: pv.hashLeaf(tree.Shape[s], even, odd),
			})
		}
		index := bits.Shr[top-bottom]
		if t == pv.xTree {
			gInv := plan.Generators[0]
			gInv.Inverse(&gInv)
			xInv = m.Lanes(m.MerklePathX(leaf, siblings, index, pv.treeRoots[tree.Batch], gInv, injections...),
				0, KindScalar)
		} else {
			m.MerklePath(leaf, siblings, index, pv.treeRoots[tree.Batch], injections...)
		}
	}

	// running layers: running[j] = (value at the query point, at its
	// conjugate), 1 ≤ j < NumRounds; none at round 0 nor at the boundary
	// round NumRounds
	running := make([][2]int, plan.NumRounds+1)
	for j := 1; j < plan.NumRounds; j++ {
		self := pv.input(KindE6, func(p *fri.OpeningProof) Cell {
			return Cell(p.FRIProof.RunningQueries[q][j-1].Leaf)
		})
		height := plan.LogCodewordSize - j
		siblings := make([]int, height)
		for i := range height {
			// Siblings are root first: the leaf level's is the last
			kind := KindDigest
			if i == 0 {
				kind = KindE6
			}
			siblings[i] = pv.input(kind, func(p *fri.OpeningProof) Cell {
				return Cell(p.FRIProof.RunningQueries[q][j-1].Siblings[height-1-i])
			})
		}
		m.MerklePath(self, siblings, bits.Shr[j], pv.roots[j-1])
		running[j] = [2]int{self, siblings[0]}
	}

	// the query point x_j at every round with a level, x_0 = 1/x⁻¹
	xs := []int{m.Div(pv.one, xInv)}
	for j := 1; j <= plan.Levels[len(plan.Levels)-1].Round; j++ {
		xs = append(xs, m.Mul(xs[j-1], xs[j-1]))
	}

	pairs := make(map[int][2]int, len(plan.Levels))
	for i, l := range plan.Levels {
		var vals [2]int
		for side := range 2 {
			x := xs[l.Round]
			if side == 1 {
				x = m.Sub(pv.zero, x)
			}
			vals[side] = pv.levelValue(i, rows, side, x, running[l.Round][side], l.Round > 0 && l.Round < plan.NumRounds)
		}
		if l.Round == plan.NumRounds {
			// the boundary level is not folded: its DEEP quotient is constant
			pv.v.assertEq(vals[0], vals[1])
			continue
		}
		pairs[l.Round] = vals
	}

	rounds := make([]FoldRound, plan.NumRounds)
	for j := range rounds {
		pair, ok := pairs[j]
		if !ok {
			pair = running[j]
		}
		target := pv.final
		if j+1 < plan.NumRounds {
			target = running[j+1][0]
		}
		rounds[j] = FoldRound{P: pair[0], Q: pair[1], Alpha: pv.alphas[j], Target: target}
	}
	m.FoldChain(xInv, rounds)
}

// selectRow returns, cell by cell, a if bit else b.
func (pv *pcsVerifier) selectRow(bit int, a, b rowCells) rowCells {
	var res rowCells
	for k := range a.base {
		res.base = append(res.base, pv.v.m.Select(bit, a.base[k], b.base[k]))
	}
	for k := range a.ext {
		res.ext = append(res.ext, pv.v.m.Select(bit, a.ext[k], b.ext[k]))
	}
	return res
}

// levelValue returns level i's value at a side of the query (0: the queried
// row, at x; 1: its conjugate, at −x):
//
//	a^R·running + Σ_z (Σ_c a^(p(c))·f_c(x) − V_z)/(x − z)
//
// The division binds its result only if x ≠ z, which holds but with
// negligible probability for z = ζ·ω^shift, ζ a Fiat–Shamir coin (the
// native verifier rejects the proof there).
func (pv *pcsVerifier) levelValue(i int, rows []map[int][2]rowCells, side, x, running int, hasRunning bool) int {
	m, l, lv := pv.v.m, pv.plan.Levels[i], pv.levels[i]
	res := -1
	if hasRunning {
		res = m.Mul(lv.aR, running)
	}
	for k, s := range lv.shifts {
		var chain []DeepRow
		for _, idx := range lv.order[lv.top[k]:] {
			e := l.Entries[idx]
			sel := uint8(0)
			if slices.Contains(e.Shifts, s) {
				sel = 1
			}
			rc := rows[e.Batch][l.SizeLog2][side]
			if e.IsExt {
				chain = append(chain, DeepRow{Cell: rc.ext[e.Row], Ext: true, Act: 1, Sel: sel})
				continue
			}
			cell, lane := rc.base[e.Row/CellWidth], e.Row%CellWidth
			// extend the previous row if this value is its cell's next lane
			// down: a base row steps through its lanes from the highest
			if n := len(chain); n > 0 && !chain[n-1].Ext && chain[n-1].Cell == cell &&
				chain[n-1].Act&(1<<(lane+1)) != 0 && chain[n-1].Act&(1<<(lane+1)-1) == 0 {
				chain[n-1].Act |= 1 << lane
				chain[n-1].Sel |= sel << lane
				continue
			}
			chain = append(chain, DeepRow{Cell: cell, Act: 1 << lane, Sel: sel << lane})
		}
		term := m.Div(m.Sub(m.DeepHorner(lv.a, chain), lv.vz[k]), m.Sub(x, lv.z[k]))
		if res < 0 {
			res = term
		} else {
			res = m.Add(res, term)
		}
	}
	return res
}

// checkOpeningShape checks an opening proof has the shape the machine
// verifies, as fri.PCS.Verify checks it before reading it, so that the
// inputs' values can be read from it.
func (pv *pcsVerifier) checkOpeningShape(p *fri.OpeningProof) error {
	plan := pv.plan
	if p == nil {
		return fmt.Errorf("no PCS opening proof")
	}
	f := p.FRIProof
	if len(f.RoundRoots) != plan.NumRounds-1 || len(f.RoundCaps) != plan.NumRounds-1 {
		return fmt.Errorf("%d round roots and %d caps, want %d", len(f.RoundRoots), len(f.RoundCaps), plan.NumRounds-1)
	}
	for j, c := range f.RoundCaps {
		if len(c.Nodes) != 0 || len(c.Aux) != 0 {
			return fmt.Errorf("round %d: a Merkle cap", j+1)
		}
	}
	if len(f.FinalPoly) != 1<<plan.LogFinalPolySize {
		return fmt.Errorf("a final polynomial of %d coefficients, want %d", len(f.FinalPoly), 1<<plan.LogFinalPolySize)
	}
	if len(f.RunningQueries) != pv.op.NumQueries || len(p.InputQueries) != pv.op.NumQueries {
		return fmt.Errorf("%d running and %d input queries, want %d",
			len(f.RunningQueries), len(p.InputQueries), pv.op.NumQueries)
	}
	if len(p.InputCaps) != len(plan.Trees) {
		return fmt.Errorf("%d input caps, want %d", len(p.InputCaps), len(plan.Trees))
	}
	for t, c := range p.InputCaps {
		if len(c.Nodes) != 0 || len(c.Tables) != 0 {
			return fmt.Errorf("input tree %d: a Merkle cap", t)
		}
	}
	for q := range pv.op.NumQueries {
		if err := pv.checkQueryShape(p, q); err != nil {
			return fmt.Errorf("query %d: %w", q, err)
		}
	}
	return nil
}

func (pv *pcsVerifier) checkQueryShape(p *fri.OpeningProof, q int) error {
	plan := pv.plan
	if len(p.InputQueries[q]) != len(plan.Trees) {
		return fmt.Errorf("%d input tree openings, want %d", len(p.InputQueries[q]), len(plan.Trees))
	}
	for t, tree := range plan.Trees {
		o := p.InputQueries[q][t]
		if len(o.Leaves) != tree.Height || len(o.Siblings) != tree.Height-1 {
			return fmt.Errorf("input tree %d: %d row levels and %d siblings, want %d and %d",
				t, len(o.Leaves), len(o.Siblings), tree.Height, tree.Height-1)
		}
		want := map[int]int{}
		for _, s := range treeSizes(tree) {
			want[pv.treeLevel(tree, s)] = s
		}
		for level, pair := range o.Leaves {
			s, ok := want[level]
			if (pair != nil) != ok {
				return fmt.Errorf("input tree %d: unexpected row pair at level %d", t, level)
			}
			if !ok {
				continue
			}
			for side, row := range pair {
				if len(row.Base) != tree.Shape[s].BaseWidth || len(row.Ext) != tree.Shape[s].ExtWidth {
					return fmt.Errorf("input tree %d level %d: row %d has the wrong shape", t, level, side)
				}
			}
		}
	}
	if len(p.FRIProof.RunningQueries[q]) != plan.NumRounds-1 {
		return fmt.Errorf("%d running layers, want %d", len(p.FRIProof.RunningQueries[q]), plan.NumRounds-1)
	}
	for i, b := range p.FRIProof.RunningQueries[q] {
		height := plan.LogCodewordSize - i - 1
		if len(b.Siblings) != height || len(b.AuxSiblings) != height {
			return fmt.Errorf("round %d: %d siblings, want %d", i+1, len(b.Siblings), height)
		}
		for _, aux := range b.AuxSiblings {
			if len(aux) != 0 {
				return fmt.Errorf("round %d: an injection in a running layer", i+1)
			}
		}
		// the layer's leaves are extension values
		for _, o := range []field.Octuplet{b.Leaf, b.Siblings[height-1]} {
			if !o[6].IsZero() || !o[7].IsZero() {
				return fmt.Errorf("round %d: a leaf that is not an extension value", i+1)
			}
		}
	}
	return nil
}
