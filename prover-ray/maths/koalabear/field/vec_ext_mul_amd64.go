//go:build !purego

package field

import "github.com/consensys/gnark-crypto/utils/cpu"

// Constants of the AVX-512 kernel (see vec_ext_mul_amd64.s): the KoalaBear
// modulus and -q⁻¹ mod 2³².
//
//nolint:unused // referenced from the assembly kernel
const (
	extMulQ       = 2130706433
	extMulQInvNeg = 2130706431
)

// extMulGatherIndex holds the offsets, in 32-bit words, of the first limb of
// 16 consecutive extension elements.
//
//nolint:unused // referenced from the assembly kernel
var extMulGatherIndex = func() []uint32 {
	idx := make([]uint32, 16)
	for i := range idx {
		idx[i] = uint32(6 * i)
	}
	return idx
}()

//go:noescape
func vecMulExtAVX512(res, a, b *Ext, n uint64)

// vecMulExt sets res[i] = a[i]·b[i]. With AVX-512 it multiplies 16 elements
// at a time, each as a tower Karatsuba product of 18 base-field products;
// the tail goes through [Ext.Mul]. Field elements are canonical, so the
// results are those of [Ext.Mul]. res may alias a or b.
func vecMulExt(res, a, b []Ext) {
	if n := len(res) &^ 15; cpu.SupportAVX512 && n > 0 {
		vecMulExtAVX512(&res[0], &a[0], &b[0], uint64(n))
		res, a, b = res[n:], a[n:], b[n:]
	}
	for i := range res {
		res[i].Mul(&a[i], &b[i])
	}
}
