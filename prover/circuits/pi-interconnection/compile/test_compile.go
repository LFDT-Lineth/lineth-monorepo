package main

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/profile"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/utils/test_utils"
	"github.com/stretchr/testify/assert"
)

func main() {

	fmt.Println("creating wizard circuit")

	c, err := pi_interconnection.Compile(config.PublicInput{
		MaxNbDataAvailability: 400,
		MaxNbExecution:        400,
		ExecutionMaxNbMsg:     16,
		L2MsgMerkleDepth:      5,
		L2MsgMaxNbMerkle:      10,
	}, keccak.WizardCompilationParameters())

	var t test_utils.FakeTestingT
	assert.NoError(t, err)

	p := profile.Start()
	_, err = frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, c.Circuit, frontend.WithCapacity(1<<27))
	p.Stop()
	assert.NoError(t, err)
}
