package gkrverifier

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// foldRow is a step of the Horner evaluations, at a level that is not a skip level, of the claimed
// values Σᵢ cⁱvᵢ and of the weighted gate results Σᵢ cⁱeᵢφᵢ, over the level's claims i in gnark's order.
// The rows of a level run from the last claim to the first: with the sums s and t of the rows before,
//
//	sum' = v + c·sum,    final' = e·φ + c·final.
type foldRow struct {
	// the ids of the claimed value v, of the gate result φ, of eq e, which is -1 if e = 1, and of the
	// folding challenge c, which is -1 if c = 0
	value, phi, eq, fold int
	first, last          bool
	// sum and final are the ids that the last row of a level exposes its two sums under.
	sum, final int
}

// eqRow is a step of eq(p, r) = Πⱼ (1 − pⱼ − rⱼ + 2pⱼrⱼ): the row of the variable j reads pⱼ and rⱼ.
type eqRow struct {
	p, r  int
	first bool
	// result is the id that the last row of a product exposes it under, and -1 on the others.
	result int
}

// roundRow is a round of a sumcheck: with a the claim of the round, c₁, …, c_d the coefficients of the
// round polynomial that the proof holds, and r the challenge,
//
//	κc₀ + μ·Σ cₖ = a,    a' = c₀ + r·Σ cₖrᵏ⁻¹,
//
// where (κ, μ) = (2, 1) at a sumcheck level and (1, qⱼ) at a single-source level, qⱼ being the
// coordinate of the source's point.
type roundRow struct {
	kappa int
	// the ids of r, of qⱼ (-1 at a sumcheck level, where μ = 1), and of c₁, …, c_d
	r, mu  int
	coeffs []int
	// the first row of a level starts from the sum with the id sum, the last ends on the value with the
	// id final; they are -1 on the others.
	first, last bool
	sum, final  int
}

// claimLayout is a claim that gnark's Verify returns: the multilinear of the wire evaluates, at the
// point whose coordinates are the challenges with the ids point, to the proof's element at value.
type claimLayout struct {
	wire, index int
	point       []int
	value       int
}

// lowerPoints sets the outgoing evaluation points of the levels, from the output level down: the
// initial point, then at a skip level those of its sources, and at any other level its challenges.
func (l *layout) lowerPoints() {
	l.points = make([][][]int, len(l.schedule)+1)
	l.points[len(l.schedule)] = [][]int{l.initialChallenges}
	for levelI := len(l.schedule) - 1; levelI >= 0; levelI-- {
		lv := &l.levels[levelI]
		if !lv.skip {
			l.points[levelI] = [][]int{lv.roundChallenges}
			continue
		}
		for _, source := range l.schedule[levelI].ClaimGroups()[0].ClaimSources {
			l.points[levelI] = append(l.points[levelI], l.points[source.Level][source.OutgoingClaimIndex])
		}
	}
}

// lowerSumchecks lays out the rows of the folds, the eqs and the rounds of every level that is not a
// skip level, and the ids they expose, which follow the gates'.
func (l *layout) lowerSumchecks() {
	next := l.firstGate + len(l.gateRows)
	alloc := func() int {
		next++
		return next - 1
	}
	for levelI, level := range l.schedule {
		lv := &l.levels[levelI]
		if lv.skip {
			continue
		}
		l.maxDegree = max(l.maxDegree, lv.degree)
		lv.sum, lv.final = alloc(), alloc()
		groups := level.ClaimGroups()

		if !lv.single {
			for _, group := range groups {
				ids := make([]int, len(group.ClaimSources))
				for k, source := range group.ClaimSources {
					ids[k] = alloc()
					point := l.points[source.Level][source.OutgoingClaimIndex]
					for j := range l.n {
						row := eqRow{p: point[j], r: lv.roundChallenges[j], first: j == 0, result: -1}
						if j == l.n-1 {
							row.result = ids[k]
						}
						l.eqRows = append(l.eqRows, row)
					}
				}
				lv.eqs = append(lv.eqs, ids)
			}
		}

		// the claims in gnark's order
		var claims []foldRow
		levelWireI := 0
		for groupI, group := range groups {
			for _, wireI := range group.Wires {
				for claimI, source := range group.ClaimSources {
					claim := foldRow{
						value: l.claimPosition(source, l.claimValueIndices[wireI][claimI]),
						phi:   lv.results[levelWireI],
						eq:    -1,
						fold:  lv.foldChallenge,
						sum:   -1, final: -1,
					}
					if !lv.single {
						claim.eq = lv.eqs[groupI][claimI]
					}
					claims = append(claims, claim)
				}
				levelWireI++
			}
		}
		for i := len(claims) - 1; i >= 0; i-- {
			row := claims[i]
			row.first, row.last = i == len(claims)-1, i == 0
			if row.last {
				row.sum, row.final = lv.sum, lv.final
			}
			l.foldRows = append(l.foldRows, row)
		}

		for j := range l.n {
			row := roundRow{
				kappa: 2, r: lv.roundChallenges[j], mu: -1, coeffs: rangeOf(lv.start+j*lv.degree, lv.degree),
				first: j == 0, last: j == l.n-1, sum: -1, final: -1,
			}
			if lv.single {
				source := groups[0].ClaimSources[0]
				row.kappa, row.mu = 1, l.points[source.Level][source.OutgoingClaimIndex][j]
			}
			if row.first {
				row.sum = lv.sum
			}
			if row.last {
				row.final = lv.final
			}
			l.roundRows = append(l.roundRows, row)
		}
	}
	l.nbIDs = next
}

// lowerClaims lists the claims that gnark's Verify returns, in the order it appends them: the outputs
// that are not consolidated, then the levels from the top down to level 0, whose every wire gets a
// claim, the other levels giving one to each unconsolidated input.
func (l *layout) lowerClaims() {
	counts := make(map[int]int)
	add := func(wireI int, point []int, value int) {
		l.claims = append(l.claims, claimLayout{wire: wireI, index: counts[wireI], point: point, value: value})
		counts[wireI]++
	}
	for i, wireI := range l.circuit.Outputs() {
		if !l.consolidated[wireI] {
			add(wireI, l.initialChallenges, l.outputs+i)
		}
	}
	for levelI := len(l.schedule) - 1; levelI >= 0; levelI-- {
		level, lv := l.schedule[levelI], &l.levels[levelI]
		unique, _ := l.circuit.LevelCircuit(l.schedule, levelI, identityGate()).InputMapping(level)
		for uniqueI, wireI := range unique {
			if levelI != 0 && (!l.circuit.IsInput(wireI) || l.consolidated[wireI]) {
				continue
			}
			for k, point := range l.points[levelI] {
				add(wireI, point, lv.finals+level.FinalEvalProofIndex(uniqueI, k))
			}
		}
	}
}

// foldWitness holds the values of the columns of the fold rows.
type foldWitness struct {
	v, phi, e, c, sum, final []field.Ext
}

// eqWitness holds the values of the columns of the eq rows: the factor 1 − p − r + 2pr and the product.
type eqWitness struct {
	p, r, factor, product []field.Ext
}

// roundWitness holds the values of the columns of the round rows: the coefficients c₀, …, c_D, and the
// intermediates h₁, …, h_{D−1} of Horner's rule, hₖ = cₖ + r·hₖ₊₁ with h_D = c_D.
type roundWitness struct {
	a, out, r, mu, c0 []field.Ext
	c, h              [][]field.Ext
}

// evaluateEqs runs the eq rows, writing the results to values.
func (l *layout) evaluateEqs(values []field.Ext) *eqWitness {
	n := len(l.eqRows)
	w := &eqWitness{
		p: make([]field.Ext, n), r: make([]field.Ext, n), factor: make([]field.Ext, n), product: make([]field.Ext, n),
	}
	for i, row := range l.eqRows {
		w.p[i], w.r[i] = values[row.p], values[row.r]
		var pr field.Ext
		pr.Mul(&w.p[i], &w.r[i])
		w.factor[i] = field.OneExt()
		w.factor[i].Sub(&w.factor[i], &w.p[i])
		w.factor[i].Sub(&w.factor[i], &w.r[i])
		w.factor[i].Add(&w.factor[i], &pr)
		w.factor[i].Add(&w.factor[i], &pr)
		w.product[i] = w.factor[i]
		if !row.first {
			w.product[i].Mul(&w.product[i], &w.product[i-1])
		}
		if row.result >= 0 {
			values[row.result] = w.product[i]
		}
	}
	return w
}

// evaluateFolds runs the fold rows, writing the sums of each level to values.
func (l *layout) evaluateFolds(values []field.Ext) *foldWitness {
	n := len(l.foldRows)
	w := &foldWitness{
		v: make([]field.Ext, n), phi: make([]field.Ext, n), e: make([]field.Ext, n), c: make([]field.Ext, n),
		sum: make([]field.Ext, n), final: make([]field.Ext, n),
	}
	for i, row := range l.foldRows {
		w.v[i], w.phi[i] = values[row.value], values[row.phi]
		w.e[i] = field.OneExt()
		if row.eq >= 0 {
			w.e[i] = values[row.eq]
		}
		if row.fold >= 0 {
			w.c[i] = values[row.fold]
		}
		w.sum[i] = w.v[i]
		w.final[i].Mul(&w.e[i], &w.phi[i])
		if !row.first {
			var term field.Ext
			term.Mul(&w.c[i], &w.sum[i-1])
			w.sum[i].Add(&w.sum[i], &term)
			term.Mul(&w.c[i], &w.final[i-1])
			w.final[i].Add(&w.final[i], &term)
		}
		if row.last {
			values[row.sum], values[row.final] = w.sum[i], w.final[i]
		}
	}
	return w
}

// evaluateRounds runs the round rows, which start from the sums that the folds wrote to values.
func (l *layout) evaluateRounds(values []field.Ext) *roundWitness {
	n, d := len(l.roundRows), l.maxDegree
	w := &roundWitness{
		a: make([]field.Ext, n), out: make([]field.Ext, n), r: make([]field.Ext, n), mu: make([]field.Ext, n),
		c0: make([]field.Ext, n), c: make([][]field.Ext, n), h: make([][]field.Ext, n),
	}
	var inverseOfTwo field.Element
	inverseOfTwo.SetUint64(2)
	inverseOfTwo.Inverse(&inverseOfTwo)

	for i, row := range l.roundRows {
		w.r[i] = values[row.r]
		w.mu[i] = field.OneExt()
		if row.mu >= 0 {
			w.mu[i] = values[row.mu]
		}
		if row.first {
			w.a[i] = values[row.sum]
		} else {
			w.a[i] = w.out[i-1]
		}
		w.c[i], w.h[i] = make([]field.Ext, d), make([]field.Ext, max(d-1, 0))
		var total field.Ext
		for k, id := range row.coeffs {
			w.c[i][k] = values[id]
			total.Add(&total, &w.c[i][k])
		}

		// c₀ = (a − μ·Σ cₖ) / κ
		w.c0[i].Mul(&w.mu[i], &total)
		w.c0[i].Sub(&w.a[i], &w.c0[i])
		if row.kappa == 2 {
			w.c0[i].MulByElement(&w.c0[i], &inverseOfTwo)
		}

		horner := w.c[i][d-1]
		for k := d - 2; k >= 0; k-- {
			horner.Mul(&horner, &w.r[i])
			horner.Add(&horner, &w.c[i][k])
			w.h[i][k] = horner
		}
		w.out[i].Mul(&horner, &w.r[i])
		w.out[i].Add(&w.out[i], &w.c0[i])
	}
	return w
}
