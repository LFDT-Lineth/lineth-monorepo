package proofserialization

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

// The types below mirror verifier-ray's proof types one-for-one. They exist so
// the encoder has something with the verifier's shape to walk: a wiop.Proof is
// structurally different (maps keyed by ObjectID rather than round-major dense
// arrays), so serialization is a projection onto these types followed by an
// exact dump of them. See README.md sections 2 and 3.
//
// Field order here matches the Zig declarations, which since
// verifier-ray/src/crypto/merkle.zig declares align-descending is also the
// memory order. The offset constants in encode.go are the authority for the
// encoder, though — never the field order of these Go structs, whose layout Go
// chooses independently.

// Element is one KoalaBear base field element: a single u32 in canonical form
// (0 ≤ value < modulus). Mirrors Zig's field.Element, which stores canonical
// representatives and uses plain modular arithmetic.
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

// RoundMessage is one round's verifier-visible data. Mirrors Zig's
// protocol.RoundMessage.
//
// Columns never travel raw: a committed round is represented solely by its
// Merkle root, and Commitment is nil for a round that commits nothing.
type RoundMessage struct {
	Cells      []Scalar
	Commitment *Digest
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

// MerkleCap mirrors verifier-ray's merkle.MerkleCap. Aux contains optional
// digests: a nil entry means that the corresponding auxiliary node is absent.
type MerkleCap struct {
	Nodes []Digest
	Aux   []*Digest
}

// InputCap mirrors verifier-ray's pcs.InputCap. It authenticates the shared
// frontier of one input tree and carries the fully revealed tables above it.
type InputCap struct {
	Nodes  []Digest
	Tables []InputCapTable
}

// InputCapTable mirrors verifier-ray's pcs.InputCapTable. Rows are the
// fully-revealed encoded rows for one auxiliary table.
type InputCapTable struct {
	SizeLog2 uint8
	Rows     []RowOpening
}

// FriProof is the running-layer FRI proof. Mirrors Zig's fri.Proof.
type FriProof struct {
	RoundRoots     []Digest
	RoundCaps      []MerkleCap
	FinalPoly      []Ext
	RunningQueries [][]Branch
}

// OpeningProof bundles the PCS input-tree openings with the FRI proof. Mirrors
// Zig's pcs.OpeningProof.
type OpeningProof struct {
	InputQueries [][]InputTreeOpening
	InputCaps    []InputCap
	FriProof     FriProof
}

// Proof mirrors Zig's verifier.Proof: the verifier-visible transcript.
//
// Note that Rounds[*].Cells OMITS public-input cells; those travel in
// [VerifyInput.PublicInputs] instead, in registration order.
type Proof struct {
	Rounds      []RoundMessage
	ModuleSizes []uint64
	// The claimed evaluations are not carried here: they are ordinary
	// `LagrangeEval.EvaluationClaims` cells, already present in
	// `Proof.Rounds[*].Cells`. The verifier reconstructs them at verify time by
	// reading the compiled system's per-column claim-cell table against the
	// rounds it already has.
	PcsOpening OpeningProof
}

// VerifyInput is the root of the image. Mirrors Zig's verifier.VerifyInput.
//
// It must land at image offset 0: verifier-ray's loaders cast the input region's
// base address directly to *const VerifyInput without parsing.
//
// PublicInputs is the flat public-input statement in prover-ray registration
// order — one entry per cell registered via System.RegisterPublicInputs. The
// verifier absorbs these separately from the round messages, which is why
// Proof.Rounds[*].Cells must not repeat them.
type VerifyInput struct {
	Proof        Proof
	PublicInputs []Scalar
}

// ---------------------------------------------------------------------------
// Conversions from the prover-side field types.
//
// These are the seam the wiop.Proof projection will use. They are spelled out
// rather than done by unsafe cast: Go's field.Ext and this Ext happen to have
// the same size and layout today, but relying on that would make the image
// silently wrong the moment either side changes.
// ---------------------------------------------------------------------------

// ExtFrom converts a prover-side extension element to canonical form.
// Go's field.Ext stores limbs in Montgomery form; Zig's ext.Ext stores canonical
// representatives. Bits() performs the fromMont conversion.
func ExtFrom(e field.Ext) Ext {
	return Ext{
		Element(e.B0.A0.Bits()[0]), Element(e.B0.A1.Bits()[0]),
		Element(e.B1.A0.Bits()[0]), Element(e.B1.A1.Bits()[0]),
		Element(e.B2.A0.Bits()[0]), Element(e.B2.A1.Bits()[0]),
	}
}

// DigestFrom converts a prover-side commitment to canonical form.
func DigestFrom(o field.Octuplet) Digest {
	var d Digest
	for i := range o {
		d[i] = Element(o[i].Bits()[0])
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
		out[i] = Element(xs[i].Bits()[0])
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
