package ringsis

import (
	"fmt"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCasesKey = []struct {
	Size int
	Params
}{
	{
		Size:   2,
		Params: StdParams,
	},
	{
		Size:   100,
		Params: StdParams,
	},
	{
		Size:   32,
		Params: StdParams,
	},
	{
		Size:   5,
		Params: StdParams,
	},
	{
		Size:   576,
		Params: StdParams,
	},
	{
		Size: 43,
		Params: Params{
			LogTwoBound:  1,
			LogTwoDegree: 1,
		},
	},
	{
		Size: 23,
		Params: Params{
			LogTwoBound:  1,
			LogTwoDegree: 1,
		},
	},
	{
		Size: 256,
		Params: Params{
			LogTwoBound:  1,
			LogTwoDegree: 1,
		},
	},
}

func TestKeyMaxNumFieldHashable(t *testing.T) {

	for _, testCase := range testCasesKey {
		t.Run(fmt.Sprintf("case-%++v", testCase), func(t *testing.T) {
			numFieldPerPoly := 8 * field.Bytes / testCase.LogTwoBound
			key := GenerateKey(testCase.Params, testCase.Size)
			assert.LessOrEqual(t, testCase.Size, key.MaxNumFieldHashable())
			assert.LessOrEqual(t, key.MaxNumFieldHashable(), testCase.Size+numFieldPerPoly-1)
		})
	}
}

func TestHashModXnMinusOne(t *testing.T) {

	runTest := func(t *testing.T, key *Key, limbs []field.Element) {

		var (
			dualHash       = key.HashModXnMinus1(limbs)
			recomputedHash = []field.Element{}
			flattenedKey   = key.FlattenedKey() // accounts for the Montgommery skip
		)

		for i := range key.gnarkInternal.A {
			ai := flattenedKey[i*key.OutputSize() : (i+1)*key.OutputSize()]
			si := make([]field.Element, key.OutputSize())
			copy(si, limbs[i*key.OutputSize():])
			tmp := poly.Mul(ai, si)
			recomputedHash = poly.Add(recomputedHash, tmp)
		}

		for i := key.OutputSize(); i < len(recomputedHash); i++ {
			recomputedHash[i-key.OutputSize()].Add(&recomputedHash[i-key.OutputSize()], &recomputedHash[i])
		}

		recomputedHash = recomputedHash[:key.OutputSize()]
		require.Equal(t, dualHash, recomputedHash)

	}

	for i, testCase := range testCasesKey {

		key := GenerateKey(testCasesKey[i].Params, testCase.Size)

		t.Run(fmt.Sprintf("case-%++v/all-ones", i), func(t *testing.T) {
			runTest(t, &key, vector.Repeat(field.One(), key.maxNumLimbsHashable()))
		})

		t.Run(fmt.Sprintf("case-%++v/all-zeroes", i), func(t *testing.T) {
			runTest(t, &key, vector.Repeat(field.Zero(), key.maxNumLimbsHashable()))
		})

		t.Run(fmt.Sprintf("case-%++v/rand-constant", i), func(t *testing.T) {
			var r field.Element
			r.SetRandom()
			runTest(t, &key, vector.Repeat(r, key.maxNumLimbsHashable()))
		})

		t.Run(fmt.Sprintf("case-%++v/full-rand", i), func(t *testing.T) {
			runTest(t, &key, vector.Rand(key.maxNumLimbsHashable()))
		})

		// ==== passing shorter vectors

		t.Run(fmt.Sprintf("case-%++v/all-ones-shorter", i), func(t *testing.T) {
			runTest(t, &key, vector.Repeat(field.One(), key.maxNumLimbsHashable()-1))
		})

		t.Run(fmt.Sprintf("case-%++v/all-zeroes-shorter", i), func(t *testing.T) {
			runTest(t, &key, vector.Repeat(field.Zero(), key.maxNumLimbsHashable()-1))
		})

		t.Run(fmt.Sprintf("case-%++v/rand-constant-shorter", i), func(t *testing.T) {
			var r field.Element
			r.SetRandom()
			runTest(t, &key, vector.Repeat(r, key.maxNumLimbsHashable()-1))
		})

		t.Run(fmt.Sprintf("case-%++v/full-rand-shorter", i), func(t *testing.T) {
			runTest(t, &key, vector.Rand(key.maxNumLimbsHashable()-1))
		})
	}
}

func TestLimbSplit(t *testing.T) {

	randX := mimc.BlockCompression(field.Zero(), field.Zero())
	randX.Inverse(&randX)

	arrays := [][]field.Element{
		{field.One()},
		{field.One(), field.One()},
		{field.NewElement(0), field.NewElement(1), field.NewElement(2), field.NewElement(3)},
		{field.NewElement(0xffffffffffffffff), field.NewFromString("-1"), field.NewElement(8)},
		{randX, field.One(), field.NewFromString("-1")},
	}

	coreTest := func(subT *testing.T, v []field.Element, key Key) {
		bound := field.NewElement(1 << key.LogTwoBound)
		limbs := key.LimbSplit(v)
		for i := range v {
			subLimbs := limbs[i*key.NumLimbs() : (i+1)*key.NumLimbs()]
			recomposed := poly.EvalUnivariate(subLimbs, bound)
			assert.Equal(subT, v[i].String(), recomposed.String())
		}
	}

	for arrID, arr := range arrays {
		for keyID, tcKey := range testCasesKey {
			key := GenerateKey(tcKey.Params, tcKey.Size)
			t.Run(
				fmt.Sprintf("array-#%v-key-#%v", arrID, keyID),
				func(t *testing.T) {
					coreTest(t, arr, key)
				},
			)
		}
	}
}

func TestHashFromLimbs(t *testing.T) {

	coreTest := func(t *testing.T, key *Key, v []field.Element) {
		hashed := key.Hash(v)
		limbs := key.LimbSplit(v)
		hashedFromLimbs := key.hashFromLimbs(limbs)
		assert.Equal(t, hashed, hashedFromLimbs)
	}

	testCaseVecs := [][]field.Element{
		{field.One()},
		{field.One(), field.One()},
		{field.NewElement(0), field.NewElement(1), field.NewElement(2), field.NewElement(3)},
		{field.NewElement(0xffffffffffffffff), field.NewFromString("-1"), field.NewElement(8)},
		vector.Rand(4),
		vector.Rand(16),
		vector.Rand(5),
		vector.Rand(17),
	}

	for pId, tcParams := range testCasesKey {
		for vecId, tcVec := range testCaseVecs {
			key := GenerateKey(tcParams.Params, len(tcVec))
			t.Run(fmt.Sprintf("params-#%v-vec-1%v", pId, vecId), func(t *testing.T) {
				coreTest(t, &key, tcVec)
			})
		}
	}
}

func TestTransveralHashFromLimbs(t *testing.T) {

	testCaseDimensions := []struct {
		NumRows, NumCols int
	}{
		{
			NumRows: 4,
			NumCols: 16,
		},
		{
			NumRows: 5,
			NumCols: 4,
		},
		{
			NumRows: 4,
			NumCols: 5,
		},
		{
			NumRows: 4,
			NumCols: 18,
		},
		{
			NumRows: 5,
			NumCols: 18,
		},
		{
			NumRows: 128,
			NumCols: 512,
		},
		{
			NumRows: 128,
			NumCols: 512 - 1,
		},
		{
			NumRows: 128,
			NumCols: 512 + 1,
		},
		{
			NumRows: 128 - 1,
			NumCols: 512,
		},
		{
			NumRows: 128 - 1,
			NumCols: 512 - 1,
		},
		{
			NumRows: 128 - 1,
			NumCols: 512 + 1,
		},
		{
			NumRows: 128 + 1,
			NumCols: 512,
		},
		{
			NumRows: 128 + 1,
			NumCols: 512 - 1,
		},
		{
			NumRows: 128 + 1,
			NumCols: 512 + 1,
		},
	}

	for pId, tcKeyParams := range testCasesKey {
		for _, tcDim := range testCaseDimensions {
			t.Run(
				fmt.Sprintf("params-%v-numRow=%v-nCols=%v", pId, tcDim.NumRows, tcDim.NumCols),
				func(t *testing.T) {

					key := GenerateKey(tcKeyParams.Params, tcDim.NumRows)

					inputs := make([]smartvectors.SmartVector, 4)
					for i := range inputs {
						inputs[i] = smartvectors.Rand(16)
					}

					transposed := make([][]field.Element, 16)
					for i := range transposed {
						transposed[i] = make([]fr.Element, 4)
						for j := range transposed[i] {
							transposed[i][j] = inputs[j].Get(i)
						}
					}

					res := key.TransversalHash(inputs)
					for i := range transposed {
						baseline := key.Hash(transposed[i])
						assert.Equal(t, baseline, res[i*key.OutputSize():(i+1)*key.OutputSize()])
					}

				},
			)
		}
	}
}

func TestHashLimbsFromSlice(t *testing.T) {

	for i, tcParams := range testCasesKey {
		t.Run(fmt.Sprintf("params-%v", i), func(t *testing.T) {

			var (
				key          = GenerateKey(tcParams.Params, tcParams.Size)
				inputs       = vector.Rand(tcParams.Size)
				keyVec       = key.FlattenedKey()
				limbs        = key.LimbSplit(inputs)
				expectedHash = key.hashFromLimbs(limbs)
				hashToTest   = hashLimbsWithSlice(keyVec, limbs, key.gnarkInternal.Domain, key.OutputSize())
			)

			require.Equal(t, vector.Prettify(expectedHash), vector.Prettify(hashToTest))

		})
	}
}

// For testing purposes
func hashLimbsWithSlice(keySlice []field.Element, limbs []field.Element, domain *fft.Domain, degree int) []field.Element {

	nbPolyUsed := utils.DivCeil(len(limbs), degree)

	if len(limbs) > len(keySlice) {
		utils.Panic("Too many inputs max is %v but has %v", len(limbs), len(keySlice))
	}

	// we can hash now.
	res := make([]field.Element, degree)

	// method 1: fft
	limbsChunk := make([]field.Element, degree)
	keyChunk := make([]field.Element, degree)
	for i := 0; i < nbPolyUsed; i++ {
		// extract the key element, without copying
		copy(keyChunk, keySlice[i*degree:(i+1)*degree])
		domain.FFT(keyChunk, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))

		// extract a copy of the limbs
		// The last poly may be incomplete
		copy(limbsChunk, limbs[i*degree:])

		// so we may need to zero-pad the last one if incomplete. This
		// loop will be skipped if limns[i*degree] is larger than
		// the degree
		for i := len(limbs[i*degree:]); i < degree; i++ {
			limbsChunk[i].SetZero()
		}

		domain.FFT(limbsChunk, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))

		var tmp field.Element
		for j := range res {
			tmp.Mul(&keyChunk[j], &limbsChunk[j])
			res[j].Add(&res[j], &tmp)
		}
	}

	domain.FFTInverse(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(1)) // -> reduces mod Xᵈ+1
	return res
}

// hashFromLimbs hashes a vector of limbs in Montgommery form. Unoptimized, only
// there for testing purpose.
func (key *Key) hashFromLimbs(limbs []field.Element) []field.Element {

	nbPolyUsed := utils.DivCeil(len(limbs), key.OutputSize())

	if nbPolyUsed > len(key.gnarkInternal.Ag) {
		utils.Panic("Too many inputs max is %v but has %v", len(key.gnarkInternal.Ag)*key.OutputSize(), len(limbs))
	}

	var (
		// res accumulates and stores the result of the hash as we compute it
		// throughout the function
		res = make([]field.Element, key.OutputSize())

		// k serves as a preallocated buffer aimed at storing a copy of the
		// limbs as we compute them. Its purpose is to minimize the number of
		// allocations made throughout the hashing procedure.
		k = make([]field.Element, key.modulusDegree())
	)

	for i := 0; i < nbPolyUsed; i++ {

		// This accounts for the fact that limbs may be smaller than the modulus
		// degree. When that happens, there won't be an OOB error but will need
		// to manually zeroize the buffer.
		copy(k, limbs[i*key.modulusDegree():])
		for i := len(limbs[i*key.modulusDegree():]); i < key.modulusDegree(); i++ {
			k[i].SetZero()
		}

		key.gnarkInternal.Domain.FFT(k, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))
		var tmp field.Element
		for j := range res {
			tmp.Mul(&k[j], &key.gnarkInternal.Ag[i][j])
			res[j].Add(&res[j], &tmp)
		}
	}

	// Since the Ag are normally assumed to work with non-montgomery limbs
	// (when doing normal hashing)
	for j := range res {
		res[j] = field.MulRInv(res[j])
	}

	key.gnarkInternal.Domain.FFTInverse(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(1)) // -> reduces mod Xᵈ+1
	return res
}
