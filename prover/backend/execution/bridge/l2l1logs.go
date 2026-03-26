package bridge

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"
)

// L2L1Log message
type L2L1Log struct {
	from, to          common.Address
	fees, value, salt [32]byte
	calldata          []byte
	messageHash       common.Hash
}

// Returns the L2L1LogHashes from a list of logs and abi.encode them
func L2L1MessageHashes(logs []ethtypes.Log, l2BridgeAddress common.Address) []types.FullBytes32 {

	logrus.Tracef("Filtering the following logs: %++v", logs)
	res := []types.FullBytes32{}
	for _, log := range logs {
		// Filters out the uninteresting logs
		if !isL2L1Log(log, l2BridgeAddress) {
			continue
		}
		// If we are there, it means we are interested in this log so ABI encode it
		abiLog := ParseL2L1Log(log).MsgHash()
		// Although, we ultimately rely on the hash of the L2L1 log
		res = append(res, abiLog)
	}
	return res
}

// Returns true iff the log is an l2l1 event
func isL2L1Log(log ethtypes.Log, l2BridgeAddress common.Address) bool {

	// the log should originate the right address
	if log.Address != l2BridgeAddress {
		return false
	}

	// Sanity-check, in case the log has no topic
	if len(log.Topics) == 0 {
		logrus.Warnf("Found a log with no topic, is it legit ? %++v", log)
		return false
	}

	// the log should have the right signature. i.e,
	//
	// event MessageSent(
	// 		address indexed _from,
	// 		address indexed _to,
	// 		uint256 _fee,
	// 		uint256 _value,
	// 		uint256 _salt,
	// 		bytes _calldata,
	// 		bytes32 indexed _messageHash
	// );
	if log.Topics[0] != L2L1Topic0() {
		return false
	}

	// Then, it's a good L2L1 log
	return true
}

// ABI encode an L2L1 log
func ParseL2L1Log(log ethtypes.Log) L2L1Log {

	// Get the from and to from the topics
	res := L2L1Log{}
	copy(res.from[:], log.Topics[1][12:])      // only keep the 20 last bytes
	copy(res.to[:], log.Topics[2][12:])        // only keep the last 20 bytes
	copy(res.messageHash[:], log.Topics[3][:]) // only keep

	// decode the rest of the field from the data
	buf := bytes.Buffer{}
	buf.Write(log.Data)
	tmp := make([]byte, 32) // tmp buffer used to read 32bytes by 32bytes the buffer

	// tmpBig : tmp bigint used to hold the length and the offset of the calldata field
	var tmpBig big.Int

	// the fees
	_, err := buf.Read(res.fees[:])
	if err != nil {
		utils.Panic("Unable to read the fees from the log data : %v", err)
	}
	// value
	_, err = buf.Read(res.value[:])
	if err != nil {
		utils.Panic("Unable to read the value from the log data : %v", err)
	}
	// salt
	_, err = buf.Read(res.salt[:])
	if err != nil {
		utils.Panic("Unable to read the salt from the log data : %v", err)
	}

	// offset for calldata. we are expecting a fixed value here
	_, err = buf.Read(tmp)
	if err != nil {
		utils.Panic("Unable to read the calldata offset from the log data : %v", err)
	}
	offset := tmpBig.SetBytes(tmp).Int64()
	if offset != 4*32 { // 7th field and each field takes 32 bytes
		utils.Panic("Bad offset. expected %v but got %v", 4*32, offset)
	}

	// Length of the buffer
	buf.Read(tmp)
	length := tmpBig.SetBytes(tmp).Int64() // length in bytes

	// That means we are receiving truncated logs
	if buf.Len() < int(length) {
		utils.Panic("Misformatted log data : parsed calldata length is %v, but the remaining number of bytes in the buffer is %v", length, buf.Len())
	}

	res.calldata = make([]byte, length)
	buf.Read(res.calldata)

	trimmed := buf.Bytes()
	if len(trimmed) == 0 {
		// clean parsing, nothing remains in the buffer : authorized
		return res
	}

	// Too much data was sent, checking first if this is a slice of zeroes.

	if reflect.DeepEqual(trimmed, make([]byte, len(trimmed))) {
		// there remains data in the buffer but it's all zeroes : authorized
		logrus.Debugf("Trimming %d zeroes from the logdata", len(trimmed))
		return res
	}

	// there remains data in the buffer and not all of it is zeroes : forbidden
	utils.Panic("Misformatted log data : trimmed the following bytes from the logdata : 0x%x", trimmed)
	panic("unreachable")
}

// Returns the message hash of the log in hex format
func (l L2L1Log) MsgHash() types.FullBytes32 {
	return types.FullBytes32(l.messageHash)
}

// Returns the selector for the L2 L1 logs
func L2L1Topic0() (res [32]byte) {
	// signature of the event
	const signature string = "MessageSent(address,address,uint256,uint256,uint256,bytes,bytes32)"
	hashed := crypto.Keccak256([]byte(signature))
	copy(res[:], hashed)
	return res
}
