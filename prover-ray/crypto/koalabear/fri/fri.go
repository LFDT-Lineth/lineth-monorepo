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
		for j := 0; j <= numRounds; j++ {
			res.domains[j] = fft.NewDomain(uint64(n) >> j)
		}
	}
	res.domainsLight = make([]domainLight, numRounds+1)
	for j := 0; j <= numRounds; j++ {
		g, err := koalabear.Generator(uint64(n) >> j)
		if err != nil {
			return Params{}, err
		}
		res.domainsLight[j] = domainLight{cardinality: uint64(n) >> j, generator: g}

	}

	return res, nil
}

type domainLight struct {
	cardinality uint64
	generator   field.Element
}

// QueryLayer holds the two opened values and a single Merkle proof for one
// folding level. Exactly one rail is populated, selected by Field.
// LeafP = layer[base], LeafQ = layer[base + Nⱼ/2] where base = s % (Nⱼ/2).
type QueryLayer Branch // authenticates the pair; depth = log₂(Nⱼ/2)

// Query holds the opening data for one full query path across all r levels.
type Query []QueryLayer // len = numRounds

// Level holds one polynomial introduced at the folding round where the running
// polynomial's degree matches Level.D. Tree is the pre-built paired-leaf Merkle
// tree for Evals; build it with Params.BuildLevelTree or Params.BuildLevelTreeExt
// so the leaf/node hashers match.
type Level struct {
	D     int
	Evals []field.Ext
	Tree  *Tree
}

// Proof is the complete multi-degree FRI proof. Level polynomial Merkle roots
// are NOT stored here — they are passed externally to Verify (the caller
// commits to those polynomials before invoking FRI).
type Proof struct {
	// LevelQueries[l-1][k] = opening for levels[l].Evals at outer query k.
	LevelQueries [][]QueryLayer

	// Running-polynomial FRI path
	FRIRoots     []field.Octuplet // Merkle roots for running poly T_1..T_{r-1}
	FinalPolyExt []field.Ext
	FRIQueries   []Query // len = NumQueries
}

// FullDomainGenerator returns the generator of the full evaluation domain (layer 0, size N).
func (p Params) FullDomainGenerator() field.Element {
	return p.domains[0].Generator
}

// ────────────────────────────────────────────────────────────────────────────────
// Prove — multi-degree FRI prover
// ────────────────────────────────────────────────────────────────────────────────

// provePlan is the validated, precomputed schedule that NewProverState derives
// from the caller-supplied levels. It answers two questions the commit/query
// phases need:
//
//   - numLevels: how many committed polynomials are in play. levels[0] is the
//     main degree-D polynomial; levels[1..numLevels-1] are the lower-degree
//     polynomials batched in mid-fold. numLevels-1 is the count of extra levels.
//
//   - levelAtRound: the multi-degree FRI schedule, mapping a folding round j to
//     the level index l (1-based) introduced at that round. A level of size
//     levels[l].D is mixed into the fold output (batched with α²) at round
//     jl = log2(p.D / levels[l].D) — precisely when the running polynomial has
//     folded down to that level's degree. ProverState.Fold consults this map to
//     know when to batch each extra level, and the verifier rebuilds the same
//     map to replay the batching.
type provePlan struct {
	numLevels    int
	levelAtRound map[int]int
}

// buildProvePlan validates levels and computes the provePlan schedule. It
// enforces that levels[0].D == p.D with a populated single-rail evaluation
// vector of length N and a non-nil tree, that every extra level is a power-of-two
// degree on the same rail with the matching evaluation length and tree, and that
// no two levels are introduced at the same folding round. levels must already be
// sorted in decreasing order of D (Prove does this).
func buildProvePlan(p Params, levels []Level) (provePlan, error) {
	var plan provePlan
	if len(levels) == 0 {
		return plan, fmt.Errorf("fri: Prove: at least one level required")
	}
	if levels[0].D != p.D {
		return plan, fmt.Errorf("fri: Prove: levels[0].D=%d must equal p.D=%d", levels[0].D, p.D)
	}
	if len(levels[0].Evals) != p.N {
		return plan, fmt.Errorf("fri: Prove: levels[0].Evals length %d != N=%d", len(levels[0].Evals), p.N)
	}
	if levels[0].Tree == nil {
		return plan, fmt.Errorf("fri: Prove: levels[0].Tree is nil")
	}

	plan.numLevels = len(levels)

	// Build levelAtRound: folding round j → level index l (1-based).
	plan.levelAtRound = make(map[int]int, plan.numLevels-1)
	for l := 1; l < plan.numLevels; l++ {
		if levels[l].D <= 0 || levels[l].D&(levels[l].D-1) != 0 {
			return plan, fmt.Errorf("fri: Prove: levels[%d].D=%d is not a positive power of two", l, levels[l].D)
		}

		jl := utils.Log2Ceil(p.D / levels[l].D)
		if jl < 1 || jl >= p.numRounds {
			return plan, fmt.Errorf(
				"fri: Prove: levels[%d].D=%d gives intro round %d, must be in 1..%d",
				l, levels[l].D, jl, p.numRounds-1)
		}
		if _, dup := plan.levelAtRound[jl]; dup {
			return plan, fmt.Errorf("fri: Prove: two levels share intro round %d", jl)
		}
		plan.levelAtRound[jl] = l
		Nl := p.N >> jl
		if len(levels[l].Evals) != Nl {
			return plan, fmt.Errorf("fri: Prove: levels[%d].Evals length %d != N_l=%d", l, len(levels[l].Evals), Nl)
		}
		if levels[l].Tree == nil {
			return plan, fmt.Errorf("fri: Prove: levels[%d].Tree is nil", l)
		}
	}

	return plan, nil
}

// ────────────────────────────────────────────────────────────────────────────────
// Verify — multi-degree FRI verifier
// ────────────────────────────────────────────────────────────────────────────────

// Verify checks a multi-degree FRI proof.
//
// levelRoots[l] is the Merkle root of levels[l].Evals (committed by the caller
// before invoking FRI). levelRoots[0] plays the role of "root0" in
// single-degree FRI.
//
// levelDs[l] is the polynomial-size parameter D for level l; levelDs[0] must
// equal p.D and the slice must be ordered consistently with how Prove was
// called (i.e. decreasing D).
//
// ts must be in the same state as when Prove was called.
func Verify(p Params, levelRoots []field.Octuplet, levelDs []int, prf Proof,
	alphas []field.Ext, openedPositions []int) error {

	if len(levelDs) == 0 {
		return fmt.Errorf("fri: Verify: at least one level required")
	}
	if len(levelRoots) != len(levelDs) {
		return fmt.Errorf("fri: Verify: levelRoots has %d entries, levelDs has %d", len(levelRoots), len(levelDs))
	}
	if levelDs[0] != p.D {
		return fmt.Errorf("fri: Verify: levelDs[0]=%d must equal p.D=%d", levelDs[0], p.D)
	}

	numLevels := len(levelDs)
	numExtraLevels := numLevels - 1

	wantFRIRoots := p.numRounds - 1
	if p.numRounds <= 1 {
		wantFRIRoots = 0
	}
	if len(prf.FRIRoots) != wantFRIRoots {
		return fmt.Errorf("fri: Verify: proof has %d FRI roots, want %d", len(prf.FRIRoots), wantFRIRoots)
	}
	if len(prf.FRIQueries) != p.NumQueries {
		return fmt.Errorf("fri: Verify: proof has %d FRI queries, want %d", len(prf.FRIQueries), p.NumQueries)
	}
	if len(prf.LevelQueries) != numExtraLevels {
		return fmt.Errorf("fri: Verify: proof has %d level query sets, want %d", len(prf.LevelQueries), numExtraLevels)
	}
	for l, qs := range prf.LevelQueries {
		if len(qs) != p.NumQueries {
			return fmt.Errorf("fri: Verify: proof has %d queries for extra level %d, want %d", len(qs), l+1, p.NumQueries)
		}
	}
	if len(prf.FinalPolyExt) == 0 {
		return fmt.Errorf("fri: Verify: ext final field with empty FinalPolyExt")
	}

	// levelAtRound: folding round j → level index l (1-based).
	levelAtRound := make(map[int]int, numExtraLevels)
	for l := 1; l < numLevels; l++ {
		if levelDs[l] <= 0 || levelDs[l]&(levelDs[l]-1) != 0 {
			return fmt.Errorf("fri: Verify: levelDs[%d]=%d is not a positive power of two", l, levelDs[l])
		}
		ratio := p.D / levelDs[l]
		if ratio <= 0 || ratio*levelDs[l] != p.D || ratio&(ratio-1) != 0 {
			return fmt.Errorf("fri: Verify: levelDs[%d]=%d does not divide p.D=%d by a power-of-two ratio", l, levelDs[l], p.D)
		}
		jl := utils.Log2Ceil(ratio)
		if jl < 1 || jl >= p.numRounds {
			return fmt.Errorf(
				"fri: Verify: levelDs[%d]=%d gives intro round %d, must be in 1..%d",
				l, levelDs[l], jl, p.numRounds-1)
		}
		if _, dup := levelAtRound[jl]; dup {
			return fmt.Errorf("fri: Verify: two levels share intro round %d", jl)
		}
		levelAtRound[jl] = l
	}

	// Structural validation: reject a malformed proof here, before any hashing,
	// so that a missing or extra entry can never cause an out-of-bounds access or
	// a silently-ignored field later in verification.
	if len(openedPositions) < p.NumQueries {
		return fmt.Errorf("fri: Verify: %d opened positions, need at least %d",
			len(openedPositions), p.NumQueries)
	}
	if len(alphas) < p.numRounds {
		return fmt.Errorf("fri: Verify: %d folding challenges, need at least %d",
			len(alphas), p.numRounds)
	}
	if want := p.N >> p.numRounds; len(prf.FinalPolyExt) != want {
		return fmt.Errorf("fri: Verify: FinalPolyExt has %d entries, want %d",
			len(prf.FinalPolyExt), want)
	}
	for k := 0; k < p.NumQueries; k++ {
		if s := openedPositions[k]; s < 0 || s >= p.N {
			return fmt.Errorf("fri: Verify: opened position %d out of range [0,%d)", s, p.N)
		}
		q := prf.FRIQueries[k]
		if len(q) != p.numRounds {
			return fmt.Errorf("fri: Verify: query %d has %d layers, want %d", k, len(q), p.numRounds)
		}
		for j := 0; j < p.numRounds; j++ {
			if err := checkBranchShape(Branch(q[j]), p.N>>j); err != nil {
				return fmt.Errorf("fri: Verify: query %d round %d: %w", k, j, err)
			}
		}
		for jl, li := range levelAtRound {
			if err := checkBranchShape(Branch(prf.LevelQueries[li-1][k]), p.N>>jl); err != nil {
				return fmt.Errorf("fri: Verify: query %d extra level %d: %w", k, li, err)
			}
		}
	}

	// Assemble FRI running-polynomial roots: roots[0] is the level-0 root;
	// roots[1..r-1] come from prf.FRIRoots.
	roots := make([]field.Octuplet, p.numRounds)
	roots[0] = levelRoots[0]
	for j := 1; j < p.numRounds; j++ {
		roots[j] = prf.FRIRoots[j-1]
	}

	var levelRootsExtra []field.Octuplet
	if numExtraLevels > 0 {
		levelRootsExtra = levelRoots[1:]
	}

	return verifyExt(p, levelRoots, levelRootsExtra, levelAtRound, roots, prf, alphas, openedPositions)
}

func verifyExt(
	p Params,
	levelRoots, levelRootsExtra []field.Octuplet,
	levelAtRound map[int]int,
	roots []field.Octuplet,
	prf Proof,
	alphas []field.Ext,
	queryIndexes []int,
) error {

	numLevels := len(levelRoots)
	numExtraLevels := numLevels - 1

	for k := 0; k < p.NumQueries; k++ {

		s := queryIndexes[k]

		var levelQueriesForQuery []QueryLayer
		if numExtraLevels > 0 {
			levelQueriesForQuery = make([]QueryLayer, numExtraLevels)
			for l := 0; l < numExtraLevels; l++ {
				levelQueriesForQuery[l] = prf.LevelQueries[l][k]
			}
		}

		if err := checkQueryExt(s, prf.FRIQueries[k], levelQueriesForQuery, levelRootsExtra,
			levelAtRound, roots, prf.FinalPolyExt, alphas, p); err != nil {
			return fmt.Errorf("fri: Verify: query %d failed: %w", k, err)
		}
	}

	return nil
}

// checkBranchShape verifies that a branch has the shape of an opening into a
// complete binary tree with numLeaves leaves: exactly log2(numLeaves) siblings,
// and one auxiliary sibling per sibling. This lets Verify reject branches with a
// missing or extra node before RecoverRoot walks them.
func checkBranchShape(b Branch, numLeaves int) error {
	want := utils.Log2Ceil(numLeaves)
	if len(b.Siblings) != want {
		return fmt.Errorf("branch has %d siblings, want %d", len(b.Siblings), want)
	}
	if len(b.AuxSiblings) != want {
		return fmt.Errorf("branch has %d aux siblings, want %d", len(b.AuxSiblings), want)
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
// back into the next round. It can optionally mix in a second auxiliary fold;
// when aux is non-empty we expect len(aux) == len(layer)/2 and aux to be in the
// same bit-reversed half-domain order as the output.
//
// The folding formula, writing x = g^i for the natural-order domain point of
// pair j (i.e. i = bitReverse(j) over the half-domain):
//
//	next[j] = (layer[2j] + layer[2j+1]) / 2
//	        + alpha   * (layer[2j] - layer[2j+1]) / (2x)
//	        + alpha^2 * aux[j]                            // only when aux given
func foldLayerInternally(
	layer []field.Ext,
	aux []field.Ext,
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

	var (
		half   = len(layer) / 2
		next   = make([]field.Ext, half)
		alpha2 = new(field.Ext).Square(&alpha)
	)

	// invTwiddles[j] holds (1/2)·x⁻¹ for pair j, where x = g^i is its
	// natural-order domain point. We build the powers g⁻ⁱ/2 in natural order
	// then bit-reverse the slice so that index j lines up with the bit-reversed
	// layout of layer. This mirrors Plonky3, which bit-reverses its
	// halve_inv_powers (reverse_slice_index_bits) for the very same reason.
	invTwiddles := make([]field.Element, half)
	genPowI := invTwo
	for i := 0; i < half; i++ {
		invTwiddles[i] = genPowI
		genPowI.Mul(&genPowI, &domain.GeneratorInv)
	}
	gutils.BitReverse(invTwiddles)

	for j := 0; j < half; j++ {
		p, q := layer[2*j], layer[2*j+1]

		var sum, diff field.Ext
		sum.Add(&p, &q)
		sum.MulByElement(&sum, &invTwo)

		diff.Sub(&p, &q)
		diff.MulByElement(&diff, &invTwiddles[j])
		diff.Mul(&diff, &alpha)

		next[j].Add(&sum, &diff)

		var auxTerm field.Ext

		// if there is an aux, add it.
		// @alex: this could be expanded in 2 loops to avoid rechecking len(aux)
		// at every step.
		if len(aux) > 0 {
			auxTerm.Mul(&aux[j], alpha2)
			next[j].Add(&next[j], &auxTerm)
		}
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

// openQueryExt builds the Merkle opening data for query index s across all r
// extension folding levels.
func openQueryExt(s int, layers [][]field.Ext, trees []*Tree, numRounds int) Query {
	q := make(Query, numRounds)
	for j := 0; j < numRounds; j++ {

		var (
			base = s >> j
			path = trees[j].OpenBranch(base)
		)

		// Each fold halves the layer, so layer j has half the entries of layer
		// j-1. base = s>>j is the bit-reversed position of the query in layer j.
		if j > 0 && len(layers[j])*2 != len(layers[j-1]) {
			panic("fri: openQueryExt: layers must halve at each round")
		}

		q[j] = QueryLayer(path)
	}

	return q
}

func checkQueryExt(s int, fq Query,
	levelQueriesForQuery []QueryLayer,
	levelRoots []field.Octuplet,
	levelAtRound map[int]int,
	roots []field.Octuplet,
	finalPoly []field.Ext,
	alphas []field.Ext,
	p Params) error {

	for j := 0; j < p.numRounds; j++ {

		var (
			Nj     = int(p.domainsLight[j].cardinality)
			kj     = bits.TrailingZeros(uint(Nj)) // log₂(Nⱼ)
			base   = s >> j                       // bit-reversed position of the query in layer j
			branch = Branch(fq[j])
		)

		// Authenticate the opened leaf against the round-j root. The hashing now
		// lives in the tree itself (Branch.RecoverRoot replays hashNode up to the
		// root), so there is no separate leaf/node hasher to call.
		root, err := branch.RecoverRoot(base)
		if err != nil {
			return fmt.Errorf("round %d: recover root: %w", j, err)
		}
		if root != roots[j] {
			return fmt.Errorf("round %d: Merkle proof invalid (base=%d)", j, base)
		}

		// The opened leaf and its deepest sibling are the conjugate pair p(x),
		// p(-x): in bit-reversed order the conjugates sit at adjacent positions
		// base and base^1, which are sibling leaves. We don't need to know which
		// side base landed on — the fold is invariant under swapping (x, p(x),
		// p(-x)) → (-x, p(-x), p(x)).
		if len(branch.Siblings) == 0 {
			return fmt.Errorf("round %d: branch carries no sibling", j)
		}
		self, err := octupletToExt(branch.Leaf)
		if err != nil {
			return fmt.Errorf("round %d: decode leaf: %w", j, err)
		}
		sib, err := octupletToExt(branch.Siblings[len(branch.Siblings)-1])
		if err != nil {
			return fmt.Errorf("round %d: decode sibling: %w", j, err)
		}

		// x is the domain point of the opened leaf. The codeword is bit-reversed,
		// so the natural-order index of position base is bitReverse(base) and
		// x = gⱼ^{bitReverse(base)} with gⱼ the size-Nⱼ generator.
		xExp := int(bits.Reverse64(uint64(base)) >> (64 - kj))
		var x, xInv field.Element
		x.Exp(p.domainsLight[j].generator, big.NewInt(int64(xExp)))
		xInv.Inverse(&x)

		// fold: (self + sib)/2 + alpha · (self - sib)/(2x)
		var sum, diff, expected field.Ext
		sum.Add(&self, &sib)
		sum.MulByElement(&sum, &p.invTwo)
		diff.Sub(&self, &sib)
		diff.MulByElement(&diff, &p.invTwo)
		diff.MulByElement(&diff, &xInv)
		diff.Mul(&diff, &alphas[j])
		expected.Add(&sum, &diff)

		var alpha2, term field.Ext
		alpha2.Square(&alphas[j])

		// Mix in the auxiliary half-codeword: the level entering at round j+1,
		// batched with alpha², exactly as foldLayerInternally does on the prover.
		if li, ok := levelAtRound[j+1]; ok {
			var (
				lv    = Branch(levelQueriesForQuery[li-1])
				lvIdx = s >> (j + 1) // the level's codeword shares layer j+1's indexing
			)
			lvRoot, err := lv.RecoverRoot(lvIdx)
			if err != nil {
				return fmt.Errorf("level %d: recover root: %w", li, err)
			}
			if lvRoot != levelRoots[li-1] {
				return fmt.Errorf("level %d: Merkle proof invalid", li)
			}
			aux, err := octupletToExt(lv.Leaf)
			if err != nil {
				return fmt.Errorf("level %d: decode leaf: %w", li, err)
			}

			term.Mul(&aux, &alpha2)
			expected.Add(&expected, &term)
		}

		// The fold output must equal the queried leaf of the next layer (whose
		// position is base>>1 = s>>(j+1)); at the last round, the final polynomial.
		if j < p.numRounds-1 {
			next, err := octupletToExt(Branch(fq[j+1]).Leaf)
			if err != nil {
				return fmt.Errorf("round %d: decode next leaf: %w", j, err)
			}
			if !expected.Equal(&next) {
				return fmt.Errorf("round %d: folded value mismatch with round %d leaf", j, j+1)
			}
		} else {
			finalVal := finalPoly[s>>p.numRounds]
			if !expected.Equal(&finalVal) {
				return fmt.Errorf("round %d (final): folded value does not match FinalPoly", j)
			}
		}
	}

	return nil
}
