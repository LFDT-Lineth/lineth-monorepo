package fiatshamir_bls12377

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_bls12377"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/

type FS struct {
	koalaBuf []field.Element
	h        *poseidon2_bls12377.MDHasher
}

func NewFS(verboseMode ...bool) *FS {
	return &FS{h: poseidon2_bls12377.NewMDHasher(verboseMode...)}
}

// --------------------------------------------------
// List of methods to updae the FS state with frElmts
func (fs *FS) UpdateFrElmt(vec ...fr.Element) {
	fs.flushKoala()
	fs.h.WriteElements(vec...)
}

func (fs *FS) UpdateVecFrElmt(vecs ...[]fr.Element) {
	for i := range vecs {
		fs.UpdateFrElmt(vecs[i]...)
	}
}

func (fs *FS) RandomFieldFrElmt() fr.Element {
	fs.flushKoala()
	defer fs.safeguardUpdate()
	return fs.h.SumElement()
}

// ------------------------------------------------------
// List of methods to updae the FS state with koala elmts

func (fs *FS) Update(vec ...field.Element) {
	fs.koalaBuf = append(fs.koalaBuf, vec...)
}

func (fs *FS) UpdateExt(vec ...fext.Element) {

	for i := 0; i < len(vec); i++ {
		fs.koalaBuf = append(fs.koalaBuf, vec[i].B0.A0)
		fs.koalaBuf = append(fs.koalaBuf, vec[i].B0.A1)
		fs.koalaBuf = append(fs.koalaBuf, vec[i].B1.A0)
		fs.koalaBuf = append(fs.koalaBuf, vec[i].B1.A1)
	}
}

// UpdateSV updates the FS state with a smart-vector. No-op if the smart-vector
// has a length of zero.
func (fs *FS) UpdateSV(sv smartvectors.SmartVector) {
	if sv.Len() == 0 {
		return
	}
	if smartvectors.IsBase(sv) {
		vec := make([]field.Element, sv.Len())
		sv.WriteInSlice(vec)
		fs.Update(vec...)
	} else {
		vec := make([]fext.Element, sv.Len())
		sv.WriteInSliceExt(vec)
		fs.UpdateExt(vec...)
	}
}

func (fs *FS) RandomField() field.Octuplet {
	r := fs.RandomFieldFrElmt() // the safeguard update is called
	res := encoding.EncodeFrElementToOctuplet(r)
	return res
}

func (fs *FS) RandomManyIntegers(num, upperBound int) []int {
	n := utils.NextPowerOfTwo(upperBound)
	mask := n - 1
	i := 0
	res := make([]int, num)
	for i < num {
		// thake the remainder mod n of each limb
		c := fs.RandomField() // already calls safeguardUpdate()
		for j := 0; j < 8; j++ {
			b := c[j].Bits()
			res[i] = int(b[0]) & mask
			i++
			if i >= num {
				break
			}
		}
	}
	return res
}

func (fs *FS) RandomFext() fext.Element {
	s := fs.RandomField() // calls safeguardUpdate
	var res fext.Element
	res.B0.A0 = s[0]
	res.B0.A1 = s[1]
	res.B1.A0 = s[2]
	res.B1.A1 = s[3]

	return res
}
func (fs *FS) RandomFieldFromSeed(seed field.Octuplet, name string) fext.Element {
	panic("not implemented")
}

func (fs *FS) SetState(s field.Octuplet) {
	fs.koalaBuf = fs.koalaBuf[:0]
	state := encoding.EncodeKoalabearOctupletToFrElement(s)
	fs.h.SetStateFrElement(state)
}

func (fs *FS) State() field.Octuplet {
	state := fs.h.State()
	return encoding.EncodeFrElementToOctuplet(state)
}

func (fs *FS) safeguardUpdate() {
	fs.UpdateFrElmt(fr.Element{})
}

// flushKoala writes all the content of the koala buffer to the hasher
func (fs *FS) flushKoala() {
	if len(fs.koalaBuf) == 0 {
		return
	}
	fs.h.WriteKoalabearElements(fs.koalaBuf...)
	fs.koalaBuf = fs.koalaBuf[:0]
}
