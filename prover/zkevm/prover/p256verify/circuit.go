package p256verify

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/bitslice"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/math/emulated/emparams"
)

const (
	nbBits  = 128 // for large-field we use 128-bit limbs both for base and scalar fields
	nbBytes = nbBits / 8

	nbFrLimbs  = 2 // P-256 scalar field represented with 2 limbs of 128 bits
	nbFpLimbs  = 2 // P-256 base field represented with 2 limbs of 128 bits
	nbResLimbs = 2

	nbG1Limbs = 2 * nbFpLimbs // (Ax, Ay)

	nbRows = 3*nbFrLimbs + nbG1Limbs + nbResLimbs // msg
)

type scalarfield = emparams.P256Fr
type basefield = emparams.P256Fp

var fpParams basefield
var frParams scalarfield

type scalarElementWizard struct {
	S [nbFrLimbs]frontend.Variable
}

func (c scalarElementWizard) ToElement(api frontend.API, fr *emulated.Field[scalarfield]) *emulated.Element[scalarfield] {
	Slimbs := make([]frontend.Variable, frParams.NbLimbs())
	Slimbs[2], Slimbs[3] = bitslice.Partition(api, c.S[0], 64, bitslice.WithNbDigits(128))
	Slimbs[0], Slimbs[1] = bitslice.Partition(api, c.S[1], 64, bitslice.WithNbDigits(128))
	return fr.NewElement(Slimbs)
}

type baseElementWizard struct {
	P [nbFpLimbs]frontend.Variable
}

func (c baseElementWizard) ToElement(api frontend.API, fp *emulated.Field[basefield]) *emulated.Element[basefield] {
	Plimbs := make([]frontend.Variable, fpParams.NbLimbs())
	Plimbs[2], Plimbs[3] = bitslice.Partition(api, c.P[0], 64, bitslice.WithNbDigits(128))
	Plimbs[0], Plimbs[1] = bitslice.Partition(api, c.P[1], 64, bitslice.WithNbDigits(128))
	return fp.NewElement(Plimbs)
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
		// the high limb of the result is always zero
		api.AssertIsEqual(c.Instances[i].Expected[0], 0)
		// the expected result should be boolean
		expected := c.Instances[i].Expected[1]
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
