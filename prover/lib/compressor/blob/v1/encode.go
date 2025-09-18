package v1

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// EncodeBlockForCompression encodes a block for compression.
func EncodeBlockForCompression(block *types.Block, w io.Writer) error {

	if block == nil {
		return fmt.Errorf("block is nil")
	}

	if block.Transactions() == nil {
		return fmt.Errorf("block has nil transactions")
	}

	var (
		// timestamp is the timestamp of the encoded block. The timestamp is
		// encoded over a uint32 as this is sufficient in practice.
		timestamp = uint32(block.Time())
		// transactions lists the txs of the encoded block.
		transactions = block.Transactions()
		// blkHash holds the block hash
		blockHash = block.Hash()
		// numTxs holds the number of transaction. We use a uint16 as this is a
		// realistic maximal value to store the transactions.
		numTxs = uint16(len(transactions))
	)

	binary.Write(w, binary.BigEndian, numTxs)
	binary.Write(w, binary.BigEndian, timestamp)
	w.Write(blockHash[:])

	for i, tx := range transactions {
		if err := EncodeTxForCompression(tx, w); err != nil {
			return fmt.Errorf("could not encode transaction #%v: %w", i, err)
		}
	}

	return nil
}

// EncodeTxForCompression encodes a single transaction
func EncodeTxForCompression(tx *types.Transaction, w io.Writer) error {
	if tx == nil {
		return fmt.Errorf("transactions is nil")
	}

	var (
		from    = ethereum.GetFrom(tx)
		txRlp   = ethereum.EncodeTxForSigning(tx)
		_, err1 = w.Write(from[:])
		_, err2 = w.Write(txRlp[:])
	)

	return errors.Join(err1, err2)
}

// ScanBlockByteLen scans the stream of bytes `b`, expecting to find an encoded
// block starting from position 0 and returns the length of the block. It returns
// an error if the scanner goes out of bound.
func ScanBlockByteLen(b []byte) (int, error) {

	const (
		// preTxBufSize corresponds to the size of the buffer area used for
		// encoding the block-hash and the timestamp. They are are stored ahead
		// of the transaction
		preTxBufSize = 32 + 4

		// heuristicMaxNbTxs corresponds to a tacit maximal value that we can
		// expect to be contained in the currently scanned block. The theoretical
		// max value is 2**16 but a value higher than heuristic value is
		// considered "odd" and triggers an error. The check can be easily remove
		// in the future if we decide to make large blocks. This is used as an
		// early fail mechanism.
		heuristicMaxNbTxs = 1 << 10
	)

	var (
		r = bytes.NewReader(b)
		// decNumTxs corresponds to the number of transaction that are contained
		// in the currently scanned block.
		decNumTxs uint16
	)

	if err := binary.Read(r, binary.BigEndian, &decNumTxs); err != nil {
		return -1, fmt.Errorf("could not decode nb txs: %w", err)
	}

	if decNumTxs > heuristicMaxNbTxs {
		return -1, fmt.Errorf("invalid block: the decoded nb tx is %v > %v", decNumTxs, heuristicMaxNbTxs)
	}

	r.Seek(preTxBufSize, io.SeekCurrent)

	for i := 0; i < int(decNumTxs); i++ {
		// Pass the tx from address
		r.Seek(int64(len(common.Address{})), io.SeekCurrent)

		// The transaction of type dynamicFee and accessList have a prefix ahead
		// the RLP encoding. When that is the case, we need to skip it before
		// scanning the RLP prefix of the transaction.
		prefix, err := r.ReadByte()
		if err != nil {
			return -1, fmt.Errorf("could not read the prefix byte of the tx #%v", i)
		}

		// Prefer reading as list (legacy tx type)
		// This limits the number of possible transaction types
		// to 192. Beyond that point this encoding becomes ambiguous.
		if int(prefix) > 0xc0 {
			if err = r.UnreadByte(); err != nil {
				return -1, fmt.Errorf("failed to rewind tx prefix: %w", err)
			}
		}

		if err = passRlpList(r); err != nil {
			return -1, fmt.Errorf("failed passing transaction #%v: %w", i, err)
		}
	}

	return int(r.Size()) - r.Len(), nil
}

// passRlpList advances the reader through an RLP list assuming the reader is
// currently pointing to an RLP-encoded list. This is used to scan the encoded
// number of bytes of an encoded blocks and more specifically to find the
// boundaries of an RLP-encoded transaction.
func passRlpList(r *bytes.Reader) error {
	firstByte, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("could not read the first byte: %w", err)
	}

	if firstByte < 0xc0 {
		return fmt.Errorf("not an RLP list, the first byte is `0x%x`", firstByte)
	}

	if firstByte <= 0xf7 {
		payLoadLen := int(firstByte) - 0xc0
		r.Seek(int64(payLoadLen), io.SeekCurrent)
		return nil
	}

	if firstByte > 0xf7 {
		l := int(firstByte) - 0xf7
		var buf [8]byte
		r.Read(buf[8-l:])
		payloadLen := binary.BigEndian.Uint64(buf[:])
		r.Seek(int64(payloadLen), io.SeekCurrent)
	}

	return nil
}

// DecodeBlockFromUncompressed inverts [EncodeBlockForCompression]. It is primarily meant for
// testing and ensuring the encoding is bijective.
func DecodeBlockFromUncompressed(r *bytes.Reader) (encode.DecodedBlockData, error) {

	var (
		decNumTxs    uint16
		decTimestamp uint32
		blockHash    common.Hash
	)

	if err := binary.Read(r, binary.BigEndian, &decNumTxs); err != nil {
		return encode.DecodedBlockData{}, fmt.Errorf("could not decode nb txs: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &decTimestamp); err != nil {
		return encode.DecodedBlockData{}, fmt.Errorf("could not decode timestamp: %w", err)
	}

	if _, err := r.Read(blockHash[:]); err != nil {
		return encode.DecodedBlockData{}, fmt.Errorf("could not read the block hash: %w", err)
	}

	numTxs := int(decNumTxs)
	decodedBlk := encode.DecodedBlockData{
		Froms:     make([]common.Address, numTxs),
		Txs:       make([]types.TxData, numTxs),
		Timestamp: uint64(decTimestamp),
		BlockHash: blockHash,
	}

	var err error
	for i := 0; i < int(decNumTxs); i++ {
		if decodedBlk.Txs[i], err = DecodeTxFromUncompressed(r, &decodedBlk.Froms[i]); err != nil {
			return encode.DecodedBlockData{}, fmt.Errorf("could not decode transaction #%v: %w", i, err)
		}
	}

	return decodedBlk, nil
}

func DecodeTxFromUncompressed(r *bytes.Reader, from *common.Address) (types.TxData, error) {
	if _, err := r.Read(from[:]); err != nil {
		return nil, fmt.Errorf("could not read from address: %w", err)
	}

	return ethereum.DecodeTxFromBytes(r)
}
