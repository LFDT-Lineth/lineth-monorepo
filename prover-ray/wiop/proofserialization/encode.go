package proofserialization

import (
	"encoding/binary"
	"fmt"
)

// Encode lays input out as the byte image verifier-ray casts directly to a
// *const VerifyInput, relocated for base.
//
// Pointers are written as absolute addresses (base + offset in image), so the
// guest dereferences them with no arithmetic and no fix-up pass — that is what
// makes decoding free rather than merely cheap. The cost is that the image is
// only valid at this base.
//
// The layout is depth-first: each slice's payload is appended behind the
// structure referencing it, which keeps a structure and the data it points at
// adjacent for the guest's cache. Encoding cost is not a design constraint here;
// decoding cost is, and it is zero.
func Encode(input VerifyInput, base uint64) ([]byte, error) {
	if base%8 != 0 {
		return nil, fmt.Errorf("proofserialization: base 0x%x is not 8-byte aligned; "+
			"every type in the image has alignment 8 or less, and the root cast requires it", base)
	}
	// At base 0 a pointer to image offset 0 -- the root, always a valid target --
	// has the value 0, which Zig reads as null. The format's requirement that
	// every slice pointer be non-null then cannot hold, so base 0 is genuinely
	// incompatible rather than merely awkward.
	if base == 0 {
		return nil, fmt.Errorf("proofserialization: base 0 is not usable: an in-image pointer " +
			"would be indistinguishable from null, which Zig's non-optional slice pointer " +
			"forbids. Use GuestBase, or any non-zero 8-byte-aligned address")
	}

	e := &encoder{base: base}

	// The root must occupy [0, SizeProof): the loaders cast the base address
	// itself, so nothing may precede it.
	//
	// The guard below is unreachable as written — alloc on an empty buffer needs
	// no padding and returns 0 — so no test can exercise it. It is kept as an
	// invariant assertion against a future change to alloc that prefixed
	// anything, which would silently move the root. The observable property, that
	// the root's fields land at their documented offsets, is covered by
	// TestEncode_RootAtOffsetZero.
	root := e.alloc(SizeVerifyInput, 8)
	if root != 0 {
		return nil, fmt.Errorf("proofserialization: root landed at %d, must be 0", root)
	}
	e.putProof(root+OffVerifyInputProof, input.Proof)

	pis := e.putSlice(root+OffVerifyInputPublicInputs, len(input.PublicInputs), SizeScalar, SizeElement)
	for i, pi := range input.PublicInputs {
		e.putScalar(pis+i*SizeScalar, pi)
	}

	if err := checkImageSize(len(e.buf)); err != nil {
		return nil, err
	}
	return e.buf, nil
}

// checkImageSize rejects an image too large for the guest's input region.
//
// Split out so it can be tested without allocating a gigabyte: exercising it
// through [Encode] would mean actually building an oversized image.
func checkImageSize(n int) error {
	if n > MaxImageSize {
		return fmt.Errorf("proofserialization: image is %d bytes, exceeds the guest "+
			"input region's %d (LENGTH(IN) in the guest linker script)", n, MaxImageSize)
	}
	return nil
}

// encoder is a bump allocator over the image being built.
type encoder struct {
	buf  []byte
	base uint64
}

// alloc reserves size bytes at the next align-aligned offset and returns it.
//
// Padding is zero because the buffer is only ever grown with zero bytes: Zig
// leaves padding undefined, so anything non-zero here would make the image
// non-deterministic and unhashable for no benefit.
func (e *encoder) alloc(size, align int) int {
	if pad := len(e.buf) % align; pad != 0 {
		e.buf = append(e.buf, make([]byte, align-pad)...)
	}
	off := len(e.buf)
	e.buf = append(e.buf, make([]byte, size)...)
	return off
}

// putSlice reserves count elements and writes the slice header at hdr, returning
// the payload offset.
//
// An empty slice gets ptr = base rather than 0: Zig's []const T holds a
// non-optional [*]const T, so null is undefined behaviour even at length zero.
// Zig's own empty-slice literals use a small aligned sentinel (0x4); base is
// used instead because it is equally valid — non-null, aligned, and never
// dereferenced at length 0 — while staying inside the image so [Validate] can
// bounds-check every pointer uniformly.
func (e *encoder) putSlice(hdr, count, elemSize, elemAlign int) int {
	payload := 0
	ptr := e.base
	if count > 0 {
		payload = e.alloc(count*elemSize, elemAlign)
		ptr = e.base + uint64(payload)
	}
	binary.LittleEndian.PutUint64(e.buf[hdr:], ptr)
	binary.LittleEndian.PutUint64(e.buf[hdr+8:], uint64(count))
	return payload
}

func (e *encoder) putElement(off int, v Element) {
	binary.LittleEndian.PutUint32(e.buf[off:], uint32(v))
}

func (e *encoder) putExt(off int, v Ext) {
	for i, x := range v {
		e.putElement(off+i*SizeElement, x)
	}
}

func (e *encoder) putDigest(off int, v Digest) {
	for i, x := range v {
		e.putElement(off+i*SizeElement, x)
	}
}

func (e *encoder) putUsize(off int, v uint64) {
	binary.LittleEndian.PutUint64(e.buf[off:], v)
}

func (e *encoder) putElements(hdr int, xs []Element) {
	p := e.putSlice(hdr, len(xs), SizeElement, SizeElement)
	for i, x := range xs {
		e.putElement(p+i*SizeElement, x)
	}
}

func (e *encoder) putExts(hdr int, xs []Ext) {
	p := e.putSlice(hdr, len(xs), SizeExt, SizeElement)
	for i, x := range xs {
		e.putExt(p+i*SizeExt, x)
	}
}

func (e *encoder) putDigests(hdr int, xs []Digest) {
	p := e.putSlice(hdr, len(xs), SizeDigest, SizeElement)
	for i, x := range xs {
		e.putDigest(p+i*SizeDigest, x)
	}
}

func (e *encoder) putProof(off int, p Proof) {
	// Written before the payloads so the root's own bytes stay contiguous.
	e.putPcsOpening(off+OffProofPcsOpening, p.PcsOpening)

	rounds := e.putSlice(off+OffProofRounds, len(p.Rounds), SizeRoundMessage, 8)
	for i, r := range p.Rounds {
		e.putRoundMessage(rounds+i*SizeRoundMessage, r)
	}

	sizes := e.putSlice(off+OffProofModuleSizes, len(p.ModuleSizes), SizeUsize, SizeUsize)
	for i, s := range p.ModuleSizes {
		e.putUsize(sizes+i*SizeUsize, s)
	}
}

func (e *encoder) putRoundMessage(off int, r RoundMessage) {
	commitment := off + OffRoundMessageCommitment
	if r.Commitment != nil {
		e.buf[commitment+OffOptCommitmentFlag] = TagOptCommitmentPresent
		e.putDigest(commitment+OffOptCommitmentPayload, *r.Commitment)
	} else {
		// Flag stays 0 and the payload stays zeroed: Zig reads neither.
		e.buf[commitment+OffOptCommitmentFlag] = TagOptCommitmentNull
	}

	cells := e.putSlice(off+OffRoundMessageCells, len(r.Cells), SizeScalar, SizeElement)
	for i, c := range r.Cells {
		e.putScalar(cells+i*SizeScalar, c)
	}
}

func (e *encoder) putScalar(off int, s Scalar) {
	// The payload is the full 24-byte Ext either way; only the tag differs. A
	// base-valued Gen already stores its value lifted, so writing all 24 bytes
	// is both correct and canonical.
	e.putExt(off+OffScalarPayload, s.Value)
	if s.IsExt {
		e.buf[off+OffScalarTag] = TagScalarExt
		return
	}
	e.buf[off+OffScalarTag] = TagScalarBase
}

func (e *encoder) putPcsOpening(off int, p PcsOpening) {
	e.putOpeningProof(off+OffPcsOpeningProof, p.Proof)
}

func (e *encoder) putOpeningProof(off int, p OpeningProof) {
	e.putFriProof(off+OffOpeningProofFriProof, p.FriProof)

	queries := e.putSlice(off+OffOpeningProofInputQueries, len(p.InputQueries), SizeSlice, 8)
	for i, q := range p.InputQueries {
		inner := e.putSlice(queries+i*SizeSlice, len(q), SizeInputTreeOpen, 8)
		for j, open := range q {
			e.putInputTreeOpening(inner+j*SizeInputTreeOpen, open)
		}
	}
}

func (e *encoder) putFriProof(off int, p FriProof) {
	e.putDigests(off+OffFriProofRoundRoots, p.RoundRoots)
	e.putExts(off+OffFriProofFinalPoly, p.FinalPoly)

	queries := e.putSlice(off+OffFriProofRunningQueries, len(p.RunningQueries), SizeSlice, 8)
	for i, rq := range p.RunningQueries {
		inner := e.putSlice(queries+i*SizeSlice, len(rq), SizeBranch, 8)
		for j, br := range rq {
			e.putBranch(inner+j*SizeBranch, br)
		}
	}
}

func (e *encoder) putBranch(off int, b Branch) {
	e.putDigest(off+OffBranchLeaf, b.Leaf)
	e.putDigests(off+OffBranchSiblings, b.Siblings)
}

func (e *encoder) putInputTreeOpening(off int, o InputTreeOpening) {
	e.putDigests(off+OffInputTreeOpeningSiblings, o.Siblings)

	leaves := e.putSlice(off+OffInputTreeOpeningLeaves, len(o.Leaves), SizeOptRowPair, 8)
	for i, l := range o.Leaves {
		slot := leaves + i*SizeOptRowPair
		if l == nil {
			// Flag stays 0 and the payload stays zeroed: Zig reads neither.
			e.buf[slot+OffOptRowPairFlag] = TagOptRowPairNull
			continue
		}
		e.buf[slot+OffOptRowPairFlag] = TagOptRowPairPresent
		for k, row := range l {
			e.putRowOpening(slot+OffOptRowPairPayload+k*SizeRowOpening, row)
		}
	}
}

func (e *encoder) putRowOpening(off int, r RowOpening) {
	e.putElements(off+OffRowOpeningBase, r.Base)
	e.putExts(off+OffRowOpeningExt, r.Ext)
}
