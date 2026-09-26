package fiatshamir

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark/frontend"
)

// GnarkFiatShamir is the in-circuit counterpart of [FiatShamir]. It replays
// the exact same Poseidon2 sponge over circuit variables, so a verifier
// circuit re-derives every challenge from the transcript instead of trusting
// prover-supplied coins. Method by method it mirrors [FiatShamir]; the two
// must be kept in lock-step or the circuit derives different coins than the
// native verifier.
type GnarkFiatShamir struct {
	api *circuit.KoalaBearAPI
	h   *poseidon2.KoalagnarkMDHasher
}

// NewGnarkFiatShamir creates a transcript over the given frontend. It works in
// both native KoalaBear circuits and emulated ones.
func NewGnarkFiatShamir(api frontend.API) *GnarkFiatShamir {
	return &GnarkFiatShamir{
		api: circuit.NewAPI(api),
		h:   poseidon2.NewKoalagnarkMDHasher(api),
	}
}

// API returns the koalagnark API the transcript operates with.
func (fs *GnarkFiatShamir) API() *circuit.KoalaBearAPI { return fs.api }

// Update mirrors [FiatShamir.Update].
func (fs *GnarkFiatShamir) Update(vec ...circuit.Element) {
	fs.h.Write(vec...)
}

// UpdateExt mirrors [FiatShamir.UpdateExt]: each extension element is
// absorbed as its six coordinates in canonical order.
func (fs *GnarkFiatShamir) UpdateExt(vec ...circuit.Ext) {
	for i := range vec {
		b0a0, b0a1, b1a0, b1a1, b2a0, b2a1 := vec[i].Coordinates()
		fs.h.Write(b0a0, b0a1, b1a0, b1a1, b2a0, b2a1)
	}
}

// UpdateOctuplet absorbs the eight coordinates of an octuplet.
func (fs *GnarkFiatShamir) UpdateOctuplet(o poseidon2.KoalagnarkOctuplet) {
	fs.h.Write(o[:]...)
}

// RandomDigest mirrors [FiatShamir.RandomDigest].
func (fs *GnarkFiatShamir) RandomDigest() poseidon2.KoalagnarkOctuplet {
	res := fs.h.Sum()
	fs.safeguardUpdate()
	return res
}

// RandomFext mirrors [FiatShamir.RandomFext]: the first six coordinates of a
// digest form the extension element, the last two are discarded.
func (fs *GnarkFiatShamir) RandomFext() circuit.Ext {
	s := fs.RandomDigest()
	return circuit.Ext{
		B0: circuit.E2{A0: s[0], A1: s[1]},
		B1: circuit.E2{A0: s[2], A1: s[3]},
		B2: circuit.E2{A0: s[4], A1: s[5]},
	}
}

// RandomManyIntegers mirrors [FiatShamir.RandomManyIntegers]: it samples num
// integers in [0, upperBound) as native frontend variables. upperBound must be
// a power of two, so the reduction is the low log2(upperBound) bits of each
// digest coordinate's canonical value.
func (fs *GnarkFiatShamir) RandomManyIntegers(num, upperBound int) []frontend.Variable {
	if !utils.IsPowerOfTwo(upperBound) {
		panic("upperBound must be a power of two")
	}
	logBound := bits.TrailingZeros(uint(upperBound))
	native := fs.api.Frontend()

	res := make([]frontend.Variable, 0, num)
	for len(res) < num {
		c := fs.RandomDigest()
		for j := 0; j < 8 && len(res) < num; j++ {
			b := fs.api.ToBinaryCanonical(c[j])
			if logBound == 0 {
				res = append(res, 0)
				continue
			}
			res = append(res, native.FromBinary(b[:logBound]...))
		}
	}
	return res
}

// SetState mirrors [FiatShamir.SetState].
func (fs *GnarkFiatShamir) SetState(s poseidon2.KoalagnarkOctuplet) {
	fs.h.SetState(s)
}

// State mirrors [FiatShamir.State].
func (fs *GnarkFiatShamir) State() poseidon2.KoalagnarkOctuplet {
	return fs.h.State()
}

func (fs *GnarkFiatShamir) safeguardUpdate() {
	fs.Update(fs.api.Zero())
}
