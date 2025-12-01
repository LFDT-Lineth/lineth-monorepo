package gkrmimc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/constraint/solver/gkrgates"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/gkrapi"
	"github.com/consensys/gnark/std/gkrapi/gkr"
	"github.com/consensys/gnark/std/multicommit"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

var (
	prefetchSize     = 4 // all layers before the "actual" MiMc round layers
	numGates     int = prefetchSize + len(mimc.Constants)
	gateNames    []gkr.GateName
)

func init() {
	// Registers the names of the GKR gates into the global GKR registry.
	createGateNames()
	if err := registerGates(); err != nil {
		panic(fmt.Errorf("failed to register gates: %w", err))
	}

	solver.RegisterHint(mimcHintfunc)
}

// writePaddedHex appends the integer `n` (assumedly less than 1<<(4*nbDigits))
// into `sbb` formatted: (1) in hexadecimal, (2) left padded with zeroes so that
// the total size of the appended string is `nbDigits` characters.
func writePaddedHex(sbb *strings.Builder, n, nbDigits int) {
	hex := strconv.FormatInt(int64(n), 16)
	sbb.WriteString(strings.Repeat("0", nbDigits-len(hex)))
	sbb.WriteString(hex)
}

// createGateName initializes and populates the `gateNames` global variable
// with the name of the gates forming each the GKR layers of the MiMC circuit.
func createGateNames() {
	nbDigits := 0
	for i := numGates; i > 0; i /= 16 {
		nbDigits++
	}

	gateNames = make([]gkr.GateName, numGates)

	gateNames[0] = "input" // no name necessary. will map to nil. this is just for clarity
	gateNames[1] = "input"
	gateNames[2] = "identity" // actual identity gate
	gateNames[3] = "identity"

	for i := 4; i < numGates; i++ {
		var gateNameBuilder strings.Builder
		gateNameBuilder.WriteString("mimc")
		writePaddedHex(&gateNameBuilder, i, nbDigits)
		gateNames[i] = gkr.GateName(gateNameBuilder.String())
	}
}

// registerGates instantiates and populates the cGkr and gkr global variables
// which contains the "normal" and the "gnark" version of the GKR gates forming
// the MiMC GKR circuit.
func registerGates() error {
	const (
		ROUND_GATE_NB_INPUTS = 2  // initial state and current state
		FINAL_GATE_NB_INPUTS = 3  // initial state, block and current state
		GATE_DEGREE          = 17 // MiMC S-box degree for BLS12-377
	)
	for i := 4; i < numGates-1; i++ {
		if err := gkrgates.Register(
			RoundGate(mimc.Constants[i-prefetchSize]),
			ROUND_GATE_NB_INPUTS,
			gkrgates.WithName(gateNames[i]),
			gkrgates.WithUnverifiedDegree(GATE_DEGREE),
			gkrgates.WithCurves(ecc.BLS12_377),
		); err != nil {
			return fmt.Errorf("failed to register gate %s: %v", gateNames[i], err)
		}
	}

	if err := gkrgates.Register(
		FinalRoundGate(mimc.Constants[len(mimc.Constants)-1]),
		FINAL_GATE_NB_INPUTS,
		gkrgates.WithName(gateNames[numGates-1]),
		gkrgates.WithUnverifiedDegree(GATE_DEGREE),
		gkrgates.WithCurves(ecc.BLS12_377),
	); err != nil {
		return fmt.Errorf("failed to register gate %s: %v", gateNames[numGates-1], err)
	}

	return nil
}

// gkrMiMC constructs and return the GKR circuit. The function is concretely
// responsible for declaring the topology of the MiMC circuit: "which" gate takes
// as input the result of "which" gate.
//
// The returned object symbolizees the last layer of the GKR circuit and formally
// contains the result of the MiMC block compression as computed by the GKR
// circuit.
func gkrMiMC(gkrapi *gkrapi.API, initStates, blocks []frontend.Variable) (gkr.Variable, error) {

	var err error
	v := make([]gkr.Variable, numGates-1)
	if v[0], err = gkrapi.Import(initStates); err != nil {
		panic(err)
	}
	if v[1], err = gkrapi.Import(blocks); err != nil {
		panic(err)
	}

	v[2] = gkrapi.NamedGate("identity", v[0])
	v[3] = gkrapi.NamedGate("identity", v[1])

	for i := 4; i < numGates-1; i++ {
		v[i] = gkrapi.NamedGate(gateNames[i], v[2], v[i-1])
	}

	res := gkrapi.NamedGate(gateNames[numGates-1], v[2], v[3], v[numGates-2])

	return res, nil
}

// checkWithGkr encapsulate the verification of the statement that: all
// triplets initStates[i], blocks[i] and allegedNewState[i] satisfies that
// allegedNewState == mimcBlockCompression(initState, block) within a gnark
// circuit
//
// Toy compression: just summing them up. TODO: Replace with actual compression
// instances are the inner indexes
func checkWithGkr(api frontend.API, initStates, blocks, allegedNewState []frontend.Variable) {

	gkr := gkrapi.New()

	D, err := gkrMiMC(gkr, initStates, blocks)
	if err != nil {
		panic(err)
	}

	// This creates a placeholder that will contain the GKR assignment
	var solution gkrapi.Solution
	if solution, err = gkr.Solve(api); err != nil {
		panic(err)
	}

	// builds the string of data that we need for the initial randomness. Note
	// that since this strings of data contains the full transcript of the rest
	// of the protocol
	feedToInitialRand := append(initStates, blocks...)
	feedToInitialRand = append(feedToInitialRand, allegedNewState...)

	multicommit.WithCommitment(
		api,
		func(api frontend.API, initialChallenge frontend.Variable) error {

			// "MIMC" means that we are using MiMC hashes to compute the FS challenges
			// this part is responsible for verifying the GKR proof.
			err = solution.Verify("MIMC", initialChallenge)
			if err != nil {
				panic(err)
			}

			// Export the last gkr layer as an array of frontend variable
			d := solution.Export(D)
			if len(d) != len(allegedNewState) {
				utils.Panic("length mismatch %v != %v", len(d), len(allegedNewState))
			}

			for i := range d {
				// Ensures GKR and
				api.AssertIsEqual(d[i], allegedNewState[i])
			}

			return nil
		},
		feedToInitialRand...,
	)

}

// RoundGate represents a normal round of gkr (i.e. any round except for the
// first and last ones). It represents the computation of the S-box of MiMC
//
//	(curr + init + ark)^17
//
// This struct is meant to be used to represent the GKR gate within a gnark
// circuit and is used for the verifier part of GKR.
func RoundGate(ark field.Element) gkr.GateFunction {
	return func(api gkr.GateAPI, input ...frontend.Variable) frontend.Variable {
		if len(input) != 2 {
			panic("mimc has fan-in 2")
		}

		initialState := input[0]
		curr := input[1]

		// Compute the s-box (curr + init + ark)^17
		sum := api.Add(curr, initialState, ark)

		sumPow16 := api.Mul(sum, sum)          // sum^2
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^4
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^8
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^16
		return api.Mul(sumPow16, sum)
	}
}

// FinalRoundGate represents the last round in a gnark circuit
//
// It performs all the actions required to complete the compression function of
// MiMC; including (1) the last application of the S-box x^17 as in the
// intermediate rounds and then adds twice the initial state and once the block
// to the result before returning.
func FinalRoundGate(ark field.Element) gkr.GateFunction {
	return func(api gkr.GateAPI, input ...frontend.Variable) frontend.Variable {
		if len(input) != 3 {
			utils.Panic("expected fan-in of 3, got %v", len(input))
		}

		// Parse the inputs
		initialState := input[0]
		block := input[1]
		currentState := input[2]

		// Compute the S-box function
		sum := api.Add(currentState, initialState, ark)
		sumPow16 := api.Mul(sum, sum)          // sum^2
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^4
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^8
		sumPow16 = api.Mul(sumPow16, sumPow16) // sum^16
		sum = api.Mul(sumPow16, sum)

		// And add back the last values, following the Miyaguchi-Preneel
		// construction.
		return api.Add(sum, initialState, initialState, block)
	}
}
