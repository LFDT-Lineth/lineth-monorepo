package poseidon2_bls12377

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	_ "github.com/consensys/gnark/std/hash/poseidon2"
	gkr_poseidon2 "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// GnarkMDHasher Merkle Damgard implementation using poseidon2 as compression function with width 16
// The hashing process goes as follow:
// newState := compress(old state, buf) where buf is an octuplet, left padded with zeroes if needed
type GnarkMDHasher struct {
	api frontend.API

	// Sponge construction state
	state frontend.Variable

	// data to hash
	buffer []frontend.Variable

	compressor hash.Compressor

	verbose bool
}

// NewGnarkMDHasher returns a new Octuplet
func NewGnarkMDHasher(api frontend.API, verbose ...bool) (GnarkMDHasher, error) {
	var res GnarkMDHasher
	res.state = 0
	res.api = api
	var err error

	if len(verbose) > 0 {
		res.verbose = verbose[0]
	}

	// default parameters
	// note; when we do more than 16k hashes, we should use the gkr poseidon compressor
	// less than that the plain poseidon2 compressor is faster.
	// plain --> poseidon2.NewPoseidon2FromParameters(api, 2, 6, 26)
	res.compressor, err = gkr_poseidon2.NewCompressor(api)
	if err != nil {
		return res, err
	}
	return res, nil
}

func (h *GnarkMDHasher) Reset() {
	h.buffer = h.buffer[:0]
	h.state = 0
}

func (h *GnarkMDHasher) Write(data ...frontend.Variable) {
	h.buffer = append(h.buffer, data...)
}

func (h *GnarkMDHasher) WriteWVs(data ...koalagnark.Element) {
	_data := encoding.EncodeWVsToFVs(h.api, data)
	h.buffer = append(h.buffer, _data...)
}

func (h *GnarkMDHasher) SetState(state frontend.Variable) {
	h.buffer = h.buffer[:0]
	h.state = state
}

func (h *GnarkMDHasher) State() frontend.Variable {
	// If the buffer is clean, we can short-path the execution and directly
	if len(h.buffer) == 0 {
		return h.state
	}

	// If the buffer is not clean, we cannot clean it locally as it would modify
	// the state of the hasher locally. Instead, we clone the buffer and flush
	// the buffer on the clone.
	clone, _ := NewGnarkMDHasher(h.api)
	clone.buffer = make([]frontend.Variable, len(h.buffer))
	copy(clone.buffer, h.buffer)
	clone.state = h.state
	_ = clone.Sum()
	return clone.state
}

func (h *GnarkMDHasher) Sum() frontend.Variable {

	if h.verbose {
		h.api.Println("[gnark fs flush] oldState", h.state, "buf")
		h.api.Println(h.buffer...)
	}

	for i := 0; i < len(h.buffer); i++ {
		h.state = h.compressor.Compress(h.state, h.buffer[i])
	}
	h.buffer = h.buffer[:0]
	return h.state
}
