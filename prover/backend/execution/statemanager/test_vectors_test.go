package statemanager_test

import (
	"io"
	"math/big"
	"testing"

	eth "github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/assert"
)

// Construct a dummy address from an integer
func DummyAddress(i int) (a eth.Address) {
	a[0] = byte(i)
	return a
}

// Constructs a dummy full bytes from an integer
func DummyFullByte(i int) (f eth.FullBytes32) {
	f[0] = byte(i)
	return f
}

// Returns a dummy digest
func DummyDigest(i int) (d eth.Digest) {
	return types.DummyKoalaOctuplet(i)
}

func Hash(t io.WriterTo) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	t.WriteTo(hasher)
	return types.MustBytesToKoalaOctuplet(hasher.Sum(nil))
}

/*
Hash of an account with zero in every fields
*/
func TestHashZeroAccountMiMC(t *testing.T) {
	// Test the hash of an emptyAccount with zeroes everywhere
	emptyAccount := types.Account{}
	emptyAccount.Balance = big.NewInt(0)

	assert.Equal(
		t,
		"0x0be39dd910329801041c54896705cb664779584732a232276e59ce2e7ca1b5a7",
		Hash(emptyAccount).Hex(),
	)
}

/*
- Test the empty storage trie hash
*/
func TestEmptyStorageTrieHash(t *testing.T) {
	assert.Equal(t, "0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7", eth.ZKHASH_EMPTY_STORAGE.Hex())
}

/*
- Root hash of the empty world state
*/
func TestEmptyWorldStateMiMC(t *testing.T) {
	worldstate := eth.NewWorldState()
	// should be the top root of an empty accumulator
	assert.Equal(t, "0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7", worldstate.AccountTrie.TopRoot().Hex())
}

/*
  - Insert EOA A
    -> check root hash
*/
func TestWorldStateWithAnAccountMiMC(t *testing.T) {

	account := eth.NewEOA(65, big.NewInt(835))
	// This gives a non-zero dummy address to the account
	address := DummyAddress(36)

	worldstate := eth.NewWorldState()
	worldstate.AccountTrie.InsertAndProve(address, account)

	// check that the hash of the inserted account matches
	assert.Equal(t, "0x495340db00ecc17b5cb435d5731f8d6635e6b3ef42507a8303a068d178a95d22", Hash(account).Hex(), "inserted EOA hash")
	// check that the hash of the address matches
	assert.Equal(t, "0x279326f6312018a0650c885c019a8ce35d9cb0a526599870628957d84a3655b3", Hash(address).Hex(), "hash of address")

	// finally check the root hash after insertion
	assert.Equal(t, "0x6b5d2e111a55e55826396df03ac3d0055dc4a88671edbcc8274db3f0117e0f97", worldstate.AccountTrie.TopRoot().Hex(), "root hash after insertion")
}

/*
  - Insert EOA A
  - Insert EOA B
    -> check root hash
*/
func TestWorldStateWithTwoAccountMiMC(t *testing.T) {

	accountA := eth.NewEOA(65, big.NewInt(835))
	// This gives a non-zero dummy address to the account
	addressA := DummyAddress(36)

	accountB := eth.NewEOA(42, big.NewInt(354))
	addressB := DummyAddress(41) // must be a different address or the insert will panic

	worldstate := eth.NewWorldState()
	worldstate.AccountTrie.InsertAndProve(addressA, accountA)
	worldstate.AccountTrie.InsertAndProve(addressB, accountB)

	// finally check the root hash after insertion
	assert.Equal(t, "0x7bcc50ea4546465f153f5c0a35e3cddb59f492f4183438ed52733bc92823fbb7", worldstate.AccountTrie.TopRoot().Hex())
}

/*
- Insert EOA A
- Insert contract B (empty storage) -> root hash
- Insert slot in storage of B -> root hash
- Delete A -> root hash
- Delete the slot in the storage of B -> root hash
- Rewrite again in the storage of B -> root hash
*/
func TestWorldStateEoaAndContractMiMC(t *testing.T) {

	// dummy EOA
	accountA := eth.NewEOA(65, big.NewInt(835))
	addressA := DummyAddress(36)

	// dummy Account
	accountB := eth.NewContractEmptyStorage(41, big.NewInt(15353), DummyDigest(75), DummyFullByte(15), 7)
	addressB := DummyAddress(47)

	worldstate := eth.NewWorldState()
	worldstate.AccountTrie.InsertAndProve(addressA, accountA)
	worldstate.AccountTrie.InsertAndProve(addressB, accountB)

	// Give a storage trie to B
	worldstate.StorageTries.InsertNew(addressB, eth.NewStorageTrie(addressB))

	// The storage tries map stores pointers to the storage tries so we don't need to
	// update the map after modifying the storage however, we still need to manually
	// set the new storage root hash on the account
	storageB := worldstate.StorageTries.MustGet(addressB)

	// finally check the root hash after insertion
	{
		// Check the root hash after inserting A and B
		assert.Equal(t, "0x14b74e4f2fa3edaf180c7a3c74417a5662ef843809717b014116863c256ee66d", worldstate.AccountTrie.TopRoot().Hex(), "after insertion of B")
	}

	// Write something in the storage of B
	{
		storageB.InsertAndProve(DummyFullByte(14), DummyFullByte(18))
		newAccountB := accountB
		newAccountB.StorageRoot = storageB.TopRoot()
		worldstate.AccountTrie.UpdateAndProve(addressB, newAccountB)

		// finally check the root hash after insertion
		assert.Equal(t, "0x480d862e3f395c2f1bb80a0504fa083701440727747a75562203eff125fbc57f", worldstate.AccountTrie.TopRoot().Hex(), "after inserting a slot in the storage of B")
	}

	// Delete the account A
	{
		worldstate.AccountTrie.DeleteAndProve(addressA)
		assert.Equal(t, "0x27aa6c982bd6ec5c52225e46410806a52377632e76a1715343e2e33d2b569de1", worldstate.AccountTrie.TopRoot().Hex())
	}

	// Remove what we wrote into B
	{
		storageB.DeleteAndProve(DummyFullByte(14))
		newAccountB := accountB
		newAccountB.StorageRoot = storageB.TopRoot()
		worldstate.AccountTrie.UpdateAndProve(addressB, newAccountB)
		assert.Equal(t, "0x0664e56b4fb256025fc4ebf6100ca8566a39394746b5f5b50e0c289b399aa19e", worldstate.AccountTrie.TopRoot().Hex())
	}

	// Write again, somewhere else
	{
		storageB.InsertAndProve(DummyFullByte(11), DummyFullByte(78))
		newAccountB := accountB
		newAccountB.StorageRoot = storageB.TopRoot()
		worldstate.AccountTrie.UpdateAndProve(addressB, newAccountB)
		assert.Equal(t, "0x532f7e8577d8f445505ecfbd053340cd007e742309d6fdf5286dcf0420463fe3", worldstate.AccountTrie.TopRoot().Hex())
	}
}

/*
insert EOA A
insert contract B
delete A
insert C
*/
func TestAddAaddBdelAaddCMiMC(t *testing.T) {

	// dummy EOA
	accountA := eth.NewEOA(65, big.NewInt(835))
	addressA := DummyAddress(36)

	// dummy Account
	accountB := eth.NewContractEmptyStorage(41, big.NewInt(15353), DummyDigest(75), DummyFullByte(15), 7)
	addressB := DummyAddress(47)

	// dummy Contract
	accountC := eth.NewContractEmptyStorage(48, big.NewInt(9835), DummyDigest(54), DummyFullByte(85), 19)
	addressC := DummyAddress(120)

	worldstate := eth.NewWorldState()
	worldstate.AccountTrie.InsertAndProve(addressA, accountA)
	worldstate.AccountTrie.InsertAndProve(addressB, accountB)
	worldstate.AccountTrie.DeleteAndProve(addressA)
	worldstate.AccountTrie.InsertAndProve(addressC, accountC)

	assert.Equal(t, "0x1d32f4e20bff17297318716a798a05a039dff3fa29fbc956320499370be459e9", worldstate.AccountTrie.TopRoot().Hex())
}
