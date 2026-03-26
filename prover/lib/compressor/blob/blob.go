package blob

import (
	"bytes"
	"errors"
	"os"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	v0 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	v2 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
	"github.com/ethereum/go-ethereum/rlp"
)

func GetVersion(blob []byte) uint16 {
	if len(blob) < 3 {
		return 0
	}

	// packed two bytes into 254 usable bits of an BLS12-381 scalar
	if blob[0] == 0x3f && blob[1] == 0xff { // upper 14 bits suggest a "small" version
		return 4 - uint16(blob[2]>>6) // the remaining two bits determine the version (1 through 4 are possible)
	}

	return 0
}

func LoadDict(dictPath string) ([]byte, error) {
	return os.ReadFile(dictPath)
}

// DecompressBlob takes in a Linea blob and outputs an RLP encoded list of RLP encoded blocks.
// Due to information loss during pre-compression encoding, two pieces of information are represented "hackily":
// The block hash is in the ParentHash field.
// The transaction from address is in the signature.R field.
func DecompressBlob(blob []byte, dictStore dictionary.Store) ([]byte, error) {
	vsn := GetVersion(blob)
	var (
		blockDecoder func(*bytes.Reader) (encode.DecodedBlockData, error)
		blocks       [][]byte
		err          error
	)
	switch vsn {
	case 0:
		_, _, blocks, err = v0.DecompressBlob(blob, dictStore)
		blockDecoder = v0.DecodeBlockFromUncompressed
	case 1:
		r, _err := v1.DecompressBlob(blob, dictStore)
		blocks = r.Blocks
		err = _err
		blockDecoder = v1.DecodeBlockFromUncompressed
	case 2:
		r, _err := v2.DecompressBlob(blob, dictStore)
		blocks = r.Blocks
		err = _err
		blockDecoder = v1.DecodeBlockFromUncompressed
	default:
		return nil, errors.New("unrecognized blob version")
	}

	if err != nil {
		return nil, err
	}
	blocksSerialized := make([][]byte, len(blocks))
	var decodedBlock encode.DecodedBlockData
	for i, block := range blocks {
		if decodedBlock, err = blockDecoder(bytes.NewReader(block)); err != nil {
			return nil, err
		}
		if blocksSerialized[i], err = rlp.EncodeToBytes(decodedBlock.ToStd()); err != nil {
			return nil, err
		}
	}
	return rlp.EncodeToBytes(blocksSerialized)
}
