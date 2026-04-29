package invalidity_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/circuits/invalidity"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	. "github.com/consensys/linea-monorepo/prover/utils/types"
	linTypes "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-playground/assert/v2"
	"github.com/stretchr/testify/require"
)

// TestNonceFromRLP tests the extraction of nonce from RLP-encoded EIP-1559 transactions.
func TestNonceFromRLP(t *testing.T) {
	var (
		tx        = types.NewTx(&tcases[1].Tx)
		encodedTx = ethereum.EncodeTxForSigning(tx)
		a         byte
	)
	extractedNonce, _ := invalidity.ExtractNonceFromRLP(encodedTx)
	require.Equal(t, tx.Nonce(), extractedNonce, "extractedNonce and actualNonce are different")

	witness := circuitExtractNonceFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		Nonce:        tx.Nonce(),
	}
	for i := range encodedTx {
		a = encodedTx[i]
		witness.RlpEncodedtx[i] = a
	}

	circuit := circuitExtractNonceFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)

	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = ccs.IsSolved(twitness)
	require.NoError(t, err)
}

type circuitExtractNonceFromRLP struct {
	RlpEncodedtx []frontend.Variable
	Nonce        frontend.Variable
}

func (c circuitExtractNonceFromRLP) Define(api frontend.API) error {
	gnarkExtractedNonce := invalidity.ExtractNonceFromRLPZk(api, c.RlpEncodedtx)
	api.AssertIsEqual(gnarkExtractedNonce, c.Nonce)
	return nil
}

// TestTxCostFromRLP tests transaction cost extraction: cost = value + gasLimit * maxFeePerGas.
func TestTxCostFromRLP(t *testing.T) {
	var (
		tx        = types.NewTx(&tcases[1].Tx)
		encodedTx = ethereum.EncodeTxForSigning(tx)
		a         byte
	)

	expectedCost := tx.Value().Uint64() + tx.Gas()*tx.GasFeeCap().Uint64()

	extractedCost, err := invalidity.ExtractTxCostFromRLP(encodedTx)
	require.NoError(t, err)
	require.Equal(t, expectedCost, extractedCost, "extractedCost and expectedCost are different")

	witness := circuitExtractTxCostFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		Cost:         expectedCost,
	}
	for i := range encodedTx {
		a = encodedTx[i]
		witness.RlpEncodedtx[i] = a
	}

	circuit := circuitExtractTxCostFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)

	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = ccs.IsSolved(twitness)
	require.NoError(t, err)
}

type circuitExtractTxCostFromRLP struct {
	RlpEncodedtx []frontend.Variable
	Cost         frontend.Variable
}

func (c circuitExtractTxCostFromRLP) Define(api frontend.API) error {
	gnarkExtractedCost := invalidity.ExtractTxCostFromRLPZk(api, c.RlpEncodedtx)
	api.AssertIsEqual(gnarkExtractedCost, c.Cost)
	return nil
}

// TestToAddressFromRLP tests extraction of the "to" address (field index 5) from RLP-encoded EIP-1559 transactions.
func TestToAddressFromRLP(t *testing.T) {
	toAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD50")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(59144),
		Nonce:     42,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(100000000000),
		Gas:       500000,
		To:        &toAddr,
		Value:     big.NewInt(1000000000000000000),
	})

	encodedTx := ethereum.EncodeTxForSigning(tx)

	// Test native extraction
	extractedTo, err := invalidity.ExtractToAddressFromRLP(encodedTx)
	require.NoError(t, err)
	require.Equal(t, toAddr.Bytes(), extractedTo, "extracted to address mismatch")

	// Test ZK circuit
	expectedToInt := new(big.Int).SetBytes(toAddr.Bytes())

	witness := circuitExtractToAddressFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		ToAddress:    expectedToInt,
	}
	for i, b := range encodedTx {
		witness.RlpEncodedtx[i] = b
	}

	circuit := circuitExtractToAddressFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)

	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)

	err = ccs.IsSolved(twitness)
	require.NoError(t, err)
}

type circuitExtractToAddressFromRLP struct {
	RlpEncodedtx []frontend.Variable
	ToAddress    frontend.Variable
}

func (c circuitExtractToAddressFromRLP) Define(api frontend.API) error {
	gnarkExtractedTo := invalidity.ExtractToAddressFromRLPZk(api, c.RlpEncodedtx)
	api.AssertIsEqual(gnarkExtractedTo, c.ToAddress)
	return nil
}

// TestNonceFromRLPVariances covers nonce encoding variants:
// zero (0x80), single byte, boundary (128), multi-byte, multi-byte chainId.
func TestNonceFromRLPVariances(t *testing.T) {
	testCases := []struct {
		name    string
		nonce   uint64
		chainID *big.Int
		desc    string
	}{
		{"zero_nonce", 0, big.NewInt(1), "0x80 encoding"},
		{"single_byte_nonce", 127, big.NewInt(1), "direct byte"},
		{"boundary_nonce", 128, big.NewInt(1), "encoding transition"},
		{"multi_byte_nonce", 0xFFFFFFFF, big.NewInt(1), "byte reconstruction"},
		{"multi_byte_chainid", 42, big.NewInt(59144), "variable chainId offset"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   tc.chainID,
				Nonce:     tc.nonce,
				GasTipCap: big.NewInt(1),
				GasFeeCap: big.NewInt(1),
				Gas:       21000,
				Value:     big.NewInt(1000),
			})

			encodedTx := ethereum.EncodeTxForSigning(tx)

			extractedNonce, err := invalidity.ExtractNonceFromRLP(encodedTx)
			require.NoError(t, err, "failed to extract nonce: %s", tc.desc)
			require.Equal(t, tc.nonce, extractedNonce, "nonce mismatch for %s", tc.desc)

			witness := circuitExtractNonceFromRLP{
				RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
				Nonce:        tc.nonce,
			}
			for i, b := range encodedTx {
				witness.RlpEncodedtx[i] = b
			}

			circuit := circuitExtractNonceFromRLP{
				RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
			}

			ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
			require.NoError(t, err, "failed to compile circuit: %s", tc.desc)

			twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
			require.NoError(t, err, "failed to create witness: %s", tc.desc)

			err = ccs.IsSolved(twitness)
			require.NoError(t, err, "circuit not satisfied: %s", tc.desc)
		})
	}
}

// TestTxCostFromRLPVariances tests cost = value + gasLimit * maxFeePerGas.
// Covers: zero fields (0x80), typical values, large values.
func TestTxCostFromRLPVariances(t *testing.T) {
	testCases := []struct {
		name      string
		value     *big.Int
		gasFeeCap *big.Int
		gas       uint64
		desc      string
	}{
		{"zero_cost", big.NewInt(0), big.NewInt(0), 21000, "0x80 encoding"},
		{"typical_transfer", big.NewInt(1000000000000000000), big.NewInt(50000000000), 21000, "multi-byte fields"},
		{"large_values", big.NewInt(5000000000000000000), big.NewInt(500000000000), 1000000, "larger multi-byte"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   big.NewInt(1),
				Nonce:     1,
				GasTipCap: big.NewInt(1),
				GasFeeCap: tc.gasFeeCap,
				Gas:       tc.gas,
				Value:     tc.value,
			})

			encodedTx := ethereum.EncodeTxForSigning(tx)
			expectedCost := tc.value.Uint64() + tc.gas*tc.gasFeeCap.Uint64()

			extractedCost, err := invalidity.ExtractTxCostFromRLP(encodedTx)
			require.NoError(t, err, "failed to extract cost: %s", tc.desc)
			require.Equal(t, expectedCost, extractedCost, "cost mismatch for %s", tc.desc)

			witness := circuitExtractTxCostFromRLP{
				RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
				Cost:         expectedCost,
			}
			for i, b := range encodedTx {
				witness.RlpEncodedtx[i] = b
			}

			circuit := circuitExtractTxCostFromRLP{
				RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
			}

			ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
			require.NoError(t, err, "failed to compile circuit: %s", tc.desc)

			twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
			require.NoError(t, err, "failed to create witness: %s", tc.desc)

			err = ccs.IsSolved(twitness)
			require.NoError(t, err, "circuit not satisfied: %s", tc.desc)
		})
	}
}

// TestRLPLongListEncoding tests 0xf8+ prefix (most real txs exceed 55 bytes).
func TestRLPLongListEncoding(t *testing.T) {
	toAddr := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD50")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     999,
		GasTipCap: big.NewInt(1000000000),
		GasFeeCap: big.NewInt(100000000000),
		Gas:       500000,
		To:        &toAddr,
		Value:     big.NewInt(1000000000000000000),
		Data:      make([]byte, 50),
	})

	encodedTx := ethereum.EncodeTxForSigning(tx)
	require.GreaterOrEqual(t, encodedTx[1], byte(0xf8), "expected long RLP list encoding")

	// Test nonce extraction
	extractedNonce, err := invalidity.ExtractNonceFromRLP(encodedTx)
	require.NoError(t, err)
	require.Equal(t, uint64(999), extractedNonce)

	// Test cost extraction
	expectedCost := uint64(1000000000000000000) + 500000*uint64(100000000000)
	extractedCost, err := invalidity.ExtractTxCostFromRLP(encodedTx)
	require.NoError(t, err)
	require.Equal(t, expectedCost, extractedCost)

	// Test to address extraction
	extractedTo, err := invalidity.ExtractToAddressFromRLP(encodedTx)
	require.NoError(t, err)
	require.Equal(t, toAddr.Bytes(), extractedTo)

	// ZK circuit for nonce
	witnessNonce := circuitExtractNonceFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		Nonce:        uint64(999),
	}
	for i, b := range encodedTx {
		witnessNonce.RlpEncodedtx[i] = b
	}
	circuitNonce := circuitExtractNonceFromRLP{RlpEncodedtx: make([]frontend.Variable, len(encodedTx))}
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuitNonce, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)
	twitness, err := frontend.NewWitness(&witnessNonce, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)
	require.NoError(t, ccs.IsSolved(twitness))

	// ZK circuit for cost
	witnessCost := circuitExtractTxCostFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		Cost:         expectedCost,
	}
	for i, b := range encodedTx {
		witnessCost.RlpEncodedtx[i] = b
	}
	circuitCost := circuitExtractTxCostFromRLP{RlpEncodedtx: make([]frontend.Variable, len(encodedTx))}
	ccsCost, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuitCost, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)
	twitnessCost, err := frontend.NewWitness(&witnessCost, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)
	require.NoError(t, ccsCost.IsSolved(twitnessCost))

	// ZK circuit for to address
	expectedToInt := new(big.Int).SetBytes(toAddr.Bytes())
	witnessTo := circuitExtractToAddressFromRLP{
		RlpEncodedtx: make([]frontend.Variable, len(encodedTx)),
		ToAddress:    expectedToInt,
	}
	for i, b := range encodedTx {
		witnessTo.RlpEncodedtx[i] = b
	}
	circuitTo := circuitExtractToAddressFromRLP{RlpEncodedtx: make([]frontend.Variable, len(encodedTx))}
	ccsTo, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuitTo, frontend.IgnoreUnconstrainedInputs())
	require.NoError(t, err)
	twitnessTo, err := frontend.NewWitness(&witnessTo, ecc.BLS12_377.ScalarField())
	require.NoError(t, err)
	require.NoError(t, ccsTo.IsSolved(twitnessTo))
}

// generate a tree for testing (proofs, leafs, root) using smt_koalabear
func getMerkleProof(t *testing.T) (smt_koalabear.Proof, field.Octuplet, field.Octuplet) {

	depth := 10

	// Generate random field elements for leaves
	nbLeaves := 1 << depth
	leaves := make([]field.Octuplet, nbLeaves)
	for i := range leaves {
		leaves[i] = field.RandomOctuplet()
	}

	// And generate the tree using Poseidon2
	tree := smt_koalabear.NewTree(leaves)

	// Get leaf and proof at position 0
	leaf, _ := tree.GetLeaf(0)
	proof, _ := tree.Prove(0)

	// Directly verify the proof
	err := smt_koalabear.Verify(&proof, leaf, tree.Root)
	require.NoErrorf(t, err, "pos #%v, proof #%v", 0, proof)

	return proof, leaf, tree.Root
}

// test [invalidity.MerkleProofCircuit] with Poseidon2
func TestMerkleProofs(t *testing.T) {

	// generate witness using smt_koalabear
	proof, leaf, root := getMerkleProof(t)

	var witness circuitSmt

	// Assign siblings (each is a KoalagnarkOctuplet)
	witness.Proofs.Siblings = make([]poseidon2_koalabear.KoalagnarkOctuplet, len(proof.Siblings))
	for j := 0; j < len(proof.Siblings); j++ {
		for k := 0; k < 8; k++ {
			witness.Proofs.Siblings[j][k] = koalagnark.NewElementFromKoala(proof.Siblings[j][k])
		}
	}
	witness.Proofs.Path = proof.Path

	// Assign leaf and root (each is a KoalagnarkOctuplet)
	for k := 0; k < 8; k++ {
		witness.Leaf[k] = koalagnark.NewElementFromKoala(leaf[k])
		witness.Root[k] = koalagnark.NewElementFromKoala(root[k])
	}

	// compile circuit using KoalaBear field
	var circuit circuitSmt
	circuit.Proofs.Siblings = make([]poseidon2_koalabear.KoalagnarkOctuplet, len(proof.Siblings))

	ccs, err := frontend.CompileU32(koalabear.Modulus(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit with KoalaBear modulus
	twitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}
}

type circuitSmt smt.MerkleProofKoalagnark

func (c *circuitSmt) Define(api frontend.API) error {
	return smt.KoalagnarkVerifyMerkleProof(api, c.Proofs, c.Leaf, c.Root)
}

// it tests the Poseidon2 Hashing over [types.Account]
func TestHashAccount(t *testing.T) {

	var (
		// generate Poseidon2 witness for Hash(Account)
		a = tcases[1].Account
	)

	// Hash the account natively using Poseidon2
	nativeHasher := poseidon2_koalabear.NewMDHasher()
	a.WriteTo(nativeHasher)
	nativeHashBytes := nativeHasher.Sum(nil)

	var expectedHash linTypes.KoalaOctuplet
	err := expectedHash.SetBytes(nativeHashBytes)
	require.NoError(t, err)

	// Create the circuit for testing account hashing
	circuit := &testAccountHashCircuit{}

	// Create witness with assigned values
	witness := &testAccountHashCircuit{}
	witness.Account.Assign(a)
	for i, v := range field.Octuplet(expectedHash) {
		witness.ExpectedHash[i] = v.Uint64()
	}

	// Compile the circuit using KoalaBear field (for Poseidon2)
	ccs, err := frontend.CompileU32(
		koalabear.Modulus(),
		scs.NewBuilder,
		circuit,
	)
	require.NoError(t, err)

	wit, err := frontend.NewWitness(witness, koalabear.Modulus())
	require.NoError(t, err)

	err = ccs.IsSolved(wit)
	require.NoError(t, err)
}

// testAccountHashCircuit is a test circuit for verifying account hashing
type testAccountHashCircuit struct {
	Account      invalidity.GnarkAccount
	ExpectedHash poseidon2_koalabear.GnarkOctuplet
}

func (c *testAccountHashCircuit) Define(api frontend.API) error {
	// Create Poseidon2 hasher
	koalagnarkAPI := koalagnark.NewAPI(api)
	hasher := poseidon2_koalabear.NewKoalagnarkMDHasher(koalagnarkAPI)

	// Use GnarkAccount.Hash from account_trie.go
	hash := c.Account.Hash(hasher)

	// Assert hash matches expected
	for i := 0; i < 8; i++ {
		api.AssertIsEqual(hash[i].Native(), c.ExpectedHash[i])
	}

	return nil
}

// it creates a merkle tree for the given [accumulator.LeafOpening] using smt_koalabear
func genShomei(t *testing.T, tcases []TestCases, depth int) (*smt_koalabear.Tree, []smt_koalabear.Proof, []field.Octuplet) {

	var leaves = []field.Octuplet{}
	for _, c := range tcases {
		// Hash the account using Poseidon2
		accountHash := hashAccount(&c.Account)

		// Create leaf opening and hash it
		leafOpening := accumulator.LeafOpening{
			Prev: c.Leaf.Prev,
			Next: c.Leaf.Next,
			HKey: c.Leaf.HKey,
			HVal: accountHash,
		}
		leafHash := leafOpening.Hash()
		leaves = append(leaves, leafHash.ToOctuplet())
	}

	// Build the same tree by adding the leaves one by one
	tree := smt_koalabear.NewEmptyTree(depth)
	for i := range leaves {
		tree.Update(i, leaves[i])
	}

	var (
		leafs  = []field.Octuplet{}
		proofs = []smt_koalabear.Proof{}
	)
	// Make valid proofs
	for i := range leaves {

		leaf, _ := tree.GetLeaf(i)
		proof, _ := tree.Prove(i)

		// Directly verify the proof
		err := smt_koalabear.Verify(&proof, leaf, tree.Root)
		require.NoErrorf(t, err, "pos #%v, proof #%v", i, proof)

		leafs = append(leafs, leaf)
		proofs = append(proofs, proof)
	}

	return tree, proofs, leaves
}

// it gets a leaf via its position and check it has the expected value.
func TestShomei(t *testing.T) {

	depth := 10
	tree, _, leaves := genShomei(t, tcases, depth)

	for i := range leaves {
		leaf, _ := tree.GetLeaf(i)
		c := tcases[i]

		// Hash the account using Poseidon2
		accountHash := hashAccount(&c.Account)

		// Create expected leaf hash
		expectedLeafOpening := accumulator.LeafOpening{
			Prev: c.Leaf.Prev,
			Next: c.Leaf.Next,
			HKey: c.Leaf.HKey,
			HVal: accountHash,
		}
		expectedLeaf := expectedLeafOpening.Hash().ToOctuplet()

		assert.Equal(t, leaf, expectedLeaf)
	}
}

type TestCases struct {
	Account        Account
	Leaf           accumulator.LeafOpening
	Tx             types.DynamicFeeTx
	FromAddress    common.Address
	TxHash         common.Hash
	InvalidityType invalidity.InvalidityType
}

var toAddr = common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

var tcases = []TestCases{

	{
		Account: Account{
			Balance: big.NewInt(0),
		},
		Leaf: accumulator.LeafOpening{
			Prev: 0,
			Next: 1,
			HKey: hashAddress(common.HexToAddress("0x00aed6")),
			HVal: hashAccount(&Account{
				Balance: big.NewInt(0),
			}),
		},
		Tx: types.DynamicFeeTx{
			ChainID:   big.NewInt(59144), // Linea mainnet chain ID
			Nonce:     1,                 // valid nonce
			Value:     big.NewInt(1),     //invalid balance
			Gas:       1,
			GasFeeCap: big.NewInt(1), // gas price
			To:        &toAddr,
		},
		FromAddress:    common.HexToAddress("0x00aed6"),
		TxHash:         common.HexToHash("0x3f1d2e2b4c3f4e5d6c7b8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d"),
		InvalidityType: 1,
	},
	{
		// EOA
		Account: Account{
			Nonce:          65,
			Balance:        big.NewInt(5690),
			StorageRoot:    MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
			LineaCodeHash:  MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
			KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		},

		Leaf: accumulator.LeafOpening{
			Prev: 0,
			Next: 2,
			HKey: hashAddress(common.HexToAddress("0x00aed7")),
			HVal: hashAccount(&Account{
				Nonce:          65,
				Balance:        big.NewInt(5690),
				StorageRoot:    MustHexToKoalabearOctuplet("0x0b1dfeef3db4956540da8a5f785917ef1ba432e521368da60a0a1ce430425666"),
				LineaCodeHash:  MustHexToKoalabearOctuplet("0x729aac4455d43f2c69e53bb75f8430193332a4c32cafd9995312fa8346929e73"),
				KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			}),
		},
		Tx: types.DynamicFeeTx{
			ChainID:   big.NewInt(59144), // Linea mainnet chain ID
			Nonce:     65,                // invalid nonce
			Value:     big.NewInt(5700),  // invalid value
			Gas:       1,
			GasFeeCap: big.NewInt(1), // gas price
			To:        &toAddr,
		},
		FromAddress:    common.HexToAddress("0x00aed7"),
		TxHash:         common.HexToHash("0x4f1d2e2b4c3f4e5d6c7b8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9e"),
		InvalidityType: 0, //  0 = BadNonce, 1 = BadBalance
	},
	{
		// Another EOA
		Account: Account{
			Nonce:          65,
			Balance:        big.NewInt(835),
			StorageRoot:    MustHexToKoalabearOctuplet("0x1c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c39"),
			LineaCodeHash:  MustHexToKoalabearOctuplet("0x7b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b2"),
			KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		},
		Leaf: accumulator.LeafOpening{
			Prev: 1,
			Next: 3,
			HKey: hashAddress(common.HexToAddress("0x00aed8")),
			HVal: hashAccount(&Account{
				Nonce:          65,
				Balance:        big.NewInt(835),
				StorageRoot:    MustHexToKoalabearOctuplet("0x1c41acc261451aae253f621857172d6339919d18059f35921a50aafc69eb5c39"),
				LineaCodeHash:  MustHexToKoalabearOctuplet("0x7b688b215329825e5b00e4aa4e1857bc17afab503a87ecc063614b9b227106b2"),
				KeccakCodeHash: FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
				CodeSize:       0,
			}),
		},
		Tx: types.DynamicFeeTx{
			ChainID:   big.NewInt(59144), // Linea mainnet chain ID
			Nonce:     63,                // invalid nonce
			Value:     big.NewInt(800),   // valid value
			Gas:       1,
			GasFeeCap: big.NewInt(1), // gas price
			To:        &toAddr,
		},
		FromAddress:    common.HexToAddress("0x00aed8"),
		TxHash:         common.HexToHash("0x5f1d2e2b4c3f4e5d6c7b8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9f"),
		InvalidityType: 0,
	},
}
