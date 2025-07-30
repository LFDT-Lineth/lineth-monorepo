package gkrmimc

import (
	"github.com/consensys/gnark/frontend"
	gGkr "github.com/consensys/gnark/std/gkr"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// RoundGate represents a normal round of gkr (i.e. any round except for the
// first and last ones). It represents the computation of the S-box of MiMC
//
//	(curr + init + ark)^17
//
// This struct is meant to be used to represent the GKR gate within a gnark
// circuit and is used for the verifier part of GKR.
type RoundGate struct {
	Ark frontend.Variable
}

// NewRoundGateGnark creates a new RoundGate using the provided round constant
func NewRoundGateGnark(ark field.Element) *RoundGate {
	return &RoundGate{
		Ark: ark,
	}
}

func (m RoundGate) Evaluate(api gGkr.GateAPI, input ...frontend.Variable) frontend.Variable {

	if len(input) != 2 {
		panic("mimc has fan-in 2")
	}

	initialState := input[0]
	curr := input[1]

	// Compute the s-box (curr + init + ark)^17
	sum := api.Add(curr, initialState, m.Ark)

	sumPow16 := api.Mul(sum, sum)          // sum^2
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^4
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^8
	sumPow16 = api.Mul(sumPow16, sumPow16) // sum^16
	return api.Mul(sumPow16, sum)
}

func (m RoundGate) Degree() int {
	return 17
}

// RoundGate represents a normal round of gkr (i.e. any round except for the
// first and last ones). It represents the computation of the S-box of MiMC
//
//	(curr + init + ark)^17
//
// This struct is meant to be used for the prover part of GKR
type RoundGateCrypto struct {
	Ark field.Element
}

// NewRoundGateCrypto construct a new instance of a [RoundGate] with the
// caller-supplied round constant `ark`
func NewRoundGateCrypto(ark field.Element) *RoundGateCrypto {
	return &RoundGateCrypto{Ark: ark}
}

func (m RoundGateCrypto) Evaluate(inputs ...field.Element) field.Element {

	if len(inputs) != 2 {
		panic("mimc has fan-in 2")
	}

	initialState := inputs[0]
	curr := inputs[1]

	var sum, sumPow16 field.Element
	sum.Add(&initialState, &curr).Add(&sum, &m.Ark)
	sumPow16.Mul(&sum, &sum)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sumPow16.Mul(&sumPow16, &sumPow16)
	sum.Mul(&sumPow16, &sum)

	return sum
}

func (m RoundGateCrypto) Degree() int {
	return 17
}
