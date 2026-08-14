package proofserialization

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// The types below mirror verifier-ray's proof types one-for-one. They exist so
// the encoder has something with the verifier's shape to walk: a wiop.Proof is
// structurally different (maps keyed by ObjectID rather than round-major dense
// arrays), so serialization is a projection onto these types followed by an
// exact dump of them. See docs/proof-serialization.md sections 2 and 3.
//
// Field order here matches the Zig declarations, which since
// verifier-ray/src/crypto/merkle.zig declares align-descending is also the
// memory order. The offset constants in encode.go are the authority for the
// encoder, though — never the field order of these Go structs, whose layout Go
// chooses independently.

// Element is one KoalaBear base field element: a single u32 in Montgomery form,
// stored verbatim. Mirrors Zig's field.Element.
type Element uint32

// Ext is a degree-6 extension element, flattened in memory order:
// B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1. Mirrors Zig's ext.Ext (E6), whose
// E2 pairs are {a0 @0, a1 @4} inside B0 @0, B1 @8, B2 @16.
type Ext [6]Element

// Digest is a Poseidon2 digest / Merkle commitment. Mirrors Zig's
// poseidon2.Digest and crypto.commitment.Commitment, both [8]Element.
type Digest [8]Element

// Scalar is one cell value. Mirrors Zig's value.Scalar, a tagged union whose
// 24-byte payload is always an Ext and whose discriminant sits at byte 24.
//
// IsExt is the discriminant, NOT Go's field.Gen.IsBase(): the polarity is
// inverted (Zig tags base as 0, Go stores true for base), which is why the
// conversion goes through [ScalarFrom] rather than a memcpy.
type Scalar struct {
	Value Ext
	IsExt bool
}

// Vector is a public column's values. Mirrors Zig's value.Vector, a tagged union
// over two slice types. Exactly one of Base and Ext is meaningful, selected by
// IsExt.
type Vector struct {
	Base  []Element
	Ext   []Ext
	IsExt bool
}

// ColumnMessage is one column's verifier-visible data. Mirrors Zig's
// protocol.ColumnMessage: either the Merkle commitment of an oracle column or a
// public column's raw values. IsPublic selects which.
type ColumnMessage struct {
	Commitment   Digest
	PublicColumn Vector
	IsPublic     bool
}

// RoundMessage is one round's verifier-visible data. Mirrors Zig's
// protocol.RoundMessage.
type RoundMessage struct {
	Columns []ColumnMessage
	Cells   []Scalar
}

// RowOpening is one committed row's preimage. Mirrors Zig's merkle.RowOpening.
type RowOpening struct {
	Base []Element
	Ext  []Ext
}

// RowPair is one level's conjugate row pair. Mirrors Zig's merkle.RowPair,
// which is [2]RowOpening.
type RowPair [2]RowOpening

// Branch is a Merkle opening for one running FRI layer. Mirrors Zig's
// merkle.Branch.
//
// Note the field order: Siblings precedes Leaf, matching both the Zig
// declaration and the memory layout. Zig's align-descending sort puts the
// align-8 slice first regardless of how it is declared.
type Branch struct {
	Siblings []Digest
	Leaf     Digest
}

// InputTreeOpening is a Merkle branch whose path leaves are row preimages.
// Mirrors Zig's merkle.InputTreeOpening.
//
// A nil entry in Leaves is Zig's `null` for that level, encoded as a
// ?merkle.RowPair with its presence flag clear.
type InputTreeOpening struct {
	Siblings []Digest
	Leaves   []*RowPair
}

// FriProof is the running-layer FRI proof. Mirrors Zig's fri.Proof.
type FriProof struct {
	RoundRoots     []Digest
	FinalPoly      []Ext
	RunningQueries [][]Branch
}

// OpeningProof bundles the PCS input-tree openings with the FRI proof. Mirrors
// Zig's pcs.OpeningProof.
type OpeningProof struct {
	InputQueries [][]InputTreeOpening
	FriProof     FriProof
}

// PcsOpening is the prover-supplied half of a PCS opening. Mirrors Zig's
// verifier.PcsOpening. EntryClaims is jagged: one inner slice per opened entry,
// holding that entry's claim per shift.
type PcsOpening struct {
	EntryClaims [][]Ext
	Proof       OpeningProof
}

// Proof is the root of the image. Mirrors Zig's verifier.Proof.
//
// It must land at image offset 0, because verifier-ray's loaders cast the input
// region's base address directly to *const Proof without parsing.
type Proof struct {
	Rounds      []RoundMessage
	ModuleSizes []uint64
	PcsOpening  PcsOpening
}

// ---------------------------------------------------------------------------
// Conversions from the prover-side field types.
//
// These are the seam the wiop.Proof projection will use. They are spelled out
// rather than done by unsafe cast: Go's field.Ext and this Ext happen to have
// the same size and layout today, but relying on that would make the image
// silently wrong the moment either side changes.
// ---------------------------------------------------------------------------

// ExtFrom converts a prover-side extension element, preserving Montgomery form.
func ExtFrom(e field.Ext) Ext {
	return Ext{
		Element(e.B0.A0[0]), Element(e.B0.A1[0]),
		Element(e.B1.A0[0]), Element(e.B1.A1[0]),
		Element(e.B2.A0[0]), Element(e.B2.A1[0]),
	}
}

// DigestFrom converts a prover-side commitment.
func DigestFrom(o field.Octuplet) Digest {
	var d Digest
	for i := range o {
		d[i] = Element(o[i][0])
	}
	return d
}

// ScalarFrom converts a prover-side cell value, inverting the base/ext tag:
// field.Gen records true for base, Zig's discriminant records 0 for base.
func ScalarFrom(g field.Gen) Scalar {
	return Scalar{Value: ExtFrom(g.Ext), IsExt: !g.IsBase()}
}

// ElementsFrom converts a slice of prover-side base elements.
//
// A zero-length input becomes nil, not an empty slice. The image cannot
// represent the difference -- Zig's []const T is just {ptr, len} -- so nil is the
// canonical form, and producing it here keeps a projected proof equal to what
// decoding its own image gives back.
func ElementsFrom(xs []field.Element) []Element {
	if len(xs) == 0 {
		return nil
	}
	out := make([]Element, len(xs))
	for i := range xs {
		out[i] = Element(xs[i][0])
	}
	return out
}

// ExtsFrom converts a slice of prover-side extension elements. A zero-length
// input becomes nil; see [ElementsFrom].
func ExtsFrom(xs []field.Ext) []Ext {
	if len(xs) == 0 {
		return nil
	}
	out := make([]Ext, len(xs))
	for i := range xs {
		out[i] = ExtFrom(xs[i])
	}
	return out
}

// DigestsFrom converts a slice of prover-side commitments. A zero-length input
// becomes nil; see [ElementsFrom].
func DigestsFrom(xs []field.Octuplet) []Digest {
	if len(xs) == 0 {
		return nil
	}
	out := make([]Digest, len(xs))
	for i := range xs {
		out[i] = DigestFrom(xs[i])
	}
	return out
}
