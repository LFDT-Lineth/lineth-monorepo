package commitment

import (
	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/merkle"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/parallel"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poly"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/reedsolomon"
)

const (
	leafDomainTag uint64 = 0x4c454146 // "LEAF"
	nodeDomainTag uint64 = 0x4e4f4445 // "NODE"
)

type LeafHash = hash.Digest
type NodeHash = hash.Digest

type LeafHasher interface {
	HashLeaf(base []PairBase, ext []PairExt) hash.Digest
}

// LeafSource describes the column-oriented data used to build paired Merkle
// leaves. Leaf i absorbs values at i and i+PairOffset for every base and
// extension polynomial.
type LeafSource struct {
	Base       []poly.Polynomial
	Ext        []poly.ExtPolynomial
	PairOffset int
}

// BatchLeafHasher hashes a consecutive range of leaves into dst. HashLeaf
// remains the compatibility path and the source of truth for single-leaf
// verifier checks.
type BatchLeafHasher interface {
	LeafHasher
	BatchSize() int
	HashLeaves(dst []hash.Digest, src LeafSource, start int)
}

type NodeHasher interface {
	HashNode(left, right hash.Digest) hash.Digest
}

type Poseidon2LeafHasher struct{}

type Poseidon2NodeHasher struct{}

var (
	DefaultLeafHasher Poseidon2LeafHasher
	DefaultNodeHasher Poseidon2NodeHasher
)

type RSCommit struct {
	Encoder    reedsolomon.Encoder
	LeafHasher LeafHasher
	NodeHasher NodeHasher
}

type WMerkleTree struct {
	Tree *merkle.Tree

	numLeaves int
	baseWidth int
	extWidth  int
}

// PointSampling contains the pair evaluation {f(w^i),f(-w^i)} for batches of
// base and extension polynomials at a given point w^i, where i is
// Proof.LeafIdx.
type WMerkleProof struct {
	RawLeafBase []PairBase
	RawLeafExt  []PairExt
	Proof       merkle.Proof
}

func (wt WMerkleTree) Root() hash.Digest {
	return wt.Tree.Root()
}

func (wt WMerkleTree) NumLeaves() int {
	return wt.numLeaves
}

// BaseWidth returns the number of base-field pairs stored in each leaf.
func (wt WMerkleTree) BaseWidth() int {
	return wt.baseWidth
}

// ExtWidth returns the number of extension-field pairs stored in each leaf.
func (wt WMerkleTree) ExtWidth() int {
	return wt.extWidth
}

// OpenProof returns the Merkle proof for leaf i. Raw leaf values are
// reconstructed by the prover from the committed polynomials when needed.
func (wt WMerkleTree) OpenProof(i int) (merkle.Proof, error) {
	return wt.Tree.OpenProof(i)
}

type PairBase = [2]koalabear.Element // used to store the pairs {f_k(w^i), f_k(-w^i)}
type PairExt = [2]ext.E6             // used to store the pairs {f_k(w^i), f_k(-w^i)}

func NewRSCommit(N uint64, rate uint64, leafHasher LeafHasher, nodehasher NodeHasher) RSCommit {
	return NewRSCommitWithDomainCache(N, rate, leafHasher, nodehasher, nil)
}

// NewRSCommitWithDomainCache constructs an RSCommit using cache for the
// Reed-Solomon encoder domain.
func NewRSCommitWithDomainCache(
	N uint64,
	rate uint64,
	leafHasher LeafHasher,
	nodehasher NodeHasher,
	cache *poly.DomainCache,
) RSCommit {
	rsEncoder := reedsolomon.NewEncoderWithDomainCache(rate*N, cache)
	return RSCommit{
		Encoder:    rsEncoder,
		LeafHasher: leafHasher,
		NodeHasher: nodehasher,
	}
}

// CommitConfig configures RSCommit.Commit.
type CommitConfig struct {
	DomainCache *poly.DomainCache
}

// CommitOption configures RSCommit.Commit.
type CommitOption func(c *CommitConfig) error

// WithDomainCache reuses cache for input-polynomial FFT domains.
func WithDomainCache(cache *poly.DomainCache) CommitOption {
	return func(c *CommitConfig) error {
		c.DomainCache = cache
		return nil
	}
}

func (Poseidon2LeafHasher) HashLeaf(base []PairBase, ext []PairExt) hash.Digest {
	h := hash.NewPoseidon2SpongeHasher()
	h.WriteElements(hash.NewElement(leafDomainTag), hash.NewElement(uint64(len(base))), hash.NewElement(uint64(len(ext))))
	for _, pair := range base {
		h.WriteElements(pair[0], pair[1])
	}
	for _, pair := range ext {
		h.WriteExt(pair[0], pair[1])
	}
	return h.Sum()
}

func (lh Poseidon2LeafHasher) HashLeaves(dst []hash.Digest, src LeafSource, start int) {
	if src.PairOffset < hash.Poseidon2SpongeBatchSize || len(dst) < hash.Poseidon2SpongeBatchSize {
		hashLeavesScalar(lh, dst, src, start)
		return
	}

	fullBatches := len(dst) / hash.Poseidon2SpongeBatchSize
	for batch := 0; batch < fullBatches; batch++ {
		offset := batch * hash.Poseidon2SpongeBatchSize
		lh.hashLeavesBatch16(dst[offset:offset+hash.Poseidon2SpongeBatchSize], src, start+offset)
	}
	if tail := fullBatches * hash.Poseidon2SpongeBatchSize; tail < len(dst) {
		hashLeavesScalar(lh, dst[tail:], src, start+tail)
	}
}

func (Poseidon2LeafHasher) BatchSize() int {
	return hash.Poseidon2SpongeBatchSize
}

func (Poseidon2NodeHasher) HashNode(left, right hash.Digest) hash.Digest {
	return hash.Poseidon2NodeCompress(nodeDomainTag, left, right)
}

// BatchSize is the lane width of the SIMD-batched Poseidon2 permutation.
func (Poseidon2NodeHasher) BatchSize() int { return hash.Poseidon2SpongeBatchSize }

// HashNodes compresses BatchSize() (left, right) pairs in one batched
// permutation. dst, left, right must all have length BatchSize().
func (Poseidon2NodeHasher) HashNodes(dst, left, right []hash.Digest) {
	const n = hash.Poseidon2SpongeBatchSize
	if len(dst) != n || len(left) != n || len(right) != n {
		panic("Poseidon2NodeHasher.HashNodes: input slices must have length BatchSize()")
	}
	var l, r [n]hash.Digest
	copy(l[:], left)
	copy(r[:], right)
	out := hash.Poseidon2NodeCompressBatch16(nodeDomainTag, &l, &r)
	copy(dst, out[:])
}

// Commit commits to base and extension polynomials in one Merkle tree. Inputs
// are assumed to be in Lagrange form and may have different sizes. Each leaf
// hash absorbs all base pairs followed by all extension pairs.
func (rs *RSCommit) Commit(
	basePolys []poly.Polynomial,
	extPolys []poly.ExtPolynomial,
	opts ...CommitOption,
) (WMerkleTree, error) {
	var config CommitConfig
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return WMerkleTree{}, err
		}
	}
	domainCache := config.DomainCache
	if domainCache == nil {
		domainCache = &poly.DomainCache{}
	}

	// 1- encode every polynomial on its rail. Each Encode is independent
	//    (disjoint input/output slices, shared read-only domain). Each outer
	//    worker calls 2 FFTs in Encode; cap the FFT's internal parallelism so
	//    outer × inner ≤ NumCPU.
	fftOuter := max(len(basePolys), len(extPolys))
	fftOpt := fft.WithNbTasks(parallel.NbTasksPerJob(fftOuter))

	encodedBase := make([]poly.Polynomial, len(basePolys))
	parallel.Execute(len(basePolys), func(start, end int) {
		for i := start; i < end; i++ {
			pol := basePolys[i]
			encodedBase[i] = rs.Encoder.Encode(pol, domainCache.Get(uint64(len(pol))), fftOpt)
		}
	})

	encodedExt := make([]poly.ExtPolynomial, len(extPolys))
	parallel.Execute(len(extPolys), func(start, end int) {
		for i := start; i < end; i++ {
			pol := extPolys[i]
			encodedExt[i] = rs.Encoder.EncodeExt(pol, domainCache.Get(uint64(len(pol))), fftOpt)
		}
	})

	// 2- build the merkle tree, with rs.N/2 leafs
	// the i-th leaf is base pairs followed by extension pairs.
	N := rs.Encoder.Domain.Cardinality
	halfN := int(N >> 1)
	tree, err := merkle.New(halfN, rs.NodeHasher)
	if err != nil {
		return WMerkleTree{}, err
	}
	wTree := WMerkleTree{
		Tree:      tree,
		numLeaves: halfN,
		baseWidth: len(encodedBase),
		extWidth:  len(encodedExt),
	}
	leaves := make([]hash.Digest, halfN)
	src := LeafSource{
		Base:       encodedBase,
		Ext:        encodedExt,
		PairOffset: halfN,
	}
	HashLeavesParallel(rs.LeafHasher, leaves, src)

	if err := tree.Build(leaves); err != nil {
		return WMerkleTree{}, err
	}

	return wTree, nil
}

// HashLeavesParallel hashes len(dst) paired leaves from src into dst, using
// the batched leaf hasher when available (rate-16 Poseidon2 sponge) and
// fanning the work out across goroutines.
func HashLeavesParallel(lh LeafHasher, dst []hash.Digest, src LeafSource) {
	if batchHasher, ok := lh.(BatchLeafHasher); ok {
		hashLeavesBatchParallel(batchHasher, dst, src)
		return
	}
	parallel.Execute(len(dst), func(start, end int) {
		hashLeavesScalar(lh, dst[start:end], src, start)
	})
}

func hashLeavesBatchParallel(lh BatchLeafHasher, dst []hash.Digest, src LeafSource) {
	batchSize := lh.BatchSize()
	if batchSize <= 0 {
		batchSize = 1
	}

	if batchSize == 1 || len(dst) < batchSize {
		parallel.Execute(len(dst), func(start, end int) {
			lh.HashLeaves(dst[start:end], src, start)
		})
		return
	}

	full := (len(dst) / batchSize) * batchSize
	parallel.Execute(full/batchSize, func(startBatch, endBatch int) {
		start := startBatch * batchSize
		end := endBatch * batchSize
		lh.HashLeaves(dst[start:end], src, start)
	})
	if full < len(dst) {
		lh.HashLeaves(dst[full:], src, full)
	}
}

func hashLeavesScalar(lh LeafHasher, dst []hash.Digest, src LeafSource, start int) {
	baseLeaf := make([]PairBase, len(src.Base))
	extLeaf := make([]PairExt, len(src.Ext))
	for k := range dst {
		i := start + k
		if len(src.Base) > 0 {
			for j := range src.Base {
				baseLeaf[j][0].Set(&src.Base[j][i])
				baseLeaf[j][1].Set(&src.Base[j][i+src.PairOffset])
			}
		}
		if len(src.Ext) > 0 {
			for j := range src.Ext {
				extLeaf[j][0].Set(&src.Ext[j][i])
				extLeaf[j][1].Set(&src.Ext[j][i+src.PairOffset])
			}
		}
		dst[k] = lh.HashLeaf(baseLeaf, extLeaf)
	}
}

func (lh Poseidon2LeafHasher) hashLeavesBatch16(dst []hash.Digest, src LeafSource, start int) {
	sponge := hash.NewPoseidon2SpongeBatch16()
	sponge.WriteSameElement(hash.NewElement(leafDomainTag))
	sponge.WriteSameElement(hash.NewElement(uint64(len(src.Base))))
	sponge.WriteSameElement(hash.NewElement(uint64(len(src.Ext))))

	for _, pol := range src.Base {
		var lo, hi [hash.Poseidon2SpongeBatchSize]koalabear.Element
		for lane := 0; lane < hash.Poseidon2SpongeBatchSize; lane++ {
			i := start + lane
			lo[lane].Set(&pol[i])
			hi[lane].Set(&pol[i+src.PairOffset])
		}
		sponge.WriteElementBatch(lo)
		sponge.WriteElementBatch(hi)
	}

	for _, pol := range src.Ext {
		var lo, hi [hash.Poseidon2SpongeBatchSize]ext.E6
		for lane := 0; lane < hash.Poseidon2SpongeBatchSize; lane++ {
			i := start + lane
			lo[lane].Set(&pol[i])
			hi[lane].Set(&pol[i+src.PairOffset])
		}
		sponge.WriteExtBatch(lo)
		sponge.WriteExtBatch(hi)
	}

	digests := sponge.Sum()
	copy(dst, digests[:])
}
