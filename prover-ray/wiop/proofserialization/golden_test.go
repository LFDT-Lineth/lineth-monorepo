package proofserialization_test

import (
	"os"
	"testing"

	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

// goldenImagePath is the image verifier-ray's proof_image_test.zig maps and
// reads. It is the only test in which a byte produced by this encoder is
// consumed by the actual verifier rather than by this package's own decoder.
const goldenImagePath = "../../../verifier-ray/testdata/proof_image.bin"

// goldenBase is the address the golden image is relocated for, and the address
// the Zig test maps it at with MAP_FIXED.
//
// Not GuestBase: macOS reserves the low address space, so mapping at 0x08800000
// fails there (measured — 0x08800000, 0x30000000 and 0x100000000 all fail on
// arm64 macOS, 0x400000000 succeeds). The production image still uses GuestBase;
// this is a test-only address chosen to be mappable on both hosts.
const goldenBase = 0x400000000

// goldenProof is the fixture both sides agree on. Deliberately small and with
// hand-picked values, so the Zig assertions read as literals rather than as a
// second implementation of the encoder.
//
// Element values are raw u32s, not results of field arithmetic: the image stores
// Montgomery limbs verbatim, so both sides compare the same raw numbers without
// either having to do arithmetic.
func goldenProof() ps.Proof {
	return ps.Proof{
		Rounds: []ps.RoundMessage{
			{
				// An oracle commitment, and one cell of each variant.
				Columns: []ps.ColumnMessage{
					{Commitment: ps.Digest{10, 11, 12, 13, 14, 15, 16, 17}},
				},
				Cells: []ps.Scalar{
					{Value: ps.Ext{100, 101, 102, 103, 104, 105}},              // base
					{Value: ps.Ext{200, 201, 202, 203, 204, 205}, IsExt: true}, // ext
				},
			},
			{
				// Both Vector variants. wiop does not currently produce public
				// columns, but the format carries them and the encoder must get
				// both discriminants right.
				Columns: []ps.ColumnMessage{
					{IsPublic: true, PublicColumn: ps.Vector{Base: []ps.Element{31, 32, 33}}},
					{IsPublic: true, PublicColumn: ps.Vector{IsExt: true, Ext: []ps.Ext{
						{41, 42, 43, 44, 45, 46},
					}}},
				},
			},
			{}, // an empty round: empty slices must still be readable
		},
		ModuleSizes: []uint64{8, 16},
		PcsOpening: ps.PcsOpening{
			EntryClaims: [][]ps.Ext{
				{{50, 51, 52, 53, 54, 55}, {60, 61, 62, 63, 64, 65}},
				nil, // a zero-length inner slice
			},
			Proof: ps.OpeningProof{
				InputQueries: [][]ps.InputTreeOpening{
					{
						{
							Siblings: []ps.Digest{{70, 71, 72, 73, 74, 75, 76, 77}},
							Leaves: []*ps.RowPair{
								nil, // a null level
								{
									{Base: []ps.Element{80, 81}, Ext: []ps.Ext{{90, 91, 92, 93, 94, 95}}},
									{Base: []ps.Element{82, 83}, Ext: nil},
								},
							},
						},
					},
				},
				FriProof: ps.FriProof{
					RoundRoots: []ps.Digest{{110, 111, 112, 113, 114, 115, 116, 117}},
					FinalPoly:  []ps.Ext{{120, 121, 122, 123, 124, 125}},
					RunningQueries: [][]ps.Branch{
						{
							{
								Siblings: []ps.Digest{{130, 131, 132, 133, 134, 135, 136, 137}},
								Leaf:     ps.Digest{140, 141, 142, 143, 144, 145, 146, 147},
							},
						},
					},
				},
			},
		},
	}
}

// TestGoldenImage keeps the committed cross-language fixture in sync.
//
// The image is committed rather than generated at Zig test time so verifier-ray's
// suite stays self-contained — it needs no Go toolchain. The cost is that the
// file can go stale, which is what this test prevents. Regenerate with
// UPDATE_GOLDEN_IMAGE=1.
func TestGoldenImage(t *testing.T) {
	image, err := ps.Encode(goldenProof(), goldenBase)
	require.NoError(t, err)
	require.NoError(t, ps.Validate(image, goldenBase))

	// The fixture must survive our own round trip before it is worth asking Zig
	// to read it.
	decoded, err := ps.Decode(image, goldenBase)
	require.NoError(t, err)
	reencoded, err := ps.Encode(decoded, goldenBase)
	require.NoError(t, err)
	require.Equal(t, image, reencoded)

	if os.Getenv("UPDATE_GOLDEN_IMAGE") != "" {
		require.NoError(t, os.WriteFile(goldenImagePath, image, 0o644))
		t.Logf("wrote %d bytes to %s", len(image), goldenImagePath)
		return
	}

	committed, err := os.ReadFile(goldenImagePath)
	if err != nil {
		t.Skipf("verifier-ray not checked out alongside prover-ray (%v); "+
			"run UPDATE_GOLDEN_IMAGE=1 go test ./wiop/proofserialization/ to create %s",
			err, goldenImagePath)
	}

	require.Equal(t, committed, image,
		"the committed cross-language image is stale. verifier-ray's proof_image_test.zig "+
			"asserts against it, so regenerate with UPDATE_GOLDEN_IMAGE=1 and re-run "+
			"`zig build test` in verifier-ray to confirm the Zig side still agrees")
}
