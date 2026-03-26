package p256verify

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/math/emulated/emparams"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
)

const (
	nbBits = 16 // for large-field we use 128-bit limbs both for base and scalar fields

	nbFrLimbs  = 16 // P-256 scalar field represented with 16 limbs of 16 bits
	nbFpLimbs  = 16 // P-256 base field represented with 16 limbs of 16 bits
	nbResLimbs = 16 // it is a boolean result represented with 16 limbs of 16 bits.

	nbG1Limbs = 2 * nbFpLimbs // (Ax, Ay)

	nbRows = 3*nbFrLimbs + nbG1Limbs + nbResLimbs // msg
)

type scalarfield = emparams.P256Fr
type basefield = emparams.P256Fp

type scalarElementWizard struct {
	S [nbFrLimbs]frontend.Variable
}

func (c scalarElementWizard) ToElement(api frontend.API, fr *emulated.Field[scalarfield]) *emulated.Element[scalarfield] {
	S16 := make([]frontend.Variable, nbFrLimbs)
	copy(S16[0:8], c.S[8:16])
	copy(S16[8:16], c.S[0:8])
	return gnarkutil.EmulatedFromLimbSlice(api, fr, S16, nbBits)
}

type baseElementWizard struct {
	P [nbFpLimbs]frontend.Variable
}

func (c baseElementWizard) ToElement(api frontend.API, fp *emulated.Field[basefield]) *emulated.Element[basefield] {
	P16 := make([]frontend.Variable, nbFpLimbs)
	copy(P16[0:8], c.P[8:16])
	copy(P16[8:16], c.P[0:8])
	return gnarkutil.EmulatedFromLimbSlice(api, fp, P16, nbBits)
}

type P256VerifyInstance struct {
	H        scalarElementWizard           `gnark:",public"`
	R        scalarElementWizard           `gnark:",public"`
	S        scalarElementWizard           `gnark:",public"`
	Qx       baseElementWizard             `gnark:",public"`
	Qy       baseElementWizard             `gnark:",public"`
	Expected [nbResLimbs]frontend.Variable `gnark:",public"`
}

type multiP256VerifyInstanceCircuit struct {
	Instances []P256VerifyInstance
}

func (c *multiP256VerifyInstanceCircuit) Define(api frontend.API) error {
	scalarApi, err := emulated.NewField[scalarfield](api)
	if err != nil {
		return fmt.Errorf("new scalar field: %w", err)
	}
	baseApi, err := emulated.NewField[basefield](api)
	if err != nil {
		return fmt.Errorf("new base field: %w", err)
	}

	nbInstances := len(c.Instances)
	for i := 0; i < nbInstances; i++ {
		h := c.Instances[i].H.ToElement(api, scalarApi)
		r := c.Instances[i].R.ToElement(api, scalarApi)
		s := c.Instances[i].S.ToElement(api, scalarApi)
		qx := c.Instances[i].Qx.ToElement(api, baseApi)
		qy := c.Instances[i].Qy.ToElement(api, baseApi)
		// the high limb of the result is always zero. It is represented as 8 limbs of 16 bits.
		for j := range nbResLimbs / 2 {
			api.AssertIsEqual(c.Instances[i].Expected[j], 0)
		}
		// and low limb is LSB boolean
		for j := range nbResLimbs/2 - 1 {
			api.AssertIsEqual(c.Instances[i].Expected[nbResLimbs/2+j+1], 0)
		}
		// the expected result should be boolean
		expected := c.Instances[i].Expected[nbResLimbs/2]
		api.AssertIsBoolean(expected)
		res := evmprecompiles.P256Verify(api, h, r, s, qx, qy)
		api.AssertIsEqual(res, expected)
	}
	return nil
}

func newP256VerifyCircuit(limits *Limits) frontend.Circuit {
	return &multiP256VerifyInstanceCircuit{
		Instances: make([]P256VerifyInstance, limits.NbInputInstances),
	}
}
