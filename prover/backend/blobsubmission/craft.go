package blobsubmission

import (
	"encoding/base64"
	"errors"
	"fmt"
	"hash"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
	"golang.org/x/crypto/sha3"
)

var b64 = base64.StdEncoding

// Prepare a response object by computing all the fields except for the proof.
func CraftResponseCalldata(req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("crafting response: request must not be nil")
	}

	// Flat pass the request parameters to the response
	var (
		errs             [4]error
		parentZkRootHash []byte
		newZkRootHash    []byte
		prevShnarf       []byte
		compressedStream []byte
	)

	// Validate the request parameters
	parentZkRootHash, errs[0] = utils.HexDecodeString(req.ParentStateRootHash)
	newZkRootHash, errs[1] = utils.HexDecodeString(req.FinalStateRootHash)
	prevShnarf, errs[2] = utils.HexDecodeString(req.PrevShnarf)
	compressedStream, errs[3] = b64.DecodeString(req.CompressedData)

	// Collect and wrap the errors if any, so that we get a friendly error message
	if errors.Join(errs[:]...) != nil {
		errsFiltered := make([]error, 0)
		if errs[0] != nil {
			errsFiltered = append(errsFiltered, fmt.Errorf("bad parent zk root hash: %w", errs[0]))
		}
		if errs[1] != nil {
			errsFiltered = append(errsFiltered, fmt.Errorf("bad new zk root hash: %w", errs[1]))
		}
		if errs[2] != nil {
			errsFiltered = append(errsFiltered, fmt.Errorf("bad previous shnarf: %w", errs[2]))
		}
		if errs[3] != nil {
			errsFiltered = append(errsFiltered, fmt.Errorf("bad compressed data: %w", errs[3]))
		}
		return nil, fmt.Errorf("crafting response:\n%w", errors.Join(errsFiltered...))
	}

	resp := &Response{
		ConflationOrder: req.ConflationOrder,
		// Reencode all the parameters to ensure that they are in 0x prefixed format
		CompressedData:      b64.EncodeToString(compressedStream),
		ParentStateRootHash: utils.HexEncodeToString(parentZkRootHash),
		FinalStateRootHash:  utils.HexEncodeToString(newZkRootHash),
		DataParentHash:      req.DataParentHash,
		PrevShnarf:          utils.HexEncodeToString(prevShnarf),
		Eip4844Enabled:      req.Eip4844Enabled, // this is guaranteed to be false
		// Pass an the hex for an empty commitments and proofs instead of passing
		// empty string so that the response is always a valid hex string.
		KzgProofContract: "0x",
		KzgProofSidecar:  "0x",
		Commitment:       "0x",
	}

	// Compute all the prover fields
	snarkHash, err := encode.Poseidon2ChecksumPackedData(compressedStream, fr381.Bits-1, encode.NoTerminalSymbol())
	if err != nil {
		return nil, fmt.Errorf("crafting response: could not compute snark hash: %w", err)
	}

	keccakHash := utils.KeccakHash(compressedStream)
	x := evaluationChallenge(snarkHash, keccakHash)
	y, err := EvalStream(compressedStream, x)
	if err != nil {
		errorMsg := "crafting response: could not compute y: %w"
		return nil, fmt.Errorf(errorMsg, err)
	}

	parts := Shnarf{
		OldShnarf:        prevShnarf,
		SnarkHash:        snarkHash,
		NewStateRootHash: newZkRootHash,
		Y:                y,
		X:                x,
	}
	newShnarf := parts.Compute()

	// Assign all the fields in the input
	resp.DataHash = utils.HexEncodeToString(keccakHash)
	resp.SnarkHash = utils.HexEncodeToString(snarkHash)
	xBytes, yBytes := x, y.Bytes()
	resp.ExpectedX = utils.HexEncodeToString(xBytes)
	resp.ExpectedY = utils.HexEncodeToString(yBytes[:])
	resp.ExpectedShnarf = utils.HexEncodeToString(newShnarf)

	return resp, nil
}

// Returns an evaluation challenge point from a SNARK hash and a blob hash. The
// evaluation challenge is obtained as the hash of the SnarkHash and the keccak
// hash (or the blob hash once we go EIP4844) in that order. The digest is
// returned as a field element modulo the scalar field of the curve BLS12-381.
func evaluationChallenge(snarkHash, keccakHash []byte) (x []byte) {

	// Use the keccak hash
	h := sha3.NewLegacyKeccak256()
	h.Write(snarkHash)
	h.Write(keccakHash)
	d := h.Sum(nil)
	return d
}

// Cast the list of the field into a vector of field elements and performs a
// polynomial evaluation in the scalar field of BLS12-381. The bytes are split
// in chunks of 31 bytes representing each a field element in bigendian order.
// The last chunk is padded to the right with zeroes. The input x is taken as
// an array of 32 bytes because the smart-contract generating it will be using
// the result of the keccak directly. The modular reduction is implicitly done
// during the evaluation of the compressed data polynomial representation.
func EvalStream(stream []byte, x_ []byte) (fr381.Element, error) {
	streamLen := len(stream)

	const chunkSize = 32
	var p, x, y fr381.Element

	x.SetBytes(x_)

	if streamLen%chunkSize != 0 {
		return fr381.Element{}, fmt.Errorf("stream length must be a multiple of 32; received length: %d", streamLen)
	}

	// Compute y by the Horner method. NB: y is initialized to zero when
	// allocated but not assigned.
	for k := streamLen; k > 0; k -= chunkSize {
		if k < len(stream) {
			y.Mul(&y, &x)
		}
		start, stop := k-chunkSize, k
		if err := p.SetBytesCanonical(stream[start:stop]); err != nil {
			return fr381.Element{}, fmt.Errorf("stream is invalid: %w", err)
		}
		y.Add(&y, &p)
	}

	return y, nil
}

// schnarfParts wrap the arguments needed to create a new Shnarf by calling
// the NewSchnarf() function.
type Shnarf struct {
	OldShnarf, SnarkHash, NewStateRootHash []byte
	X                                      []byte
	Y                                      fr381.Element
	Hash                                   hash.Hash
}

// Compute returns the new shnarf given, the old shnarf, the snark hash, the new state
// root hash, the x and the y in that order. All over 32 bytes.
func (s *Shnarf) Compute() []byte {
	if s.Hash == nil {
		s.Hash = sha3.NewLegacyKeccak256()
	}
	yBytes := s.Y.Bytes()
	s.Hash.Reset()
	s.Hash.Write(s.OldShnarf)
	s.Hash.Write(s.SnarkHash)
	s.Hash.Write(s.NewStateRootHash)
	s.Hash.Write(s.X)
	s.Hash.Write(yBytes[:])

	return s.Hash.Sum(nil)
}
