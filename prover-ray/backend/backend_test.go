package backend

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
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

	fromFull, err := predecoding.PrepareInputs(minimal_elf.MinimalElfProgram, ssz)
	require.NoError(t, err)

	assert.Equal(t, fromFull, fromCore, "precomputed path must produce identical output to buildZkcInputs")
}

func TestCore_BuildInputs_DoesNotMutateCachedProgram(t *testing.T) {
	program, decoded := prepareTestProgram(t)
	c := &Core{program: program, decoded: decoded}
	wantProgramData := append([]byte(nil), c.program.Blobs[0].Data...)
	wantDecoded := append([]byte(nil), c.decoded.Decoded...)

	const workers = 16
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for i := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			inputs, err := c.buildInputs(Job{Payload: []byte{byte(i)}})
			if err != nil {
				errors <- err
				return
			}
			if !bytes.Equal(inputs[predecoding.DecodedInput], wantDecoded) {
				errors <- fmt.Errorf("worker %d received mutated decoded input", i)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	assert.Equal(t, wantProgramData, c.program.Blobs[0].Data)
	assert.Equal(t, wantDecoded, c.decoded.Decoded)
}

// embeddedZKCBin serializes the embedded R5 arithmetization, writes it to a
// temp file, and returns the path. Core.New reads a compiled circuit bin from
// disk and binds its guest_output memory as public inputs unconditionally, so
// the bin must declare one.
func embeddedZKCBin(t *testing.T) string {
	t.Helper()

	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
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

	c, err := New(Config{CircuitBinPath: embeddedZKCBin(t), GuestELFPath: elfPath})
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
	binPath := embeddedZKCBin(t)

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
