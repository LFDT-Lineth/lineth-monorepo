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
	LogCodewordSize  uint8 // log2 of the codeword domain size
	LogPlainTextSize uint8 // log2 of the plaintext polynomial size; always numRounds + logFinalPolySize
	NumQueries       uint  // number of independent queries (controls soundness error ≈ (1-δ)^Q)

	logFinalPolySize uint8         // logFinalPolySize = log2(the final polynomial's degree + 1)
	domains          []*fft.Domain // domains[j] has cardinality 2^(LogCodewordSize-j), generator ωⱼ
	domainsLight     []domainLight // domainLight stores only the cardinality and the domain generator
}

func (p Params) numRounds() uint8 { return p.LogPlainTextSize - p.logFinalPolySize }

type Option func(c *Params) error

// LogFinalPolySize sets the log2 of the number of coefficients FRI folds down
// to before stopping (default 0: fold to a single constant). Folding stops
// earlier the larger this is, trading fold rounds for a larger revealed
// final polynomial.
func LogFinalPolySize(log2Size uint8) Option {
	return func(c *Params) error {
		c.logFinalPolySize = log2Size
		return nil
	}
}

const MaxLogCodewordSize = 40

// NewParams constructs and validates a Params, precomputing r+1 domains and inv(2).
func NewParams(
	logCodewordSize, logPlainTextSize uint8, numQueries uint,
	opts ...Option,
) (Params, error) {
	res := Params{
		LogCodewordSize:  logCodewordSize,
		LogPlainTextSize: logPlainTextSize,
		NumQueries:       numQueries,
	}

	for _, opt := range opts {
		if err := opt(&res); err != nil {
			return Params{}, err
		}
	}

	if logCodewordSize > MaxLogCodewordSize {
		return Params{}, fmt.Errorf("fri: codeword size too large: 2^%d > 2^%d", logCodewordSize, MaxLogCodewordSize)
	}
	if logPlainTextSize >= logCodewordSize {
		return Params{}, fmt.Errorf("fri: want codeword size > plaintext size, got 2^%d ≤ 2^%d",
			logCodewordSize, logPlainTextSize)
	}
	if numQueries <= 0 {
		return Params{}, fmt.Errorf("fri: numQueries must be positive, got %d", numQueries)
	}
	if res.logFinalPolySize > logPlainTextSize {
		return Params{}, fmt.Errorf("fri: final poly size 2^%d exceeds plaintext size 2^%d",
			res.logFinalPolySize, logPlainTextSize)
	}

	res.domains = make([]*fft.Domain, res.numRounds()+1)
	for j := range res.numRounds() + 1 {
		res.domains[j] = fft.NewDomain(uint64(1) << (logCodewordSize - j))
	}
	res.domainsLight = make([]domainLight, res.numRounds()+1)
	for j := range res.numRounds() + 1 {
		g, err := koalabear.Generator(uint64(1) << (logCodewordSize - j))
		if err != nil {
			return Params{}, err
		}
		res.domainsLight[j] = domainLight{cardinality: uint64(1) << (logCodewordSize - j), generator: g}

	}

	return res, nil
}

// restrictTo returns a Params that runs FRI over the top sub-domain of plaintext
// size 2^topSizeLog2 (which must be <= PlainTextSize), reusing this Params'
// precomputed domains via a slice offset. It lets a single statically-sized
// Params — built once at the maximum supported size — drive proofs whose
// witness is smaller: the number of fold rounds becomes
// topSizeLog2-logFinalPolySize (witness-dependent) rather than p.numRounds,
// and the final polynomial still has size 2^logFinalPolySize. No domains
// are rebuilt and no zero rounds are folded. offset always lands on
// p.numRounds (in the unrestricted array) regardless of topSizeLog2, so
// res.domains[res.numRounds] is always p.domains[p.numRounds] -- the one
// domain NewParams always builds.
func (p Params) restrictTo(topSizeLog2 uint8) (Params, error) {
	if topSizeLog2 < p.logFinalPolySize || topSizeLog2 > p.LogPlainTextSize {
		return Params{}, fmt.Errorf("fri: restrictTo: top size 2^%d outside [2^%d, PlainTextSize=2^%d]",
			topSizeLog2, p.logFinalPolySize, p.LogPlainTextSize)
	}
	offset := p.LogPlainTextSize - topSizeLog2
	res := Params{
		LogCodewordSize:  p.LogCodewordSize - offset,
		LogPlainTextSize: topSizeLog2,
		NumQueries:       p.NumQueries,
		logFinalPolySize: p.logFinalPolySize,
		domains:          p.domains[offset:],
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
// polynomial's codeword length matches len(Columns[0].Evals). Trees are the
// pre-built paired-leaf Merkle trees backing it.
type Level struct {
	Trees []*Tree

	Columns             []quotientColumn
	ClaimPointIndexes   map[field.Ext]int
	ClaimPoints         []field.Ext
	DenominatorInverses []field.Ext
}

// EvalsAt returns this level's evaluations, combined with the running codeword.
// running seeds the ladder, so its contribution is weighted by alphaDeep^len(Columns).
func (l Level) EvalsAt(alphaDeep field.Ext, running []field.Ext) []field.Ext {
	evals := make([]field.Ext, len(running))
	copy(evals, running)
	for pos := range evals {
		for c := len(l.Columns) - 1; c >= 0; c-- {
			column := l.Columns[c]
			var columnSum field.Ext
			for _, claim := range column.Claims {
				pointIdx := l.ClaimPointIndexes[claim.Point]
				inv := l.DenominatorInverses[pos*len(l.ClaimPoints)+pointIdx]

				var numerator, term field.Ext
				numerator.Sub(&column.Evals[pos], &claim.Value)
				term.Mul(&numerator, &inv)
				columnSum.Add(&columnSum, &term)
			}

			evals[pos].Mul(&evals[pos], &alphaDeep)
			evals[pos].Add(&evals[pos], &columnSum)
		}
	}
	return evals
}

// Proof is the complete multi-degree FRI proof. Level polynomial Merkle roots
// are NOT stored here — they are passed externally to Verify (the caller
// commits to those polynomials before invoking FRI).
type Proof struct {
	// Running-polynomial FRI path
	RoundRoots []field.Octuplet // Merkle roots for running poly T_1..T_{r-1}
	// FinalPoly holds the folded polynomial's 2^logFinalPolySize coefficients.
	FinalPoly      []field.Ext
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
// level index l introduced there.
type provePlan struct {
	numLevels    uint8
	levelAtRound map[uint8]uint8
}

// buildProvePlan validates levels and computes the provePlan schedule. It
// enforces no two levels are introduced at the same folding round, and that
// exactly one level is introduced at round 0 (i.e. with codeword length 2^p.LogCodewordSize).
func buildProvePlan(p Params, levels []Level) (provePlan, error) {
	var plan provePlan
	if len(levels) == 0 || len(levels) > 255 {
		return plan, fmt.Errorf("fri: Prove: need between 1 and 255 levels, got %d", len(levels))
	}

	plan.numLevels = uint8(len(levels))
	plan.levelAtRound = make(map[uint8]uint8, plan.numLevels)
	for l := range plan.numLevels {
		if len(levels[l].Columns) == 0 {
			return plan, fmt.Errorf("fri: Prove: levels[%d] has no columns", l)
		}
		logCodewordLen := uint8(utils.Log2Ceil(len(levels[l].Columns[0].Evals)))
		if logCodewordLen > p.LogCodewordSize {
			return plan,
				fmt.Errorf("fri: Prove: levels[%d] codeword length 2^%d exceeds p.LogCodewordSize=%d",
					l, logCodewordLen, p.LogCodewordSize)
		}

		jl := p.LogCodewordSize - logCodewordLen
		if jl > p.numRounds() {
			return plan, fmt.Errorf(
				"fri: Prove: levels[%d] codeword 2^%d gives intro round %d, which exceeds numRounds %d",
				l, logCodewordLen, jl, p.numRounds())
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
		return plan, fmt.Errorf("fri: Prove: no level introduced at round 0 (there must be a top level)")
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
	var wantRoundRoots uint8
	if p.numRounds() > 0 {
		wantRoundRoots = p.numRounds() - 1
	}
	wantLayersPerRQuery := wantRoundRoots
	if len(prf.RoundRoots) != int(wantRoundRoots) {
		return fmt.Errorf("fri: pcs.Verify: proof has %d round roots, want %d", len(prf.RoundRoots), wantRoundRoots)
	}
	if len(prf.RunningQueries) != int(p.NumQueries) {
		return fmt.Errorf(
			"fri: pcs.Verify: proof has %d running queries, want %d", len(prf.RunningQueries), p.NumQueries,
		)
	}
	// FinalPoly holds the folded polynomial's coefficients directly (not its
	// codeword): its length alone bounds the degree.
	if want := 1 << p.logFinalPolySize; len(prf.FinalPoly) != want {
		return fmt.Errorf("fri: pcs.Verify: FinalPoly has %d entries, want %d", len(prf.FinalPoly), want)
	}
	if len(foldAlphas) < int(p.numRounds()) {
		return fmt.Errorf("fri: pcs.Verify: %d folding challenges, need at least %d", len(foldAlphas), p.numRounds())
	}
	for k, q := range prf.RunningQueries {
		if len(q) != int(wantLayersPerRQuery) {
			return fmt.Errorf(
				"fri: pcs.Verify: query %d has %d running layers, want %d", k, len(q), wantLayersPerRQuery)
		}
		if codewordSize, s := 1<<p.LogCodewordSize, positions[k]; s < 0 || s >= codewordSize {
			return fmt.Errorf("fri: pcs.Verify: opened position %d out of range [0,%d)", s, codewordSize)
		}
	}
	return nil
}

// resolvedQuery holds every fold input for one query, already authenticated
// against the committed trees and (for every level) reconstructed by the
// caller. checkFolds consumes only this: it never touches a row, a root, a
// branch, or alpha_DEEP.
type resolvedQuery struct {
	Rounds []inputPair         // Rounds[j] = (self, sibling) of the running codeword at round j (unused at j=0)
	Aux    map[uint8]inputPair // Aux[j] = the pair of the level introduced at round j, if any (always present at j=0).
	Final  field.Ext           // the final-polynomial target for this query
}

// checkFolds verifies the FRI fold recurrence for every query against values
// the caller has already authenticated and reconstructed: pure arithmetic,
// no Merkle proof or row ever passes through it.
func checkFolds(p Params, resolved []resolvedQuery, foldAlphas []field.Ext, positions []int) error {
	for queryIdx, rq := range resolved {
		s := positions[queryIdx]
		xInv := domainPoint(p.domainsLight[0], s)
		xInv.Inverse(&xInv)

		for j := range p.numRounds() {
			self, sib := rq.Rounds[j].Self, rq.Rounds[j].Sibling

			// A level introduced at this round takes over as the primary
			// pair being folded (it already has the running codeword baked
			// in, see resolvedQuery.Aux).
			if levelPair, ok := rq.Aux[j]; ok {
				self, sib = levelPair.Self, levelPair.Sibling
			}

			// fold: (self + sib)/2 + alpha · (self - sib)/(2x)
			var sum, diff field.Ext
			sum.Add(&self, &sib)
			diff.Sub(&self, &sib)
			diff.MulByElement(&diff, &xInv)
			diff.Mul(&diff, &foldAlphas[j])
			sum.Add(&sum, &diff)
			sum.Halve()

			// The fold output must equal the queried leaf of the next layer (whose
			// position is base>>1 = s>>(j+1)); at the last round, the final polynomial.
			if j < p.numRounds()-1 {
				if !sum.Equal(&rq.Rounds[j+1].Self) {
					return fmt.Errorf("fri: pcs.Verify: query %d round %d: folded value mismatch with round %d leaf",
						queryIdx, j, j+1)
				}
			} else if !sum.Equal(&rq.Final) {
				return fmt.Errorf("fri: pcs.Verify: query %d round %d (final): folded value does not match FinalPoly",
					queryIdx, j)
			}

			xInv.Square(&xInv)
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
// The folding formula, writing x = g^i for the natural-order domain point of
// pair j (i.e. i = bitReverse(j) over the half-domain):
//
//	next[j] = (layer[2j] + layer[2j+1]) / 2 + alpha * (layer[2j] - layer[2j+1]) / (2x)
func foldLayerInternally(layer []field.Ext, alpha field.Ext, domain *fft.Domain) []field.Ext {

	// domain is the input layer's domain: its generator supplies the twiddles
	// g^{-i} for the conjugate pairs, so its cardinality matches len(layer) (the
	// half-size output uses this same domain, not its own).
	if int(domain.Cardinality) != len(layer) {
		panic("fri: foldLayerInternally: len(layer) != domain.Cardinality")
	}

	var (
		half = len(layer) / 2
		next = make([]field.Ext, half)
	)

	// invTwiddles[j] holds (1/2)·x⁻¹ for pair j, where x = g^i is its
	// natural-order domain point. We build the powers g⁻ⁱ/2 in natural order
	// then bit-reverse the slice so that index j lines up with the bit-reversed
	// layout of layer.
	invTwiddles := make([]field.Element, half)
	genPowI := field.One()
	genPowI.Halve()
	for i := range half {
		invTwiddles[i] = genPowI
		genPowI.Mul(&genPowI, &domain.GeneratorInv)
	}
	gutils.BitReverse(invTwiddles)

	for j := range half {
		p, q := layer[2*j], layer[2*j+1]

		var sum, diff field.Ext
		sum.Add(&p, &q)
		sum.Halve()

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
func (st *ProverState) openRunningQueryExt(s int) RunningQuery {
	if st.p.numRounds() <= 1 {
		return nil
	}
	q := make(RunningQuery, st.p.numRounds()-1)
	for j := uint8(1); j < st.p.numRounds(); j++ {

		var (
			base = s >> j
			path = st.trees[j].OpenBranch(base)
		)

		// Each fold halves the layer, so layer j has half the entries of layer
		// j-1. base = s>>j is the bit-reversed position of the query in layer j.
		if len(st.layers[j])*2 != len(st.layers[j-1]) {
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
	roundIdx uint8,
	opening QueryLayer,
	roots QueryLayerRoots,
	base int,
) (Branch, error) {

	if len(opening) != len(roots) {
		return Branch{},
			fmt.Errorf("round %d: has %d tree openings, want %d roots", roundIdx, len(opening), len(roots))
	}
	if len(opening) == 0 {
		return Branch{}, fmt.Errorf("round %d: opening is empty", roundIdx)
	}
	for i, branch := range opening {
		root, err := branch.RecoverRoot(base)
		if err != nil {
			return Branch{}, fmt.Errorf("round %d tree %d: recover root: %w", roundIdx, i, err)
		}
		if root != roots[i] {
			return Branch{}, fmt.Errorf("round %d tree %d: Merkle proof invalid", roundIdx, i)
		}
	}
	return opening[0], nil
}
