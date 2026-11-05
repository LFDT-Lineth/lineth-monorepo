package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/guests"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeGuest writes elf into dir under a release name and returns its Program ID.
func writeGuest(t *testing.T, dir string, elf []byte) [32]byte {
	t.Helper()
	id := guests.ProgramID(elf)
	name := fmt.Sprintf("test-v0.0.0-%x.elf", id[:])
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), elf, 0o600)) //nolint:gosec // G703: test-controlled path
	return id
}

func TestLoadGuest(t *testing.T) {
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.MinimalElfProgram)
	c := &Core{guests: guests.Store{Dir: dir}}

	g, err := c.loadGuest(id)
	require.NoError(t, err)
	ssz := []byte{0xAA, 0xBB}
	got, err := c.buildInputs(Job{Payload: ssz}, g)
	require.NoError(t, err)
	want, err := predecoding.PrepareInputs(minimal_elf.MinimalElfProgram, ssz)
	require.NoError(t, err)
	assert.Equal(t, want, got, "the guest named by the ID must give the same inputs as parsing its ELF")
}

func TestLoadGuest_Errors(t *testing.T) {
	dir := t.TempDir()
	notELF := writeGuest(t, dir, []byte("not an elf"))
	c := &Core{guests: guests.Store{Dir: dir}}

	_, err := c.loadGuest([32]byte{1})
	require.ErrorIs(t, err, guests.ErrUnknownGuest, "an ID with no file is an unknown guest")

	_, err = c.loadGuest(notELF)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ELF", "a file that is not an ELF must fail to load")
}

func TestProve_UnknownGuest(t *testing.T) {
	for _, mode := range []ProverMode{ProverModeDevZkVM, ProverModeFull} {
		t.Run(string(mode), func(t *testing.T) {
			c := &Core{mode: mode, guests: guests.Store{Dir: t.TempDir()}}
			res := c.Prove(context.Background(), Job{ID: "unknown-guest", GuestProgramID: [32]byte{1}})
			require.Equal(t, ResultStatusFailed, res.Status)
			assert.ErrorIs(t, res.Err, guests.ErrUnknownGuest, "the job fails, naming the unknown guest")
		})
	}
}

func TestProve_DevMockReadsNoGuest(t *testing.T) {
	c, err := New(Config{Mode: ProverModeDevMock, GuestsDir: "/does/not/exist"})
	require.NoError(t, err)
	res := c.Prove(context.Background(), Job{ID: "dev-mock", GuestProgramID: [32]byte{1}})
	assert.Equal(t, ResultStatusOK, res.Status, "dev-mock runs no guest, so it must not read the guests folder")
}
