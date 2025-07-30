package accumulator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accumulatorTestingModule(maxNumProofs int) (
	define wizard.DefineFunc,
	prover func(traces []statemanager.DecodedTrace) wizard.MainProverStep,
) {

	var mod Module

	// The testing wizard uniquely calls the accumulator module
	define = func(b *wizard.Builder) {
		mod = NewModule(b.CompiledIOP, Settings{
			Name:            "ACCUMULATOR_TEST",
			MaxNumProofs:    maxNumProofs,
			MerkleTreeDepth: 40,
		})
	}

	// And the prover (instanciated for traces) is called
	prover = func(
		traces []statemanager.DecodedTrace,
	) wizard.MainProverStep {
		return func(run *wizard.ProverRuntime) {
			// Assigns the module
			mod.Assign(run, traces)
		}
	}

	return define, prover
}

func TestShomeiFiles(t *testing.T) {
	filenames := []string{
		"../../../../backend/execution/statemanager/testdata/block-20000-20002.json",
		"../../../../backend/execution/statemanager/testdata/delete-account.json",
		"../../../../backend/execution/statemanager/testdata/insert-1-account.json",
		"../../../../backend/execution/statemanager/testdata/insert-2-accounts.json",
		"../../../../backend/execution/statemanager/testdata/insert-account-and-contract.json",
		"../../../../backend/execution/statemanager/testdata/read-account.json",
		"../../../../backend/execution/statemanager/testdata/read-zero.json",
	}
	for _, fname := range filenames {
		t.Run(fmt.Sprintf("file-%v", fname), func(t *testing.T) {
			trace := []statemanager.DecodedTrace{}
			f := files.MustRead(fname)
			var parsed statemanager.ShomeiOutput
			err := json.NewDecoder(f).Decode(&parsed)

			require.NoErrorf(t, err, "failed to decode the JSON file (%v)", fname)
			f.Close()

			t.Logf("file has %v traces", len(parsed.Result.ZkStateMerkleProof))
			for _, blockTraces := range parsed.Result.ZkStateMerkleProof {
				trace = append(trace, blockTraces...)
			}
			definer, prover := accumulatorTestingModule(1024)
			comp := wizard.Compile(definer, dummy.Compile)
			proof := wizard.Prove(comp, prover(trace))
			assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid accumulator proof")
		})

	}
}
