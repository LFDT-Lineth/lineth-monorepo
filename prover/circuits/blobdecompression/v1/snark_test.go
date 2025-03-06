package v1

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1/test_utils"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/test"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	blobtestutils "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO Must fail on invalid input
func TestParseHeader(t *testing.T) {
	maxBlobSize := 1024

	blobs := [][]byte{
		blobtestutils.GenTestBlob(t, 100000),
	}

	for _, blobData := range blobs {
		if len(blobData) > maxBlobSize {
			maxBlobSize = len(blobData)
		}
	}

	circuit := testParseHeaderCircuit{
		Blob: make([]frontend.Variable, maxBlobSize),
	}

	options := []test.TestingOption{
		test.WithCurves(ecc.BLS12_377), test.WithBackends(backend.PLONK),
		test.NoTestEngine(),
	}

	dictStore, err := dictionary.SingletonStore(blobtestutils.GetDict(t), 1)
	assert.NoError(t, err)

	for _, blobData := range blobs {

		r, err := blob.DecompressBlob(blobData, dictStore)
		assert.NoError(t, err)

		assert.LessOrEqual(t, len(r.Blocks), MaxNbBatches, "too many batches")

		unpacked, err := encode.UnpackAlign(blobData, fr381.Bits-1, false)
		require.NoError(t, err)

		assignment := &testParseHeaderCircuit{
			Blob:      test_utils.PadBytes(unpacked, maxBlobSize),
			HeaderLen: r.Header.ByteSize(),
			NbBatches: r.Header.NbBatches(),
			BlobLen:   len(unpacked),
		}

		for i := range assignment.BlocksPerBatch {
			if i < r.Header.NbBatches() {
				assignment.BlocksPerBatch[i] = r.Header.BatchSizes[i]
			} else {
				assignment.BlocksPerBatch[i] = 0
			}
		}

		options = append(options, test.WithValidAssignment(assignment))
	}

	test.NewAssert(t).CheckCircuit(&circuit, options...)
}

func TestChecksumBatches(t *testing.T) {
	const (
		nbAssignments = 200
	)
	var blobData [2000 / 32 * 32]byte // just make sure it's a multiple of the packing size
	blobLen := 0

	var batchEndss [nbAssignments][]int
	for i := range batchEndss {
		batchEndss[i] = make([]int, blobtestutils.RandIntn(MaxNbBatches)+1)
		for j := range batchEndss[i] {
			batchEndss[i][j] = 31 + blobtestutils.RandIntn(62)
			if j > 0 {
				batchEndss[i][j] += batchEndss[i][j-1]
			}
			if batchEndss[i][j] > len(blobData) {
				if j == 0 || batchEndss[i][j-1]+31 < len(blobData) {
					batchEndss[i][j] = len(blobData)
					batchEndss[i] = batchEndss[i][:j+1]
				} else {
					batchEndss[i] = batchEndss[i][:j]
				}
				break
			}
		}
		if v := batchEndss[i][len(batchEndss[i])-1]; v > blobLen {
			blobLen = v
		}
	}

	blobLen = (blobLen + 31) / 32 * 32
	_, err := rand.Read(blobData[:blobLen])
	assert.NoError(t, err)

	testChecksumBatches(t, blobData[:blobLen], batchEndss[:]...)

}

func TestChecksumBatchesTrickyCases(t *testing.T) { // this consists of cases that have at some point failed
	// TODO Test scenario where nbBatches = MaxNbBatches

	testChecksumBatches(t, _range(128), []int{31})
	testChecksumBatches(t, _range(128), []int{32, 93})

	testChecksumBatches(t, _range(93), []int{33, 64}) // a batch of length 31 but not word aligned
	testChecksumBatches(t, _range(124), []int{32, 85, 124})
	testChecksumBatches(t, _range(180), []int{50, 110, 148})
}

func TestChecksumBatchesSimple(t *testing.T) {
	blobData := _range(31 * 4)
	batchEnds := []int{32, 63, 100}
	testChecksumBatches(t, blobData, batchEnds)
}

func testChecksumBatches(t *testing.T, blob []byte, batchEndss ...[]int) {
	hsh := hash.MIMC_BLS12_377.New()
	circuit := testChecksumCircuit{
		Blob: make([]frontend.Variable, len(blob)),
	}
	for _, batchEnds := range batchEndss {
		var sums, lengths [MaxNbBatches]frontend.Variable
		batchStart := 0
		buf := make([]byte, 32)

		for j := range sums {
			if j < len(batchEnds) {
				gnarkutil.ChecksumLooselyPackedBytes(blob[batchStart:batchEnds[j]], buf, hsh)
				lengths[j] = batchEnds[j] - batchStart
				sums[j] = bytes.Clone(buf)
				batchStart = batchEnds[j]
			} else {
				sums[j], lengths[j] = 0, 0
			}
		}

		assignment := testChecksumCircuit{
			Blob:      utils.ToVariableSlice(blob),
			Lengths:   lengths,
			Sums:      sums,
			NbBatches: len(batchEnds),
		}
		assignment.Sums[blobtestutils.RandIntn(len(batchEnds))] = 3

		assert.Error(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))

		assignment.Sums = sums
		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
	}
}

type testParseHeaderCircuit struct {
	Blob                          []frontend.Variable
	BlocksPerBatch                [MaxNbBatches]frontend.Variable
	HeaderLen, NbBatches, BlobLen frontend.Variable
}

func (c *testParseHeaderCircuit) Define(api frontend.API) error {
	headerLen, _, nbBatches, blocksPerBatch, err := parseHeader(api, c.Blob, c.BlobLen)
	if err != nil {
		return err
	}
	api.AssertIsEqual(headerLen, c.HeaderLen)
	api.AssertIsEqual(nbBatches, c.NbBatches)

	internal.AssertSliceEquals(api, blocksPerBatch, c.BlocksPerBatch[:])

	return nil
}

type testChecksumCircuit struct {
	Blob          []frontend.Variable
	Lengths, Sums [MaxNbBatches]frontend.Variable
	NbBatches     frontend.Variable
}

func (c *testChecksumCircuit) Define(api frontend.API) error {
	api.AssertIsLessOrEqual(c.NbBatches, MaxNbBatches-1)
	api.AssertIsEqual(len(c.Lengths), MaxNbBatches)
	api.AssertIsEqual(len(c.Sums), MaxNbBatches)

	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}

	if err = CheckBatchesSums(api, &hsh, c.NbBatches, c.Blob, c.Lengths[:], c.Sums[:]); err != nil {
		return err
	}
	return nil
}

// python style
func _range(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestUnpackCircuit(t *testing.T) {

	runTest := func(b []byte) {
		var packedBuf bytes.Buffer
		_, err := encode.PackAlign(&packedBuf, b, fr381.Bits-1) // todo use two different slices
		assert.NoError(t, err)

		circuit := unpackCircuit{
			PackedBytes: make([]frontend.Variable, packedBuf.Len()),
			Bytes:       make([]frontend.Variable, len(b)),
		}

		assignment := unpackCircuit{
			PackedBytes: utils.ToVariableSlice(packedBuf.Bytes()),
			Bytes:       utils.ToVariableSlice(b),
			NbUsedBytes: len(b),
		}
		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
	}

	runTest([]byte{1})

	for n := 3100; n < 3131; n++ {
		b := make([]byte, n)
		_, err := rand.Read(b)
		assert.NoError(t, err)
		runTest(b)
	}
}

type unpackCircuit struct {
	// "real-world" circuit input
	PackedBytes []frontend.Variable

	// expected results
	Bytes       []frontend.Variable
	NbUsedBytes frontend.Variable
}

func (c *unpackCircuit) Define(api frontend.API) error {
	if len(c.PackedBytes)*31 < len(c.Bytes)+1 {
		return errors.New("bytes won't fit in the packed array")
	}

	crumbs := internal.PackedBytesToCrumbs(api, c.PackedBytes, fr381.Bits-1)

	_bytes, nbUsedBytes := crumbStreamToByteStream(api, crumbs)

	api.AssertIsEqual(c.NbUsedBytes, nbUsedBytes)
	if len(_bytes) < len(c.Bytes) {
		return errors.New("incongruent lengths")
	}
	for i := range c.Bytes {
		api.AssertIsEqual(c.Bytes[i], _bytes[i])
	}
	return nil
}

func TestBlobChecksum(t *testing.T) { // aka "snark hash"
	const (
		minLenBytes       = 3100
		maxLenBytes       = 3140
		maxLenBytesPadded = (maxLenBytes + fr381.Bytes - 1) / fr381.Bytes * fr381.Bytes
		mask              = 0xff >> (8 - ((fr381.Bits - 1) % 8))
	)
	var data [maxLenBytes]byte
	_, err := rand.Read(data[:])
	assert.NoError(t, err)
	for i := 0; i < len(data); i += fr381.Bytes {
		data[i] &= mask
	}

	var dataPadded [maxLenBytesPadded]byte
	copy(dataPadded[:], data[:minLenBytes])
	dataVarsPadded := utils.ToVariableSlice(dataPadded[:])
	for n := minLenBytes; n <= maxLenBytes; n++ {
		nPadded := (n + fr381.Bytes - 1) / fr381.Bytes * fr381.Bytes

		circuit := testDataChecksumCircuit{
			DataBytes: make([]frontend.Variable, nPadded),
		}

		dataPadded[n-1] = data[n-1]
		dataVarsPadded[n-1] = data[n-1]

		assignment := testDataChecksumCircuit{
			DataBytes: dataVarsPadded[:nPadded],
		}
		assignment.Checksum, err = encode.MiMCChecksumPackedData(dataPadded[:nPadded], fr381.Bits-1, encode.NoTerminalSymbol())
		assert.NoError(t, err)

		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
	}
}

type testDataChecksumCircuit struct {
	DataBytes []frontend.Variable
	Checksum  frontend.Variable
}

func (c *testDataChecksumCircuit) Define(api frontend.API) error {
	dataCrumbs := internal.PackedBytesToCrumbs(api, c.DataBytes, fr381.Bits-1)

	hsh, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}

	blobPacked377 := internal.PackFull(api, dataCrumbs, 2) // repack into bls12-377 elements to compute a checksum
	hsh.Write(blobPacked377...)
	checksum := hsh.Sum()

	api.AssertIsEqual(c.Checksum, checksum)

	return nil
}

func TestDictHash(t *testing.T) {
	blobBytes := blobtestutils.GenTestBlob(t, 1)
	dict := blobtestutils.GetDict(t)
	dictStore, err := dictionary.SingletonStore(blobtestutils.GetDict(t), 1)
	assert.NoError(t, err)
	r, err := blob.DecompressBlob(blobBytes, dictStore) // a bit roundabout, but the header field is not public
	assert.NoError(t, err)

	circuit := testDataDictHashCircuit{
		DictBytes: make([]frontend.Variable, len(dict)),
	}
	assignment := testDataDictHashCircuit{
		DictBytes: utils.ToVariableSlice(dict),
		Checksum:  r.Header.DictChecksum[:],
	}

	assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}

type testDataDictHashCircuit struct {
	DictBytes []frontend.Variable
	Checksum  frontend.Variable
}

func (c *testDataDictHashCircuit) Define(api frontend.API) error {
	return CheckDictChecksum(api, c.Checksum, c.DictBytes)
}
