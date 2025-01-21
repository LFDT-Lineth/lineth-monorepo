package v1

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	gcHash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/compress"
	snarkHash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/rangecheck"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"

	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

// TODO make as many things package private as possible

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
	Dict []frontend.Variable

	// The uncompressed and compressed data corresponds to the data that is
	// made available on L1 when we submit a blob of transactions. The circuit
	// proves that these two correspond to the same data. The uncompressed
	// data is then passed to the EVM execution circuit.
	BlobBytes []frontend.Variable

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
	FuncPI FunctionalPublicInputSnark

	MaxBlobPayloadNbBytes int
	UseGkrMiMC            bool
}

// FunctionalPublicInputQSnark the "unique" portion of the functional public input that cannot be inferred from other circuits in the same aggregation batch
type FunctionalPublicInputQSnark struct {
	Y              [2]frontend.Variable // Y[1] holds 252 bits
	SnarkHash      frontend.Variable
	Eip4844Enabled frontend.Variable
	NbBatches      frontend.Variable
	X              [32]frontend.Variable // unreduced value
}

type FunctionalPublicInputSnark struct {
	FunctionalPublicInputQSnark
	BatchSums [MaxNbBatches]frontend.Variable
}

type FunctionalPublicInput struct {
	X              [32]byte  // X[1] holds 252 bits
	Y              [2][]byte // Y[1] holds 252 bits
	SnarkHash      []byte
	Eip4844Enabled bool
	BatchSums      [][]byte
}

// RangeCheck checks that values are within range
func (i *FunctionalPublicInputQSnark) RangeCheck(api frontend.API) {
	rc := rangecheck.New(api)
	for j := range i.X {
		rc.Check(i.X[j], 8)
	}
	const yLo = fr377.Bits - 1
	const yHi = fr381.Bits - yLo
	rc.Check(i.Y[0], yHi)
	rc.Check(i.Y[1], yLo)
	api.AssertIsLessOrEqual(i.NbBatches, MaxNbBatches) // if too big it can turn "negative" and compromise the interconnection logic
}

func (i *FunctionalPublicInput) ToSnarkType() (FunctionalPublicInputSnark, error) {
	res := FunctionalPublicInputSnark{
		FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
			Y:              [2]frontend.Variable{i.Y[0], i.Y[1]},
			SnarkHash:      i.SnarkHash,
			Eip4844Enabled: utils.Ite(i.Eip4844Enabled, 1, 0),
			NbBatches:      len(i.BatchSums),
		},
	}
	utils.Copy(res.X[:], i.X[:])
	if len(i.BatchSums) > len(res.BatchSums) {
		return res, errors.New("batches do not fit in circuit")
	}
	for n := utils.Copy(res.BatchSums[:], i.BatchSums); n < len(res.BatchSums); n++ {
		res.BatchSums[n] = 0
	}
	return res, nil
}

type fpiSumSettings struct {
	batchesSum    []byte
	batchesSumSet bool
	hsh           hash.Hash
}

type FPISumOption func(settings *fpiSumSettings)

func WithBatchesSum(b []byte) FPISumOption {
	return func(settings *fpiSumSettings) {
		settings.batchesSum = b
		settings.batchesSumSet = true
	}
}

func WithHash(h hash.Hash) FPISumOption {
	return func(settings *fpiSumSettings) {
		settings.hsh = h
	}
}

func (i *FunctionalPublicInput) Sum(opts ...FPISumOption) ([]byte, error) {

	var settings fpiSumSettings
	for _, o := range opts {
		o(&settings)
	}
	if !settings.batchesSumSet {
		settings.batchesSum = internal.ChecksumSlice(i.BatchSums)
	}
	hsh := settings.hsh
	if hsh == nil {
		hsh = gcHash.MIMC_BLS12_377.New()
	}

	hsh.Reset()
	hsh.Write(i.X[:16])
	hsh.Write(i.X[16:])
	hsh.Write(i.Y[0])
	hsh.Write(i.Y[1])
	hsh.Write(i.SnarkHash)
	hsh.Write(utils.Ite(i.Eip4844Enabled, []byte{1}, []byte{0}))
	hsh.Write(settings.batchesSum)
	return hsh.Sum(nil), nil
}

// Sum ignores NbBatches, as its value is expected to be incorporated into batchesSum
func (i *FunctionalPublicInputQSnark) Sum(api frontend.API, hsh snarkHash.FieldHasher, batchesSum frontend.Variable) frontend.Variable {
	radix := big.NewInt(256)
	hsh.Reset()
	hsh.Write(compress.ReadNum(api, i.X[:16], radix), compress.ReadNum(api, i.X[16:], radix), i.Y[0], i.Y[1], i.SnarkHash, i.Eip4844Enabled, batchesSum)
	return hsh.Sum()
}

// Sum hashes the inputs together into a single "de facto" public input
// WARNING: i.X[-] are not range-checked here
func (i *FunctionalPublicInputSnark) Sum(api frontend.API, hsh snarkHash.FieldHasher) frontend.Variable {
	return i.FunctionalPublicInputQSnark.Sum(api, hsh, internal.VarSlice{
		Values: i.BatchSums[:],
		Length: i.NbBatches,
	}.Checksum(api, hsh))
}

func (c Circuit) Define(api frontend.API) error {
	var hsh snarkHash.FieldHasher
	if c.UseGkrMiMC {
		hsh = gkrmimc.NewHasherFactory(api).NewHasher()
	} else {
		if h, err := mimc.NewMiMC(api); err != nil {
			return err
		} else {
			hsh = &h
		}
	}

	batchSums := internal.VarSlice{
		Values: c.FuncPI.BatchSums[:],
		Length: c.FuncPI.NbBatches,
	}

	blobSum, y, err := ProcessBlob(api, hsh, c.MaxBlobPayloadNbBytes, c.BlobBytes, c.FuncPI.X, c.FuncPI.Eip4844Enabled, batchSums, c.Dict)
	if err != nil {
		return err
	}
	api.AssertIsEqual(c.FuncPI.SnarkHash, blobSum)
	api.AssertIsEqual(c.FuncPI.Y[0], y[0])
	api.AssertIsEqual(c.FuncPI.Y[1], y[1])

	api.AssertIsEqual(c.PublicInput, c.FuncPI.Sum(api, hsh))
	return nil
}

type builder struct {
	dictionaryLength int
}

func NewBuilder(dictionaryLength int) *builder {
	return &builder{dictionaryLength: dictionaryLength}
}

// Compile the decompression circuit
// Make sure to add the gkrmimc solver options in proving time
func (b *builder) Compile() (constraint.ConstraintSystem, error) {
	return Compile(b.dictionaryLength), nil
}

func Compile(dictionaryLength int) constraint.ConstraintSystem {
	// TODO @gbotrel make signature return error...
	if cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &Circuit{
		Dict:                  make([]frontend.Variable, dictionaryLength),
		BlobBytes:             make([]frontend.Variable, blob.MaxUsableBytes),
		MaxBlobPayloadNbBytes: blob.MaxUncompressedBytes,
		UseGkrMiMC:            true,
	}, frontend.WithCapacity(1<<27)); err != nil {
		panic(err)
	} else {
		return cs
	}
}

func AssignFPI(blobBytes []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (fpi FunctionalPublicInput, dict []byte, err error) {
	if len(blobBytes) != blob.MaxUsableBytes {
		err = fmt.Errorf("decompression circuit assignment : invalid blob length : %d. expected %d", len(blobBytes), blob.MaxUsableBytes)
		return
	}

	header, payload, _, dict, err := blob.DecompressBlob(blobBytes, dictStore)
	if err != nil {
		return
	}

	if header.NbBatches() > MaxNbBatches {
		err = fmt.Errorf("decompression circuit assignment : too many batches in the header : %d. max %d", header.NbBatches(), MaxNbBatches)
		return
	}
	batchEnds := make([]int, header.NbBatches())
	if header.NbBatches() > 0 {
		batchEnds[0] = header.BatchSizes[0]
	}
	for i := 1; i < len(header.BatchSizes); i++ {
		batchEnds[i] = batchEnds[i-1] + header.BatchSizes[i]
	}

	fpi.BatchSums = BatchesChecksumAssign(batchEnds, payload)

	fpi.X = x

	fpi.Y, err = internal.Bls12381ScalarToBls12377Scalars(y)
	if err != nil {
		return
	}

	fpi.Eip4844Enabled = eip4844Enabled

	if len(blobBytes) != 128*1024 {
		panic("blobBytes length is not 128*1024")
	}
	fpi.SnarkHash, err = encode.MiMCChecksumPackedData(blobBytes, fr381.Bits-1, encode.NoTerminalSymbol()) // TODO if forced to remove the above check, pad with zeros

	return
}

func Assign(blobBytes []byte, dictStore dictionary.Store, eip4844Enabled bool, x [32]byte, y fr381.Element) (assignment frontend.Circuit, publicInput fr377.Element, snarkHash []byte, err error) {

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

	sfpi, err := fpi.ToSnarkType()
	if err != nil {
		return
	}

	assignment = &Circuit{
		Dict:        utils.ToVariableSlice(dict),
		BlobBytes:   utils.ToVariableSlice(blobBytes),
		PublicInput: publicInput,
		FuncPI:      sfpi,
	}

	registerHints() // @Alexandre.Belling right place for this? TODO make sure this covers all hints used

	return
}

// the result for each batch is <data (31 bytes)> ... <data (31 bytes)>
// for soundness some kind of length indicator must later be incorporated.
func BatchesChecksumAssign(ends []int, payload []byte) [][]byte {
	res := make([][]byte, len(ends))

	in := make([]byte, len(payload)+31)
	copy(in, payload) // pad with 31 bytes to avoid out of range panic

	hsh := gcHash.MIMC_BLS12_377.New()
	var buf [32]byte

	batchStart := 0
	for i := range res {
		gnarkutil.ChecksumLooselyPackedBytes(payload[batchStart:ends[i]], buf[:], hsh)
		res[i] = bytes.Clone(buf[:])
		batchStart = ends[i]
	}

	return res
}
