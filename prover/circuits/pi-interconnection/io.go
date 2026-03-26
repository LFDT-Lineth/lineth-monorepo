package pi_interconnection

import (
	"math/big"
	"slices"

	fr381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
)

// the resulting bytes will have been range-checked
func fr377EncodedFr381ToBytes(api frontend.API, x [2]frontend.Variable) [32]frontend.Variable {
	const (
		loNbBits   = 128
		hiBits381  = fr381.Bits - 128
		hiNbCrumbs = (hiBits381 + 1) / 2
		loNbCrumbs = loNbBits / 2
	)
	hi := api.ToBinary(x[0], hiBits381)
	lo := internal.ToCrumbs(api, x[1], loNbCrumbs)
	slices.Reverse(lo)

	cr := make([]frontend.Variable, hiNbCrumbs+len(lo))
	for i := 0; i < hiNbCrumbs; i++ {
		b := hi[2*i : min(2*i+2, len(hi))]
		cr[(hiNbCrumbs-1)-i] = api.FromBinary(b...)
	}
	copy(cr[hiNbCrumbs:], lo)

	if len(cr) != 128 {
		panic("unexpected length")
	}

	var res [32]frontend.Variable
	radix := big.NewInt(4)
	for i := range res {
		res[i] = compress.ReadNum(api, cr[i*4:i*4+4], radix)
	}
	return res
}
