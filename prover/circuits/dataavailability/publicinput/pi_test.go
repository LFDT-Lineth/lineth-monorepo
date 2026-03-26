//go:build !fuzzlight

package publicinput

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr/iop"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/assert"
)

const (
	msbNbBits            = (fr381.Bits - 1) % 8
	msbMask              = 1<<msbNbBits - 1
	testDataDir          = "../../../../contracts/test/hardhat/_testData/compressedDataEip4844"
	testDataNoEip4844Dir = "../../../../contracts/test/hardhat/_testData/compressedData"
)

func createRandomBlobElems(n int) [][]byte {

	blob := make([][]byte, n)
	for i := range blob {
		blob[i] = make([]byte, 32)
		if _, err := rand.Read(blob[i]); err != nil {
			panic(err)
		}
		blob[i][0] &= msbMask
	}
	return blob
}

// TestInterpolateLagrange tests the EIP4844 consistency check against the gnark iop package
func TestInterpolateLagrange(t *testing.T) {

	assignment := func(unitCircleEvaluations []interface{}, evaluationPoint interface{}) *testInterpolateLagrangeCircuit {

		unitCircleEvaluationsFr := mapSlice(unitCircleEvaluations[:], func(i interface{}) fr381.Element {
			var res fr381.Element
			_, err := res.SetInterface(i)
			assert.NoError(t, err)
			return res
		})

		// randomize an evaluation point that fits in a bls12-377 fr element
		var (
			evaluationPointFr fr381.Element
		)
		_, err := evaluationPointFr.SetInterface(evaluationPoint)
		assert.NoError(t, err)

		assignment := testInterpolateLagrangeCircuit{
			UnitCircleEvaluationsBytes: make([][fr381.Bytes]frontend.Variable, len(unitCircleEvaluations)),
		}
		for i := range unitCircleEvaluations {
			bytes := unitCircleEvaluationsFr[i].Bytes()
			copy(assignment.UnitCircleEvaluationsBytes[i][:], utils.ToVariableSlice(bytes[:]))
		}

		// compute the evaluation using the iop package
		domain := fft.NewDomain(uint64(len(unitCircleEvaluations)))
		poly := iop.NewPolynomial(&unitCircleEvaluationsFr, iop.Form{Basis: iop.Lagrange, Layout: iop.Regular})
		poly.ToCanonical(domain)
		evaluation := poly.Evaluate(evaluationPointFr)
		evaluationBytes := evaluation.Bytes()
		var evaluationInt big.Int
		evaluationInt.SetBytes(evaluationBytes[:])

		{
			scalars := types.Bls12381Fr(evaluationPointFr.Bytes())
			assert.NoError(t, err)
			assignment.EvaluationPoint[0] = scalars[:16]
			assignment.EvaluationPoint[1] = scalars[16:]
		}

		{
			scalars := types.Bls12377Fr(evaluation.Bytes())
			assert.NoError(t, err)
			assignment.Evaluation[0] = scalars[:16]
			assignment.Evaluation[1] = scalars[16:]
		}

		return &assignment
	}

	randomAssignment := func(n int) *testInterpolateLagrangeCircuit {
		unitCircleEvaluations := createRandomBlobElems(n)
		var evaluationPoint fr381.Element
		_, err := evaluationPoint.SetRandom()
		assert.NoError(t, err)
		return assignment(mapSlice(unitCircleEvaluations, func(b []byte) interface{} { return b }), evaluationPoint)
	}

	assignments := []*testInterpolateLagrangeCircuit{
		assignment([]interface{}{0, 4, 0, 0}, 0),
		assignment([]interface{}{"221797350491448557374835382936094284962079105806616932502871687042746686348", "5449307655738973627560541249222884206820629036484003960051650107064657316177"}, "6506134398774570609831295452620385261047212455886876242937577495553156355635"),
		assignment([]interface{}{1, 0}, 3),
	}

	for n := 4096; n > 2; n /= 32 {
		assignments = append(assignments, randomAssignment(n))
	}

	slices.SortFunc(assignments, func(a, b *testInterpolateLagrangeCircuit) int {
		return len(a.UnitCircleEvaluationsBytes) - len(b.UnitCircleEvaluationsBytes)
	})

	for i := range assignments {
		assert.NoError(t, test.IsSolved(
			&testInterpolateLagrangeCircuit{UnitCircleEvaluationsBytes: make([][fr381.Bytes]frontend.Variable, len(assignments[i].UnitCircleEvaluationsBytes))},
			assignments[i], ecc.BLS12_377.ScalarField(),
		))
	}
}

type testInterpolateLagrangeCircuit struct {
	EvaluationPoint            [2]frontend.Variable
	Evaluation                 [2]frontend.Variable
	UnitCircleEvaluationsBytes [][fr381.Bytes]frontend.Variable
}

func (c *testInterpolateLagrangeCircuit) Define(api frontend.API) error {
	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	unitCircleEvaluations := mapSlice(c.UnitCircleEvaluationsBytes, func(v [fr381.Bytes]frontend.Variable) *emulated.Element[emulated.BLS12381Fr] {
		var bits [len(v) * 8]frontend.Variable
		for i := range v {
			copy(bits[i*8:], api.ToBinary(v[len(v)-1-i], 8))
		}
		return field.FromBits(bits[:]...)
	})
	evaluationPoint := newElementFromVars(api, c.EvaluationPoint)
	if err != nil {
		return err
	}

	evaluation, err := interpolateLagrangeBls12381(field, unitCircleEvaluations, evaluationPoint)
	if err != nil {
		return err
	}
	evaluationHL := bls12381ScalarToBls12377Scalars(api, evaluation)
	api.AssertIsEqual(c.Evaluation[0], evaluationHL[0])
	api.AssertIsEqual(c.Evaluation[1], evaluationHL[1])

	return nil
}

type blobConsistencyCheckCircuit struct {
	BlobBytes      []frontend.Variable   // "the blob" in EIP-4844 parlance
	X              [32]frontend.Variable `gnark:",public"` // "high" and "low"
	Y              [2]frontend.Variable  `gnark:",public"`
	Eip4844Enabled frontend.Variable
}

func (c *blobConsistencyCheckCircuit) Define(api frontend.API) error {
	blobCrumbs := internal.PackedBytesToCrumbs(api, c.BlobBytes, fr381.Bits-1)
	y, err := VerifyBlobConsistency(api, blobCrumbs, c.X, c.Eip4844Enabled)
	if err != nil {
		return err
	}
	api.AssertIsEqual(y[0], c.Y[0])
	api.AssertIsEqual(y[1], c.Y[1])
	return nil
}

type BlobConsistencyCheckTestCase struct {
	Eip4844Enabled bool   `json:"eip4844Enabled"`
	CompressedData string `json:"compressedData"`
	ExpectedX      string `json:"expectedX"`
	ExpectedY      string `json:"expectedY"`
	SnarkHash      string `json:"snarkHash"`
}

func decodeHex(t *testing.T, s string) []byte {
	assert.Equal(t, s[:2], "0x")
	b, err := hex.DecodeString(s[2:])
	assert.NoError(t, err)
	return b
}

func decodeHexHL(t *testing.T, s string) (r [2]frontend.Variable) {
	b := decodeHex(t, s)
	assert.Equal(t, len(b), 32)

	scalars := types.Bls12377Fr(b)
	r[0] = scalars[:16]
	r[1] = scalars[16:]

	return
}

func TestVerifyBlobConsistencyIntegration(t *testing.T) {
	circuit := &blobConsistencyCheckCircuit{
		BlobBytes: make([]frontend.Variable, 4096*32),
	}

	loadTestsInFolder := func(path string) {
		files, err := os.ReadDir(path)
		folderName := filepath.Base(path)
		assert.NoError(t, err)
		for _, file := range files {
			folderAndFile := filepath.Join(folderName, file.Name())
			if !strings.HasPrefix(file.Name(), "blocks-") || !strings.HasSuffix(file.Name(), ".json") {
				t.Logf("skipping \"%s\"", folderAndFile)
				continue
			}
			t.Logf("loading \"%s\"", folderAndFile)
			filePath := filepath.Join(path, file.Name())
			fileRaw, err := os.ReadFile(filePath)
			assert.NoError(t, err)
			var testCase BlobConsistencyCheckTestCase
			assert.NoError(t, json.Unmarshal(fileRaw, &testCase))

			var assignment blobConsistencyCheckCircuit

			blob, err := base64.StdEncoding.DecodeString(testCase.CompressedData)
			assert.NoError(t, err)
			assert.Zero(t, len(blob)%32, "blob not consisting of 32-byte field elements")
			assert.LessOrEqual(t, len(blob), 4096*32, "blob too large")
			blob = append(blob, make([]byte, 4096*32-len(blob))...) // pad if necessary
			assignment.BlobBytes = utils.ToVariableSlice(blob)

			if assignment.Eip4844Enabled = 0; testCase.Eip4844Enabled {
				assignment.Eip4844Enabled = 1
			}

			utils.Copy(assignment.X[:], decodeHex(t, testCase.ExpectedX))
			assignment.Y = decodeHexHL(t, testCase.ExpectedY)

			t.Run(folderAndFile, func(t *testing.T) {
				assert.NoError(t, test.IsSolved(circuit, &assignment, ecc.BLS12_377.ScalarField()))
			})
		}
	}

	loadTestsInFolder(testDataDir)
	loadTestsInFolder(testDataNoEip4844Dir)

}

func TestCompileBlobConsistencyCheck(t *testing.T) {
	circuit := &blobConsistencyCheckCircuit{
		BlobBytes: make([]frontend.Variable, 4096*32),
	}
	cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit, frontend.WithCapacity(8000000))
	assert.NoError(t, err)
	fmt.Println(cs.GetNbConstraints())
}

func TestVectorIopCompatibility(t *testing.T) {
	var testCase BlobConsistencyCheckTestCase
	fileRaw, err := os.ReadFile(filepath.Join(testDataDir, "blocks-1-46.json"))
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(fileRaw, &testCase))

	domain := fft.NewDomain(4096)
	blob, err := base64.StdEncoding.DecodeString(testCase.CompressedData)
	assert.NoError(t, err)
	blob = append(blob, make([]byte, 4096*32-len(blob))...) // pad if necessary
	blobElems := make([]fr381.Element, 4096)
	for i := range blobElems {
		assert.NoError(t, blobElems[bitReverse(i, 12)].SetBytesCanonical(blob[i*32:(i+1)*32]), i)
	}
	poly := iop.NewPolynomial(&blobElems, iop.Form{Basis: iop.Lagrange, Layout: iop.Regular})
	poly.ToCanonical(domain)

	var evaluationPoint fr381.Element
	evaluationPoint.SetBytes(decodeHex(t, testCase.ExpectedX))

	y := poly.Evaluate(evaluationPoint)

	// Compare with expected Y - both are BLS12-381 field elements
	var expectedY fr381.Element
	expectedYBytes := decodeHex(t, testCase.ExpectedY)
	expectedY.SetBytes(expectedYBytes)
	assert.Equal(t, expectedY.Text(16), y.Text(16))
}

func TestConsistencyCheckFlagRange(t *testing.T) {
	circuit := &blobConsistencyCheckCircuit{
		BlobBytes: make([]frontend.Variable, 4096*32),
	}
	assignments := []*blobConsistencyCheckCircuit{
		{
			BlobBytes:      utils.ToVariableSlice(make([]byte, 4096*32)),
			Y:              [2]frontend.Variable{0, 0},
			Eip4844Enabled: 0,
		},
		{
			BlobBytes:      utils.ToVariableSlice(make([]byte, 4096*32)),
			Y:              [2]frontend.Variable{0, 0},
			Eip4844Enabled: 1,
		},
		{
			BlobBytes:      utils.ToVariableSlice(make([]byte, 4096*32)),
			Y:              [2]frontend.Variable{0, 0},
			Eip4844Enabled: 2,
		},
	}
	for i := range assignments {
		setZero(assignments[i].X[:])
	}

	internal.RegisterHints()

	options := []test.TestingOption{
		test.WithCurves(ecc.BLS12_377), test.WithBackends(backend.PLONK),
		test.WithValidAssignment(assignments[0]),
		test.WithValidAssignment(assignments[1]),
		test.WithInvalidAssignment(assignments[2]),
		test.NoTestEngine(),
	}
	test.NewAssert(t).CheckCircuit(circuit, options...)
}

func setZero(s []frontend.Variable) {
	for i := range s {
		s[i] = 0
	}
}

func TestFrConversions(t *testing.T) {

	testCases := [][]byte{
		types.Bls12381FrFromHex("0x0045231284858326638837332416019018714005183587760015845327913118").Bytes(), // 1 << 248
		types.Bls12381FrFromHex("0x0265791615971334361278052958182119172036361757827436807633319032").Bytes(),
		append([]byte{0, 64}, make([]byte, 30)...), // 2^254 or 0x40000...
	}

	for i := 0; i < 100; i++ {
		var b [32]byte
		_, err := rand.Read(b[:])
		assert.NoError(t, err)

		var x fr377.Element
		x.SetBytes(b[:])
		b = x.Bytes()

		testCases = append(testCases, b[:])
	}

	options := []test.TestingOption{
		test.WithCurves(ecc.BLS12_377), test.WithBackends(backend.PLONK),
		test.NoTestEngine(),
	}

	var twoTo128 fr381.Element
	{
		i := big.NewInt(1)
		i.Lsh(i, 128)
		twoTo128.SetBigInt(i)
	}

	for _, testCase := range testCases {

		t.Logf("test case: %x", testCase)
		xPartitioned := types.Bls12377Fr(testCase)
		xPartitioned.MustGetFrElement()

		var assignment testFrConversionCircuit
		assignment.X[0] = xPartitioned[:16]
		assignment.X[1] = xPartitioned[16:]
		options = append(options, test.WithValidAssignment(&assignment))
	}

	test.NewAssert(t).CheckCircuit(&testFrConversionCircuit{}, options...)
}

type testFrConversionCircuit struct {
	X [2]frontend.Variable
}

func (c *testFrConversionCircuit) Define(api frontend.API) error {
	x := newElementFromVars(api, c.X)
	xBack := bls12381ScalarToBls12377Scalars(api, x)
	api.AssertIsEqual(c.X[0], xBack[0])
	api.AssertIsEqual(c.X[1], xBack[1])
	return nil
}

func TestPackCrumbEmulated(t *testing.T) {
	var bytes [fr381.Bytes]byte
	_, err := rand.Read(bytes[:])
	assert.NoError(t, err)
	bytes[0] &= msbMask
	var assignment testPackCrumbEmulatedCircuit
	copy(assignment.Bytes[:], utils.ToVariableSlice(bytes[:]))
	test.NewAssert(t).CheckCircuit(
		&testPackCrumbEmulatedCircuit{}, test.WithValidAssignment(&assignment), test.WithCurves(ecc.BLS12_377), test.WithBackends(backend.PLONK),
		test.NoTestEngine(),
	)
}

type testPackCrumbEmulatedCircuit struct {
	Bytes [fr381.Bytes]frontend.Variable // big endian
}

func (c *testPackCrumbEmulatedCircuit) Define(api frontend.API) error {
	var bits [fr381.Bits - 1]frontend.Variable // big endian
	copy(bits[:], api.ToBinary(c.Bytes[0], msbNbBits))
	for i := 1; i < len(c.Bytes); i++ {
		copy(bits[msbNbBits+8*i-8:], api.ToBinary(c.Bytes[i], 8))
	}

	var crumbs [len(bits) / 2]frontend.Variable
	for i := range crumbs {
		crumbs[i] = api.Add(api.Mul(bits[2*i], 2), bits[2*i+1])
	}

	slices.Reverse(bits[:]) // now little endian

	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	if err != nil {
		return err
	}
	expected := field.FromBits(bits[:]...)

	actual := packCrumbsEmulated(api, crumbs[:])
	if len(actual) != 1 {
		return fmt.Errorf("expected one element, got %d", len(actual))
	}

	field.AssertIsEqual(expected, actual[0])

	return nil
}

func newElementFromVars(api frontend.API, x [2]frontend.Variable) *emulated.Element[emulated.BLS12381Fr] {
	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	if err != nil {
		panic(err)
	}
	const (
		lSize = 16 * 8
		hSize = fr381.Bits - lSize
	)
	lBin := api.ToBinary(x[1], lSize)
	hBin := api.ToBinary(x[0], hSize)
	return field.FromBits(append(lBin, hBin...)...)
}
