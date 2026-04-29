package invalidity_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestInvalidity(t *testing.T) {

	const maxRlpByteSize = 1024
	var (
		depth      = 10
		tree, _, _ = genShomei(t, tcases, depth)
		root       = tree.Root
	)
	for pos := range tcases {
		var (
			proof, _ = tree.Prove(pos)
			// leaf, _  = tree.GetLeaf(pos)
			tcase = tcases[pos]
			leaf  = tcase.Leaf.Hash()

			lo = invalidity.MerkleLeafProof{
				Proof:       proof,
				Leaf:        leaf,
				LeafOpening: tcase.Leaf,
			}

			topRoot = invalidity.ComputeTopRoot(3, linTypes.KoalaOctuplet(root))

			assi = invalidity.AssigningInputs{
				AccountTrieInputs: invalidity.AccountTrieInputs{
					TargetHKey:    tcase.Leaf.HKey,
					ProofMinus:    lo,
					ProofPlus:     lo,
					SubRoot:       root,
					NextFreeNode:  3,
					TopRoot:       topRoot.ToOctuplet(),
					Account:       tcase.Account,
					AccountExists: true,
				},
				Transaction:    types.NewTx(&tcase.Tx),
				FromAddress:    tcase.FromAddress,
				MaxRlpByteSize: maxRlpByteSize,
				InvalidityType: tcase.InvalidityType,
			}

			b = ethereum.EncodeTxForSigning(assi.Transaction)
		)

		// RLP encode the transaction
		assi.RlpEncodedTx = make([]byte, len(b[:])) // include the type byte
		copy(assi.RlpEncodedTx, b[:])

		assi.FuncInputs = public_input.Invalidity{
			ToAddress:     linTypes.EthAddress(*assi.Transaction.To()),
			StateRootHash: topRoot,
			TxHash:        common.Hash(crypto.Keccak256(assi.RlpEncodedTx)),
			FromAddress:   linTypes.EthAddress(assi.FromAddress),
		}

		// generate keccak proof for the circuit
		kcomp, kproof := invalidity.MakeKeccakProofs(assi.Transaction, maxRlpByteSize, dummy.Compile)
		assi.KeccakCompiledIOP = kcomp
		assi.KeccakProof = kproof

		// define the circuit
		circuit := invalidity.CircuitInvalidity{
			SubCircuit: &invalidity.BadNonceBalanceCircuit{},
		}

		// allocate the circuit
		circuit.Allocate(invalidity.Config{
			KeccakCompiledIOP: kcomp,
			Depth:             depth,
			MaxRlpByteSize:    maxRlpByteSize,
		})

		// compile the circuit
		scs, err := frontend.Compile(
			ecc.BLS12_377.ScalarField(),
			scs.NewBuilder,
			&circuit,
		)
		require.NoError(t, err)

		// assign the circuit
		assignment := invalidity.CircuitInvalidity{
			SubCircuit: &invalidity.BadNonceBalanceCircuit{},
		}
		assignment.Assign(assi)

		witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
		require.NoError(t, err)

		err = scs.IsSolved(witness)
		require.NoError(t, err)
	}

}

// TestInvalidityNonExistingAccount tests BadBalance for an account that does
// not exist in the state trie. The circuit proves non-membership via two
// adjacent leaves (minus, plus) whose HKeys sandwich the target's HKey.
func TestInvalidityNonExistingAccount(t *testing.T) {
	const (
		maxRlpByteSize = 1024
		depth          = 10
	)

	// Find three addresses whose HKeys are lexicographically ordered.
	// minus and plus are existing leaves; target is the non-existing account.
	type addrHKey struct {
		addr common.Address
		hkey linTypes.KoalaOctuplet
	}
	candidates := make([]addrHKey, 20)
	for i := range candidates {
		candidates[i].addr = common.BigToAddress(big.NewInt(int64(i + 1)))
		candidates[i].hkey = hashAddress(candidates[i].addr)
	}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].hkey.Cmp(candidates[i].hkey) < 0 {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	addrTarget := candidates[1].addr
	hkeyMinus := candidates[0].hkey
	hkeyTarget := candidates[1].hkey
	hkeyPlus := candidates[2].hkey

	require.True(t, hkeyMinus.Cmp(hkeyTarget) < 0, "minus.HKey must be < target.HKey")
	require.True(t, hkeyTarget.Cmp(hkeyPlus) < 0, "target.HKey must be < plus.HKey")

	// Dummy accounts for the two existing leaves
	dummyAccount := linTypes.Account{
		Nonce:          10,
		Balance:        big.NewInt(5000),
		StorageRoot:    linTypes.MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
		LineaCodeHash:  linTypes.MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
		KeccakCodeHash: linTypes.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
	}
	dummyAccountHash := hashAccount(&dummyAccount)

	// Minus leaf at index 0, plus leaf at index 1
	loMinus := accumulator.LeafOpening{
		Prev: 0,
		Next: 1,
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

	tree := smt_koalabear.NewEmptyTree(depth)
	tree.Update(0, leafHashMinus)
	tree.Update(1, leafHashPlus)

	proofMinus, err := tree.Prove(0)
	require.NoError(t, err)
	proofPlus, err := tree.Prove(1)
	require.NoError(t, err)

	// Non-existing target account: zero balance, zero nonce
	targetAccount := linTypes.Account{
		Balance: big.NewInt(0),
	}

	// Transaction from the non-existing address.
	// Any tx with cost > 0 triggers BadBalance since balance is 0.
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(59144),
		Nonce:     42,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(1),
		Gas:       21000,
		To:        &toAddr,
		Value:     big.NewInt(1),
	})

	assi := invalidity.AssigningInputs{
		AccountTrieInputs: invalidity.AccountTrieInputs{
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
			TopRoot:       invalidity.ComputeTopRoot(3, linTypes.KoalaOctuplet(tree.Root)).ToOctuplet(),
			Account:       targetAccount,
			AccountExists: false,
		},
		Transaction:    tx,
		FromAddress:    addrTarget,
		MaxRlpByteSize: maxRlpByteSize,
		InvalidityType: invalidity.BadBalance,
	}

	b := ethereum.EncodeTxForSigning(assi.Transaction)
	assi.RlpEncodedTx = make([]byte, len(b))
	copy(assi.RlpEncodedTx, b)

	assi.FuncInputs = public_input.Invalidity{
		ToAddress:     linTypes.EthAddress(*assi.Transaction.To()),
		StateRootHash: invalidity.ComputeTopRoot(3, tree.Root),
		TxHash:        common.Hash(crypto.Keccak256(assi.RlpEncodedTx)),
		FromAddress:   linTypes.EthAddress(assi.FromAddress),
	}

	kcomp, kproof := invalidity.MakeKeccakProofs(assi.Transaction, maxRlpByteSize, dummy.Compile)
	assi.KeccakCompiledIOP = kcomp
	assi.KeccakProof = kproof

	circuit := invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.BadNonceBalanceCircuit{},
	}
	circuit.Allocate(invalidity.Config{
		KeccakCompiledIOP: kcomp,
		Depth:             depth,
		MaxRlpByteSize:    maxRlpByteSize,
	})

	cs, err := frontend.Compile(
		ecc.BLS12_377.ScalarField(),
		scs.NewBuilder,
		&circuit,
	)
	require.NoError(t, err)

	assignment := invalidity.CircuitInvalidity{
		SubCircuit: &invalidity.BadNonceBalanceCircuit{},
	}
	assignment.Assign(assi)

	witness, err := frontend.NewWitness(&assignment, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = cs.IsSolved(witness)
	require.NoError(t, err)
}
