package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbRowsPerMillerLoop = 2*nbGtLimbs + nbG1Limbs + nbG2Limbs   // current accumulator Gt, next accumulator Gt, G1 input G1 and G2 input G2
	nbRowsPerFinalExp   = nbGtLimbs + nbG1Limbs + nbG2Limbs + 2 // current accumulator Gt, G1 input G1 and G2 input G2. PLus 2 for the expected result
)

type multiMillerLoopMulCircuit struct {
	Instances []millerLoopMulInstance `gnark:",public"`
}

func newMultiMillerLoopMulCircuit(limits *Limits) *multiMillerLoopMulCircuit {
	return &multiMillerLoopMulCircuit{
		Instances: make([]millerLoopMulInstance, limits.NbMillerLoopInputInstances),
	}
}

func (c *multiMillerLoopMulCircuit) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bls12381.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	pairing, err := sw_bls12381.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("check instance %d: %w", i, err)
		}
	}
	return nil
}

type millerLoopMulInstance struct {
	Prev    gtElementWizard
	P       g1ElementWizard
	Q       g2ElementWizard
	Current gtElementWizard
}

func (c *millerLoopMulInstance) Check(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField], pairing *sw_bls12381.Pairing) error {
	prev := c.Prev.ToElement(api, fp)
	p := c.P.ToElement(api, fp)
	q := c.Q.ToElement(api, fp)
	current := c.Current.ToElement(api, fp)

	return evmprecompiles.ECPairBLSMillerLoopAndMul(api, prev, p, q, current)
}

type multiMillerLoopFinalExpCircuit struct {
	Instances []millerLoopFinalExpInstance `gnark:",public"`
}

func newMultiMillerLoopFinalExpCircuit(limits *Limits) *multiMillerLoopFinalExpCircuit {
	return &multiMillerLoopFinalExpCircuit{
		Instances: make([]millerLoopFinalExpInstance, limits.NbFinalExpInputInstances),
	}
}

func (c *multiMillerLoopFinalExpCircuit) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bls12381.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	pairing, err := sw_bls12381.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("check instance %d: %w", i, err)
		}
	}
	return nil
}

type millerLoopFinalExpInstance struct {
	Prev     gtElementWizard
	P        g1ElementWizard
	Q        g2ElementWizard
	Expected [2]frontend.Variable
}

func (c *millerLoopFinalExpInstance) Check(api frontend.API, fp *emulated.Field[sw_bls12381.BaseField], pairing *sw_bls12381.Pairing) error {
	prev := c.Prev.ToElement(api, fp)
	p := c.P.ToElement(api, fp)
	q := c.Q.ToElement(api, fp)

	api.AssertIsEqual(c.Expected[0], 0)
	return evmprecompiles.ECPairBLSMillerLoopAndFinalExpCheck(api, prev, p, q, c.Expected[1])
}
