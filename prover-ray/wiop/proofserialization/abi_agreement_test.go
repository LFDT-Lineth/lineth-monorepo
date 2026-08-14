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
