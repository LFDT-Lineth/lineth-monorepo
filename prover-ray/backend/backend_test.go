package backend

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCore_BuildInputs_UsesPrecomputedELFBlobs(t *testing.T) {
	g := prepareTestProgram(t)
	c := &Core{}

	payload1 := []byte{0x01, 0x02}
	payload2 := []byte{0xFF, 0xFE}

	inputs1, err := c.buildInputs(Job{Payload: payload1}, g)
	require.NoError(t, err)
	inputs2, err := c.buildInputs(Job{Payload: payload2}, g)
	require.NoError(t, err)

	assert.NotEqual(t, inputs1["blobs_data"], inputs2["blobs_data"], "different payloads must produce different blobs_data")
	assert.Equal(t, inputs1["entry_point_and_blobs_count"], inputs2["entry_point_and_blobs_count"], "same ELF must produce identical entry_point_and_blobs_count")
}

func TestCore_BuildInputs_MatchesBuildZkcInputs(t *testing.T) {
	g := prepareTestProgram(t)
	c := &Core{}

	ssz := []byte{0xAA, 0xBB}

	// Core.buildInputs (precomputed path) must produce identical output to
	// buildZkcInputs (parse-every-call helper).
	fromCore, err := c.buildInputs(Job{Payload: ssz}, g)
	require.NoError(t, err)

	fromFull, err := predecoding.PrepareInputs(minimal_elf.MinimalElfProgram, ssz)
	require.NoError(t, err)

	assert.Equal(t, fromFull, fromCore, "precomputed path must produce identical output to buildZkcInputs")
}

func TestCore_BuildInputs_DoesNotMutateCachedProgram(t *testing.T) {
	g := prepareTestProgram(t)
	c := &Core{}
	wantProgramData := append([]byte(nil), g.program.Blobs[0].Data...)
	wantDecoded := append([]byte(nil), g.decoded.Decoded...)

	const workers = 16
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for i := range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			inputs, err := c.buildInputs(Job{Payload: []byte{byte(i)}}, g)
			if err != nil {
				errors <- err
				return
			}
			if got := inputs[predecoding.DecodedInput]; !bytes.Equal(got.Bytes(), wantDecoded) {
				errors <- fmt.Errorf("worker %d received mutated decoded input", i)
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	assert.Equal(t, wantProgramData, g.program.Blobs[0].Data)
	assert.Equal(t, wantDecoded, g.decoded.Decoded)
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

// TestNew checks that New needs no guest ELF: the guest is loaded per job.
func TestNew(t *testing.T) {
	_, err := New(Config{CircuitBinPath: embeddedZKCBin(t), GuestsDir: t.TempDir()})
	require.NoError(t, err, "an empty guests folder is enough to start")
}

func prepareTestProgram(t *testing.T) guestProgram {
	t.Helper()
	g, err := parseGuest(minimal_elf.MinimalElfProgram)
	require.NoError(t, err)
	return g
}

// TestNew_Errors verifies that New reports missing startup inputs.
func TestNew_Errors(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{"MissingCircuitBin",
			Config{CircuitBinPath: "does/not/exist.bin", GuestsDir: t.TempDir()},
			"circuit bin"},
		{"MissingGuestsDir",
			Config{CircuitBinPath: embeddedZKCBin(t)},
			"guests folder"},
		{"DevZkVMMissingGuestsDir",
			Config{Mode: ProverModeDevZkVM},
			"guests folder"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
