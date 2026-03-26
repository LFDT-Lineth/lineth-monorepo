package blobsubmission

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/linea-monorepo/prover/utils"
	gokzg4844 "github.com/crate-crypto/go-kzg-4844"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/stretchr/testify/assert"
)

const (
	_inFile                = "./samples/sample0.json"
	_outFile               = "./samples/sample0-response.json"
	_inFileEIP4844         = "./samples/sample1.json"
	_outFileEIP4844        = "./samples/sample1-response.json"
	_inFileEIP4844MaxSize  = "./samples/sample-max-size.json"
	_outFileEIP4844MaxSize = "./samples/sample-max-size-response.json"
	_inFileEIP4844Empty    = "./samples/sample-empty.json"
	_outFileEIP4844Empty   = "./samples/sample-empty-response.json"
	_inFileEIP4844TooLarge = "./samples/sample-too-large.json"
)

// blobsubmission with callData
// eip4844Enabled=false
func TestBlobSubmission(t *testing.T) {
	fIn, err := os.Open(_inFile)
	if err != nil {
		t.Fatalf("could not open %s: %v", _inFile, err)
	}
	defer fIn.Close()

	fOut, err := os.Open(_outFile)
	if err != nil {
		t.Fatalf("could not open %s: %v", _outFile, err)
	}
	defer fOut.Close()

	var (
		inp         Request
		outExpected Response
	)

	if err = json.NewDecoder(fIn).Decode(&inp); err != nil {
		t.Fatalf("could not decode %++v: %v", inp, err)
	}

	if err = json.NewDecoder(fOut).Decode(&outExpected); err != nil {
		t.Fatalf("could not decode %++v: %v", outExpected, err)
	}

	// call CraftResponseCalldata()
	out, err := CraftResponse(&inp)

	ok := assert.NoErrorf(t, err, "could not craft the response: %v", err)
	if ok {
		assert.Equal(t, outExpected, *out, "the response file should be the same")
	}

	// Stop the test after the first failed file to not overwhelm the
	// logs.
	if t.Failed() {
		t.Fatalf("Got errors for file %s, stopping the test", _outFile)
	}
}

// eip4844 blob submission
// eip4844Enabled = true
func TestBlobSubmissionEIP4844(t *testing.T) {
	fIn, err := os.Open(_inFileEIP4844)
	if err != nil {
		t.Fatalf("could not open %s: %v", _inFileEIP4844, err)
	}
	defer fIn.Close()

	fOut, err := os.Open(_outFileEIP4844)
	if err != nil {
		t.Fatalf("could not open %s: %v", _outFileEIP4844, err)
	}
	defer fOut.Close()

	var (
		inp         Request
		outExpected Response
	)

	if err = json.NewDecoder(fIn).Decode(&inp); err != nil {
		t.Fatalf("could not decode %++v: %v", inp, err)
	}

	if err = json.NewDecoder(fOut).Decode(&outExpected); err != nil {
		t.Fatalf("could not decode %++v: %v", outExpected, err)
	}

	out, err := CraftResponse(&inp)

	ok := assert.NoErrorf(t, err, "could not craft the response: %v", err)
	if ok {
		assert.Equal(t, outExpected, *out, "the response file should be the same")
	}

	// Stop the test after the first failed file to not overwhelm the
	// logs.
	if t.Failed() {
		t.Fatalf("Got errors for file %s, stopping the test", _outFileEIP4844)
	}
}

// empty blob
func TestBlobSubmissionEIP4844EmptyBlob(t *testing.T) {
	fIn, err := os.Open(_inFileEIP4844Empty)
	if err != nil {
		t.Fatalf("could not open %s: %v", _inFileEIP4844Empty, err)
	}
	defer fIn.Close()

	fOut, err := os.Open(_outFileEIP4844Empty)
	if err != nil {
		t.Fatalf("could not open %s: %v", _outFileEIP4844Empty, err)
	}
	defer fOut.Close()

	var (
		inp         Request
		outExpected Response
	)

	if err = json.NewDecoder(fIn).Decode(&inp); err != nil {
		t.Fatalf("could not decode %++v: %v", inp, err)
	}

	if err = json.NewDecoder(fOut).Decode(&outExpected); err != nil {
		t.Fatalf("could not decode %++v: %v", outExpected, err)
	}

	compressedStream, _ := b64.DecodeString(inp.CompressedData)
	// Check if len(compressedStream) is equal to 0
	expectedLength := 0
	actualLength := len(compressedStream)
	assert.Equal(t, expectedLength, actualLength, "compressed stream length mismatch")

	out, err := CraftResponse(&inp)

	ok := assert.NoErrorf(t, err, "could not craft the response: %v", err)
	if ok {
		assert.Equal(t, outExpected, *out, "the response file should be the same")
	}

	// Stop the test after the first failed file to not overwhelm the
	// logs.
	if t.Failed() {
		t.Fatalf("Got errors for file %s, stopping the test", _outFileEIP4844)
	}
}

// random blob with [131072] bytes
func TestBlobSubmissionEIP4844MaxSize(t *testing.T) {
	fIn, err := os.Open(_inFileEIP4844MaxSize)
	if err != nil {
		t.Fatalf("could not open %s: %v", _inFileEIP4844MaxSize, err)
	}
	defer fIn.Close()

	fOut, err := os.Open(_outFileEIP4844MaxSize)
	if err != nil {
		t.Fatalf("could not open %s: %v", _outFileEIP4844MaxSize, err)
	}
	defer fOut.Close()

	var (
		inp         Request
		outExpected Response
	)

	if err = json.NewDecoder(fIn).Decode(&inp); err != nil {
		t.Fatalf("could not decode %++v: %v", inp, err)
	}

	if err = json.NewDecoder(fOut).Decode(&outExpected); err != nil {
		t.Fatalf("could not decode %++v: %v", _outFileEIP4844MaxSize, err)
	}

	compressedStream, _ := b64.DecodeString(inp.CompressedData)
	// Check if len(compressedStream) is equal to 131072
	expectedLength := 131072
	actualLength := len(compressedStream)
	assert.Equal(t, expectedLength, actualLength, "compressed stream length mismatch")

	out, err := CraftResponse(&inp)

	ok := assert.NoErrorf(t, err, "could not craft the response: %v", err)
	if ok {
		assert.Equal(t, outExpected, *out, "the response file should be the same")
	}

	// Stop the test after the first failed file to not overwhelm the
	// logs.
	if t.Failed() {
		t.Fatalf("Got errors for file %s, stopping the test", _outFileEIP4844MaxSize)
	}
}

// blob too large error with [131072 + 32] bytes
func TestBlobSubmissionEIP4844BlobTooLarge(t *testing.T) {

	var blob kzg4844.Blob
	fIn, err := os.Open(_inFileEIP4844TooLarge)
	if err != nil {
		t.Fatalf("could not open %s: %v", _inFileEIP4844TooLarge, err)
	}
	defer fIn.Close()

	var (
		inp Request
	)

	if err = json.NewDecoder(fIn).Decode(&inp); err != nil {
		t.Fatalf("could not decode %++v: %v", inp, err)
	}

	_, err = CraftResponse(&inp)

	// Check if err is not nil
	if err == nil {
		t.Errorf("expected an error `compressedStream length exceeds blob length (131072)`, but got nil")
		return
	}

	compressedStream, _ := b64.DecodeString(inp.CompressedData)
	// Check if the error message contains the expected substring
	expectedErrorMsg := fmt.Sprintf("compressedStream length (%d) exceeds blob length (%d)", len(compressedStream), len(blob))
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("expected error message to contain %q, got %q", expectedErrorMsg, err.Error())
	}

}

// a random blob from kzg4844 package and xPoint from evaluationChallenge()
func TestKZGWithPoint(t *testing.T) {

	blobBytes := randBlob()

	// Also zeroize all the bytes that are at position multiples of 32. This
	// ensures that we will not have overflow when casting to the bls12 scalar
	// field.
	n := len(blobBytes)
	n = utils.DivCeil(n, 32) * 32 // round up to the next multiple of 32
	for i := 0; i < n; i += 32 {
		blobBytes[i] = 0
	}

	commitment, err := kzg4844.BlobToCommitment(&blobBytes)
	if err != nil {
		t.Fatalf("failed to create KZG commitment from blob: %v", err)
	}

	// blobHash
	blobHash := kzg4844.CalcBlobHashV1(sha256.New(), &commitment)
	if !kzg4844.IsValidVersionedHash(blobHash[:]) {
		t.Fatalf("crafting response: invalid versionedHash (blobHash, dataHash):  %v", err)
	}

	// Compute all the prover fields
	snarkHash, err := encode.Poseidon2ChecksumPackedData(blobBytes[:], fr381.Bits-1, encode.NoTerminalSymbol())
	assert.NoError(t, err)

	xUnreduced := evaluationChallenge(snarkHash, blobHash[:])
	var tmp fr381.Element
	tmp.SetBytes(xUnreduced[:])
	xPoint := kzg4844.Point(tmp.Bytes())

	proof, claim, err := kzg4844.ComputeProof(&blobBytes, xPoint)
	if err != nil {
		t.Fatalf("failed to create KZG proof at point: %v", err)
	}
	if err := kzg4844.VerifyProof(commitment, xPoint, claim, proof); err != nil {
		t.Fatalf("failed to verify KZG proof at point: %v", err)
	}
}

// a random blob and xPoint from kzg4844 package
func TestKZGWithPointSimple(t *testing.T) {

	blob := randBlob()

	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		t.Fatalf("failed to create KZG commitment from blob: %v", err)
	}
	xPoint := randFieldElement()
	proof, claim, err := kzg4844.ComputeProof(&blob, xPoint)
	if err != nil {
		t.Fatalf("failed to create KZG proof at point: %v", err)
	}
	if err := kzg4844.VerifyProof(commitment, xPoint, claim, proof); err != nil {
		t.Fatalf("failed to verify KZG proof at point: %v", err)
	}
}

// simple kzg from kzg4844 package
func TestKZGWithBlob(t *testing.T) {

	blob := randBlob()

	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		t.Fatalf("failed to create KZG commitment from blob: %v", err)
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		t.Fatalf("failed to create KZG proof for blob: %v", err)
	}
	if err := kzg4844.VerifyBlobProof(&blob, commitment, proof); err != nil {
		t.Fatalf("failed to verify KZG proof for blob: %v", err)
	}
}

// randFieldElement() from kzg4844 package
func randFieldElement() [32]byte {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		panic("failed to get random field element")
	}
	var r fr381.Element
	r.SetBytes(bytes)

	return gokzg4844.SerializeScalar(r)
}

// randBlob() from kzg4844 package
func randBlob() kzg4844.Blob {
	var blob kzg4844.Blob
	for i := 0; i < len(blob); i += gokzg4844.SerializedScalarSize {
		fieldElementBytes := randFieldElement()
		copy(blob[i:i+gokzg4844.SerializedScalarSize], fieldElementBytes[:])
	}
	return blob
}
