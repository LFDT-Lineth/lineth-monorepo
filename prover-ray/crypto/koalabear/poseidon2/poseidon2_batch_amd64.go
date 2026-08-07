//go:build !purego

package poseidon2

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gcpu "github.com/consensys/gnark-crypto/utils/cpu"
)

// q and qInvNeg are referenced by the assembly kernel through go_asm.h.
const (
	q       = 2130706433 //nolint:unused // read by poseidon2_batch_amd64.s via go_asm.h
	qInvNeg = 2130706431 //nolint:unused // read by poseidon2_batch_amd64.s via go_asm.h
)

// indexScatter8 is read by the kernel epilogue to scatter the 16 final states
// into the row-major [16][8] output.
//
//nolint:unused // read by poseidon2_batch_amd64.s as a package symbol
var indexScatter8 = func() []uint32 {
	idx := make([]uint32, 16)
	for i := range idx {
		idx[i] = uint32(i * 8)
	}
	return idx
}()

//go:noescape
func permutation16x16xNStateAVX512(
	matrix *field.Element, roundKeys [][]field.Element, result *field.Element, nbSteps uint64, state *field.Element,
)

func compressChain16(state, matrix []field.Element, out []field.Octuplet) {
	if !gcpu.SupportAVX512 {
		compressChain16Generic(state, matrix, out)
		return
	}
	nbSteps := uint64(len(matrix) / (8 * BatchLanes))
	permutation16x16xNStateAVX512(&matrix[0], batchRoundKeys, &out[0][0], nbSteps, &state[0])
}
