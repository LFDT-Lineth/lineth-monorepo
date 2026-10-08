package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/require"
)

// TestRunProve_SplitTraceRound checks that [New] puts the program columns (the
// decoded instruction table) in [zkcdriver.ProgramRound] and the rest of the
// trace in round 1, and that proving the R5 arithmetization verifies. It proves
// twice on the same Core, as the system is reused across jobs.
func TestRunProve_SplitTraceRound(t *testing.T) {
	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	binBytes, err := binF.MarshalBinary()
	require.NoError(t, err)
	binPath := filepath.Join(t.TempDir(), "circuit.bin")
	require.NoError(t, os.WriteFile(binPath, binBytes, 0o600))

	elf, _ := minimal_elf.AllInOneElfProgram()
	elfPath := filepath.Join(t.TempDir(), "guest.elf")
	require.NoError(t, os.WriteFile(elfPath, elf, 0o600))

	c, err := New(Config{CircuitBinPath: binPath, GuestELFPath: elfPath})
	require.NoError(t, err)

	// Every committed column of the decoded module is in the program round, and
	// every other one in the trace round. Precomputed columns are in neither.
	var numProgram, numTrace int
	for _, mod := range c.sys.Modules {
		isProgram := strings.HasSuffix(mod.Context.Path(), "module-decoded")
		for _, col := range mod.Columns {
			if col.Round() == &c.sys.PrecomputedRound.Round {
				continue
			}
			if isProgram {
				require.Equal(t, zkcdriver.ProgramRound, col.Round().ID, "column %s", col.Context.Path())
				numProgram++
			} else {
				require.Equal(t, 1, col.Round().ID, "column %s", col.Context.Path())
				numTrace++
			}
		}
	}
	require.NotZero(t, numProgram, "the decoded module must have columns in the program round")
	require.NotZero(t, numTrace, "the trace must have columns in the trace round")

	inputs, err := c.buildInputs(Job{})
	require.NoError(t, err)

	for range 2 {
		_, _, err := c.runProve(context.Background(), &zkcdriver.PreReadInputs{Inputs: inputs})
		require.NoError(t, err)
	}
}
