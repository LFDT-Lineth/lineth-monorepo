package ringsis

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"

	"github.com/consensys/linea-monorepo/prover/maths/common/poly"
	"github.com/consensys/linea-monorepo/prover/maths/common/polyext"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// var StdParams = Params{LogTwoBound: 16, LogTwoDegree: 9}
var testCasesKey = []struct {
	Size         int
	LogTwoBound  int
	LogTwoDegree int
}{
	{
		Size:         2,
		LogTwoBound:  16,
		LogTwoDegree: 9,
	},
	{
		Size:         100,
		LogTwoBound:  16,
		LogTwoDegree: 9,
	},
	{
		Size:         32,
		LogTwoBound:  16,
		LogTwoDegree: 9,
	},
	{
		Size:         5,
		LogTwoBound:  16,
		LogTwoDegree: 9,
	},
	{
		Size:         576,
		LogTwoBound:  16,
		LogTwoDegree: 9,
	},
	{
		Size: 43,

		LogTwoBound:  8,
		LogTwoDegree: 5,
	},
	{
		Size: 23,

		LogTwoBound:  8,
		LogTwoDegree: 6,
	},
	{
		Size: 256,

		LogTwoBound:  8,
		LogTwoDegree: 6,
	},
}

func TestHashModXnMinusOne(t *testing.T) {

	runTest := func(t *testing.T, key *Key, limbs []fext.Element) {

		var (
			dualHash       = key.HashModXnMinus1(limbs)
			recomputedHash = []fext.Element{}
			flattenedKey   = key.FlattenedKey() // accounts for the Montgommery skip
		)

		for i := range key.SisGnarkCrypto.A {
			ai := flattenedKey[i*key.OutputSize() : (i+1)*key.OutputSize()]
			si := make([]fext.Element, key.OutputSize())
			copy(si, limbs[i*key.OutputSize():])
			tmp := polyext.MulByElement(si, ai)
			recomputedHash = polyext.Add(recomputedHash, tmp)
		}

		for i := key.OutputSize(); i < len(recomputedHash); i++ {
			recomputedHash[i-key.OutputSize()].Add(&recomputedHash[i-key.OutputSize()], &recomputedHash[i])
		}

		recomputedHash = recomputedHash[:key.OutputSize()]
		require.Equal(t, dualHash, recomputedHash)

	}

	for i, testCase := range testCasesKey {

		key := GenerateKey(testCasesKey[i].LogTwoDegree, testCasesKey[i].LogTwoBound, testCase.Size)

		t.Run(fmt.Sprintf("case-%++v/all-ones", i), func(t *testing.T) {
			runTest(t, key, vectorext.Repeat(fext.One(), key.maxNumLimbsHashable()))
		})

	}

}

func TestLimbSplit(t *testing.T) {
	var rng = rand.New(utils.NewRandSource(0)) // #nosec G404
	randX := field.PseudoRand(rng)

	arrays := [][]field.Element{
		{field.One()},
		{field.One(), field.One()},
		{field.NewElement(0), field.NewElement(1), field.NewElement(2), field.NewElement(3)},
		{field.NewElement(0xffffffffffffffff), field.NewFromString("-1"), field.NewElement(8)},
		{randX, field.One(), field.NewFromString("-1")},
	}

	coreTest := func(subT *testing.T, v []field.Element, key Key) {
		bound := field.NewElement(1 << key.LogTwoBound())
		limbs := key.LimbSplit(v)
		for i := range v {
			subLimbs := limbs[i*key.NumLimbs() : (i+1)*key.NumLimbs()]
			recomposed := poly.Eval(subLimbs, bound)
			assert.Equal(subT, v[i].String(), recomposed.String())
		}
	}

	for arrID, arr := range arrays {
		for keyID, tcKey := range testCasesKey {
			key := GenerateKey(tcKey.LogTwoDegree, tcKey.LogTwoBound, tcKey.Size)
			t.Run(
				fmt.Sprintf("array-#%v-key-#%v", arrID, keyID),
				func(t *testing.T) {
					coreTest(t, arr, *key)
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
			key := GenerateKey(tcParams.LogTwoDegree, tcParams.LogTwoBound, len(tcVec))
			t.Run(fmt.Sprintf("params-#%v-vec-1%v", pId, vecId), func(t *testing.T) {
				coreTest(t, key, tcVec)
			})
		}
	}
}

func TestHashLimbsFromSlice(t *testing.T) {

	for i, tcParams := range testCasesKey {
		t.Run(fmt.Sprintf("params-%v", i), func(t *testing.T) {

			var (
				key          = GenerateKey(tcParams.LogTwoDegree, tcParams.LogTwoBound, tcParams.Size)
				inputs       = vector.Rand(tcParams.Size)
				keyVec       = key.FlattenedKey()
				limbs        = key.LimbSplit(inputs)
				expectedHash = key.hashFromLimbs(limbs)
				hashToTest   = hashLimbsWithSlice(keyVec, limbs, key.SisGnarkCrypto.Domain, key.OutputSize())
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

	if nbPolyUsed > len(key.SisGnarkCrypto.Ag) {
		utils.Panic("Too many inputs max is %v but has %v", len(key.SisGnarkCrypto.Ag)*key.OutputSize(), len(limbs))
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

		key.SisGnarkCrypto.Domain.FFT(k, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))
		var tmp field.Element
		for j := range res {
			tmp.Mul(&k[j], &key.SisGnarkCrypto.Ag[i][j])
			res[j].Add(&res[j], &tmp)
		}
	}

	// Since the Ag are normally assumed to work with non-montgomery limbs
	// (when doing normal hashing)

	for j := range res {
		res[j] = field.MulRInv(res[j])
	}

	key.SisGnarkCrypto.Domain.FFTInverse(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(1)) // -> reduces mod Xᵈ+1
	return res
}
