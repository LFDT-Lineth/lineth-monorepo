package invalidity

import (
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
)

// MockAccountProof holds the result of CreateMockAccountProof.
type MockAccountProof struct {
	Inputs  invalidity.AccountTrieInputs
	Address types.EthAddress
	SubRoot types.KoalaOctuplet
}

// CreateMockAccountProof creates a mock account Merkle proof with a valid
// SMT structure. Returns AccountTrieInputs ready for circuit assignment,
// plus the address and SubRoot (for setting ZkParentStateRootHash).
// Optional depth parameter controls the tree depth (default: smt.DefaultDepth = 40).
func CreateMockAccountProof(address common.Address, account types.Account, depth ...int) MockAccountProof {
	hKey := HashAddress(types.EthAddress(address))
	hVal := HashAccount(account)

	leafOpening := accumulator.LeafOpening{
		Prev: 0,
		Next: 1,
		HKey: hKey,
		HVal: hVal,
	}

	leaf := leafOpening.Hash()

	tree := smt.NewEmptyTree(depth...)
	tree.Update(0, leaf)

	proof, err := tree.Prove(0)
	if err != nil {
		panic(fmt.Sprintf("failed to create Merkle proof: %v", err))
	}

	lo := invalidity.MerkleLeafProof{
		LeafOpening: leafOpening,
		Leaf:        field.Octuplet(leaf),
		Proof:       proof,
	}

	subRoot := types.KoalaOctuplet(tree.Root)
	var nextFreeNode int64 = 2
	topRoot := invalidity.ComputeTopRoot(nextFreeNode, subRoot)

	return MockAccountProof{
		Inputs: invalidity.AccountTrieInputs{
			Account:       account,
			TargetHKey:    hKey,
			ProofMinus:    lo,
			ProofPlus:     lo,
			SubRoot:       tree.Root,
			NextFreeNode:  nextFreeNode,
			TopRoot:       field.Octuplet(topRoot),
			AccountExists: true,
		},
		Address: types.EthAddress(address),
		SubRoot: subRoot,
	}
}

// CreateMockNonExistingAccountProof creates a mock non-membership Merkle proof
// for an address that does not exist in the trie. Uses Head (hKey=0) and
// Tail (hKey=Max) as the minus/plus neighbors so that any Hash(address) falls
// between them.
// Optional depth parameter controls the tree depth (default: smt.DefaultDepth = 40).
func CreateMockNonExistingAccountProof(address common.Address, depth ...int) MockAccountProof {
	loMinus := accumulator.Head() // position 0, hKey = 0
	loPlus := accumulator.Tail()  // position 1, hKey = Max

	leafMinus := loMinus.Hash()
	leafPlus := loPlus.Hash()

	tree := smt.NewEmptyTree(depth...)
	tree.Update(0, leafMinus)
	tree.Update(1, leafPlus)

	proofMinus, err := tree.Prove(0)
	if err != nil {
		panic(fmt.Sprintf("failed to create minus Merkle proof: %v", err))
	}
	proofPlus, err := tree.Prove(1)
	if err != nil {
		panic(fmt.Sprintf("failed to create plus Merkle proof: %v", err))
	}

	minus := invalidity.MerkleLeafProof{
		LeafOpening: loMinus,
		Leaf:        field.Octuplet(leafMinus),
		Proof:       proofMinus,
	}
	plus := invalidity.MerkleLeafProof{
		LeafOpening: loPlus,
		Leaf:        field.Octuplet(leafPlus),
		Proof:       proofPlus,
	}

	subRoot := types.KoalaOctuplet(tree.Root)
	var nextFreeNode int64 = 2
	topRoot := invalidity.ComputeTopRoot(nextFreeNode, subRoot)

	return MockAccountProof{
		Inputs: invalidity.AccountTrieInputs{
			Account:       types.Account{Balance: big.NewInt(0)},
			TargetHKey:    HashAddress(types.EthAddress(address)),
			ProofMinus:    minus,
			ProofPlus:     plus,
			SubRoot:       tree.Root,
			NextFreeNode:  nextFreeNode,
			TopRoot:       field.Octuplet(topRoot),
			AccountExists: false,
		},
		Address: types.EthAddress(address),
		SubRoot: subRoot,
	}
}

// CreateMockEOAAccount creates a mock EOA (Externally Owned Account) with given nonce and balance.
func CreateMockEOAAccount(nonce int64, balance *big.Int) types.Account {
	return types.Account{
		Nonce:          nonce,
		Balance:        balance,
		StorageRoot:    types.KoalaOctuplet{},
		LineaCodeHash:  types.KoalaOctuplet{},
		KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
		CodeSize:       0,
	}
}

// HashAddress computes Poseidon2 hash of an EthAddress.
func HashAddress(add types.EthAddress) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	add.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// HashAccount computes Poseidon2 hash of an account (used for HVal in leaf opening).
func HashAccount(a types.Account) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	a.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d types.KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}
