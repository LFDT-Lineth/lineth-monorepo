package testcase_gen

import (
	"bytes"
	"fmt"
	"math/big"
	"math/rand/v2"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Random number generator
type RandGen struct {
	rand.Rand
	Params struct {
		SupTxPerBlock         int
		SupL2L1LogsPerBlock   int
		SupMsgReceiptPerBlock int
	}
}

// Create a generator from the CLI
func MakeGeneratorFromCLI() (res RandGen) {
	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	res = RandGen{Rand: *rand.New(utils.NewRandSource(Seed()))}
	res.Params.SupTxPerBlock = MaxTxPerBlock() + 1
	res.Params.SupL2L1LogsPerBlock = MaxL2L1LogsPerBlock() + 1
	res.Params.SupMsgReceiptPerBlock = MaxL1L2ReceiptPerBlock() + 1
	return res
}

// Returns an non-zero integer in the range
func (g *RandGen) PositiveInt(sup int) int {
	return utils.Max(1, g.IntN(sup))
}

// Returns a random hex string representing n bytes
func (g *RandGen) HexStringForNBytes(numBytes int) string {
	return hexutil.Encode(g.Bytes(numBytes))
}

// Returns an integer in the range
func (g *RandGen) IntRange(start, stop int) int {
	return start + g.PositiveInt(stop-start)
}

// Returns an hex string with a varying length in [start, stop)
func (g *RandGen) HexStringForVaryingNBytes(min, sup int) string {
	return g.HexStringForNBytes(g.IntRange(min, sup))
}

// Create t transactions RLP. Returns a uint16 indicating the position of the
// batch L1 Msg reception transactions.
func (g *RandGen) TxRlp(numTxs int) ([]string, []uint16) {

	// Random transaction RLP
	rlpTxs := make([]string, numTxs)
	for i := range rlpTxs {
		rlpTxs[i] = g.AnyTypeTxRlp()
	}

	var receptionPos []uint16

	// overwrite one of the tx with a receipt confirmation one
	txPos := g.IntN(numTxs)
	rlpTxs[txPos] = g.MsgReceiptConfirmationTx()
	receptionPos = append(receptionPos, utils.ToUint16(txPos))

	return rlpTxs, receptionPos
}

// Returns a list of addresses with the given size
func (g *RandGen) FromAddresses(numTxs int) (fromAddresses []types.EthAddress) {
	fromAddresses = make([]types.EthAddress, numTxs)
	for i := range fromAddresses {
		fromAddresses[i] = types.EthAddress(*g.Address())
	}
	return fromAddresses
}

// Create the Log hashes
func (g *RandGen) ListOfBytes32(size int) []string {
	res := make([]string, size)
	for i := range res {
		res[i] = g.HexStringForNBytes(32)
	}
	return res
}

// Populate the json with random data
func (g *RandGen) PopulateCoordOutput(p *execution.Response) {

	// The working fields for the coordination
	p.Version = "0.0.1"
	p.ProverMode = config.ProverModeFull

	// Random values for the initial root hashes
	p.ParentStateRootHash = StartFromRootHash()
	prevTimeStamps := StartFromTimeStamp()

	numBlocks := NumBlock()
	blockInputs := make([]execution.BlockData, numBlocks)

	for i := range blockInputs {
		prevTimeStamps = g.PopulateBlockData(&blockInputs[i], prevTimeStamps)
	}

	if len(EndWithRootHash()) == 0 {
		blockInputs[numBlocks-1].RootHash = EndWithRootHash()
	}

	p.FirstBlockNumber = StartFromBlock()
	p.BlocksData = blockInputs
}

// Populate the json with random functional inputs
func (g *RandGen) PopulateBlockData(
	pbi *execution.BlockData,
	prevTimeStamp uint64,
) (nextTimeStamp uint64) {
	pbi.L2ToL1MsgHashes = g.L2L1MsgHashes()
	pbi.RlpEncodedTransactions, pbi.BatchReceptionIndices = g.TxRlp(g.Params.SupTxPerBlock)
	pbi.FromAddresses = g.FromAddresses(g.Params.SupTxPerBlock)
	pbi.RootHash = types.BytesToKoalaOctupletLoose(g.Bytes(32))
	pbi.TimeStamp = prevTimeStamp + uint64(g.PositiveInt(24))
	return pbi.TimeStamp
}

func (g *RandGen) Bytes(nb int) []byte {
	res := make([]byte, nb)
	utils.ReadPseudoRand(&g.Rand, res)
	return res
}

// Generates a tx of any type
func (g *RandGen) AnyTypeTxRlp() (res string) {
	switch g.IntN(3) {
	case 0:
		res = g.LegacyTxRLP()
	case 1:
		res = g.EIP2930Tx()
	case 2:
		res = g.DynFeeTx()
	default:
		panic("unreachable")
	}
	return res
}

// Generate a legacy tx
func (g *RandGen) LegacyTxRLP() string {
	t0 := common.BytesToAddress(g.Bytes(20))
	tx := ethtypes.LegacyTx{
		Nonce:    g.Nonce(),
		GasPrice: g.BigInt(1_000_000),
		Gas:      g.Uint64() % 100_000,
		To:       &t0,
		Value:    g.Value(),
		Data:     g.CallData(),
	}
	encoded := ethereum.EncodeTxForSigning(ethtypes.NewTx(&tx))
	return hexutil.Encode(encoded)
}

// Generate an access list tx
func (g *RandGen) EIP2930Tx() string {
	tx := ethtypes.AccessListTx{
		ChainID:  ChainID(),
		Nonce:    g.Nonce(),
		GasPrice: g.BigInt(1_000_000),
		Gas:      g.Gas(),
		To:       g.Address(),
		Value:    g.Value(),
		Data:     g.CallData(),
	}
	encoded := ethereum.EncodeTxForSigning(ethtypes.NewTx(&tx))
	return hexutil.Encode(encoded)
}

// Generates a dynamic fee tx
func (g *RandGen) DynFeeTx() string {
	tx := ethtypes.DynamicFeeTx{
		ChainID:   ChainID(),
		Nonce:     g.Nonce(),
		GasTipCap: g.BigInt(1_000_000),
		GasFeeCap: g.BigInt(1_000_000),
		Gas:       g.Gas(),
		To:        g.Address(),
		Value:     g.Value(),
		Data:      g.CallData(),
	}
	encoded := ethereum.EncodeTxForSigning(ethtypes.NewTx(&tx))
	return hexutil.Encode(encoded)
}

// Generates an L1 messages batch receipt
func (g *RandGen) MsgReceiptConfirmationTx() string {

	nbMsg := g.Params.SupMsgReceiptPerBlock

	/*
		Sample produced by the function

		0x
			f4b476e1
			0000000000000000000000000000000000000000000000000000000000000020
			0000000000000000000000000000000000000000000000000000000000000011
			38f9aae2816d3e4507593de238f25e9de7d018f67b1ea0e0af37efe14a439b46
			925e9aef99376d2127f209c883298c62dcc3d3f7a0e4e9211a3af74a5b7b88c0
			244c3612e8d34418962b3e29baf17fad750a4d2cf71ae728b234ba127dd774a0
			5a40132d4b220ee9d77f8791d83bcef052832a398a3eee5f8c771df9e83af171
			7e86106bee7221195be6dd5855ae3cde82b79ff5e0255276fc9c25185c0575c4
			3299e9fa19bec28aff106d0616abf71633d44ea78c29eb9fc2f90dfd2482339d
			24a6aa726fe4cf33c3af6ebb0f638a62e29310f4c9dedfbac231eff050f61134
			7e2ade7e3b26de4fd3db1fb6992e8050a4968ac29e1cfee6584e5d9f99acdca0
			e4653acbf99568e7ae8468d4965912fd2713f31b8296f470c81c7cb631cf2354
			4232749045f926b1306db049120f54350b3eb9b10564533544d5338dc413f255
			0c849c34a9da0d278a8048ac7e4520371e748fdf2c0791979a7c87f4d1118184
			0b4957ef85ab30e3e36a2c5fd8f9c6fbc8edb66e94124bf83f83c9b31dc58c09
			5859a3d6654e64eb17a4148ef628db80ad9f9d984d42f8eefc9b4712c7fe5e3e
			371bc7d2a1fe382e22adfb487b94d33e55a4f2106d0a6958d56120b814ff9c64
			52a3e15d284745966f66f859d0fe168b0e199a8578e51b626fd68e0d6c9c3a7f
			cc534e0ab9131a5d372bc6fef6cc07e7519bc195b70201b49cf7a17203d69a83
			3933506f0274247c4520ba162a16b6617824cdff8456712ae886c94ca6176da9

		Sample expected by the solidity team

		0x
			0xf4b476e1
			0000000000000000000000000000000000000000000000000000000000000020
			0000000000000000000000000000000000000000000000000000000000000005
			f887bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f
			dd87bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f
			aa87bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f
			cc87bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f
			1187bbc07b0e849fb625aafadf4cb6b65b98e492fbb689705312bf1db98ead7f
	*/

	// Create a random payload
	txDataBuf := bytes.Buffer{}
	txDataBuf.Write(bridge.MsgConfirmSelector())
	txDataBuf.Write(intWordAsBytes(32))    // the offset
	txDataBuf.Write(intWordAsBytes(nbMsg)) // the length prefix

	for i := 0; i < nbMsg; i++ {
		txDataBuf.Write(g.Bytes(32))
	}

	// Craft the transaction, randomly from any of
	var tx ethtypes.TxData
	switch g.IntN(3) {
	case 0:
		tx = &ethtypes.LegacyTx{
			Nonce:    g.Nonce(),
			GasPrice: g.BigInt(1000),
			Gas:      g.Gas(),
			To:       L2BridgeAddress(),
			Value:    big.NewInt(0),
			Data:     txDataBuf.Bytes(),
		}
	case 1:
		tx = &ethtypes.AccessListTx{
			ChainID:    ChainID(),
			Nonce:      g.Nonce(),
			GasPrice:   g.BigInt(1000),
			Gas:        g.Gas(),
			To:         L2BridgeAddress(),
			Value:      big.NewInt(0),
			Data:       txDataBuf.Bytes(),
			AccessList: ethtypes.AccessList{},
		}
	case 2:
		tx = &ethtypes.DynamicFeeTx{
			ChainID:    ChainID(),
			Nonce:      g.Nonce(),
			GasTipCap:  g.BigInt(1000),
			GasFeeCap:  g.BigInt(1000), // a.k.a. maxFeePerGas
			Gas:        g.Gas(),
			To:         L2BridgeAddress(),
			Value:      big.NewInt(0),
			Data:       txDataBuf.Bytes(),
			AccessList: ethtypes.AccessList{},
		}
	}

	encoded := ethereum.EncodeTxForSigning(ethtypes.NewTx(tx))

	// Append a random from to the transaction RLP
	return g.AppendAddress(hexutil.Encode(encoded))
}

// Returns an offset word on 32 bytes
func intWordAsBytes(offset int) []byte {
	res := make([]byte, 32)
	val := big.NewInt(int64(offset))
	valBytes := val.Bytes()
	copy(res[32-len(valBytes):], valBytes)
	return res
}

// Generate call data for a transaction
func (g *RandGen) CallData() []byte {

	const probZero float64 = 0.75
	size := g.IntRange(MinTxByteSize(), MaxTxByteSize())
	res := g.Bytes(size)

	// IRL: most of the transaction data is zero
	if ZeroesInCalldata() {
		for i := range res {
			if g.Float64() < probZero {
				res[i] = 0
			}
		}
	}

	return res
}

// Generates a random address
func (g *RandGen) Address() *common.Address {
	res := common.BytesToAddress(g.Bytes(20))
	return &res
}

// Generate a random tx nonce
func (g *RandGen) Nonce() uint64 {
	return g.Uint64() % 15
}

// Generates a random tx value
func (g *RandGen) Value() *big.Int {
	return big.NewInt(g.Int64N(1_000_000))
}

// Generate a random tx gas limit
func (g *RandGen) Gas() uint64 {
	return g.Uint64() % 1_000_000
}

// Generate a random big int
func (g *RandGen) BigInt(n int64) *big.Int {
	return big.NewInt(g.Int64N(n))
}

// Generates a list of L2 msg logs
func (g *RandGen) L2L1MsgHashes() (hashes []types.FullBytes32) {
	hashes = []types.FullBytes32{}
	n := g.IntN(g.Params.SupL2L1LogsPerBlock)
	for i := 0; i < n; i++ {
		hashes = append(hashes, types.FullBytes32FromHex(g.HexStringForNBytes(32)))
	}
	return hashes
}

// The input string is an hex encoded string representing a transaction
// RLP.
func (g *RandGen) AppendAddress(s string) string {
	addr := g.Address()
	return s + fmt.Sprintf("%x", addr)
}
