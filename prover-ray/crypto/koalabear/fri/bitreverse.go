package fri

import (
	gutils "github.com/consensys/gnark-crypto/utils"
)

func bitReverse[T any](v []T) {
	gutils.BitReverse(v)
}

func bitReverseCopy[T any](dst, src []T) {
	gutils.BitReverseCopy(dst, src)
}
