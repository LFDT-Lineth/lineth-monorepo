package fri

import (
	"errors"
	"fmt"
	"math/big"
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	gutils "github.com/consensys/gnark-crypto/utils"
)

// Params holds the FRI configuration and precomputed per-level data.
// Build once with NewParams; reuse across many Prove/Verify calls.
type Params struct {
	N          int // 2^n: the dimension of the code
	D          int // 2^m: the size of the plaintext polynomial
	NumQueries int // number of independent queries (controls soundness error ≈ (1-δ)^Q)

	numRounds    int // numRounds = m
	invTwo       field.Element
	domains      []*fft.Domain // domains[j] has cardinality N/2^j, generator ωⱼ
	domainsLight []domainLight // domainLight stores only the cardinality and the domain generator
}

type Config struct {
	WoFullDomainAllocation bool
}

type Option func(c *Config) error

func WoFullDomainAllocation() Option {
	return func(c *Config) error {
		c.WoFullDomainAllocation = true
		return nil
	}
}

// NewParams constructs and validates a Params, precomputing r+1 domains and inv(2).
func NewParams(
	n, d, numQueries int,
	opts ...Option,
) (Params, error) {
	if n <= 0 || n&(n-1) != 0 {
		return Params{}, fmt.Errorf("fri: N must be a positive power of two, got %d", n)
	}
	if d <= 0 || d&(d-1) != 0 {
		return Params{}, fmt.Errorf("fri: D must be a positive power of two, got %d", d)
	}
	if d >= n {
		return Params{}, fmt.Errorf("fri: D must be < N, got D=%d N=%d", d, n)
	}
	if numQueries <= 0 {
		return Params{}, fmt.Errorf("fri: numQueries must be positive, got %d", numQueries)
	}

	var config Config
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return Params{}, err
		}
	}

	numRounds := utils.Log2Ceil(d) // r = m = log₂(D)

	var two, invTwo field.Element
	two.SetUint64(2)
	invTwo.Inverse(&two)

	res := Params{
		N:          n,
		D:          d,
		NumQueries: numQueries,
		numRounds:  numRounds,
		invTwo:     invTwo,
	}

	if !config.WoFullDomainAllocation {
		res.domains = make([]*fft.Domain, numRounds+1)
		for j := range numRounds + 1 {
			res.domains[j] = fft.NewDomain(uint64(n) >> j)
		}
	}
	res.domainsLight = make([]domainLight, numRounds+1)
	for j := range numRounds + 1 {
		g, err := koalabear.Generator(uint64(n) >> j)
		if err != nil {
			return Params{}, err
		}
		res.domainsLight[j] = domainLight{cardinality: uint64(n) >> j, generator: g}

	}

	return res, nil
}

// restrictTo returns a Params that runs FRI over the top sub-domain of plaintext
// size 2^topSizeLog2 (which must be <= D), reusing this Params' precomputed
// domains via a slice offset. It lets a single statically-sized Params — built
// once at the maximum supported size — drive proofs whose witness is smaller:
// the number of fold rounds becomes topSizeLog2 (witness-dependent) rather than
// p.numRounds, and the final polynomial still has size N/D = the inverse rate.
// No domains are rebuilt and no zero rounds are folded.
func (p Params) restrictTo(topSizeLog2 int) (Params, error) {
	if topSizeLog2 < 0 || topSizeLog2 > p.numRounds {
		return Params{}, fmt.Errorf("fri: restrictTo: top size 2^%d outside [1, D=2^%d]", topSizeLog2, p.numRounds)
	}
	offset := p.numRounds - topSizeLog2
	res := Params{
		N:          p.N >> offset,
		D:          1 << topSizeLog2,
		NumQueries: p.NumQueries,
		numRounds:  topSizeLog2,
		invTwo:     p.invTwo,
	}
	if p.domains != nil {
		res.domains = p.domains[offset:]
	}
	res.domainsLight = p.domainsLight[offset:]
	return res, nil
}

type domainLight struct {
	cardinality uint64
	generator   field.Element
}

func domainPoint(domain domainLight, position int) field.Element {
	logSize := bits.TrailingZeros64(domain.cardinality)
	exponent := bits.Reverse64(uint64(position)) >> (64 - logSize)

	var x field.Element
	x.Exp(domain.generator, big.NewInt(int64(exponent)))
	return x
}

func domainPointExt(domain domainLight, position int) field.Ext {
	return field.Lift(domainPoint(domain, position))
}

// QueryLayer holds one Merkle branch per tree backing one folding level. The
// running FRI layers use one branch; virtual PCS levels may use several.
type QueryLayer []Branch

// QueryLayerRoots holds the Merkle roots corresponding to a QueryLayer.
type QueryLayerRoots []field.Octuplet

// RunningQuery holds the running-layer openings for one query.
// RunningQuery[j-1] opens folding round j, so len(RunningQuery) =
// numRounds-1.
type RunningQuery []QueryLayer

// Level holds one polynomial introduced at the folding round where the running
// polynomial's degree matches Level.D. Trees are the pre-built paired-leaf
// Merkle trees backing it.
//
// The polynomial's evaluations are not stored directly: Size, Columns, and
// the ClaimPoints/ClaimPointIndexes/DenominatorInverses data batch the
// per-column DEEP quotients into it, but only once alphaDeep is known -- see
// EvalsAt. That challenge is the square of this level's own introduction
// round's fold challenge, sampled at fold time, so the batched evaluations
// cannot be precomputed any earlier.
type Level struct {
	D     int
	Trees []*Tree

	Size                int
	Columns             []quotientColumn
	ClaimPointIndexes   map[field.Ext]int
	ClaimPoints         []field.Ext
	DenominatorInverses []field.Ext
}

// EvalsAt returns this level's evaluations, column-batched using alphaDeep as
// the power-ladder base.
func (l Level) EvalsAt(alphaDeep field.Ext) []field.Ext {
	alphaDeepPowers := powers(alphaDeep, len(l.Columns))
	evals := make([]field.Ext, l.Size)
	for pos := range evals {
		for columnIdx, column := range l.Columns {
			var columnSum field.Ext
			for _, claim := range column.Claims {
				pointIdx := l.ClaimPointIndexes[claim.Point]
				inv := l.DenominatorInverses[pos*len(l.ClaimPoints)+pointIdx]

				var numerator, term field.Ext
				numerator.Sub(&column.Evals[pos], &claim.Value)
				term.Mul(&numerator, &inv)
				columnSum.Add(&columnSum, &term)
			}

			var weighted field.Ext
			weighted.Mul(&columnSum, &alphaDeepPowers[columnIdx])
			evals[pos].Add(&evals[pos], &weighted)
		}
	}
	return evals
}

// Proof is the complete multi-degree FRI proof. Level polynomial Merkle roots
// are NOT stored here — they are passed externally to Verify (the caller
// commits to those polynomials before invoking FRI).
type Proof struct {
	// Running-polynomial FRI path
	RoundRoots     []field.Octuplet // Merkle roots for running poly T_1..T_{r-1}
	FinalPolyExt   []field.Ext
	RunningQueries []RunningQuery
}

// FullDomainGenerator returns the generator of the full evaluation domain (layer 0, size N).
func (p Params) FullDomainGenerator() field.Element {
	return p.domains[0].Generator
}

// ────────────────────────────────────────────────────────────────────────────────
// Prove — multi-degree FRI prover
// ────────────────────────────────────────────────────────────────────────────────

// provePlan is the validated, precomputed schedule that NewProverState derives
// from the caller-supplied levels: levelAtRound maps a folding round j to the
// level index l introduced there. A level of size levels[l].D is introduced
// at round jl = log2(p.D / levels[l].D) — precisely when the running
// polynomial has folded down to that level's degree. This includes the main
// degree-D polynomial itself, always introduced at round 0 (jl=0, since
// D=p.D): it is folded exactly like any other level, against the running
// codeword from the preceding round -- except at round 0, where there is no
// preceding round, so nothing to fold against (see ProverState.Fold).
// ProverState.Fold consults this map to know when to fold in each level, and
// the verifier rebuilds the same map to replay the combination.
type provePlan struct {
	numLevels    int
	levelAtRound map[int]int
}

// buildProvePlan validates levels and computes the provePlan schedule. It
// enforces that every level is a power-of-two degree at most p.D with
// non-empty, non-nil trees, that no two levels are introduced at the same
// folding round, and that exactly one level is introduced at round 0 (i.e.
// has D == p.D). levels must already be sorted in decreasing order of D
// (Prove does this).
func buildProvePlan(p Params, levels []Level) (provePlan, error) {
	var plan provePlan
	if len(levels) == 0 {
		return plan, fmt.Errorf("fri: Prove: at least one level required")
	}

	plan.numLevels = len(levels)
	plan.levelAtRound = make(map[int]int, plan.numLevels)
	for l := range levels {
		if levels[l].D <= 0 || levels[l].D&(levels[l].D-1) != 0 {
			return plan, fmt.Errorf("fri: Prove: levels[%d].D=%d is not a positive power of two", l, levels[l].D)
		}
		if levels[l].D > p.D {
			return plan, fmt.Errorf("fri: Prove: levels[%d].D=%d exceeds p.D=%d", l, levels[l].D, p.D)
		}

		jl := utils.Log2Ceil(p.D / levels[l].D)
		if jl >= p.numRounds {
			return plan, fmt.Errorf(
				"fri: Prove: levels[%d].D=%d gives intro round %d, must be in 0..%d",
				l, levels[l].D, jl, p.numRounds-1)
		}
		if _, dup := plan.levelAtRound[jl]; dup {
			return plan, fmt.Errorf("fri: Prove: two levels share intro round %d", jl)
		}
		plan.levelAtRound[jl] = l
		if err := checkLevelTrees(fmt.Sprintf("levels[%d]", l), levels[l].Trees); err != nil {
			return plan, fmt.Errorf("fri: Prove: %w", err)
		}
	}
	if _, ok := plan.levelAtRound[0]; !ok {
		return plan, fmt.Errorf("fri: Prove: no level introduced at round 0 (need one with D=p.D)")
	}

	return plan, nil
}

func checkLevelTrees(label string, trees []*Tree) error {
	if len(trees) == 0 {
		return fmt.Errorf("%s.Trees is empty", label)
	}
	for i, tree := range trees {
		if tree == nil {
			return fmt.Errorf("%s.Trees[%d] is nil", label, i)
		}
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────────
// Proof checks — shape validation, authentication helpers, and fold arithmetic
// ────────────────────────────────────────────────────────────────────────────────

// checkOpeningProofShape validates prf's structure against p and the
// challenge lengths before any authentication or reconstruction runs, so a
// malformed proof can never cause an out-of-bounds access later.
func checkOpeningProofShape(p Params, prf Proof, foldAlphas []field.Ext, positions []int) error {
	wantRoundRoots := p.numRounds - 1
	if p.numRounds <= 1 {
		wantRoundRoots = 0
	}
	wantLayersPerRunningQuery := wantRoundRoots
	if len(prf.RoundRoots) != wantRoundRoots {
		return fmt.Errorf("fri: pcs.Verify: proof has %d round roots, want %d", len(prf.RoundRoots), wantRoundRoots)
	}
	if len(prf.RunningQueries) != p.NumQueries {
		return fmt.Errorf("fri: pcs.Verify: proof has %d running queries, want %d", len(prf.RunningQueries), p.NumQueries)
	}
	if want := p.N >> p.numRounds; len(prf.FinalPolyExt) != want {
		return fmt.Errorf("fri: pcs.Verify: FinalPolyExt has %d entries, want %d", len(prf.FinalPolyExt), want)
	}
	if len(foldAlphas) < p.numRounds {
		return fmt.Errorf("fri: pcs.Verify: %d folding challenges, need at least %d", len(foldAlphas), p.numRounds)
	}
	for k, q := range prf.RunningQueries {
		if len(q) != wantLayersPerRunningQuery {
			return fmt.Errorf("fri: pcs.Verify: query %d has %d running layers, want %d", k, len(q), wantLayersPerRunningQuery)
		}
		if s := positions[k]; s < 0 || s >= p.N {
			return fmt.Errorf("fri: pcs.Verify: opened position %d out of range [0,%d)", s, p.N)
		}
	}
	return nil
}

// resolvedQuery holds every fold input for one query, already authenticated
// against the committed trees and (for every level) reconstructed by the
// caller. checkFolds consumes only this: it never touches a row, a root, a
// branch, or alpha_DEEP.
type resolvedQuery struct {
	Rounds     []inputPair       // Rounds[j] = (self, sibling) of the running codeword at round j (unused at j=0)
	Aux        map[int]inputPair // Aux[j] = the pair of the level introduced at round j, if any (always present at j=0)
	AuxColumns map[int]int       // AuxColumns[j] = number of columns batched into Aux[j] (sets the running codeword's weight in checkFolds)
	Final      field.Ext         // the final-polynomial target for this query
}

// checkFolds verifies the FRI fold recurrence for every query against values
// the caller has already authenticated and reconstructed: pure arithmetic,
// no Merkle proof or row ever passes through it.
func checkFolds(p Params, resolved []resolvedQuery, foldAlphas []field.Ext, positions []int) error {
	for queryIdx, rq := range resolved {
		s := positions[queryIdx]
		for j := range p.numRounds {
			base := s >> j
			self, sib := rq.Rounds[j].Self, rq.Rounds[j].Sibling

			// A level introduced at this round is the primary pair being
			// folded; the running codeword from the preceding round --
			// absent at round 0, since there is no round -1 -- combines with
			// it before the single fold below (Fold is linear), weighted by
			// alphaDeep^n: alphaDeep the square of THIS SAME round's challenge.

			if levelPair, ok := rq.Aux[j]; ok {
				self, sib = levelPair.Self, levelPair.Sibling
				if j > 0 {
					var alphaDeep, wRunning, wSib field.Ext
					alphaDeep.Square(&foldAlphas[j])
					n := rq.AuxColumns[j]
					weight := powers(alphaDeep, n+1)[n]
					wRunning.Mul(&rq.Rounds[j].Self, &weight)
					wSib.Mul(&rq.Rounds[j].Sibling, &weight)
					self.Add(&self, &wRunning)
					sib.Add(&sib, &wSib)
				}
			}

			// x is the domain point of the opened leaf. The codeword is bit-reversed,
			// so the natural-order index of position base is bitReverse(base) and
			// x = gⱼ^{bitReverse(base)} with gⱼ the size-Nⱼ generator.
			var xInv field.Element
			x := domainPoint(p.domainsLight[j], base)
			xInv.Inverse(&x)

			// fold: (self + sib)/2 + alpha · (self - sib)/(2x)
			var sum, diff, expected field.Ext
			sum.Add(&self, &sib)
			sum.MulByElement(&sum, &p.invTwo)
			diff.Sub(&self, &sib)
			diff.MulByElement(&diff, &p.invTwo)
			diff.MulByElement(&diff, &xInv)
			diff.Mul(&diff, &foldAlphas[j])
			expected.Add(&sum, &diff)

			// The fold output must equal the queried leaf of the next layer (whose
			// position is base>>1 = s>>(j+1)); at the last round, the final polynomial.
			if j < p.numRounds-1 {
				if !expected.Equal(&rq.Rounds[j+1].Self) {
					return fmt.Errorf("fri: pcs.Verify: query %d round %d: folded value mismatch with round %d leaf",
						queryIdx, j, j+1)
				}
			} else if !expected.Equal(&rq.Final) {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d (final): folded value does not match FinalPoly",
					queryIdx, j)
			}
		}
	}
	return nil
}

func checkQueryLayerShape(opening QueryLayer, roots QueryLayerRoots, numLeaves int, exactSiblings bool) error {
	if len(opening) != len(roots) {
		return fmt.Errorf("opening has %d branches, want %d", len(opening), len(roots))
	}
	for i, branch := range opening {
		if err := checkBranchShape(branch, numLeaves, exactSiblings); err != nil {
			return fmt.Errorf("branch %d: %w", i, err)
		}
	}
	return nil
}

func checkBranchShape(b Branch, numLeaves int, exactSiblings bool) error {
	want := utils.Log2Ceil(numLeaves)
	if exactSiblings && len(b.Siblings) != want {
		return fmt.Errorf("branch has %d siblings, want %d", len(b.Siblings), want)
	}
	if !exactSiblings && len(b.Siblings) < want {
		return fmt.Errorf("branch has %d siblings, want at least %d", len(b.Siblings), want)
	}
	wantAux := want
	if !exactSiblings {
		wantAux = len(b.Siblings)
	}
	if len(b.AuxSiblings) != wantAux {
		return fmt.Errorf("branch has %d aux siblings, want %d", len(b.AuxSiblings), wantAux)
	}
	return nil
}

// buildTreeExt builds the FRI Merkle tree over one folding layer: a complete
// binary tree whose leaves are the layer's extension elements (padded into
// octuplets). Unlike NewTree, which is the 3-ary multi-size builder, this is a
// plain power-of-two binary tree with no auxiliary leaves.
func buildTreeExt(layer []field.Ext) *Tree {
	return newCompleteBinaryTree(mapExtToOctuplet(layer))
}

// foldLayerInternally computes one step of the FRI split-and-fold routine on a
// codeword stored in bit-reversed order (the order produced by
// [RSEncoder.Encode] / [RSEncoder.EncodeExt]). In that layout the two conjugate
// evaluations of a fold, p(x) and p(-x), sit at the adjacent positions 2j and
// 2j+1, so the fold combines layer[2j] and layer[2j+1] into next[j]. The output
// is itself in bit-reversed order over the half-size domain, ready to be fed
// back into the next round.
//
// aux, when non-empty, is a second codeword on the same domain as layer (the
// running codeword, weighted in alongside a level introduced this round):
// since Fold is linear, its weighted contribution auxWeight·Fold(aux) is
// computed by folding layer+auxWeight·aux once, rather than folding layer and
// aux separately and adding the results.
//
// The folding formula, writing x = g^i for the natural-order domain point of
// pair j (i.e. i = bitReverse(j) over the half-domain):
//
//	next[j] = (layer[2j] + auxWeight·aux[2j] + layer[2j+1] + auxWeight·aux[2j+1]) / 2
//	        + alpha * (layer[2j] + auxWeight·aux[2j] - layer[2j+1] - auxWeight·aux[2j+1]) / (2x)
func foldLayerInternally(
	layer []field.Ext,
	aux []field.Ext,
	auxWeight field.Ext,
	alpha field.Ext,
	domain *fft.Domain,
	invTwo field.Element,
) []field.Ext {

	// domain is the input layer's domain: its generator supplies the twiddles
	// g^{-i} for the conjugate pairs, so its cardinality matches len(layer) (the
	// half-size output uses this same domain, not its own).
	if int(domain.Cardinality) != len(layer) {
		panic("fri: foldLayerInternally: len(layer) != domain.Cardinality")
	}
	if len(aux) > 0 && len(aux) != len(layer) {
		panic("fri: foldLayerInternally: len(aux) != len(layer)")
	}

	var (
		half = len(layer) / 2
		next = make([]field.Ext, half)
	)

	// invTwiddles[j] holds (1/2)·x⁻¹ for pair j, where x = g^i is its
	// natural-order domain point. We build the powers g⁻ⁱ/2 in natural order
	// then bit-reverse the slice so that index j lines up with the bit-reversed
	// layout of layer. This mirrors Plonky3, which bit-reverses its
	// halve_inv_powers (reverse_slice_index_bits) for the very same reason.
	invTwiddles := make([]field.Element, half)
	genPowI := invTwo
	for i := range half {
		invTwiddles[i] = genPowI
		genPowI.Mul(&genPowI, &domain.GeneratorInv)
	}
	gutils.BitReverse(invTwiddles)

	for j := range half {
		p, q := layer[2*j], layer[2*j+1]

		if len(aux) > 0 {
			var wp, wq field.Ext
			wp.Mul(&aux[2*j], &auxWeight)
			wq.Mul(&aux[2*j+1], &auxWeight)
			p.Add(&p, &wp)
			q.Add(&q, &wq)
		}

		var sum, diff field.Ext
		sum.Add(&p, &q)
		sum.MulByElement(&sum, &invTwo)

		diff.Sub(&p, &q)
		diff.MulByElement(&diff, &invTwiddles[j])
		diff.Mul(&diff, &alpha)

		next[j].Add(&sum, &diff)
	}

	return next
}

// octupletToExt converts an octuplet into a field extension. It expects its
// coordinates 6 and 7 to be zero.
func octupletToExt(o field.Octuplet) (field.Ext, error) {

	if !o[6].IsZero() || !o[7].IsZero() {
		return field.Ext{}, errors.New("octupletToExt: coordinates 6 and 7 must be zero")
	}

	var res field.Ext
	res.B0.A0 = o[0]
	res.B0.A1 = o[1]
	res.B1.A0 = o[2]
	res.B1.A1 = o[3]
	res.B2.A0 = o[4]
	res.B2.A1 = o[5]

	return res, nil
}

// mapExtToOctuplet converts a slice of field extensions into a slice of
// octuplets, packing each extension's six coordinates into the first six
// octuplet entries and leaving coordinates 6 and 7 zero. It is the slice-wise
// inverse of octupletToExt.
func mapExtToOctuplet(exts []field.Ext) []field.Octuplet {
	res := make([]field.Octuplet, len(exts))
	for i := range exts {
		e := exts[i]
		res[i] = field.Octuplet{
			e.B0.A0, e.B0.A1,
			e.B1.A0, e.B1.A1,
			e.B2.A0, e.B2.A1,
		}
	}
	return res
}

// openRunningQueryExt builds the Merkle opening data for query index s across
// running extension folding levels. Input-tree openings are carried separately.
func openRunningQueryExt(s int, layers [][]field.Ext, trees []*Tree, numRounds int) RunningQuery {
	if numRounds <= 1 {
		return nil
	}
	q := make(RunningQuery, numRounds-1)
	for j := 1; j < numRounds; j++ {

		var (
			base = s >> j
			path = trees[j].OpenBranch(base)
		)

		// Each fold halves the layer, so layer j has half the entries of layer
		// j-1. base = s>>j is the bit-reversed position of the query in layer j.
		if len(layers[j])*2 != len(layers[j-1]) {
			panic("fri: openRunningQueryExt: layers must halve at each round")
		}

		q[j-1] = QueryLayer{path}
	}

	return q
}

// inputPair is one fold round's conjugate values (self, sibling), whether
// resolved by PCS reconstruction (round 0, auxiliary levels) or decoded
// directly from a running FRI tree's leaf (every other round).
type inputPair struct {
	Self    field.Ext
	Sibling field.Ext
}

func authenticateQueryLayer(
	label string,
	opening QueryLayer,
	roots QueryLayerRoots,
	base int,
) (Branch, error) {
	first, err := authenticateQueryLayerRoots(label, opening, roots, func(Branch) (int, error) {
		return base, nil
	})
	if err != nil {
		return Branch{}, err
	}
	return first, nil
}

func authenticateQueryLayerRoots(
	label string,
	opening QueryLayer,
	roots QueryLayerRoots,
	branchIndex func(Branch) (int, error),
) (Branch, error) {
	if len(opening) != len(roots) {
		return Branch{}, fmt.Errorf("%s: has %d tree openings, want %d roots", label, len(opening), len(roots))
	}
	if len(opening) == 0 {
		return Branch{}, fmt.Errorf("%s: opening is empty", label)
	}
	for i, branch := range opening {
		idx, err := branchIndex(branch)
		if err != nil {
			return Branch{}, fmt.Errorf("%s tree %d: leaf index: %w", label, i, err)
		}
		root, err := branch.RecoverRoot(idx)
		if err != nil {
			return Branch{}, fmt.Errorf("%s tree %d: recover root: %w", label, i, err)
		}
		if root != roots[i] {
			return Branch{}, fmt.Errorf("%s tree %d: Merkle proof invalid", label, i)
		}
	}
	return opening[0], nil
}
