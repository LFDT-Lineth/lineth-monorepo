package execution

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/config"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/assert"
)

func makeConfig(chainID, baseFee uint, coinBase, msgSvc ethcommon.Address) *config.Config {
	return &config.Config{
		Layer2: struct {
			ChainID           uint              `mapstructure:"chain_id" validate:"required"`
			BaseFee           uint              `mapstructure:"base_fee"`
			MsgSvcContractStr string            `mapstructure:"message_service_contract" validate:"required,eth_addr"`
			MsgSvcContract    ethcommon.Address `mapstructure:"-"`
			CoinBaseStr       string            `mapstructure:"coin_base" validate:"required,eth_addr"`
			CoinBase          ethcommon.Address `mapstructure:"-"`
		}{
			ChainID:        chainID,
			BaseFee:        baseFee,
			CoinBase:       coinBase,
			MsgSvcContract: msgSvc,
		},
	}
}

func makeBlock(number uint64, coinBase ethcommon.Address, baseFee *big.Int, txs ...*ethtypes.Transaction) ethtypes.Block {
	header := &ethtypes.Header{
		Number:   new(big.Int).SetUint64(number),
		Coinbase: coinBase,
		BaseFee:  baseFee,
	}
	return *ethtypes.NewBlock(header, &ethtypes.Body{Transactions: txs}, nil, trie.NewListHasher())
}

func makeLegacyTx() *ethtypes.Transaction {
	return ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1),
		Gas:      21000,
	})
}

func makeDynamicTx(chainID *big.Int) *ethtypes.Transaction {
	return ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     0,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(100),
		Gas:       21000,
	})
}

func TestValidateChainConfig_EmptyBlocks(t *testing.T) {
	cfg := makeConfig(1337, 7, ethcommon.Address{}, ethcommon.Address{})
	// Should not panic with empty blocks
	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, nil)
	})
	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{})
	})
}

func TestValidateChainConfig_MatchingConfig(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	msgSvc := ethcommon.HexToAddress("0xe537D669CA013d86EBeF1D64e40fC74CADC91987")
	cfg := makeConfig(1337, 7, coinBase, msgSvc)

	block := makeBlock(1, coinBase, big.NewInt(7),
		makeDynamicTx(big.NewInt(1337)),
	)

	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
}

func TestValidateChainConfig_MultipleBlocks(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	blocks := []ethtypes.Block{
		makeBlock(1, coinBase, big.NewInt(7)),
		makeBlock(2, coinBase, big.NewInt(7)),
		makeBlock(3, coinBase, big.NewInt(7)),
	}

	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, blocks)
	})
}

func TestValidateChainConfig_CoinbaseMismatch(t *testing.T) {
	cfgCoinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	blockCoinBase := ethcommon.HexToAddress("0x19bf28626BE6f6aE4ca7d41A5aDe0305e9DC5FCA")
	cfg := makeConfig(1337, 7, cfgCoinBase, ethcommon.Address{})

	block := makeBlock(1, blockCoinBase, big.NewInt(7))

	msg := recoverPanicMsg(func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
	assert.True(t, len(msg) > 0, "expected panic")
	assert.Contains(t, msg, "coinBase")
	assert.Contains(t, msg, "CONFIG")
	assert.Contains(t, msg, "BLOCK")
}

func TestValidateChainConfig_BaseFeeMismatch(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	block := makeBlock(1, coinBase, big.NewInt(42))

	assert.Panics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
}

func TestValidateChainConfig_ChainIDMismatchInTx(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	block := makeBlock(1, coinBase, big.NewInt(7),
		makeDynamicTx(big.NewInt(59139)), // wrong chainID
	)

	assert.Panics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
}

func TestValidateChainConfig_LegacyTxSkipped(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	// Legacy transactions have chainID == 0, should be skipped
	block := makeBlock(1, coinBase, big.NewInt(7), makeLegacyTx())

	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
}

func TestValidateChainConfig_NilBaseFeeSkipped(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	// Block with nil baseFee — check should be skipped
	block := makeBlock(1, coinBase, nil)

	assert.NotPanics(t, func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})
}

func TestValidateChainConfig_MismatchOnSecondBlock(t *testing.T) {
	coinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	wrongCoinBase := ethcommon.HexToAddress("0x19bf28626BE6f6aE4ca7d41A5aDe0305e9DC5FCA")
	cfg := makeConfig(1337, 7, coinBase, ethcommon.Address{})

	blocks := []ethtypes.Block{
		makeBlock(1, coinBase, big.NewInt(7)),
		makeBlock(2, wrongCoinBase, big.NewInt(7)), // mismatch on second block
	}

	assert.Panics(t, func() {
		sanityCheckChainConfig(cfg, blocks)
	})
}

func TestSanityCheckChainConfig_SepoliaVsMainnet(t *testing.T) {
	// Simulates the scenario: using Sepolia config against Mainnet blocks
	sepoliaCoinBase := ethcommon.HexToAddress("0xA27342f1b74c0cfB2cda74bac1628d0C1A9752f2")
	mainnetCoinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")

	// Sepolia config (chainID 59141)
	cfg := makeConfig(59141, 7, sepoliaCoinBase, ethcommon.HexToAddress("0x971e727e956690b9957be6d51Ec16E73AcAC83A7"))

	// Mainnet block (chainID 59144, different coinbase)
	block := makeBlock(1, mainnetCoinBase, big.NewInt(7),
		makeDynamicTx(big.NewInt(59144)),
	)

	msg := recoverPanicMsg(func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})

	// Should report both coinBase and chainID mismatches in one panic
	assert.Contains(t, msg, "coinBase")
	assert.Contains(t, msg, "chainID")
	assert.Contains(t, msg, "59141")
	assert.Contains(t, msg, "59144")
	assert.Contains(t, msg, sepoliaCoinBase.Hex())
	assert.Contains(t, msg, mainnetCoinBase.Hex())
}

func TestSanityCheckChainConfig_AllMismatches(t *testing.T) {
	// Config: Sepolia
	sepoliaCoinBase := ethcommon.HexToAddress("0xA27342f1b74c0cfB2cda74bac1628d0C1A9752f2")
	cfg := makeConfig(59141, 7, sepoliaCoinBase, ethcommon.HexToAddress("0x971e727e956690b9957be6d51Ec16E73AcAC83A7"))

	// Block: Mainnet with every field different
	mainnetCoinBase := ethcommon.HexToAddress("0x8F81e2E3F8b46467523463835F965fFE476E1c9E")
	block := makeBlock(100, mainnetCoinBase, big.NewInt(42),
		makeDynamicTx(big.NewInt(59144)),
	)

	msg := recoverPanicMsg(func() {
		sanityCheckChainConfig(cfg, []ethtypes.Block{block})
	})

	// All three mismatches should appear in one message
	assert.Contains(t, msg, "coinBase")
	assert.Contains(t, msg, "baseFee")
	assert.Contains(t, msg, "chainID")
	// Table headers should be present
	assert.Contains(t, msg, "CONFIG")
	assert.Contains(t, msg, "BLOCK")
}

// recoverPanicMsg runs fn and returns the panic message as a string.
// Returns empty string if fn did not panic.
func recoverPanicMsg(fn func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = func() string {
				switch v := r.(type) {
				case string:
					return v
				case error:
					return v.Error()
				default:
					return ""
				}
			}()
		}
	}()
	fn()
	return ""
}
