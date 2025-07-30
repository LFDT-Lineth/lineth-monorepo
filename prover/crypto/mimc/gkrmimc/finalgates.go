package gkrmimc

import (
	"github.com/consensys/gnark/frontend"
	gGkr "github.com/consensys/gnark/std/gkr"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// FinalRoundGate represents the last round in a gnark circuit
//
// It performs all the actions required to complete the compression function of
// MiMC; including (1) the last application of the S-box x^17 as in the
// intermediate rounds and then adds twice the initial state and once the block
// to the result before returning.
type FinalRoundGate struct {
	Ark frontend.Variable
}

// NewFinalRoundGateGnark creates a new FinalRoundGate using the provided
// round constant which should correspond to the final rounds's constant of
// MiMC.
func NewFinalRoundGateGnark(ark field.Element) FinalRoundGate {
	return FinalRoundGate{
		Ark: ark,
	}
}

func (m FinalRoundGate) Evaluate(api gGkr.GateAPI, input ...frontend.Variable) frontend.Variable {

	if len(input) != 3 {
		utils.Panic("expected fan-in of 3, got %v", len(input))
	}

	// Parse the inputs
	initialState := input[0]
	block := input[1]
	currentState := input[2]

	// Compute the S-box function
	sum := api.Add(currentState, initialState, m.Ark)
	sumPow16 := api.Mul(sum, sum)          // sum^2
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^4
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^8
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^16
	sum = api.Mul(sumPow16, sum)

	// And add back the last values, following the Miyaguchi-Preneel
	// construction.
	return api.Add(sum, initialState, initialState, block)
}

func (m FinalRoundGate) Degree() int {
	return 17
}

// FinalRoundGateCrypto represents the last round in the GKR circuit for MiMC.
//
// It performs all the actions required to complete the compression function of
// MiMC; including (1) the last application of the S-box x^17 as in the
// intermediate rounds and then adds twice the initial state and once the block
// to the result before returning.
type FinalRoundGateCrypto struct {
	Ark field.Element
}

// NewFinalRoundGateCrypto creates a new MiMCCipherGate with the given parameters
func NewFinalRoundGateCrypto(ark field.Element) FinalRoundGateCrypto {
	return FinalRoundGateCrypto{
		Ark: ark,
	}
}

func (m FinalRoundGateCrypto) Evaluate(input ...field.Element) field.Element {

	if len(input) != 3 {
		utils.Panic("expected fan-in of 3, got %v", len(input))
	}

	// Parse the inputs
	initialState := input[0]
	block := input[1]
	curr := input[2]

	// Compute the S-box function
	var sum, sumPow16 field.Element
	sum.Add(&initialState, &curr).Add(&sum, &m.Ark)
	sumPow16.Mul(&sum, &sum)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sum.Mul(&sumPow16, &sum)

	// And add back the last values, following the Miyaguchi-Preneel
	// construction.
	sum.Add(&sum, &initialState)
	sum.Add(&sum, &initialState)
	sum.Add(&sum, &block)
	return sum
}

func (m FinalRoundGateCrypto) Degree() int {
	return 17
}
