package proofserialization_test

import (
	"encoding/binary"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

const testBase = ps.GuestBase

// richProof exercises every branch of the encoder: both Scalar variants, both
// ColumnMessage variants, both Vector variants, present and null RowPairs, jagged
// entry claims, and empty slices at several depths.
func richProof() ps.Proof {
	ext := func(n uint32) ps.Ext {
		return ps.Ext{ps.Element(n), ps.Element(n + 1), ps.Element(n + 2),
			ps.Element(n + 3), ps.Element(n + 4), ps.Element(n + 5)}
	}
	dig := func(n uint32) ps.Digest {
		var d ps.Digest
		for i := range d {
			d[i] = ps.Element(n + uint32(i))
		}
		return d
	}

	return ps.Proof{
		Rounds: []ps.RoundMessage{
			{
				// An oracle commitment plus a base-valued public column.
				Columns: []ps.ColumnMessage{
					{Commitment: dig(100)},
					{IsPublic: true, PublicColumn: ps.Vector{Base: []ps.Element{7, 8, 9}}},
				},
				Cells: []ps.Scalar{
					{Value: ext(200)},              // base variant
					{Value: ext(300), IsExt: true}, // ext variant
				},
			},
			{
				// An extension-valued public column, and no cells at all.
				Columns: []ps.ColumnMessage{
					{IsPublic: true, PublicColumn: ps.Vector{IsExt: true, Ext: []ps.Ext{ext(400), ext(410)}}},
				},
			},
			// A completely empty round.
			{},
		},
		ModuleSizes: []uint64{8, 16, 1 << 20},
		PcsOpening: ps.PcsOpening{
			// Jagged, including an empty inner slice.
			EntryClaims: [][]ps.Ext{
				{ext(500), ext(510)},
				nil, // zero-length inner slice
				{ext(520)},
			},
			Proof: ps.OpeningProof{
				InputQueries: [][]ps.InputTreeOpening{
					{
						{
							Siblings: []ps.Digest{dig(600), dig(610)},
							Leaves: []*ps.RowPair{
								nil, // null level
								{
									{Base: []ps.Element{1, 2}, Ext: []ps.Ext{ext(700)}},
									{Base: []ps.Element{3, 4}, Ext: []ps.Ext{ext(710)}},
								},
								{
									// Empty base, non-empty ext: both slice
									// headers still have to be well-formed.
									{Base: nil, Ext: []ps.Ext{ext(720)}},
									{Base: []ps.Element{5}, Ext: nil},
								},
							},
						},
						{Siblings: nil, Leaves: nil},
					},
					nil, // a query with no input trees
				},
				FriProof: ps.FriProof{
					RoundRoots: []ps.Digest{dig(800), dig(810)},
					FinalPoly:  []ps.Ext{ext(900)},
					RunningQueries: [][]ps.Branch{
						{
							{Siblings: []ps.Digest{dig(1000)}, Leaf: dig(1010)},
							{Siblings: nil, Leaf: dig(1020)},
						},
						nil, // a query with no branches
					},
				},
			},
		},
	}
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	want := richProof()

	image, err := ps.Encode(want, testBase)
	require.NoError(t, err)

	got, err := ps.Decode(image, testBase)
	require.NoError(t, err)

	require.Equal(t, want, got, "decoding an encoded proof must reproduce it exactly")

	// The property that actually matters is that the IMAGE is stable, since the
	// image is what the guest casts. Re-encoding the decoded value must reproduce
	// it byte for byte.
	again, err := ps.Encode(got, testBase)
	require.NoError(t, err)
	require.Equal(t, image, again, "encode(decode(image)) must be the identity on images")
}

// TestEncode_NilAndEmptySlicesAreIndistinguishable pins a deliberate lossiness:
// Go separates a nil slice from a zero-length one, Zig does not — a []const T is
// just {ptr, len} — so both encode to the same header and decode back as nil.
//
// Worth asserting rather than leaving implicit: it is the one way the round trip
// is not an exact Go-value identity, and a future encoder change that tried to
// preserve the distinction would be inventing information the verifier cannot
// read.
func TestEncode_NilAndEmptySlicesAreIndistinguishable(t *testing.T) {
	withNil := ps.Proof{Rounds: []ps.RoundMessage{{Cells: nil}}}
	withEmpty := ps.Proof{Rounds: []ps.RoundMessage{{Cells: []ps.Scalar{}}}}

	a, err := ps.Encode(withNil, testBase)
	require.NoError(t, err)
	b, err := ps.Encode(withEmpty, testBase)
	require.NoError(t, err)
	require.Equal(t, a, b, "nil and empty must produce the same image")

	decoded, err := ps.Decode(b, testBase)
	require.NoError(t, err)
	require.Nil(t, decoded.Rounds[0].Cells, "a zero-length slice decodes back as nil")
}

func TestEncode_Deterministic(t *testing.T) {
	p := richProof()

	first, err := ps.Encode(p, testBase)
	require.NoError(t, err)
	second, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	require.Equal(t, first, second,
		"the image must be a pure function of the proof and base, so it can be hashed and diffed")
}

func TestEncode_RootAtOffsetZero(t *testing.T) {
	image, err := ps.Encode(richProof(), testBase)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(image), ps.SizeProof,
		"the image must at least hold the root")

	// verifier-ray's loaders cast the base address straight to *const Proof, so
	// the root's slice headers must be the first thing in the image.
	rounds := binary.LittleEndian.Uint64(image[0:])
	require.GreaterOrEqual(t, rounds, uint64(testBase),
		"Proof.rounds pointer at offset 0 must point into the image")
	require.Equal(t, uint64(3), binary.LittleEndian.Uint64(image[8:]),
		"Proof.rounds length must be at offset 8")
	require.Equal(t, uint64(3), binary.LittleEndian.Uint64(image[24:]),
		"Proof.module_sizes length must be at offset 24")
}

func TestEncode_EmptySliceHasNonNullPointer(t *testing.T) {
	// A []const T in Zig holds a non-optional pointer, so a null pointer is
	// undefined behaviour even at length 0.
	p := ps.Proof{}
	image, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		off  int
	}{
		{"Proof.rounds", 0},
		{"Proof.module_sizes", 16},
	} {
		ptr := binary.LittleEndian.Uint64(image[tc.off:])
		count := binary.LittleEndian.Uint64(image[tc.off+8:])
		require.Zero(t, count, "%s should be empty in this fixture", tc.name)
		require.NotZero(t, ptr, "%s: an empty slice must still carry a non-null pointer", tc.name)
		require.Zero(t, ptr%8, "%s: an empty slice's pointer must stay aligned", tc.name)
	}
}

func TestEncode_ScalarTagPolarity(t *testing.T) {
	// Zig tags .base as 0; Go's field.Gen stores true for base. Getting this
	// backwards would silently flip every cell's variant, which is why the
	// conversion is not a memcpy.
	p := ps.Proof{Rounds: []ps.RoundMessage{{Cells: []ps.Scalar{
		{Value: ps.Ext{1}, IsExt: false},
		{Value: ps.Ext{2}, IsExt: true},
	}}}}

	image, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	cellsPtr := binary.LittleEndian.Uint64(image[0:]) // rounds payload
	roundOff := int(cellsPtr - testBase)
	cellsOff := int(binary.LittleEndian.Uint64(image[roundOff+16:]) - testBase)

	require.Equal(t, byte(0), image[cellsOff+24],
		"a base-valued Scalar must carry discriminant 0")
	require.Equal(t, byte(1), image[cellsOff+ps.SizeScalar+24],
		"an extension-valued Scalar must carry discriminant 1")
}

// TestEncode_ColumnMessageTagValues asserts the discriminant bytes literally,
// independently of the package's own constants.
//
// Round-trip tests cannot see a wrong tag value: encode and decode share the
// constant, so changing it keeps them in agreement while making the image
// disagree with the verifier. Only a literal assertion here or the ABI
// cross-check catches that.
func TestEncode_ColumnMessageTagValues(t *testing.T) {
	p := ps.Proof{Rounds: []ps.RoundMessage{{Columns: []ps.ColumnMessage{
		{Commitment: ps.Digest{1}},
		{IsPublic: true, PublicColumn: ps.Vector{Base: []ps.Element{2}}},
		{IsPublic: true, PublicColumn: ps.Vector{IsExt: true, Ext: []ps.Ext{{3}}}},
	}}}}

	image, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	roundOff := int(binary.LittleEndian.Uint64(image[0:]) - testBase)
	colsOff := int(binary.LittleEndian.Uint64(image[roundOff:]) - testBase)

	require.Equal(t, byte(0), image[colsOff+32],
		"an oracle column's discriminant must be 0 at byte 32")
	require.Equal(t, byte(1), image[colsOff+40+32],
		"a public column's discriminant must be 1 at byte 32")

	// The nested Vector carries its own discriminant at byte 16 of the payload.
	require.Equal(t, byte(0), image[colsOff+40+16],
		"a base-valued Vector's discriminant must be 0 at byte 16")
	require.Equal(t, byte(1), image[colsOff+80+16],
		"an extension-valued Vector's discriminant must be 1 at byte 16")
}

func TestScalarFrom_InvertsGoTag(t *testing.T) {
	base := ps.ScalarFrom(field.ElemFromBase(field.NewElement(5)))
	require.False(t, base.IsExt, "a base Gen must become the Zig .base variant")

	ext := ps.ScalarFrom(field.ElemFromExt(field.Lift(field.NewElement(5))))
	require.True(t, ext.IsExt, "an ext Gen must become the Zig .ext variant")

	// Both hold the same numeric value; only the discriminant differs. That is
	// exactly the transcript-malleability surface documented in the spec.
	require.Equal(t, base.Value, ext.Value,
		"the tag is metadata: it does not change the 24-byte payload")
}

func TestEncode_PaddingIsZeroed(t *testing.T) {
	// Zig leaves padding undefined. Zeroing it is what makes the image
	// reproducible; anything else would make it unhashable for no benefit.
	p := ps.Proof{Rounds: []ps.RoundMessage{{Cells: []ps.Scalar{{Value: ps.Ext{1}}}}}}
	image, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	roundOff := int(binary.LittleEndian.Uint64(image[0:]) - testBase)
	cellsOff := int(binary.LittleEndian.Uint64(image[roundOff+16:]) - testBase)

	// Scalar is 28 bytes: 24 payload, 1 tag, 3 padding.
	require.Equal(t, []byte{0, 0, 0}, image[cellsOff+25:cellsOff+28],
		"the 3 bytes after a Scalar's discriminant must be zeroed")
}

func TestEncode_OptRowPairFlagAndPadding(t *testing.T) {
	present := &ps.RowPair{
		{Base: []ps.Element{1}, Ext: nil},
		{Base: []ps.Element{2}, Ext: nil},
	}
	p := ps.Proof{PcsOpening: ps.PcsOpening{Proof: ps.OpeningProof{
		InputQueries: [][]ps.InputTreeOpening{{{Leaves: []*ps.RowPair{nil, present}}}},
	}}}

	image, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	// Proof.pcs_opening @32 -> .proof @16 -> .input_queries @0
	qOuter := int(binary.LittleEndian.Uint64(image[32+16+0:]) - testBase)
	qInner := int(binary.LittleEndian.Uint64(image[qOuter:]) - testBase)
	leaves := int(binary.LittleEndian.Uint64(image[qInner+16:]) - testBase)

	require.Equal(t, byte(0), image[leaves+64],
		"a null leaf must clear the presence flag")
	require.Equal(t, byte(1), image[leaves+ps.SizeOptRowPair+64],
		"a present leaf must set the presence flag")
	// ?RowPair is 72 bytes: 64 payload, 1 flag, 7 padding.
	require.Equal(t, make([]byte, 7), image[leaves+65:leaves+72],
		"the 7 bytes after the presence flag must be zeroed")

	// A null leaf's 64-byte payload is never read by the verifier, but leaving
	// it uninitialised would break determinism.
	require.Equal(t, make([]byte, 64), image[leaves:leaves+64],
		"a null leaf's payload must be zeroed")
}

func TestEncode_RejectsMisalignedBase(t *testing.T) {
	_, err := ps.Encode(ps.Proof{}, ps.GuestBase+1)
	require.ErrorContains(t, err, "not 8-byte aligned",
		"every type in the image has alignment 8 or less, and the root cast requires it")
}

func TestDecode_RejectsMalformedImages(t *testing.T) {
	good, err := ps.Encode(richProof(), testBase)
	require.NoError(t, err)
	require.NoError(t, ps.Validate(good, testBase), "the fixture must validate")

	corrupt := func(mutate func([]byte)) []byte {
		c := make([]byte, len(good))
		copy(c, good)
		mutate(c)
		return c
	}

	for _, tc := range []struct {
		name  string
		image []byte
		want  string
	}{
		{
			name:  "truncated below the root",
			image: good[:ps.SizeProof-1],
			want:  "shorter than the",
		},
		{
			name:  "null slice pointer",
			image: corrupt(func(b []byte) { binary.LittleEndian.PutUint64(b[0:], 0) }),
			want:  "null slice pointer",
		},
		{
			name:  "pointer below the base",
			image: corrupt(func(b []byte) { binary.LittleEndian.PutUint64(b[0:], testBase-8) }),
			want:  "below the image base",
		},
		{
			name:  "pointer past the end",
			image: corrupt(func(b []byte) { binary.LittleEndian.PutUint64(b[0:], testBase+uint64(len(good))+8) }),
			want:  "past the end",
		},
		{
			// A zero-length slice dereferences nothing, so only the pointer-range
			// check can reject this. Without a length-0 case the check is dead
			// weight that no test exercises — a mutation run proved exactly that,
			// since the case above is really caught by the length check.
			name: "zero-length slice pointing outside the image",
			image: corrupt(func(b []byte) {
				binary.LittleEndian.PutUint64(b[16:], testBase+uint64(len(good))+8) // module_sizes ptr
				binary.LittleEndian.PutUint64(b[24:], 0)                            // ... with length 0
			}),
			want: "past the end",
		},
		{
			name:  "length exceeding the image",
			image: corrupt(func(b []byte) { binary.LittleEndian.PutUint64(b[8:], 1<<40) }),
			want:  "exceeds what a",
		},
		{
			name:  "elements running past the end",
			image: corrupt(func(b []byte) { binary.LittleEndian.PutUint64(b[24:], uint64(len(good))/8) }),
			want:  "run past the end",
		},
		{
			name: "misaligned payload",
			image: corrupt(func(b []byte) {
				binary.LittleEndian.PutUint64(b[16:], testBase+1) // module_sizes ptr
				binary.LittleEndian.PutUint64(b[24:], 1)
			}),
			want: "not 8-byte aligned",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ps.Validate(tc.image, testBase)
			require.ErrorContains(t, err, tc.want,
				"a zero-decode format has no parse step, so the validator is the only "+
					"thing standing between a malformed image and out-of-bounds guest reads")
		})
	}
}

func TestDecode_RejectsUnknownDiscriminants(t *testing.T) {
	p := ps.Proof{Rounds: []ps.RoundMessage{{
		Columns: []ps.ColumnMessage{{Commitment: ps.Digest{1}}},
		Cells:   []ps.Scalar{{Value: ps.Ext{1}}},
	}}}
	good, err := ps.Encode(p, testBase)
	require.NoError(t, err)

	roundOff := int(binary.LittleEndian.Uint64(good[0:]) - testBase)
	colsOff := int(binary.LittleEndian.Uint64(good[roundOff:]) - testBase)
	cellsOff := int(binary.LittleEndian.Uint64(good[roundOff+16:]) - testBase)

	for _, tc := range []struct {
		name string
		off  int
		want string
	}{
		{"Scalar", cellsOff + 24, "Scalar: discriminant 7"},
		{"ColumnMessage", colsOff + 32, "ColumnMessage: discriminant 7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := make([]byte, len(good))
			copy(c, good)
			c[tc.off] = 7
			require.ErrorContains(t, ps.Validate(c, testBase), tc.want,
				"an out-of-range discriminant would select a variant Zig does not have")
		})
	}
}

func TestEncode_RelocatesForBase(t *testing.T) {
	p := richProof()

	atGuest, err := ps.Encode(p, ps.GuestBase)
	require.NoError(t, err)
	const otherBase = 0x10000000
	atOther, err := ps.Encode(p, otherBase)
	require.NoError(t, err)

	require.Equal(t, len(atGuest), len(atOther),
		"the base shifts pointers but must not change the layout or size")
	require.NotEqual(t, atGuest, atOther,
		"pointers are absolute, so a different base must produce different bytes")

	// Each image is only valid at its own base — the reason section 5.2 of the
	// spec wants the native loader to map at a fixed address.
	require.NoError(t, ps.Validate(atOther, otherBase))
	require.Error(t, ps.Validate(atGuest, otherBase),
		"an image relocated for one base must not validate at another")

	roundTripped, err := ps.Decode(atOther, otherBase)
	require.NoError(t, err)
	require.Equal(t, p, roundTripped, "relocation must not change the decoded value")
}

// TestEncode_RejectsZeroBase pins why base 0 is refused rather than silently
// producing an image whose empty-slice pointers read as null.
func TestEncode_RejectsZeroBase(t *testing.T) {
	_, err := ps.Encode(ps.Proof{}, 0)
	require.ErrorContains(t, err, "base 0 is not usable",
		"at base 0 an in-image pointer is indistinguishable from null")
}
