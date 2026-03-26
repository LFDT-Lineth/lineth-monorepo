package encode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/linea-monorepo/prover/backend/ethereum"
	typesLinea "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/icza/bitio"
)

// UnpackAlign unpacks r (packed with PackAlign) and returns the unpacked data.
func UnpackAlign(r []byte, packingSize int, noTerminalSymbol bool) ([]byte, error) {
	bytesPerElem := (packingSize + 7) / 8
	packingSizeLastU64 := uint8(packingSize % 64)
	if packingSizeLastU64 == 0 {
		packingSizeLastU64 = 64
	}

	n := len(r) / bytesPerElem
	if n*bytesPerElem != len(r) {
		return nil, fmt.Errorf("invalid data length; expected multiple of %d", bytesPerElem)
	}

	var out bytes.Buffer
	w := bitio.NewWriter(&out)
	for i := 0; i < n; i++ {
		// read bytes
		element := r[bytesPerElem*i : bytesPerElem*(i+1)]
		// write bits
		w.TryWriteBits(binary.BigEndian.Uint64(element[0:8]), packingSizeLastU64)
		for j := 8; j < bytesPerElem; j += 8 {
			w.TryWriteBits(binary.BigEndian.Uint64(element[j:j+8]), 64)
		}
	}
	if w.TryError != nil {
		return nil, fmt.Errorf("when writing to bitio.Writer: %w", w.TryError)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("when closing bitio.Writer: %w", err)
	}

	if !noTerminalSymbol {
		// the last nonzero byte should be 0xff
		outLen := out.Len() - 1
		for out.Bytes()[outLen] == 0 {
			outLen--
		}
		if out.Bytes()[outLen] != 0xff {
			return nil, errors.New("invalid terminal symbol")
		}
		out.Truncate(outLen)
	}

	return out.Bytes(), nil
}

type packAlignSettings struct {
	dataNbBits           int
	lastByteNbUnusedBits uint8
	noTerminalSymbol     bool
	additionalInput      [][]byte
}

func (s *packAlignSettings) initialize(length int, options ...packAlignOption) {

	for _, opt := range options {
		opt(s)
	}

	nbBytes := length
	for _, data := range s.additionalInput {
		nbBytes += len(data)
	}

	if !s.noTerminalSymbol {
		nbBytes++
	}

	s.dataNbBits = nbBytes*8 - int(s.lastByteNbUnusedBits)
}

type packAlignOption func(*packAlignSettings)

func NoTerminalSymbol() packAlignOption {
	return func(o *packAlignSettings) {
		o.noTerminalSymbol = true
	}
}

// PackAlignSize returns the size of the data when packed with PackAlign.
func PackAlignSize(length0, packingSize int, options ...packAlignOption) (n int) {
	var s packAlignSettings
	s.initialize(length0, options...)

	// we may need to add some bits to a and b to ensure we can process some blocks of 248 bits
	extraBits := (packingSize - s.dataNbBits%packingSize) % packingSize
	nbBits := s.dataNbBits + extraBits

	return (nbBits / packingSize) * ((packingSize + 7) / 8)
}

func WithAdditionalInput(data ...[]byte) packAlignOption {
	return func(o *packAlignSettings) {
		o.additionalInput = append(o.additionalInput, data...)
	}
}

func WithLastByteNbUnusedBits(n uint8) packAlignOption {
	if n > 7 {
		panic("only 8 bits to a byte")
	}
	return func(o *packAlignSettings) {
		o.lastByteNbUnusedBits = n
	}
}

// PackAlign writes a and b to w, aligned to fr.Element (bls12-377) boundary.
// It returns the length of the data written to w.
func PackAlign(w io.Writer, a []byte, packingSize int, options ...packAlignOption) (n int64, err error) {

	var s packAlignSettings
	s.initialize(len(a), options...)
	if !s.noTerminalSymbol && s.lastByteNbUnusedBits != 0 {
		return 0, errors.New("terminal symbols with byte aligned input not yet supported")
	}

	// we may need to add some bits to a and b to ensure we can process some blocks of packingSize bits
	nbBits := (s.dataNbBits + (packingSize - 1)) / packingSize * packingSize
	extraBits := nbBits - s.dataNbBits

	// padding will always be less than bytesPerElem bytes
	bytesPerElem := (packingSize + 7) / 8
	packingSizeLastU64 := uint8(packingSize % 64)
	if packingSizeLastU64 == 0 {
		packingSizeLastU64 = 64
	}
	bytePadding := (extraBits + 7) / 8
	buf := make([]byte, bytesPerElem, bytesPerElem+1)

	// the last nonzero byte is 0xff
	if !s.noTerminalSymbol {
		buf = append(buf, 0)
		buf[0] = 0xff
	}

	inReaders := make([]io.Reader, 2+len(s.additionalInput))
	inReaders[0] = bytes.NewReader(a)
	for i, data := range s.additionalInput {
		inReaders[i+1] = bytes.NewReader(data)
	}
	inReaders[len(inReaders)-1] = bytes.NewReader(buf[:bytePadding+1])

	r := bitio.NewReader(io.MultiReader(inReaders...))

	var tryWriteErr error
	tryWrite := func(v uint64) {
		if tryWriteErr == nil {
			tryWriteErr = binary.Write(w, binary.BigEndian, v)
		}
	}

	for i := 0; i < nbBits/packingSize; i++ {
		tryWrite(r.TryReadBits(packingSizeLastU64))
		for j := int(packingSizeLastU64); j < packingSize; j += 64 {
			tryWrite(r.TryReadBits(64))
		}
	}

	if tryWriteErr != nil {
		return 0, fmt.Errorf("when writing to w: %w", tryWriteErr)
	}

	if r.TryError != nil {
		return 0, fmt.Errorf("when reading from multi-reader: %w", r.TryError)
	}

	n1 := (nbBits / packingSize) * bytesPerElem
	if n1 != PackAlignSize(len(a), packingSize, options...) {
		return 0, errors.New("inconsistent PackAlignSize")
	}
	return int64(n1), nil
}

// PackData re-packs the data tightly into bls12-377 elements. Useful for minimizing the number of hash permutations.
// only supporting packing without a terminal symbol. Input with a terminal symbol will be interpreted in full padded length.
func PackData(data []byte, inputPackingSize int, hashPackingOptions ...packAlignOption) ([]byte, error) {
	dataNbBits := len(data) * 8
	if inputPackingSize%8 != 0 {
		inputBytesPerElem := (inputPackingSize + 7) / 8
		dataNbBits = dataNbBits / inputBytesPerElem * inputPackingSize
		var err error
		if data, err = UnpackAlign(data, inputPackingSize, true); err != nil {
			return nil, err
		}
	}

	lastByteNbUnusedBits := 8 - dataNbBits%8
	if lastByteNbUnusedBits == 8 {
		lastByteNbUnusedBits = 0
	}

	var bb bytes.Buffer
	packingOptions := make([]packAlignOption, len(hashPackingOptions)+1)
	copy(packingOptions, hashPackingOptions)
	packingOptions[len(packingOptions)-1] = WithLastByteNbUnusedBits(uint8(lastByteNbUnusedBits))
	_, err := PackAlign(&bb, data, fr.Bits-1, packingOptions...)

	return bb.Bytes(), err
}

// MiMCChecksumPackedData re-packs the data tightly into bls12-377 elements and computes the MiMC checksum.
// only supporting packing without a terminal symbol. Input with a terminal symbol will be interpreted in full padded length.
func MiMCChecksumPackedData(data []byte, inputPackingSize int, hashPackingOptions ...packAlignOption) ([]byte, error) {
	b, err := PackData(data, inputPackingSize, hashPackingOptions...)

	hsh := hash.MIMC_BLS12_377.New()
	hsh.Write(b)
	return hsh.Sum(nil), err
}

// Poseidon2ChecksumPackedData re-packs the data tightly into bls12-377 elements and computes the MiMC checksum.
// only supporting packing without a terminal symbol. Input with a terminal symbol will be interpreted in full padded length.
func Poseidon2ChecksumPackedData(data []byte, inputPackingSize int, hashPackingOptions ...packAlignOption) ([]byte, error) {
	b, err := PackData(data, inputPackingSize, hashPackingOptions...)

	hsh := hash.POSEIDON2_BLS12_377.New()
	hsh.Write(b)
	return hsh.Sum(nil), err
}

// DecodedBlockData is a wrapper struct storing the different fields of a block
// that we deserialize when decoding an ethereum block.
type DecodedBlockData struct {
	// BlockHash stores the decoded block hash
	BlockHash common.Hash
	// Timestamp holds the Unix timestamp of the block in
	Timestamp uint64
	// Froms stores the list of the sender address of every transaction
	Froms []common.Address
	// Txs stores the list of the decoded transactions.
	Txs []types.TxData
}

func InjectFromAddressIntoR(txData types.TxData, from *common.Address) *types.Transaction {
	switch txData := txData.(type) {
	case *types.DynamicFeeTx:
		tx := *txData
		tx.R = new(big.Int)
		tx.R.SetBytes(from[:])
		tx.S = big.NewInt(1)
		return types.NewTx(&tx)
	case *types.AccessListTx:
		tx := *txData
		tx.R = new(big.Int)
		tx.R.SetBytes(from[:])
		tx.S = big.NewInt(1)
		return types.NewTx(&tx)
	case *types.LegacyTx:
		tx := *txData
		tx.R = new(big.Int)
		tx.R.SetBytes(from[:])
		tx.S = big.NewInt(1)
		return types.NewTx(&tx)
	case *types.SetCodeTx:
		tx := *txData
		tx.R = new(uint256.Int)
		tx.R.SetBytes(from[:])
		tx.S = uint256.NewInt(1)
		return types.NewTx(&tx)
	default:
		panic("unexpected transaction type")
	}
}

// ToStd converts the decoded block data into a standard
// block object capable of being encoded in a way consumable
// by existing decoders. The process involves some abuse,
// whereby 1) the "from" address of a transaction is put in the
// signature.R field, though the signature as a whole is invalid.
// 2) the block hash is stored in the ParentHash field in the block
// header.
func (d *DecodedBlockData) ToStd() *types.Block {
	header := types.Header{
		ParentHash: d.BlockHash,
		Time:       d.Timestamp,
	}

	body := types.Body{
		Transactions: make([]*types.Transaction, len(d.Txs)),
	}

	for i := range d.Txs {
		body.Transactions[i] = InjectFromAddressIntoR(d.Txs[i], &d.Froms[i])
	}

	return types.NewBlock(&header, &body, nil, emptyTrieHasher{})
}

func GetAddressFromR(tx *types.Transaction) typesLinea.EthAddress {
	_, r, _ := tx.RawSignatureValues()
	var res typesLinea.EthAddress
	r.FillBytes(res[:])
	return res
}

type emptyTrieHasher struct{}

func (h emptyTrieHasher) Reset() {
}

func (h emptyTrieHasher) Update(_, _ []byte) error {
	return nil
}

func (h emptyTrieHasher) Hash() common.Hash {
	return common.Hash{}
}

type TxAddressGetter func(*types.Transaction) typesLinea.EthAddress

type Config struct {
	GetAddress TxAddressGetter
}

func NewConfig() Config {
	return Config{
		GetAddress: ethereum.GetFrom,
	}
}

type Option func(*Config)

func WithTxAddressGetter(g TxAddressGetter) Option {
	return func(cfg *Config) {
		cfg.GetAddress = g
	}
}
