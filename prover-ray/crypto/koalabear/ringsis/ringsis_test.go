package ringsis

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/consensys/linea-monorepo/prover-ray/utils"
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

	runTest := func(t *testing.T, key *Key, limbs []field.Ext) {
		t.Helper()

		var (
			dualHash       = key.HashModXnMinus1(limbs)
			recomputedHash = []field.Ext{}
			flattenedKey   = key.FlattenedKey() // accounts for the Montgommery skip
		)

		for i := range key.SisGnarkCrypto.A {
			ai := flattenedKey[i*key.OutputSize() : (i+1)*key.OutputSize()]
			si := make([]field.Ext, key.OutputSize())
			copy(si, limbs[i*key.OutputSize():])
			tmp := testPolyMulExtBase(si, ai)
			recomputedHash = testPolyAddExt(recomputedHash, tmp)
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
			runTest(t, key, field.VecRepeatExt(field.OneExt(), key.maxNumLimbsHashable()))
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

	coreTest := func(t *testing.T, v []field.Element, key Key) {
		t.Helper()
		bound := field.NewElement(1 << key.LogTwoBound())
		limbs := key.LimbSplit(v)
		for i := range v {
			subLimbs := limbs[i*key.NumLimbs() : (i+1)*key.NumLimbs()]
			recomposed := polynomials.EvalCanonical(field.VecFromBase(subLimbs), field.ElemFromBase(bound))
			got := recomposed.AsBase()
			assert.Equal(t, v[i].String(), got.String())
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
		t.Helper()
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
		field.VecRandomBase(4),
		field.VecRandomBase(16),
		field.VecRandomBase(5),
		field.VecRandomBase(17),
	}

	for pID, tcParams := range testCasesKey {
		for vecID, tcVec := range testCaseVecs {
			key := GenerateKey(tcParams.LogTwoDegree, tcParams.LogTwoBound, len(tcVec))
			t.Run(fmt.Sprintf("params-#%v-vec-1%v", pID, vecID), func(t *testing.T) {
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
				inputs       = field.VecRandomBase(tcParams.Size)
				keyVec       = key.FlattenedKey()
				limbs        = key.LimbSplit(inputs)
				expectedHash = key.hashFromLimbs(limbs)
				hashToTest   = hashLimbsWithSlice(keyVec, limbs, key.SisGnarkCrypto.Domain, key.OutputSize())
			)

			require.Equal(t, field.VecPrettifyBase(expectedHash), field.VecPrettifyBase(hashToTest))

		})
	}
}

// testPolyMulExtBase computes the polynomial product of p (ext coefficients) and
// q (base coefficients) in the coefficient domain. The result has length
// len(p)+len(q)-1.
func testPolyMulExtBase(p []field.Ext, q []field.Element) []field.Ext {
	if len(p) == 0 || len(q) == 0 {
		return nil
	}
	res := make([]field.Ext, len(p)+len(q)-1)
	for i, pi := range p {
		for j, qj := range q {
			var term field.Ext
			term.MulByElement(&pi, &qj)
			res[i+j].Add(&res[i+j], &term)
		}
	}
	return res
}

// testPolyAddExt adds two ext-coefficient polynomials of possibly different
// lengths, returning a result of length max(len(a), len(b)).
func testPolyAddExt(a, b []field.Ext) []field.Ext {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	res := make([]field.Ext, maxLen)
	copy(res, a)
	for i, bi := range b {
		res[i].Add(&res[i], &bi)
	}
	return res
}

// For testing purposes
func hashLimbsWithSlice(keySlice []field.Element, limbs []field.Element, domain *fft.Domain,
	degree int) []field.Element {

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
		// loop will be skipped if limbs[i*degree] is larger than the degree
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
func (k *Key) hashFromLimbs(limbs []field.Element) []field.Element {

	nbPolyUsed := utils.DivCeil(len(limbs), k.OutputSize())

	if nbPolyUsed > len(k.SisGnarkCrypto.Ag) {
		utils.Panic("Too many inputs max is %v but has %v", len(k.SisGnarkCrypto.Ag)*k.OutputSize(), len(limbs))
	}

	var (
		// res accumulates and stores the result of the hash as we compute it
		// throughout the function
		res = make([]field.Element, k.OutputSize())

		// preallocatedBuffer serves as a preallocated buffer aimed at storing a copy of the
		// limbs as we compute them. Its purpose is to minimize the number of
		// allocations made throughout the hashing procedure.
		preallocatedBuffer = make([]field.Element, k.modulusDegree())
	)

	for i := 0; i < nbPolyUsed; i++ {

		// This accounts for the fact that limbs may be smaller than the modulus
		// degree. When that happens, there won't be an OOB error but will need
		// to manually zeroize the buffer.
		copy(preallocatedBuffer, limbs[i*k.modulusDegree():])
		for i := len(limbs[i*k.modulusDegree():]); i < k.modulusDegree(); i++ {
			preallocatedBuffer[i].SetZero()
		}

		k.SisGnarkCrypto.Domain.FFT(preallocatedBuffer, fft.DIF, fft.OnCoset(), fft.WithNbTasks(1))
		var tmp field.Element
		for j := range res {
			tmp.Mul(&preallocatedBuffer[j], &k.SisGnarkCrypto.Ag[i][j])
			res[j].Add(&res[j], &tmp)
		}
	}

	// Since the Ag are normally assumed to work with non-montgomery limbs
	// (when doing normal hashing)

	for j := range res {
		res[j] = field.MulRInv(res[j])
	}

	k.SisGnarkCrypto.Domain.FFTInverse(res, fft.DIT, fft.OnCoset(), fft.WithNbTasks(1)) // -> reduces mod Xᵈ+1
	return res
}
