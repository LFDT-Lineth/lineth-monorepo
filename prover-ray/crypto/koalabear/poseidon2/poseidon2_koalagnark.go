package poseidon2

import (
	"math/big"
	"slices"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/circuit"
)

// KoalagnarkOctuplet is an octuplet of circuit.Element
type KoalagnarkOctuplet [8]circuit.Element

// KoalagnarkMDHasher is a Merkle-Damgard hasher using Poseidon2 as compression function.
// This implementation uses circuit.Element and circuit.API, allowing it to work
// in both native KoalaBear circuits and emulated circuits (e.g., BLS12-377).
type KoalagnarkMDHasher struct {
	api      frontend.API
	koalaAPI *circuit.API

	// Sponge construction state
	state KoalagnarkOctuplet

	// data to hash
	buffer []circuit.Element
}

// NewKoalagnarkMDHasher creates a new Merkle-Damgard hasher using koalagnark types.
func NewKoalagnarkMDHasher(api frontend.API) *KoalagnarkMDHasher {
	koalaAPI := circuit.NewAPI(api)
	var res KoalagnarkMDHasher
	for i := 0; i < 8; i++ {
		res.state[i] = koalaAPI.Zero()
	}
	res.api = api
	res.koalaAPI = koalaAPI
	return &res
}

// Reset clears the buffer and resets the state to zero.
func (h *KoalagnarkMDHasher) Reset() {
	h.buffer = h.buffer[:0]
	for i := 0; i < 8; i++ {
		h.state[i] = h.koalaAPI.Zero()
	}
}

func (h *KoalagnarkMDHasher) Write(data ...circuit.Element) {
	h.buffer = append(h.buffer, data...)
}

// WriteOctuplet appends octuplets to the hash buffer.
func (h *KoalagnarkMDHasher) WriteOctuplet(data ...KoalagnarkOctuplet) {
	for i := 0; i < len(data); i++ {
		h.buffer = append(h.buffer, data[i][:]...)
	}
}

// SetState resets the hasher and sets its state to the given octuplet.
func (h *KoalagnarkMDHasher) SetState(state KoalagnarkOctuplet) {
	h.Reset()
	copy(h.state[:], state[:])
}

// State returns the current hash state without consuming the buffer.
func (h *KoalagnarkMDHasher) State() KoalagnarkOctuplet {
	// State will flush the buffer, take the state and restore the initial
	// state of the hasher.
	oldState := h.state
	oldBuffer := slices.Clone(h.buffer)

	_ = h.Sum() // this flushes the hasher
	res := h.state

	h.state = oldState
	h.buffer = oldBuffer

	return res
}

// Sum finalizes the hash and returns the digest as a KoalagnarkOctuplet.
func (h *KoalagnarkMDHasher) Sum() KoalagnarkOctuplet {
	for len(h.buffer) != 0 {
		var buf [BlockSize]circuit.Element
		for i := 0; i < BlockSize; i++ {
			buf[i] = h.koalaAPI.Zero()
		}
		// in this case we left pad by zeroes
		if len(h.buffer) < BlockSize {
			copy(buf[BlockSize-len(h.buffer):], h.buffer)
			h.buffer = h.buffer[:0]
		} else {
			copy(buf[:], h.buffer)
			h.buffer = h.buffer[BlockSize:]
		}

		h.state = h.compressPoseidon2(h.state, buf)
	}
	return h.state
}

func (h *KoalagnarkMDHasher) compressPoseidon2(a, b KoalagnarkOctuplet) KoalagnarkOctuplet {
	res := KoalagnarkOctuplet{}

	var x [16]circuit.Element
	copy(x[:], a[:])
	copy(x[8:], b[:])

	// Create a buffer to hold the feed-forward input.
	copy(res[:], x[8:])

	err := koalagnarkCompressPerm.Permutation(h.koalaAPI, x[:])
	if err != nil {
		panic(err)
	}

	for i := range res {
		res[i] = h.koalaAPI.Add(res[i], x[8+i])
	}
	return res
}

var (
	koalagnarkCompressPerm koalagnarkPermutation
	koalagnarkOnce         sync.Once
)

func init() {
	koalagnarkOnce.Do(func() {
		koalagnarkCompressPerm = NewKoalagnarkPermutation()
	})
}

// koalagnarkPermutation is a Poseidon2 permutation using koalagnark types.
type koalagnarkPermutation struct {
	params *poseidon2.Parameters
}

// NewKoalagnarkPermutation creates a permutation with KoalaBear parameters.
// It mirrors NewPermutation in poseidon2_circuit.go but uses koalagnark arithmetic.
// nolint -- we are fine with returning an unexported type as we want
// this to be a protected constructor.
func NewKoalagnarkPermutation() koalagnarkPermutation {
	params := poseidon2.NewParameters(16, 6, 21)
	return koalagnarkPermutation{params: params}
}

// sBox applies the sBox on buffer[index]
func (p *koalagnarkPermutation) sBox(api *circuit.API, index int, input []circuit.Element) {
	// sbox degree is 3: x^3
	tmp := input[index]
	input[index] = api.Mul(input[index], input[index])
	input[index] = api.Mul(tmp, input[index])
}

// matMulM4InPlace computes s <- M4*s
// where M4=
// (2 3 1 1)
// (1 2 3 1)
// (1 1 2 3)
// (3 1 1 2)
// on chunks of 4 elements on each part of the buffer
func (p *koalagnarkPermutation) matMulM4InPlace(api *circuit.API, s []circuit.Element) {
	c := len(s) / 4
	for i := 0; i < c; i++ {
		var t01, t23, t0123, t01123, t01233 circuit.Element
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

// matMulExternalInPlace multiplies by circ(2M4,M4,..,M4)
func (p *koalagnarkPermutation) matMulExternalInPlace(api *circuit.API, input []circuit.Element) {
	if p.params.Width%4 != 0 {
		panic("only Width = 0 mod 4 are supported")
	}
	p.matMulM4InPlace(api, input)

	tmp := make([]circuit.Element, 4)
	for i := 0; i < 4; i++ {
		tmp[i] = api.Zero()
	}
	for i := 0; i < p.params.Width/4; i++ {
		tmp[0] = api.Add(tmp[0], input[4*i])
		tmp[1] = api.Add(tmp[1], input[4*i+1])
		tmp[2] = api.Add(tmp[2], input[4*i+2])
		tmp[3] = api.Add(tmp[3], input[4*i+3])
	}
	for i := 0; i < p.params.Width/4; i++ {
		input[4*i] = api.Add(input[4*i], tmp[0])
		input[4*i+1] = api.Add(input[4*i+1], tmp[1])
		input[4*i+2] = api.Add(input[4*i+2], tmp[2])
		input[4*i+3] = api.Add(input[4*i+3], tmp[3])
	}
}

// matMulInternalInPlace applies the internal matrix multiplication
// diag16: [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
func (p *koalagnarkPermutation) matMulInternalInPlace(api *circuit.API, input []circuit.Element) {
	// width = 16
	sum := input[0]
	for i := 1; i < p.params.Width; i++ {
		sum = api.Add(sum, input[i])
	}

	var temp circuit.Element

	// input[0]: sum + (-2)*input[0] = sum - 2*input[0]
	temp = api.Add(input[0], input[0])
	input[0] = api.Sub(sum, temp)

	// input[1]: sum + 1*input[1]
	input[1] = api.Add(sum, input[1])

	// input[2]: sum + 2*input[2]
	temp = api.Add(input[2], input[2])
	input[2] = api.Add(sum, temp)

	// input[3]: sum + (1/2)*input[3]
	temp = api.Div(input[3], api.Const(2))
	input[3] = api.Add(sum, temp)

	// input[4]: sum + 3*input[4]
	temp = api.Add(input[4], input[4])
	temp = api.Add(temp, input[4])
	input[4] = api.Add(sum, temp)

	// input[5]: sum + 4*input[5]
	temp = api.Add(input[5], input[5])
	temp = api.Add(temp, temp)
	input[5] = api.Add(sum, temp)

	// input[6]: sum + (-1/2)*input[6] = sum - (1/2)*input[6]
	temp = api.Div(input[6], api.Const(2))
	input[6] = api.Sub(sum, temp)

	// input[7]: sum + (-3)*input[7] = sum - 3*input[7]
	temp = api.Add(input[7], input[7])
	temp = api.Add(temp, input[7])
	input[7] = api.Sub(sum, temp)

	// input[8]: sum + (-4)*input[8] = sum - 4*input[8]
	temp = api.Add(input[8], input[8])
	temp = api.Add(temp, temp)
	input[8] = api.Sub(sum, temp)

	// input[9]: sum + (1/2^8)*input[9]
	temp = api.Div(input[9], api.Const(1<<8))
	input[9] = api.Add(sum, temp)

	// input[10]: sum + (1/8)*input[10]
	temp = api.Div(input[10], api.Const(1<<3))
	input[10] = api.Add(sum, temp)

	// input[11]: sum + (1/2^24)*input[11]
	temp = api.Div(input[11], api.Const(1<<24))
	input[11] = api.Add(sum, temp)

	// input[12]: sum + (-1/2^8)*input[12] = sum - (1/2^8)*input[12]
	temp = api.Div(input[12], api.Const(1<<8))
	input[12] = api.Sub(sum, temp)

	// input[13]: sum + (-1/8)*input[13] = sum - (1/8)*input[13]
	temp = api.Div(input[13], api.Const(1<<3))
	input[13] = api.Sub(sum, temp)

	// input[14]: sum + (-1/16)*input[14] = sum - (1/16)*input[14]
	temp = api.Div(input[14], api.Const(1<<4))
	input[14] = api.Sub(sum, temp)

	// input[15]: sum + (-1/2^24)*input[15] = sum - (1/2^24)*input[15]
	temp = api.Div(input[15], api.Const(1<<24))
	input[15] = api.Sub(sum, temp)
}

// addRoundKeyInPlace adds the round-th key to the buffer
func (p *koalagnarkPermutation) addRoundKeyInPlace(api *circuit.API, round int, input []circuit.Element) {
	for i := 0; i < len(p.params.RoundKeys[round]); i++ {
		rk := api.ConstBig(p.params.RoundKeys[round][i].BigInt(new(big.Int)))
		input[i] = api.Add(input[i], rk)
	}
}

// Permutation applies the Poseidon2 permutation on input
func (p *koalagnarkPermutation) Permutation(api *circuit.API, input []circuit.Element) error {
	if len(input) != p.params.Width {
		return ErrInvalidSizebuffer
	}

	// external matrix multiplication
	p.matMulExternalInPlace(api, input)

	rf := p.params.NbFullRounds / 2
	for i := 0; i < rf; i++ {
		// one round = matMulExternal(sBox_Full(addRoundKey))
		p.addRoundKeyInPlace(api, i, input)
		for j := 0; j < p.params.Width; j++ {
			p.sBox(api, j, input)
		}
		p.matMulExternalInPlace(api, input)
	}

	for i := rf; i < rf+p.params.NbPartialRounds; i++ {
		// one round = matMulInternal(sBox_sparse(addRoundKey))
		p.addRoundKeyInPlace(api, i, input)
		p.sBox(api, 0, input)
		p.matMulInternalInPlace(api, input)
	}

	for i := rf + p.params.NbPartialRounds; i < p.params.NbFullRounds+p.params.NbPartialRounds; i++ {
		// one round = matMulExternal(sBox_Full(addRoundKey))
		p.addRoundKeyInPlace(api, i, input)
		for j := 0; j < p.params.Width; j++ {
			p.sBox(api, j, input)
		}
		p.matMulExternalInPlace(api, input)
	}

	return nil
}
