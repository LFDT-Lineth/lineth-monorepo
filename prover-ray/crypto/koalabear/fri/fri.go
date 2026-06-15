package fri

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/fiatshamirrefactor"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/merkle"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/parallel"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poly"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/reedsolomon"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// foldParallelThreshold is the smallest half-layer size at which fan-out
// across goroutines beats the precomputed-xInv seeding overhead per chunk.
const foldParallelThreshold = 1 << 12

// Params holds the FRI configuration and precomputed per-level data.
// Build once with NewParams; reuse across many Prove/Verify calls.
type Params struct {
	N          int // 2^n: size of the evaluation domain
	D          int // 2^m: degree of the purported polynomial
	NumQueries int // number of independent queries (controls soundness error ≈ (1-δ)^Q)
	LeafHasher commitment.LeafHasher
	NodeHasher commitment.NodeHasher

	numRounds    int // numRounds = m
	invTwo       koalabear.Element
	domains      []*fft.Domain // domains[j] has cardinality N/2^j, generator ωⱼ
	domainsLight []domainLight // domainLight stores only the cardinality and the domain generator
	grinding     int           // grinding bits for PoW, on the alpha
}

type Config struct {
	WoFullDomainAllocation bool
	// Grinding is the number of grinding bits for PoW. More grinding bits => more cpu work for the
	// prover, but security goes from log_blowup * num_queries to
	// log_blowup * num_queries + query_proof_of_work_bits.
	Grinding int
}

type Option func(c *Config) error

func WoFullDomainAllocation() Option {
	return func(c *Config) error {
		c.WoFullDomainAllocation = true
		return nil
	}
}

// WithGrinding forces the folding challenges to start with nbBits at zeroes, to increase security
// It lowers the space <wrong proof x miraculously valid challenges>.
// Security goes from log_blowup * num_queries to log_blowup * num_queries + query_proof_of_work_bits.
func WithGrinding(nbBits int) Option {
	return func(c *Config) error {
		c.Grinding = nbBits
		return nil
	}
}

// NewParams constructs and validates a Params, precomputing r+1 domains and inv(2).
func NewParams(
	N, D, numQueries int,
	lh commitment.LeafHasher,
	nh commitment.NodeHasher,
	opts ...Option,
) (Params, error) {
	if N <= 0 || N&(N-1) != 0 {
		return Params{}, fmt.Errorf("fri: N must be a positive power of two, got %d", N)
	}
	if D <= 0 || D&(D-1) != 0 {
		return Params{}, fmt.Errorf("fri: D must be a positive power of two, got %d", D)
	}
	if D >= N {
		return Params{}, fmt.Errorf("fri: D must be < N, got D=%d N=%d", D, N)
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

	numRounds := log2(D) // r = m = log₂(D)

	var two, invTwo koalabear.Element
	two.SetUint64(2)
	invTwo.Inverse(&two)

	res := Params{
		N:          N,
		D:          D,
		NumQueries: numQueries,
		LeafHasher: lh,
		NodeHasher: nh,
		numRounds:  numRounds,
		invTwo:     invTwo,
		grinding:   config.Grinding,
	}

	if !config.WoFullDomainAllocation {
		res.domains = make([]*fft.Domain, numRounds+1)
		for j := 0; j <= numRounds; j++ {
			res.domains[j] = fft.NewDomain(uint64(N) >> j)
		}
	}
	res.domainsLight = make([]domainLight, numRounds+1)
	for j := 0; j <= numRounds; j++ {
		g, err := koalabear.Generator(uint64(N) >> j)
		if err != nil {
			return Params{}, err
		}
		res.domainsLight[j] = domainLight{cardinality: uint64(N) >> j, generator: g}

	}

	return res, nil
}

type domainLight struct {
	cardinality uint64
	generator   koalabear.Element
}

// QueryLayer holds the two opened values and a single Merkle proof for one
// folding level. Exactly one rail is populated, selected by Field.
// LeafP = layer[base], LeafQ = layer[base + Nⱼ/2] where base = s % (Nⱼ/2).
type QueryLayer struct {
	Field     field.Kind
	LeafPBase koalabear.Element // populated when Field == field.KindBase
	LeafQBase koalabear.Element
	LeafPExt  ext.E6 // populated when Field == field.KindExt
	LeafQExt  ext.E6
	Path      merkle.Proof // authenticates the pair; depth = log₂(Nⱼ/2)
}

// Query holds the opening data for one full query path across all r levels.
type Query struct {
	Layers []QueryLayer // len = numRounds
}

// ────────────────────────────────────────────────────────────────────────────────
// Multi-degree FRI types (see multi_degree_fri.txt for the full protocol spec)
// ────────────────────────────────────────────────────────────────────────────────

// LevelEvals stores the evaluation vector for one FRI level. Exactly one rail
// must be populated; callers must fold any extra same-degree polynomials before
// invoking FRI.
type LevelEvals struct {
	Base []koalabear.Element
	Ext  []ext.E6
}

// Field returns the populated rail. Invalid mixed/empty values are rejected by
// Prove's validation; the zero value reports field.KindBase.
func (e LevelEvals) Field() field.Kind {
	if len(e.Ext) > 0 {
		return field.KindExt
	}
	return field.KindBase
}

func (e LevelEvals) checkedField() (field.Kind, error) {
	hasBase := len(e.Base) > 0
	hasExt := len(e.Ext) > 0
	switch {
	case hasBase && hasExt:
		return field.KindBase, fmt.Errorf("both base and ext rails are populated")
	case !hasBase && !hasExt:
		return field.KindBase, fmt.Errorf("no rail is populated")
	case hasExt:
		return field.KindExt, nil
	default:
		return field.KindBase, nil
	}
}

func (e LevelEvals) Len() int {
	if e.Field() == field.KindExt {
		return len(e.Ext)
	}
	return len(e.Base)
}

// Level holds one polynomial introduced at the folding round where the running
// polynomial's degree matches Level.D. Tree is the pre-built paired-leaf Merkle
// tree for Evals; build it with Params.BuildLevelTree or Params.BuildLevelTreeExt
// so the leaf/node hashers match.
type Level struct {
	D     int
	Evals LevelEvals
	Tree  *merkle.Tree
}

// Proof is the complete multi-degree FRI proof. Level polynomial Merkle roots
// are NOT stored here — they are passed externally to Verify (the caller
// commits to those polynomials before invoking FRI).
type Proof struct {
	// LevelQueries[l-1][k] = opening for levels[l].Evals at outer query k.
	LevelQueries [][]QueryLayer

	// Running-polynomial FRI path
	FRIRoots      []hash.Digest // Merkle roots for running poly T_1..T_{r-1}
	FinalField    field.Kind
	FinalPolyBase []koalabear.Element                       // populated when FinalField == field.KindBase
	FinalPolyExt  []ext.E6                                  // populated when FinalField == field.KindExt
	FRIQueries    []Query                                   // len = NumQueries
	PoW           map[string]fiatshamirrefactor.ProofOfWork // proof of work in case grinding has nbBits > 0
}

// FullDomainGenerator returns the generator of the full evaluation domain (layer 0, size N).
func (p Params) FullDomainGenerator() koalabear.Element {
	return p.domains[0].Generator
}

// Encode converts a polynomial from Lagrange form (size D) to its evaluation
// on the full domain of size N. The result is a₀, ready to pass to Prove.
func (p Params) Encode(poly []koalabear.Element) ([]koalabear.Element, error) {
	if len(poly) != p.D {
		return nil, fmt.Errorf("fri: Encode: polynomial length %d != D=%d", len(poly), p.D)
	}
	enc := reedsolomon.NewEncoder(uint64(p.N))
	domainD := fft.NewDomain(uint64(p.D))
	return enc.Encode(poly, domainD), nil
}

// EncodeExt is the extension-field counterpart of Encode.
func (p Params) EncodeExt(poly []ext.E6) ([]ext.E6, error) {
	if len(poly) != p.D {
		return nil, fmt.Errorf("fri: EncodeExt: polynomial length %d != D=%d", len(poly), p.D)
	}
	enc := reedsolomon.NewEncoder(uint64(p.N))
	domainD := fft.NewDomain(uint64(p.D))
	return enc.EncodeExt(poly, domainD), nil
}

// BuildLevelTree builds the paired-leaf Merkle tree expected by FRI for a
// level polynomial: tree of len(layer)/2 leaves where
// leaf k = LeafHasher(encode(layer[k]) || encode(layer[k + len(layer)/2])).
func (p Params) BuildLevelTree(layer []koalabear.Element) (*merkle.Tree, error) {
	return buildTreeBase(layer, p.LeafHasher, p.NodeHasher)
}

// BuildLevelTreeExt builds the paired-leaf Merkle tree expected by FRI for an
// extension-field level polynomial.
func (p Params) BuildLevelTreeExt(layer []ext.E6) (*merkle.Tree, error) {
	return buildTreeExt(layer, p.LeafHasher, p.NodeHasher)
}

// ────────────────────────────────────────────────────────────────────────────────
// Prove — multi-degree FRI prover
// ────────────────────────────────────────────────────────────────────────────────

// Prove runs multi-degree FRI (commit + query phase) and returns a Proof together
// with the query positions. levels[0].D must equal p.D and every Level must
// contain one evaluation vector on exactly one rail. levels is sorted in-place
// in decreasing order of D.
// ts must already have been initialised with any prior-round context.
func Prove(p Params, levels []Level, ts *fiatshamirrefactor.Transcript) (Proof, []int, error) {
	sort.Slice(levels, func(i, j int) bool { return levels[i].D > levels[j].D })

	plan, err := buildProvePlan(p, levels)
	if err != nil {
		return Proof{}, nil, err
	}
	if err := registerChallenges(p, plan.introductions, ts); err != nil {
		return Proof{}, nil, err
	}

	if plan.rail == field.KindExt {
		return proveExt(p, levels, plan, ts)
	}
	return proveBase(p, levels, plan, ts)
}

type provePlan struct {
	rail          field.Kind
	numLevels     int
	introductions levelIntroductions
}

type levelIntroductions struct {
	levelAtRound map[int]int // which polynomial is introduced at round j?
	introRounds  []int       // at which round is polynomial i introduced into the fold?
}

func newLevelIntroductions(p Params, levelDs []int) (levelIntroductions, error) {
	numExtraLevels := len(levelDs) - 1
	if numExtraLevels < 0 {
		numExtraLevels = 0
	}
	res := levelIntroductions{
		levelAtRound: make(map[int]int, numExtraLevels),
		introRounds:  make([]int, len(levelDs)),
	}
	for level := range res.introRounds {
		res.introRounds[level] = -1
	}
	for level := 1; level < len(levelDs); level++ {
		levelD := levelDs[level]
		if levelD <= 0 || levelD&(levelD-1) != 0 {
			return res, fmt.Errorf("level %d D=%d is not a positive power of two", level, levelD)
		}
		ratio := p.D / levelD
		if ratio <= 0 || ratio*levelD != p.D || ratio&(ratio-1) != 0 {
			return res, fmt.Errorf("level %d D=%d does not divide p.D=%d by a power-of-two ratio", level, levelD, p.D)
		}
		round := log2(ratio)
		if round < 1 || round >= p.numRounds {
			return res, fmt.Errorf("level %d D=%d gives intro round %d, must be in 1..%d",
				level, levelD, round, p.numRounds-1)
		}
		if _, dup := res.levelAtRound[round]; dup {
			return res, fmt.Errorf("two levels share intro round %d", round)
		}
		res.levelAtRound[round] = level
		res.introRounds[level] = round
	}
	return res, nil
}

func buildProvePlan(p Params, levels []Level) (provePlan, error) {
	var plan provePlan
	if len(levels) == 0 {
		return plan, fmt.Errorf("fri: Prove: at least one level required")
	}
	if levels[0].D != p.D {
		return plan, fmt.Errorf("fri: Prove: levels[0].D=%d must equal p.D=%d", levels[0].D, p.D)
	}
	rail, err := levels[0].Evals.checkedField()
	if err != nil {
		return plan, fmt.Errorf("fri: Prove: levels[0].Evals: %w", err)
	}
	plan.rail = rail
	if levels[0].Evals.Len() != p.N {
		return plan, fmt.Errorf("fri: Prove: levels[0].Evals length %d != N=%d", levels[0].Evals.Len(), p.N)
	}
	if levels[0].Tree == nil {
		return plan, fmt.Errorf("fri: Prove: levels[0].Tree is nil")
	}

	plan.numLevels = len(levels)

	levelDs := make([]int, plan.numLevels)
	for l := range levels {
		levelDs[l] = levels[l].D
	}
	if plan.introductions, err = newLevelIntroductions(p, levelDs); err != nil {
		return plan, fmt.Errorf("fri: Prove: %w", err)
	}

	for l := 1; l < plan.numLevels; l++ {
		levelRail, err := levels[l].Evals.checkedField()
		if err != nil {
			return plan, fmt.Errorf("fri: Prove: levels[%d].Evals: %w", l, err)
		}
		if levelRail != rail {
			return plan, fmt.Errorf("fri: Prove: levels[%d] is %s, running rail is %s", l, levelRail, rail)
		}
		jl := plan.introductions.introRounds[l]
		Nl := p.N >> jl
		if levels[l].Evals.Len() != Nl {
			return plan, fmt.Errorf("fri: Prove: levels[%d].Evals length %d != N_l=%d", l, levels[l].Evals.Len(), Nl)
		}
		if levels[l].Tree == nil {
			return plan, fmt.Errorf("fri: Prove: levels[%d].Tree is nil", l)
		}
	}

	return plan, nil
}

func registerChallenges(p Params, introductions levelIntroductions, ts *fiatshamirrefactor.Transcript) error {
	if len(introductions.introRounds) > 1 {
		if err := ts.NewChallenge(gammaName()); err != nil {
			return err
		}
	}
	for j := 0; j < p.numRounds; j++ {
		if err := ts.NewChallenge(foldName(j)); err != nil {
			return err
		}
	}
	for k := 0; k < p.NumQueries; k++ {
		if err := ts.NewChallenge(queryName(k)); err != nil {
			return err
		}
	}
	return nil
}

func proveBase(p Params, levels []Level, plan provePlan, ts *fiatshamirrefactor.Transcript) (Proof, []int, error) {
	// ── Gamma computation (all level roots, including level 0, bound upfront) ─
	gammas := make([]koalabear.Element, plan.numLevels)
	if plan.numLevels > 1 {
		for l := 0; l < plan.numLevels; l++ {
			root := levels[l].Tree.Root()
			if err := ts.Bind(gammaName(), root[:]); err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: bind level l=%d: %w", l, err)
			}
		}
		challenge, err := ts.ComputeChallenge(gammaName())
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute gamma: %w", err)
		}
		var gamma koalabear.Element
		gamma.Set(&challenge[0])
		gammas[1].Set(&gamma)
		for l := 2; l < plan.numLevels; l++ {
			gammas[l].Mul(&gammas[l-1], &gamma)
		}
	}

	// ── Commit phase ─────────────────────────────────────────────────────────

	// running is the current evaluation vector; copy levels[0].Evals.Base so we own it.
	running := make([]koalabear.Element, p.N)
	copy(running, levels[0].Evals.Base)

	layers := make([][]koalabear.Element, p.numRounds+1)
	friTrees := make([]*merkle.Tree, p.numRounds)
	alphas := make([]koalabear.Element, p.numRounds)

	var prf Proof
	if p.numRounds > 1 {
		prf.FRIRoots = make([]hash.Digest, p.numRounds-1)
	}

	for j := 0; j < p.numRounds; j++ {
		// Level batching step (j > 0 only; j=0 reuses the caller-supplied levels[0].Tree).
		if j > 0 {
			if l, ok := plan.introductions.levelAtRound[j]; ok {
				gamma := gammas[l]
				// Mix γ^l * levels[l].Evals into running (pointwise).
				for k, v := range levels[l].Evals.Base {
					var term koalabear.Element
					term.Mul(&v, &gamma)
					running[k].Add(&running[k], &term)
				}
			}
		}

		// layers[j] = running after batching, before folding (= what T_j commits to).
		layers[j] = running

		var tree *merkle.Tree
		if j == 0 {
			tree = levels[0].Tree // caller-supplied, root must match running pre-fold
		} else {
			var err error
			tree, err = buildTreeBase(running, p.LeafHasher, p.NodeHasher)
			if err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: build tree layer %d: %w", j, err)
			}
		}
		friTrees[j] = tree
		root := tree.Root()

		name := foldName(j)
		if err := ts.Bind(name, root[:]); err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: bind fold %d: %w", j, err)
		}
		challenge, err := computeProverFoldChallenge(ts, name, p.grinding)
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute fold challenge %d: %w", j, err)
		}
		alphas[j].Set(&challenge[0])

		// Root of T_0 is passed to Verify separately; only T_1..T_{r-1} go in the proof.
		if j > 0 {
			prf.FRIRoots[j-1] = root
		}

		// foldLayer returns a new slice, so running for round j+1 is independent.
		running = foldLayerBase(running, alphas[j], p.domains[j], p.invTwo)
	}
	layers[p.numRounds] = running
	prf.FinalField = field.KindBase
	prf.FinalPolyBase = running
	if err := recordFoldProofsOfWork(p, &prf, ts); err != nil {
		return Proof{}, nil, fmt.Errorf("fri: Prove: record proof of work: %w", err)
	}

	if err := ts.Bind(queryName(0), transcriptBasePoly(prf.FinalPolyBase)); err != nil {
		return Proof{}, nil, fmt.Errorf("fri: Prove: bind final poly: %w", err)
	}

	// ── Query phase ───────────────────────────────────────────────────────────

	prf.FRIQueries = make([]Query, p.NumQueries)
	if plan.numLevels > 1 {
		prf.LevelQueries = make([][]QueryLayer, plan.numLevels-1)
		for l := range prf.LevelQueries {
			prf.LevelQueries[l] = make([]QueryLayer, p.NumQueries)
		}
	}

	queryPositions := make([]int, p.NumQueries)
	for k := 0; k < p.NumQueries; k++ {
		challenge, err := ts.ComputeChallenge(queryName(k))
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute query challenge %d: %w", k, err)
		}
		s := queryIndex(challenge, p.N/2)
		queryPositions[k] = s

		if k < p.NumQueries-1 {
			if err := ts.Bind(queryName(k+1), challenge[:]); err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: bind query chain %d: %w", k+1, err)
			}
		}

		q, err := openQueryBase(s, layers, friTrees, p.numRounds)
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: open FRI query %d: %w", k, err)
		}
		prf.FRIQueries[k] = q

		for l := 1; l < plan.numLevels; l++ {
			jl := plan.introductions.introRounds[l]
			Nl := p.N >> jl
			base := s % (Nl / 2)

			path, err := levels[l].Tree.OpenProof(base)
			if err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: open level query l=%d k=%d: %w", l, k, err)
			}
			prf.LevelQueries[l-1][k] = QueryLayer{
				Field:     field.KindBase,
				LeafPBase: levels[l].Evals.Base[base],
				LeafQBase: levels[l].Evals.Base[base+Nl/2],
				Path:      path,
			}
		}
	}

	return prf, queryPositions, nil
}

func proveExt(p Params, levels []Level, plan provePlan, ts *fiatshamirrefactor.Transcript) (Proof, []int, error) {
	// ── Gamma computation (all level roots, including level 0, bound upfront) ─
	gammas := make([]ext.E6, plan.numLevels)
	if plan.numLevels > 1 {
		for l := 0; l < plan.numLevels; l++ {
			root := levels[l].Tree.Root()
			if err := ts.Bind(gammaName(), root[:]); err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: bind level l=%d: %w", l, err)
			}
		}
		challenge, err := ts.ComputeChallenge(gammaName())
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute gamma: %w", err)
		}
		gamma := hash.OutputToExt(challenge)
		gammas[1] = gamma
		for l := 2; l < plan.numLevels; l++ {
			gammas[l].Mul(&gammas[l-1], &gamma)
		}
	}

	running := make([]ext.E6, p.N)
	copy(running, levels[0].Evals.Ext)

	layers := make([][]ext.E6, p.numRounds+1)
	friTrees := make([]*merkle.Tree, p.numRounds)
	alphas := make([]ext.E6, p.numRounds)

	var prf Proof
	if p.numRounds > 1 {
		prf.FRIRoots = make([]hash.Digest, p.numRounds-1)
	}

	for j := 0; j < p.numRounds; j++ {
		if j > 0 {
			if l, ok := plan.introductions.levelAtRound[j]; ok {
				gamma := gammas[l]
				for k, v := range levels[l].Evals.Ext {
					var term ext.E6
					term.Mul(&v, &gamma)
					running[k].Add(&running[k], &term)
				}
			}
		}

		layers[j] = running

		var tree *merkle.Tree
		if j == 0 {
			tree = levels[0].Tree
		} else {
			var err error
			tree, err = buildTreeExt(running, p.LeafHasher, p.NodeHasher)
			if err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: build tree layer %d: %w", j, err)
			}
		}
		friTrees[j] = tree
		root := tree.Root()

		name := foldName(j)
		if err := ts.Bind(name, root[:]); err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: bind fold %d: %w", j, err)
		}
		challenge, err := computeProverFoldChallenge(ts, name, p.grinding)
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute fold challenge %d: %w", j, err)
		}
		alphas[j] = hash.OutputToExt(challenge)

		if j > 0 {
			prf.FRIRoots[j-1] = root
		}

		running = foldLayerExt(running, alphas[j], p.domains[j], p.invTwo)
	}
	layers[p.numRounds] = running
	prf.FinalField = field.KindExt
	prf.FinalPolyExt = running
	if err := recordFoldProofsOfWork(p, &prf, ts); err != nil {
		return Proof{}, nil, fmt.Errorf("fri: Prove: record proof of work: %w", err)
	}

	if err := ts.Bind(queryName(0), transcriptExtPoly(prf.FinalPolyExt)); err != nil {
		return Proof{}, nil, fmt.Errorf("fri: Prove: bind final poly: %w", err)
	}

	prf.FRIQueries = make([]Query, p.NumQueries)
	if plan.numLevels > 1 {
		prf.LevelQueries = make([][]QueryLayer, plan.numLevels-1)
		for l := range prf.LevelQueries {
			prf.LevelQueries[l] = make([]QueryLayer, p.NumQueries)
		}
	}

	queryPositions := make([]int, p.NumQueries)
	for k := 0; k < p.NumQueries; k++ {
		challenge, err := ts.ComputeChallenge(queryName(k))
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: compute query challenge %d: %w", k, err)
		}
		s := queryIndex(challenge, p.N/2)
		queryPositions[k] = s

		if k < p.NumQueries-1 {
			if err := ts.Bind(queryName(k+1), challenge[:]); err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: bind query chain %d: %w", k+1, err)
			}
		}

		q, err := openQueryExt(s, layers, friTrees, p.numRounds)
		if err != nil {
			return Proof{}, nil, fmt.Errorf("fri: Prove: open FRI query %d: %w", k, err)
		}
		prf.FRIQueries[k] = q

		for l := 1; l < plan.numLevels; l++ {
			jl := plan.introductions.introRounds[l]
			Nl := p.N >> jl
			base := s % (Nl / 2)

			path, err := levels[l].Tree.OpenProof(base)
			if err != nil {
				return Proof{}, nil, fmt.Errorf("fri: Prove: open level query l=%d k=%d: %w", l, k, err)
			}
			prf.LevelQueries[l-1][k] = QueryLayer{
				Field:    field.KindExt,
				LeafPExt: levels[l].Evals.Ext[base],
				LeafQExt: levels[l].Evals.Ext[base+Nl/2],
				Path:     path,
			}
		}
	}

	return prf, queryPositions, nil
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
func Verify(p Params, levelRoots []hash.Digest, levelDs []int, prf Proof, ts *fiatshamirrefactor.Transcript) error {
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
	for k, q := range prf.FRIQueries {
		if len(q.Layers) != p.numRounds {
			return fmt.Errorf("fri: Verify: proof FRI query %d has %d layers, want %d", k, len(q.Layers), p.numRounds)
		}
	}
	if len(prf.LevelQueries) != numExtraLevels {
		return fmt.Errorf("fri: Verify: proof has %d level query sets, want %d", len(prf.LevelQueries), numExtraLevels)
	}
	for l, qs := range prf.LevelQueries {
		if len(qs) != p.NumQueries {
			return fmt.Errorf("fri: Verify: proof has %d queries for extra level %d, want %d", len(qs), l+1, p.NumQueries)
		}
	}
	if prf.FinalField != field.KindBase && prf.FinalField != field.KindExt {
		return fmt.Errorf("fri: Verify: invalid final field %s", prf.FinalField)
	}
	if prf.FinalField == field.KindBase && len(prf.FinalPolyBase) == 0 {
		return fmt.Errorf("fri: Verify: base final field with empty FinalPolyBase")
	}
	if prf.FinalField == field.KindExt && len(prf.FinalPolyExt) == 0 {
		return fmt.Errorf("fri: Verify: ext final field with empty FinalPolyExt")
	}

	introductions, err := newLevelIntroductions(p, levelDs)
	if err != nil {
		return fmt.Errorf("fri: Verify: %w", err)
	}

	if err := registerChallenges(p, introductions, ts); err != nil {
		return err
	}

	// Assemble FRI running-polynomial roots: roots[0] is the level-0 root;
	// roots[1..r-1] come from prf.FRIRoots.
	roots := make([]hash.Digest, p.numRounds)
	roots[0] = levelRoots[0]
	for j := 1; j < p.numRounds; j++ {
		roots[j] = prf.FRIRoots[j-1]
	}

	var levelRootsExtra []hash.Digest
	if numExtraLevels > 0 {
		levelRootsExtra = levelRoots[1:]
	}

	if prf.FinalField == field.KindExt {
		return verifyExt(p, levelRoots, levelRootsExtra, introductions, roots, prf, ts)
	}
	return verifyBase(p, levelRoots, levelRootsExtra, introductions, roots, prf, ts)
}

func verifyBase(
	p Params,
	levelRoots, levelRootsExtra []hash.Digest,
	introductions levelIntroductions,
	roots []hash.Digest,
	prf Proof,
	ts *fiatshamirrefactor.Transcript,
) error {
	numLevels := len(levelRoots)
	numExtraLevels := numLevels - 1

	// ── Replay commit phase ───────────────────────────────────────────────────
	gammas := make([]koalabear.Element, numLevels)
	if numExtraLevels > 0 {
		for l := 0; l < numLevels; l++ {
			root := levelRoots[l]
			if err := ts.Bind(gammaName(), root[:]); err != nil {
				return fmt.Errorf("fri: Verify: bind level l=%d: %w", l, err)
			}
		}
		challenge, err := ts.ComputeChallenge(gammaName())
		if err != nil {
			return fmt.Errorf("fri: Verify: compute gamma: %w", err)
		}
		var gamma koalabear.Element
		gamma.Set(&challenge[0])
		gammas[1].Set(&gamma)
		for l := 2; l < numLevels; l++ {
			gammas[l].Mul(&gammas[l-1], &gamma)
		}
	}

	alphas := make([]koalabear.Element, p.numRounds)
	for j := 0; j < p.numRounds; j++ {
		name := foldName(j)
		root := roots[j]
		if err := ts.Bind(name, root[:]); err != nil {
			return fmt.Errorf("fri: Verify: bind fold %d: %w", j, err)
		}
		challenge, err := computeVerifierFoldChallenge(ts, name, p.grinding, prf.PoW)
		if err != nil {
			return fmt.Errorf("fri: Verify: compute fold challenge %d: %w", j, err)
		}
		alphas[j].Set(&challenge[0])
	}

	if err := ts.Bind(queryName(0), transcriptBasePoly(prf.FinalPolyBase)); err != nil {
		return fmt.Errorf("fri: Verify: bind final poly: %w", err)
	}

	// ── Query phase ───────────────────────────────────────────────────────────
	for k := 0; k < p.NumQueries; k++ {
		challenge, err := ts.ComputeChallenge(queryName(k))
		if err != nil {
			return fmt.Errorf("fri: Verify: compute query challenge %d: %w", k, err)
		}
		s := queryIndex(challenge, p.N/2)

		if k < p.NumQueries-1 {
			if err := ts.Bind(queryName(k+1), challenge[:]); err != nil {
				return fmt.Errorf("fri: Verify: bind query chain %d: %w", k+1, err)
			}
		}

		var levelQueriesForQuery []QueryLayer
		if numExtraLevels > 0 {
			levelQueriesForQuery = make([]QueryLayer, numExtraLevels)
			for l := 0; l < numExtraLevels; l++ {
				levelQueriesForQuery[l] = prf.LevelQueries[l][k]
			}
		}

		if err := checkQuery(s, prf.FRIQueries[k], levelQueriesForQuery, levelRootsExtra,
			introductions, gammas, roots, prf.FinalPolyBase, alphas, p); err != nil {
			return fmt.Errorf("fri: Verify: query %d failed: %w", k, err)
		}
	}

	return nil
}

func verifyExt(
	p Params,
	levelRoots, levelRootsExtra []hash.Digest,
	introductions levelIntroductions,
	roots []hash.Digest,
	prf Proof,
	ts *fiatshamirrefactor.Transcript,
) error {
	numLevels := len(levelRoots)
	numExtraLevels := numLevels - 1

	gammas := make([]ext.E6, numLevels)
	if numExtraLevels > 0 {
		for l := 0; l < numLevels; l++ {
			root := levelRoots[l]
			if err := ts.Bind(gammaName(), root[:]); err != nil {
				return fmt.Errorf("fri: Verify: bind level l=%d: %w", l, err)
			}
		}
		challenge, err := ts.ComputeChallenge(gammaName())
		if err != nil {
			return fmt.Errorf("fri: Verify: compute gamma: %w", err)
		}
		gamma := hash.OutputToExt(challenge)
		gammas[1] = gamma
		for l := 2; l < numLevels; l++ {
			gammas[l].Mul(&gammas[l-1], &gamma)
		}
	}

	alphas := make([]ext.E6, p.numRounds)
	for j := 0; j < p.numRounds; j++ {
		name := foldName(j)
		root := roots[j]
		if err := ts.Bind(name, root[:]); err != nil {
			return fmt.Errorf("fri: Verify: bind fold %d: %w", j, err)
		}
		challenge, err := computeVerifierFoldChallenge(ts, name, p.grinding, prf.PoW)
		if err != nil {
			return fmt.Errorf("fri: Verify: compute fold challenge %d: %w", j, err)
		}
		alphas[j] = hash.OutputToExt(challenge)
	}

	if err := ts.Bind(queryName(0), transcriptExtPoly(prf.FinalPolyExt)); err != nil {
		return fmt.Errorf("fri: Verify: bind final poly: %w", err)
	}

	for k := 0; k < p.NumQueries; k++ {
		challenge, err := ts.ComputeChallenge(queryName(k))
		if err != nil {
			return fmt.Errorf("fri: Verify: compute query challenge %d: %w", k, err)
		}
		s := queryIndex(challenge, p.N/2)

		if k < p.NumQueries-1 {
			if err := ts.Bind(queryName(k+1), challenge[:]); err != nil {
				return fmt.Errorf("fri: Verify: bind query chain %d: %w", k+1, err)
			}
		}

		var levelQueriesForQuery []QueryLayer
		if numExtraLevels > 0 {
			levelQueriesForQuery = make([]QueryLayer, numExtraLevels)
			for l := 0; l < numExtraLevels; l++ {
				levelQueriesForQuery[l] = prf.LevelQueries[l][k]
			}
		}

		if err := checkQueryExt(s, prf.FRIQueries[k], levelQueriesForQuery, levelRootsExtra,
			introductions, gammas, roots, prf.FinalPolyExt, alphas, p); err != nil {
			return fmt.Errorf("fri: Verify: query %d failed: %w", k, err)
		}
	}

	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func gammaName() string      { return "fri_gamma" }
func foldName(j int) string  { return fmt.Sprintf("fri_fold_%d", j) }
func queryName(k int) string { return fmt.Sprintf("fri_query_%d", k) }

func computeProverFoldChallenge(
	ts *fiatshamirrefactor.Transcript,
	name string,
	grinding int,
) ([8]koalabear.Element, error) {
	if grinding == 0 {
		return ts.ComputeChallenge(name)
	}
	return ts.ComputeChallenge(name, fiatshamirrefactor.WithGrinding(grinding))
}

func computeVerifierFoldChallenge(
	ts *fiatshamirrefactor.Transcript,
	name string,
	grinding int,
	proofsOfWork map[string]fiatshamirrefactor.ProofOfWork,
) ([8]koalabear.Element, error) {
	if grinding == 0 {
		return ts.ComputeChallenge(name)
	}

	pow, ok := proofsOfWork[name]
	if !ok {
		return [8]koalabear.Element{}, fmt.Errorf("missing proof of work for %s", name)
	}
	if err := ts.SetProofOfWork(name, pow); err != nil {
		return [8]koalabear.Element{}, err
	}
	return ts.ComputeChallenge(name, fiatshamirrefactor.WithGrinding(grinding))
}

func recordFoldProofsOfWork(p Params, prf *Proof, ts *fiatshamirrefactor.Transcript) error {
	if p.grinding == 0 || p.numRounds == 0 {
		return nil
	}
	prf.PoW = make(map[string]fiatshamirrefactor.ProofOfWork, p.numRounds)
	for j := 0; j < p.numRounds; j++ {
		name := foldName(j)
		pow, ok := ts.ProofOfWork(name)
		if !ok {
			return fmt.Errorf("missing proof of work for %s", name)
		}
		prf.PoW[name] = pow
	}
	return nil
}

func log2(n int) int {
	k := 0
	for n > 1 {
		n >>= 1
		k++
	}
	return k
}

// buildTreeBase builds a Merkle tree of Nⱼ/2 leaves where
// leaf k = LeafHasher(layer[k] || layer[k + Nⱼ/2]).
func buildTreeBase(
	layer []koalabear.Element,
	lh commitment.LeafHasher,
	nh commitment.NodeHasher,
) (*merkle.Tree, error) {
	half := len(layer) / 2
	tree, err := merkle.New(half, nh)
	if err != nil {
		return nil, err
	}
	leaves := make([]hash.Digest, half)
	commitment.HashLeavesParallel(lh, leaves, commitment.LeafSource{
		Base:       []poly.Polynomial{layer},
		PairOffset: half,
	})
	return tree, tree.Build(leaves)
}

// buildTreeExt is the extension-field counterpart of buildTreeBase.
func buildTreeExt(layer []ext.E6, lh commitment.LeafHasher, nh commitment.NodeHasher) (*merkle.Tree, error) {
	half := len(layer) / 2
	tree, err := merkle.New(half, nh)
	if err != nil {
		return nil, err
	}
	leaves := make([]hash.Digest, half)
	commitment.HashLeavesParallel(lh, leaves, commitment.LeafSource{
		Ext:        []poly.ExtPolynomial{layer},
		PairOffset: half,
	})
	return tree, tree.Build(leaves)
}

// foldLayerBase folds a base-field layer of size Nⱼ into a layer of size Nⱼ/2.
//
// The naive loop carries a serial dependency on xInv = GeneratorInv^i; each
// parallel chunk seeds xInv with GeneratorInv^chunkStart so chunks run
// independently.
func foldLayerBase(
	layer []koalabear.Element,
	alpha koalabear.Element,
	domain *fft.Domain,
	invTwo koalabear.Element,
) []koalabear.Element {
	half := len(layer) / 2
	next := make([]koalabear.Element, half)
	parallel.ExecuteWithThreshold(half, foldParallelThreshold, func(start, end int) {
		xInv := poly.PowUint64(domain.GeneratorInv, uint64(start))
		var sum, diff koalabear.Element
		for i := start; i < end; i++ {
			p, q := layer[i], layer[i+half]

			sum.Add(&p, &q)
			sum.Mul(&sum, &invTwo)

			diff.Sub(&p, &q)
			diff.Mul(&diff, &invTwo)
			diff.Mul(&diff, &xInv)
			diff.Mul(&diff, &alpha)

			next[i].Add(&sum, &diff)
			xInv.Mul(&xInv, &domain.GeneratorInv)
		}
	})
	return next
}

// foldLayerExt is the extension-field counterpart of foldLayerBase; domain
// factors stay in the base field and are multiplied via MulByElement.
func foldLayerExt(layer []ext.E6, alpha ext.E6, domain *fft.Domain, invTwo koalabear.Element) []ext.E6 {
	half := len(layer) / 2
	next := make([]ext.E6, half)
	parallel.ExecuteWithThreshold(half, foldParallelThreshold, func(start, end int) {
		xInv := poly.PowUint64(domain.GeneratorInv, uint64(start))
		for i := start; i < end; i++ {
			p, q := layer[i], layer[i+half]

			var sum, diff ext.E6
			sum.Add(&p, &q)
			sum.MulByElement(&sum, &invTwo)

			diff.Sub(&p, &q)
			diff.MulByElement(&diff, &invTwo)
			diff.MulByElement(&diff, &xInv)
			diff.Mul(&diff, &alpha)

			next[i].Add(&sum, &diff)
			xInv.Mul(&xInv, &domain.GeneratorInv)
		}
	})
	return next
}

func transcriptBasePoly(poly []koalabear.Element) []koalabear.Element {
	res := make([]koalabear.Element, 0, 2+len(poly))
	res = append(res, hash.NewElement(0x42415345), hash.NewElement(uint64(len(poly)))) // "BASE"
	res = append(res, poly...)
	return res
}

func transcriptExtPoly(poly []ext.E6) []koalabear.Element {
	res := make([]koalabear.Element, 0, 2+hash.ExtDegree*len(poly))
	res = append(res, hash.NewElement(0x45585450), hash.NewElement(uint64(len(poly)))) // "EXTP"
	for _, v := range poly {
		res = hash.AppendExtElements(res, v)
	}
	return res
}

func queryIndex(challenge hash.Digest, modulus int) int {
	if modulus <= 0 {
		return 0
	}
	v := (challenge[0].Uint64() << 31) ^ challenge[1].Uint64()
	return int(v % uint64(modulus))
}

// openQueryBase builds the Merkle opening data for query index s across all r
// base folding levels.
func openQueryBase(s int, layers [][]koalabear.Element, trees []*merkle.Tree, numRounds int) (Query, error) {
	q := Query{Layers: make([]QueryLayer, numRounds)}
	for j := 0; j < numRounds; j++ {
		Nj := len(layers[j])
		base := s % (Nj / 2)

		path, err := trees[j].OpenProof(base)
		if err != nil {
			return Query{}, fmt.Errorf("layer %d OpenProof base=%d: %w", j, base, err)
		}

		q.Layers[j] = QueryLayer{
			Field:     field.KindBase,
			LeafPBase: layers[j][base],
			LeafQBase: layers[j][base+Nj/2],
			Path:      path,
		}
	}
	return q, nil
}

// openQueryExt builds the Merkle opening data for query index s across all r
// extension folding levels.
func openQueryExt(s int, layers [][]ext.E6, trees []*merkle.Tree, numRounds int) (Query, error) {
	q := Query{Layers: make([]QueryLayer, numRounds)}
	for j := 0; j < numRounds; j++ {
		Nj := len(layers[j])
		base := s % (Nj / 2)

		path, err := trees[j].OpenProof(base)
		if err != nil {
			return Query{}, fmt.Errorf("layer %d OpenProof base=%d: %w", j, base, err)
		}

		q.Layers[j] = QueryLayer{
			Field:    field.KindExt,
			LeafPExt: layers[j][base],
			LeafQExt: layers[j][base+Nj/2],
			Path:     path,
		}
	}
	return q, nil
}

func (li levelIntroductions) checkLevelQueryBase(s, lIdx, leafIdx, domainSize int) error {
	level := lIdx + 1
	if level >= len(li.introRounds) || li.introRounds[level] < 0 {
		return fmt.Errorf("level %d: missing introduction round", level)
	}
	base := s % ((domainSize >> li.introRounds[level]) / 2)
	if leafIdx != base {
		return fmt.Errorf("level %d: Merkle proof opened leaf %d, want %d", level, leafIdx, base)
	}
	return nil
}

// checkQuery verifies one base-field multi-degree FRI query:
//   - Merkle proofs for all level polynomial openings
//   - Merkle proofs and fold consistency for the running-polynomial path
//   - Batching consistency at each level introduction round
//
// levelQueriesForQuery[l-1] holds the opening for levels[l] (l 0-based index offset by 1).
// levelRoots[l-1] is the Merkle root of levels[l].Evals (l 0-based offset by 1).
// gammas[l] is the batching challenge for levels[l] (1-based; gammas[0] unused).
func checkQuery(s int, fq Query,
	levelQueriesForQuery []QueryLayer,
	levelRoots []hash.Digest,
	introductions levelIntroductions,
	gammas []koalabear.Element,
	roots []hash.Digest,
	finalPoly []koalabear.Element,
	alphas []koalabear.Element,
	p Params) error {

	// Verify Merkle proofs for all level polynomial openings.
	for lIdx, ld := range levelQueriesForQuery {
		if ld.Field != field.KindBase {
			return fmt.Errorf("level %d: expected base query layer, got %s", lIdx+1, ld.Field)
		}
		if err := introductions.checkLevelQueryBase(s, lIdx, ld.Path.LeafIdx, p.N); err != nil {
			return err
		}
		pair := []commitment.PairBase{{ld.LeafPBase, ld.LeafQBase}}
		leaf := p.LeafHasher.HashLeaf(pair, nil)
		if !merkle.Verify(levelRoots[lIdx], ld.Path, leaf, p.NodeHasher) {
			return fmt.Errorf("level %d: Merkle proof invalid", lIdx+1)
		}
	}

	// Verify running-polynomial fold path with batching consistency checks.
	for j := 0; j < p.numRounds; j++ {
		Nj := int(p.domainsLight[j].cardinality)
		base := s % (Nj / 2)
		layer := fq.Layers[j]
		if layer.Field != field.KindBase {
			return fmt.Errorf("round %d: expected base query layer, got %s", j, layer.Field)
		}
		if layer.Path.LeafIdx != base {
			return fmt.Errorf("round %d: Merkle proof opened leaf %d, want %d", j, layer.Path.LeafIdx, base)
		}

		pair := []commitment.PairBase{{layer.LeafPBase, layer.LeafQBase}}
		leaf := p.LeafHasher.HashLeaf(pair, nil)
		if !merkle.Verify(roots[j], layer.Path, leaf, p.NodeHasher) {
			return fmt.Errorf("round %d: Merkle proof invalid (base=%d)", j, base)
		}

		// Fold: expected = (LeafP+LeafQ)/2 + α*(LeafP-LeafQ)/(2·ωⱼ^base).
		var xInv, sum, diff, expected koalabear.Element
		xInv.Exp(p.domainsLight[j].generator, big.NewInt(int64(Nj-base)))
		sum.Add(&layer.LeafPBase, &layer.LeafQBase)
		sum.Mul(&sum, &p.invTwo)
		diff.Sub(&layer.LeafPBase, &layer.LeafQBase)
		diff.Mul(&diff, &p.invTwo)
		diff.Mul(&diff, &xInv)
		diff.Mul(&diff, &alphas[j])
		expected.Add(&sum, &diff)

		if j < p.numRounds-1 {
			Nj1 := Nj / 2
			nextLayer := fq.Layers[j+1]
			isLeafP := base < Nj1/2

			// expectedNext = fold output + level contributions at round j+1 (if any).
			var expectedNext koalabear.Element
			expectedNext.Set(&expected)

			if li, ok := introductions.levelAtRound[j+1]; ok {
				gamma := gammas[li]
				ld := levelQueriesForQuery[li-1]
				var leafVal koalabear.Element
				if isLeafP {
					leafVal.Set(&ld.LeafPBase)
				} else {
					leafVal.Set(&ld.LeafQBase)
				}
				var term koalabear.Element
				term.Mul(&leafVal, &gamma)
				expectedNext.Add(&expectedNext, &term)
			}

			if isLeafP {
				if !expectedNext.Equal(&nextLayer.LeafPBase) {
					return fmt.Errorf("round %d: folded value mismatch at round %d LeafP", j, j+1)
				}
			} else {
				if !expectedNext.Equal(&nextLayer.LeafQBase) {
					return fmt.Errorf("round %d: folded value mismatch at round %d LeafQ", j, j+1)
				}
			}
		} else {
			finalVal := finalPoly[s%len(finalPoly)]
			if !expected.Equal(&finalVal) {
				return fmt.Errorf("round %d (final): folded value does not match FinalPoly", j)
			}
		}
	}

	return nil
}

func checkQueryExt(s int, fq Query,
	levelQueriesForQuery []QueryLayer,
	levelRoots []hash.Digest,
	introductions levelIntroductions,
	gammas []ext.E6,
	roots []hash.Digest,
	finalPoly []ext.E6,
	alphas []ext.E6,
	p Params) error {

	for lIdx, ld := range levelQueriesForQuery {
		if ld.Field != field.KindExt {
			return fmt.Errorf("level %d: expected ext query layer, got %s", lIdx+1, ld.Field)
		}
		if err := introductions.checkLevelQueryBase(s, lIdx, ld.Path.LeafIdx, p.N); err != nil {
			return err
		}
		pair := []commitment.PairExt{{ld.LeafPExt, ld.LeafQExt}}
		leaf := p.LeafHasher.HashLeaf(nil, pair)
		if !merkle.Verify(levelRoots[lIdx], ld.Path, leaf, p.NodeHasher) {
			return fmt.Errorf("level %d: Merkle proof invalid", lIdx+1)
		}
	}

	for j := 0; j < p.numRounds; j++ {
		Nj := int(p.domainsLight[j].cardinality)
		base := s % (Nj / 2)
		layer := fq.Layers[j]
		if layer.Field != field.KindExt {
			return fmt.Errorf("round %d: expected ext query layer, got %s", j, layer.Field)
		}
		if layer.Path.LeafIdx != base {
			return fmt.Errorf("round %d: Merkle proof opened leaf %d, want %d", j, layer.Path.LeafIdx, base)
		}

		pair := []commitment.PairExt{{layer.LeafPExt, layer.LeafQExt}}
		leaf := p.LeafHasher.HashLeaf(nil, pair)
		if !merkle.Verify(roots[j], layer.Path, leaf, p.NodeHasher) {
			return fmt.Errorf("round %d: Merkle proof invalid (base=%d)", j, base)
		}

		var xInv koalabear.Element
		xInv.Exp(p.domainsLight[j].generator, big.NewInt(int64(Nj-base)))

		var sum, diff, expected ext.E6
		sum.Add(&layer.LeafPExt, &layer.LeafQExt)
		sum.MulByElement(&sum, &p.invTwo)
		diff.Sub(&layer.LeafPExt, &layer.LeafQExt)
		diff.MulByElement(&diff, &p.invTwo)
		diff.MulByElement(&diff, &xInv)
		diff.Mul(&diff, &alphas[j])
		expected.Add(&sum, &diff)

		if j < p.numRounds-1 {
			Nj1 := Nj / 2
			nextLayer := fq.Layers[j+1]
			isLeafP := base < Nj1/2

			var expectedNext ext.E6
			expectedNext.Set(&expected)

			if li, ok := introductions.levelAtRound[j+1]; ok {
				gamma := gammas[li]
				ld := levelQueriesForQuery[li-1]
				var leafVal ext.E6
				if isLeafP {
					leafVal.Set(&ld.LeafPExt)
				} else {
					leafVal.Set(&ld.LeafQExt)
				}
				var term ext.E6
				term.Mul(&leafVal, &gamma)
				expectedNext.Add(&expectedNext, &term)
			}

			if isLeafP {
				if !expectedNext.Equal(&nextLayer.LeafPExt) {
					return fmt.Errorf("round %d: folded value mismatch at round %d LeafP", j, j+1)
				}
			} else {
				if !expectedNext.Equal(&nextLayer.LeafQExt) {
					return fmt.Errorf("round %d: folded value mismatch at round %d LeafQ", j, j+1)
				}
			}
		} else {
			finalVal := finalPoly[s%len(finalPoly)]
			if !expected.Equal(&finalVal) {
				return fmt.Errorf("round %d (final): folded value does not match FinalPoly", j)
			}
		}
	}

	return nil
}
