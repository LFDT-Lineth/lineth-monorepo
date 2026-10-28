package genericbench

// Frozen copy of the non-generic KoalaBear MD hasher and Fiat-Shamir
// transcript, as of commit 3c01857c7 (before crypto/poseidon2 and
// crypto/fiatshamir were made generic). Only the methods exercised by the
// benchmarks are kept; their bodies are unchanged, including the embedded
// gnark-crypto StateStorer that NewMDHasher used to allocate. Do not
// "improve" this file: it is the baseline.

import (
	"slices"
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/gnark-crypto/hash"
	"golang.org/x/crypto/blake2b"

	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/types"
)

const legacyBlockSize = 8

type legacyMDHasher struct {
	hash.StateStorer

	// Sponge construction state
	state field.Octuplet

	// data to hash
	buffer         []field.Element
	bufferPosition int
}

func newLegacyMDHasher() *legacyMDHasher {
	return &legacyMDHasher{
		StateStorer: gnarkposeidon2.NewMerkleDamgardHasher(),
		state:       field.Octuplet{},
	}
}

func (d *legacyMDHasher) Reset() {
	d.buffer = d.buffer[:0]
	d.bufferPosition = 0
	d.state = field.Octuplet{}
}

func (d *legacyMDHasher) SetStateOctuplet(state field.Octuplet) {
	d.Reset()
	d.buffer = d.buffer[:0]
	d.bufferPosition = 0
	for i := 0; i < 8; i++ {
		d.state[i].Set(&state[i])
	}
}

func (d *legacyMDHasher) GetStateOctuplet() field.Octuplet {
	oldState := d.state
	oldBuffer := slices.Clone(d.buffer)
	oldBufferPosition := d.bufferPosition

	_ = d.SumDigest() // this flushes the hasher
	res := slices.Clone(d.state[:])

	d.state = oldState
	d.buffer = oldBuffer
	d.bufferPosition = oldBufferPosition

	return field.Octuplet(res)
}

func (d *legacyMDHasher) WriteElements(elmts ...field.Element) {
	d.buffer = append(d.buffer, elmts[:]...)
}

func (d *legacyMDHasher) SumDigest() field.Octuplet {
	for len(d.buffer) != 0 {
		var buf [legacyBlockSize]field.Element

		// in this case we left pad by zeroes
		if len(d.buffer) < legacyBlockSize {
			copy(buf[legacyBlockSize-len(d.buffer):], d.buffer)
			d.buffer = d.buffer[:0]
		} else {
			copy(buf[:], d.buffer)
			d.buffer = d.buffer[legacyBlockSize:]
		}

		d.state = vortex.CompressPoseidon2(d.state, buf)
	}

	return d.state
}

type legacyFiatShamir struct {
	h *legacyMDHasher
}

func newLegacyFiatShamir() *legacyFiatShamir {
	return &legacyFiatShamir{h: newLegacyMDHasher()}
}

func (fs *legacyFiatShamir) Update(vec ...field.Element) {
	fs.h.WriteElements(vec...)
}

func (fs *legacyFiatShamir) UpdateExt(vec ...field.Ext) {
	if len(vec) == 0 {
		return
	}
	vElems := unsafe.Slice((*field.Element)(unsafe.Pointer(&vec[0])), field.ExtensionDegree*len(vec)) //nolint
	fs.h.WriteElements(vElems...)
}

func (fs *legacyFiatShamir) RandomDigest() field.Octuplet {
	defer fs.safeguardUpdate()
	return fs.h.SumDigest()
}

func (fs *legacyFiatShamir) RandomFext() field.Ext {
	s := fs.RandomDigest() // already calls safeguardUpdate()
	var res field.Ext
	res.B0.A0 = s[0]
	res.B0.A1 = s[1]
	res.B1.A0 = s[2]
	res.B1.A1 = s[3]
	res.B2.A0 = s[4]
	res.B2.A1 = s[5]

	return res
}

func (fs *legacyFiatShamir) RandomFextFromSeed(seed field.Octuplet, name string) field.Ext {
	nameBytes := []byte(name)
	hasher, _ := blake2b.New256(nil)
	hasher.Write(nameBytes)
	nameBytes = hasher.Sum(nil)
	nameOctuplet := types.BytesToKoalaOctupletLoose(nameBytes)

	oldState := fs.State()
	defer fs.SetState(oldState)

	fs.SetState(seed)
	fs.h.WriteElements(nameOctuplet[:]...)
	res := fs.RandomFext()
	return res
}

func (fs *legacyFiatShamir) RandomManyIntegers(num, upperBound int) []int {
	if !utils.IsPowerOfTwo(upperBound) {
		panic("upperBound must be a power of two")
	}

	var (
		i   = 0
		res = make([]int, num)
	)

	for i < num {
		// take the remainder mod n of each limb
		c := fs.RandomDigest() // already calls safeguardUpdate()
		for j := 0; j < 8; j++ {
			b := c[j].Bits()
			res[i] = int(b[0]) % upperBound
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

func (fs *legacyFiatShamir) SetState(s field.Octuplet) {
	fs.h.SetStateOctuplet(s)
}

func (fs *legacyFiatShamir) State() field.Octuplet {
	return fs.h.GetStateOctuplet()
}

func (fs *legacyFiatShamir) safeguardUpdate() {
	fs.Update(field.Zero())
}
