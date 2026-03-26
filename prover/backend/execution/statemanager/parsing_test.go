//go:build !fuzzlight

package statemanager_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/backend/files"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicTree(t *testing.T) {

	address, _ := types.AddressFromHex("0x35a5e43d3d3195b49cbfe78cd944115eaa2e09db")
	subTree := statemanager.NewStorageTrie(address)
	stoKey := types.FullBytes32FromHex("0x0000000000000000000000000000000000000000000000000000000000000000")
	stoValue := types.FullBytes32FromHex("0x0000000000000000000000001b9abeec3215d8ade8a33607f2cf0f4f60e5f0d0")
	subTree.InsertAndProve(stoKey, stoValue)

	root := subTree.TopRoot()
	assert.Equal(t, "0x1969de27292936c44a32eee03ec7e31d2823768c2abf7c5d6655886750ff9fb2", root.Hex())

}

func TestJsonExample(t *testing.T) {

	dirEntries, err := os.ReadDir("./testdata")
	if err != nil {
		t.Fatalf("could not read testdata dir, %v. Did it change location?", err)
	}

	filenames := []string{}
	for i := range dirEntries {
		filenames = append(filenames, "./testdata/"+dirEntries[i].Name())
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
				t.Logf("(fname= %v) old: %v, parent: %v new: %v", fname, old.Hex(), parent.Hex(), new.Hex())
				require.NoError(t, err, "inspection found an error in the traces (%v)", fname)
				require.Equal(t, parent.Hex(), old.Hex(), "expected parent and recovered parent root hash mismatch (%v)", fname)
				parent = new
			}

			t.Logf("testing file %v - PASS", fname)
		})

	}

}
