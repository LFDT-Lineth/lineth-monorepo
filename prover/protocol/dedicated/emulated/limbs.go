package emulated

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// csPolyEval constructs a symbolic expression that evaluates the polynomial
// defined by the limbs and the challenge powers.
func csPolyEval(val limbs.Limbs[limbs.LittleEndian], chinfo *coin.Info) *symbolic.Expression {
	coefs := val.GetLimbs()
	coefsF := make([]*symbolic.Expression, len(coefs))
	for i, l := range coefs {
		coefsF[i] = ifaces.ColumnAsVariable(l)
	}
	ch := chinfo.AsVariable()
	res := symbolic.NewPolyEval(ch, coefsF)
	return res
}

// limbsToBigInt recomposes the limbs at loc into res using buf as a temporary
// buffer.
func limbsToBigInt(res *big.Int, buf []uint64, rows limbs.VecRow[limbs.LittleEndian], loc int, nbBitsPerLimb int) error {
	if res == nil {
		return fmt.Errorf("result not initialized")
	}
	nbLimbs := rows[0].NumLimbs()
	if len(buf) < nbLimbs {
		buf = append(buf, make([]uint64, nbLimbs-len(buf))...)
	}
	for j := range nbLimbs {
		buf[j] = rows[loc].T[j].Uint64()
	}
	if err := IntLimbRecompose(buf, nbBitsPerLimb, res); err != nil {
		return err
	}
	return nil
}

// bigIntToLimbs decomposes input into limbs and sets the outputCols at loc
// accordingly.
func bigIntToLimbs(input *big.Int, buf []uint64, ls limbs.Limbs[limbs.LittleEndian], outputCols [][]field.Element, loc int, nbBitsPerLimb int) error {
	if len(buf) != ls.NumLimbs() {
		return fmt.Errorf("mismatched size between limbs and buffer")
	}
	if err := IntLimbDecompose(input, nbBitsPerLimb, buf); err != nil {
		return fmt.Errorf("failed to decompose big.Int into limbs: %v", err)
	}
	for i := range ls.NumLimbs() {
		outputCols[i][loc].SetUint64(buf[i])
	}
	return nil
}

// IntLimbDecompose decomposes the input into buffer as integers of width
// nbBits. The decomposition is stored in res in little-endian order. It errors
// if the decomposition does not fit into res or if res is uninitialized.
//
// The following holds
//
//	input = \sum_{i=0}^{len(res)} res[i] * 2^{nbBits * i}
func IntLimbDecompose(input *big.Int, nbBits int, res []uint64) error {
	// limb modulus
	if input.BitLen() > len(res)*int(nbBits) {
		return errors.New("decomposed integer does not fit into res")
	}
	base := new(big.Int).Lsh(big.NewInt(1), uint(nbBits))
	base.Sub(base, big.NewInt(1))
	tmp := new(big.Int).Set(input)
	tmp2 := new(big.Int)
	for i := range res {
		tmp2.And(tmp, base)
		res[i] = tmp2.Uint64()
		tmp.Rsh(tmp, uint(nbBits))
	}
	return nil
}

// Recompose takes the little-endian limbs in inputs and combines them into res.
// It errors if inputs is uninitialized or zero-length and if the result is
// uninitialized.
//
// The following holds
//
//	res = \sum_{i=0}^{len(inputs)} inputs[i] * 2^{nbBits * i}
func IntLimbRecompose(inputs []uint64, nbBits int, res *big.Int) error {
	if res == nil {
		return errors.New("result not initialized")
	}
	res.SetUint64(0)
	tmp := new(big.Int)
	for i := range inputs {
		res.Lsh(res, uint(nbBits))
		tmp.SetUint64(inputs[len(inputs)-i-1])
		res.Add(res, tmp)
	}
	// we do not mod-reduce here as the result is mod-reduced by the caller if
	// needed. In some places we need non-reduced results.
	return nil
}

// limbMul performs limb multiplication between lhs and rhs storing the result
// in res.
func limbMul(res, lhs, rhs []uint64) error {
	if len(res) != nbMultiplicationResLimbs(len(lhs), len(rhs)) {
		return errors.New("result slice length mismatch")
	}
	clearBuffer(res)
	for i := range lhs {
		for j := range rhs {
			res[i+j] += lhs[i] * rhs[j]
		}
	}
	return nil
}

// nbMultiplicationResLimbs returns the number of limbs which fit the
// multiplication result.
func nbMultiplicationResLimbs(lenLeft, lenRight int) int {
	res := max(lenLeft+lenRight-1, 0)
	return res
}

// clearBuffer buffers the big.Int slice by setting all elements to zero.
func clearBuffer[E interface{ uint64 | *big.Int }, S []E](buf S) {
	switch any(buf).(type) {
	case []*big.Int:
		clearBufferBigInt(any(buf).([]*big.Int))
	case []uint64:
		clearBufferUint64(any(buf).([]uint64))
	default:
		panic("unsupported type")
	}
}

func clearBufferBigInt(buf []*big.Int) {
	for i := range buf {
		buf[i].SetInt64(0)
	}
}

func clearBufferUint64(buf []uint64) {
	for i := range buf {
		buf[i] = 0
	}
}
