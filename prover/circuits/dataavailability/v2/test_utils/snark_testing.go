package test_utils

import (
	"crypto/rand"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// TODO Delete most of the following

// Larger tests will need to be run as independent executables. So we need to make some testing utils publicly available.

func PadBytes(b []byte, targetLen int) []frontend.Variable {
	padded := make([]byte, targetLen)
	copy(padded, b)
	if _, err := rand.Read(padded[len(b):]); err != nil {
		panic(err)
	}
	return utils.ToVariableSlice(padded)
}
