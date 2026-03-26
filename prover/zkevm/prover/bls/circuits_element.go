package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/fields_bls12381"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
)

const (
	nbBits  = 16         // we use 16 bits limbs for the BLS12-381 field
	nbBytes = nbBits / 8 // 16 bits = 2 bytes

	// BLS scalar field is 255 bits, and we use 16 limbs of 16 bits to represent
	nbFrLimbs    = 16 // (x_1, x_0) MSB order
	nbFrLimbs128 = 2

	// BLS base field is 381 bits, and we use 32 limbs of 16 bits to represent
	// it. However, the highest 8 limbs are always zero, but the arithmetization
	// keeps it for nice alignment. We pass it to the circuit but check
	// explicitly that its 0.
	nbFpLimbs    = 32 // (x_3, x_2, x_1, x_0) MSB order
	nbFpLimbs128 = 4

	nbG1Limbs    = 2 * nbFpLimbs  // (Ax, Ay)
	nbG2Limbs    = 4 * nbFpLimbs  // (BxIm, BxRe, ByIm, ByRe)
	nbGtLimbs    = 12 * nbFpLimbs // representation according to gnark - we don't use Gt in arithmetization, only in glue for accumulation
	nbG1Limbs128 = 2 * nbFpLimbs128
	nbG2Limbs128 = 4 * nbFpLimbs128
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

func nbLimbs128(g Group) int {
	switch g {
	case G1:
		return nbG1Limbs128
	case G2:
		return nbG2Limbs128
	default:
		panic("unknown group for bls nbLimbs128")
	}
}

type scalarElementWizard struct {
	S [nbFrLimbs]frontend.Variable
}

func (c scalarElementWizard) ToElement(api frontend.API, fr *emulated.Field[sw_bls12381.ScalarField]) *sw_bls12381.Scalar {
	// Reorder limbs from MSB to LSB
	S16 := make([]frontend.Variable, nbFrLimbs)
	copy(S16[0:8], c.S[8:16])
	copy(S16[8:16], c.S[0:8])
	S := gnarkutil.EmulatedFromLimbSlice(api, fr, S16, nbBits)
	return S
}

type baseElementWizard struct {
	P [nbFpLimbs]frontend.Variable
}

func (c baseElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *emulated.Element[sw_bls12381.BaseField] {
	// Reorder limbs from MSB to LSB. We additionally ignore the highest 8 limbs which are expected to be 0.
	for i := range 8 {
		api.AssertIsEqual(c.P[i], 0)
	}
	P16 := make([]frontend.Variable, 24)
	copy(P16[0:8], c.P[24:32])
	copy(P16[8:16], c.P[16:24])
	copy(P16[16:24], c.P[8:16])
	return gnarkutil.EmulatedFromLimbSlice(api, fp, P16, nbBits)
}

type g1ElementWizard struct {
	P [nbG1Limbs]frontend.Variable
}

func (c g1ElementWizard) ToElement(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField]) *sw_bls12381.G1Affine {
	// gnark represents the BLS12-381 Fp element on 6 limbs of 64 bits.
	// Arithmetization uses 4 limbs of 128 bits, but the MSB limb is always 0.
	// Arithmetization layout for G1 point P:
	//  - limbs 0-32 are X
	//  - limbs 32-64 are Y
	//
	// Furthermore, arithmetization uses orders the limbs in HI-LO format:
	//  - limbs 0-16 are X_HI
	//  - limbs 16-32 are X_LO
	//  - limbs 32-48 are Y_HI
	//  - limbs 48-64 are Y_LO
	//
	// As BLS12-381 requires only 381 bits, then highest 0 limbs are expected to be 0:
	// - limbs 0-8 of X_HI are 0
	// - limbs 0-8 of Y_HI are 0
	for i := range 8 {
		api.AssertIsEqual(c.P[i], 0)
		api.AssertIsEqual(c.P[nbFpLimbs+i], 0)
	}
	PX16 := make([]frontend.Variable, 24)
	copy(PX16[0:8], c.P[24:32])
	copy(PX16[8:16], c.P[16:24])
	copy(PX16[16:24], c.P[8:16])
	PY16 := make([]frontend.Variable, 24)
	copy(PY16[0:8], c.P[56:64])
	copy(PY16[8:16], c.P[48:56])
	copy(PY16[16:24], c.P[40:48])
	PX := gnarkutil.EmulatedFromLimbSlice(api, fp, PX16, nbBits)
	PY := gnarkutil.EmulatedFromLimbSlice(api, fp, PY16, nbBits)
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
	// see comments in g1ElementWizard.ToElement regarding limb ordering and zero
	// padding.
	// Arithmetization layout for G2 point Q:
	//  - limbs 0-64 are X = (X_im, X_re)
	//  - limbs 64-128 are Y = (Y_im, Y_re)
	for i := range 8 {
		api.AssertIsEqual(c.Q[i], 0)
		api.AssertIsEqual(c.Q[nbFpLimbs+i], 0)
		api.AssertIsEqual(c.Q[2*nbFpLimbs+i], 0)
		api.AssertIsEqual(c.Q[3*nbFpLimbs+i], 0)
	}
	QXA16 := make([]frontend.Variable, 24)
	QXB16 := make([]frontend.Variable, 24)
	QYA16 := make([]frontend.Variable, 24)
	QYB16 := make([]frontend.Variable, 24)
	// 0-32 are XA
	copy(QXA16[0:8], c.Q[24:32])
	copy(QXA16[8:16], c.Q[16:24])
	copy(QXA16[16:24], c.Q[8:16])
	// 32-64 are XB
	copy(QXB16[0:8], c.Q[56:64])
	copy(QXB16[8:16], c.Q[48:56])
	copy(QXB16[16:24], c.Q[40:48])
	// 64-96 are YA
	copy(QYA16[0:8], c.Q[88:96])
	copy(QYA16[8:16], c.Q[80:88])
	copy(QYA16[16:24], c.Q[72:80])
	// 96-128 are YB
	copy(QYB16[0:8], c.Q[120:128])
	copy(QYB16[8:16], c.Q[112:120])
	copy(QYB16[16:24], c.Q[104:112])

	QXA := gnarkutil.EmulatedFromLimbSlice(api, fp, QXA16, nbBits)
	QXB := gnarkutil.EmulatedFromLimbSlice(api, fp, QXB16, nbBits)
	QYA := gnarkutil.EmulatedFromLimbSlice(api, fp, QYA16, nbBits)
	QYB := gnarkutil.EmulatedFromLimbSlice(api, fp, QYB16, nbBits)
	QX := fields_bls12381.E2{
		A0: *QXA,
		A1: *QXB,
	}
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
	// Gt element consists of 12 Fp elements (E12 = E6 x E6, E6 = E2 x E2 x E2, E2 = Fp x Fp)
	// Each Fp element is 32 limbs of 16 bits in arithmetization (4 x 128-bit limbs split into 8 sublimbs each).
	// The limbs are in HI-LO format within each Fp element, and the first 8 limbs (highest 128-bit chunk) are 0.
	//
	// Arithmetization layout for Gt element T:
	//  - limbs 0-32 are A0 (C0.B0.A0)
	//  - limbs 32-64 are A1 (C0.B0.A1)
	//  - limbs 64-96 are A2 (C0.B1.A0)
	//  - limbs 96-128 are A3 (C0.B1.A1)
	//  - limbs 128-160 are A4 (C0.B2.A0)
	//  - limbs 160-192 are A5 (C0.B2.A1)
	//  - limbs 192-224 are A6 (C1.B0.A0)
	//  - limbs 224-256 are A7 (C1.B0.A1)
	//  - limbs 256-288 are A8 (C1.B1.A0)
	//  - limbs 288-320 are A9 (C1.B1.A1)
	//  - limbs 320-352 are A10 (C1.B2.A0)
	//  - limbs 352-384 are A11 (C1.B2.A1)

	// Assert that the MSB 8 limbs of each Fp element are 0
	for i := range 12 {
		for j := range 8 {
			api.AssertIsEqual(c.T[i*nbFpLimbs+j], 0)
		}
	}

	// Helper to extract 24 limbs from a 32-limb Fp element, reversing the 128-bit chunk order
	extractFpLimbs := func(offset int) []frontend.Variable {
		limbs := make([]frontend.Variable, 24)
		// Reverse order of 128-bit chunks (each chunk is 8 x 16-bit limbs)
		copy(limbs[0:8], c.T[offset+24:offset+32])  // lowest 128-bit chunk
		copy(limbs[8:16], c.T[offset+16:offset+24]) // middle 128-bit chunk
		copy(limbs[16:24], c.T[offset+8:offset+16]) // highest non-zero 128-bit chunk
		return limbs
	}

	A0 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(0*nbFpLimbs), 16)
	A1 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(1*nbFpLimbs), 16)
	A2 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(2*nbFpLimbs), 16)
	A3 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(3*nbFpLimbs), 16)
	A4 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(4*nbFpLimbs), 16)
	A5 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(5*nbFpLimbs), 16)
	A6 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(6*nbFpLimbs), 16)
	A7 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(7*nbFpLimbs), 16)
	A8 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(8*nbFpLimbs), 16)
	A9 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(9*nbFpLimbs), 16)
	A10 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(10*nbFpLimbs), 16)
	A11 := gnarkutil.EmulatedFromLimbSlice(api, fp, extractFpLimbs(11*nbFpLimbs), 16)

	pairing, err := sw_bls12381.NewPairing(api)
	if err != nil {
		panic(fmt.Sprintf("new pairing: %v", err))
	}
	T := pairing.Ext12.FromTower([12]*emulated.Element[sw_bls12381.BaseField]{
		A0, A1, A2, A3, A4, A5, A6, A7, A8, A9, A10, A11,
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
