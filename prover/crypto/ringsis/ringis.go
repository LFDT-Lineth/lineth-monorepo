package ringsis

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/sis"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	// The seed of the ring sis key
	RING_SIS_SEED int64 = 42069
)

// Key encapsulates the public parameters of an instance of the ring-SIS hash
// instance.
type Key struct {
	// gnarkInternal stores the SIS key itself and some precomputed domain
	// twiddles.
	gnarkInternal *sis.RSis
	// Params provides the parameters of the ring-SIS instance (logTwoBound,
	// degree etc)
	Params
}

// GenerateKey generates a ring-SIS key from a set of a [Params] and a max
// number of elements to hash
func GenerateKey(params Params, maxNumFieldToHash int) Key {

	// Sanity-check : if the logTwoBound is larger or equal than 64 it can
	// create overflows as we cast the small-norm limbs into
	if params.LogTwoBound >= 64 {
		utils.Panic("Log two bound cannot be larger than 64")
	}

	rsis, err := sis.NewRSis(RING_SIS_SEED, params.LogTwoDegree, params.LogTwoBound, maxNumFieldToHash)
	if err != nil {
		panic(err)
	}

	res := Key{
		gnarkInternal: rsis,
		Params:        params,
	}

	return res
}

// Ag returns the SIS key
func (s *Key) Ag() [][]field.Element {
	return s.gnarkInternal.Ag
}

// Hash interprets the input vector as a sequence of coefficients of size
// r.LogTwoBound bits long and returns the hash of the polynomial corresponding
// to the sum sum_i A[i]*m Mod X^{d}+1
//
// It is equivalent to calling r.Write(element.Marshal()); outBytes = r.Sum(nil);
func (s *Key) Hash(v []field.Element) []field.Element {

	// write the input as byte
	sum := make([]field.Element, s.OutputSize())
	if err := s.gnarkInternal.Hash(v, sum); err != nil {
		panic(err)
	}
	return sum
}

// LimbSplit breaks down the entries of `v` into short limbs representing
// `LogTwoBound` bits each. The function then flatten and flatten them in a
// vector, casted as field elements in Montgommery form.
func (s *Key) LimbSplit(vReg []field.Element) []field.Element {
	m := make([]field.Element, len(vReg)*s.NumLimbs())

	it := sis.NewLimbIterator(sis.NewVectorIterator(vReg), s.LogTwoBound/8)

	// The limbs are in regular form, we reconvert them back into montgommery
	// form
	var ok bool
	for i := range m {
		m[i][0], ok = it.NextLimb()
		if !ok {
			// the rest is 0 we can stop (note that if we change the padding
			// policy we may need to change this)
			break
		}
		m[i] = field.MulR(m[i])
	}

	return m
}

// HashModXnMinus1 applies the SIS hash modulo X^n - 1, (instead of X^n + 1).
// This is used as part of the self-recursion procedure. Note that this **does
// not implement** a collision-resistant hash function and it is meant to. Its
// purpose is to help constructing a witness for the correctness of computation
// of a batch of SIS hashes.
//
// The vector of limbs has to be provided in Montgommery form.
func (s Key) HashModXnMinus1(limbs []field.Element) []field.Element {

	// inputReader is a subslice of `limbs` and it is meant to be used as a
	// reader. We periodically pop the first element by reassigning the input
	// reader to `inputReader[1:]`.
	inputReader := limbs

	if len(limbs) > s.maxNumLimbsHashable() {
		utils.Panic("Wrong size : %v > %v", len(limbs), s.maxNumLimbsHashable())
	}

	nbPolyUsed := utils.DivCeil(len(limbs), s.modulusDegree())

	/*
		To perform the modulo X^n - 1 operation, we use the FFT method

		Let a, k, r be vectors of size d

		k <- FFT(k)
		a <- FFT(a)
		r += k * a (element-wise)
		r <- FFTInv(r)
	*/

	domain := s.gnarkInternal.Domain
	k := make([]field.Element, s.modulusDegree())
	a := make([]field.Element, s.modulusDegree())
	r := make([]field.Element, s.OutputSize())

	for i := 0; i < nbPolyUsed; i++ {
		copy(a, s.gnarkInternal.A[i])
		copy(k, inputReader)

		// consume the "reader"
		if i < nbPolyUsed-1 {
			inputReader = inputReader[s.modulusDegree():]
		} else {
			// we may need to zero-pad the last one if incomplete
			for i := len(inputReader); i < s.modulusDegree(); i++ {
				k[i].SetZero()
			}
			// we can give the inputReader back
			inputReader = nil
		}

		domain.FFT(k, fft.DIF)
		domain.FFT(a, fft.DIF)

		var tmp field.Element
		for i := range r {
			tmp.Mul(&k[i], &a[i])
			r[i].Add(&r[i], &tmp)
		}
	}

	if len(inputReader) > 0 {
		utils.Panic("%v elements remains in the reader after hashing", len(inputReader))
	}

	// by linearity, we defer the fft inverse at the end
	domain.FFTInverse(r, fft.DIT)

	// also account for the Montgommery issue : in gnark's implementation
	// the key is implictly multiplied by RInv
	for i := range r {
		r[i] = field.MulRInv(r[i])
	}

	return r
}

// FlattenedKey returns the SIS key multiplied by RInv and laid out in a
// single-vector of field elements. This function is meant to be used as part
// of the self-recursion of Vortex and serves constructing an assignment for
// the column storing the chunks of the SIS key.
//
// The function stores the key polynomial per polynomials. In coefficient form
// and multiplied by RInv so that it can account for the "Montgommery skip".
// See [Key.Hash] for more details.
func (s *Key) FlattenedKey() []field.Element {
	res := make([]field.Element, 0, s.maxNumLimbsHashable())
	for i := range s.gnarkInternal.A {
		for j := range s.gnarkInternal.A[i] {
			t := s.gnarkInternal.A[i][j]
			t = field.MulRInv(t)
			res = append(res, t)
		}
	}
	return res
}
