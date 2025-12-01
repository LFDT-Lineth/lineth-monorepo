package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/fields_bls12381"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/math/bitslice"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbBits  = 128        // we use 128 bits limbs for the BLS12-381 field
	nbBytes = nbBits / 8 // 128 bits = 16 bytes

	// BLS scalar field is 255 bits, and we use 2 limbs of 128 bits to represent
	nbFrLimbs = 2 // (x_1, x_0) MSB order

	// BLS base field is 381 bits, and we use 4 limbs of 128 bits to represent
	// it. However, the highest limb is always zero, but the arithmetization
	// keeps it for nice alignment. We pass it to the circuit but check
	// explicitly that its 0.
	nbFpLimbs = 4 // (x_3, x_2, x_1, x_0) MSB order

	nbG1Limbs = 2 * nbFpLimbs  // (Ax, Ay)
	nbG2Limbs = 4 * nbFpLimbs  // (BxIm, BxRe, ByIm, ByRe)
	nbGtLimbs = 12 * nbFpLimbs // representation according to gnark - we don't use Gt in arithmetization, only in glue for accumulation
)

func nbLimbs(g Group) int {
	switch g {
	case G1:
		return nbG1Limbs
	case G2:
		return nbG2Limbs
	default:
		panic("unknown group for bls nbLimbs")
	}
}

var fpParams sw_bls12381.BaseField
var frParams sw_bls12381.ScalarField

type scalarElementWizard struct {
	S [nbFrLimbs]frontend.Variable
}

func (c scalarElementWizard) ToElement(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) *sw_bls12381.Scalar {
	// gnark represents the BLS12-381 Fr element on 4 limbs of 64 bits.
	Slimbs := make([]frontend.Variable, frParams.NbLimbs())
	Slimbs[2], Slimbs[3] = bitslice.Partition(api, c.S[0], 64, bitslice.WithNbDigits(128))
	Slimbs[0], Slimbs[1] = bitslice.Partition(api, c.S[1], 64, bitslice.WithNbDigits(128))
	return fr.NewElement(Slimbs)
}

type baseElementWizard struct {
	P [nbFpLimbs]frontend.Variable
}

func (c baseElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *emulated.Element[sw_bls12381.BaseField] {
	// gnark represents the BLS12-381 Fp elements on 6 limbs of 64 bits.
	Plimbs := make([]frontend.Variable, fpParams.NbLimbs())
	Plimbs[4], Plimbs[5] = bitslice.Partition(api, c.P[1], 64, bitslice.WithNbDigits(128))
	Plimbs[2], Plimbs[3] = bitslice.Partition(api, c.P[2], 64, bitslice.WithNbDigits(128))
	Plimbs[0], Plimbs[1] = bitslice.Partition(api, c.P[3], 64, bitslice.WithNbDigits(128))
	return fp.NewElement(Plimbs)
}

type g1ElementWizard struct {
	P [nbG1Limbs]frontend.Variable
}

func (c g1ElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *sw_bls12381.G1Affine {
	PXlimbs := make([]frontend.Variable, fpParams.NbLimbs())
	PYlimbs := make([]frontend.Variable, fpParams.NbLimbs())

	// gnark represents the BLS12-381 Fp element on 6 limbs of 64 bits.
	// Arithmetization uses 4 limbs of 128 bits, but the MSB limb is always 0.
	api.AssertIsEqual(c.P[0], 0)
	api.AssertIsEqual(c.P[nbFpLimbs], 0)
	for i := range nbFpLimbs - 1 {
		PXlimbs[len(PXlimbs)-(2*i+2)], PXlimbs[len(PXlimbs)-(2*i+1)] = bitslice.Partition(api, c.P[i+1], 64, bitslice.WithNbDigits(128))
		PYlimbs[len(PYlimbs)-(2*i+2)], PYlimbs[len(PYlimbs)-(2*i+1)] = bitslice.Partition(api, c.P[nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
	}
	PX := fp.NewElement(PXlimbs)
	PY := fp.NewElement(PYlimbs)
	P := sw_bls12381.G1Affine{
		X: *PX,
		Y: *PY,
	}
	return &P
}

type g2ElementWizard struct {
	Q [nbG2Limbs]frontend.Variable
}

func (c g2ElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *sw_bls12381.G2Affine {
	QXAlimbs := make([]frontend.Variable, fpParams.NbLimbs())
	QXBlimbs := make([]frontend.Variable, fpParams.NbLimbs())
	QYAlimbs := make([]frontend.Variable, fpParams.NbLimbs())
	QYBlimbs := make([]frontend.Variable, fpParams.NbLimbs())

	// assert that the MSB limb is 0 in arithmetization
	for i := range 4 {
		api.AssertIsEqual(c.Q[i*nbFpLimbs], 0)
	}

	for i := range nbFpLimbs - 1 {
		QXAlimbs[len(QXAlimbs)-(2*i+2)], QXAlimbs[len(QXAlimbs)-(2*i+1)] = bitslice.Partition(api, c.Q[i+1], 64, bitslice.WithNbDigits(128))
		QXBlimbs[len(QXBlimbs)-(2*i+2)], QXBlimbs[len(QXBlimbs)-(2*i+1)] = bitslice.Partition(api, c.Q[nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		QYAlimbs[len(QYAlimbs)-(2*i+2)], QYAlimbs[len(QYAlimbs)-(2*i+1)] = bitslice.Partition(api, c.Q[2*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		QYBlimbs[len(QYBlimbs)-(2*i+2)], QYBlimbs[len(QYBlimbs)-(2*i+1)] = bitslice.Partition(api, c.Q[3*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
	}
	QXA := fp.NewElement(QXAlimbs)
	QXB := fp.NewElement(QXBlimbs)
	QX := fields_bls12381.E2{
		A0: *QXA,
		A1: *QXB,
	}
	QYA := fp.NewElement(QYAlimbs)
	QYB := fp.NewElement(QYBlimbs)
	QY := fields_bls12381.E2{
		A0: *QYA,
		A1: *QYB,
	}
	var Q sw_bls12381.G2Affine
	Q.P.X = QX
	Q.P.Y = QY

	return &Q
}

type gtElementWizard struct {
	T [nbGtLimbs]frontend.Variable
}

func (c gtElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *sw_bls12381.GTEl {
	A0Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A1Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A2Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A3Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A4Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A5Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A6Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A7Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A8Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A9Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A10Limbs := make([]frontend.Variable, fpParams.NbLimbs())
	A11Limbs := make([]frontend.Variable, fpParams.NbLimbs())

	// assert that the MSB limb is 0 in arithmetization
	for i := range 12 {
		api.AssertIsEqual(c.T[i*nbFpLimbs], 0)
	}

	for i := range nbFpLimbs - 1 {
		A0Limbs[len(A0Limbs)-(2*i+2)], A0Limbs[len(A0Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[i+1], 64, bitslice.WithNbDigits(128))
		A1Limbs[len(A1Limbs)-(2*i+2)], A1Limbs[len(A1Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A2Limbs[len(A2Limbs)-(2*i+2)], A2Limbs[len(A2Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[2*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A3Limbs[len(A3Limbs)-(2*i+2)], A3Limbs[len(A3Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[3*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A4Limbs[len(A4Limbs)-(2*i+2)], A4Limbs[len(A4Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[4*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A5Limbs[len(A5Limbs)-(2*i+2)], A5Limbs[len(A5Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[5*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A6Limbs[len(A6Limbs)-(2*i+2)], A6Limbs[len(A6Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[6*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A7Limbs[len(A7Limbs)-(2*i+2)], A7Limbs[len(A7Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[7*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A8Limbs[len(A8Limbs)-(2*i+2)], A8Limbs[len(A8Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[8*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A9Limbs[len(A9Limbs)-(2*i+2)], A9Limbs[len(A9Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[9*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A10Limbs[len(A10Limbs)-(2*i+2)], A10Limbs[len(A10Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[10*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
		A11Limbs[len(A11Limbs)-(2*i+2)], A11Limbs[len(A11Limbs)-(2*i+1)] = bitslice.Partition(api, c.T[11*nbFpLimbs+i+1], 64, bitslice.WithNbDigits(128))
	}

	pairing, err := sw_bls12381.NewPairing(api)
	if err != nil {
		panic(fmt.Sprintf("new pairing: %v", err))
	}
	T := pairing.Ext12.FromTower([12]*emulated.Element[sw_bls12381.BaseField]{
		fp.NewElement(A0Limbs),
		fp.NewElement(A1Limbs),
		fp.NewElement(A2Limbs),
		fp.NewElement(A3Limbs),
		fp.NewElement(A4Limbs),
		fp.NewElement(A5Limbs),
		fp.NewElement(A6Limbs),
		fp.NewElement(A7Limbs),
		fp.NewElement(A8Limbs),
		fp.NewElement(A9Limbs),
		fp.NewElement(A10Limbs),
		fp.NewElement(A11Limbs),
	})

	return T
}

type element interface {
	sw_bls12381.G1Affine | sw_bls12381.G2Affine | sw_bls12381.GTEl
}

type convertable[T element] interface {
	g1ElementWizard | g2ElementWizard | gtElementWizard
	ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *T
}
