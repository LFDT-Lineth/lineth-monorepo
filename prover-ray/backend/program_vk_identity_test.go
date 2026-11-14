package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProgramVKIdentity proves and verifies both guests of each
// minimal_elf same-code pair. They have different guest outputs but the same
// program VK, because the VK commits only to the decoded instruction table.
func TestProgramVKIdentity(t *testing.T) {
	binPath := embeddedZKCBin(t)
	pairs := map[string]func() (elfs, outputs [2][]byte){
		"entry point": minimal_elf.EntryPointPairELFs,
		"rodata":      minimal_elf.RodataPairELFs,
	}
	for name, pair := range pairs {
		t.Run(name, func(t *testing.T) {
			elfs, outputs := pair()
			var vks [2]field.Octuplet
			for i, elf := range elfs {
				elfPath := filepath.Join(t.TempDir(), "guest.elf")
				require.NoError(t, os.WriteFile(elfPath, elf, 0o600))
				c, err := New(Config{CircuitBinPath: binPath, GuestELFPath: elfPath})
				require.NoError(t, err)
				_, pub, err := proveJob(c, Job{}) // proves and verifies
				require.NoError(t, err)

				guestOutput := risc5.GetGuestPublicOutputs(c.sys, pub)
				output := make([]byte, 0, len(guestOutput))
				for _, b := range guestOutput {
					output = append(output, byte(b.Uint64()))
				}
				require.Equal(t, outputs[i], output)
				vks[i] = risc5.GetProgramVK(c.sys, pub)
				t.Logf("guest %d: output=%q vk=%v", i, output, vks[i])
			}
			assert.Equal(t, vks[0], vks[1], "different guests, same program VK")
		})
	}
}
