package invalidity_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestAccountTrie tests the full AccountTrie circuit which verifies:
// 1. Hash(Account) == LeafOpening.HVal
// 2. Hash(LeafOpening) == MerkleProof.Leaf
// 3. MerkleProof is valid for the given root
func TestAccountTrie(t *testing.T) {
	const depth = 10

	// Create test accounts
	accounts := []types.Account{
		{
			Nonce:          1,
			Balance:        big.NewInt(1000),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
			KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		},
		{
			Nonce:          65,
			Balance:        big.NewInt(5690),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x1c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c39"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x7b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b2"),
			KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       100,
		},
	}

	// Create leaf openings with account hashes
	addresses := []common.Address{
		common.HexToAddress("0x00aed6"),
		common.HexToAddress("0x00aed7"),
	}

	leafOpenings := make([]accumulator.LeafOpening, len(accounts))
	leafHashes := make([]field.Octuplet, len(accounts))

	for i, account := range accounts {
		// Hash the account
		accountHash := hashAccount(&account)

		// Hash the address to get HKey
		hKey := hashAddress(addresses[i])

		// Create leaf opening
		leafOpenings[i] = accumulator.LeafOpening{
			Prev: int64(i),
			Next: int64(i + 1),
			HKey: hKey,
			HVal: accountHash,
		}

		// Hash the leaf opening
		leafHashes[i] = leafOpenings[i].Hash().ToOctuplet()
	}

	// Build the Merkle tree
	tree := smt_koalabear.NewEmptyTree(depth)
	for i, leafHash := range leafHashes {
		tree.Update(i, leafHash)
	}

	// Test each account
	for i := range accounts {
		t.Run(addresses[i].Hex(), func(t *testing.T) {
			// Get proof for this leaf
			proof, err := tree.Prove(i)
			require.NoError(t, err)

			// Verify proof natively first
			leaf, err := tree.GetLeaf(i)
			require.NoError(t, err)
			err = smt_koalabear.Verify(&proof, leaf, tree.Root)
			require.NoError(t, err, "native Merkle proof verification failed")

			// The circuit verifies all 3 Merkle proofs unconditionally.
			// For existing accounts, reuse the same valid proof for the
			// unused minus/plus slots.
			lo := invalidity.MerkleLeafProof{
				LeafOpening: leafOpenings[i],
				Leaf:        leafHashes[i],
				Proof:       proof,
			}

			inputs := invalidity.AccountTrieInputs{
				Account:       accounts[i],
				TargetHKey:    hashAddress(addresses[i]),
				ProofMinus:    lo,
				ProofPlus:     lo,
				SubRoot:       tree.Root,
				NextFreeNode:  3,
				TopRoot:       invalidity.ComputeTopRoot(3, tree.Root),
				AccountExists: true,
			}

			// Create and allocate circuit
			circuit := &invalidity.AccountTrie{}
			circuit.Allocate(invalidity.Config{Depth: depth})

			// Create and assign witness
			witness := &invalidity.AccountTrie{}
			witness.Assign(inputs)

			// Compile circuit
			cs, err := frontend.Compile(
				ecc.BLS12_377.ScalarField(),
				scs.NewBuilder,
				circuit,
			)
			require.NoError(t, err)

			// Create witness
			fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
			require.NoError(t, err, "witness creation failed")

			// Verify the circuit is satisfied
			err = cs.IsSolved(fullWitness)
			require.NoError(t, err, "circuit is not satisfied")

		})
	}
}

// hashAccount hashes an account using Poseidon2
func hashAccount(a *types.Account) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	a.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// hashAddress hashes an Ethereum address using Poseidon2
func hashAddress(addr common.Address) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	addrBytes := addr.Bytes()
	b := types.LeftPadded(addrBytes)
	hasher.Write(b)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// TestNonExistingAccount tests the AccountTrie circuit for the non-membership
// path: the target account does NOT exist in the trie and we prove it by
// showing two adjacent leaves (minus, plus) that sandwich the target's HKey.
func TestNonExistingAccount(t *testing.T) {
	const depth = 10

	// Find three addresses whose HKeys are ordered: hkey0 < hkey1 < hkey2.
	// We'll use addr0/addr2 as the existing leaves (minus/plus) and addr1 as
	// the non-existing target.
	type addrHKey struct {
		addr common.Address
		hkey types.KoalaOctuplet
	}
	candidates := make([]addrHKey, 20)
	for i := range candidates {
		candidates[i].addr = common.BigToAddress(big.NewInt(int64(i + 1)))
		candidates[i].hkey = hashAddress(candidates[i].addr)
	}
	// Sort by HKey lexicographically
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].hkey.Cmp(candidates[i].hkey) < 0 {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	// Pick three consecutive: minus (idx 0), target (idx 1), plus (idx 2)
	addrMinus := candidates[0].addr
	addrTarget := candidates[1].addr
	addrPlus := candidates[2].addr
	hkeyMinus := candidates[0].hkey
	hkeyTarget := candidates[1].hkey
	hkeyPlus := candidates[2].hkey

	t.Logf("minus  addr=%s hkey=%v", addrMinus.Hex(), hkeyMinus)
	t.Logf("target addr=%s hkey=%v", addrTarget.Hex(), hkeyTarget)
	t.Logf("plus   addr=%s hkey=%v", addrPlus.Hex(), hkeyPlus)

	require.True(t, hkeyMinus.Cmp(hkeyTarget) < 0, "minus.HKey must be < target.HKey")
	require.True(t, hkeyTarget.Cmp(hkeyPlus) < 0, "target.HKey must be < plus.HKey")

	// Create dummy account data for the two existing leaves. The exact values
	// don't matter; they just need a valid hash.
	dummyAccount := types.Account{
		Nonce:          1,
		Balance:        big.NewInt(100),
		StorageRoot:    types.MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
		LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
		KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
		CodeSize:       0,
	}
	dummyAccountHash := hashAccount(&dummyAccount)

	// The minus leaf is at tree index 0, plus at a large 32-bit index
	// to verify adjacency checks work when Next exceeds a single KoalaBear element.
	const plusIdx = 1 << 4
	loMinus := accumulator.LeafOpening{
		Prev: 0,
		Next: plusIdx,
		HKey: hkeyMinus,
		HVal: dummyAccountHash,
	}
	loPlus := accumulator.LeafOpening{
		Prev: 0,
		Next: 0,
		HKey: hkeyPlus,
		HVal: dummyAccountHash,
	}

	leafHashMinus := loMinus.Hash().ToOctuplet()
	leafHashPlus := loPlus.Hash().ToOctuplet()

	// Build Merkle tree with these two leaves
	tree := smt_koalabear.NewEmptyTree(depth)
	tree.Update(0, leafHashMinus)
	tree.Update(plusIdx, leafHashPlus)

	proofMinus, err := tree.Prove(0)
	require.NoError(t, err)
	proofPlus, err := tree.Prove(plusIdx)
	require.NoError(t, err)

	inputs := invalidity.AccountTrieInputs{
		Account:    dummyAccount,
		TargetHKey: hkeyTarget,
		ProofMinus: invalidity.MerkleLeafProof{
			LeafOpening: loMinus,
			Leaf:        leafHashMinus,
			Proof:       proofMinus,
		},
		ProofPlus: invalidity.MerkleLeafProof{
			LeafOpening: loPlus,
			Leaf:        leafHashPlus,
			Proof:       proofPlus,
		},
		SubRoot:       tree.Root,
		NextFreeNode:  3,
		TopRoot:       invalidity.ComputeTopRoot(3, tree.Root),
		AccountExists: false,
	}

	circuit := &invalidity.AccountTrie{}
	circuit.Allocate(invalidity.Config{Depth: depth})

	witness := &invalidity.AccountTrie{}
	witness.Assign(inputs)

	cs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		circuit,
	)
	require.NoError(t, err)

	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	require.NoError(t, err, "witness creation failed")

	err = cs.IsSolved(fullWitness)
	require.NoError(t, err, "circuit is not satisfied")
}
