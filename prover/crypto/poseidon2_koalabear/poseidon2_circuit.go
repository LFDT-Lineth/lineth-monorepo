package poseidon2_koalabear

import (
	"errors"
	"slices"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/frontend"
)

// GnarkKoalaHasher is an interface implemented by structures that can compute
// Poseidon2 hashes based on koalabear.
type GnarkKoalaHasher interface {
	Reset()
	Write(data ...frontend.Variable)
	WriteOctuplet(data ...GnarkOctuplet)
	SetState(state GnarkOctuplet)
	State() GnarkOctuplet
	Sum() GnarkOctuplet
}

// GnarkOctuplet is an octuplet of frontend.Variable
type GnarkOctuplet [8]frontend.Variable

// GnarkMDHasher Merkle Damgard implementation using poseidon2 as compression function with width 16
// The hashing process goes as follow:
// newState := compress(old state, buf) where buf is an octuplet, left padded with zeroes if needed
type GnarkMDHasher struct {
	api frontend.API

	// Sponge construction state
	state GnarkOctuplet

	// data to hash
	buffer []frontend.Variable
}

// NewGnarkMDHasher returns a new Octuplet
func NewGnarkMDHasher(api frontend.API) (GnarkMDHasher, error) {
	var res GnarkMDHasher
	for i := 0; i < 8; i++ {
		res.state[i] = 0
	}
	res.api = api
	return res, nil
}

func (h *GnarkMDHasher) Reset() {
	h.buffer = h.buffer[:0]
	for i := 0; i < 8; i++ {
		h.state[i] = 0
	}
}

func (h *GnarkMDHasher) Write(data ...frontend.Variable) {
	h.buffer = append(h.buffer, data...)
}

func (h *GnarkMDHasher) WriteOctuplet(data ...GnarkOctuplet) {
	for i := 0; i < len(data); i++ {
		h.buffer = append(h.buffer, data[i][:]...)
	}
}

func (h *GnarkMDHasher) SetState(state GnarkOctuplet) {
	h.Reset()
	copy(h.state[:], state[:])
}

func (h *GnarkMDHasher) State() GnarkOctuplet {

	// State will flush the buffer, take the state and restore the initiat
	// state of the hasher.
	oldState := h.state
	oldBuffer := slices.Clone(h.buffer)

	_ = h.Sum() // this flushes the hasher
	res := h.state

	h.state = oldState
	h.buffer = oldBuffer

	return res
}

func (h *GnarkMDHasher) Sum() GnarkOctuplet {

	for len(h.buffer) != 0 {
		var buf [BlockSize]frontend.Variable
		for i := 0; i < BlockSize; i++ {
			buf[i] = 0
		}
		// in this case we left pad by zeroes
		if len(h.buffer) < BlockSize {
			copy(buf[BlockSize-len(h.buffer):], h.buffer)
			h.buffer = h.buffer[:0]
		} else {
			copy(buf[:], h.buffer)
			h.buffer = h.buffer[BlockSize:]
		}

		h.state = CompressPoseidon2(h.api, h.state, buf)
	}
	return h.state
}

func CompressPoseidon2(api frontend.API, a, b GnarkOctuplet) GnarkOctuplet {
	res := GnarkOctuplet{}

	var x [16]frontend.Variable
	copy(x[:], a[:])
	copy(x[8:], b[:])

	// Create a buffer to hold the feed-forward input.
	copy(res[:], x[8:])
	if err := compressPerm.Permutation(api, x[:]); err != nil {
		// can't error (size is correct)
		panic(err)
	}

	for i := range res {
		res[i] = api.Add(res[i], x[8+i])
	}
	return res
}

var (
	ErrInvalidSizebuffer = errors.New("the size of the input should match the size of the hash buffer")
	compressPerm         permutation
	once                 sync.Once
)

func init() {
	once.Do(func() {
		compressPerm = NewPermutation()
	})
}

func NewPermutation() permutation {
	// same params than gnark-crypto/field/koalabear/vortex/hash.go
	params := poseidon2.NewParameters(16, 6, 21)
	return permutation{params: params}
}

type permutation struct {
	params *poseidon2.Parameters
}

// sBox applies the sBox on buffer[index]
func (h *permutation) sBox(api frontend.API, index int, input []frontend.Variable) {
	// sbox degree is 3
	tmp := input[index]
	input[index] = api.Mul(input[index], input[index])
	input[index] = api.Mul(tmp, input[index])
}

// matMulM4 computes
// s <- M4*s
// where M4=
// (2 3 1 1)
// (1 2 3 1)
// (1 1 2 3)
// (3 1 1 2)
// on chunks of 4 elements on each part of the buffer
// see https://eprint.iacr.org/2023/323.pdf appendix B for the addition chain
func (h *permutation) matMulM4InPlace(api frontend.API, s []frontend.Variable) {

	c := len(s) / 4
	for i := 0; i < c; i++ {
		var t01, t23, t0123, t01123, t01233 frontend.Variable
		t01 = api.Add(s[4*i], s[4*i+1])
		t23 = api.Add(s[4*i+2], s[4*i+3])
		t0123 = api.Add(t01, t23)
		t01123 = api.Add(t0123, s[4*i+1])
		t01233 = api.Add(t0123, s[4*i+3])
		// The order here is important. Need to overwrite x[0] and x[2] after x[1] and x[3].
		s[4*i+3] = api.Add(s[4*i], s[4*i])
		s[4*i+3] = api.Add(s[4*i+3], t01233)
		s[4*i+1] = api.Add(s[4*i+2], s[4*i+2])
		s[4*i+1] = api.Add(s[4*i+1], t01123)
		s[4*i] = api.Add(t01, t01123)
		s[4*i+2] = api.Add(t23, t01233)
	}
}

// when t=2,3 the buffer is multiplied by circ(2,1) and circ(2,1,1)
// see https://eprint.iacr.org/2023/323.pdf page 15, case t=2,3
//
// when t=0[4], the buffer is multiplied by circ(2M4,M4,..,M4)
// see https://eprint.iacr.org/2023/323.pdf
func (h *permutation) matMulExternalInPlace(api frontend.API, input []frontend.Variable) {
	if h.params.Width%4 != 0 {
		panic("only Width = 0 mod 4 are supported")
	}
	// at this stage t is supposed to be a multiple of 4
	// the MDS matrix is circ(2M4,M4,..,M4)
	h.matMulM4InPlace(api, input)

	tmp := make([]frontend.Variable, 4)
	tmp[0] = 0
	tmp[1] = 0
	tmp[2] = 0
	tmp[3] = 0
	for i := 0; i < h.params.Width/4; i++ {
		tmp[0] = api.Add(tmp[0], input[4*i])
		tmp[1] = api.Add(tmp[1], input[4*i+1])
		tmp[2] = api.Add(tmp[2], input[4*i+2])
		tmp[3] = api.Add(tmp[3], input[4*i+3])
	}
	for i := 0; i < h.params.Width/4; i++ {
		input[4*i] = api.Add(input[4*i], tmp[0])
		input[4*i+1] = api.Add(input[4*i+1], tmp[1])
		input[4*i+2] = api.Add(input[4*i+2], tmp[2])
		input[4*i+3] = api.Add(input[4*i+3], tmp[3])
	}
}

// when t=2,3 the matrix are respectively [[2,1][1,3]] and [[2,1,1][1,2,1][1,1,3]]
// otherwise the matrix is filled with ones except on the diagonal,
func (h *permutation) matMulInternalInPlace(api frontend.API, input []frontend.Variable) {
	// width = 16
	sum := input[0]
	for i := 1; i < h.params.Width; i++ {
		sum = api.Add(sum, input[i])
	}
	// mul by diag16:
	// [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
	v := 2
	var temp frontend.Variable
	temp = api.Add(input[0], input[0])
	input[0] = api.Sub(sum, temp)
	input[1] = api.Add(sum, input[1])
	temp = api.Add(input[2], input[2])
	input[2] = api.Add(sum, temp)
	temp = api.Div(input[3], v)
	input[3] = api.Add(sum, temp)
	temp = api.Add(input[4], input[4])
	temp = api.Add(temp, input[4])
	input[4] = api.Add(sum, temp)
	temp = api.Add(input[5], input[5])
	temp = api.Add(temp, temp)
	input[5] = api.Add(sum, temp)
	temp = api.Div(input[6], v)
	input[6] = api.Sub(sum, temp)
	temp = api.Add(input[7], input[7])
	temp = api.Add(temp, input[7])
	input[7] = api.Sub(sum, temp)
	temp = api.Add(input[8], input[8])
	temp = api.Add(temp, temp)
	input[8] = api.Sub(sum, temp)
	v = 1 << 8
	temp = api.Div(input[9], v)
	input[9] = api.Add(sum, temp)
	v = 1 << 3
	temp = api.Div(input[10], v)
	input[10] = api.Add(sum, temp)
	v = 1 << 24
	temp = api.Div(input[11], v)
	input[11] = api.Add(sum, temp)
	v = 1 << 8
	temp = api.Div(input[12], v)
	input[12] = api.Sub(sum, temp)
	v = 1 << 3
	temp = api.Div(input[13], v)
	input[13] = api.Sub(sum, temp)
	v = 1 << 4
	temp = api.Div(input[14], v)
	input[14] = api.Sub(sum, temp)
	v = 1 << 24
	temp = api.Div(input[15], v)
	input[15] = api.Sub(sum, temp)
}

// addRoundKeyInPlace adds the round-th key to the buffer
func (h *permutation) addRoundKeyInPlace(api frontend.API, round int, input []frontend.Variable) {
	var rk frontend.Variable
	for i := 0; i < len(h.params.RoundKeys[round]); i++ {
		rk = h.params.RoundKeys[round][i].String()
		input[i] = api.Add(input[i], rk)
	}
}

// permutation applies the permutation on input, and stores the result in input.
func (h *permutation) Permutation(api frontend.API, input []frontend.Variable) error {
	if len(input) != h.params.Width {
		return ErrInvalidSizebuffer
	}

	// external matrix multiplication, cf https://eprint.iacr.org/2023/323.pdf page 14 (part 6)
	h.matMulExternalInPlace(api, input)

	rf := h.params.NbFullRounds / 2
	for i := 0; i < rf; i++ {
		// one round = matMulExternal(sBox_Full(addRoundKey))
		h.addRoundKeyInPlace(api, i, input)
		for j := 0; j < h.params.Width; j++ {
			h.sBox(api, j, input)
		}
		h.matMulExternalInPlace(api, input)
	}

	for i := rf; i < rf+h.params.NbPartialRounds; i++ {
		// one round = matMulInternal(sBox_sparse(addRoundKey))
		h.addRoundKeyInPlace(api, i, input)
		h.sBox(api, 0, input)
		h.matMulInternalInPlace(api, input)
	}

	for i := rf + h.params.NbPartialRounds; i < h.params.NbFullRounds+h.params.NbPartialRounds; i++ {
		// one round = matMulExternal(sBox_Full(addRoundKey))
		h.addRoundKeyInPlace(api, i, input)
		for j := 0; j < h.params.Width; j++ {
			h.sBox(api, j, input)
		}
		h.matMulExternalInPlace(api, input)
	}

	return nil
}
