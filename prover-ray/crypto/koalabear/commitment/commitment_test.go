package commitment

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/merkle"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/poly"
)

func TestRSCommitDualRailProof(t *testing.T) {
	basePolys := []poly.Polynomial{
		{baseElement(1), baseElement(2), baseElement(3), baseElement(4)},
		{baseElement(5), baseElement(6), baseElement(7), baseElement(8)},
	}
	extPolys := []poly.ExtPolynomial{
		{
			extElement(1, 2, 3, 4),
			extElement(5, 6, 7, 8),
			extElement(9, 10, 11, 12),
			extElement(13, 14, 15, 16),
		},
	}

	committer := NewRSCommit(4, 2, DefaultLeafHasher, DefaultNodeHasher)
	tree, err := committer.Commit(basePolys, extPolys)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.NumLeaves(); got != 4 {
		t.Fatalf("NumLeaves = %d, want 4", got)
	}
	if got := tree.BaseWidth(); got != len(basePolys) {
		t.Fatalf("base rail width = %d, want %d", got, len(basePolys))
	}
	if got := tree.ExtWidth(); got != len(extPolys) {
		t.Fatalf("ext rail width = %d, want %d", got, len(extPolys))
	}

	const leafIdx = 1
	proof, err := tree.OpenProof(leafIdx)
	if err != nil {
		t.Fatal(err)
	}
	baseLeaf, extLeaf := rawLeafFromPolys(committer, basePolys, extPolys, leafIdx)
	leaf := DefaultLeafHasher.HashLeaf(baseLeaf, extLeaf)
	if !merkle.Verify(tree.Root(), proof, leaf, DefaultNodeHasher) {
		t.Fatal("dual-rail Merkle proof did not verify")
	}
}

func TestWMerkleTreeOpenProof(t *testing.T) {
	basePolys := []poly.Polynomial{
		{baseElement(1), baseElement(2), baseElement(3), baseElement(4)},
		{baseElement(5), baseElement(6), baseElement(7), baseElement(8)},
	}
	extPolys := []poly.ExtPolynomial{
		{
			extElement(1, 2, 3, 4),
			extElement(5, 6, 7, 8),
			extElement(9, 10, 11, 12),
			extElement(13, 14, 15, 16),
		},
	}

	committer := NewRSCommit(4, 2, DefaultLeafHasher, DefaultNodeHasher)
	tree, err := committer.Commit(basePolys, extPolys)
	if err != nil {
		t.Fatal(err)
	}

	const leafIdx = 2
	proof, err := tree.OpenProof(leafIdx)
	if err != nil {
		t.Fatal(err)
	}

	baseLeaf, extLeaf := rawLeafFromPolys(committer, basePolys, extPolys, leafIdx)
	leaf := DefaultLeafHasher.HashLeaf(baseLeaf, extLeaf)
	if !merkle.Verify(tree.Root(), proof, leaf, DefaultNodeHasher) {
		t.Fatal("opened Merkle proof did not verify")
	}
}

func TestRSCommitWithDomainCache(t *testing.T) {
	basePolys := []poly.Polynomial{
		{baseElement(1), baseElement(2), baseElement(3), baseElement(4)},
		{baseElement(5), baseElement(6), baseElement(7), baseElement(8)},
	}

	var cache poly.DomainCache
	committer := NewRSCommitWithDomainCache(4, 2, DefaultLeafHasher, DefaultNodeHasher, &cache)
	tree, err := committer.Commit(basePolys, nil, WithDomainCache(&cache))
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.NumLeaves(); got != 4 {
		t.Fatalf("NumLeaves = %d, want 4", got)
	}
	if got := cache.Get(4); got != cache.Get(4) {
		t.Fatalf("DomainCache did not reuse input domain: %p vs %p", got, cache.Get(4))
	}
}

func TestRSCommitBatchLeafHasherMatchesScalarRoot(t *testing.T) {
	basePolys := []poly.Polynomial{
		{baseElement(1), baseElement(2), baseElement(3), baseElement(4)},
		{baseElement(5), baseElement(6), baseElement(7), baseElement(8)},
	}
	extPolys := []poly.ExtPolynomial{
		{
			extElement(1, 2, 3, 4),
			extElement(5, 6, 7, 8),
			extElement(9, 10, 11, 12),
			extElement(13, 14, 15, 16),
		},
	}

	tests := []struct {
		name string
		lh   LeafHasher
		nh   NodeHasher
	}{
		{
			name: "poseidon2",
			lh:   DefaultLeafHasher,
			nh:   DefaultNodeHasher,
		},
		{
			name: "sha256",
			lh:   SHA256LeafHasher{},
			nh:   SHA256NodeHasher{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchCommitter := NewRSCommit(4, 2, tt.lh, tt.nh)
			batchTree, err := batchCommitter.Commit(basePolys, extPolys)
			if err != nil {
				t.Fatal(err)
			}

			scalarCommitter := NewRSCommit(4, 2, scalarOnlyLeafHasher{inner: tt.lh}, tt.nh)
			scalarTree, err := scalarCommitter.Commit(basePolys, extPolys)
			if err != nil {
				t.Fatal(err)
			}

			if batchTree.Root() != scalarTree.Root() {
				t.Fatalf("batched root differs from scalar root: got %v, want %v", batchTree.Root(), scalarTree.Root())
			}
		})
	}
}

func TestPoseidon2BatchLeafHasherMatchesScalarLeaves(t *testing.T) {
	tests := []struct {
		name    string
		leaves  int
		nbBase  int
		nbExt   int
		offset  int
		wantEnd int
	}{
		{name: "small mixed fallback", leaves: 8, nbBase: 2, nbExt: 1},
		{name: "exact base only", leaves: hash.Poseidon2SpongeBatchSize, nbBase: 3},
		{name: "tail ext only", leaves: hash.Poseidon2SpongeBatchSize + 1, nbExt: 2},
		{name: "multiple batches mixed", leaves: 2*hash.Poseidon2SpongeBatchSize + 1, nbBase: 4, nbExt: 2},
		{name: "subrange", leaves: 2 * hash.Poseidon2SpongeBatchSize, nbBase: 2, nbExt: 2, offset: 3, wantEnd: hash.Poseidon2SpongeBatchSize + 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := testLeafSource(tt.leaves, tt.nbBase, tt.nbExt)
			start := tt.offset
			end := tt.wantEnd
			if end == 0 {
				end = tt.leaves
			}
			got := make([]hash.Digest, end-start)
			DefaultLeafHasher.HashLeaves(got, src, start)

			for k := range got {
				i := start + k
				baseLeaf, extLeaf := leafFromSource(src, i)
				if want := DefaultLeafHasher.HashLeaf(baseLeaf, extLeaf); got[k] != want {
					t.Fatalf("leaf %d: batched digest differs from scalar digest", i)
				}
			}
		})
	}
}

func TestPoseidon2BatchNodeHasherMatchesScalarHash(t *testing.T) {
	const n = hash.Poseidon2SpongeBatchSize
	left := make([]hash.Digest, n)
	right := make([]hash.Digest, n)
	for i := 0; i < n; i++ {
		for j := 0; j < len(left[i]); j++ {
			left[i][j].SetUint64(uint64(0xabcd0000 + i*16 + j))
			right[i][j].SetUint64(uint64(0xdcba0000 + i*16 + j))
		}
	}

	got := make([]hash.Digest, n)
	DefaultNodeHasher.HashNodes(got, left, right)

	for i := 0; i < n; i++ {
		want := DefaultNodeHasher.HashNode(left[i], right[i])
		if got[i] != want {
			t.Fatalf("pair %d: batched node digest differs from scalar digest:\n got  %v\n want %v", i, got[i], want)
		}
	}
}

func TestRSCommitEmptyRails(t *testing.T) {
	basePolys := []poly.Polynomial{
		{baseElement(1), baseElement(2), baseElement(3), baseElement(4)},
	}
	committer := NewRSCommit(4, 2, DefaultLeafHasher, DefaultNodeHasher)
	baseTree, err := committer.Commit(basePolys, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := baseTree.ExtWidth(); got != 0 {
		t.Fatalf("base-only tree ext rail width = %d, want 0", got)
	}

	extPolys := []poly.ExtPolynomial{
		{
			extElement(1, 2, 3, 4),
			extElement(5, 6, 7, 8),
			extElement(9, 10, 11, 12),
			extElement(13, 14, 15, 16),
		},
	}
	extTree, err := committer.Commit(nil, extPolys)
	if err != nil {
		t.Fatal(err)
	}
	if got := extTree.BaseWidth(); got != 0 {
		t.Fatalf("ext-only tree base rail width = %d, want 0", got)
	}
	if got := extTree.NumLeaves(); got != 4 {
		t.Fatalf("ext-only NumLeaves = %d, want 4", got)
	}
}

type scalarOnlyLeafHasher struct {
	inner LeafHasher
}

func (h scalarOnlyLeafHasher) HashLeaf(base []PairBase, ext []PairExt) hash.Digest {
	return h.inner.HashLeaf(base, ext)
}

func testLeafSource(nLeaves, nbBase, nbExt int) LeafSource {
	src := LeafSource{
		Base:       make([]poly.Polynomial, nbBase),
		Ext:        make([]poly.ExtPolynomial, nbExt),
		PairOffset: nLeaves,
	}
	for j := range src.Base {
		src.Base[j] = make(poly.Polynomial, 2*nLeaves)
		for i := range src.Base[j] {
			src.Base[j][i] = baseElement(uint64(1000*(j+1) + i + 1))
		}
	}
	for j := range src.Ext {
		src.Ext[j] = make(poly.ExtPolynomial, 2*nLeaves)
		for i := range src.Ext[j] {
			v := uint64(10000*(j+1) + 10*(i+1))
			src.Ext[j][i] = extElement(v+1, v+2, v+3, v+4)
		}
	}
	return src
}

func leafFromSource(src LeafSource, i int) ([]PairBase, []PairExt) {
	baseLeaf := make([]PairBase, len(src.Base))
	for j := range src.Base {
		baseLeaf[j][0].Set(&src.Base[j][i])
		baseLeaf[j][1].Set(&src.Base[j][i+src.PairOffset])
	}

	extLeaf := make([]PairExt, len(src.Ext))
	for j := range src.Ext {
		extLeaf[j][0].Set(&src.Ext[j][i])
		extLeaf[j][1].Set(&src.Ext[j][i+src.PairOffset])
	}

	return baseLeaf, extLeaf
}

func baseElement(v uint64) koalabear.Element {
	var e koalabear.Element
	e.SetUint64(v)
	return e
}

func extElement(a0, a1, b0, b1 uint64, b2 ...uint64) ext.E6 {
	var e ext.E6
	e.B0.A0.SetUint64(a0)
	e.B0.A1.SetUint64(a1)
	e.B1.A0.SetUint64(b0)
	e.B1.A1.SetUint64(b1)
	if len(b2) > 0 {
		e.B2.A0.SetUint64(b2[0])
	}
	if len(b2) > 1 {
		e.B2.A1.SetUint64(b2[1])
	}
	return e
}

func rawLeafFromPolys(committer RSCommit, basePolys []poly.Polynomial, extPolys []poly.ExtPolynomial, leafIdx int) ([]PairBase, []PairExt) {
	var cache poly.DomainCache
	halfN := int(committer.Encoder.Domain.Cardinality >> 1)

	baseLeaf := make([]PairBase, len(basePolys))
	for j, p := range basePolys {
		encoded := committer.Encoder.Encode(p, cache.Get(uint64(len(p))))
		baseLeaf[j][0].Set(&encoded[leafIdx])
		baseLeaf[j][1].Set(&encoded[leafIdx+halfN])
	}

	extLeaf := make([]PairExt, len(extPolys))
	for j, p := range extPolys {
		encoded := committer.Encoder.EncodeExt(p, cache.Get(uint64(len(p))))
		extLeaf[j][0].Set(&encoded[leafIdx])
		extLeaf[j][1].Set(&encoded[leafIdx+halfN])
	}

	return baseLeaf, extLeaf
}
