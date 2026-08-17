package proofserialization

import (
	"encoding/binary"
	"fmt"
)

// Decode reads an image produced by [Encode] back into a [Proof], resolving
// pointers relative to base.
//
// The guest does none of this — it casts the image and reads native pointers.
// Decode exists for two host-side purposes: round-trip testing, and validating
// an image before it is handed to a guest ([Validate]).
//
// Every pointer and length is checked against the image bounds and its type's
// alignment. That matters more than it looks: a zero-decode format has no parse
// step, so it has no natural place to reject a malformed proof. If a bad image
// reaches the guest, the cast still succeeds and the verifier reads out of
// bounds. Recursion depth is bounded by the type nesting, not by the image, so a
// hostile image cannot drive this into unbounded recursion.
//
// Zero-length slices decode as nil. Go distinguishes a nil slice from an empty
// one and the image does not — Zig's []const T is just {ptr, len} — so a decoded
// proof is equal to the original up to that distinction, and re-encoding it
// reproduces the same bytes.
func Decode(image []byte, base uint64) (Proof, error) {
	d := &decoder{buf: image, base: base}
	if len(image) < SizeProof {
		return Proof{}, fmt.Errorf("proofserialization: image is %d bytes, shorter than the "+
			"%d-byte root", len(image), SizeProof)
	}
	if base%8 != 0 {
		return Proof{}, fmt.Errorf("proofserialization: base 0x%x is not 8-byte aligned", base)
	}
	if base == 0 {
		return Proof{}, fmt.Errorf("proofserialization: base 0 is not usable: an in-image " +
			"pointer would be indistinguishable from null")
	}
	return d.proof(0)
}

// Validate reports whether image is a well-formed proof image at base: every
// pointer in range and aligned, every length consistent with the image size.
//
// It validates structure, not values — the verifier's own checks
// (fri.checkOpeningProofShape and friends) remain responsible for those. It
// allocates, since it decodes to walk; that is acceptable because it runs
// host-side, never in the guest.
func Validate(image []byte, base uint64) error {
	_, err := Decode(image, base)
	return err
}

type decoder struct {
	buf  []byte
	base uint64
}

// slice resolves the slice header at off, returning the payload offset and the
// element count.
func (d *decoder) slice(off, elemSize, elemAlign int, what string) (int, int, error) {
	if off < 0 || off+SizeSlice > len(d.buf) {
		return 0, 0, fmt.Errorf("proofserialization: %s: slice header at %d is outside the "+
			"%d-byte image", what, off, len(d.buf))
	}
	ptr := binary.LittleEndian.Uint64(d.buf[off:])
	count := binary.LittleEndian.Uint64(d.buf[off+8:])

	if ptr == 0 {
		return 0, 0, fmt.Errorf("proofserialization: %s: null slice pointer; Zig's []const T "+
			"holds a non-optional pointer, so null is undefined behaviour even at length 0", what)
	}
	if ptr < d.base {
		return 0, 0, fmt.Errorf("proofserialization: %s: pointer 0x%x is below the image base 0x%x",
			what, ptr, d.base)
	}
	payload := ptr - d.base
	if payload > uint64(len(d.buf)) {
		return 0, 0, fmt.Errorf("proofserialization: %s: pointer 0x%x is past the end of the "+
			"%d-byte image at base 0x%x", what, ptr, len(d.buf), d.base)
	}
	if count == 0 {
		return int(payload), 0, nil
	}
	// Bound the count before multiplying, so a hostile length cannot overflow.
	if count > uint64(len(d.buf)/elemSize) {
		return 0, 0, fmt.Errorf("proofserialization: %s: length %d exceeds what a %d-byte image "+
			"can hold at %d bytes per element", what, count, len(d.buf), elemSize)
	}
	if payload%uint64(elemAlign) != 0 {
		return 0, 0, fmt.Errorf("proofserialization: %s: payload offset %d is not %d-byte aligned",
			what, payload, elemAlign)
	}
	if payload+count*uint64(elemSize) > uint64(len(d.buf)) {
		return 0, 0, fmt.Errorf("proofserialization: %s: %d elements of %d bytes at offset %d "+
			"run past the end of the %d-byte image", what, count, elemSize, payload, len(d.buf))
	}
	return int(payload), int(count), nil
}

func (d *decoder) element(off int) Element {
	return Element(binary.LittleEndian.Uint32(d.buf[off:]))
}

func (d *decoder) ext(off int) Ext {
	var e Ext
	for i := range e {
		e[i] = d.element(off + i*SizeElement)
	}
	return e
}

func (d *decoder) digest(off int) Digest {
	var g Digest
	for i := range g {
		g[i] = d.element(off + i*SizeElement)
	}
	return g
}

func (d *decoder) elements(off int, what string) ([]Element, error) {
	p, n, err := d.slice(off, SizeElement, SizeElement, what)
	if err != nil || n == 0 {
		return nil, err
	}
	out := make([]Element, n)
	for i := range out {
		out[i] = d.element(p + i*SizeElement)
	}
	return out, nil
}

func (d *decoder) exts(off int, what string) ([]Ext, error) {
	p, n, err := d.slice(off, SizeExt, SizeElement, what)
	if err != nil || n == 0 {
		return nil, err
	}
	out := make([]Ext, n)
	for i := range out {
		out[i] = d.ext(p + i*SizeExt)
	}
	return out, nil
}

func (d *decoder) digests(off int, what string) ([]Digest, error) {
	p, n, err := d.slice(off, SizeDigest, SizeElement, what)
	if err != nil || n == 0 {
		return nil, err
	}
	out := make([]Digest, n)
	for i := range out {
		out[i] = d.digest(p + i*SizeDigest)
	}
	return out, nil
}

func (d *decoder) proof(off int) (Proof, error) {
	var p Proof
	var err error

	roundsOff, n, err := d.slice(off+OffProofRounds, SizeRoundMessage, 8, "Proof.rounds")
	if err != nil {
		return p, err
	}
	if n > 0 {
		p.Rounds = make([]RoundMessage, n)
		for i := range p.Rounds {
			if p.Rounds[i], err = d.roundMessage(roundsOff + i*SizeRoundMessage); err != nil {
				return p, err
			}
		}
	}

	sizesOff, n, err := d.slice(off+OffProofModuleSizes, SizeUsize, SizeUsize, "Proof.module_sizes")
	if err != nil {
		return p, err
	}
	if n > 0 {
		p.ModuleSizes = make([]uint64, n)
		for i := range p.ModuleSizes {
			p.ModuleSizes[i] = binary.LittleEndian.Uint64(d.buf[sizesOff+i*SizeUsize:])
		}
	}

	if p.PcsOpening, err = d.pcsOpening(off + OffProofPcsOpening); err != nil {
		return p, err
	}
	return p, nil
}

func (d *decoder) roundMessage(off int) (RoundMessage, error) {
	var r RoundMessage
	var err error

	commitment := off + OffRoundMessageCommitment
	switch flag := d.buf[commitment+OffOptCommitmentFlag]; flag {
	case TagOptCommitmentNull:
		r.Commitment = nil
	case TagOptCommitmentPresent:
		digest := d.digest(commitment + OffOptCommitmentPayload)
		r.Commitment = &digest
	default:
		return r, fmt.Errorf("proofserialization: RoundMessage.commitment: presence flag %d "+
			"is neither null (%d) nor present (%d)", flag, TagOptCommitmentNull, TagOptCommitmentPresent)
	}

	cellsOff, n, err := d.slice(off+OffRoundMessageCells, SizeScalar, SizeElement, "RoundMessage.cells")
	if err != nil {
		return r, err
	}
	if n > 0 {
		r.Cells = make([]Scalar, n)
		for i := range r.Cells {
			if r.Cells[i], err = d.scalar(cellsOff + i*SizeScalar); err != nil {
				return r, err
			}
		}
	}
	return r, nil
}

func (d *decoder) scalar(off int) (Scalar, error) {
	tag := d.buf[off+OffScalarTag]
	switch tag {
	case TagScalarBase:
		return Scalar{Value: d.ext(off + OffScalarPayload)}, nil
	case TagScalarExt:
		return Scalar{Value: d.ext(off + OffScalarPayload), IsExt: true}, nil
	default:
		return Scalar{}, fmt.Errorf("proofserialization: Scalar: discriminant %d is neither "+
			"base (%d) nor ext (%d)", tag, TagScalarBase, TagScalarExt)
	}
}

func (d *decoder) pcsOpening(off int) (PcsOpening, error) {
	var p PcsOpening
	var err error

	claimsOff, n, err := d.slice(off+OffPcsOpeningEntryClaims, SizeSlice, 8, "PcsOpening.entry_claims")
	if err != nil {
		return p, err
	}
	if n > 0 {
		p.EntryClaims = make([][]Ext, n)
		for i := range p.EntryClaims {
			if p.EntryClaims[i], err = d.exts(claimsOff+i*SizeSlice, "PcsOpening.entry_claims[i]"); err != nil {
				return p, err
			}
		}
	}

	p.Proof, err = d.openingProof(off + OffPcsOpeningProof)
	return p, err
}

func (d *decoder) openingProof(off int) (OpeningProof, error) {
	var p OpeningProof
	var err error

	queriesOff, n, err := d.slice(off+OffOpeningProofInputQueries, SizeSlice, 8, "OpeningProof.input_queries")
	if err != nil {
		return p, err
	}
	if n > 0 {
		p.InputQueries = make([][]InputTreeOpening, n)
		for i := range p.InputQueries {
			innerOff, m, err := d.slice(queriesOff+i*SizeSlice, SizeInputTreeOpen, 8, "OpeningProof.input_queries[i]")
			if err != nil {
				return p, err
			}
			if m == 0 {
				continue
			}
			p.InputQueries[i] = make([]InputTreeOpening, m)
			for j := range p.InputQueries[i] {
				if p.InputQueries[i][j], err = d.inputTreeOpening(innerOff + j*SizeInputTreeOpen); err != nil {
					return p, err
				}
			}
		}
	}

	p.FriProof, err = d.friProof(off + OffOpeningProofFriProof)
	return p, err
}

func (d *decoder) friProof(off int) (FriProof, error) {
	var p FriProof
	var err error

	if p.RoundRoots, err = d.digests(off+OffFriProofRoundRoots, "fri.Proof.round_roots"); err != nil {
		return p, err
	}
	if p.FinalPoly, err = d.exts(off+OffFriProofFinalPoly, "fri.Proof.final_poly"); err != nil {
		return p, err
	}

	queriesOff, n, err := d.slice(off+OffFriProofRunningQueries, SizeSlice, 8, "fri.Proof.running_queries")
	if err != nil {
		return p, err
	}
	if n > 0 {
		p.RunningQueries = make([][]Branch, n)
		for i := range p.RunningQueries {
			innerOff, m, err := d.slice(queriesOff+i*SizeSlice, SizeBranch, 8, "fri.Proof.running_queries[i]")
			if err != nil {
				return p, err
			}
			if m == 0 {
				continue
			}
			p.RunningQueries[i] = make([]Branch, m)
			for j := range p.RunningQueries[i] {
				if p.RunningQueries[i][j], err = d.branch(innerOff + j*SizeBranch); err != nil {
					return p, err
				}
			}
		}
	}
	return p, nil
}

func (d *decoder) branch(off int) (Branch, error) {
	sibs, err := d.digests(off+OffBranchSiblings, "merkle.Branch.siblings")
	if err != nil {
		return Branch{}, err
	}
	return Branch{Siblings: sibs, Leaf: d.digest(off + OffBranchLeaf)}, nil
}

func (d *decoder) inputTreeOpening(off int) (InputTreeOpening, error) {
	var o InputTreeOpening
	var err error

	if o.Siblings, err = d.digests(off+OffInputTreeOpeningSiblings, "InputTreeOpening.siblings"); err != nil {
		return o, err
	}

	leavesOff, n, err := d.slice(off+OffInputTreeOpeningLeaves, SizeOptRowPair, 8, "InputTreeOpening.leaves")
	if err != nil {
		return o, err
	}
	if n == 0 {
		return o, nil
	}
	o.Leaves = make([]*RowPair, n)
	for i := range o.Leaves {
		slot := leavesOff + i*SizeOptRowPair
		switch flag := d.buf[slot+OffOptRowPairFlag]; flag {
		case TagOptRowPairNull:
			o.Leaves[i] = nil
		case TagOptRowPairPresent:
			var pair RowPair
			for k := range pair {
				if pair[k], err = d.rowOpening(slot + OffOptRowPairPayload + k*SizeRowOpening); err != nil {
					return o, err
				}
			}
			o.Leaves[i] = &pair
		default:
			return o, fmt.Errorf("proofserialization: ?RowPair: presence flag %d is neither "+
				"null (%d) nor present (%d)", flag, TagOptRowPairNull, TagOptRowPairPresent)
		}
	}
	return o, nil
}

func (d *decoder) rowOpening(off int) (RowOpening, error) {
	var r RowOpening
	var err error
	if r.Base, err = d.elements(off+OffRowOpeningBase, "RowOpening.base"); err != nil {
		return r, err
	}
	r.Ext, err = d.exts(off+OffRowOpeningExt, "RowOpening.ext")
	return r, err
}
