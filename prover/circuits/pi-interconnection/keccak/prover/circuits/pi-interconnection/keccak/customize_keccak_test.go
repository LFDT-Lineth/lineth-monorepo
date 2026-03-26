package keccak

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestCustomizedKeccak(t *testing.T) {
	t.Log("compiling")
	c := NewWizardVerifierSubCircuit(400, dummy.Compile)
	t.Log("proving")

	var providers [][]byte
	// generate 20 random slices of bytes
	for i := 0; i < 20; i++ {
		// choose a random length for the slice
		nBig, err := rand.Int(rand.Reader, big.NewInt(1000))
		if err != nil {
			panic(err)
		}
		length := nBig.Int64()

		// generate random bytes
		slice := make([]byte, length)
		rand.Read(slice)
		providers = append(providers, slice)
	}
	proof := c.prove(providers)
	t.Log("verifying")
	assert.NoError(t, wizard.Verify(c.compiled, proof))
}
