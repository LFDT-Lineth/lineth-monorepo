package v2

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/std/compress/lzss"
	gkrposeidon2 "github.com/consensys/gnark/std/hash/poseidon2/gkr-poseidon2"
	poseidon2permutation "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/config"
	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/publicinput"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/utils/types"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	gcHash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
	"github.com/consensys/linea-monorepo/prover/utils"
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
	config.CircuitSizes

	// The dictionary used in the compression algorithm
	Dict []frontend.Variable

	// The uncompressed and compressed data corresponds to the data that is
	// made available on L1 when we submit a blob of transactions. The circuit
	// proves that these two correspond to the same data. The uncompressed
	// data is then passed to the EVM execution circuit.
	BlobBytes []frontend.Variable

	// The final public input. It is the hash of
	// 	- the hash of the uncompressed message.
	// 	- the hash of the compressed message. aka the SnarkHash on the contract
	// 	- the X and Y evaluation points claimed on the contract
	PublicInput frontend.Variable `gnark:",public"`

	FuncPI FunctionalPublicInputSnark
}

// FunctionalPublicInputQSnark the "unique" portion of the functional public input that cannot be inferred from other circuits in the same aggregation batch
type FunctionalPublicInputQSnark struct {
	// The X and Y evaluation point that are "claimed" by the contract. The
	// circuit will verify that the polynomial describing the compressed data
	// does evaluate to Y when evaluated at point X. This needed to ensure that
	// both the keccak hash of the compressed data (claimed on the contract) and
	// the blob-hash are correct.
	X              [32]frontend.Variable // unreduced value
	Y              [2]frontend.Variable  // Y[1] holds the less significant 16 bytes
	SnarkHash      frontend.Variable
	Eip4844Enabled frontend.Variable
	NbBatches      frontend.Variable
	AllBatchesSum  frontend.Variable
}

type FunctionalPublicInputSnark struct {
	FunctionalPublicInputQSnark
	BatchSums []execution.DataChecksumSnark
}

type FunctionalPublicInput struct {
	X              types.Bls12381Fr
	Y              types.Bls12381Fr
	SnarkHash      []byte
	Eip4844Enabled bool
	BatchSums      []public_input.ExecDataChecksum
	AllBatchesSum  types.Bls12377Fr
}

// Check well-formedness, including range checks.
func (i *FunctionalPublicInputSnark) Check(api frontend.API, sizes config.CircuitSizes) error {
	if len(i.BatchSums) != sizes.MaxNbBatches {
		return fmt.Errorf("batch sums capacity: expected %d, got %d", sizes.MaxNbBatches, len(i.BatchSums))
	}
	for _, sum := range i.BatchSums {
		if err := sum.Check(api); err != nil {
			return err
		}
	}
	api.AssertIsLessOrEqual(i.NbBatches, sizes.MaxNbBatches) // if too big it can turn "negative" and compromise the interconnection logic

	compressor, err := poseidon2permutation.NewCompressor(api)
	if err != nil {
		panic(err)
	}
	batchesToHash := make([]frontend.Variable, len(i.BatchSums))
	for n := range i.BatchSums {
		batchesToHash[n] = i.BatchSums[n].Hash
	}

	api.AssertIsEqual(
		i.AllBatchesSum,
		gnarkutil.SumMerkleDamgardDynamicLength(api, compressor, 0, i.NbBatches, batchesToHash),
	)

	i.FunctionalPublicInputQSnark.RangeCheck(api)

	return nil
}

func (i *FunctionalPublicInputQSnark) RangeCheck(api frontend.API) {
	rc := rangecheck.New(api)
	for j := range i.X {
		rc.Check(i.X[j], 8)
	}
	rc.Check(i.Y[0], 128)
	rc.Check(i.Y[1], 128)
}

func (i *FunctionalPublicInput) ToSnarkType(maxNbBatches int) (FunctionalPublicInputSnark, error) {
	res := FunctionalPublicInputSnark{
		FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
			Y:              [2]frontend.Variable{i.Y[:16], i.Y[16:]},
			SnarkHash:      i.SnarkHash,
			Eip4844Enabled: utils.Ite(i.Eip4844Enabled, 1, 0),
			NbBatches:      len(i.BatchSums),
			AllBatchesSum:  i.AllBatchesSum[:],
		},
	}
	utils.Copy(res.X[:], i.X[:])

	if len(i.BatchSums) > maxNbBatches {
		return res, fmt.Errorf("too many batches: expected at most %d, got %d", maxNbBatches, len(i.BatchSums))
	}

	res.BatchSums = make([]execution.DataChecksumSnark, maxNbBatches)
	return res, executionDataSumsToSnarkType(res.BatchSums, i.BatchSums)
}

func executionDataSumsToSnarkType(dst []execution.DataChecksumSnark, src []public_input.ExecDataChecksum) error {
	if len(dst) < len(src) {
		return fmt.Errorf("too many execution batches - expected at most %d, got %d", len(dst), len(src))
	}
	for i := range src {
		dst[i].Assign(&src[i])
	}
	// pad the rest with zeros
	var zero [1]byte
	padding, err := public_input.NewExecDataChecksum(zero[:])
	if err != nil {
		return err
	}
	for j := len(src); j < len(dst); j++ {
		dst[j].Assign(&padding)
	}
	return nil
}

func (i *FunctionalPublicInput) Sum() ([]byte, error) {
	hsh := gcHash.POSEIDON2_BLS12_377.New()

	// NB! Merkle-Damgard hashes can be range extended.
	// This won't compromise the collision resistance of the sum,
	// as the batches sum is not
	for n := range i.BatchSums {
		hsh.Write(i.BatchSums[n].Hash[:])
	}

	batchesSum := hsh.Sum(nil)
	hsh.Reset()

	hsh.Write(i.X[:16])
	hsh.Write(i.X[16:])
	hsh.Write(i.Y[:16])

	// To eliminate a hash permutation, incorporate Eip4844Enabled flag into Y[1], which
	// is never full
	var buf [17]byte
	copy(buf[:], i.Y[16:])
	buf[16] = utils.Ite(i.Eip4844Enabled, byte(1), 0)
	hsh.Write(buf[:])

	hsh.Write(i.SnarkHash)
	hsh.Write(batchesSum)
	return hsh.Sum(nil), nil
}

// Sum produces the public input
// Sum ignores NbBatches, as its value is expected to be incorporated into batchesSum.
func (i *FunctionalPublicInputQSnark) Sum(api frontend.API) frontend.Variable {
	radix := big.NewInt(256)
	hsh, err := gkrposeidon2.New(api)
	if err != nil {
		panic(err)
	}
	hsh.Reset()
	hsh.Write(
		compress.ReadNum(api, i.X[:16], radix),
		compress.ReadNum(api, i.X[16:], radix),
		i.Y[0],
		api.Add(api.Mul(i.Y[1], 256), i.Eip4844Enabled),
		i.SnarkHash,
		i.AllBatchesSum,
	)
	return hsh.Sum()
}

func (c Circuit) Define(api frontend.API) error {

	if c.CircuitSizes.DictNbBytes != len(c.Dict) {
		return fmt.Errorf("dictionary length mismatch: expected %d, got %d", c.CircuitSizes.DictNbBytes, len(c.Dict))
	}
	if c.CircuitSizes.MaxNbBatches != len(c.FuncPI.BatchSums) {
		return fmt.Errorf("mismatch between maximum number of batches: expected %d from config, got %d from circuit", c.CircuitSizes.MaxNbBatches, len(c.FuncPI.BatchSums))
	}

	hsh, err := gkrposeidon2.New(api)
	if err != nil {
		return err
	}

	// validate the input's form and range
	if err = c.FuncPI.Check(api, c.CircuitSizes); err != nil {
		return err
	}

	blobCrumbs := internal.PackedBytesToCrumbs(api, c.BlobBytes, blob.PackingSizeU256)

	blobPacked377 := internal.PackFull(api, blobCrumbs, 2) // repack into bls12-377 elements to compute a checksum
	hsh.Reset()
	hsh.Write(blobPacked377...)
	api.AssertIsEqual(c.FuncPI.SnarkHash, hsh.Sum())

	// EIP-4844 stuff
	if evaluation, err := publicinput.VerifyBlobConsistency(api, blobCrumbs, c.FuncPI.X, c.FuncPI.Eip4844Enabled); err != nil {
		return err
	} else {
		api.AssertIsEqual(c.FuncPI.Y[0], evaluation[0])
		api.AssertIsEqual(c.FuncPI.Y[1], evaluation[1])
	}

	// repack into bytes TODO possible optimization: pass bits directly to decompressor
	// unpack into bytes
	blobUnpackedBytes, blobUnpackedNbBytes := crumbStreamToByteStream(api, blobCrumbs)

	// get header length, number of batches, and length of each batch
	bytesPerBatch := make([]frontend.Variable, len(c.FuncPI.BatchSums))
	for i := range bytesPerBatch {
		bytesPerBatch[i] = c.FuncPI.BatchSums[i].Length
	}
	headerLen, dictChecksum, nbBatches, err := parseHeader(api, bytesPerBatch, blobUnpackedBytes[:maxBlobNbBytes], blobUnpackedNbBytes)
	if err != nil {
		return err
	}
	api.AssertIsEqual(nbBatches, c.FuncPI.NbBatches)

	// check if the given decompression dictionary checksum matches the one used.
	if err = CheckDictChecksum(api, dictChecksum, c.Dict); err != nil {
		return err
	}

	// decompress the batches
	payload := make([]frontend.Variable, c.MaxUncompressedNbBytes)
	payloadLen, err := lzss.Decompress(
		api,
		compress.ShiftLeft(api, blobUnpackedBytes[:maxBlobNbBytes], headerLen), // TODO Signal to the decompressor that the input is zero padded; to reduce constraint numbers
		api.Sub(blobUnpackedNbBytes, headerLen),
		payload,
		c.Dict,
	)
	if err != nil {
		return err
	}
	api.AssertIsDifferent(payloadLen, -1) // decompression should not fail

	for i := range c.FuncPI.BatchSums {
		if err = c.FuncPI.BatchSums[i].Check(api); err != nil {
			return err
		}
	}

	if err = CheckBatchesPartialSums(api, nbBatches, payload, c.FuncPI.BatchSums); err != nil {
		return err
	}

	publicInput := c.FuncPI.Sum(api)
	api.AssertIsEqual(c.PublicInput, publicInput)

	return nil
}

type builder struct {
	config.CircuitSizes
}

func NewBuilder(cfg config.CircuitSizes) *builder {
	return &builder{cfg}
}

// Compile the decompression circuit
func (b *builder) Compile() (constraint.ConstraintSystem, error) {
	return Compile(b.CircuitSizes)
}

func Compile(sizes config.CircuitSizes) (constraint.ConstraintSystem, error) {
	return frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, Allocate(sizes), frontend.WithCapacity(1<<27))
}

func assignExecutionDataSums(blobUncompressedPayload []byte, batchSizes []int) ([]public_input.ExecDataChecksum, error) {
	var err error
	lastBatchEnd := 0
	res := make([]public_input.ExecDataChecksum, len(batchSizes))
	for i := range batchSizes {
		batchEnd := lastBatchEnd + batchSizes[i]
		if res[i], err = public_input.NewExecDataChecksum(blobUncompressedPayload[lastBatchEnd:batchEnd]); err != nil {
			return nil, err
		}
		lastBatchEnd = batchEnd
	}
	return res, nil
}

func AssignFPI(blobBytes []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (fpi FunctionalPublicInput, dict []byte, err error) {
	if len(blobBytes) != blob.MaxUsableBytes {
		err = fmt.Errorf("decompression circuit assignment : invalid blob length : %d. expected %d", len(blobBytes), blob.MaxUsableBytes)
		return
	}

	r, err := blob.DecompressBlob(blobBytes, dictStore)
	if err != nil {
		return
	}
	dict = r.Dict

	if len(r.RawPayload) > blob.MaxUncompressedBytes {
		err = fmt.Errorf("decompression circuit assignment: blob payload too large : %d. max %d", len(r.RawPayload), blob.MaxUncompressedBytes)
		return
	}

	if fpi.BatchSums, err = assignExecutionDataSums(r.RawPayload, r.Header.BatchSizes); err != nil {
		return
	}

	hsh := gcHash.POSEIDON2_BLS12_377.New()
	for i := range fpi.BatchSums {
		hsh.Write(fpi.BatchSums[i].Hash[:])
	}
	copy(fpi.AllBatchesSum[:], hsh.Sum(nil))

	fpi.X = x
	yBytes := y.Bytes()
	fpi.Y = types.AsBls12381Fr(yBytes[:])

	fpi.Eip4844Enabled = eip4844Enabled

	if len(blobBytes) != 128*1024 {
		panic("blobBytes length is not 128*1024")
	}
	fpi.SnarkHash, err = encode.Poseidon2ChecksumPackedData(blobBytes, fr381.Bits-1, encode.NoTerminalSymbol()) // TODO if forced to remove the above check, pad with zeros

	return
}

func Assign(sizes config.CircuitSizes, blobBytes []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (assignment frontend.Circuit, publicInput fr377.Element, snarkHash []byte, err error) {
	fpi, dict, err := AssignFPI(blobBytes, dictStore, eip4844Enabled, x, y)
	if err != nil {
		return
	}
	snarkHash = fpi.SnarkHash

	pi, err := fpi.Sum()
	if err != nil {
		return
	}
	if err = publicInput.SetBytesCanonical(pi); err != nil {
		return
	}

	sfpi, err := fpi.ToSnarkType(sizes.MaxNbBatches)
	if err != nil {
		return
	}

	assignment = &Circuit{
		Dict:        utils.ToVariableSlice(dict),
		BlobBytes:   utils.ToVariableSlice(blobBytes),
		PublicInput: publicInput,
		FuncPI:      sfpi,
	}

	return
}

func Allocate(sizes config.CircuitSizes) *Circuit {
	return &Circuit{
		CircuitSizes: sizes,
		Dict:         make([]frontend.Variable, sizes.DictNbBytes),
		BlobBytes:    make([]frontend.Variable, blob.MaxUsableBytes),
		FuncPI: FunctionalPublicInputSnark{
			BatchSums: make([]execution.DataChecksumSnark, sizes.MaxNbBatches),
		},
	}
}
