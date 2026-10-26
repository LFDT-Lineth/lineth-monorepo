package backend

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProgramVK checks that the program VK is the commitment to the program
// round, is bound to the program only, and cannot be forged.
func TestProgramVK(t *testing.T) {
	binPath := embeddedZKCBin(t)

	// newCore returns a [Core] proving elf over the embedded arithmetization.
	newCore := func(t *testing.T, elf []byte) *Core {
		elfPath := filepath.Join(t.TempDir(), "guest.elf")
		require.NoError(t, os.WriteFile(elfPath, elf, 0o600))
		c, err := New(Config{CircuitBinPath: binPath, GuestELFPath: elfPath})
		require.NoError(t, err)
		return c
	}

	elf, _ := minimal_elf.AllInOneElfProgram()
	c := newCore(t, elf)

	proof, pub, err := proveJob(c, Job{})
	require.NoError(t, err)
	vk := risc5.GetProgramVK(c.sys, pub)
	assert.Equal(t, proof.Commitments[zkcdriver.ProgramRound], vk)
	assert.NotEqual(t, field.Octuplet{}, vk, "the program round must be committed")

	t.Run("same program", func(t *testing.T) {
		other, otherPub, err := proveJob(c, Job{Payload: make([]byte, 1<<12)})
		require.NoError(t, err)
		require.NotEqual(t, proof.Commitments[1], other.Commitments[1], "the two executions must differ")
		assert.Equal(t, vk, risc5.GetProgramVK(c.sys, otherPub))
	})

	t.Run("other program", func(t *testing.T) {
		otherElf, _ := minimal_elf.OutputELF(risc5.NumGuestPublicOutputs)
		other := newCore(t, otherElf)
		_, otherPub, err := proveJob(other, Job{})
		require.NoError(t, err)
		assert.NotEqual(t, vk, risc5.GetProgramVK(other.sys, otherPub))
	})

	t.Run("forged", func(t *testing.T) {
		_, pos := c.sys.LookupPublicInputByTag(risc5.ProgramVKPI, 0)
		forged := append(wiop.PublicInput(nil), pub...)
		limb := forged[pos].AsBase()
		limb.SetUint64(limb.Uint64() ^ 1)
		forged[pos] = field.ElemFromBase(limb)
		assert.ErrorContains(t, c.sys.Verify(proof, forged), "ProgramVKChecker")
	})
}

// TestProgramVKMisuse checks that the program VK registration and the
// public-input check reject an ill-defined system.
func TestProgramVKMisuse(t *testing.T) {
	binBytes, err := os.ReadFile(embeddedZKCBin(t))
	require.NoError(t, err)

	newSys := func(settings zkcdriver.Settings) *wiop.System {
		sys := wiop.NewSystemf("program-vk-test")
		sys.NewRound()
		zkcdriver.NewZkCDriver(sys, settings, bytes.NewReader(binBytes))
		return sys
	}

	t.Run("trace in program round", func(t *testing.T) {
		sys := newSys(zkcdriver.Settings{})
		assert.Panics(t, func() { risc5.RegisterProgramVK(sys) })
	})

	// CheckPublicInputs catches a system on which RegisterPublicInputs was not
	// called.
	t.Run("missing registration", func(t *testing.T) {
		sys := newSys(zkcdriver.Settings{TraceRound: 1})
		require.NoError(t, compilers.CompileFull(sys,
			compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
		))
		assert.Panics(t, func() { risc5.CheckPublicInputs(sys) })
	})
}
