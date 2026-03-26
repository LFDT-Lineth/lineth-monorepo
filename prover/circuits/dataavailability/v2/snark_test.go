package v2

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	gkrposeidon2 "github.com/consensys/gnark/std/hash/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2/test_utils"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/dictionary"
	"github.com/consensys/linea-monorepo/prover/lib/compressor/blob/encode"
	"github.com/consensys/linea-monorepo/prover/utils"
	"golang.org/x/exp/constraints"

	"github.com/consensys/linea-monorepo/prover/circuits/internal"

	"github.com/consensys/gnark-crypto/ecc"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2"
	blobtestutils "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v2/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maxNbBatches = 100

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

	dictStore, err := dictionary.SingletonStore(blobtestutils.GetDict(t), 2)
	assert.NoError(t, err)

	for _, blobData := range blobs {

		r, err := blob.DecompressBlob(blobData, dictStore)
		assert.NoError(t, err)

		assert.LessOrEqual(t, len(r.Blocks), maxNbBatches, "too many batches")

		unpacked, err := encode.UnpackAlign(blobData, fr381.Bits-1, false)
		require.NoError(t, err)

		assignment := &testParseHeaderCircuit{
			Blob:      test_utils.PadBytes(unpacked, maxBlobSize),
			HeaderLen: r.Header.ByteSize(),
			NbBatches: r.Header.NbBatches(),
			BlobLen:   len(unpacked),
		}

		for i := range assignment.BytesPerBatch {
			if i < r.Header.NbBatches() {
				assignment.BytesPerBatch[i] = r.Header.BatchSizes[i]
			} else {
				assignment.BytesPerBatch[i] = 0
			}
		}

		require.NoError(t, test.IsSolved(&circuit, assignment, ecc.BLS12_377.ScalarField()))
	}
}

func TestChecksumBatches(t *testing.T) {
	const (
		nbAssignments = 200
	)
	var blobData [2000 / 32 * 32]byte // just make sure it's a multiple of the packing size
	blobLen := 0

	var batchEndss [nbAssignments][]int
	for i := range batchEndss {
		batchEndss[i] = make([]int, blobtestutils.RandIntn(maxNbBatches)+1)
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
	// TODO Test scenario where nbBatches = maxNbBatches

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

func differences[T constraints.Integer](s []T) []T {
	res := make([]T, len(s))
	last := T(0)
	for i := range s {
		res[i] = s[i] - last
		last = s[i]
	}
	return res
}

func testChecksumBatches(t *testing.T, blob []byte, batchEndss ...[]int) {
	maxNbBatches := 0
	for _, batchEnds := range batchEndss {
		maxNbBatches = max(maxNbBatches, len(batchEnds))
	}

	circuit := testChecksumCircuit{
		Blob: make([]frontend.Variable, len(blob)),
		Sums: make([]execution.DataChecksumSnark, maxNbBatches),
	}
	for _, batchEnds := range batchEndss {

		assignment := testChecksumCircuit{
			Blob:      utils.ToVariableSlice(blob),
			Sums:      make([]execution.DataChecksumSnark, maxNbBatches),
			NbBatches: len(batchEnds),
		}

		sums, err := assignExecutionDataSums(blob, differences(batchEnds))
		require.NoError(t, err)
		require.NoError(t, executionDataSumsToSnarkType(assignment.Sums[:], sums))
		assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))

		assignment.Sums[blobtestutils.RandIntn(len(batchEnds))].PartialHash = 3
		assert.Error(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
	}
}

type testParseHeaderCircuit struct {
	Blob                          []frontend.Variable
	BytesPerBatch                 [maxNbBatches]frontend.Variable
	HeaderLen, NbBatches, BlobLen frontend.Variable
}

func (c *testParseHeaderCircuit) Define(api frontend.API) error {
	headerLen, _, nbBatches, err := parseHeader(api, c.BytesPerBatch[:], c.Blob, c.BlobLen)
	if err != nil {
		return err
	}
	api.AssertIsEqual(headerLen, c.HeaderLen)
	api.AssertIsEqual(nbBatches, c.NbBatches)

	return nil
}

type testChecksumCircuit struct {
	Blob      []frontend.Variable
	Sums      []execution.DataChecksumSnark
	NbBatches frontend.Variable
}

func (c *testChecksumCircuit) Define(api frontend.API) error {
	api.AssertIsLessOrEqual(c.NbBatches, len(c.Sums))

	return CheckBatchesPartialSums(
		api,
		c.NbBatches,
		c.Blob,
		c.Sums,
	)
}

// [
//
//	starts[0], starts[0] + 1, ..., starts[0] + n - 1,
//	starts[1], starts[1] + 1, ..., starts[1] + n - 1,
//	...
//
// ]
// if len(starts) = 0 it'll be treated as if starts = [0]
func _range(n int, starts ...int) []byte {
	if len(starts) == 0 {
		starts = []int{0}
	}
	b := make([]byte, 0, n*len(starts))
	for i := range starts {
		for j := range n {
			b = append(b, byte(starts[i]+j))
		}
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
		assignment.Checksum, err = encode.Poseidon2ChecksumPackedData(dataPadded[:nPadded], fr381.Bits-1, encode.NoTerminalSymbol())
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

	hsh, err := gkrposeidon2.New(api)
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
	dictStore, err := dictionary.SingletonStore(dict, 2)
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

type testPackBatchesCircuit struct {
	BlobBytes             []frontend.Variable
	NbBatches             frontend.Variable
	BatchLengths          []frontend.Variable
	ExpectedPackedBatches packedBatches
}

func (c *testPackBatchesCircuit) Define(api frontend.API) error {
	packedBatches, err := packBatches(api, c.NbBatches, c.BlobBytes, c.BatchLengths)
	if err != nil {
		return err
	}
	internal.AssertSliceEquals(api, c.ExpectedPackedBatches.Ends, packedBatches.Ends)
	internal.AssertSliceEquals(api, c.ExpectedPackedBatches.Range.InRange, packedBatches.Range.InRange)
	internal.AssertSliceEquals(api, c.ExpectedPackedBatches.Range.IsLast, packedBatches.Range.IsLast)
	internal.AssertSliceEquals(api, c.ExpectedPackedBatches.Range.IsFirstBeyond, packedBatches.Range.IsFirstBeyond)

	if len(packedBatches.Iterations) != len(c.ExpectedPackedBatches.Iterations) {
		return fmt.Errorf("expected %d batch packing Iterations, got %d", len(c.ExpectedPackedBatches.Iterations), len(packedBatches.Iterations))
	}
	for i := range packedBatches.Iterations {
		api.AssertIsEqual(c.ExpectedPackedBatches.Iterations[i].BatchI, packedBatches.Iterations[i].BatchI)
		api.AssertIsEqual(c.ExpectedPackedBatches.Iterations[i].NextStarts, packedBatches.Iterations[i].NextStarts)
		api.AssertIsEqual(c.ExpectedPackedBatches.Iterations[i].Next, packedBatches.Iterations[i].Next)
		api.AssertIsEqual(c.ExpectedPackedBatches.Iterations[i].Current, packedBatches.Iterations[i].Current)
		api.AssertIsEqual(c.ExpectedPackedBatches.Iterations[i].NoOp, packedBatches.Iterations[i].NoOp)
	}
	return nil
}

type packBatchesIterationJson struct {
	Current    string `json:"current"`
	Next       string `json:"next"`
	NextStarts bool   `json:"nextStarts"`
	NoOp       bool   `json:"noOp"`
	BatchI     int    `json:"batchI"`
}
type packBatchesTestCase struct {
	BlobLength   int                        `json:"blobLength"`
	BatchLengths []int                      `json:"batchLengths"`
	Iterations   []packBatchesIterationJson `json:"Iterations"`
}

func readFromJsonFile[T any](t *testing.T, filename string) T {
	f, err := os.Open(filename)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	var res T
	require.NoError(t, json.NewDecoder(f).Decode(&res))
	return res
}

func assignRange(dynamic, static int) *internal.Range {
	if dynamic > static {
		panic("dynamic length must be no more than static length")
	}
	res := internal.Range{
		InRange:       make([]frontend.Variable, static),
		IsLast:        make([]frontend.Variable, static),
		IsFirstBeyond: make([]frontend.Variable, static),
	}

	for i := range static {
		res.InRange[i] = 0
		res.IsLast[i] = 0
		res.IsFirstBeyond[i] = 0
	}

	for i := range dynamic {
		res.InRange[i] = 1
	}

	res.IsLast[dynamic-1] = 1

	if dynamic < static {
		res.IsFirstBeyond[dynamic] = 1
	}

	return &res
}

func testPackBatches(t *testing.T, testCase packBatchesTestCase) {
	assignment := testPackBatchesCircuit{
		BlobBytes:    utils.ToVariableSlice(_range(testCase.BlobLength)),
		NbBatches:    len(testCase.BatchLengths),
		BatchLengths: utils.ToVariableSlice(testCase.BatchLengths),
		ExpectedPackedBatches: packedBatches{
			Iterations: make([]batchPackingIteration, len(testCase.Iterations)),
			Ends:       utils.ToVariableSlice(partialSums(testCase.BatchLengths)),
			Range:      assignRange(len(testCase.BatchLengths), len(testCase.BatchLengths)), // can add test cases with maxNbBatches > nbBatches
		},
	}

	var err error
	for i := range testCase.Iterations {
		assignment.ExpectedPackedBatches.Iterations[i].Next, err = utils.HexDecodeString(testCase.Iterations[i].Next)
		require.NoError(t, err, "decoding the \"next\" field at iteration %d; string provided \"%s\"", i, testCase.Iterations[i].Next)

		assignment.ExpectedPackedBatches.Iterations[i].Current, err = utils.HexDecodeString(testCase.Iterations[i].Current)
		require.NoError(t, err, "decoding the \"current\" field at iteration %d; string provided \"%s\"", i, testCase.Iterations[i].Current)

		assignment.ExpectedPackedBatches.Iterations[i].BatchI = testCase.Iterations[i].BatchI
		assignment.ExpectedPackedBatches.Iterations[i].NextStarts = utils.Ite(testCase.Iterations[i].NextStarts, 1, 0)
		assignment.ExpectedPackedBatches.Iterations[i].NoOp = utils.Ite(testCase.Iterations[i].NoOp, 1, 0)
	}

	circuit := testPackBatchesCircuit{
		BlobBytes:    make([]frontend.Variable, len(assignment.BlobBytes)),
		BatchLengths: make([]frontend.Variable, len(assignment.BatchLengths)),
		ExpectedPackedBatches: packedBatches{
			Iterations: make([]batchPackingIteration, len(assignment.ExpectedPackedBatches.Iterations)),
			Ends:       make([]frontend.Variable, len(assignment.ExpectedPackedBatches.Ends)),
			Range: &internal.Range{
				InRange:       make([]frontend.Variable, len(assignment.ExpectedPackedBatches.Range.InRange)),
				IsLast:        make([]frontend.Variable, len(assignment.ExpectedPackedBatches.Range.IsLast)),
				IsFirstBeyond: make([]frontend.Variable, len(assignment.ExpectedPackedBatches.Range.IsFirstBeyond)),
			},
		},
	}

	require.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
}

func partialSums[T constraints.Integer](s []T) []T {
	prev := T(0)
	res := make([]T, len(s))
	for i := range s {
		res[i] = prev + s[i]
		prev = res[i]
	}
	return res
}

func TestPackBatches(t *testing.T) {
	t.Run("[0,1,...,123]", func(t *testing.T) {
		blobData := _range(31 * 4)
		batchEnds := []int{32, 63, 100}
		testChecksumBatches(t, blobData, batchEnds)
	})

	// todo @gusiri: the testdata/pack_batches directory with JSON test fixtures
	// is missing (possibly removed or not yet regenerated for the koala bear
	// field transition). Skipping the directory-based test cases until the test
	// data is available.
	const dirPath = "testdata/pack_batches"
	dir, err := os.ReadDir(dirPath)
	if err != nil {
		t.Skipf("skipping directory-based pack_batches tests: %v", err)
	}

	for _, dirEntry := range dir {
		if dirEntry.IsDir() {
			t.Logf("skipping subdirectory \"%s\"", dirEntry.Name())
			continue
		}
		if filepath.Ext(dirEntry.Name()) != ".json" {
			t.Logf("skipping non-json file \"%s\"", dirEntry.Name())
			continue
		}
		t.Run(dirEntry.Name()[:len(dirEntry.Name())-len(".json")], func(t *testing.T) {
			testPackBatches(t, readFromJsonFile[packBatchesTestCase](t, filepath.Join(dirPath, dirEntry.Name())))
		})
	}
}
