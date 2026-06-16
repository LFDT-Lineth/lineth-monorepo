package hash

import (
	"crypto/sha256"
	stdhash "hash"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// SHA256FieldHasher implements FieldHasher by serializing field elements to
// bytes before hashing. It is intended for non-recursive workflows that prefer
// the native SHA-256 implementation over an algebraic hash.
type SHA256FieldHasher struct {
	h stdhash.Hash
}

func NewSHA256FieldHasher() *SHA256FieldHasher {
	return &SHA256FieldHasher{h: sha256.New()}
}

func (h *SHA256FieldHasher) Reset() {
	h.ensure()
	h.h.Reset()
}

func (h *SHA256FieldHasher) WriteElements(elmts ...koalabear.Element) {
	h.ensure()
	for i := range elmts {
		b := elmts[i].Bytes()
		_, _ = h.h.Write(b[:])
	}
}

func (h *SHA256FieldHasher) WriteExt(elmts ...ext.E6) {
	for _, elmt := range elmts {
		h.WriteElements(elmt.B0.A0, elmt.B0.A1, elmt.B1.A0, elmt.B1.A1, elmt.B2.A0, elmt.B2.A1)
	}
}

func (h *SHA256FieldHasher) Sum() Digest {
	h.ensure()
	sum := h.h.Sum(nil)
	var b [sha256.Size]byte
	copy(b[:], sum)
	return DigestFromBytes32(b)
}

func (h *SHA256FieldHasher) ensure() {
	if h.h == nil {
		h.h = sha256.New()
	}
}

func DigestFromBytes32(b [32]byte) Digest {
	var out Digest
	for i := range out {
		out[i].SetBytes(b[i*koalabear.Bytes : (i+1)*koalabear.Bytes])
	}
	return out
}
