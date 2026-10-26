package backend

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
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

// newProgramVKCore returns a [Core] proving elf over the embedded arithmetization.
func newProgramVKCore(t *testing.T, binBytes, elf []byte) *Core {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "circuit.bin")
	require.NoError(t, os.WriteFile(binPath, binBytes, 0o600))
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elf, 0o600))

	c, err := New(Config{CircuitBinPath: binPath, GuestELFPath: elfPath})
	require.NoError(t, err)
	return c
}

// proveProgramVK proves and verifies job on c.
func proveProgramVK(t *testing.T, c *Core, job Job) (wiop.Proof, wiop.PublicInput) {
	t.Helper()
	inputs, err := c.buildInputs(job)
	require.NoError(t, err)
	proof, pub, err := c.runProve(context.Background(), &zkcdriver.PreReadInputs{Inputs: inputs})
	require.NoError(t, err)
	return proof, pub
}

func embeddedBinBytes(t *testing.T) []byte {
	t.Helper()
	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	binBytes, err := binF.MarshalBinary()
	require.NoError(t, err)
	return binBytes
}

// TestProgramVK checks that the program VK is the commitment to the program
// round, is bound to the program only, and cannot be forged.
func TestProgramVK(t *testing.T) {
	binBytes := embeddedBinBytes(t)
	elf, _ := minimal_elf.AllInOneElfProgram()
	c := newProgramVKCore(t, binBytes, elf)

	proof, pub := proveProgramVK(t, c, Job{})
	vk := risc5.GetProgramVK(c.sys, pub)
	assert.Equal(t, proof.Commitments[zkcdriver.ProgramRound], vk)
	assert.NotEqual(t, field.Octuplet{}, vk, "the program round must be committed")

	t.Run("same program", func(t *testing.T) {
		other, otherPub := proveProgramVK(t, c, Job{Payload: make([]byte, 1<<12)})
		require.NotEqual(t, proof.Commitments[1], other.Commitments[1], "the two executions must differ")
		assert.Equal(t, vk, risc5.GetProgramVK(c.sys, otherPub))
	})

	t.Run("other program", func(t *testing.T) {
		otherElf, _ := minimal_elf.OutputELF(risc5.NumGuestPublicOutputs)
		other := newProgramVKCore(t, binBytes, otherElf)
		_, otherPub := proveProgramVK(t, other, Job{})
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

// TestProgramVKRejectsTraceInProgramRound checks that the program VK is not
// registered when the trace is in the program round.
func TestProgramVKRejectsTraceInProgramRound(t *testing.T) {
	sys := wiop.NewSystemf("program-vk-test")
	sys.NewRound()
	zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(embeddedBinBytes(t)))

	assert.Panics(t, func() { risc5.RegisterProgramVK(sys) })
}

// TestCheckPublicInputsMissingRegistration checks that [risc5.CheckPublicInputs]
// catches a system on which [risc5.RegisterPublicInputs] was not called.
func TestCheckPublicInputsMissingRegistration(t *testing.T) {
	sys := wiop.NewSystemf("program-vk-test")
	sys.NewRound()
	zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{TraceRound: 1}, bytes.NewReader(embeddedBinBytes(t)))
	require.NoError(t, compilers.CompileFull(sys,
		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
	))

	assert.Panics(t, func() { risc5.CheckPublicInputs(sys) })
}
