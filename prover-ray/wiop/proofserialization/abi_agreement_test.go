// Tests that keep prover-ray's encoder and verifier-ray in step. Two halves:
// the layout numbers must agree (TestABIAgreement), and the image fixture
// verifier-ray reads must match what the encoder currently produces
// (TestVerifierRayImageIsUpToDate).
package proofserialization_test

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

// proofABIPath is verifier-ray's pinned layout, the authority this package
// encodes against.
const proofABIPath = "../../../verifier-ray/src/proof_abi.zig"

// TestABIAgreement checks this package's size and offset constants against the
// assertions in verifier-ray/src/proof_abi.zig.
//
// This closes the one drift direction nothing else covers. proof_abi.zig catches
// Zig's layout moving out from under the pins; the encoder's own tests catch Go
// bugs against Go's constants. Neither notices if the two sides' NUMBERS diverge
// — someone updating a pin in Zig without updating the encoder — and that
// failure mode is silent: the image still casts cleanly and the verifier reads
// misplaced bytes.
//
// It reads the Zig source rather than building it, so it costs nothing and does
// not couple the Go tests to a Zig toolchain.
func TestABIAgreement(t *testing.T) {
	src, err := os.ReadFile(proofABIPath)
	if err != nil {
		t.Skipf("verifier-ray not checked out alongside prover-ray (%v); "+
			"the ABI cross-check needs %s", err, proofABIPath)
	}

	// wantSize maps a Zig type as written in proof_abi.zig to this package's
	// corresponding size constant. Types with no Go counterpart in the image
	// (the slice header itself is checked via SizeSlice) are listed explicitly so
	// a newly pinned type shows up as unmapped rather than being ignored.
	wantSize := map[string]int{
		"[]const u8":              ps.SizeSlice,
		"base.Element":            ps.SizeElement,
		"ext.Ext":                 ps.SizeExt,
		"poseidon2.Digest":        ps.SizeDigest,
		"protocol.RoundMessage":   ps.SizeRoundMessage,
		"merkle.RowOpening":       ps.SizeRowOpening,
		"merkle.RowPair":          ps.SizeRowPair,
		"merkle.InputTreeOpening": ps.SizeInputTreeOpen,
		"merkle.Branch":           ps.SizeBranch,
		"fri.Proof":               ps.SizeFriProof,
		"pcs.OpeningProof":        ps.SizeOpeningProof,
		"verifier.PcsOpening":     ps.SizePcsOpening,
		"verifier.Proof":          ps.SizeProof,
		"value.Scalar":            ps.SizeScalar,
		"value.Vector":            ps.SizeVector,
		"protocol.ColumnMessage":  ps.SizeColumnMessage,
		"?merkle.RowPair":         ps.SizeOptRowPair,
	}

	// wantOffset maps a pinned (type, field) to this package's offset constant.
	wantOffset := map[[2]string]int{
		{"ext.Ext", "B0"}: 0,
		{"ext.Ext", "B1"}: 8,
		{"ext.Ext", "B2"}: 16,

		{"protocol.RoundMessage", "columns"}: ps.OffRoundMessageColumns,
		{"protocol.RoundMessage", "cells"}:   ps.OffRoundMessageCells,

		{"merkle.RowOpening", "base"}: ps.OffRowOpeningBase,
		{"merkle.RowOpening", "ext"}:  ps.OffRowOpeningExt,

		{"merkle.InputTreeOpening", "siblings"}: ps.OffInputTreeOpeningSiblings,
		{"merkle.InputTreeOpening", "leaves"}:   ps.OffInputTreeOpeningLeaves,

		{"merkle.Branch", "siblings"}: ps.OffBranchSiblings,
		{"merkle.Branch", "leaf"}:     ps.OffBranchLeaf,

		{"fri.Proof", "round_roots"}:     ps.OffFriProofRoundRoots,
		{"fri.Proof", "final_poly"}:      ps.OffFriProofFinalPoly,
		{"fri.Proof", "running_queries"}: ps.OffFriProofRunningQueries,

		{"pcs.OpeningProof", "input_queries"}: ps.OffOpeningProofInputQueries,
		{"pcs.OpeningProof", "fri_proof"}:     ps.OffOpeningProofFriProof,

		{"verifier.PcsOpening", "entry_claims"}: ps.OffPcsOpeningEntryClaims,
		{"verifier.PcsOpening", "proof"}:        ps.OffPcsOpeningProof,

		{"verifier.Proof", "rounds"}:       ps.OffProofRounds,
		{"verifier.Proof", "module_sizes"}: ps.OffProofModuleSizes,
		{"verifier.Proof", "pcs_opening"}:  ps.OffProofPcsOpening,
	}

	// These must reference the package's constants, not literal copies of the
	// pinned values. Hardcoding the numbers here compares Zig's pin against a
	// copy of itself, which passes no matter what the encoder actually writes —
	// a mutation run caught exactly that, with TagColumnPublic changed to 2 and
	// every test still green.
	wantTag := map[[2]string]int{
		{"value.Scalar", "base"}:                        ps.TagScalarBase,
		{"value.Scalar", "ext"}:                         ps.TagScalarExt,
		{"value.Vector", "base"}:                        ps.TagVectorBase,
		{"value.Vector", "ext"}:                         ps.TagVectorExt,
		{"protocol.ColumnMessage", "oracle_commitment"}: ps.TagColumnOracle,
		{"protocol.ColumnMessage", "public_column"}:     ps.TagColumnPublic,
	}

	sizeRe := regexp.MustCompile(`expectSize\((\??[\w.\[\]\s]+?),\s*(\d+),\s*(\d+)\);`)
	fieldRe := regexp.MustCompile(`expectField\(([\w.]+),\s*"(\w+)",\s*(\d+)\);`)
	tagRe := regexp.MustCompile(`expectTag\(([\w.]+),\s*\.(\w+),\s*(\d+)\);`)

	sizes := sizeRe.FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, sizes, "found no expectSize assertions in %s; has it been restructured?",
		proofABIPath)
	for _, m := range sizes {
		zigType, want := m[1], mustAtoi(t, m[2])
		got, ok := wantSize[zigType]
		require.True(t, ok, "%s pins a size for %q that this package does not map to a "+
			"constant; add it to wantSize (and to the encoder if the image carries it)",
			proofABIPath, zigType)
		require.Equal(t, want, got,
			"size of %s: %s pins %d, proofserialization uses %d — the encoder would write the "+
				"wrong number of bytes", zigType, proofABIPath, want, got)
	}

	fields := fieldRe.FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, fields, "found no expectField assertions in %s", proofABIPath)
	for _, m := range fields {
		key := [2]string{m[1], m[2]}
		want := mustAtoi(t, m[3])
		got, ok := wantOffset[key]
		require.True(t, ok, "%s pins an offset for %s.%s that this package does not map; "+
			"add it to wantOffset", proofABIPath, key[0], key[1])
		require.Equal(t, want, got,
			"offset of %s.%s: %s pins %d, proofserialization uses %d — the encoder would write "+
				"this field to the wrong place", key[0], key[1], proofABIPath, want, got)
	}

	tags := tagRe.FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, tags, "found no expectTag assertions in %s", proofABIPath)
	for _, m := range tags {
		key := [2]string{m[1], m[2]}
		want := mustAtoi(t, m[3])
		got, ok := wantTag[key]
		require.True(t, ok, "%s pins a discriminant for %s.%s that this package does not map; "+
			"add it to wantTag", proofABIPath, key[0], key[1])
		require.Equal(t, want, got,
			"discriminant of %s.%s: %s pins %d, proofserialization uses %d — the encoder would "+
				"select the wrong variant", key[0], key[1], proofABIPath, want, got)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err, "parsing %q from %s", s, proofABIPath)
	return n
}

// verifierRayImagePath is the image verifier-ray's proof_image_test.zig maps and
// reads. It is the only test in which a byte produced by this encoder is
// consumed by the actual verifier rather than by this package's own decoder.
const verifierRayImagePath = "../../../verifier-ray/testdata/proof_image.bin"

// verifierRayImageBase is the address the fixture image is relocated for, and
// the address verifier-ray's Zig test maps it at with MAP_FIXED.
//
// Not GuestBase: macOS reserves the low address space, so mapping at 0x08800000
// fails there (measured — 0x08800000, 0x30000000 and 0x100000000 all fail on
// arm64 macOS, 0x400000000 succeeds). The production image still uses GuestBase;
// this is a test-only address chosen to be mappable on both hosts.
const verifierRayImageBase = 0x400000000

// verifierRayFixture is the fixture both sides agree on. Deliberately small and with
// hand-picked values, so the Zig assertions read as literals rather than as a
// second implementation of the encoder.
//
// Element values are raw u32s, not results of field arithmetic: the image stores
// Montgomery limbs verbatim, so both sides compare the same raw numbers without
// either having to do arithmetic.
func verifierRayFixture() ps.Proof {
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

// TestVerifierRayImageIsUpToDate keeps the committed cross-language fixture in sync.
//
// The image is committed rather than generated at Zig test time so verifier-ray's
// suite stays self-contained — it needs no Go toolchain. The cost is that the
// file can go stale, which is what this test prevents. Regenerate with
// UPDATE_VERIFIER_RAY_IMAGE=1.
func TestVerifierRayImageIsUpToDate(t *testing.T) {
	image, err := ps.Encode(verifierRayFixture(), verifierRayImageBase)
	require.NoError(t, err)
	require.NoError(t, ps.Validate(image, verifierRayImageBase))

	// The fixture must survive our own round trip before it is worth asking Zig
	// to read it.
	decoded, err := ps.Decode(image, verifierRayImageBase)
	require.NoError(t, err)
	reencoded, err := ps.Encode(decoded, verifierRayImageBase)
	require.NoError(t, err)
	require.Equal(t, image, reencoded)

	if os.Getenv("UPDATE_VERIFIER_RAY_IMAGE") != "" {
		require.NoError(t, os.WriteFile(verifierRayImagePath, image, 0o600))
		t.Logf("wrote %d bytes to %s", len(image), verifierRayImagePath)
		return
	}

	committed, err := os.ReadFile(verifierRayImagePath)
	if err != nil {
		t.Skipf("verifier-ray not checked out alongside prover-ray (%v); "+
			"run UPDATE_VERIFIER_RAY_IMAGE=1 go test ./wiop/proofserialization/ to create %s",
			err, verifierRayImagePath)
	}

	require.Equal(t, committed, image,
		"the image verifier-ray reads is stale. verifier-ray's proof_image_test.zig "+
			"asserts against it, so regenerate with UPDATE_VERIFIER_RAY_IMAGE=1 and re-run "+
			"`zig build test` in verifier-ray to confirm the Zig side still agrees")
}
