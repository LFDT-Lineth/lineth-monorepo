// Package guests finds guest ELFs by the Program ID the coordinator sends in
// each request (guestProgramId).
package guests

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/sha3"
)

var (
	// ErrUnknownGuest means no file in the folder is named by the Program ID.
	ErrUnknownGuest = errors.New("unknown guest program")
	// ErrHashMismatch means a file is named by the Program ID but its bytes do
	// not hash to it.
	ErrHashMismatch = errors.New("guest ELF does not hash to its program ID")
)

// Store loads guest ELFs from Dir. Files use the release name
// <guest>-v<version>-<id>.elf (riscv-guests/build_common/program_id.zig), where
// id is the lowercase hex keccak256 of the ELF bytes.
type Store struct {
	Dir string
}

// Load returns the bytes of the ELF whose Program ID is id, after checking that
// they hash to id.
func (s Store) Load(id [32]byte) ([]byte, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("reading guests folder: %w", err)
	}
	idHex := hex.EncodeToString(id[:])
	suffix := "-" + idHex + ".elf"
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		path := filepath.Join(s.Dir, e.Name())
		elf, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading guest ELF %q: %w", path, err)
		}
		if ProgramID(elf) != id {
			return nil, fmt.Errorf("%w: %q", ErrHashMismatch, path)
		}
		return elf, nil
	}
	return nil, fmt.Errorf("%w 0x%s in %q", ErrUnknownGuest, idHex, s.Dir)
}

// ProgramID returns the Program ID of a guest ELF: the keccak256 of its bytes.
func ProgramID(elf []byte) [32]byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(elf)
	return [32]byte(h.Sum(nil))
}
