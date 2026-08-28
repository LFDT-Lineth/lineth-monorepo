package backend

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCore_BuildInputs_UsesPrecomputedELFBlobs(t *testing.T) {
	program, decoded := prepareTestProgram(t)
	c := &Core{program: program, decoded: decoded}

	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0xFF, 0xFE}

	inputs1, err := c.buildInputs(Job{Payload: payload1})
	require.NoError(t, err)
	inputs2, err := c.buildInputs(Job{Payload: payload2})
	require.NoError(t, err)

	assert.NotEqual(t, inputs1["blobs_data"], inputs2["blobs_data"], "different payloads must produce different blobs_data")
	assert.Equal(t, inputs1["entry_point_and_blobs_count"], inputs2["entry_point_and_blobs_count"], "same ELF must produce identical entry_point_and_blobs_count")
}

func TestCore_BuildInputs_MatchesBuildZkcInputs(t *testing.T) {
	program, decoded := prepareTestProgram(t)
	c := &Core{program: program, decoded: decoded}

	ssz := []byte{0xAA, 0xBB}

	// Core.buildInputs (precomputed path) must produce identical output to
	// buildZkcInputs (parse-every-call helper).
	fromCore, err := c.buildInputs(Job{Payload: ssz})
	require.NoError(t, err)

	fromFull, err := zkc_r5.PrepareInput(minimal_elf.MinimalElfProgram, ssz)
	require.NoError(t, err)

	assert.Equal(t, fromFull, fromCore, "precomputed path must produce identical output to buildZkcInputs")
}

// zkcTestSrc is a small ZkC source program shared with the zkcdriver tests;
// compileZKCBin turns it into a bin that NewZkCDriver accepts. It declares a
// public output memory of [risc5.NumGuestPublicOutputs] bytes, like every real
// guest circuit does, because [New] binds that memory as public inputs
// unconditionally.
const zkcTestSrc = "../zkcdriver/testdata/guest_output.zkc"

// compileZKCBin compiles a .zkc source into a serialized ZkC binary in the
// current zkc format, writes it to a temp file, and returns the path. Core.New
// reads a compiled circuit bin from disk, so tests need one built from source
// rather than a checked-in artifact that goes stale on every zkc version bump.
//
// This mirrors compileBinaryConstraints in zkcdriver's test package; it can be
// dropped in favor of a shared call once zkcdriver exposes a public compile
// helper.
func compileZKCBin(t *testing.T, srcPath string) string {
	t.Helper()

	srcBytes, err := os.ReadFile(srcPath)
	require.NoError(t, err)

	src := source.NewSourceFile(srcPath, srcBytes)
	zkcField := field.KOALABEAR_16
	zkcCfg := codegen.DEFAULT_CONFIG

	macroProgram, _, errs := compiler.Compile(zkcField, zkcCfg.GetMaxStaticHeight(), *src)
	if len(errs) > 0 {
		t.Fatalf("zkc macro compile %q: %v", srcPath, errs)
	}
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	if len(errs) > 0 {
		t.Fatalf("zkc ast compile %q: %v", srcPath, errs)
	}

	binF := constraints.NewBinaryFile[koalabear.Element](nil, nil, ir)
	binBytes, err := binF.MarshalBinary()
	require.NoError(t, err)

	binPath := filepath.Join(t.TempDir(), "circuit.bin")
	//nolint:gosec // G703 false positive: binPath is under the test's own t.TempDir().
	require.NoError(t, os.WriteFile(binPath, binBytes, 0o600))
	return binPath
}

// TestNew verifies that New precomputes the ELF blobs and entry point at
// construction and that the resulting Core builds the same inputs as the
// parse-every-call path.
func TestNew(t *testing.T) {
	elfBytes := minimal_elf.Make(minimal_elf.DefaultEntryPoint, minimal_elf.DefaultSectionAddr, minimal_elf.ValidSectionData)
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elfBytes, 0o600))

	c, err := New(Config{CircuitBinPath: compileZKCBin(t, zkcTestSrc), GuestELFPath: elfPath})
	require.NoError(t, err)

	assert.Len(t, c.program.Blobs, 1, "one loadable section must be precomputed")
	assert.Equal(t, uint64(minimal_elf.DefaultEntryPoint), c.program.EntryPoint, "entry point must be precomputed")
	assert.NotEmpty(t, c.decoded.Decoded, "decoded program must be precomputed")

	ssz := []byte{0xAA, 0xBB}
	fromCore, err := c.buildInputs(Job{Payload: ssz})
	require.NoError(t, err)
	fromFull, err := predecoding.PrepareInputs(minimal_elf.MinimalElfProgram, ssz)
	require.NoError(t, err)
	assert.Equal(t, fromFull, fromCore)
}

func prepareTestProgram(t *testing.T) (elfmapping.Program, predecoding.DecodedProgram) {
	t.Helper()
	program, err := elfmapping.Load(bytes.NewReader(minimal_elf.MinimalElfProgram))
	require.NoError(t, err)
	decoded, err := predecoding.Predecode(program)
	require.NoError(t, err)
	return program, decoded
}

// TestNew_Errors verifies that New reports missing or invalid startup inputs
// with errors naming the offending path.
func TestNew_Errors(t *testing.T) {
	elfBytes := minimal_elf.Make(minimal_elf.DefaultEntryPoint, minimal_elf.DefaultSectionAddr, minimal_elf.ValidSectionData)
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elfBytes, 0o600))

	badELFPath := filepath.Join(t.TempDir(), "bad.elf")
	require.NoError(t, os.WriteFile(badELFPath, []byte("not an elf"), 0o600))

	// A valid circuit bin so the guest-ELF cases fail on the ELF, not the bin.
	binPath := compileZKCBin(t, zkcTestSrc)

	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"MissingCircuitBin",
			Config{CircuitBinPath: "does/not/exist.bin", GuestELFPath: elfPath},
			"circuit bin"},
		{"MissingGuestELF",
			Config{CircuitBinPath: binPath, GuestELFPath: "does/not/exist.elf"},
			"guest ELF"},
		{"InvalidGuestELF",
			Config{CircuitBinPath: binPath, GuestELFPath: badELFPath},
			"ELF"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
