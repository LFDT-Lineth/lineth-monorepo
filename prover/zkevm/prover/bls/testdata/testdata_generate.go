package main

import (
	"crypto/rand"
	"fmt"
	"iter"
	"math/big"
	"slices"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	fp_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	fr_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type affine interface {
	bls12381.G1Affine | bls12381.G2Affine
}

func generateG1Trivial() bls12381.G1Affine {
	var p bls12381.G1Affine
	p.SetInfinity()
	return p
}

func generateG2Trivial() bls12381.G2Affine {
	var p bls12381.G2Affine
	p.SetInfinity()
	return p
}

func generateTrivial[T affine]() T {
	var res T
	switch vv := any(&res).(type) {
	case *bls12381.G1Affine:
		v := generateG1Trivial()
		*vv = v
	case *bls12381.G2Affine:
		v := generateG2Trivial()
		*vv = v
	}
	return res
}

func generateG1OnCurve() bls12381.G1Affine {
	var x fp_bls12381.Element
	x.MustSetRandom()
	P := bls12381.GeneratePointNotInG1(x)
	var pa bls12381.G1Affine
	pa.FromJacobian(&P)
	if !pa.IsOnCurve() {
		panic("generated point is not on curve")
	}
	if pa.IsInSubGroup() {
		panic("generated point is in subgroup")
	}
	return pa
}

func generateG2OnCurve() bls12381.G2Affine {
	var x, y fp_bls12381.Element
	x.MustSetRandom()
	y.MustSetRandom()
	e := bls12381.E2{A0: x, A1: y}
	P := bls12381.GeneratePointNotInG2(e)
	var pa bls12381.G2Affine
	pa.FromJacobian(&P)
	if !pa.IsOnCurve() {
		panic("generated point is not on curve")
	}
	if pa.IsInSubGroup() {
		panic("generated point is in subgroup")
	}
	return pa
}

func generateOnCurve[T affine]() T {
	var res T
	switch vv := any(&res).(type) {
	case *bls12381.G1Affine:
		v := generateG1OnCurve()
		*vv = v
	case *bls12381.G2Affine:
		v := generateG2OnCurve()
		*vv = v
	}
	return res
}

func generateG1InSubgroup() bls12381.G1Affine {
	var p bls12381.G1Affine
	var s fr_bls12381.Element
	s.MustSetRandom()
	p.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
	return p
}

func generateG2InSubgroup() bls12381.G2Affine {
	var p bls12381.G2Affine
	var s fr_bls12381.Element
	s.MustSetRandom()
	p.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
	return p
}

func generateInSubgroup[T affine]() T {
	var res T
	switch vv := any(&res).(type) {
	case *bls12381.G1Affine:
		v := generateG1InSubgroup()
		*vv = v
	case *bls12381.G2Affine:
		v := generateG2InSubgroup()
		*vv = v
	}
	return res
}

func generateG1Invalid() bls12381.G1Affine {
	var p bls12381.G1Affine
	for {
		p.X.SetRandom()
		p.Y.SetRandom()
		if !p.IsOnCurve() {
			return p
		}
	}
}

func generateG2Invalid() bls12381.G2Affine {
	var p bls12381.G2Affine
	for {
		p.X.A0.SetRandom()
		p.X.A1.SetRandom()
		p.Y.A0.SetRandom()
		p.Y.A1.SetRandom()
		if !p.IsOnCurve() {
			return p
		}
	}
}

func generateInvalid[T affine]() T {
	var res T
	switch vv := any(&res).(type) {
	case *bls12381.G1Affine:
		v := generateG1Invalid()
		*vv = v
	case *bls12381.G2Affine:
		v := generateG2Invalid()
		*vv = v
	}
	return res
}

func generateScalar(scalarType msmInputType) *big.Int {
	switch scalarType {
	case msmScalarTrivial:
		return big.NewInt(0)
	case msmScalarRange:
		r, err := rand.Int(rand.Reader, fr_bls12381.Modulus())
		if err != nil {
			panic(fmt.Sprintf("failed to generate random scalar: %v", err))
		}
		return r
	case msmScalarBig:
		bound := new(big.Int).Lsh(big.NewInt(1), fr_bls12381.Bits)
		bound.Sub(bound, fr_bls12381.Modulus()) // ensure the scalar is less than the modulus
		// Generate a random scalar that is guaranteed to be big
		r, err := rand.Int(rand.Reader, bound)
		if err != nil {
			panic(fmt.Sprintf("failed to generate random scalar: %v", err))
		}
		r.Add(r, fr_bls12381.Modulus()) // ensure the scalar is larger than modulus
		return r
	default:
		panic(fmt.Sprintf("unknown scalar type: %d", scalarType))
	}
}

func recIt[T any](newIterator func() iter.Seq2[int, func() T], yield func([]T) bool, width int, vals []func() T) {
	if width == 0 {
		return
	}
	for _, v := range newIterator() {
		newVals := append(vals, v)
		if len(newVals) == width {
			ret := make([]T, len(newVals))
			for i, f := range newVals {
				ret[i] = f()
			}
			if !yield(ret) {
				return
			}
		} else {
			recIt(newIterator, yield, width, newVals)
		}
	}
}

func cartesianProduct[T any](width int, newIterator func() iter.Seq2[int, func() T]) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		recIt(newIterator, yield, width, []func() T{})
	}
}

func splitG1ToLimbs(p bls12381.G1Affine) []string {
	limbs := slices.Concat(
		splitBaseToLimbs(p.X),
		splitBaseToLimbs(p.Y),
	)
	return limbs
}

func splitG2ToLimbs(q bls12381.G2Affine) []string {
	limbs := slices.Concat(
		splitBaseToLimbs(q.X.A0),
		splitBaseToLimbs(q.X.A1),
		splitBaseToLimbs(q.Y.A0),
		splitBaseToLimbs(q.Y.A1),
	)
	return limbs
}

func splitToLimbs[T affine](p T) []string {
	switch vv := any(p).(type) {
	case bls12381.G1Affine:
		return splitG1ToLimbs(vv)
	case bls12381.G2Affine:
		return splitG2ToLimbs(vv)
	default:
		panic(fmt.Sprintf("unknown type for splitting to limbs: %T", p))
	}
}

func splitScalarToLimbs(s *big.Int) []string {
	var sb [32]byte
	s.FillBytes(sb[:])
	limbs := []string{
		fmt.Sprintf("0x%x", sb[0:16]),
		fmt.Sprintf("0x%x", sb[16:32]),
	}
	return limbs
}

func splitBaseToLimbs(x fp_bls12381.Element) []string {
	xb := x.Bytes()
	limbs := []string{
		"0x00000000000000000000000000000000",
		fmt.Sprintf("0x%x", xb[0:16]),
		fmt.Sprintf("0x%x", xb[16:32]),
		fmt.Sprintf("0x%x", xb[32:48]),
	}
	return limbs
}

func formatBoolAsInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func splitG1CompressedLimbs(bts [bls12381.SizeOfG1AffineCompressed]byte) []string {
	limbs := []string{
		fmt.Sprintf("0x%x", bts[0:16]),
		fmt.Sprintf("0x%x", bts[16:32]),
		fmt.Sprintf("0x%x", bts[32:48]),
	}
	return limbs
}

func splitVersionedHashToLimbs(h [32]byte) []string {
	limbs := []string{
		fmt.Sprintf("0x%x", h[0:16]),
		fmt.Sprintf("0x%x", h[16:32]),
	}
	return limbs
}
