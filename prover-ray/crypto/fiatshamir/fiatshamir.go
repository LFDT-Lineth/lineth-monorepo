// Package fiatshamir implements a field-agnostic Fiat-Shamir transcript over a
// Merkle-Damgard hasher. Field-specific packages instantiate it, see
// crypto/koalabear/fiatshamir.
package fiatshamir

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/hash"
)

// Element is the constraint on the base field element. It lists the methods
// the transcript needs beyond the hashing itself.
type Element interface {
	// Uint64 returns the canonical (non-Montgomery) value of the element.
	Uint64() uint64
}

// Extension is the constraint on the extension field element E over the base
// field element F.
type Extension[F any] interface {
	// AppendBase appends the base field coordinates of the element to s and
	// returns the extended slice.
	AppendBase(s []F) []F
}

// SetBaser is the constraint on the pointer to the extension field element E,
// for building an element from its coordinates.
type SetBaser[F, E any] interface {
	*E
	// SetBase sets the base field coordinates of the element to s.
	SetBase(s []F) *E
}

// GenElement is a field element whose kind, base or extension, is known only
// at runtime. E is the extension field element.
type GenElement[F, E any] interface {
	// IsBase reports whether the element is in the base field.
	IsBase() bool
	// AsBase returns the element as a base field element. It is only called
	// when IsBase is true.
	AsBase() F
	// AsExt returns the element as an extension field element.
	AsExt() E
}

// Transcript accumulates the transcript of a protocol (e.g. the set of
// messages sent between the prover and the verifier). The accumulated
// transcripts can then be used to sample random coins that are obtained by
// hashing the provided transcript. The hashing is incremental and can be
// updated at any moment.
//
// F is the base field element, E the extension field element and D the
// digest. The extension degree must not exceed the digest length, so that a
// coin fits in a single digest.
type Transcript[F Element, E Extension[F], D hash.ArrayBlock[F]] struct {
	h *hash.MerkleDamgardArray[F, D]
	// degree is the number of base field coordinates of E.
	degree int
	// buf holds the coordinates of a batch of extension elements.
	buf []F
	// ext keeps the receiver of SetBase off the stack: calls through the
	// generic dictionary would make a local escape.
	ext E
}

// extBatch is the number of extension elements whose coordinates are written
// to the hasher at once.
const extBatch = 32

// NewTranscript creates a new transcript accumulator hashing with
// Merkle-Damgard over the compression function c, from the zero state.
func NewTranscript[F Element, E Extension[F], D hash.ArrayBlock[F]](c hash.FieldCompressor[D]) *Transcript[F, E, D] {
	var (
		iv D
		e  E
	)
	// The degree is at most len(D), so buf never grows.
	buf := make([]F, 0, extBatch*len(iv))
	degree := len(e.AppendBase(buf))
	if degree > len(iv) {
		panic(fmt.Sprintf("fiatshamir: extension degree %d exceeds digest length %d", degree, len(iv)))
	}
	return &Transcript[F, E, D]{
		h:      hash.NewMerkleDamgardArray[F](c, iv),
		degree: degree,
		buf:    buf,
	}
}

// Update adds base field elements to the transcript.
func (fs *Transcript[F, E, D]) Update(vec ...F) {
	fs.h.WriteElements(vec...)
}

// UpdateExt adds extension field elements to the transcript, as their base
// field coordinates.
func (fs *Transcript[F, E, D]) UpdateExt(vec ...E) {
	for len(vec) > 0 {
		n := min(len(vec), extBatch)
		buf := fs.buf[:0]
		for i := range n {
			buf = vec[i].AppendBase(buf)
		}
		fs.h.WriteElements(buf...)
		vec = vec[n:]
	}
}

// UpdateGeneric adds elements of runtime-determined kind to the transcript:
// base field elements as one coordinate, extension field elements as all of
// theirs.
func (fs *Transcript[F, E, D]) UpdateGeneric[G GenElement[F, E]](vec ...G) {
	for _, g := range vec {
		if g.IsBase() {
			fs.h.WriteElement(g.AsBase())
		} else {
			fs.UpdateExt(g.AsExt())
		}
	}
}

// UpdateVec adds slices of base field elements to the transcript.
func (fs *Transcript[F, E, D]) UpdateVec(vecs ...[]F) {
	for i := range vecs {
		fs.Update(vecs[i]...)
	}
}

// UpdateVecExt updates the Fiat-Shamir state by passing one or more slices of
// extension field elements.
func (fs *Transcript[F, E, D]) UpdateVecExt(vecs ...[]E) {
	for i := range vecs {
		fs.UpdateExt(vecs[i]...)
	}
}

// RandomDigest samples a random digest from the current transcript state.
func (fs *Transcript[F, E, D]) RandomDigest() D {
	fs.h.PadBlock()
	d := fs.h.SumDigest()
	fs.safeguardUpdate()
	return d
}

// RandomFext samples a random extension field element from the transcript.
// It uses the first coordinates of a digest; the remaining ones are discarded.
// PE is inferred from E.
func (fs *Transcript[F, E, D]) RandomFext[PE SetBaser[F, E]]() E {
	s := fs.RandomDigest() // already calls safeguardUpdate()
	// D is a union of array types, which cannot be sliced.
	coords := fs.buf[:fs.degree]
	for i := range coords {
		coords[i] = s[i]
	}
	PE(&fs.ext).SetBase(coords)
	return fs.ext
}

// RandomManyIntegers samples num integers uniformly in [0, upperBound). The function expects
// upperBound to be a power of two.
func (fs *Transcript[F, E, D]) RandomManyIntegers(num, upperBound int) []int {

	if !utils.IsPowerOfTwo(upperBound) {
		panic("upperBound must be a power of two")
	}

	var (
		i   = 0
		res = make([]int, num)
	)

	for i < num {
		// take the remainder mod n of each limb
		c := fs.RandomDigest() // already calls safeguardUpdate()
		for j := 0; j < len(c); j++ {
			res[i] = int(c[j].Uint64() % uint64(upperBound))
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

// SetState replaces the internal hasher state with s and discards pending
// input.
func (fs *Transcript[F, E, D]) SetState(s D) {
	fs.h.SetStateDigest(s)
}

// State returns the current internal hasher state, with pending input
// committed as by RandomDigest, without modifying the transcript.
func (fs *Transcript[F, E, D]) State() D {
	return fs.h.SumDigest()
}

func (fs *Transcript[F, E, D]) safeguardUpdate() {
	var zero F
	fs.h.WriteElement(zero)
}
