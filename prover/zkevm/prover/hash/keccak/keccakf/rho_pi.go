package keccakfkoalabear

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf/common"
)

// rhoPi module, responsible for updating the state in the rhoPi step of keccakf
type rhoPi struct {
	// state before applying the rho step
	StateCurr common.StateInBits
	// state after bit rotation, in bits
	StateNext *common.StateInBits
}

// newRho creates a new rho module, declares the columns and constraints and returns its pointer
func newRho(comp *wizard.CompiledIOP, stateCurr common.StateInBits) *rhoPi {

	rho := &rhoPi{
		StateCurr: stateCurr,
		StateNext: &common.StateInBits{},
	}

	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			for z := 0; z < 64; z++ {
				// find the new position of each bit after rotation
				newPos := (z + keccak.LR[x][y]) % 64
				// assign the bit to the new position
				rho.StateNext[y][(2*x+3*y)%5][newPos] = stateCurr[x][y][z]
			}
		}
	}
	return rho
}

// assignRho assigns the values to the columns of rho step.
func (rho *rhoPi) assignRho(run *wizard.ProverRuntime, stateCurr common.StateInBits) {
	// it does nothing as it is just rotation and shuffling of columns and does not create any new columns.
}
