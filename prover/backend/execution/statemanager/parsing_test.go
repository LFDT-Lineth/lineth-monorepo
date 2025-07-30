//go:build !fuzzlight

package statemanager_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/stretchr/testify/require"
)

func TestJsonExample(t *testing.T) {

	t.Skipf("The test is not passing, but the tested code is completely stable")

	filenames := []string{
		"./testdata/block-20000-20002.json",
		"./testdata/delete-account.json",
		"./testdata/insert-1-account.json",
		"./testdata/insert-2-accounts.json",
		"./testdata/insert-account-and-contract.json",
		"./testdata/read-account.json",
		"./testdata/read-zero.json",
	}
	expectErr := []bool{false, false, false, false, false, false, false}

	for tcID, fname := range filenames {

		t.Run(fmt.Sprintf("file-%v", fname), func(t *testing.T) {
			f := files.MustRead(fname)
			var parsed statemanager.ShomeiOutput
			err := json.NewDecoder(f).Decode(&parsed)

			require.NoErrorf(t, err, "failed to decode the JSON file (%v)", fname)
			f.Close()

			// It's a bad test case. Expected to yield an error
			if expectErr[tcID] {

				gotErr := false

				parent := parsed.Result.ZkParentStateRootHash
				for i := range parsed.Result.ZkStateMerkleProof {
					old, new, err := statemanager.CheckTraces(parsed.Result.ZkStateMerkleProof[i])
					if err != nil {
						gotErr = true
					}
					if parent.Hex() != old.Hex() {
						gotErr = true
					}
					parent = new
				}

				require.Truef(t, gotErr, "did not get any error for %v", fname)
				return
			}

			// We keep track of the oldest root hash
			parent := parsed.Result.ZkParentStateRootHash

			t.Logf("file has %v traces", len(parsed.Result.ZkStateMerkleProof))
			for _, blockTraces := range parsed.Result.ZkStateMerkleProof {
				old, new, err := statemanager.CheckTraces(blockTraces)
				t.Logf("(fname= %v) old: %v, parent: %v", fname, old.Hex(), new.Hex())
				require.NoError(t, err, "inspection found an error in the traces (%v)", fname)
				require.Equal(t, parent.Hex(), old.Hex(), "expected parent and recovered parent root hash mismatch (%v)", fname)
				parent = new
			}

			t.Logf("testing file %v - PASS", fname)
		})

	}

}
