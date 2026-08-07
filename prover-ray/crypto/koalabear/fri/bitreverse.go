package fri

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gutils "github.com/consensys/gnark-crypto/utils"
)

// bitRevCobraMinLog2 is the naive/COBRA crossover measured on a 96-core
// EPYC 9R45 with all cores bit-reversing concurrently (the Encode regime):
// naive wins through 2^20, COBRA wins 1.4x at 2^21 and 2.1x at 2^22.
const bitRevCobraMinLog2 = 21

func bitReverse(v []field.Element) {
	if len(v) < 1<<bitRevCobraMinLog2 {
		gutils.BitReverseNaive(v)
		return
	}
	gutils.BitReverseCobra(v)
}

func bitReverseCopy[T any](dst, src []T) {
	if len(src) < 1<<bitRevCobraMinLog2 {
		gutils.BitReverseCopyNaive(dst, src)
		return
	}
	gutils.BitReverseCopyCobra(dst, src)
}
