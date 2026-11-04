package guests_test

import (
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/guests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// elf and elfIDHex are the golden vector of the release tool's own test
// (riscv-guests/build_common/program_id.zig): the Program ID is keccak256 of the
// ELF bytes.
const (
	elf      = "\x7fELFguest data"
	elfIDHex = "8e7dae55483e7b63d384e441238245d76cf07ab4dd89eaac76e00ce9e811a665"
)

func elfID(t *testing.T) [32]byte {
	t.Helper()
	b, err := hex.DecodeString(elfIDHex)
	require.NoError(t, err)
	return [32]byte(b)
}

func writeFile(t *testing.T, dir, name, data string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "rollup-v0.0.1-"+elfIDHex+".elf", elf)

	got, err := guests.Store{Dir: dir}.Load(elfID(t))
	require.NoError(t, err)
	assert.Equal(t, []byte(elf), got, "Load should return the bytes of the file named by the ID")
}

func TestLoad_SameIDTwoNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "l2-execution-v0.0.1-debug-"+elfIDHex+".elf", elf)
	writeFile(t, dir, "rollup-v0.0.1-"+elfIDHex+".elf", elf)

	got, err := guests.Store{Dir: dir}.Load(elfID(t))
	require.NoError(t, err)
	assert.Equal(t, []byte(elf), got, "either file has the same bytes")
}

func TestLoad_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "rollup-v0.0.1-"+elfIDHex+".elf", "other bytes")

	_, err := guests.Store{Dir: dir}.Load(elfID(t))
	assert.ErrorIs(t, err, guests.ErrHashMismatch, "a file whose bytes do not hash to its ID must be refused")
}

func TestLoad_UnknownGuest(t *testing.T) {
	// Each case puts one entry (none if entry is empty) in the folder; none of
	// them may match the ID.
	cases := []struct {
		name  string
		entry string
		isDir bool
	}{
		{name: "EmptyDir"},
		{name: "OtherID", entry: "rollup-v0.0.1-00" + elfIDHex[2:] + ".elf"},
		{name: "NoReleasePrefix", entry: elfIDHex + ".elf"},
		{name: "LongerHexBeforeID", entry: "rollup-v0.0.1-0" + elfIDHex + ".elf"},
		{name: "WrongExtension", entry: "rollup-v0.0.1-" + elfIDHex + ".elf.tmp"},
		{name: "Directory", entry: "rollup-v0.0.1-" + elfIDHex + ".elf", isDir: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			switch {
			case c.isDir:
				require.NoError(t, os.Mkdir(filepath.Join(dir, c.entry), 0o700))
			case c.entry != "":
				writeFile(t, dir, c.entry, elf)
			}

			_, err := guests.Store{Dir: dir}.Load(elfID(t))
			require.ErrorIs(t, err, guests.ErrUnknownGuest, "no file should match the ID")
			assert.Contains(t, err.Error(), "0x"+elfIDHex, "the error should name the requested ID")
		})
	}
}

func TestLoad_MissingDir(t *testing.T) {
	_, err := guests.Store{Dir: filepath.Join(t.TempDir(), "missing")}.Load(elfID(t))
	assert.ErrorIs(t, err, fs.ErrNotExist, "a missing guests folder is a config error, not an unknown guest")
}

func TestProgramID(t *testing.T) {
	assert.Equal(t, elfID(t), guests.ProgramID([]byte(elf)), "the Program ID is the keccak256 of the ELF bytes")
}
