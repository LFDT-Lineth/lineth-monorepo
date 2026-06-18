package commitment

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/hash"
)

const (
	HashBackendPoseidon2 = "poseidon2"
	HashBackendSHA256    = "sha256"
)

// HashBackend contains every hash primitive that must agree between setup,
// proving, and verification.
type HashBackend struct {
	ID                  string
	LeafHasher          LeafHasher
	NodeHasher          NodeHasher
	NewTranscriptHasher func() hash.FieldHasher
}

type SHA256LeafHasher struct{}

type SHA256NodeHasher struct{}

func Poseidon2HashBackend() HashBackend {
	return HashBackend{
		ID:         HashBackendPoseidon2,
		LeafHasher: DefaultLeafHasher,
		NodeHasher: DefaultNodeHasher,
		NewTranscriptHasher: func() hash.FieldHasher {
			h := hash.NewPoseidon2SpongeHasher()
			return &h
		},
	}
}

func SHA256HashBackend() HashBackend {
	return HashBackend{
		ID:                  HashBackendSHA256,
		LeafHasher:          SHA256LeafHasher{},
		NodeHasher:          SHA256NodeHasher{},
		NewTranscriptHasher: func() hash.FieldHasher { return hash.NewSHA256FieldHasher() },
	}
}

func DefaultHashBackend() HashBackend {
	return Poseidon2HashBackend()
}

func HashBackendByID(id string) (HashBackend, error) {
	switch normalizeHashBackendID(id) {
	case HashBackendPoseidon2:
		return Poseidon2HashBackend(), nil
	case HashBackendSHA256:
		return SHA256HashBackend(), nil
	default:
		return HashBackend{}, fmt.Errorf("unknown hash backend %q", id)
	}
}

// ResolveHashBackend returns the explicitly configured backend, or the backend
// identified by keyID when no explicit backend was provided. Empty IDs preserve
// compatibility with keys and proofs produced before backend metadata existed.
func ResolveHashBackend(configured HashBackend, keyID string) (HashBackend, error) {
	if isZeroHashBackend(configured) {
		return HashBackendByID(keyID)
	}
	if err := validateHashBackend(configured); err != nil {
		return HashBackend{}, err
	}
	if keyID != "" && keyID != configured.ID {
		return HashBackend{}, fmt.Errorf("hash backend mismatch: configured %q, key uses %q", configured.ID, keyID)
	}
	return configured, nil
}

func NormalizeHashBackendID(id string) string {
	return normalizeHashBackendID(id)
}

func isZeroHashBackend(backend HashBackend) bool {
	return backend.ID == "" && backend.LeafHasher == nil && backend.NodeHasher == nil && backend.NewTranscriptHasher == nil
}

func validateHashBackend(backend HashBackend) error {
	if backend.ID == "" {
		return fmt.Errorf("hash backend ID is empty")
	}
	if backend.LeafHasher == nil {
		return fmt.Errorf("hash backend %q has nil leaf hasher", backend.ID)
	}
	if backend.NodeHasher == nil {
		return fmt.Errorf("hash backend %q has nil node hasher", backend.ID)
	}
	if backend.NewTranscriptHasher == nil {
		return fmt.Errorf("hash backend %q has nil transcript hasher factory", backend.ID)
	}
	if backend.NewTranscriptHasher() == nil {
		return fmt.Errorf("hash backend %q returned nil transcript hasher", backend.ID)
	}
	return nil
}

func normalizeHashBackendID(id string) string {
	if id == "" {
		return HashBackendPoseidon2
	}
	return id
}

func (SHA256LeafHasher) HashLeaf(base []PairBase, ext []PairExt) hash.Digest {
	h := hash.NewSHA256FieldHasher()
	h.WriteElements(hash.NewElement(leafDomainTag), hash.NewElement(uint64(len(base))), hash.NewElement(uint64(len(ext))))
	for _, pair := range base {
		h.WriteElements(pair[0], pair[1])
	}
	for _, pair := range ext {
		h.WriteExt(pair[0], pair[1])
	}
	return h.Sum()
}

func (lh SHA256LeafHasher) HashLeaves(dst []hash.Digest, src LeafSource, start int) {
	hashLeavesScalar(lh, dst, src, start)
}

func (SHA256LeafHasher) BatchSize() int {
	return 1
}

func (SHA256NodeHasher) HashNode(left, right hash.Digest) hash.Digest {
	h := hash.NewSHA256FieldHasher()
	h.WriteElements(hash.NewElement(nodeDomainTag))
	h.WriteElements(left[:]...)
	h.WriteElements(right[:]...)
	return h.Sum()
}
