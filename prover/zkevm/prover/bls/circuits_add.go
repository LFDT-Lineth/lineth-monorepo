package bls

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bls12381"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
)

const (
	nbRowsPerG1Add = 3 * nbG1Limbs // 2 for the inputs, 1 for the output
	nbRowsPerG2Add = 3 * nbG2Limbs // 2 for the inputs, 1 for the output
)

func nbRowsPerAdd(g Group) int {
	switch g {
	case G1:
		return nbRowsPerG1Add
	case G2:
		return nbRowsPerG2Add
	default:
		panic(fmt.Sprintf("unknown group for BLS add circuit: %v", g))
	}
}

type addInstance[C convertable[T], T element] struct {
	InputLeft, InputRight C `gnark:",public"`
	Res                   C `gnark:",public"`
}
type multiAddCircuit[C convertable[T], T element] struct {
	Instances []addInstance[C, T]
}

func (c *multiAddCircuit[C, T]) Define(api frontend.API) error {
	f, err := emulated.NewField[sw_bls12381.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field: %w", err)
	}
	nbInstances := len(c.Instances)
	switch vv := any(c.Instances).(type) {
	case []addInstance[g1ElementWizard, sw_bls12381.G1Affine]:
		for i := 0; i < nbInstances; i++ {
			left := vv[i].InputLeft.ToElement(api, f)
			right := vv[i].InputRight.ToElement(api, f)
			expected := vv[i].Res.ToElement(api, f)
			evmprecompiles.ECAddG1BLS(api, left, right, expected)
		}
	case []addInstance[g2ElementWizard, sw_bls12381.G2Affine]:
		for i := 0; i < nbInstances; i++ {
			left := vv[i].InputLeft.ToElement(api, f)
			right := vv[i].InputRight.ToElement(api, f)
			expected := vv[i].Res.ToElement(api, f)
			evmprecompiles.ECAddG2BLS(api, left, right, expected)
		}
	default:
		return fmt.Errorf("unknown element type %T for bls add circuit", vv)
	}

	return nil
}

func newAddCircuit(g Group, limits *Limits) frontend.Circuit {
	switch g {
	case G1:
		return &multiAddCircuit[g1ElementWizard, sw_bls12381.G1Affine]{Instances: make([]addInstance[g1ElementWizard, sw_bls12381.G1Affine], limits.NbG1AddInputInstances)}
	case G2:
		return &multiAddCircuit[g2ElementWizard, sw_bls12381.G2Affine]{Instances: make([]addInstance[g2ElementWizard, sw_bls12381.G2Affine], limits.NbG2AddInputInstances)}
	default:
		panic(fmt.Sprintf("unknown group for bls add circuit: %v", g))
	}
}
