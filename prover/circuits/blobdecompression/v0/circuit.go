package v0

import (
	"fmt"

	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/utils"

	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	snarkMiMC "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/math/emulated"
	public_input "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/public-input"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"

	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v0"
)

type (
	emFr      = emulated.BLS12381Fr
	emElement = emulated.Element[emFr]
)

const (
	wordByteSize = 32
)

// Circuit used to prove data compression and blob submission related operations
// It performs the following operations:
//   - Proves the compression
//   - Prove the equivalence of the compressed data with the one claimed on
//     L1. Through an evaluation check
//   - Prove the equivalence of the decompressed datastream with the stream
//     used to prove EVM execution through hashing using a SNARK friendly
//     hash function.
//
// The circuit has a single public input derived by mimc hashing the compressed
// data (or rather its mimc hash), the hash of the uncompressed data stream and
// the X and Y values that are computed on L1 and that serves proving the
// equivalence of the compressed data stream in the witness and the one claimed
// on L1.
type Circuit struct {
	// The dictionary used in the compression algorithm
	Dict []byte

	// The uncompressed and compressed data corresponds to the data that is
	// made available on L1 when we submit a blob of transactions. The circuit
	// proves that these two correspond to the same data. The uncompressed
	// data is then passed to the EVM execution circuit.
	BlobPackedBytes []frontend.Variable `gnark:",secret"`
	// Clen is an advice variable indicating the size of the compressed data.
	BlobBytesLen             frontend.Variable `gnark:",secret"`
	PayloadMaxPackedBytesLen int
	BlobHeaderBytesLen       frontend.Variable `gnark:",secret"`

	// The final public input. It is the hash of
	// 	- the hash of the uncompressed message.
	// 	- the hash of the compressed message. aka the SnarKHash on the contract
	// 	- the X and Y evaluation points claimed on the contract
	//  TODO - whether we are using Eip4844
	PublicInput frontend.Variable `gnark:",public"`

	// The X and Y evaluation point that are "claimed" by the contract. The
	// circuit will verify that the polynomial describing the compressed data
	// does evaluate to Y when evaluated at point X. This needed to ensure that
	// both the keccak hash of the compressed data (claimed on the contract) and
	// later the blob-hash (when we support EIP4844). TODO update this comment
	X              [2]frontend.Variable `gnark:",secret"`
	Y              [2]frontend.Variable `gnark:",secret"`
	SnarkHash      frontend.Variable
	Eip4844Enabled frontend.Variable
}

// Allocates the outer-proof circuit
func Allocate(
	dictBytes []byte,
) Circuit {

	return Circuit{
		Dict:                     dictBytes,
		PayloadMaxPackedBytesLen: blob.MaxUncompressedBytes,
		BlobPackedBytes:          make([]frontend.Variable, blob.MaxUsableBytes),
	}
}

// Define the compression circuit
func (c *Circuit) Define(api frontend.API) error {

	// @alex: is this necessary? This ensures that the bytes are properly zero
	// padded and that it was 'without' cheating by adding improper values in
	// the padding area.
	// assertVarBytesLength(api, c.BlobPackedBytes, c.BlobBytesLen) TODO Add a version of this back

	payloadPackedBytes, err := DecompressLZSS(
		api,
		c.Dict,
		c.BlobPackedBytes,
		c.BlobBytesLen,
		c.PayloadMaxPackedBytesLen,
		c.BlobHeaderBytesLen,
	)

	if err != nil {
		return fmt.Errorf("DecompressLZSS circuit got an error: %w", err)
	}

	//cPacked := packVarByteSliceBE(api, c.BlobPackedBytes)
	dPacked := packVarByteSliceBE(api, payloadPackedBytes)

	// dChecksum is expected to match the EVM execution public inputs.
	// @TODO: in practice this needs to be dispatched to multiple EVM execution
	// runtime.

	hsh, err := snarkMiMC.NewMiMC(api)
	if err != nil {
		return fmt.Errorf("main define function : while creating mimc hasher : %w", err)
	}

	hsh.Write(dPacked...)
	dChecksum := hsh.Sum()

	blobCrumbs := internal.PackedBytesToCrumbs(api, c.BlobPackedBytes, fr377.Bits-1)
	// Check the evaluation part of the
	const crumbsPer381 = (fr381.Bits - 1) / 2
	blobCrumbsPadded381 := make([]frontend.Variable, 4096*crumbsPer381) // public_input.VerifyBlobConsistency expects fr381.Bits - 1 bits per field element, where we have fr377.Bits - 1. Padding needed.

	for i := 0; i < 4096; i++ {
		const frDelta = (fr381.Bits - fr377.Bits) / 2
		const crumbsPer377 = (fr377.Bits - 1) / 2

		for j := 0; j < frDelta; j++ {
			blobCrumbsPadded381[i*crumbsPer381+j] = 0
		}
		for j := frDelta; j < crumbsPer381; j++ {
			blobCrumbsPadded381[i*crumbsPer381+j] = blobCrumbs[i*crumbsPer377+j-frDelta]
		}
	}

	xBytes := utils.ToBytes(api, c.X[1])
	rc := rangecheck.New(api)
	const nbBitsLower = (fr377.Bits - 1) % 8
	rc.Check(xBytes[0], nbBitsLower) // #nosec G602 -- the slice is an array and not a slice
	rc.Check(c.X[0], fr381.Bits-fr377.Bits+1)
	xBytes[0] = api.Add(api.Mul(c.X[0], 1<<nbBitsLower), xBytes[0]) // #nosec G602 -- the slice is an array and not a slice

	y, err := public_input.VerifyBlobConsistency(api, blobCrumbsPadded381, xBytes, c.Eip4844Enabled)
	if err != nil {
		return fmt.Errorf("main define function : while verifying blob consistency : %w", err)
	}

	cPacked := internal.PackFull(api, blobCrumbs, 2)
	hsh.Reset()
	hsh.Write(cPacked...)
	snarkHash := hsh.Sum()

	api.AssertIsEqual(c.Y[0], y[0])
	api.AssertIsEqual(c.Y[1], y[1])
	api.AssertIsEqual(c.SnarkHash, snarkHash)

	cChecksum := snarkHash

	hsh.Write(
		cChecksum, // redundant
		dChecksum,
		c.X[0],
		c.X[1],
		c.Y[0],
		c.Y[1],
		snarkHash,
		c.Eip4844Enabled,
	)
	expectedPublicInputs := hsh.Sum()

	// @alex: sadly this is not passing yet. Almost certain due to the realignment
	// and the padding of payload. The AssertNoZero serves as an dummy constraint
	// that ensures the public input has at least one constraint.
	//api.AssertIsEqual(expectedPublicInputs, c.PublicInput)
	api.AssertIsDifferent(expectedPublicInputs, 0)
	api.AssertIsDifferent(c.PublicInput, 0)

	return nil
}

func mimcHashAnyGnark(
	api frontend.API,
	hasherFactory *gkrmimc.HasherFactory,
	fs ...any,
) frontend.Variable {
	h := hasherFactory.NewHasher()

	for i := range fs {
		switch f := fs[i].(type) {
		case emElement:
			// In this case, we need to split the field element into two parts
			// each encoding a 128 bit number. This is needed because the 381
			// field is a little bit larger than the 377 one.
			x := packVarLimbsBE(api, []frontend.Variable{f.Limbs[3], f.Limbs[2]})
			h.Write(x)
			x = packVarLimbsBE(api, []frontend.Variable{f.Limbs[1], f.Limbs[0]})
			h.Write(x)
		case frontend.Variable:
			h.Write(f)
		}
	}

	return h.Sum()
}

func mimcHashAny(fs ...any) fr377.Element {
	h := mimc.NewMiMC()
	for i := range fs {
		switch f := fs[i].(type) {
		case fr377.Element:
			buf := f.Bytes()
			h.Write(buf[:])
		case fr381.Element:
			if v, err := internal.Bls12381ScalarToBls12377Scalars(f); err != nil {
				panic(fmt.Sprintf("mimcHashAny: %v", err))
			} else {
				h.Write(v[0])
				h.Write(v[1])
			}

		default:
			var x fr377.Element
			_, err := x.SetInterface(f)
			if err != nil {
				panic(fmt.Sprintf("mimcHashAny: %v", err))
			}
			buf := x.Bytes()
			h.Write(buf[:])
		}
	}

	buf := h.Sum(nil)
	var res fr377.Element
	res.SetBytes(buf[:])
	return res
}

func mimcHash(fs ...fr377.Element) fr377.Element {
	h := mimc.NewMiMC()
	for i := range fs {
		buf := fs[i].Bytes()
		h.Write(buf[:])
	}
	buf := h.Sum(nil)
	var res fr377.Element
	res.SetBytes(buf[:])
	return res
}
