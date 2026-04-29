package invalidity_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	backend "github.com/consensys/linea-monorepo/prover/backend/invalidity"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeAccountTrieInputs_FromRequestFile(t *testing.T) {
	fileBytes, err := os.ReadFile("testdata/5-1-getZkInvalidityProof-v2.json")
	require.NoError(t, err)

	var req backend.Request
	err = json.Unmarshal(fileBytes, &req)
	require.NoError(t, err)

	tx, err := ethereum.RlpDecodeWithSignature(req.RlpEncodedTx)
	require.NoError(t, err)

	fromAddress := ethereum.GetFrom(tx)

	require.NotNil(t, req.AccountMerkleProof)
	inputs, addr, topRoot, err := backend.DecodeAccountTrieInputs(*req.AccountMerkleProof)
	require.NoError(t, err)

	t.Logf("fromAddress (from RLP)  = %s", fromAddress.Hex())
	t.Logf("subRoot (inputs.Root)  = %s", types.KoalaOctuplet(inputs.SubRoot).Hex())
	t.Logf("topRoot (computed)     = %s", topRoot.Hex())
	t.Logf("zkParentStateRootHash  = %s", req.ZkParentStateRootHash.Hex())
	t.Logf("subRoot == zkParent?   %v", types.KoalaOctuplet(inputs.SubRoot) == req.ZkParentStateRootHash)

	assert.Equal(t, int64(0), inputs.Account.Nonce, "non-existing account nonce should be 0")
	assert.Equal(t, "0", inputs.Account.Balance.String(), "non-existing account balance should be 0")

	assert.Equal(t, smt_koalabear.DefaultDepth, len(inputs.ProofMinus.Proof.Siblings),
		"minus proof should have %d siblings", smt_koalabear.DefaultDepth)

	assert.Equal(t, smt_koalabear.DefaultDepth, len(inputs.ProofPlus.Proof.Siblings),
		"plus proof should have %d siblings", smt_koalabear.DefaultDepth)

	assert.Equal(t, field.Octuplet(inputs.ProofMinus.LeafOpening.Hash()), inputs.ProofMinus.Leaf,
		"minus leaf hash should match Hash(LeafOpening)")
	assert.Equal(t, field.Octuplet(inputs.ProofPlus.LeafOpening.Hash()), inputs.ProofPlus.Leaf,
		"plus leaf hash should match Hash(LeafOpening)")

	recoveredMinus, err := smt_koalabear.RecoverRoot(&inputs.ProofMinus.Proof, inputs.ProofMinus.Leaf)
	require.NoError(t, err)
	assert.Equal(t, field.Octuplet(inputs.SubRoot), recoveredMinus,
		"minus proof should recover the subRoot")

	recoveredPlus, err := smt_koalabear.RecoverRoot(&inputs.ProofPlus.Proof, inputs.ProofPlus.Leaf)
	require.NoError(t, err)
	assert.Equal(t, field.Octuplet(inputs.SubRoot), recoveredPlus,
		"plus proof should recover the subRoot")

	hKey := backend.HashAddress(addr)
	assert.Equal(t, -1, inputs.ProofMinus.LeafOpening.HKey.Cmp(hKey),
		"hKey(minus) should be less than Hash(address)")
	assert.Equal(t, -1, hKey.Cmp(inputs.ProofPlus.LeafOpening.HKey),
		"Hash(address) should be less than hKey(plus)")

	assert.Equal(t, req.ZkParentStateRootHash, topRoot,
		"topRoot should match ZkParentStateRootHash from request")

	assert.Equal(t, fromAddress, addr,
		"fromAddress should be the same as the key")

}

func TestDecodeAndRecoverRoot_ExistingAccount(t *testing.T) {
	fileBytes, err := os.ReadFile("testdata/proof.json")
	require.NoError(t, err)

	var response struct {
		AccountProof backend.ShomeiAccountProof `json:"accountProof"`
	}
	err = json.Unmarshal(fileBytes, &response)
	require.NoError(t, err)

	inputs, addr, _, err := backend.DecodeAccountTrieInputs(response.AccountProof)
	require.NoError(t, err)
	require.True(t, inputs.AccountExists)

	hKey := backend.HashAddress(addr)
	assert.Equal(t, hKey, inputs.ProofMinus.LeafOpening.HKey,
		"hKey in leaf opening should equal Hash(address)")

	assert.Equal(t, field.Octuplet(inputs.ProofMinus.LeafOpening.Hash()), inputs.ProofMinus.Leaf,
		"leaf hash should match Hash(LeafOpening)")

	recoveredRoot, err := smt_koalabear.RecoverRoot(&inputs.ProofMinus.Proof, inputs.ProofMinus.Leaf)
	require.NoError(t, err)
	assert.Equal(t, field.Octuplet(inputs.SubRoot), recoveredRoot,
		"recovered root should match subRoot from JSON")

	t.Logf("Root match: %x", inputs.SubRoot)
}
