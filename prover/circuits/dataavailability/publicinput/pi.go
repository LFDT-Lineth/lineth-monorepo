package publicinput

import (
	"fmt"
	"math/big"
	"math/bits"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func interpolateLagrangeBls12381(field *emulated.Field[emulated.BLS12381Fr], unitCircleEvaluations []*emulated.Element[emulated.BLS12381Fr], evaluationPoint *emulated.Element[emulated.BLS12381Fr]) (evaluation *emulated.Element[emulated.BLS12381Fr], err error) {
	logN := bits.Len(uint(len(unitCircleEvaluations))) - 1
	if len(unitCircleEvaluations) != 1<<logN {
		return nil, fmt.Errorf("expected power of 2 number of evaluations, got %d", len(unitCircleEvaluations))
	}

	x, nInv := genInterpolateLagrangeParams(len(unitCircleEvaluations))

	summands := make([]*emulated.Element[emulated.BLS12381Fr], len(unitCircleEvaluations))
	for i := range summands {
		xI := field.NewElement(x[i])
		summands[i] = field.Div(
			field.Mul(unitCircleEvaluations[i], xI),
			field.Sub(evaluationPoint, xI),
		)
	}

	minPolyAtEp := evaluationPoint
	for i := 0; i < logN; i++ {
		minPolyAtEp = field.Mul(minPolyAtEp, minPolyAtEp)
	}
	minPolyAtEp = field.Sub(minPolyAtEp, field.NewElement(1))

	coeff := field.Mul(field.NewElement(nInv), minPolyAtEp)

	return field.Mul(coeff, sum(field, summands...)), nil
}

func sum[T emulated.FieldParams](field *emulated.Field[T], elems ...*emulated.Element[T]) *emulated.Element[T] {
	switch len(elems) {
	case 0:
		return field.NewElement(0)
	case 1:
		return elems[0]
	}
	res := field.Add(elems[0], elems[1])
	for i := 2; i < len(elems); i++ {
		res = field.Add(res, elems[i])
	}
	return res
}

// compile time parameter generation
// returns the nth roots of unity and 1/n
func genInterpolateLagrangeParams(n int) (x []fr381.Element, nInv fr381.Element) {
	x = make([]fr381.Element, n)
	x1, err := fr381.Generator(uint64(n)) // x1 is the canonical nth root of unity
	if err != nil {
		panic(err)
	}
	x[0].SetOne()
	x[1] = x1
	for i := 2; i < len(x); i++ {
		x[i].Mul(&x[i-1], &x1)
	}

	nInv.SetUint64(uint64(n)).Inverse(&nInv)

	return
}

// VerifyBlobConsistency opens the "commitment" to the blob at evaluationChallenge; if bypassEip4844 is set, it does so in a KZG-like manner using a Lagrange basis on the unit circle. if not, a Reed-Solomon type method is used.
// TODO consider using the batch hashes as "snarkHash" instead of hashing the data here to save on constraints
func VerifyBlobConsistency(api frontend.API, blobCrumbs []frontend.Variable, evaluationChallenge [32]frontend.Variable, eip4844Enabled frontend.Variable) (evaluation [2]frontend.Variable, err error) {
	snarkFieldLen := api.Compiler().Field().BitLen()
	if snarkFieldLen >= fr381.Bits {
		err = fmt.Errorf("large field moduli ( %d≥%d ) not yet supported", snarkFieldLen, fr381.Bits)
		return
	}

	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	if err != nil {
		return
	}

	blobEmulated := packCrumbsEmulated(api, blobCrumbs) // perf-TODO use the original blob bytes
	evaluationChallengeEmulated := utils.NewElementFromBytes[emulated.BLS12381Fr](api, evaluationChallenge[:])

	blobEmulatedBitReversed := make([]*emulated.Element[emulated.BLS12381Fr], len(blobEmulated))
	copy(blobEmulatedBitReversed, blobEmulated)
	bitReverseSlice(blobEmulatedBitReversed)
	lagrangeEval, err := interpolateLagrangeBls12381(field, blobEmulatedBitReversed, evaluationChallengeEmulated)
	if err != nil {
		return
	}

	polyEval, err := evalPolyBls12381(field, blobEmulated, evaluationChallengeEmulated)
	if err != nil {
		return
	}
	l := bls12381ScalarToBls12377Scalars(api, lagrangeEval)
	p := bls12381ScalarToBls12377Scalars(api, polyEval)

	api.AssertIsBoolean(eip4844Enabled)
	evaluation[0] = api.Select(eip4844Enabled, l[0], p[0])
	evaluation[1] = api.Select(eip4844Enabled, l[1], p[1])

	return
}

func evalPolyBls12381(field *emulated.Field[emulated.BLS12381Fr], coeffs []*emulated.Element[emulated.BLS12381Fr], evaluationPoint *emulated.Element[emulated.BLS12381Fr]) (evaluation *emulated.Element[emulated.BLS12381Fr], err error) {
	// lower degree coeff first
	evaluation = coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		evaluation = field.Mul(evaluation, evaluationPoint)
		evaluation = field.Add(evaluation, coeffs[i])
	}
	return
}

func mapSlice[X, Y any](slice []X, f func(X) Y) []Y {
	res := make([]Y, len(slice))
	for i, x := range slice {
		res[i] = f(x)
	}
	return res
}

func bitReverse(n, logN int) int {
	return utils.ToInt(bits.Reverse64(uint64(n)) >> (64 - logN))
}

func bitReverseSlice[K interface{}](list []K) {
	n := uint64(len(list))

	// The standard library's bits.Reverse64 inverts its input as a 64-bit unsigned integer.
	// However, we need to invert it as a log2(len(list))-bit integer, so we need to correct this by
	// shifting appropriately.
	shiftCorrection := uint64(64 - bits.TrailingZeros64(n))

	for i := uint64(0); i < n; i++ {
		// Find index irev, such that i and irev get swapped
		irev := bits.Reverse64(i) >> shiftCorrection
		if irev > i {
			list[i], list[irev] = list[irev], list[i]
		}
	}
}

func packCrumbsEmulated(api frontend.API, words []frontend.Variable) []*emulated.Element[emulated.BLS12381Fr] {
	var fieldParams emulated.BLS12381Fr
	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	if err != nil {
		panic(err)
	}
	const bitsPerWord = 2
	const bitsPerElem = fr381.Bits - 1
	const wordsPerElem = bitsPerElem / bitsPerWord

	res := make([]*emulated.Element[emulated.BLS12381Fr], (len(words)+wordsPerElem-1)/wordsPerElem)
	if len(words) != len(res)*wordsPerElem {
		tmp := words
		words = make([]frontend.Variable, len(res)*wordsPerElem)
		copy(words, tmp)
		for i := len(tmp); i < len(words); i++ {
			words[i] = 0
		}
	}

	nbLimbs := int(fieldParams.NbLimbs())                       // #nosec G115 -- Small number of limbs expected
	limbNbWords := int(fieldParams.BitsPerLimb()) / bitsPerWord // #nosec G115 -- Expected to be 64
	if uint(limbNbWords*bitsPerWord) != fieldParams.BitsPerLimb() {
		panic("bitsPerWord must divide bitsPerLimb")
	}
	msLimbNbWords := wordsPerElem - (nbLimbs-1)*limbNbWords
	if msLimbNbWords < 0 || msLimbNbWords >= limbNbWords {
		panic("remainder out of range")
	}

	radix := new(big.Int).Lsh(big.NewInt(1), uint(bitsPerWord))
	limbs := make([]frontend.Variable, nbLimbs*len(res))

	for i := range res {
		currLimbs := limbs[i*nbLimbs : (i+1)*nbLimbs]
		currLimbs[len(currLimbs)-1] = compress.ReadNum(api, words[:msLimbNbWords], radix)
		words = words[msLimbNbWords:]
		for j := 1; j < nbLimbs; j++ {
			currLimbs[len(currLimbs)-1-j] = compress.ReadNum(api, words[:limbNbWords], radix)
			words = words[limbNbWords:]
		}

		res[i] = field.NewElement(currLimbs)
	}
	return res
}

func bls12381ScalarToBls12377Scalars(api frontend.API, e *emulated.Element[emulated.BLS12381Fr]) (r [2]frontend.Variable) {
	field, err := emulated.NewField[emulated.BLS12381Fr](api)
	if err != nil {
		panic(err)
	}
	const nbLowerBits = fr381.Bytes / 2 * 8
	e = field.Reduce(e)
	bts := field.ToBits(e)
	r[1] = api.FromBinary(bts[:nbLowerBits]...)
	r[0] = api.FromBinary(bts[nbLowerBits:]...)
	return
}
