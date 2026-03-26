package v1

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"

	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
)

// A Header is a list of batches of blocks of len(blocks)
// len(BatchSizes) == nb of batches in the blob
type Header struct {
	BatchSizes         []int // BatchSizes[i] == byte size of the i-th batch
	CurrBatchBlocksLen []int // CurrBatchBlocksLen[i] == byte size of the i-th block in the current batch
	DictChecksum       [fr.Bytes]byte
	Version            uint16
}

func (s *Header) Equals(other *Header) bool {
	if other == nil {
		return false
	}
	if s.DictChecksum != other.DictChecksum {
		return false
	}

	sSum := sum(s.CurrBatchBlocksLen)
	otherSum := sum(other.CurrBatchBlocksLen)

	if sSum == otherSum {
		return slices.Equal(s.BatchSizes, other.BatchSizes)
	}

	small, large, smallSum := s, other, sSum // small/large as in the number of batches in the blob
	if sSum == 0 {                           // so "small" in smallSum is not an adjective, but a noun. smallSum is in fact supposed to be the only non-zero of the two sums
		small, large, smallSum = other, s, otherSum
	} else if otherSum != 0 { // if the sums are not equal and both nonzero, there's no way they can be considered equal
		return false
	}

	if len(large.BatchSizes) != len(small.BatchSizes)+1 {
		return false
	}

	return slices.Equal(small.BatchSizes, large.BatchSizes[:len(small.BatchSizes)]) &&
		large.BatchSizes[len(small.BatchSizes)] == smallSum
}

func (s *Header) NbBatches() int {
	return len(s.BatchSizes)
}

// if there is a batch currently being built, finalize it and start a new one
func (s *Header) sealBatch() {
	if len(s.CurrBatchBlocksLen) != 0 {
		batchLen := sum(s.CurrBatchBlocksLen)
		s.BatchSizes = append(s.BatchSizes, batchLen)
		s.CurrBatchBlocksLen = s.CurrBatchBlocksLen[:0]
	}
}

func (s *Header) ByteSizePacked() int { // TODO better not contaminate this file with packing logic
	byteSizeUnpacked := s.ByteSize()
	return byteSizeUnpacked + utils.DivCeil(byteSizeUnpacked*8, PackingSizeU256)
}

// addBlock adds a block to the last batch
func (s *Header) addBlock(blockLen int) {
	s.CurrBatchBlocksLen = append(s.CurrBatchBlocksLen, blockLen)
}

func (s *Header) removeLastBlock() {
	s.CurrBatchBlocksLen = s.CurrBatchBlocksLen[:len(s.CurrBatchBlocksLen)-1]
}

func (s *Header) resetTable() {
	s.BatchSizes = s.BatchSizes[:0]
	s.CurrBatchBlocksLen = s.CurrBatchBlocksLen[:0]
}

// WriteTo writes the header to w. It tentatively considers the current batch as sealed.
func (s *Header) WriteTo(w io.Writer) (int64, error) {
	var written int64

	if err := binary.Write(w, binary.BigEndian, 0xffff-s.Version+1); err != nil {
		return written, err
	}
	written += 2

	// write dictChecksum (32 bytes)
	if _, err := w.Write(s.DictChecksum[:]); err != nil {
		return written, err
	}
	written += 32

	// write nbBatches (uint16)
	nbBatches := uint16(len(s.BatchSizes))
	if len(s.CurrBatchBlocksLen) != 0 {
		nbBatches++
	}

	if err := binary.Write(w, binary.BigEndian, nbBatches); err != nil {
		return written, err
	}
	written += NbElemsEncodingBytes

	writeLen := func(batchLen int) error {
		// write nbBlocksInBatch ("uint24")
		if batchLen < 0 || batchLen >= 1<<24 {
			return fmt.Errorf("batch size out of range [0, 2²⁴)")
		}
		if _, err := w.Write([]byte{byte(batchLen >> 16), byte(batchLen >> 8), byte(batchLen)}); err != nil {
			return err
		}
		written += 3
		return nil
	}

	for _, batchLen := range s.BatchSizes {
		if err := writeLen(batchLen); err != nil {
			return written, err
		}
	}

	if len(s.CurrBatchBlocksLen) != 0 {
		if err := writeLen(sum(s.CurrBatchBlocksLen)); err != nil {
			return written, err
		}
	}

	return written, nil
}

// ReadFrom reads the header BatchSizes from r.
func (s *Header) ReadFrom(r io.Reader) (int64, error) {
	var read int64

	var givenVersion uint16
	if err := binary.Read(r, binary.BigEndian, &givenVersion); err != nil {
		return read, err
	}
	read += 2
	if givenVersion+s.Version != 0 {
		return read, fmt.Errorf("unsupported blob version %d", givenVersion)
	}

	// read dictChecksum (32 bytes)
	if _, err := io.ReadFull(r, s.DictChecksum[:]); err != nil {
		return read, err
	}
	read += 32

	// read nbBatches (uint16)
	var nbBatches uint16

	if err := binary.Read(r, binary.BigEndian, &nbBatches); err != nil {
		return read, err
	}
	read += NbElemsEncodingBytes

	s.BatchSizes = make([]int, nbBatches)

	var buf [3]byte
	for i := uint16(0); i < nbBatches; i++ {
		// read batch dataNbBytes ("uint24")
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return read, err
		}
		read += ByteLenEncodingBytes
		s.BatchSizes[i] = int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	}

	return read, nil
}

func (s *Header) ByteSize() int {
	exCurr := 2 + 32 + NbElemsEncodingBytes + len(s.BatchSizes)*ByteLenEncodingBytes
	if len(s.CurrBatchBlocksLen) != 0 {
		exCurr += ByteLenEncodingBytes
	}
	return exCurr
}

func sum(s []int) int {
	sum := 0
	for _, v := range s {
		sum += v
	}
	return sum
}
