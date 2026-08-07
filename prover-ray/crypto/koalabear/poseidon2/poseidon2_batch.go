package poseidon2

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
)

// BatchLanes is the number of independent compression chains processed by one
// CompressChain16 call.
const BatchLanes = 16

// batchRoundKeys are the round keys of the width-16 Poseidon2 permutation, the
// same deterministic parameters vortex.CompressPoseidon2 uses, so batch and
// scalar compressions are bit-identical.
var batchRoundKeys = gnarkposeidon2.NewParameters(16, 6, 21).RoundKeys

// CompressChain16 runs 16 independent Poseidon2 compression chains:
//
//	out[lane] = C(...C(C(state_lane, block_lane_0), block_lane_1)..., block_lane_{n-1})
//
// where C is the same compression as [Compress]. Unlike
// vortex.CompressPoseidon2x16Columns, the chains start from the given states,
// not from zero, so a single-block call is a direct batched Compress.
//
// state holds the 16 initial states column-major (len 128) and matrix holds n
// blocks of 8 elements per lane in the same layout (len n*128); use
// [StageOctuplet] to fill them. out receives the 16 final states.
func CompressChain16(state, matrix []field.Element, out []field.Octuplet) {
	if len(state) != 8*BatchLanes || len(matrix) == 0 || len(matrix)%(8*BatchLanes) != 0 || len(out) != BatchLanes {
		panic("poseidon2: CompressChain16: invalid input geometry")
	}
	compressChain16(state, matrix, out)
}

// StageOctuplet writes o into lane's column of the column-major layout
// [CompressChain16] expects: dst[pos*BatchLanes+lane] = o[pos]. dst is one
// 8-element block region (a state buffer, or matrix[step*8*BatchLanes:]).
func StageOctuplet(dst []field.Element, lane int, o *field.Octuplet) {
	for pos := range 8 {
		dst[pos*BatchLanes+lane] = o[pos]
	}
}

// compressChain16Generic is the portable reference implementation; the AVX-512
// path must stay bit-identical to it.
func compressChain16Generic(state, matrix []field.Element, out []field.Octuplet) {
	nbSteps := len(matrix) / (8 * BatchLanes)
	for lane := range BatchLanes {
		var s field.Octuplet
		for pos := range 8 {
			s[pos] = state[pos*BatchLanes+lane]
		}
		for step := range nbSteps {
			var block field.Octuplet
			for pos := range 8 {
				block[pos] = matrix[(step*8+pos)*BatchLanes+lane]
			}
			s = vortex.CompressPoseidon2(s, block)
		}
		out[lane] = s
	}
}
