package ringsis

import (
	"runtime"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/sis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/arena"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/parallel"
)

const (
	// The seed of the ring sis key
	RING_SIS_SEED int64 = 42069
)

// Key encapsulates the public parameters of an instance of the ring-SIS hash
// instance.
type Key struct {
	// Params provides the parameters of the ring-SIS instance (logTwoBound,
	// degree etc)
	*KeyGen

	// lock guards the access to the SIS key and prevents the user from hashing
	// concurrently with the same SIS key.
	lock *sync.Mutex `serde:"omit"`
	// gnarkInternal stores the SIS key itself and some precomputed domain
	// twiddles.
	gnarkInternal *sis.RSis `serde:"omit"`

	// twiddleCosets stores the list of twiddles that we use to implement the
	// SIS parameters. The twiddleAreInternally are only used when dealing with
	// the parameters modulusDegree=64 and logTwoBound=8 and is passed as input
	// to the specially unrolled [sis.FFT64] function. They are thus optionally
	// constructed when [GenerateKey] is called.
}

type KeyGen struct {
	*Params
	MaxNumFieldToHash int
}

// GenerateKey generates a ring-SIS key from a set of a [Params] and a max
// number of elements to hash
func GenerateKey(params Params, maxNumFieldToHash int) *Key {

	// Sanity-check : if the logTwoBound is larger or equal than 64 it can
	// create overflows as we cast the small-norm limbs into
	if params.LogTwoBound >= 64 {
		utils.Panic("Log two bound cannot be larger than 64")
	}

	rsis, err := sis.NewRSis(RING_SIS_SEED, params.LogTwoDegree, params.LogTwoBound, maxNumFieldToHash)
	if err != nil {
		panic(err)
	}

	res := &Key{
		lock:          &sync.Mutex{},
		gnarkInternal: rsis,
		KeyGen: &KeyGen{
			Params:            &params,
			MaxNumFieldToHash: maxNumFieldToHash,
		},
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

	// since hashing writes into internal buffers
	// we need to guard against races conditions.
	s.lock.Lock()
	defer s.lock.Unlock()

	res := make([]field.Element, s.OutputSize())
	if err := s.gnarkInternal.Hash(v, res); err != nil {
		panic(err)
	}

	return res
}

// LimbSplit breaks down the entries of `v` into short limbs representing
// `LogTwoBound` bits each. The function then flatten and flatten them in a
// vector, casted as field elements in Montgommery form.
func (s *Key) LimbSplit(vReg []field.Element) []field.Element {

	it := sis.NewLimbIterator(sis.NewVectorIterator(vReg), s.LogTwoBound/8)

	var (
		numLimbs = len(vReg) * field.Bytes * 8 / s.LogTwoBound
		res      = make([]field.Element, 0, numLimbs)
	)

	for i := 0; i < numLimbs; i++ {

		limb, ok := it.NextLimb()
		if !ok {
			break
		}

		var v field.Element
		v.SetUint64(limb)
		res = append(res, v)
	}

	return res
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

// TransversalHash evaluates SIS hashes transversally over a list of smart-vectors.
// Each smart-vector is seen as the row of a matrix. All rows must have the same
// size or panic. The function returns the hash of the columns. The column hashes
// are concatenated into a single array.
//
// The function is optimize to deal with the ring-SIS instances parametrized by
//
//   - modulus degree: 	64  log2(bound): 	8
//   - modulus degree: 	64  log2(bound): 	16
//   - modulus degree: 	32  log2(bound): 	8
func (s *Key) TransversalHash(v []smartvectors.SmartVector) []field.Element {
	// nbRows is the number of rows in the matrix
	nbRows := len(v)
	if nbRows == 0 || nbRows > s.MaxNumFieldToHash {
		utils.Panic("nbRows=%v out of bounds [1,%v]", nbRows, s.MaxNumFieldToHash)
	}

	// nbCols is the number of columns in the matrix
	nbCols := v[0].Len()
	if nbCols == 0 {
		utils.Panic("Provided a 0-colums matrix")
	}

	for i := range v {
		if v[i].Len() != nbCols {
			utils.Panic("Unexpected : all inputs smart-vectors should have the same length the first one has length %v, but #%v has length %v",
				nbCols, i, v[i].Len())
		}
	}

	sisKeySize := s.OutputSize()

	res := make([]field.Element, nbCols*sisKeySize)
	transposedArena := arena.NewVectorArena[field.Element](nbRows * 2 * runtime.GOMAXPROCS(0))
	parallel.Execute(nbCols, func(start, end int) {
		// we transpose the columns using a windowed approach
		// this is done to improve memory accesses when transposing the matrix

		// perf note; we could allocate only blocks of 256 elements here and do the SIS hash
		// block by block, but surprisingly it is slower than the current implementation
		// it would however save some memory allocation.
		windowSize := 2
		n := end - start
		for n%windowSize != 0 {
			windowSize /= 2
		}
		// using arena here just favors contiguous memory allocation

		transposed := make([][]field.Element, windowSize)
		for i := range transposed {
			transposed[i] = arena.Get[field.Element](transposedArena, nbRows)
		}
		for col := start; col < end; col += windowSize {
			for i := 0; i < nbRows; i++ {
				switch vi := v[i].(type) {
				case *smartvectors.Constant:
					cst := vi.Value
					for j := range transposed {
						transposed[j][i] = cst
					}
				case *smartvectors.Regular:
					for j := range transposed {
						transposed[j][i] = (*vi)[col+j]
					}
				default:
					for j := range transposed {
						transposed[j][i] = v[i].Get(col + j)
					}
				}
			}
			for j := range transposed {
				s.gnarkInternal.Hash(transposed[j], res[(col+j)*sisKeySize:(col+j)*sisKeySize+sisKeySize])
			}
		}
	})

	return res
}
