package testcase_gen

import (
	"flag"
	"math"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

const (
	// seed for the RNG
	SEED_FLAG  string = "seed"
	SEED_USAGE string = "required, integer, seed to use for the RNG"

	// chain-id
	CHAIN_ID_FLAG  string = "chainid"
	CHAIN_ID_USAGE string = "required, integer, the chainID of L2"

	// l2 bridge address
	L2_MESSAGE_SERVICE_CONTRACT_FLAG  string = "l2-bridge-address"
	L2_MESSAGE_SERVICE_CONTRACT_USAGE string = "l2-bridge-address: required, the 0x prefixed address of the bridge"

	// time stamp
	START_FROM_TIMESTAMP_FLAG  string = "start-from-timestamp"
	START_FROM_TIMESTAMP_USAGE string = "required, UNIX-timestamp, timestamp starting which we generate the blocks"

	// initial root hash
	START_FROM_ROOT_HASH_FLAG  string = "start-from-root-hash"
	START_FROM_ROOT_HASH_USAGE string = "root upon which we generate a proof"

	// final root hash
	END_WITH_ROOT_HASH_FLAG  string = "end-with-root-hash"
	END_WITH_ROOT_HASH_USAGE string = "optional, root hash at which the sequence of blocks should end"

	// output file
	OUTPUT_FILE_FLAG  string = "ofile"
	OUTPUT_FILE_USAGE string = "required, path, file where to write the testcase file"

	// max number of block
	NUM_BLOCKS_FLAG  string = "num-blocks"
	NUM_BLOCKS_USAGE string = "optional, integer, maximal number of block in the conflated batch"

	// bound on the number of tx per block
	MAX_TX_PER_BLOCK_FLAG  string = "max-tx-per-block"
	MAX_TX_PER_BLOCK_USAGE string = "optional, integer, maximal number of tx per block"

	// bound on the number of l2 msg logs per block
	// #nosec G101 -- Not a credential
	MAX_L2L1_LOGS_PER_BLOCKS_FLAG  string = "max-l2-l1-logs-per-block"
	MAX_L2L1_LOGS_PER_BLOCKS_USAGE string = "optional, int, maximal number of l2 msg emitted per block"

	// bound on the number of msg receipt confirmation
	MAX_L1L2_MSG_RECEIPT_PER_BLOCK_FLAG  string = "max-l1-l2-msg-receipt-per-block"
	MAX_L1L2_MSG_RECEIPT_PER_BLOCK_USAGE string = "optional, int, maximal number of l1 msg received per block (at most one batch per block)."

	// min tx size
	MIN_TX_BYTE_SIZE_FLAG  string = "min-tx-byte-size"
	MIN_TX_BYTE_SIZE_USAGE string = "optional, int, minimal size of an rlp encoded tx"

	// max-tx-size
	MAX_TX_BYTE_SIZE_FLAG  string = "max-tx-byte-size"
	MAX_TX_BYTE_SIZE_USAGE string = "optional, int, maximal size of an rlp encoded tx"

	// calldata-with-zero
	ZEROES_IN_CALLDATA_FLAG  string = "zeroes-in-calldata"
	ZEROES_IN_CALLDATA_USAGE string = "optional, bool, default to true. If set to true 75pc of the bytes of the calldata of the generated txs is set to zero"

	// start from block number
	START_FROM_BLOCK_NUMBER_FLAG  string = "start-from-block"
	START_FROM_BLOCK_NUMBER_USAGE string = "required, integer"
)

// placeholder variables for the flag values
var (
	seed, chainid                                        *int64
	l2BridgeAddress, startFromRootHash, oFile            *string
	endWithRootHash                                      *string
	startFromTimestamp                                   *uint64
	numBlock, maxTxPerBlock, maxL2L1LogsPerBlock         *int
	maxL1L2ReceiptPerBlock, minTxByteSize, maxTxByteSize *int
	startFromBlock                                       *int
	zeroesInCalldata                                     *bool
)

func Initialize() {
	// Ensures all the parameters have been provided

	// Requireds,
	seed = flag.Int64(SEED_FLAG, math.MinInt64, SEED_USAGE)
	chainid = flag.Int64(CHAIN_ID_FLAG, math.MinInt64, CHAIN_ID_USAGE)
	l2BridgeAddress = flag.String(L2_MESSAGE_SERVICE_CONTRACT_FLAG, "", L2_MESSAGE_SERVICE_CONTRACT_USAGE)
	startFromTimestamp = flag.Uint64(START_FROM_TIMESTAMP_FLAG, 0, START_FROM_TIMESTAMP_USAGE)
	startFromRootHash = flag.String(START_FROM_ROOT_HASH_FLAG, "", START_FROM_ROOT_HASH_USAGE)
	oFile = flag.String(OUTPUT_FILE_FLAG, "", OUTPUT_FILE_USAGE)
	startFromBlock = flag.Int(START_FROM_BLOCK_NUMBER_FLAG, -1, START_FROM_BLOCK_NUMBER_USAGE)

	// Optional,
	numBlock = flag.Int(NUM_BLOCKS_FLAG, 10, NUM_BLOCKS_USAGE)
	maxTxPerBlock = flag.Int(MAX_TX_PER_BLOCK_FLAG, 20, MAX_TX_PER_BLOCK_USAGE)
	maxL2L1LogsPerBlock = flag.Int(MAX_L2L1_LOGS_PER_BLOCKS_FLAG, 16, MAX_L2L1_LOGS_PER_BLOCKS_USAGE)
	maxL1L2ReceiptPerBlock = flag.Int(MAX_L1L2_MSG_RECEIPT_PER_BLOCK_FLAG, 16, MAX_L1L2_MSG_RECEIPT_PER_BLOCK_USAGE)
	minTxByteSize = flag.Int(MIN_TX_BYTE_SIZE_FLAG, 128, MIN_TX_BYTE_SIZE_USAGE)
	maxTxByteSize = flag.Int(MAX_TX_BYTE_SIZE_FLAG, 512, MAX_TX_BYTE_SIZE_USAGE)
	zeroesInCalldata = flag.Bool(ZEROES_IN_CALLDATA_FLAG, true, ZEROES_IN_CALLDATA_USAGE)
	endWithRootHash = flag.String(END_WITH_ROOT_HASH_FLAG, "", END_WITH_ROOT_HASH_USAGE)

	// parse the flags
	flag.Parse()
	flag.PrintDefaults()
}

func Seed() int64 {
	// MinInt is used a garbage value to indicate the flag was not provided
	if *seed == math.MinInt {
		utils.Panic("seed is required")
	}
	return *seed
}

func ChainID() *big.Int {
	// MinInt is used a garbage value to indicate the flag was not provided
	if *chainid < 0 {
		utils.Panic("chain id is required")
	}
	return big.NewInt(*chainid)
}

func L2BridgeAddress() *common.Address {
	// empty string means the address was not sent
	if *l2BridgeAddress == "" {
		utils.Panic("l2 bridge address is required")
	}

	// check that the address is correctly formatted
	if b, err := hexutil.Decode(*l2BridgeAddress); err != nil || len(b) != 20 {
		utils.Panic("the address should be an 0x prefixed string with 40 hexes")
	}

	res := common.HexToAddress(*l2BridgeAddress)
	return &res
}

func NumBlock() int {
	return *numBlock
}

func MaxTxPerBlock() int {
	return *maxTxPerBlock
}

func MaxL2L1LogsPerBlock() int {
	return *maxL2L1LogsPerBlock
}

func MaxL1L2ReceiptPerBlock() int {
	return *maxL1L2ReceiptPerBlock
}

func MinTxByteSize() int {
	return *minTxByteSize
}

func MaxTxByteSize() int {
	return *maxTxByteSize
}

func StartFromRootHash() types.KoalaOctuplet {
	// empty string means the address was not sent
	if *startFromRootHash == "" {
		utils.Panic("l2 bridge address is required")
	}

	r, err := types.HexToKoalabearOctuplet(*startFromRootHash)
	if err != nil {
		utils.Panic("the address should be an 0x prefixed string with 64 hexes, got %v", startFromRootHash)
	}
	return r
}

func StartFromTimeStamp() uint64 {
	// start from timestamp
	// MinInt is used a garbage value to indicate the flag was not provided
	if *startFromTimestamp == 0 {
		utils.Panic("t is required")
	}
	return *startFromTimestamp

}

func Ofile() string {
	if *oFile == "" {
		panic("ofile is required, should be a valid filepath")
	}
	return *oFile
}

func ZeroesInCalldata() bool {
	return *zeroesInCalldata
}

func StartFromBlock() int {
	if *startFromBlock == -1 {
		panic("did pass the start-from-block argument")
	}
	return *startFromBlock
}

func EndWithRootHash() types.KoalaOctuplet {
	return types.MustHexToKoalabearOctuplet(*endWithRootHash)
}
