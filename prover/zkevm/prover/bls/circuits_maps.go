package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/fields_bls12381"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbRowsPerG1Map = nbFpLimbs + nbG1Limbs   // input is Fp element and expected output is G1 element
	nbRowsPerG2Map = 2*nbFpLimbs + nbG2Limbs // input is Fp2 element and expected output is G2 element
)

func nbRowsPerMap(g Group) int {
	switch g {
	case G1:
		return nbRowsPerG1Map
	case G2:
		return nbRowsPerG2Map
	default:
		panic("unknown group for nbRowsPerMap")
	}
}

type mapInstance[C convertable[T], T element] struct {
	Input  []baseElementWizard // len==1 for G1, len==2 for G2
	Mapped C
}
type multiMapCircuit[C convertable[T], T element] struct {
	Instances []mapInstance[C, T] `gnark:",public"`
}

func (c *multiMapCircuit[C, T]) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bls12381.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	switch vv := any(c.Instances).(type) {
	case []mapInstance[g1ElementWizard, sw_bls12381.G1Affine]:
		for i := range c.Instances {
			if len(c.Instances[i].Input) != 1 {
				return fmt.Errorf("instance %d expected 1 input for G1 map to G1, got %d", i, len(c.Instances[i].Input))
			}
			toMap := vv[i].Input[0].ToElement(api, fp)
			tMapped := vv[i].Mapped.ToElement(api, fp)
			if err := evmprecompiles.ECMapToG1BLS(api, toMap, tMapped); err != nil {
				return fmt.Errorf("instance %d map to G1: %w", i, err)
			}
		}
	case []mapInstance[g2ElementWizard, sw_bls12381.G2Affine]:
		for i := range c.Instances {
			if len(c.Instances[i].Input) != 2 {
				return fmt.Errorf("expected 2 inputs for G2 map to G1, got %d", len(c.Instances[i].Input))
			}
			toMap := fields_bls12381.E2{
				A0: *c.Instances[i].Input[0].ToElement(api, fp),
				A1: *c.Instances[i].Input[1].ToElement(api, fp),
			}
			tMapped := vv[i].Mapped.ToElement(api, fp)
			if err := evmprecompiles.ECMapToG2BLS(api, &toMap, tMapped); err != nil {
				return fmt.Errorf("instance %d map to G2: %w", i, err)
			}
		}
	default:
		return fmt.Errorf("unknown group %T for bls map to G1 circuit", vv)
	}
	return nil
}

func NewMapCircuit(g Group, limits *Limits) frontend.Circuit {
	switch g {
	case G1:
		res := &multiMapCircuit[g1ElementWizard, sw_bls12381.G1Affine]{
			Instances: make([]mapInstance[g1ElementWizard, sw_bls12381.G1Affine], limits.NbG1MapToInputInstances),
		}
		for i := range res.Instances {
			res.Instances[i].Input = make([]baseElementWizard, 1)
		}
		return res
	case G2:
		res := &multiMapCircuit[g2ElementWizard, sw_bls12381.G2Affine]{
			Instances: make([]mapInstance[g2ElementWizard, sw_bls12381.G2Affine], limits.NbG2MapToInputInstances),
		}
		for i := range res.Instances {
			res.Instances[i].Input = make([]baseElementWizard, 2)
		}
		return res
	default:
		panic(fmt.Sprintf("unknown group %s for bls map to G1 circuit", g.String()))
	}
}
