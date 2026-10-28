package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/require"
)

// TestRunProve_SplitTraceRound checks that [New] puts the program columns (the
// decoded instruction table) in [zkcdriver.ProgramRound] and the rest of the
// trace in round 1, and that proving the R5 arithmetization verifies.
//
// As a Core is safe for concurrent use, it also proves two jobs with different
// traces concurrently on the same Core, and checks that each proof is the one
// of its own trace.
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

	cfg := Config{CircuitBinPath: binPath, GuestELFPath: elfPath}
	c, err := New(cfg)
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

	// The larger payload makes the trace of job B taller than the one of job A,
	// so the two proofs carry different dynamic module sizes.
	jobs := []Job{{}, {Payload: make([]byte, 1<<12)}}

	// References: the jobs proved one at a time, on a Core of their own.
	fresh, err := New(cfg)
	require.NoError(t, err)
	want := make([]wiop.Proof, len(jobs))
	for i, job := range jobs {
		want[i], err = proveJob(fresh, job)
		require.NoError(t, err)
	}
	require.NotEqual(t, want[0].DynamicSizes, want[1].DynamicSizes, "the two jobs must have different traces")

	// The jobs proved concurrently on the same Core, with the worst
	// interleaving forced: every proof hands its shard over in round 0, then
	// waits for all the others to do so before going on to the trace round. A
	// shard shared across proofs would then be the last one handed over for all
	// of them. Each proof must be the one of its own trace.
	var (
		assigned sync.WaitGroup
		done     sync.WaitGroup
		got      = make([]wiop.Proof, len(jobs))
		pubs     = make([]wiop.PublicInput, len(jobs))
	)
	assigned.Add(len(jobs))
	for i, job := range jobs {
		inputs, err := c.buildInputs(job)
		require.NoError(t, err)
		shard, errs := c.driver.TraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs}).Get(0)
		require.Empty(t, errs)

		done.Add(1)
		go func() {
			defer done.Done()
			got[i], pubs[i] = c.sys.Prove(func(rt *wiop.Runtime) {
				c.driver.AssignTraceShard(rt, shard.Unwrap(), field.Octuplet{})
				assigned.Done()
				assigned.Wait()
			})
		}()
	}
	done.Wait()

	for i := range jobs {
		require.NoError(t, c.sys.Verify(got[i], pubs[i]), "job %d", i)
		require.Equal(t, want[i].DynamicSizes, got[i].DynamicSizes,
			"job %d must be proved from its own trace", i)
	}
}

// proveJob proves and verifies job on c.
func proveJob(c *Core, job Job) (wiop.Proof, error) {
	inputs, err := c.buildInputs(job)
	if err != nil {
		return wiop.Proof{}, err
	}
	proof, _, err := c.runProve(context.Background(), &zkcdriver.PreReadInputs{Inputs: inputs})
	return proof, err
}
