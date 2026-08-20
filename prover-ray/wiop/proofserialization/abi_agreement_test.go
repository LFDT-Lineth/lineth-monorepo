// Tests that keep prover-ray's encoder and verifier-ray in step. Two halves:
// the layout numbers must agree (TestABIAgreement), and the image fixture
// verifier-ray reads must match what the encoder currently produces
// (TestVerifierRayImageIsUpToDate).
package proofserialization_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	zkccodegen "github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
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
		"verifier.VerifyInput":    ps.SizeVerifyInput,
		"value.Scalar":            ps.SizeScalar,
		"?protocol.Commitment":    ps.SizeOptCommitment,
		"?merkle.RowPair":         ps.SizeOptRowPair,
	}

	// wantOffset maps a pinned (type, field) to this package's offset constant.
	wantOffset := map[[2]string]int{
		{"ext.Ext", "B0"}: 0,
		{"ext.Ext", "B1"}: 8,
		{"ext.Ext", "B2"}: 16,

		{"protocol.RoundMessage", "cells"}:      ps.OffRoundMessageCells,
		{"protocol.RoundMessage", "commitment"}: ps.OffRoundMessageCommitment,

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

		{"verifier.PcsOpening", "proof"}: ps.OffPcsOpeningProof,

		{"verifier.VerifyInput", "proof"}:         ps.OffVerifyInputProof,
		{"verifier.VerifyInput", "public_inputs"}: ps.OffVerifyInputPublicInputs,

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
		{"value.Scalar", "base"}: ps.TagScalarBase,
		{"value.Scalar", "ext"}:  ps.TagScalarExt,
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

// TestVerifierRayImageIsUpToDate keeps the committed cross-language fixture in sync.
//
// The image is committed rather than generated at Zig test time so verifier-ray's
// suite stays self-contained — it needs no Go toolchain. The cost is that the
// file can go stale, which is what this test prevents. Regenerate with
// UPDATE_VERIFIER_RAY_IMAGE=1.
func TestVerifierRayImageIsUpToDate(t *testing.T) {
	image, err := buildHonestRiscvImage()
	require.NoError(t, err)
	require.NoError(t, ps.Validate(image, ps.GuestBase))

	// The fixture must survive our own round trip before it is worth asking Zig
	// to read it.
	decoded, err := ps.Decode(image, ps.GuestBase)
	require.NoError(t, err)
	reencoded, err := ps.Encode(decoded, ps.GuestBase)
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

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = zkccodegen.DEFAULT_CONFIG
)

func buildHonestRiscvImage() ([]byte, error) {
	sourcePath, err := honestRiscvSourcePath()
	if err != nil {
		return nil, err
	}
	binF, err := compileBinaryConstraints(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("compiling %s: %w", sourcePath, err)
	}
	compiledConstraints, err := binF.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling %s constraints: %w", sourcePath, err)
	}
	honestInputs, err := zkc_r5.PrepareInput(zkc_r5.ExitZeroGuestELF, nil)
	if err != nil {
		return nil, fmt.Errorf("PrepareInput(minimal exit guest): %w", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: honestInputs}

	sys := wiop.NewSystemf("zkc-riscv-system")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	runCompilePipeline(sys)

	proof, pub := sys.Prove(
		func(assignRt *wiop.Runtime) {
			driver.AssignWithPreRead(assignRt, inputs, koalafield.Octuplet{})
		},
		wiop.ProveOptions{CheckUnreducedQueries: true},
	)
	if err := sys.Verify(proof, pub); err != nil {
		return nil, fmt.Errorf("verifying %s proof: %w", sourcePath, err)
	}

	projected, err := ps.Project(sys, proof, pub)
	if err != nil {
		return nil, fmt.Errorf("proofserialization.Project: %w", err)
	}
	return ps.Encode(projected, ps.GuestBase)
}

func honestRiscvSourcePath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolving proofserialization test source path: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "arithmetization", "src", "main", "riscv", "main.zkc"), nil
}

func compileBinaryConstraints(srcPath string) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during zkc compilation: %v", r)
		}
	}()

	srcZkc, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read zkc source file: %w", err)
	}
	src := source.NewSourceFile(srcPath, srcZkc)
	macroProgram, _, errs := compiler.Compile(zkcField, *src)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	binfile = constraints.NewBinaryFile[koalabear.Element](nil, nil, zkcField, zkcCfg.GetMaxStaticHeight(), ir)
	return binfile, nil
}

func runCompilePipeline(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)
}
