package bls

import (
	"fmt"
	"math/big"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
)

func set(nbL int, q *fp.Element, ls []field.Element) {
	var buf []byte
	for i := range nbL - 1 {
		lbts := ls[i+1].Bytes()
		buf = append(buf, lbts[nbBytes:]...)
	}
	q.SetBytes(buf)
}

func nativeScalarMulAndSum(g Group, currentAccumulator []field.Element, point []field.Element, scalar []field.Element) (nextAccumulator []field.Element) {
	nbL := nbLimbs(g)
	if len(scalar) != nbFrLimbs {
		panic(fmt.Sprintf("scalar must have exactly %d limbs, got %d", nbFrLimbs, len(scalar)))
	}
	if len(currentAccumulator) != nbL {
		panic(fmt.Sprintf("currentAccumulator must have exactly %d limbs, got %d", nbL, len(currentAccumulator)))
	}
	if len(point) != nbL {
		panic(fmt.Sprintf("point must have exactly %d limbs, got %d", nbL, len(point)))
	}
	var buf []byte
	for i := range nbFrLimbs {
		lbts := scalar[i].Bytes()
		buf = append(buf, lbts[nbBytes:]...)
	}
	s := new(big.Int).SetBytes(buf)

	switch g {
	case G1:
		var C, P, N bls12381.G1Affine
		set(nbL/2, &P.X, point[:nbL/2])
		set(nbL/2, &P.Y, point[nbL/2:])
		set(nbL/2, &C.X, currentAccumulator[:nbL/2])
		set(nbL/2, &C.Y, currentAccumulator[nbL/2:])
		N.ScalarMultiplication(&P, s)
		N.Add(&C, &N)
		NXBytes := N.X.Bytes()
		NYBytes := N.Y.Bytes()
		nextAccumulator = make([]field.Element, nbL)
		for i := range nbL/2 - limbs.NbLimbU128 {
			nextAccumulator[i+limbs.NbLimbU128].SetBytes(NXBytes[i*nbBytes : (i+1)*nbBytes])
			nextAccumulator[i+limbs.NbLimbU128+nbL/2].SetBytes(NYBytes[i*nbBytes : (i+1)*nbBytes])
		}
	case G2:
		var C, P, N bls12381.G2Affine
		set(nbL/4, &P.X.A0, point[:nbL/4])
		set(nbL/4, &P.X.A1, point[nbL/4:nbL/2])
		set(nbL/4, &P.Y.A0, point[nbL/2:3*nbL/4])
		set(nbL/4, &P.Y.A1, point[3*nbL/4:])
		set(nbL/4, &C.X.A0, currentAccumulator[:nbL/4])
		set(nbL/4, &C.X.A1, currentAccumulator[nbL/4:nbL/2])
		set(nbL/4, &C.Y.A0, currentAccumulator[nbL/2:3*nbL/4])
		set(nbL/4, &C.Y.A1, currentAccumulator[3*nbL/4:])
		N.ScalarMultiplication(&P, s)
		N.Add(&C, &N)
		NXA0Bytes := N.X.A0.Bytes()
		NXA1Bytes := N.X.A1.Bytes()
		NYA0Bytes := N.Y.A0.Bytes()
		NYA1Bytes := N.Y.A1.Bytes()
		nextAccumulator = make([]field.Element, nbL)
		for i := range nbL/4 - limbs.NbLimbU128 {
			nextAccumulator[i+limbs.NbLimbU128].SetBytes(NXA0Bytes[i*nbBytes : (i+1)*nbBytes])
			nextAccumulator[i+limbs.NbLimbU128+nbL/4].SetBytes(NXA1Bytes[i*nbBytes : (i+1)*nbBytes])
			nextAccumulator[i+limbs.NbLimbU128+nbL/2].SetBytes(NYA0Bytes[i*nbBytes : (i+1)*nbBytes])
			nextAccumulator[i+limbs.NbLimbU128+3*nbL/4].SetBytes(NYA1Bytes[i*nbBytes : (i+1)*nbBytes])
		}
	}
	return nextAccumulator
}

func nativeGtZero() []field.Element {
	// C0.B0.A0 is 1, in MSB order with 16-bit limbs.
	// Each Fp element uses 32 limbs (256 bits for 381-bit field with padding).
	// The first limbs.NbLimbU128 (8) limbs are zero padding, then actual data follows.
	// The value 1 is in the last position of the first Fp component.
	ret := make([]field.Element, nbGtLimbs)
	ret[nbFpLimbs-1].SetOne() // 1 is at position 7 (last of the first 128-bit chunk)
	return ret
}

func nativeMillerLoopAndSum(prevAccumulator []field.Element, pointG1 []field.Element, pointG2 []field.Element) (nextAccumulator []field.Element) {
	if len(prevAccumulator) != nbGtLimbs {
		panic(fmt.Sprintf("currentAccumulator must have exactly %d limbs, got %d", nbGtLimbs, len(prevAccumulator)))
	}
	if len(pointG1) != nbG1Limbs {
		panic(fmt.Sprintf("pointG1 must have exactly %d limbs, got %d", nbG1Limbs, len(pointG1)))
	}
	if len(pointG2) != nbG2Limbs {
		panic(fmt.Sprintf("pointG2 must have exactly %d limbs, got %d", nbG2Limbs, len(pointG2)))
	}
	var prev bls12381.GT
	var P bls12381.G1Affine
	var Q bls12381.G2Affine
	set(nbGtLimbs/12, &prev.C0.B0.A0, prevAccumulator[:nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C0.B0.A1, prevAccumulator[nbGtLimbs/12:2*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C0.B1.A0, prevAccumulator[2*nbGtLimbs/12:3*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C0.B1.A1, prevAccumulator[3*nbGtLimbs/12:4*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C0.B2.A0, prevAccumulator[4*nbGtLimbs/12:5*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C0.B2.A1, prevAccumulator[5*nbGtLimbs/12:6*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B0.A0, prevAccumulator[6*nbGtLimbs/12:7*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B0.A1, prevAccumulator[7*nbGtLimbs/12:8*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B1.A0, prevAccumulator[8*nbGtLimbs/12:9*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B1.A1, prevAccumulator[9*nbGtLimbs/12:10*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B2.A0, prevAccumulator[10*nbGtLimbs/12:11*nbGtLimbs/12])
	set(nbGtLimbs/12, &prev.C1.B2.A1, prevAccumulator[11*nbGtLimbs/12:])
	set(nbG1Limbs/2, &P.X, pointG1[:nbG1Limbs/2])
	set(nbG1Limbs/2, &P.Y, pointG1[nbG1Limbs/2:])
	set(nbG2Limbs/4, &Q.X.A0, pointG2[:nbG2Limbs/4])
	set(nbG2Limbs/4, &Q.X.A1, pointG2[nbG2Limbs/4:nbG2Limbs/2])
	set(nbG2Limbs/4, &Q.Y.A0, pointG2[nbG2Limbs/2:3*nbG2Limbs/4])
	set(nbG2Limbs/4, &Q.Y.A1, pointG2[3*nbG2Limbs/4:])
	var next bls12381.GT
	lines := bls12381.PrecomputeLines(Q)
	mlres, err := bls12381.MillerLoopFixedQ(
		[]bls12381.G1Affine{P},
		[][2][len(bls12381.LoopCounter) - 1]bls12381.LineEvaluationAff{lines})
	if err != nil {
		panic(fmt.Sprintf("failed to compute miller loop: %v", err))
	}
	mlres.Conjugate(&mlres)
	next.Mul(&prev, &mlres)

	nextAccumulator = make([]field.Element, nbGtLimbs)
	C0B0A0Bytes := next.C0.B0.A0.Bytes()
	C0B0A1Bytes := next.C0.B0.A1.Bytes()
	C0B1A0Bytes := next.C0.B1.A0.Bytes()
	C0B1A1Bytes := next.C0.B1.A1.Bytes()
	C0B2A0Bytes := next.C0.B2.A0.Bytes()
	C0B2A1Bytes := next.C0.B2.A1.Bytes()
	C1B0A0Bytes := next.C1.B0.A0.Bytes()
	C1B0A1Bytes := next.C1.B0.A1.Bytes()
	C1B1A0Bytes := next.C1.B1.A0.Bytes()
	C1B1A1Bytes := next.C1.B1.A1.Bytes()
	C1B2A0Bytes := next.C1.B2.A0.Bytes()
	C1B2A1Bytes := next.C1.B2.A1.Bytes()
	// Each Fp component uses nbGtLimbs/12 = 32 limbs. The first limbs.NbLimbU128 (8) are zero padding.
	nbFpComponent := nbGtLimbs / 12
	for i := range nbFpComponent - limbs.NbLimbU128 {
		nextAccumulator[i+limbs.NbLimbU128].SetBytes(C0B0A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+nbFpComponent].SetBytes(C0B0A1Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+2*nbFpComponent].SetBytes(C0B1A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+3*nbFpComponent].SetBytes(C0B1A1Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+4*nbFpComponent].SetBytes(C0B2A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+5*nbFpComponent].SetBytes(C0B2A1Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+6*nbFpComponent].SetBytes(C1B0A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+7*nbFpComponent].SetBytes(C1B0A1Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+8*nbFpComponent].SetBytes(C1B1A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+9*nbFpComponent].SetBytes(C1B1A1Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+10*nbFpComponent].SetBytes(C1B2A0Bytes[i*nbBytes : (i+1)*nbBytes])
		nextAccumulator[i+limbs.NbLimbU128+11*nbFpComponent].SetBytes(C1B2A1Bytes[i*nbBytes : (i+1)*nbBytes])
	}
	return nextAccumulator
}
