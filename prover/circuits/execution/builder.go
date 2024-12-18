package execution

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

type builder struct {
	zkevm *zkevm.ZkEvm
}

func NewBuilder(z *zkevm.ZkEvm) *builder {
	return &builder{zkevm: z}
}

func (b *builder) Compile() (constraint.ConstraintSystem, error) {
	return makeCS(b.zkevm), nil
}

// builds the circuit
func makeCS(z *zkevm.ZkEvm) constraint.ConstraintSystem {
	circuit := Allocate(z)

	scs, err := frontend.Compile(fr.Modulus(), scs.NewBuilder, &circuit, frontend.WithCapacity(1<<24))
	if err != nil {
		panic(err)
	}
	return scs
}
