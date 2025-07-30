package gkrmimc

import (
	"strconv"
	"strings"

	cGkr "github.com/consensys/gnark-crypto/ecc/bls12-377/fr/gkr"
	"github.com/consensys/gnark/constraint"
	cs "github.com/consensys/gnark/constraint/bls12-377"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	gGkr "github.com/consensys/gnark/std/gkr"
	"github.com/consensys/gnark/std/hash"
	gmimc "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/gnark/std/multicommit"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/utils"
)

var (
	prefetchSize     = 4 // all layers before the "actual" MiMc round layers
	numGates     int = prefetchSize + len(mimc.Constants)
	gateNames    []string
)

func init() {
	// Registers the names of the GKR gates into the global GKR registry.
	createGateNames()
	registerGates()

	// Registers the mimc hash function in the hash builder registry.
	hash.Register("mimc", func(api frontend.API) (hash.FieldHasher, error) {
		h, err := gmimc.NewMiMC(api)
		return &h, err
	})

	// Registers the hasher to be used in the GKR prover
	cs.RegisterHashBuilder("mimc", mimc.NewMiMC)
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

	gateNames = make([]string, numGates)

	gateNames[0] = "input" // no name necessary. will map to nil. this is just for clarity
	gateNames[1] = "input"
	gateNames[2] = "identity" // actual identity gate
	gateNames[3] = "identity"

	for i := 4; i < numGates; i++ {
		var gateNameBuilder strings.Builder
		gateNameBuilder.WriteString("mimc")
		writePaddedHex(&gateNameBuilder, i, nbDigits)
		gateNames[i] = gateNameBuilder.String()
	}
}

// registerGates instantiates and populates the cGkr and gGkr global variables
// which contains the "normal" and the "gnark" version of the GKR gates forming
// the MiMC GKR circuit.
func registerGates() {

	var (
		cDegree17 = cGkr.WithDegree(17)
		gDegree17 = gGkr.WithUnverifiedDegree(17)
	)

	for i := 4; i < numGates-1; i++ {

		var (
			name  = gateNames[i]
			gateG = NewRoundGateGnark(mimc.Constants[i-prefetchSize])
			gateC = NewRoundGateCrypto(mimc.Constants[i-prefetchSize])
		)

		if e := gGkr.RegisterGate(gGkr.GateName(name), gateG.Evaluate, 2, gDegree17); e != nil {
			panic(e)
		}

		if e := cGkr.RegisterGate(cGkr.GateName(name), gateC.Evaluate, 2, cDegree17); e != nil {
			panic(e)
		}
	}

	var (
		name  = gateNames[numGates-1]
		gateG = NewFinalRoundGateGnark(mimc.Constants[len(mimc.Constants)-1])
		gateC = NewFinalRoundGateCrypto(mimc.Constants[len(mimc.Constants)-1])
	)

	if e := gGkr.RegisterGate(gGkr.GateName(name), gateG.Evaluate, 3, gDegree17); e != nil {
		panic(e)
	}
	if e := cGkr.RegisterGate(cGkr.GateName(name), gateC.Evaluate, 3, cDegree17); e != nil {
		panic(e)
	}
}

// gkrMiMC constructs and return the GKR circuit. The function is concretely
// responsible for declaring the topology of the MiMC circuit: "which" gate takes
// as input the result of "which" gate.
//
// The returned object symbolizees the last layer of the GKR circuit and formally
// contains the result of the MiMC block compression as computed by the GKR
// circuit.
func gkrMiMC(gkr *gGkr.API, initStates, blocks []frontend.Variable) (constraint.GkrVariable, error) {

	var err error
	v := make([]constraint.GkrVariable, numGates-1)
	if v[0], err = gkr.Import(initStates); err != nil {
		panic(err)
	}
	if v[1], err = gkr.Import(blocks); err != nil {
		panic(err)
	}

	v[2] = gkr.NamedGate("identity", v[0])
	v[3] = gkr.NamedGate("identity", v[1])

	for i := 4; i < numGates-1; i++ {
		v[i] = gkr.NamedGate(gGkr.GateName(gateNames[i]), v[2], v[i-1])
	}

	res := gkr.NamedGate(gGkr.GateName(gateNames[numGates-1]), v[2], v[3], v[numGates-2])

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

	gkr := gGkr.NewApi()

	D, err := gkrMiMC(gkr, initStates, blocks)
	if err != nil {
		panic(err)
	}

	// This creates a placeholder that will contain the GKR assignment
	var solution gGkr.Solution
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

			// "mimc" means that we are using MiMC hashes to compute the FS challenges
			// this part is responsible for verifying the GKR proof.
			err = solution.Verify("mimc", initialChallenge)
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
