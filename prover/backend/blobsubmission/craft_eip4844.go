package blobsubmission

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

// Prepare a response object by computing all the fields except for the proof.
func CraftResponse(req *Request) (*Response, error) {
	if req == nil {
		return nil, errors.New("crafting response: request must not be nil")
	}

	// if req.Eip4844Enabled == false
	// craft the legacy calldata response instead of EIP4844
	if !req.Eip4844Enabled {
		return CraftResponseCalldata(req)
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
		ParentStateRootHash: utils.HexEncodeToString(parentZkRootHash),
		FinalStateRootHash:  utils.HexEncodeToString(newZkRootHash),
		DataParentHash:      req.DataParentHash,
		PrevShnarf:          utils.HexEncodeToString(prevShnarf),
		Eip4844Enabled:      req.Eip4844Enabled, // this is guaranteed to be true
	}

	// copy compressedStream to kzg48484 blobPadded type
	// check boundary conditions and add padding if necessary
	blobPadded, err := compressedStreamToBlob(compressedStream)
	if err != nil {
		formatStr := "crafting response: compressedStreamToBlob:  %w"
		return nil, fmt.Errorf(formatStr, err)
	}

	// BlobToCommitment creates a commitment out of a data blob.
	commitment, err := kzg4844.BlobToCommitment(&blobPadded)
	if err != nil {
		formatStr := "crafting response: BlobToCommitment:  %w"
		return nil, fmt.Errorf(formatStr, err)
	}

	// blobHash
	blobHash := kzg4844.CalcBlobHashV1(sha256.New(), &commitment)
	if !kzg4844.IsValidVersionedHash(blobHash[:]) {
		formatStr := "crafting response: invalid versionedHash (blobHash, dataHash):  %w"
		return nil, fmt.Errorf(formatStr, err)
	}

	// Compute all the prover fields
	snarkHash, err := encode.Poseidon2ChecksumPackedData(append(compressedStream, make([]byte, blob.MaxUsableBytes-len(compressedStream))...), fr381.Bits-1, encode.NoTerminalSymbol())
	if err != nil {
		return nil, fmt.Errorf("crafting response: could not compute snark hash: %w", err)
	}

	// ExpectedX
	// Perform the modular reduction before passing to `ComputeProof`. That's needed because ComputeProof expects a reduced
	// x point and our x point comes out of Keccak. Thus, it has no reason to be a valid field element as is.
	// importantly, do not use `SetByteCanonical` as it will return an error because it expects a reduced input
	xUnreduced := evaluationChallenge(snarkHash, blobHash[:])
	var tmp fr381.Element
	tmp.SetBytes(xUnreduced[:])
	xPoint := kzg4844.Point(tmp.Bytes())

	// KZG Proof Contract
	kzgProofContract, yClaim, err := kzg4844.ComputeProof(&blobPadded, xPoint)
	if err != nil {
		formatStr := "kzgProofContract: kzg4844.ComputeProof error:  %w"
		return nil, fmt.Errorf(formatStr, err)
	}

	// ExpectedY
	// A claimed evaluation value in a specific point.
	y := make([]byte, len(yClaim))
	copy(y[:], yClaim[:])

	// KZG Proof Sidecar
	kzgProofSidecar, err := kzg4844.ComputeBlobProof(&blobPadded, commitment)
	if err != nil {
		formatStr := "kzgProofSidecar: kzg4844.ComputeBlobProof error:  %w"
		return nil, fmt.Errorf(formatStr, err)
	}

	// newShnarf
	parts := Shnarf{
		OldShnarf:        prevShnarf,
		SnarkHash:        snarkHash,
		NewStateRootHash: newZkRootHash,
		X:                xUnreduced,
	}
	if err = parts.Y.SetBytesCanonical(y); err != nil {
		return nil, err
	}
	newShnarf := parts.Compute()

	// Assign all the fields in the input

	// We return the unpadded blob-data and leave the coordinator the responsibility
	// to perform the padding operation.
	resp.CompressedData = req.CompressedData
	resp.Commitment = utils.HexEncodeToString(commitment[:])
	resp.KzgProofContract = utils.HexEncodeToString(kzgProofContract[:])
	resp.KzgProofSidecar = utils.HexEncodeToString(kzgProofSidecar[:])
	resp.DataHash = utils.HexEncodeToString(blobHash[:])
	resp.SnarkHash = utils.HexEncodeToString(snarkHash)
	resp.ExpectedX = utils.HexEncodeToString(xUnreduced)
	resp.ExpectedY = utils.HexEncodeToString(y)
	resp.ExpectedShnarf = utils.HexEncodeToString(newShnarf)

	return resp, nil
}

// Blob is populated with the compressedStream (with padding)
func compressedStreamToBlob(compressedStream []byte) (blob kzg4844.Blob, err error) {
	// Error is returned when len(compressedStream) is larger than the 4844 data blob [131072]byte
	if len(compressedStream) > len(blob) {
		return blob, fmt.Errorf("compressedStream length (%d) exceeds blob length (%d)", len(compressedStream), len(blob))
	}

	// Copy compressedStream to blob, padding with zeros
	copy(blob[:len(compressedStream)], compressedStream)

	// Sanity-check that the blob is right-padded with zeroes
	for i := len(compressedStream); i < len(blob); i++ {
		if blob[i] != 0 {
			utils.Panic("blob not padded correctly at index blob[%d]", i)
		}
	}
	return blob, nil

}
